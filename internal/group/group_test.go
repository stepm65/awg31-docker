package group

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

type rec struct {
	mu    sync.Mutex
	calls []string
	fail  string // команда, содержащая эту подстроку, завершается ошибкой
}

func (r *rec) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, line)
	if r.fail != "" && strings.Contains(line, r.fail) {
		return nil, errors.New("RTNETLINK answers: Invalid argument")
	}
	return nil, nil
}

func (r *rec) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.calls
	r.calls = nil
	return c
}

func has(calls []string, sub string) bool {
	for _, c := range calls {
		if strings.Contains(c, sub) {
			return true
		}
	}
	return false
}

type fakeTM struct {
	mu      sync.Mutex
	tables  map[string]int
	opts    tunnel.Options
	primary string
	refresh int
}

func (f *fakeTM) Table(n string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tables[n]
}
func (f *fakeTM) Options() tunnel.Options { return f.opts }
func (f *fakeTM) Primary() string         { return f.primary }
func (f *fakeTM) RefreshPrimary(context.Context) {
	f.mu.Lock()
	f.refresh++
	f.mu.Unlock()
}

func setup(t *testing.T, groups ...store.Group) (*Manager, *rec, *fakeTM) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_ = st.Update(func(s *store.State) error { s.Groups = groups; return nil })
	r := &rec{}
	tm := &fakeTM{tables: map[string]int{"a": 110, "b": 111, "c": 112},
		opts: tunnel.Options{AutoRoute: true, LanIface: "eth0", KillSwitch: true}}
	dir := t.TempDir()
	HashPolicyV4, HashPolicyV6 = filepath.Join(dir, "v4"), filepath.Join(dir, "v6")
	_ = os.WriteFile(HashPolicyV4, []byte("0\n"), 0o644)
	_ = os.WriteFile(HashPolicyV6, []byte("0\n"), 0o644)
	return &Manager{Store: st, Tunnels: tm, Runner: r}, r, tm
}

func pool(mode string, members ...store.GroupMember) store.Group {
	return store.Group{Name: "pool", Mode: mode, Members: members}
}

func mem(n string, w int) store.GroupMember { return store.GroupMember{Name: n, Weight: w} }

func TestBalanceBuildsWeightedECMPRoute(t *testing.T) {
	m, r, _ := setup(t, pool(store.GroupBalance, mem("a", 2), mem("b", 0)))
	m.SetHealth(context.Background(), map[string]bool{"a": true, "b": true})
	calls := r.take()
	want := "ip -4 route replace default nexthop dev a weight 2 nexthop dev b weight 1 table 150"
	if !has(calls, want) {
		t.Fatalf("нет ECMP-маршрута %q:\n%s", want, strings.Join(calls, "\n"))
	}
	if !has(calls, "ip -4 rule add priority 1150 fwmark 0x31000096 lookup 150") {
		t.Fatalf("нет правила метки для DNS «через группу»:\n%s", strings.Join(calls, "\n"))
	}
	if s, ok := m.Nexthop("pool", false); !ok || s != "nexthop dev a weight 2 nexthop dev b weight 1" {
		t.Fatalf("Nexthop v4: %q %v", s, ok)
	}
	if s, ok := m.Nexthop("pool", true); !ok || s != "dev a" {
		t.Fatalf("IPv6 идёт через одного участника: %q %v", s, ok)
	}
	if m.Table("pool") != 150 || m.Table("nope") != 0 {
		t.Fatalf("таблицы: %d %d", m.Table("pool"), m.Table("nope"))
	}
	if HashPolicyV4 == "" {
		t.Fatal("путь sysctl")
	}
	if raw, _ := os.ReadFile(HashPolicyV4); strings.TrimSpace(string(raw)) != "1" {
		t.Fatalf("для balance включается хеш по портам: %q", raw)
	}
}

func TestUnhealthyAndDownMembersLeaveTheRoute(t *testing.T) {
	m, r, tm := setup(t, pool(store.GroupBalance, mem("a", 1), mem("b", 1), mem("c", 1)))
	ctx := context.Background()
	m.SetHealth(ctx, map[string]bool{"a": true, "b": true, "c": true})
	if s, _ := m.Nexthop("pool", false); strings.Count(s, "nexthop") != 3 {
		t.Fatalf("три участника: %q", s)
	}
	r.take()
	m.SetHealth(ctx, map[string]bool{"a": true, "b": false, "c": true}) // b неисправен
	if s, _ := m.Nexthop("pool", false); s != "nexthop dev a weight 1 nexthop dev c weight 1" {
		t.Fatalf("без b: %q", s)
	}
	if !has(r.take(), "route replace default nexthop dev a weight 1 nexthop dev c weight 1 table 150") {
		t.Fatal("маршрут не переложен")
	}
	tm.mu.Lock()
	delete(tm.tables, "c") // c опущен, хотя монитор ещё считает его исправным
	tm.mu.Unlock()
	m.SetHealth(ctx, map[string]bool{"a": true, "b": false, "c": true})
	if s, ok := m.Nexthop("pool", false); !ok || s != "dev a" {
		t.Fatalf("опущенный туннель не участвует: %q %v", s, ok)
	}
	// все неисправны: blackhole, а не «пусто» (трафик не уходит мимо VPN)
	r.take()
	m.SetHealth(ctx, map[string]bool{})
	calls := r.take()
	if !has(calls, "ip -4 route replace blackhole default table 150") {
		t.Fatalf("пустая группа:\n%s", strings.Join(calls, "\n"))
	}
	if has(calls, "route flush") || has(calls, "route del") {
		t.Fatalf("маршрут заменяется одной операцией, без окна без маршрута:\n%s", strings.Join(calls, "\n"))
	}
	if _, ok := m.Nexthop("pool", false); ok || m.Table("pool") != 0 {
		t.Fatal("в пустой группе нет ни маршрута, ни таблицы для DNS")
	}
	// возврат
	m.SetHealth(ctx, map[string]bool{"a": true})
	if s, ok := m.Nexthop("pool", false); !ok || s != "dev a" {
		t.Fatalf("после возврата: %q", s)
	}
}

func TestOrderedUsesFirstHealthyAndFailsBack(t *testing.T) {
	m, r, _ := setup(t, pool(store.GroupOrdered, mem("a", 5), mem("b", 1)))
	ctx := context.Background()
	m.SetHealth(ctx, map[string]bool{"a": true, "b": true})
	if s, _ := m.Nexthop("pool", false); s != "dev a" {
		t.Fatalf("первый по порядку: %q", s)
	}
	if !has(r.take(), "ip -4 route replace default dev a table 150") {
		t.Fatal("маршрут через a")
	}
	m.SetHealth(ctx, map[string]bool{"a": false, "b": true})
	if s, _ := m.Nexthop("pool", false); s != "dev b" {
		t.Fatalf("a упал — b: %q", s)
	}
	m.SetHealth(ctx, map[string]bool{"a": true, "b": true})
	if s, _ := m.Nexthop("pool", false); s != "dev a" {
		t.Fatalf("a вернулся — снова a: %q", s)
	}
	// режим ordered не включает ECMP-хеширование
	if raw, _ := os.ReadFile(HashPolicyV4); strings.TrimSpace(string(raw)) != "0" {
		t.Fatalf("sysctl трогать незачем: %q", raw)
	}
}

func TestUpdateIsQuietWhenNothingChangesAndForcedByTunnelEvents(t *testing.T) {
	m, r, tm := setup(t, pool(store.GroupBalance, mem("a", 1), mem("b", 1)))
	ctx := context.Background()
	changes := 0
	m.OnChange(func() { changes++ })
	h := map[string]bool{"a": true, "b": true}
	m.SetHealth(ctx, h)
	if changes != 1 || tm.refresh != 1 {
		t.Fatalf("первое применение: изменений %d, обновлений основного %d", changes, tm.refresh)
	}
	r.take()
	m.SetHealth(ctx, h)
	if got := r.take(); has(got, "route replace") || changes != 1 || tm.refresh != 1 {
		t.Fatalf("ничего не менялось, а команды выполнялись: %v (изменений %d)", got, changes)
	}
	// туннель перезапустился: ядро убрало его ветку, состав прежний — маршрут обязан лечь заново
	m.TunnelsChanged()
	if got := r.take(); !has(got, "route replace default nexthop dev a weight 1 nexthop dev b weight 1 table 150") || changes != 2 || tm.refresh != 2 {
		t.Fatalf("после перезапуска туннеля маршрут не переложен: %v (изменений %d)", got, changes)
	}
}

func TestFailedRouteCommandIsRetried(t *testing.T) {
	m, r, _ := setup(t, pool(store.GroupOrdered, mem("a", 1)))
	r.fail = "route replace default dev a"
	ctx := context.Background()
	m.SetHealth(ctx, map[string]bool{"a": true})
	r.take()
	r.fail = ""
	m.SetHealth(ctx, map[string]bool{"a": true})
	if !has(r.take(), "route replace default dev a table 150") {
		t.Fatal("после ошибки следующая проверка должна повторить команду")
	}
}

func TestRemovedGroupIsFlushedAndTablesAreStable(t *testing.T) {
	m, r, _ := setup(t,
		store.Group{Name: "g1", Mode: store.GroupOrdered, Members: []store.GroupMember{mem("a", 1)}},
		store.Group{Name: "g2", Mode: store.GroupOrdered, Members: []store.GroupMember{mem("b", 1)}})
	ctx := context.Background()
	m.SetHealth(ctx, map[string]bool{"a": true, "b": true})
	t1, t2 := m.tables["g1"], m.tables["g2"]
	if t1 == t2 || t1 < tunnel.GroupTableMin || t2 > tunnel.GroupTableMax {
		t.Fatalf("таблицы групп: %d %d", t1, t2)
	}
	_ = m.Store.Update(func(s *store.State) error { s.Groups = s.Groups[1:]; return nil }) // g1 удалена
	r.take()
	m.Update(ctx)
	calls := r.take()
	if !has(calls, "ip -4 route flush table 150") || !has(calls, "ip -4 rule del priority 1150") {
		t.Fatalf("таблица удалённой группы не очищена:\n%s", strings.Join(calls, "\n"))
	}
	if m.tables["g2"] != t2 {
		t.Fatal("таблица оставшейся группы не должна меняться")
	}
	if m.Has("g1") || !m.Has("g2") {
		t.Fatal("Has")
	}
}

func TestStatusAndMembers(t *testing.T) {
	m, _, tm := setup(t, pool(store.GroupBalance, mem("a", 3), mem("b", 1)))
	tm.primary = "pool"
	m.SetHealth(context.Background(), map[string]bool{"a": true})
	st := m.Status()
	if len(st) != 1 || !st[0].Primary || st[0].Active != 1 || st[0].Route != "dev a" || st[0].Table != 150 {
		t.Fatalf("состояние: %+v", st)
	}
	if !st[0].Members[0].Active || st[0].Members[0].Weight != 3 || st[0].Members[1].Active || st[0].Members[1].Healthy {
		t.Fatalf("участники: %+v", st[0].Members)
	}
	if mm := m.Members(); !mm["a"] || !mm["b"] || mm["c"] {
		t.Fatalf("Members: %v", mm)
	}
}

func TestNoRoutesWithoutGatewayMode(t *testing.T) {
	m, r, tm := setup(t, pool(store.GroupOrdered, mem("a", 1)))
	tm.opts.LanIface = ""
	m.SetHealth(context.Background(), map[string]bool{"a": true})
	if got := r.take(); len(got) != 0 {
		t.Fatalf("без режима шлюза таблицы не трогаются: %v", got)
	}
}

func indexOf(calls []string, sub string) int {
	idx := -1
	for i, c := range calls {
		if strings.Contains(c, sub) {
			idx = i // последнее вхождение
		}
	}
	return idx
}

// Группа переименована (старая удалена, новая с теми же участниками добавлена):
// новая занимает освободившуюся таблицу, и очистка таблицы старой группы не должна
// стереть маршрут и правило метки новой.
func TestRenamedGroupReusingFreedTableKeepsItsRoutes(t *testing.T) {
	m, r, _ := setup(t, store.Group{Name: "g1", Mode: store.GroupOrdered, Members: []store.GroupMember{mem("a", 1)}})
	ctx := context.Background()
	m.SetHealth(ctx, map[string]bool{"a": true})
	if m.tables["g1"] != 150 {
		t.Fatalf("исходная таблица: %d", m.tables["g1"])
	}
	_ = m.Store.Update(func(s *store.State) error {
		s.Groups = []store.Group{{Name: "g2", Mode: store.GroupOrdered, Members: []store.GroupMember{mem("a", 1)}}}
		return nil
	})
	r.take()
	m.Update(ctx)
	calls := r.take()
	if m.tables["g2"] != 150 {
		t.Fatalf("новая группа занимает освободившуюся таблицу: %d", m.tables["g2"])
	}
	route := indexOf(calls, "ip -4 route replace default dev a table 150")
	flush := indexOf(calls, "ip -4 route flush table 150")
	ruleAdd := indexOf(calls, "ip -4 rule add priority 1150")
	ruleDel := indexOf(calls, "ip -4 rule del priority 1150")
	if route < 0 || ruleAdd < 0 {
		t.Fatalf("у новой группы нет маршрута или правила метки:\n%s", strings.Join(calls, "\n"))
	}
	if flush > route || ruleDel > ruleAdd {
		t.Fatalf("очистка таблицы старой группы стёрла маршрут новой:\n%s", strings.Join(calls, "\n"))
	}
	// и дальше всё тихо: следующая проверка здоровья ничего не перекладывает
	r.take()
	m.SetHealth(ctx, map[string]bool{"a": true})
	if got := r.take(); len(got) != 0 {
		t.Fatalf("лишние команды: %v", got)
	}
}

func TestMemberWeightDefaultsToOneInTheRoute(t *testing.T) {
	m, r, _ := setup(t, pool(store.GroupBalance, store.GroupMember{Name: "a"}, store.GroupMember{Name: "b", Weight: 100}))
	m.SetHealth(context.Background(), map[string]bool{"a": true, "b": true})
	if want := "ip -4 route replace default nexthop dev a weight 1 nexthop dev b weight 100 table 150"; !has(r.take(), want) {
		t.Fatalf("нет маршрута %q", want)
	}
}
