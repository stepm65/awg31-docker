// Package group — группы туннелей: балансировка новых соединений между
// несколькими VPS и резерв «первый исправный по порядку» (этап 5).
//
// Каждая группа получает собственную таблицу маршрутов (tunnel.GroupTableMin…
// GroupTableMax) с default-маршрутом через исправных участников:
//
//   - balance — ECMP: «nexthop dev A weight 2 nexthop dev B weight 1». Ядро
//     выбирает ветку по хешу соединения, поэтому соединение целиком идёт через
//     один туннель. Хеш по портам (fib_multipath_hash_policy=1) включается при
//     старте; без него все соединения одной пары «клиент → адрес» идут через
//     один туннель;
//   - ordered — «default dev X», где X — первый исправный участник списка.
//
// Здоровье берётся у монитора резервирования (рукопожатие и/или TCP-проба);
// неисправный или опущенный участник из маршрута убирается, вернувшийся —
// возвращается. Если исправных участников нет, при kill switch остаётся только
// blackhole: трафик не уходит мимо VPN.
//
// IPv6 идёт через одного участника (первого исправного): надёжность ECMP-
// замены маршрутов IPv6 зависит от ядра, а проверить её в каждом ядре нельзя.
package group

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Файлы sysctl (переменные для тестов).
var (
	HashPolicyV4 = "/proc/sys/net/ipv4/fib_multipath_hash_policy"
	HashPolicyV6 = "/proc/sys/net/ipv6/fib_multipath_hash_policy"
)

// TunnelAPI — то, что группам нужно от tunnel.Manager.
type TunnelAPI interface {
	Table(name string) int
	Options() tunnel.Options
	Primary() string
	RefreshPrimary(ctx context.Context)
}

type nexthop struct {
	Dev    string
	Weight int
}

// MemberStatus — участник группы для UI.
type MemberStatus struct {
	Name    string `json:"name"`
	Weight  int    `json:"weight"`
	Up      bool   `json:"up"`      // туннель поднят
	Healthy bool   `json:"healthy"` // монитор считает его исправным
	Active  bool   `json:"active"`  // участвует в маршруте сейчас
}

// Status — состояние группы для UI.
type Status struct {
	Name    string         `json:"name"`
	Mode    string         `json:"mode"`
	Table   int            `json:"table"`
	Primary bool           `json:"primary"`
	Active  int            `json:"active"`
	Route   string         `json:"route,omitempty"` // действующий маршрут IPv4
	Members []MemberStatus `json:"members"`
}

// Manager — группы туннелей.
type Manager struct {
	Store   *store.Store
	Tunnels TunnelAPI
	Runner  tunnel.Runner
	Logf    func(format string, args ...any)

	updMu   sync.Mutex // одно обновление за раз
	mu      sync.RWMutex
	tables  map[string]int
	healthy map[string]bool
	cur     map[string][]nexthop
	applied map[string]string // "4 150" -> маршрут
	force   bool              // ядро могло потерять ветки: переложить маршруты заново
	subs    []func()
	hashErr bool
	ruled   map[int]bool // правила fwmark → таблица уже созданы (только под updMu)
}

func (m *Manager) logf(f string, a ...any) {
	if m.Logf != nil {
		m.Logf(f, a...)
	}
}

func (m *Manager) init() {
	if m.tables == nil {
		m.tables, m.healthy, m.cur, m.applied = map[string]int{}, map[string]bool{}, map[string][]nexthop{}, map[string]string{}
	}
}

// OnChange регистрирует функцию, вызываемую при изменении состава активных
// участников какой-либо группы (например, чтобы пересчитать таблицу правил).
func (m *Manager) OnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subs = append(m.subs, fn)
}

// Has сообщает, что группа с таким именем задана.
func (m *Manager) Has(name string) bool {
	_, ok := m.Store.Snapshot().Group(name)
	return ok
}

// Members — туннели, состоящие хотя бы в одной группе (их не опускает режим
// switch монитора резервирования).
func (m *Manager) Members() map[string]bool {
	out := map[string]bool{}
	for _, g := range m.Store.Snapshot().Groups {
		for _, mem := range g.Members {
			out[mem.Name] = true
		}
	}
	return out
}

// Nexthop возвращает спецификацию следующих переходов группы для
// «ip route replace <префикс> …» (ok=false — исправных участников нет).
func (m *Manager) Nexthop(name string, v6 bool) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return spec(m.cur[name], v6)
}

func spec(act []nexthop, v6 bool) (string, bool) {
	switch {
	case len(act) == 0:
		return "", false
	case len(act) == 1 || v6:
		return "dev " + act[0].Dev, true
	}
	var b strings.Builder
	for i, n := range act {
		if i > 0 {
			b.WriteByte(' ')
		}
		fmt.Fprintf(&b, "nexthop dev %s weight %d", n.Dev, n.Weight)
	}
	return b.String(), true
}

// Table возвращает таблицу группы, если в ней есть исправные участники (0 — нет
// такой группы или она сейчас пуста). Так устроены метки DNS «через группу».
func (m *Manager) Table(name string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.cur[name]) == 0 {
		return 0
	}
	return m.tables[name]
}

// SetHealth сообщает, какие туннели исправны (вызывает монитор резервирования
// после каждой проверки), и пересчитывает группы.
func (m *Manager) SetHealth(ctx context.Context, healthy map[string]bool) {
	m.mu.Lock()
	m.init()
	m.healthy = healthy
	m.mu.Unlock()
	m.Update(ctx)
}

// TunnelsChanged вызывается после подъёма или остановки туннеля: ветки
// маршрутов, привязанные к исчезнувшему интерфейсу, ядро убирает само, поэтому
// маршруты кладутся заново, даже если состав не изменился.
func (m *Manager) TunnelsChanged() {
	m.mu.Lock()
	m.force = true
	m.mu.Unlock()
	m.Update(context.Background())
}

// Update приводит таблицы групп в соответствие с настройками и здоровьем.
func (m *Manager) Update(ctx context.Context) {
	m.updMu.Lock()
	defer m.updMu.Unlock()
	o := m.Tunnels.Options()
	gateway := o.AutoRoute && o.LanIface != ""
	snap := m.Store.Snapshot()

	m.mu.Lock()
	m.init()
	healthy, force := m.healthy, m.force
	m.force = false
	m.mu.Unlock()

	newCur := map[string][]nexthop{}
	groupTables := map[string]int{}
	used := map[int]bool{}
	m.mu.RLock()
	for _, g := range snap.Groups {
		if t, ok := m.tables[g.Name]; ok {
			groupTables[g.Name] = t
			used[t] = true
		}
	}
	m.mu.RUnlock()
	for _, g := range snap.Groups {
		if _, ok := groupTables[g.Name]; !ok {
			for t := tunnel.GroupTableMin; t <= tunnel.GroupTableMax; t++ {
				if !used[t] {
					groupTables[g.Name], used[t] = t, true
					break
				}
			}
		}
		var act []nexthop
		for _, mem := range g.Members {
			if healthy[mem.Name] && m.Tunnels.Table(mem.Name) != 0 {
				act = append(act, nexthop{Dev: mem.Name, Weight: mem.W()})
				if g.Mode == store.GroupOrdered {
					break
				}
			}
		}
		newCur[g.Name] = act
	}

	m.mu.RLock()
	oldTables, oldApplied := m.tables, m.applied
	m.mu.RUnlock()

	applied := map[string]string{}
	for k, v := range oldApplied {
		applied[k] = v
	}
	if gateway {
		m.ensureHash(snap.Groups)
		fams := []string{"-4"}
		if o.IPv6 {
			fams = append(fams, "-6")
		}
		// Удалённые группы убираются ДО укладки маршрутов: новая группа (например,
		// переименованная) может занять освободившуюся таблицу, и очистка не должна
		// стереть её маршрут и правило метки.
		for name, t := range oldTables {
			if _, ok := groupTables[name]; ok {
				continue
			}
			for _, fam := range fams {
				_, _ = m.Runner.Run(ctx, "ip", fam, "route", "flush", "table", strconv.Itoa(t))
				_, _ = m.Runner.Run(ctx, "ip", fam, "rule", "del", "priority", strconv.Itoa(tunnel.ProbeRulePriority(t)))
				delete(applied, fmt.Sprintf("%s %d", fam, t))
			}
			delete(m.ruled, t)
		}
		for _, g := range snap.Groups {
			m.applyTable(ctx, fams, groupTables[g.Name], newCur[g.Name], applied, force)
		}
	}

	m.mu.Lock()
	changed := !reflect.DeepEqual(m.cur, newCur) || len(oldTables) != len(groupTables) || force
	m.tables, m.cur, m.applied = groupTables, newCur, applied
	subs := append([]func(){}, m.subs...)
	m.mu.Unlock()

	if changed {
		m.Tunnels.RefreshPrimary(ctx)
		for _, fn := range subs {
			fn()
		}
	}
}

// applyTable кладёт default-маршрут группы в её таблицу. Пустая группа
// получает blackhole: «ip route replace» заменяет прежний маршрут одной
// операцией, поэтому окна без маршрута нет.
func (m *Manager) applyTable(ctx context.Context, fams []string, table int, act []nexthop, applied map[string]string, force bool) {
	tbl := strconv.Itoa(table)
	if m.ruled == nil {
		m.ruled = map[int]bool{}
	}
	for _, fam := range fams {
		key := fmt.Sprintf("%s %d", fam, table)
		want, ok := spec(act, fam == "-6")
		if !ok {
			want = "-"
		}
		if applied[key] == want && !force {
			continue
		}
		if !m.ruled[table] {
			// метка пробы и DNS «через группу» ведёт в таблицу группы
			prio := strconv.Itoa(tunnel.ProbeRulePriority(table))
			_, _ = m.Runner.Run(ctx, "ip", fam, "rule", "del", "priority", prio)
			_, _ = m.Runner.Run(ctx, "ip", fam, "rule", "add", "priority", prio, "fwmark", fmt.Sprintf("0x%x", tunnel.ProbeMark(table)), "lookup", tbl)
		}
		args := []string{fam, "route", "replace", "blackhole", "default"}
		if ok {
			args = append([]string{fam, "route", "replace", "default"}, strings.Fields(want)...)
		}
		if _, err := m.Runner.Run(ctx, "ip", append(args, "table", tbl)...); err != nil {
			if fam == "-4" {
				m.logf("group (table %d): %v", table, err)
			}
			delete(applied, key) // в следующий раз попробуем снова
			continue
		}
		applied[key] = want
	}
	m.ruled[table] = true
}

// ensureHash включает хеширование ECMP по портам, если есть группа balance.
func (m *Manager) ensureHash(groups []store.Group) {
	need := false
	for _, g := range groups {
		if g.Mode == store.GroupBalance && len(g.Members) > 1 {
			need = true
		}
	}
	if !need {
		return
	}
	for _, p := range []string{HashPolicyV4, HashPolicyV6} {
		if cur, err := os.ReadFile(p); err == nil && strings.TrimSpace(string(cur)) == "1" {
			continue
		}
		if err := os.WriteFile(p, []byte("1"), 0o644); err != nil && !m.hashErr {
			m.hashErr = true
			m.logf("balancing: could not enable per-port hashing (%s): %v; connections of one client-to-address pair will use a single tunnel; set net.ipv4.fib_multipath_hash_policy=1", p, err)
		}
	}
}

// HashPolicy возвращает действующее значение fib_multipath_hash_policy для IPv4
// (1 — по портам; "" — не удалось прочитать).
func HashPolicy() string {
	raw, err := os.ReadFile(HashPolicyV4)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

// Status возвращает состояние всех групп.
func (m *Manager) Status() []Status {
	snap := m.Store.Snapshot()
	primary := m.Tunnels.Primary()
	// туннели опрашиваются до блокировки: tunnel.Manager держит свою блокировку,
	// когда просит у группы маршрут, и два замка в разном порядке — взаимная
	// блокировка
	up := map[string]bool{}
	for _, g := range snap.Groups {
		for _, mem := range g.Members {
			up[mem.Name] = m.Tunnels.Table(mem.Name) != 0
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Status, 0, len(snap.Groups))
	for _, g := range snap.Groups {
		act := m.cur[g.Name]
		inAct := map[string]bool{}
		for _, a := range act {
			inAct[a.Dev] = true
		}
		st := Status{Name: g.Name, Mode: g.Mode, Table: m.tables[g.Name], Primary: g.Name == primary, Active: len(act)}
		st.Route, _ = spec(act, false)
		for _, mem := range g.Members {
			st.Members = append(st.Members, MemberStatus{
				Name: mem.Name, Weight: mem.W(), Up: up[mem.Name], Healthy: m.healthy[mem.Name], Active: inAct[mem.Name],
			})
		}
		out = append(out, st)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Router дополняет tunnel.Manager группами: Table(имя группы) возвращает её
// таблицу, Nexthop — маршрут. Передаётся политике и DNS-форвардеру.
type Router struct {
	*tunnel.Manager
	G *Manager
}

// Table — таблица туннеля или группы.
func (r Router) Table(name string) int {
	if t := r.G.Table(name); t != 0 {
		return t
	}
	return r.Manager.Table(name)
}

// Nexthop — маршрут группы; для имени туннеля ok=false.
func (r Router) Nexthop(name string, v6 bool) (string, bool) { return r.G.Nexthop(name, v6) }
