package failover

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

var key = base64.StdEncoding.EncodeToString(make([]byte, 32))

func conf(ep string) string {
	return "[Interface]\nPrivateKey = " + key + "\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = " + key + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = " + ep + "\n"
}

// fakeTunnels имитирует tunnel.Manager: поднятые туннели, рукопожатия, пробы.
type fakeTunnels struct {
	mu        sync.Mutex
	now       func() time.Time
	up        map[string]bool
	handshake map[string]int64 // unix; 0 — нет
	probeFail map[string]bool
	upFail    map[string]bool
	primary   string
	ups       []string
	downs     []string
}

func newFake(now func() time.Time) *fakeTunnels {
	return &fakeTunnels{now: now, up: map[string]bool{}, handshake: map[string]int64{}, probeFail: map[string]bool{}, upFail: map[string]bool{}}
}

func (f *fakeTunnels) Status(context.Context) ([]tunnel.Iface, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []tunnel.Iface
	for n := range f.up {
		out = append(out, tunnel.Iface{Name: n, Peers: []tunnel.Peer{{LatestHandshake: f.handshake[n]}}})
	}
	return out, nil
}

func (f *fakeTunnels) Up(_ context.Context, name, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.upFail[name] {
		return errors.New("awg-quick: exit status 1")
	}
	if f.up[name] {
		return tunnel.ErrAlreadyUp
	}
	f.up[name] = true
	f.handshake[name] = f.now().Unix()
	f.ups = append(f.ups, name)
	if f.primary == "" {
		f.primary = name
	}
	return nil
}

func (f *fakeTunnels) Down(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.up, name)
	f.downs = append(f.downs, name)
	if f.primary == name {
		f.primary = ""
	}
	return nil
}

func (f *fakeTunnels) Primary() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.primary
}

func (f *fakeTunnels) SetPrimary(_ context.Context, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.up[name] {
		return tunnel.ErrNotUp
	}
	f.primary = name
	return nil
}

func (f *fakeTunnels) Probe(_ context.Context, name, _ string) (time.Duration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.probeFail[name] {
		return 0, errors.New("connection timed out")
	}
	return 20 * time.Millisecond, nil
}

func (f *fakeTunnels) set(fn func()) { f.mu.Lock(); fn(); f.mu.Unlock() }

type clock struct{ t time.Time }

func (c *clock) now() time.Time       { return c.t }
func (c *clock) step(d time.Duration) { c.t = c.t.Add(d) }
func (c *clock) unix() int64          { return c.t.Unix() }
func newClock() *clock                { return &clock{t: time.Unix(1_800_000_000, 0)} }
func refresh(f *fakeTunnels, c *clock, n ...string) {
	f.set(func() {
		for _, x := range n {
			f.handshake[x] = c.unix()
		}
	})
}

func setup(t *testing.T, mode string, failback bool, names ...string) (*Monitor, *fakeTunnels, *clock, *store.Store) {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i, n := range names {
		if err := st.WriteProfile(n, conf("192.0.2."+string(rune('1'+i))+":51820")); err != nil {
			t.Fatal(err)
		}
	}
	fo := store.DefaultFailover()
	fo.Mode, fo.Failback, fo.Threshold, fo.CooldownSec = mode, failback, 2, 60
	_ = st.Update(func(s *store.State) error { s.Failover = &fo; s.Priority = names; return nil })
	c := newClock()
	f := newFake(c.now)
	m := &Monitor{Store: st, Tunnels: f, Gateway: true, Now: c.now}
	return m, f, c, st
}

func TestOrder(t *testing.T) {
	if got := Order(nil, []string{"b", "a"}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("без приоритета — по имени: %v", got)
	}
	if got := Order([]string{"c", "gone", "a", "c"}, []string{"a", "b", "c"}); !reflect.DeepEqual(got, []string{"c", "a"}) {
		t.Fatalf("приоритет без удалённых, повторов и неучаствующих: %v", got)
	}
}

func TestStandbyRaisesAllAndSwitchesOnProbeFailure(t *testing.T) {
	m, f, c, st := setup(t, store.FailoverStandby, false, "a", "b", "c")
	var switches []string
	m.OnSwitch = func(from, to, reason string) { switches = append(switches, from+">"+to) }
	ctx := context.Background()

	m.Check(ctx)
	if len(f.up) != 3 || f.Primary() != "a" {
		t.Fatalf("standby поднимает все туннели, основной — первый: up=%v primary=%s", f.up, f.Primary())
	}
	// a перестаёт отвечать: одна неудача — ещё не переключение (порог 2)
	f.set(func() { f.probeFail["a"] = true })
	c.step(15 * time.Second)
	refresh(f, c, "a", "b", "c")
	s := m.Check(ctx)
	if f.Primary() != "a" || s.Tunnels[0].State != StateFailing {
		t.Fatalf("до порога переключения быть не должно: %s %+v", f.Primary(), s.Tunnels[0])
	}
	c.step(15 * time.Second)
	s = m.Check(ctx)
	if f.Primary() != "b" {
		t.Fatalf("после порога основным должен стать b, а не %s", f.Primary())
	}
	if s.Switches != 1 || s.LastSwitchReason == "" || !reflect.DeepEqual(switches, []string{"a>b"}) {
		t.Fatalf("переключение должно учитываться и сообщаться: %+v %v", s, switches)
	}
	if st.Snapshot().Primary != "b" {
		t.Fatal("основной туннель сохраняется для перезапуска")
	}
	if len(f.downs) != 0 {
		t.Fatalf("в standby туннели не опускаются: %v", f.downs)
	}
	// a снова работает, но возврата нет (failback выключен)
	f.set(func() { f.probeFail["a"] = false })
	for i := 0; i < 10; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a", "b", "c")
		m.Check(ctx)
	}
	if f.Primary() != "b" {
		t.Fatal("без failback основной не меняется, пока он исправен")
	}
}

func TestStandbyFailbackAfterCooldownAndStableChecks(t *testing.T) {
	m, f, c, _ := setup(t, store.FailoverStandby, true, "a", "b")
	ctx := context.Background()
	m.Check(ctx)
	f.set(func() { f.probeFail["a"] = true })
	for i := 0; i < 2; i++ {
		c.step(10 * time.Second)
		refresh(f, c, "a", "b")
		m.Check(ctx)
	}
	if f.Primary() != "b" {
		t.Fatalf("ожидалось переключение на b, основной %s", f.Primary())
	}
	f.set(func() { f.probeFail["a"] = false })
	c.step(10 * time.Second)
	refresh(f, c, "a", "b")
	s := m.Check(ctx)
	if f.Primary() != "b" {
		t.Fatal("во время паузы возврата быть не должно")
	}
	var ah Health
	for _, h := range s.Tunnels {
		if h.Name == "a" {
			ah = h
		}
	}
	if ah.State != StateCooldown || ah.CooldownLeft <= 0 {
		t.Fatalf("a должен быть на паузе: %+v", ah)
	}
	for i := 0; i < 8 && f.Primary() != "a"; i++ {
		c.step(10 * time.Second)
		refresh(f, c, "a", "b")
		m.Check(ctx)
	}
	if f.Primary() != "a" {
		t.Fatal("после паузы и устойчивой работы должен быть возврат к a")
	}
}

func TestHandshakeTriggerGraceAndStale(t *testing.T) {
	m, f, c, st := setup(t, store.FailoverStandby, false, "a", "b")
	_ = st.Update(func(s *store.State) error { s.Failover.Trigger = "handshake"; return nil })
	ctx := context.Background()
	m.Check(ctx)
	// у a рукопожатий нет совсем: в пределах GraceSec это «starting», не сбой
	f.set(func() { f.handshake["a"] = 0 })
	c.step(5 * time.Second)
	refresh(f, c, "b")
	s := m.Check(ctx)
	if s.Tunnels[0].Name != "a" || s.Tunnels[0].FailStreak != 0 {
		t.Fatalf("в период ожидания первого рукопожатия неудачи не считаются: %+v", s.Tunnels[0])
	}
	// окно ожидания (35 с) прошло — теперь это сбой
	for i := 0; i < 3; i++ {
		c.step(20 * time.Second)
		refresh(f, c, "b")
		m.Check(ctx)
	}
	if f.Primary() != "b" {
		t.Fatalf("без рукопожатия после ожидания должен быть переход на b: %s", f.Primary())
	}
	// у b рукопожатие устарело (> HandshakeMaxAge)
	f.set(func() { f.handshake["b"] = c.unix() - 500 })
	s = m.Check(ctx)
	for _, h := range s.Tunnels {
		if h.Name == "b" && (h.FailStreak != 1 || h.HandshakeAge < 500) {
			t.Fatalf("устаревшее рукопожатие — неудача: %+v", h)
		}
	}
}

func TestSwitchModeKeepsOnlyPrimaryAndMovesOnFailure(t *testing.T) {
	m, f, c, _ := setup(t, store.FailoverSwitch, false, "a", "b", "c")
	ctx := context.Background()
	_ = f.Up(ctx, "a", "")
	_ = f.Up(ctx, "b", "") // лишний для режима switch
	m.Check(ctx)
	if f.up["b"] || !f.up["a"] || f.Primary() != "a" {
		t.Fatalf("в режиме switch работает только основной: %v", f.up)
	}
	f.set(func() { f.probeFail["a"] = true; f.upFail["b"] = true })
	for i := 0; i < 2; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a")
		m.Check(ctx)
	}
	if f.up["a"] || !f.up["c"] || f.Primary() != "c" {
		t.Fatalf("a опускается, b не поднялся — основным должен стать c: up=%v primary=%s", f.up, f.Primary())
	}
}

func TestOffModeAndPlainModeNeverSwitch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    string
		gateway bool
	}{{"off", store.FailoverOff, true}, {"без шлюза", store.FailoverStandby, false}} {
		t.Run(tc.name, func(t *testing.T) {
			m, f, c, _ := setup(t, tc.mode, true, "a", "b")
			m.Gateway = tc.gateway
			ctx := context.Background()
			_ = f.Up(ctx, "a", "")
			_ = f.Up(ctx, "b", "")
			f.set(func() { f.probeFail["a"] = true })
			for i := 0; i < 5; i++ {
				c.step(15 * time.Second)
				refresh(f, c, "a", "b")
				s := m.Check(ctx)
				if i == 4 && s.Tunnels[0].State != StateUnhealthy {
					t.Fatalf("здоровье всё равно показывается: %+v", s.Tunnels[0])
				}
			}
			if f.Primary() != "a" {
				t.Fatalf("автоматического переключения быть не должно: %s", f.Primary())
			}
		})
	}
}

func TestExcludedTunnelIsIgnored(t *testing.T) {
	m, f, _, st := setup(t, store.FailoverStandby, false, "a", "b")
	if err := st.WriteProfile("x", conf("192.0.2.9:51820")); err != nil {
		t.Fatal(err)
	}
	s := m.Check(context.Background())
	if f.up["x"] {
		t.Fatal("туннель вне списка приоритета не поднимается монитором")
	}
	last := s.Tunnels[len(s.Tunnels)-1]
	if last.Name != "x" || last.State != StateExcluded || last.Priority != 0 {
		t.Fatalf("неучаствующий туннель — в конце списка: %+v", last)
	}
}

func TestFailoverSettingsValidation(t *testing.T) {
	ok := store.DefaultFailover()
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mut := range []func(*store.Failover){
		func(f *store.Failover) { f.Mode = "auto" },
		func(f *store.Failover) { f.Trigger = "magic" },
		func(f *store.Failover) { f.IntervalSec = 1 },
		func(f *store.Failover) { f.Threshold = 0 },
		func(f *store.Failover) { f.HandshakeMaxAge = 60 },
		func(f *store.Failover) { f.ProbeTarget = "example.com:443" },
		func(f *store.Failover) { f.ProbeTarget = "1.1.1.1" },
		func(f *store.Failover) { f.ProbeTarget = "0.0.0.0:443" },
	} {
		f := store.DefaultFailover()
		mut(&f)
		if f.Validate() == nil {
			t.Errorf("должно отвергаться: %+v", f)
		}
	}
	f := store.DefaultFailover()
	f.Trigger, f.ProbeTarget = "handshake", ""
	if f.Validate() != nil {
		t.Fatal("без пробы цель не нужна")
	}
}

func TestRecoveredTunnelInCooldownIsNotChosen(t *testing.T) {
	m, f, c, _ := setup(t, store.FailoverStandby, false, "a", "b", "c")
	ctx := context.Background()
	m.Check(ctx)
	f.set(func() { f.probeFail["a"] = true })
	for i := 0; i < 2; i++ {
		c.step(5 * time.Second)
		refresh(f, c, "a", "b", "c")
		m.Check(ctx)
	}
	if f.Primary() != "b" {
		t.Fatalf("ожидался переход на b: %s", f.Primary())
	}
	// a снова работает, но пауза (60 с) не прошла; теперь падает b
	f.set(func() { f.probeFail["a"] = false; f.probeFail["b"] = true })
	for i := 0; i < 2; i++ {
		c.step(5 * time.Second)
		refresh(f, c, "a", "b", "c")
		m.Check(ctx)
	}
	if f.Primary() != "c" {
		t.Fatalf("туннель на паузе не выбирается основным: ожидался c, получен %s", f.Primary())
	}
}

func TestFailbackWaitsForStableChecks(t *testing.T) {
	m, f, c, st := setup(t, store.FailoverStandby, true, "a", "b")
	_ = st.Update(func(s *store.State) error { s.Failover.CooldownSec = 0; return nil })
	ctx := context.Background()
	m.Check(ctx)
	f.set(func() { f.probeFail["a"] = true })
	for i := 0; i < 2; i++ {
		c.step(5 * time.Second)
		refresh(f, c, "a", "b")
		m.Check(ctx)
	}
	f.set(func() { f.probeFail["a"] = false })
	c.step(5 * time.Second)
	refresh(f, c, "a", "b")
	m.Check(ctx)
	if f.Primary() != "b" {
		t.Fatal("одной удачной проверки мало для возврата")
	}
	c.step(5 * time.Second)
	refresh(f, c, "a", "b")
	m.Check(ctx)
	if f.Primary() != "a" {
		t.Fatal("после Threshold удачных проверок подряд — возврат")
	}
}

func TestOnHealthReportsUsableTunnelsEvenOutsidePriorityList(t *testing.T) {
	m, f, c, st := setup(t, store.FailoverOff, false, "a", "b")
	if err := st.WriteProfile("x", conf("192.0.2.9:51820")); err != nil { // вне списка приоритета
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, n := range []string{"a", "b", "x"} {
		_ = f.Up(ctx, n, "")
	}
	var got []map[string]bool
	m.OnHealth = func(_ context.Context, u map[string]bool) { got = append(got, u) }
	f.set(func() { f.probeFail["b"] = true })
	for i := 0; i < 2; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a", "b", "x")
		m.Check(ctx)
	}
	last := got[len(got)-1]
	if !last["a"] || last["b"] || !last["x"] {
		t.Fatalf("исправны a и x (x — вне списка резервирования), b — нет: %v", last)
	}
	if len(got) != 2 {
		t.Fatalf("OnHealth вызывается после каждой проверки: %d", len(got))
	}
	// опущенный туннель исправным не считается
	_ = f.Down(ctx, "a")
	m.Check(ctx)
	if got[len(got)-1]["a"] {
		t.Fatal("опущенный туннель — не исправен")
	}
}

func TestGroupPrimaryIsNeverSwitchedByMonitor(t *testing.T) {
	m, f, c, _ := setup(t, store.FailoverStandby, true, "a", "b")
	ctx := context.Background()
	_ = f.Up(ctx, "a", "")
	_ = f.Up(ctx, "b", "")
	f.set(func() { f.primary = "pool" }) // основной — группа
	m.IsGroup = func(n string) bool { return n == "pool" }
	f.set(func() { f.probeFail["a"] = true })
	var switched bool
	m.OnSwitch = func(_, _, _ string) { switched = true }
	var s Status
	for i := 0; i < 4; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a", "b")
		s = m.Check(ctx)
	}
	if switched || f.Primary() != "pool" {
		t.Fatalf("основной — группа, монитор его не трогает: %q", f.Primary())
	}
	if !strings.Contains(s.Note, "pool") {
		t.Fatalf("в статусе поясняется, почему нет переключений: %q", s.Note)
	}
	// здоровье при этом считается как обычно
	if s.Tunnels[0].Name != "a" || s.Tunnels[0].State != StateUnhealthy {
		t.Fatalf("здоровье a: %+v", s.Tunnels[0])
	}
}

func TestSwitchModeNeverDownsGroupMembers(t *testing.T) {
	m, f, c, _ := setup(t, store.FailoverSwitch, false, "a", "b", "c")
	ctx := context.Background()
	_ = f.Up(ctx, "a", "")
	_ = f.Up(ctx, "b", "")
	_ = f.Up(ctx, "c", "")
	m.Keep = func(n string) bool { return n == "b" } // b — участник группы
	m.Check(ctx)
	if !f.up["b"] || f.up["c"] || !f.up["a"] {
		t.Fatalf("switch опускает лишнее (c), но не участника группы b: %v", f.up)
	}
	// основной a неисправен: b уже поднят, исправен — он и становится основным, a остаётся только если в группе
	f.set(func() { f.probeFail["a"] = true })
	for i := 0; i < 2; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a", "b")
		m.Check(ctx)
	}
	if f.Primary() != "b" || !f.up["b"] {
		t.Fatalf("основным должен стать уже поднятый исправный b: %q up=%v", f.Primary(), f.up)
	}
	// неисправный уже поднятый участник группы кандидатом не выбирается
	f.set(func() { f.probeFail["b"] = true; f.probeFail["a"] = false })
	m.Keep = func(n string) bool { return n == "b" || n == "a" }
	_ = f.Up(ctx, "a", "")
	for i := 0; i < 3; i++ {
		c.step(15 * time.Second)
		refresh(f, c, "a", "b")
		m.Check(ctx)
	}
	if !f.up["b"] {
		t.Fatalf("участник группы b не должен опускаться монитором: %v", f.up)
	}
}
