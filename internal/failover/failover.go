// Package failover следит за здоровьем туннелей и переключает основной
// туннель (этап 2: Multi-Tunnel + failover, по мотивам a-amneziawg-mikrotik).
//
// Режимы (store.Failover.Mode):
//
//   - off     — ничего не переключается автоматически; основной туннель
//     выбирается вручную, одновременно может работать сколько угодно туннелей;
//   - standby — все туннели из списка приоритета подняты; при сбое основного
//     трафик LAN мгновенно переводится на следующий исправный
//     (ip route replace, без перезапуска); по желанию — возврат к более
//     приоритетному, когда он снова исправен;
//   - switch  — поднят только основной; при сбое он опускается, и
//     поднимается следующий по приоритету.
//
// Сбой определяется по признаку Trigger: handshake (рукопожатие старше
// HandshakeMaxAge или его нет дольше GraceSec после подъёма), probe
// (TCP-соединение с ProbeTarget через этот туннель) или hybrid (любое из двух).
// Туннель считается неисправным после Threshold неудач подряд и получает паузу
// CooldownSec, в течение которой не выбирается основным.
package failover

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Tunnels — то, что монитору нужно от tunnel.Manager (подменяется в тестах).
type Tunnels interface {
	Status(ctx context.Context) ([]tunnel.Iface, error)
	Up(ctx context.Context, name, content string) error
	Down(ctx context.Context, name string) error
	Primary() string
	SetPrimary(ctx context.Context, name string) error
	Probe(ctx context.Context, name, target string) (time.Duration, error)
}

// Состояния туннеля.
const (
	StateHealthy   = "healthy"   // проверки проходят
	StateStarting  = "starting"  // только что поднят, ждём первое рукопожатие
	StateFailing   = "failing"   // есть неудачи, порог ещё не достигнут
	StateUnhealthy = "unhealthy" // порог неудач достигнут
	StateCooldown  = "cooldown"  // после сбоя: пауза, основным не выбирается
	StateDown      = "down"      // не поднят
	StateExcluded  = "excluded"  // не участвует в резервировании
)

// Health — состояние одного туннеля для UI.
type Health struct {
	Name         string `json:"name"`
	Up           bool   `json:"up"`
	Primary      bool   `json:"primary"`
	Priority     int    `json:"priority"` // 1 — самый приоритетный, 0 — не участвует
	State        string `json:"state"`
	Reason       string `json:"reason,omitempty"`
	FailStreak   int    `json:"fail_streak"`
	HandshakeAge int64  `json:"handshake_age"` // секунды, -1 — рукопожатия не было
	ProbeOK      *bool  `json:"probe_ok,omitempty"`
	ProbeMs      int64  `json:"probe_ms,omitempty"`
	CooldownLeft int64  `json:"cooldown_left,omitempty"`
}

// Status — общее состояние для UI и API.
type Status struct {
	Settings         store.Failover `json:"settings"`
	Gateway          bool           `json:"gateway"`
	Primary          string         `json:"primary"`
	Tunnels          []Health       `json:"tunnels"`
	LastCheck        time.Time      `json:"last_check,omitempty"`
	LastSwitch       time.Time      `json:"last_switch,omitempty"`
	LastSwitchReason string         `json:"last_switch_reason,omitempty"`
	Switches         int            `json:"switches"`
	// Note — пояснение для UI (например, что основной — группа).
	Note string `json:"note,omitempty"`
}

// Monitor — монитор резервирования.
type Monitor struct {
	Store   *store.Store
	Tunnels Tunnels
	// Gateway — режим шлюза с LAN-интерфейсом: без него переключать нечего.
	Gateway bool
	Logf    func(format string, args ...any)
	// Exclude — туннели, которые не участвуют в резервировании (профили серверов).
	Exclude func(name string) bool
	// OnSwitch вызывается после переключения основного туннеля (для уведомлений).
	OnSwitch func(from, to, reason string)
	// OnHealth вызывается после каждой проверки: какие туннели сейчас исправны
	// (поднят и состояние healthy). Группы туннелей строят по нему маршруты.
	OnHealth func(ctx context.Context, usable map[string]bool)
	// Keep — туннели, которые режим switch не должен опускать (участники групп).
	Keep func(name string) bool
	// IsGroup — основной назначен группой: выбором туннеля занимается группа, а
	// монитор основной не переключает (необязательно).
	IsGroup func(name string) bool
	Now     func() time.Time

	checkMu  sync.Mutex // одна проверка за раз
	mu       sync.Mutex
	fail     map[string]int
	okStreak map[string]int
	cooldown map[string]time.Time
	started  map[string]time.Time
	status   Status
	wake     chan struct{}
	once     sync.Once
}

func (m *Monitor) init() {
	m.once.Do(func() {
		m.fail, m.okStreak = map[string]int{}, map[string]int{}
		m.cooldown, m.started = map[string]time.Time{}, map[string]time.Time{}
		m.wake = make(chan struct{}, 1)
		if m.Now == nil {
			m.Now = time.Now
		}
	})
}

func (m *Monitor) logf(format string, args ...any) {
	if m.Logf != nil {
		m.Logf(format, args...)
	}
}

// Order возвращает туннели, участвующие в резервировании, в порядке
// приоритета: список Priority (только существующие профили) или, если он
// пуст, все профили по имени.
func Order(priority, profiles []string) []string {
	exists := map[string]bool{}
	for _, p := range profiles {
		exists[p] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, p := range priority {
		if exists[p] && !seen[p] {
			out = append(out, p)
			seen[p] = true
		}
	}
	if len(priority) == 0 {
		out = append(out, profiles...)
		sort.Strings(out)
	}
	return out
}

// Wake запускает внеочередную проверку (после изменения настроек).
func (m *Monitor) Wake() {
	m.init()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Status возвращает последнее известное состояние.
func (m *Monitor) Status() Status {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.status
	st.Tunnels = append([]Health(nil), st.Tunnels...)
	return st
}

// Run выполняет проверки до отмены ctx. Период берётся из настроек при
// каждой итерации.
func (m *Monitor) Run(ctx context.Context) {
	m.init()
	timer := time.NewTimer(10 * time.Second) // дать автозапуску поднять туннели
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		case <-m.wake:
		}
		m.Check(ctx)
		interval := time.Duration(m.Store.Snapshot().FailoverSettings().IntervalSec) * time.Second
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(interval)
	}
}

// Check проверяет туннели и при необходимости переключает основной.
func (m *Monitor) Check(ctx context.Context) Status {
	m.init()
	m.checkMu.Lock()
	defer m.checkMu.Unlock()

	st := m.Store.Snapshot()
	cfg := st.FailoverSettings()
	profiles, err := m.Store.ListProfiles()
	if err != nil {
		m.logf("резервирование: список профилей: %v", err)
		return m.Status()
	}
	order := Order(st.Priority, profiles)
	if m.Exclude != nil {
		kept := order[:0]
		for _, n := range order {
			if !m.Exclude(n) {
				kept = append(kept, n)
			}
		}
		order = kept
	}
	rank := map[string]int{}
	for i, n := range order {
		rank[n] = i + 1
	}
	now := m.Now()

	ifaces, err := m.Tunnels.Status(ctx)
	if err != nil {
		m.logf("резервирование: статус туннелей: %v", err)
		return m.Status()
	}
	up := map[string]tunnel.Iface{}
	for _, i := range ifaces {
		up[i.Name] = i
	}

	m.mu.Lock()
	for n := range m.started {
		if _, ok := up[n]; !ok {
			delete(m.started, n)
		}
	}
	for n := range up {
		if _, ok := m.started[n]; !ok {
			m.started[n] = now
		}
	}
	m.mu.Unlock()

	health := map[string]*Health{}
	usable := map[string]bool{}
	for _, name := range profiles {
		h := &Health{Name: name, Priority: rank[name], HandshakeAge: -1}
		iface, isUp := up[name]
		h.Up = isUp
		if !isUp {
			h.State = StateDown
			m.mu.Lock()
			m.fail[name], m.okStreak[name] = 0, 0
			m.mu.Unlock()
		} else {
			m.evaluate(ctx, cfg, now, iface, h)
		}
		usable[name] = h.Up && h.State == StateHealthy // до пометки «не в списке»: группам список приоритета не важен
		if rank[name] == 0 {
			h.State, h.Reason = StateExcluded, "не в списке резервирования"
		}
		health[name] = h
	}

	m.mu.Lock()
	m.status.Settings, m.status.Gateway, m.status.LastCheck = cfg, m.Gateway, now
	m.mu.Unlock()

	cur := m.Tunnels.Primary()
	groupPrimary := m.IsGroup != nil && cur != "" && m.IsGroup(cur)
	note := ""
	if groupPrimary && cfg.Mode != store.FailoverOff {
		note = "основной — группа " + cur + ": переключение по списку приоритета не выполняется"
	}
	m.mu.Lock()
	m.status.Note = note
	m.mu.Unlock()
	if m.Gateway && cfg.Mode != store.FailoverOff && len(order) > 0 && !groupPrimary {
		switch cfg.Mode {
		case store.FailoverStandby:
			m.standby(ctx, cfg, order, health, now)
		case store.FailoverSwitch:
			m.switchMode(ctx, order, health, now)
		}
	}

	primary := m.Tunnels.Primary()
	list := make([]Health, 0, len(profiles))
	for _, name := range profiles {
		h := health[name]
		h.Primary = name == primary
		list = append(list, *h)
	}
	sort.SliceStable(list, func(i, j int) bool {
		ri, rj := list[i].Priority, list[j].Priority
		if (ri == 0) != (rj == 0) {
			return rj == 0
		}
		return ri < rj
	})
	m.mu.Lock()
	m.status.Primary = primary
	m.status.Tunnels = list
	m.mu.Unlock()
	if m.OnHealth != nil {
		m.OnHealth(ctx, usable)
	}
	return m.Status()
}

// evaluate проверяет поднятый туннель и обновляет счётчики.
func (m *Monitor) evaluate(ctx context.Context, cfg store.Failover, now time.Time, iface tunnel.Iface, h *Health) {
	var newest int64
	for _, p := range iface.Peers {
		if p.LatestHandshake > newest {
			newest = p.LatestHandshake
		}
	}
	if newest > 0 {
		h.HandshakeAge = max(0, now.Unix()-newest)
	}
	m.mu.Lock()
	started := m.started[h.Name]
	m.mu.Unlock()
	grace := newest == 0 && now.Sub(started) < time.Duration(cfg.GraceSec)*time.Second

	failed, reason := false, ""
	if cfg.Trigger != "probe" && !grace {
		switch {
		case newest == 0:
			failed, reason = true, "нет рукопожатия"
		case h.HandshakeAge > int64(cfg.HandshakeMaxAge):
			failed, reason = true, fmt.Sprintf("рукопожатие %d с назад", h.HandshakeAge)
		}
	}
	if cfg.Trigger != "handshake" && !failed {
		pctx, cancel := context.WithTimeout(ctx, 6*time.Second)
		rtt, err := m.Tunnels.Probe(pctx, h.Name, cfg.ProbeTarget)
		cancel()
		ok := err == nil
		h.ProbeOK = &ok
		if ok {
			h.ProbeMs = max(1, rtt.Milliseconds())
		} else if !grace {
			failed, reason = true, "проба "+cfg.ProbeTarget+": "+shortErr(err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case failed:
		m.fail[h.Name]++
		m.okStreak[h.Name] = 0
	case grace && (h.ProbeOK == nil || !*h.ProbeOK):
		// ещё не доказал работоспособность и не провалился
	default:
		m.fail[h.Name] = 0
		m.okStreak[h.Name]++
	}
	h.FailStreak = m.fail[h.Name]
	h.Reason = reason
	switch {
	case h.FailStreak >= cfg.Threshold:
		h.State = StateUnhealthy
		if until, ok := m.cooldown[h.Name]; !ok || !until.After(now) {
			m.cooldown[h.Name] = now.Add(time.Duration(cfg.CooldownSec) * time.Second)
		}
	case h.FailStreak > 0:
		h.State = StateFailing
	case grace && m.okStreak[h.Name] == 0:
		h.State, h.Reason = StateStarting, "ждём первое рукопожатие"
	default:
		h.State = StateHealthy
	}
	if until := m.cooldown[h.Name]; until.After(now) {
		h.CooldownLeft = int64((until.Sub(now) + time.Second - 1) / time.Second)
		if h.State == StateHealthy || h.State == StateStarting {
			h.State = StateCooldown
		}
	}
}

func shortErr(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "нет ответа"
	}
	s := err.Error()
	if len(s) > 120 {
		s = s[:120] + "…"
	}
	return s
}

// eligible — туннель можно сделать основным.
func eligible(h *Health) bool {
	return h != nil && h.Up && h.State == StateHealthy
}

func (m *Monitor) standby(ctx context.Context, cfg store.Failover, order []string, health map[string]*Health, now time.Time) {
	for _, name := range order {
		if h := health[name]; !h.Up {
			if err := m.up(ctx, name); err != nil {
				h.Reason = "не поднялся: " + shortErr(err)
				continue
			}
			h.Up, h.State, h.Reason = true, StateStarting, "поднят резервом"
		}
	}
	primary := m.Tunnels.Primary()
	ph := health[primary]
	need := primary == "" || ph == nil || !ph.Up || ph.State == StateUnhealthy
	target := ""
	for _, name := range order {
		if name != primary && eligible(health[name]) {
			target = name
			break
		}
	}
	switch {
	case need && target != "":
		reason := "нет основного туннеля"
		if primary != "" {
			reason = fmt.Sprintf("%s неисправен: %s", primary, orDash(health[primary]))
		}
		m.switchTo(ctx, primary, target, reason, now)
	case need:
		if primary != "" {
			m.logf("резервирование: %s неисправен, но исправной замены нет", primary)
		}
	case cfg.Failback && target != "" && rankOf(order, target) < rankOf(order, primary):
		m.mu.Lock()
		stable := m.okStreak[target] >= cfg.Threshold
		m.mu.Unlock()
		if stable {
			m.switchTo(ctx, primary, target, fmt.Sprintf("возврат к более приоритетному %s", target), now)
		}
	}
}

func (m *Monitor) switchMode(ctx context.Context, order []string, health map[string]*Health, now time.Time) {
	primary := m.Tunnels.Primary()
	ph := health[primary]
	if primary != "" && ph != nil && ph.Up && ph.State != StateUnhealthy {
		// основной в порядке: остальные из списка не нужны
		for _, name := range order {
			if name != primary && health[name].Up && !m.keep(name) {
				_ = m.Tunnels.Down(ctx, name)
				health[name].Up, health[name].State = false, StateDown
			}
		}
		return
	}
	reason := "нет основного туннеля"
	if primary != "" {
		reason = fmt.Sprintf("%s неисправен: %s", primary, orDash(ph))
		if !m.keep(primary) {
			_ = m.Tunnels.Down(ctx, primary)
			if ph != nil {
				ph.Up, ph.State = false, StateCooldown
			}
		}
	}
	for _, name := range order {
		if name == primary {
			continue
		}
		m.mu.Lock()
		cool := m.cooldown[name].After(now)
		m.mu.Unlock()
		if cool {
			continue
		}
		h := health[name]
		if !h.Up {
			if err := m.up(ctx, name); err != nil {
				h.Reason = "не поднялся: " + shortErr(err)
				continue
			}
			h.Up, h.State = true, StateStarting
		} else if h.State != StateHealthy {
			continue // уже поднят (участник группы), но неисправен
		}
		m.switchTo(ctx, primary, name, reason, now)
		return
	}
	m.logf("резервирование: %s — исправной замены нет", reason)
}

func (m *Monitor) keep(name string) bool { return m.Keep != nil && m.Keep(name) }

func (m *Monitor) up(ctx context.Context, name string) error {
	content, err := m.Store.ReadProfile(name)
	if err != nil {
		return err
	}
	err = m.Tunnels.Up(ctx, name, content)
	if errors.Is(err, tunnel.ErrAlreadyUp) {
		return nil
	}
	return err
}

func (m *Monitor) switchTo(ctx context.Context, from, to, reason string, now time.Time) {
	if err := m.Tunnels.SetPrimary(ctx, to); err != nil {
		m.logf("резервирование: переключение на %s не удалось: %v", to, err)
		return
	}
	_ = m.Store.Update(func(s *store.State) error { s.Primary = to; return nil })
	m.mu.Lock()
	m.status.LastSwitch, m.status.LastSwitchReason = now, reason
	m.status.Switches++
	m.mu.Unlock()
	m.logf("резервирование: основной туннель %s -> %s (%s)", orDashS(from), to, reason)
	if m.OnSwitch != nil {
		m.OnSwitch(from, to, reason)
	}
}

func rankOf(order []string, name string) int {
	for i, n := range order {
		if n == name {
			return i
		}
	}
	return len(order)
}

func orDash(h *Health) string {
	if h == nil || h.Reason == "" {
		return "туннель не работает"
	}
	return h.Reason
}

func orDashS(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
