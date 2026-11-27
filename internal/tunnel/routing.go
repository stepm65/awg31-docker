package tunnel

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"regexp"
	"strconv"
	"strings"

	"awg31/internal/conf"
)

// Режим шлюза устроен так же, как в a-amneziawg-mikrotik:
//
//   - маршруты из AllowedIPs каждого туннеля awg-quick кладёт в его собственную
//     таблицу (TunnelTableMin…TunnelTableMax, строка Table в рабочем конфиге);
//   - трафик LAN (правило iif <LAN> с приоритетом RulePriority) смотрит в общую
//     таблицу RouteTable, где лежит «default dev <основной туннель>»: смена
//     основного туннеля — одна команда ip route replace, без перезапуска;
//   - проба здоровья идёт через конкретный туннель по метке SO_MARK и правилу
//     fwmark -> таблица этого туннеля;
//   - собственный трафик контейнера (подписка, DNS, рукопожатия с сервером)
//     идёт по main напрямую, поэтому туннель не может отрезать сам себя;
//   - kill switch — постоянный blackhole default в RouteTable с метрикой 254 и
//     постоянное правило: пока туннеля нет, трафик LAN отбрасывается, а не
//     уходит обратно на роутер мимо VPN.
const (
	RouteTable     = 103
	RulePriority   = 300
	blackholeMetic = 254

	TunnelTableMin = 110
	TunnelTableMax = 149
	// GroupTableMin…GroupTableMax — таблицы групп туннелей (пакет group).
	GroupTableMin = 150
	GroupTableMax = 169
	probeRuleBase = 1000       // приоритет правила fwmark = probeRuleBase + таблица
	probeMarkBase = 0x31000000 // метка пробы = probeMarkBase | таблица
)

// ProbeMark — метка сокета для пробы через туннель с таблицей table.
func ProbeMark(table int) int { return probeMarkBase | table }

// ProbeRulePriority — приоритет правила «fwmark ProbeMark(table) → table».
func ProbeRulePriority(table int) int { return probeRuleBase + table }

// PrimaryMark — метка сокета, уводящая его в таблицу основного туннеля (103):
// так DNS-форвардер отправляет запросы «через VPN».
var PrimaryMark = ProbeMark(RouteTable)

var reIface = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

// ValidIfaceName проверяет имя сетевого интерфейса. Имя попадает в хуки
// awg-quick (они исполняются через eval), поэтому допускается только
// безопасный набор символов.
func ValidIfaceName(s string) bool { return reIface.MatchString(s) }

// IPv6Sysctl — файл, по которому видно, включён ли IPv6 в контейнере
// (переменная для тестов).
var IPv6Sysctl = "/proc/sys/net/ipv6/conf/all/disable_ipv6"

// IPv6Available сообщает, работает ли IPv6 в сетевом пространстве контейнера.
func IPv6Available() bool {
	raw, err := os.ReadFile(IPv6Sysctl)
	return err == nil && strings.TrimSpace(string(raw)) == "0"
}

// DetectLanIface находит интерфейс маршрута по умолчанию — это veth/eth0,
// через который роутер присылает трафик LAN.
func DetectLanIface(ctx context.Context, r Runner) (string, error) {
	out, err := r.Run(ctx, "ip", "-o", "-4", "route", "show", "default")
	if err != nil {
		return "", err
	}
	f := strings.Fields(string(out))
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "dev" {
			name, _, _ := strings.Cut(f[i+1], "@")
			if ValidIfaceName(name) {
				return name, nil
			}
			return "", fmt.Errorf("недопустимое имя интерфейса %q", f[i+1])
		}
	}
	return "", fmt.Errorf("нет маршрута по умолчанию: задайте AWG_LAN_IFACE")
}

func (m *Manager) gateway() bool { return m.opts.AutoRoute && m.opts.LanIface != "" }

// RouteHooks возвращает команды PostUp/PostDown для режима шлюза. %i
// заменяется awg-quick на имя интерфейса. table > 0 — собственная таблица
// туннеля: для неё добавляется правило fwmark, по которому ходит проба.
// awg-quick работает с set -e, поэтому всё, что может законно не сработать,
// завершается «|| true». В хуки попадают только числа и константы.
func RouteHooks(o Options, table int) conf.Hooks {
	var h conf.Hooks
	rule := func(tool, r, op string) string { return fmt.Sprintf(tool+" "+r, op) }
	add := func(tool, r string, soft bool) string {
		s := rule(tool, r, "-C") + " 2>/dev/null || " + rule(tool, r, "-A")
		if soft {
			s += " 2>/dev/null || true"
		}
		return s
	}
	del := func(tool, r string) string { return rule(tool, r, "-D") + " 2>/dev/null || true" }

	nat := "-t nat %s POSTROUTING -o %%i -j MASQUERADE"
	mss := "-t mangle %s FORWARD -o %%i -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu"
	h.PostUp = append(h.PostUp, add("iptables", nat, false), add("iptables", mss, false))
	h.PostDown = append(h.PostDown, del("iptables", nat), del("iptables", mss))
	if o.IPv6 {
		// NAT66 есть не в каждом ядре: без него туннель всё равно работает по IPv4.
		h.PostUp = append(h.PostUp, add("ip6tables", nat, true), add("ip6tables", mss, true))
		h.PostDown = append(h.PostDown, del("ip6tables", nat), del("ip6tables", mss))
	}
	if table < TunnelTableMin || table > TunnelTableMax {
		return h
	}
	fams := []string{"-4"}
	if o.IPv6 {
		fams = append(fams, "-6")
	}
	for _, fam := range fams {
		soft := ""
		if fam == "-6" {
			soft = " 2>/dev/null || true"
		}
		prio := probeRuleBase + table
		h.PostUp = append(h.PostUp, fmt.Sprintf("ip %s rule del priority %d 2>/dev/null || true; ip %s rule add priority %d fwmark 0x%x lookup %d%s",
			fam, prio, fam, prio, ProbeMark(table), table, soft))
		h.PostDown = append(h.PostDown, fmt.Sprintf("ip %s rule del priority %d 2>/dev/null || true", fam, prio))
	}
	return h
}

// allocTableLocked выдаёт туннелю свободную таблицу.
func (m *Manager) allocTableLocked(name string) (int, error) {
	if t, ok := m.tables[name]; ok {
		return t, nil
	}
	used := map[int]bool{}
	for _, t := range m.tables {
		used[t] = true
	}
	for t := TunnelTableMin; t <= TunnelTableMax; t++ {
		if !used[t] {
			m.tables[name] = t
			return t, nil
		}
	}
	return 0, fmt.Errorf("слишком много туннелей одновременно (максимум %d)", TunnelTableMax-TunnelTableMin+1)
}

// GroupRouter — группы туннелей (реализует пакет group). Tunnel не знает ни
// состава групп, ни их здоровья: ему нужен только маршрут для таблицы 103,
// когда основным назначена группа.
type GroupRouter interface {
	// Has — группа с таким именем существует.
	Has(name string) bool
	// Nexthop возвращает спецификацию следующих переходов для «ip route replace
	// default …» («dev wg0» или «nexthop dev wg0 weight 1 nexthop dev wg1 weight 1»);
	// ok=false — исправных участников нет.
	Nexthop(name string, v6 bool) (spec string, ok bool)
}

// SetGroups подключает группы туннелей.
func (m *Manager) SetGroups(g GroupRouter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.groups = g
}

// Primary — основной туннель (или группа), в который сейчас идёт трафик LAN ("" — нет).
func (m *Manager) Primary() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.primary
}

// Table — собственная таблица поднятого туннеля (0 — нет).
func (m *Manager) Table(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tables[name]
}

// SetPrimary направляет трафик LAN в туннель name (он должен быть поднят этим
// Manager). Переключение — ip route replace в таблице RouteTable, остальные
// туннели продолжают работать.
func (m *Manager) SetPrimary(ctx context.Context, name string) error {
	m.mu.Lock()
	prev := m.primary
	err := m.setPrimaryLocked(ctx, name)
	now := m.primary
	m.mu.Unlock()
	if err == nil && prev != now {
		m.changed() // правила с устройствами ведут PROXY прямо в основной туннель
	}
	return err
}

func (m *Manager) setPrimaryLocked(ctx context.Context, name string) error {
	if !m.gateway() {
		return fmt.Errorf("основной туннель выбирается только в режиме шлюза (нужен LAN-интерфейс)")
	}
	if m.groups != nil && m.groups.Has(name) {
		m.primary = name
		m.applyGroupPrimaryLocked(ctx)
		return nil
	}
	if _, ok := m.tables[name]; !ok {
		return ErrNotUp
	}
	tbl := strconv.Itoa(RouteTable)
	if _, err := m.r.Run(ctx, "ip", "-4", "route", "replace", "default", "dev", name, "table", tbl); err != nil {
		return fmt.Errorf("переключение на %s: %w", name, err)
	}
	if m.opts.IPv6 {
		_, _ = m.r.Run(ctx, "ip", "-6", "route", "replace", "default", "dev", name, "table", tbl)
	}
	if !m.opts.KillSwitch {
		// без kill switch правило живёт, только пока есть основной туннель
		m.lanRuleLocked(ctx, true)
	}
	m.primary = name
	return nil
}

// PrimaryIsGroup сообщает, что основной — группа: тогда выбором туннеля
// занимается группа, а монитор резервирования основной не переключает.
func (m *Manager) PrimaryIsGroup() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.primary != "" && m.groups != nil && m.groups.Has(m.primary)
}

// RefreshPrimary заново кладёт маршрут таблицы 103 для основной группы (состав
// исправных участников изменился). Если основной группы больше нет (удалена),
// трафик переводится на поднятый раньше других туннель. Для основного туннеля
// ничего не делает.
func (m *Manager) RefreshPrimary(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.gateway() || m.primary == "" {
		return
	}
	if _, isTunnel := m.tables[m.primary]; isTunnel {
		return
	}
	if m.groups != nil && m.groups.Has(m.primary) {
		m.applyGroupPrimaryLocked(ctx)
		return
	}
	m.primary = ""
	next, best := "", TunnelTableMax+1
	for n, t := range m.tables {
		if t < best {
			next, best = n, t
		}
	}
	if next != "" {
		if err := m.setPrimaryLocked(ctx, next); err != nil {
			m.logf("switching to tunnel %s failed: %v", next, err)
		}
	} else if !m.opts.KillSwitch {
		m.lanRuleLocked(ctx, false)
	}
}

// applyGroupPrimaryLocked выставляет default таблицы 103 по составу группы. Без
// исправных участников остаётся только kill switch (blackhole); без kill switch
// правило LAN снимается, как при отсутствии основного туннеля.
func (m *Manager) applyGroupPrimaryLocked(ctx context.Context) {
	tbl := strconv.Itoa(RouteTable)
	fams := []string{"-4"}
	if m.opts.IPv6 {
		fams = append(fams, "-6")
	}
	any := false
	for _, fam := range fams {
		spec, ok := m.groups.Nexthop(m.primary, fam == "-6")
		if !ok {
			// «ip route del default» без метрики мог бы снести и kill switch
			// (blackhole с метрикой 254), поэтому прежний маршрут заменяется
			// blackhole с метрикой по умолчанию
			_, _ = m.r.Run(ctx, "ip", fam, "route", "replace", "blackhole", "default", "table", tbl)
			continue
		}
		args := append([]string{fam, "route", "replace", "default"}, strings.Fields(spec)...)
		args = append(args, "table", tbl)
		if _, err := m.r.Run(ctx, "ip", args...); err != nil {
			if fam == "-4" {
				m.logf("primary group %s: %v", m.primary, err)
			}
			continue
		}
		if fam == "-4" {
			any = true
		}
	}
	if !m.opts.KillSwitch {
		m.lanRuleLocked(ctx, any)
	}
}

func (m *Manager) lanRuleLocked(ctx context.Context, on bool) {
	prio := strconv.Itoa(RulePriority)
	fams := []string{"-4"}
	if m.opts.IPv6 {
		fams = append(fams, "-6")
	}
	for _, fam := range fams {
		_, _ = m.r.Run(ctx, "ip", fam, "rule", "del", "priority", prio)
		if on {
			_, _ = m.r.Run(ctx, "ip", fam, "rule", "add", "priority", prio, "from", "all", "iif", m.opts.LanIface, "lookup", strconv.Itoa(RouteTable))
		}
	}
}

// EnsureRouting приводит постоянную часть маршрутизации в соответствие с
// настройками: при включённом kill switch правило и blackhole существуют всегда,
// при выключенном — убираются (их вернут хуки поднятого туннеля). Команды
// выполняются без shell.
func (m *Manager) EnsureRouting(ctx context.Context) error {
	if !m.gateway() {
		return nil
	}
	if !ValidIfaceName(m.opts.LanIface) {
		return fmt.Errorf("недопустимое имя LAN-интерфейса %q", m.opts.LanIface)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	tbl, prio, metric := strconv.Itoa(RouteTable), strconv.Itoa(RulePriority), strconv.Itoa(blackholeMetic)
	fams := []string{"-4"}
	if m.opts.IPv6 {
		fams = append(fams, "-6")
	}
	var firstErr error
	markPrio := strconv.Itoa(probeRuleBase + RouteTable)
	for _, fam := range fams {
		// метка PrimaryMark -> таблица основного туннеля (для DNS «через VPN»)
		_, _ = m.r.Run(ctx, "ip", fam, "rule", "del", "priority", markPrio)
		_, _ = m.r.Run(ctx, "ip", fam, "rule", "add", "priority", markPrio, "fwmark", fmt.Sprintf("0x%x", PrimaryMark), "lookup", tbl)
		_, _ = m.r.Run(ctx, "ip", fam, "rule", "del", "priority", prio)
		if m.opts.KillSwitch {
			_, err := m.r.Run(ctx, "ip", fam, "rule", "add", "priority", prio, "from", "all", "iif", m.opts.LanIface, "lookup", tbl)
			if err == nil {
				_, err = m.r.Run(ctx, "ip", fam, "route", "replace", "blackhole", "default", "table", tbl, "metric", metric)
			}
			if err != nil && fam == "-4" && firstErr == nil {
				firstErr = fmt.Errorf("kill switch не включён: %w", err)
			}
		} else {
			_, _ = m.r.Run(ctx, "ip", fam, "route", "del", "blackhole", "default", "table", tbl, "metric", metric)
		}
	}
	m.routingErr = ""
	if firstErr != nil {
		m.routingErr = Redact(firstErr.Error())
	}
	return firstErr
}

// prepare превращает проверенный профиль в рабочий конфиг для этого
// контейнера и возвращает замечания для журнала.
func (m *Manager) prepare(cfg *conf.Config, table int) (*conf.Config, []string) {
	out := &conf.Config{Interface: conf.Section{Name: cfg.Interface.Name}}
	var notes []string
	hasV6Addr := false
	strippedV6 := false

	keepFamily := func(v string) (string, bool) {
		var kept []string
		dropped := false
		for _, item := range strings.Split(v, ",") {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if !m.opts.IPv6 && isV6(item) {
				dropped = true
				continue
			}
			kept = append(kept, item)
		}
		return strings.Join(kept, ", "), dropped
	}

	for _, p := range cfg.Interface.Pairs {
		switch p.Key {
		case "Table":
			if m.gateway() && table > 0 {
				notes = append(notes, fmt.Sprintf("строка Table = %s заменена: в режиме шлюза маршруты туннеля идут в таблицу %d", p.Value, table))
				continue
			}
		case "DNS":
			if m.gateway() {
				// Собственный трафик контейнера идёт мимо туннеля, и DNS из
				// профиля (часто адрес внутри VPN) был бы ему недоступен.
				notes = append(notes, "строка DNS не применяется в режиме шлюза: DNS для клиентов LAN раздаёт роутер")
				continue
			}
		case "Address":
			v, dropped := keepFamily(p.Value)
			strippedV6 = strippedV6 || dropped
			for _, item := range strings.Split(v, ",") {
				if isV6(strings.TrimSpace(item)) {
					hasV6Addr = true
				}
			}
			if v == "" {
				continue
			}
			p.Value = v
		}
		out.Interface.Pairs = append(out.Interface.Pairs, p)
	}
	if m.gateway() && table > 0 {
		out.Interface.Pairs = append(out.Interface.Pairs, conf.Pair{Key: "Table", Value: strconv.Itoa(table)})
	}
	if hasV6Addr {
		for i, p := range out.Interface.Pairs {
			if p.Key == "MTU" {
				if n, err := strconv.Atoi(p.Value); err == nil && n < 1280 {
					out.Interface.Pairs[i].Value = "1280"
					notes = append(notes, fmt.Sprintf("MTU %d меньше минимума IPv6 (1280), поднят до 1280", n))
				}
			}
		}
	}
	for _, peer := range cfg.Peers {
		np := conf.Section{Name: peer.Name}
		for _, p := range peer.Pairs {
			if p.Key == "AllowedIPs" {
				v, dropped := keepFamily(p.Value)
				strippedV6 = strippedV6 || dropped
				if v == "" {
					continue
				}
				p.Value = v
			}
			np.Pairs = append(np.Pairs, p)
		}
		out.Peers = append(out.Peers, np)
	}
	if strippedV6 {
		notes = append(notes, "IPv6 в контейнере выключен: IPv6-адреса и маршруты убраны из рабочего конфига")
	}
	return out, notes
}

func isV6(item string) bool {
	if pfx, err := netip.ParsePrefix(item); err == nil {
		return pfx.Addr().Is6() && !pfx.Addr().Is4In6()
	}
	if a, err := netip.ParseAddr(item); err == nil {
		return a.Is6() && !a.Is4In6()
	}
	return false
}
