package policy

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"awg31/internal/geo"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Маршрутизация по правилам — таблица Table, в которую смотрит трафик LAN
// раньше таблицы основного туннеля (правило RulePriority < 300):
//
//   - PROXY            -> throw: поиск продолжается в таблице 103, то есть в
//     основном туннеле со всеми его kill switch и резервированием;
//   - PROXY@туннель    -> dev туннель (не поднят: blackhole при kill switch);
//   - DIRECT           -> via <шлюз LAN> — обратно на роутер, мимо VPN;
//   - DROP             -> blackhole;
//   - подключённые сети LAN -> throw (локальный трафик не трогаем).
//
// Для IP-адресов действует самый точный префикс (так устроена таблица
// маршрутов); для доменов — первое совпавшее правило. IP-адреса, которые
// вернул DNS для доменных правил, «выучиваются» как /32 и /128 на время TTL
// (не меньше LearnMin, не больше LearnMax).
const (
	Table        = 210
	RulePriority = 250
	LearnMin     = 15 * time.Minute
	LearnMax     = 24 * time.Hour
	MaxLearned   = 50000
	MaxRoutes    = 200000
)

// Tunnels — то, что нужно от tunnel.Manager.
type Tunnels interface {
	Table(name string) int
	Options() tunnel.Options
}

// Nexthoper — необязательное расширение Tunnels для групп туннелей: маршрут
// «dev wg0» или «nexthop dev wg0 weight 1 nexthop dev wg1 weight 1» для имени
// группы (ok=false — имени нет среди групп или в ней нет исправных участников).
type Nexthoper interface {
	Nexthop(name string, v6 bool) (string, bool)
}

type learned struct {
	action  Action
	target  string
	by      string
	expires time.Time
	line    int // строка правила (для счётчиков)
}

// Stats — счётчики для UI.
type Stats struct {
	Enabled   bool      `json:"enabled"`
	Default   string    `json:"default_action"`
	Rules     int       `json:"rules"`
	Routes    int       `json:"routes"`
	Learned   int       `json:"learned"`
	Queries   uint64    `json:"queries"`
	Proxy     uint64    `json:"proxy"`
	Direct    uint64    `json:"direct"`
	Dropped   uint64    `json:"dropped"`
	Gateway   string    `json:"gateway,omitempty"`
	LastApply time.Time `json:"last_apply,omitempty"`
	LastError string    `json:"last_error,omitempty"`
}

// Engine — правила, выученные адреса и применение их к таблице маршрутов.
type Engine struct {
	Store   *store.Store
	Geo     *geo.Store
	Tunnels Tunnels
	Runner  tunnel.Runner
	Dir     string // для файлов ip -batch (RUN_DIR)
	Logf    func(format string, args ...any)
	Now     func() time.Time

	applyMu sync.Mutex
	mu      sync.RWMutex
	enabled bool
	def     Action
	rules   []Rule // общие правила (без src=, port=, net=)
	all     []Rule // все правила по порядку
	scopes  []*scope
	snotes  []string // заметки о суженных правилах (неизвестные устройства и т. п.)
	learned map[netip.Addr]learned
	// learnedScoped — адреса из DNS для доменных правил областей: ключ области -> адрес
	learnedScoped map[string]map[netip.Addr]learned
	appliedScoped map[int]map[string]string // таблица области -> маршруты
	appliedRules  map[string]bool           // «4 аргументы ip rule» областей
	applied       map[string]string         // "4 1.2.3.0/24" -> спецификация маршрута
	fresh         bool                      // applied совпадает с таблицей
	stats         Stats
	dirty         chan struct{}
	once          sync.Once
	subs          []func()
	force         bool // переложить все маршруты заново (ядро могло их потерять)
}

func (e *Engine) init() {
	e.once.Do(func() {
		e.learned, e.applied = map[netip.Addr]learned{}, map[string]string{}
		e.learnedScoped, e.appliedScoped, e.appliedRules = map[string]map[netip.Addr]learned{}, map[int]map[string]string{}, map[string]bool{}
		e.dirty = make(chan struct{}, 1)
		if e.Now == nil {
			e.Now = time.Now
		}
	})
}

func (e *Engine) logf(f string, a ...any) {
	if e.Logf != nil {
		e.Logf(f, a...)
	}
}

// MarkDirty просит применить правила при ближайшей возможности.
func (e *Engine) MarkDirty() {
	e.init()
	select {
	case e.dirty <- struct{}{}:
	default:
	}
	e.mu.RLock()
	subs := append([]func(){}, e.subs...)
	e.mu.RUnlock()
	for _, fn := range subs {
		fn()
	}
}

// Invalidate просит переложить все маршруты заново: после подъёма или остановки
// туннеля ядро само убирает маршруты через исчезнувший интерфейс, а состав
// правил мог не измениться, и сравнение с прежним планом ничего не заметило бы.
func (e *Engine) Invalidate() {
	e.init()
	e.mu.Lock()
	e.force = true
	e.mu.Unlock()
	e.MarkDirty()
}

// OnChange регистрирует функцию, вызываемую при каждом изменении правил или
// выученных адресов (должна быть быстрой: например, будить свой цикл).
func (e *Engine) OnChange(fn func()) {
	e.init()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subs = append(e.subs, fn)
}

// Reload перечитывает настройки и нужные категории баз.
func (e *Engine) Reload() error {
	e.init()
	p := e.Store.Snapshot().PolicySettings()
	rules, errs := Parse(p.Rules)
	if len(errs) > 0 {
		// сохранённые правила проверены при записи; сюда попадаем, только если
		// state.json правили руками
		e.logf("policy: errors in the rules, applying the valid part: %v", errs[0])
	}
	var geoErr error
	if e.Geo != nil {
		sites, ips := Categories(rules)
		if len(sites)+len(ips) > 0 {
			geoErr = e.Geo.Load(sites, ips)
		}
	}
	devices := map[string]netip.Prefix{}
	for _, d := range e.Store.Snapshot().Devices {
		if p, ok := d.Prefix(); ok {
			devices[d.ID] = p
		}
	}
	scopes, snotes := buildScopes(rules, devices)
	global := make([]Rule, 0, len(rules))
	for _, r := range rules {
		if !r.Scoped() {
			global = append(global, r)
		}
	}
	e.mu.Lock()
	e.enabled = p.Enabled
	e.def = Proxy
	if p.DefaultAction == "direct" {
		e.def = Direct
	}
	e.rules, e.all, e.scopes, e.snotes = global, rules, scopes, snotes
	keep := map[string]bool{}
	for _, sc := range scopes {
		keep[sc.key] = true
	}
	for k := range e.learnedScoped {
		if !keep[k] {
			delete(e.learnedScoped, k) // область исчезла вместе с правилами
		}
	}
	e.mu.Unlock()
	e.MarkDirty()
	return geoErr
}

// Enabled — политика включена.
func (e *Engine) Enabled() bool {
	e.init()
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.enabled
}

// MatchDomain — действие для имени: первое совпавшее доменное правило или
// действие по умолчанию (by = "default").
func (e *Engine) MatchDomain(name string) (Action, string, string) {
	e.init()
	h := NormalizeHost(name)
	e.mu.RLock()
	rules, def := e.rules, e.def
	e.mu.RUnlock()
	if h != "" {
		for _, r := range rules {
			hit := false
			switch r.Kind {
			case KindFull:
				hit = h == r.Value
			case KindDomain:
				hit = h == r.Value || strings.HasSuffix(h, "."+r.Value)
			case KindKeyword:
				hit = strings.Contains(h, r.Value)
			case KindGeoSite:
				hit = e.Geo != nil && e.Geo.MatchSite(r.Value, h)
			}
			if hit {
				return r.Action, r.Target, string(r.Kind) + ":" + r.Value
			}
		}
	}
	return def, "", "default"
}

// MatchIP — действие для адреса по правилам сетей и geoip (самый точный
// префикс, как в таблице маршрутов) и выученным адресам.
func (e *Engine) MatchIP(a netip.Addr) (Action, string, string) {
	e.init()
	a = a.Unmap()
	e.mu.RLock()
	defer e.mu.RUnlock()
	if l, ok := e.learned[a]; ok && l.expires.After(e.Now()) {
		return l.action, l.target, l.by + " (выучен из DNS)"
	}
	best, bestBits := Rule{}, -1
	for _, r := range e.rules {
		switch r.Kind {
		case KindCIDR:
			if r.Prefix.Contains(a) && r.Prefix.Bits() > bestBits {
				best, bestBits = r, r.Prefix.Bits()
			}
		case KindGeoIP:
			if c := e.geoIPs(r.Value); c != nil && !c.Reverse {
				for _, p := range c.Prefixes {
					if p.Contains(a) && p.Bits() > bestBits {
						best, bestBits = r, p.Bits()
					}
				}
			}
		}
	}
	if bestBits >= 0 {
		return best.Action, best.Target, string(best.Kind) + ":" + best.Value
	}
	return e.def, "", "default"
}

func (e *Engine) geoIPs(cat string) *geo.IP {
	if e.Geo == nil {
		return nil
	}
	return e.Geo.IPs(cat)
}

// Learn запоминает адреса из ответа DNS для имени, совпавшего с правилом.
func (e *Engine) Learn(action Action, target, by string, addrs []netip.Addr, ttl time.Duration) {
	e.init()
	if by == "default" || len(addrs) == 0 {
		return
	}
	ttl = max(LearnMin, min(ttl, LearnMax))
	exp := e.Now().Add(ttl)
	changed := false
	e.mu.Lock()
	line := 0
	for _, r := range e.rules {
		if string(r.Kind)+":"+r.Value == by {
			line = r.Line
			break
		}
	}
	for _, a := range addrs {
		a = a.Unmap()
		if !a.IsGlobalUnicast() || a.IsPrivate() {
			continue // адреса LAN и служебные не уводим
		}
		old, ok := e.learned[a]
		if !ok && len(e.learned) >= MaxLearned {
			continue
		}
		if !ok || old.action != action || old.target != target {
			changed = true
		}
		e.learned[a] = learned{action, target, by, exp, line}
	}
	e.mu.Unlock()
	if changed {
		e.MarkDirty()
	}
}

// Note учитывает DNS-запрос в статистике.
func (e *Engine) Note(a Action) {
	e.init()
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stats.Queries++
	switch a {
	case Proxy:
		e.stats.Proxy++
	case Direct:
		e.stats.Direct++
	case Drop:
		e.stats.Dropped++
	}
}

// Stats возвращает счётчики.
func (e *Engine) Stats() Stats {
	e.init()
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := e.stats
	s.Enabled, s.Default, s.Rules, s.Learned, s.Routes = e.enabled, strings.ToLower(string(e.def)), len(e.all), len(e.learned), len(e.applied)
	for _, m := range e.learnedScoped {
		s.Learned += len(m)
	}
	for _, m := range e.appliedScoped {
		s.Routes += len(m)
	}
	return s
}

// LAN — сведения о LAN-интерфейсе для плана.
type LAN struct {
	Iface     string
	GW4, GW6  string
	Connected []netip.Prefix
}

// Plan строит желаемое содержимое таблицы: ключ «4 префикс» или «6 префикс»
// -> спецификация маршрута для «ip route replace … table 210».
func (e *Engine) Plan(lan LAN, killSwitch, ipv6 bool) (map[string]string, []string) {
	e.init()
	e.mu.RLock()
	rules, def := e.rules, e.def
	learnedSnap := make(map[netip.Addr]learned, len(e.learned))
	now := e.Now()
	for a, l := range e.learned {
		if l.expires.After(now) {
			learnedSnap[a] = l
		}
	}
	e.mu.RUnlock()

	routes := map[string]string{}
	var notes []string
	noDirect := map[int]bool{}
	gw := func(p netip.Prefix) (string, int) {
		if p.Addr().Is4() {
			return lan.GW4, 4
		}
		return lan.GW6, 6
	}
	add := func(p netip.Prefix, a Action, target string) {
		if !p.IsValid() || p.Bits() == 0 || len(routes) >= MaxRoutes {
			return
		}
		if p.Addr().Is6() && !ipv6 {
			return
		}
		p = p.Masked()
		g, fam := gw(p)
		key := fmt.Sprintf("%d %s", fam, p)
		if _, ok := routes[key]; ok {
			return // первое правило для этого префикса побеждает
		}
		switch a {
		case Drop:
			routes[key] = "blackhole " + p.String()
		case Direct:
			if g == "" {
				noDirect[fam] = true
				return
			}
			routes[key] = fmt.Sprintf("%s via %s dev %s", p, g, lan.Iface)
		case Proxy:
			nh, isGroup := "", false
			if nx, ok := e.Tunnels.(Nexthoper); ok && target != "" {
				nh, isGroup = nx.Nexthop(target, p.Addr().Is6())
			}
			switch {
			case target == "":
				routes[key] = "throw " + p.String()
			case isGroup:
				routes[key] = p.String() + " " + nh
			case e.Tunnels != nil && e.Tunnels.Table(target) != 0:
				routes[key] = fmt.Sprintf("%s dev %s", p, target)
			case killSwitch:
				routes[key] = "blackhole " + p.String()
			default:
				routes[key] = "throw " + p.String()
			}
		}
	}
	for _, c := range lan.Connected {
		add(c, Proxy, "") // throw: локальные сети не маршрутизируем
	}
	for a, l := range learnedSnap {
		add(netip.PrefixFrom(a, a.BitLen()), l.action, l.target)
	}
	for _, r := range rules {
		switch r.Kind {
		case KindCIDR:
			add(r.Prefix, r.Action, r.Target)
		case KindGeoIP:
			if c := e.geoIPs(r.Value); c != nil && !c.Reverse {
				for _, p := range c.Prefixes {
					add(p, r.Action, r.Target)
				}
			} else if c != nil && c.Reverse {
				notes = append(notes, "geoip:"+r.Value+" — категория «всё, кроме», в таблицу не переносится")
			}
		}
	}
	if def == Direct {
		for _, fam := range []int{4, 6} {
			if fam == 6 && !ipv6 {
				continue
			}
			g := lan.GW4
			if fam == 6 {
				g = lan.GW6
			}
			if g == "" {
				noDirect[fam] = true
				continue
			}
			routes[fmt.Sprintf("%d default", fam)] = fmt.Sprintf("default via %s dev %s", g, lan.Iface)
		}
	}
	for fam := range noDirect {
		notes = append(notes, fmt.Sprintf("нет шлюза IPv%d на %s: DIRECT для IPv%d не применяется", fam, lan.Iface, fam))
	}
	if len(routes) >= MaxRoutes {
		notes = append(notes, fmt.Sprintf("превышен предел %d маршрутов, часть правил не применена", MaxRoutes))
	}
	sort.Strings(notes)
	return routes, notes
}

// Expire удаляет просроченные выученные адреса.
func (e *Engine) Expire() int {
	e.init()
	now := e.Now()
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for a, l := range e.learned {
		if !l.expires.After(now) {
			delete(e.learned, a)
			n++
		}
	}
	for _, m := range e.learnedScoped {
		for a, l := range m {
			if !l.expires.After(now) {
				delete(m, a)
				n++
			}
		}
	}
	return n
}

// detectLAN читает шлюзы и подключённые сети LAN-интерфейса.
func (e *Engine) detectLAN(ctx context.Context, iface string, ipv6 bool) LAN {
	lan := LAN{Iface: iface}
	fams := []string{"-4"}
	if ipv6 {
		fams = append(fams, "-6")
	}
	for _, fam := range fams {
		if out, err := e.Runner.Run(ctx, "ip", "-o", fam, "route", "show", "default", "dev", iface); err == nil {
			f := strings.Fields(string(out))
			for i := 0; i+1 < len(f); i++ {
				if f[i] == "via" {
					if a, err := netip.ParseAddr(f[i+1]); err == nil {
						if fam == "-4" {
							lan.GW4 = a.String()
						} else {
							lan.GW6 = a.String()
						}
					}
					break
				}
			}
		}
		if out, err := e.Runner.Run(ctx, "ip", "-o", fam, "route", "show", "dev", iface, "scope", "link"); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				f := strings.Fields(line)
				if len(f) == 0 {
					continue
				}
				if p, err := netip.ParsePrefix(f[0]); err == nil && !p.Addr().IsLinkLocalUnicast() && !p.Addr().IsMulticast() {
					lan.Connected = append(lan.Connected, p)
				}
			}
		}
	}
	return lan
}

// Apply приводит таблицу Table и правило RulePriority к желаемому состоянию.
func (e *Engine) Apply(ctx context.Context) error {
	e.init()
	e.applyMu.Lock()
	defer e.applyMu.Unlock()
	o := e.Tunnels.Options()
	gateway := o.AutoRoute && o.LanIface != ""
	enabled := e.Enabled() && gateway
	prio, tbl := fmt.Sprint(RulePriority), fmt.Sprint(Table)
	fams := []string{"-4"}
	if o.IPv6 {
		fams = append(fams, "-6")
	}

	if !enabled {
		e.mu.RLock()
		had := len(e.applied) > 0 || !e.fresh
		e.mu.RUnlock()
		if had && gateway {
			for _, fam := range fams {
				_, _ = e.Runner.Run(ctx, "ip", fam, "rule", "del", "priority", prio)
				_, _ = e.Runner.Run(ctx, "ip", fam, "route", "flush", "table", tbl)
				e.dropScopes(ctx, fam)
			}
		}
		e.mu.Lock()
		e.applied, e.fresh = map[string]string{}, true
		e.appliedScoped, e.appliedRules = map[int]map[string]string{}, map[string]bool{}
		e.stats.LastError = ""
		e.mu.Unlock()
		return nil
	}

	lan := e.detectLAN(ctx, o.LanIface, o.IPv6)
	want, notes := e.Plan(lan, o.KillSwitch, o.IPv6)

	e.mu.RLock()
	prev, fresh, force := e.applied, e.fresh, e.force
	e.mu.RUnlock()
	var b4, b6 strings.Builder
	line := func(key, s string) {
		if strings.HasPrefix(key, "6 ") {
			b6.WriteString(s + "\n")
		} else {
			b4.WriteString(s + "\n")
		}
	}
	if !fresh {
		prev = map[string]string{}
		b4.WriteString("route flush table " + tbl + "\n")
		if o.IPv6 {
			b6.WriteString("route flush table " + tbl + "\n")
		}
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for k, spec := range prev {
		if _, ok := want[k]; !ok {
			line(k, "route del "+spec+" table "+tbl)
		}
	}
	for _, k := range keys {
		if force || prev[k] != want[k] {
			line(k, "route replace "+want[k]+" table "+tbl)
		}
	}
	// области: свои таблицы и ip rule
	splans, snotes := e.planScopes(lan, o.KillSwitch, o.IPv6)
	e.mu.RLock()
	prevS, prevR := e.appliedScoped, e.appliedRules
	notes = append(notes, e.snotes...)
	e.mu.RUnlock()
	notes = append(notes, snotes...)
	if !fresh {
		prevS, prevR = map[int]map[string]string{}, map[string]bool{}
		for _, fam := range fams {
			for _, d := range e.cleanScopeRules(ctx, fam) {
				if fam == "-6" {
					b6.WriteString(d + "\n")
				} else {
					b4.WriteString(d + "\n")
				}
			}
		}
		for i := 0; i < MaxScopes; i++ {
			b4.WriteString(fmt.Sprintf("route flush table %d\n", ScopeTableBase+i))
			if o.IPv6 {
				b6.WriteString(fmt.Sprintf("route flush table %d\n", ScopeTableBase+i))
			}
		}
	}
	wantS, wantR := map[int]map[string]string{}, map[string]bool{}
	for _, pl := range splans {
		wantS[pl.table] = pl.routes
		for fam, rs := range pl.rules {
			for _, r := range rs {
				wantR[fmt.Sprintf("%d %s", fam, r)] = true
			}
		}
	}
	st := func(t int) string { return " table " + strconv.Itoa(t) }
	for t, routes := range prevS {
		if _, ok := wantS[t]; !ok {
			for k, spec := range routes {
				line(k, "route del "+spec+st(t))
			}
		}
	}
	tables := make([]int, 0, len(wantS))
	for t := range wantS {
		tables = append(tables, t)
	}
	sort.Ints(tables)
	for _, t := range tables {
		w, p := wantS[t], prevS[t]
		for k, spec := range p {
			if _, ok := w[k]; !ok {
				line(k, "route del "+spec+st(t))
			}
		}
		ks := make([]string, 0, len(w))
		for k := range w {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			if force || p[k] != w[k] {
				line(k, "route replace "+w[k]+st(t))
			}
		}
	}
	// ip rule: сначала удалить ненужные, потом добавить новые
	rkeys := make([]string, 0, len(wantR))
	for k := range wantR {
		rkeys = append(rkeys, k)
	}
	sort.Strings(rkeys)
	for k := range prevR {
		if !wantR[k] {
			fam, args, _ := strings.Cut(k, " ")
			line(fam+" ", "rule del "+args)
		}
	}
	for _, k := range rkeys {
		if !prevR[k] {
			fam, args, _ := strings.Cut(k, " ")
			line(fam+" ", "rule add "+args)
		}
	}
	var errs []string
	for i, b := range []*strings.Builder{&b4, &b6} {
		if b.Len() == 0 {
			continue
		}
		if err := e.batch(ctx, fams[min(i, len(fams)-1)], b.String()); err != nil {
			errs = append(errs, err.Error())
		}
	}
	for _, fam := range fams {
		_, _ = e.Runner.Run(ctx, "ip", fam, "rule", "del", "priority", prio)
		if _, err := e.Runner.Run(ctx, "ip", fam, "rule", "add", "priority", prio, "from", "all", "iif", o.LanIface, "lookup", tbl); err != nil && fam == "-4" {
			errs = append(errs, "ip rule: "+err.Error())
		}
	}
	// DIRECT уходит обратно на роутер: подмена адреса, чтобы ответ вернулся
	// через контейнер и роутер не видел «чужой» поток (как в исходнике)
	if _, err := e.Runner.Run(ctx, "iptables", "-t", "nat", "-C", "POSTROUTING", "-o", o.LanIface, "-j", "MASQUERADE"); err != nil {
		if _, err := e.Runner.Run(ctx, "iptables", "-t", "nat", "-A", "POSTROUTING", "-o", o.LanIface, "-j", "MASQUERADE"); err != nil {
			errs = append(errs, "MASQUERADE: "+err.Error())
		}
	}
	errs = append(errs, notes...)

	e.mu.Lock()
	defer e.mu.Unlock()
	e.applied = want
	e.appliedScoped, e.appliedRules = wantS, wantR
	e.force = false
	e.fresh = len(errs) == len(notes) // ошибка ip -batch — в следующий раз полная перезаливка
	e.stats.Gateway = lan.GW4
	e.stats.LastApply = e.Now()
	e.stats.LastError = tunnel.Redact(strings.Join(errs, "; "))
	if len(errs) > len(notes) {
		return fmt.Errorf("%s", e.stats.LastError)
	}
	return nil
}

func (e *Engine) batch(ctx context.Context, fam, script string) error {
	dir := e.Dir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "policy-*.batch")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(script); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if _, err := e.Runner.Run(ctx, "ip", fam, "-force", "-batch", filepath.Clean(f.Name())); err != nil {
		return fmt.Errorf("ip -batch: %w", err)
	}
	return nil
}

// Run применяет правила при изменениях и раз в минуту (истечение выученных
// адресов, поднятые и опущенные туннели для PROXY@туннель).
func (e *Engine) Run(ctx context.Context) {
	e.init()
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.dirty:
			// несколько изменений подряд (поток DNS-ответов) — одно применение
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		case <-t.C:
			e.Expire()
		}
		if err := e.Apply(ctx); err != nil {
			e.logf("policy: %v", err)
		}
	}
}

// TestResult — ответ на «куда пойдёт этот адрес или имя».
type TestResult struct {
	Query  string `json:"query"`
	Action Action `json:"action"`
	Target string `json:"target,omitempty"`
	By     string `json:"matched_by"`
}

// Test проверяет имя или IP по правилам.
func (e *Engine) Test(q string) TestResult {
	q = strings.TrimSpace(q)
	if a, err := netip.ParseAddr(q); err == nil {
		act, tgt, by := e.MatchIP(a)
		return TestResult{q, act, tgt, by}
	}
	act, tgt, by := e.MatchDomain(q)
	return TestResult{NormalizeHost(q), act, tgt, by}
}

// LearnedAddr — выученный адрес с действием PROXY.
type LearnedAddr struct {
	Addr    netip.Addr
	Expires time.Time
	By      string
}

// Export — то, что нужно передать роутеру: сети PROXY (из правил и geoip),
// выученные адреса PROXY и имена доменных правил.
type Export struct {
	Prefixes []netip.Prefix
	Learned  []LearnedAddr
	Domains  []string
	Dropped  int // сколько сетей не вошло из-за предела
}

// Export собирает данные для роутера. limit — предел числа сетей, domainLimit —
// имён (вместе с категориями geosite).
func (e *Engine) Export(limit, domainLimit int) Export {
	e.init()
	e.mu.RLock()
	rules, enabled := e.all, e.enabled
	now := e.Now()
	var ex Export
	for a, l := range e.learned {
		if l.action == Proxy && l.expires.After(now) {
			ex.Learned = append(ex.Learned, LearnedAddr{a, l.expires, l.by})
		}
	}
	e.mu.RUnlock()
	if !enabled {
		return Export{}
	}
	seen := map[netip.Prefix]bool{}
	addP := func(p netip.Prefix) {
		p = p.Masked()
		if p.Bits() == 0 || seen[p] {
			return
		}
		if limit > 0 && len(ex.Prefixes) >= limit {
			ex.Dropped++
			return
		}
		seen[p] = true
		ex.Prefixes = append(ex.Prefixes, p)
	}
	names := map[string]bool{}
	addD := func(d string) {
		if d != "" && !names[d] && (domainLimit <= 0 || len(names) < domainLimit) {
			names[d] = true
		}
	}
	for _, r := range rules {
		switch r.Kind {
		case KindCIDR:
			if r.Action == Proxy {
				addP(r.Prefix)
			}
		case KindGeoIP:
			if c := e.geoIPs(r.Value); r.Action == Proxy && c != nil && !c.Reverse {
				for _, p := range c.Prefixes {
					addP(p)
				}
			}
		case KindDomain, KindFull:
			addD(r.Value)
		case KindGeoSite:
			if e.Geo != nil {
				for _, d := range e.Geo.SiteDomains(r.Value, domainLimit) {
					addD(d)
				}
			}
		}
	}
	for d := range names {
		ex.Domains = append(ex.Domains, d)
	}
	sort.Strings(ex.Domains)
	sort.Slice(ex.Learned, func(i, j int) bool { return ex.Learned[i].Addr.Less(ex.Learned[j].Addr) })
	return ex
}
