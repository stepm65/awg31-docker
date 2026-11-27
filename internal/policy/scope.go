package policy

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Правила, суженные устройствами, портами или протоколом (src=, port=, net=).
//
// Правила с одинаковым сужением образуют «область»: у неё своя таблица маршрутов
// (ScopeTableBase+i) и свои ip rule с приоритетом ScopePrioBase+i — раньше общего
// правила RulePriority (250). Поэтому суженное правило побеждает общее для тех же
// адресов, а если адрес в таблице области не нашёлся, поиск идёт дальше: к
// следующим областям и к общей таблице 210.
//
// PROXY без туннеля в области ведёт прямо в основной туннель или группу: «throw»,
// как в таблице 210, тут не годится — общий поиск мог бы отправить адрес мимо VPN.
const (
	ScopeTableBase = 300
	ScopePrioBase  = 200
	MaxScopes      = 40
	MaxScopeRules  = 64 // ip rule на одну область (устройства × протоколы × порты)
)

// Primarier — необязательное расширение Tunnels: основной туннель или группа.
type Primarier interface {
	Primary() string
}

type scope struct {
	key   string         // каноническое сужение: src=… port=… net=…
	srcs  []netip.Prefix // пусто — любой источник
	ports []PortRange
	nets  []string // tcp, udp; пусто при пустых портах — любой протокол
	rules []Rule   // правила области по порядку
	table int
	prio  int
}

// matches сообщает, подходит ли соединение под сужение области.
func (sc *scope) matches(src netip.Addr, proto string, dport uint16) bool {
	if len(sc.srcs) > 0 {
		ok := false
		for _, p := range sc.srcs {
			if p.Contains(src) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	if len(sc.nets) > 0 {
		ok := false
		for _, n := range sc.nets {
			if n == proto {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	if len(sc.ports) > 0 {
		for _, p := range sc.ports {
			if dport >= p.Lo && dport <= p.Hi {
				return true
			}
		}
		return false
	}
	return true
}

// ipRules — аргументы «ip rule add» области для семейства fam (4 или 6).
func (sc *scope) ipRules(fam int, lanIface string) []string {
	var srcs []string
	if len(sc.srcs) == 0 {
		srcs = []string{"all"}
	} else {
		for _, p := range sc.srcs {
			if (fam == 4) == p.Addr().Is4() {
				srcs = append(srcs, p.String())
			}
		}
		if len(srcs) == 0 {
			return nil // устройства области — только другого семейства
		}
	}
	nets := sc.nets
	if len(nets) == 0 && len(sc.ports) > 0 {
		nets = []string{"tcp", "udp"} // порт без протокола: оба
	}
	type pp struct{ proto, port string }
	var combos []pp
	switch {
	case len(nets) == 0:
		combos = []pp{{}}
	case len(sc.ports) == 0:
		for _, n := range nets {
			combos = append(combos, pp{n, ""})
		}
	default:
		for _, n := range nets {
			for _, p := range sc.ports {
				combos = append(combos, pp{n, p.String()})
			}
		}
	}
	var out []string
	for _, s := range srcs {
		for _, c := range combos {
			r := fmt.Sprintf("priority %d from %s iif %s", sc.prio, s, lanIface)
			if c.proto != "" {
				r += " ipproto " + c.proto
			}
			if c.port != "" {
				r += " dport " + c.port
			}
			out = append(out, r+" lookup "+strconv.Itoa(sc.table))
			if len(out) >= MaxScopeRules {
				return out
			}
		}
	}
	return out
}

// buildScopes собирает области из суженных правил; devices — адреса устройств по
// идентификатору. Ссылки на неизвестные устройства пропускаются с заметкой.
func buildScopes(rules []Rule, devices map[string]netip.Prefix) ([]*scope, []string) {
	var out []*scope
	var notes []string
	byKey := map[string]*scope{}
	for _, r := range rules {
		if !r.Scoped() {
			continue
		}
		var srcs []netip.Prefix
		missing := 0
		for _, s := range r.Src {
			if id, ok := strings.CutPrefix(s, "@"); ok {
				if p, ok := devices[id]; ok {
					srcs = append(srcs, p)
				} else {
					missing++
					notes = append(notes, fmt.Sprintf("строка %d: устройства @%s нет в справочнике", r.Line, id))
				}
				continue
			}
			if p, ok := prefixOf(s); ok {
				srcs = append(srcs, p)
			}
		}
		if len(r.Src) > 0 && len(srcs) == 0 {
			// все устройства правила неизвестны: правило не должно стать общим
			notes = append(notes, fmt.Sprintf("строка %d: ни одного известного устройства, правило не применяется", r.Line))
			continue
		}
		_ = missing
		sort.Slice(srcs, func(i, j int) bool { return srcs[i].String() < srcs[j].String() })
		srcs = dedupPrefixes(srcs)
		ks := make([]string, len(srcs))
		for i, p := range srcs {
			ks[i] = p.String()
		}
		probe := Rule{Ports: r.Ports, Net: r.Net}
		key := "src=" + strings.Join(ks, ",") + " " + probe.Scope()
		sc := byKey[key]
		if sc == nil {
			if len(out) >= MaxScopes {
				notes = append(notes, fmt.Sprintf("строка %d: больше %d разных сужений (устройства, порты), правило не применяется", r.Line, MaxScopes))
				continue
			}
			sc = &scope{key: key, srcs: srcs, ports: r.Ports, nets: r.Net, table: ScopeTableBase + len(out), prio: ScopePrioBase + len(out)}
			byKey[key] = sc
			out = append(out, sc)
		}
		sc.rules = append(sc.rules, r)
	}
	return out, notes
}

func dedupPrefixes(in []netip.Prefix) []netip.Prefix {
	out := in[:0]
	for i, p := range in {
		if i == 0 || p != in[i-1] {
			out = append(out, p)
		}
	}
	return out
}

// matchHost — подходит ли имя под доменное условие правила.
func (e *Engine) matchHost(r Rule, h string) bool {
	switch r.Kind {
	case KindFull:
		return h == r.Value
	case KindDomain:
		return h == r.Value || strings.HasSuffix(h, "."+r.Value)
	case KindKeyword:
		return strings.Contains(h, r.Value)
	case KindGeoSite:
		return e.Geo != nil && e.Geo.MatchSite(r.Value, h)
	}
	return false
}

// LearnScoped запоминает адреса из ответа DNS для доменных правил областей: в
// каждой области — по первому совпавшему правилу. Кто спрашивал, не важно: роутер
// часто спрашивает DNS сам, а сужение по устройству делают ip rule области.
func (e *Engine) LearnScoped(name string, addrs []netip.Addr, ttl time.Duration) {
	e.init()
	h := NormalizeHost(name)
	e.mu.RLock()
	scopes := e.scopes
	e.mu.RUnlock()
	if h == "" || len(addrs) == 0 || len(scopes) == 0 {
		return
	}
	ttl = max(LearnMin, min(ttl, LearnMax))
	exp := e.Now().Add(ttl)
	changed := false
	for _, sc := range scopes {
		var hit *Rule
		for i := range sc.rules {
			if e.matchHost(sc.rules[i], h) {
				hit = &sc.rules[i]
				break
			}
		}
		if hit == nil {
			continue
		}
		e.mu.Lock()
		m := e.learnedScoped[sc.key]
		if m == nil {
			m = map[netip.Addr]learned{}
			e.learnedScoped[sc.key] = m
		}
		for _, a := range addrs {
			a = a.Unmap()
			if !a.IsGlobalUnicast() || a.IsPrivate() {
				continue
			}
			old, ok := m[a]
			if !ok && len(m) >= MaxLearned/4 {
				continue
			}
			if !ok || old.action != hit.Action || old.target != hit.Target {
				changed = true
			}
			m[a] = learned{hit.Action, hit.Target, string(hit.Kind) + ":" + hit.Value, exp, hit.Line}
		}
		e.mu.Unlock()
	}
	if changed {
		e.MarkDirty()
	}
}

// scopePlan — содержимое таблицы и ip rule одной области.
type scopePlan struct {
	table  int
	routes map[string]string // как у Plan: «4 префикс» -> спецификация
	rules  map[int][]string  // семейство -> аргументы ip rule
}

// planScopes строит таблицы областей.
func (e *Engine) planScopes(lan LAN, killSwitch, ipv6 bool) ([]scopePlan, []string) {
	e.mu.RLock()
	scopes := e.scopes
	now := e.Now()
	snap := map[string]map[netip.Addr]learned{}
	for k, m := range e.learnedScoped {
		c := map[netip.Addr]learned{}
		for a, l := range m {
			if l.expires.After(now) {
				c[a] = l
			}
		}
		snap[k] = c
	}
	e.mu.RUnlock()
	primary := ""
	if p, ok := e.Tunnels.(Primarier); ok {
		primary = p.Primary()
	}
	var plans []scopePlan
	var notes []string
	for _, sc := range scopes {
		routes := map[string]string{}
		add := func(p netip.Prefix, a Action, target string) {
			if !p.IsValid() || p.Bits() == 0 || len(routes) >= MaxRoutes/4 {
				return
			}
			if p.Addr().Is6() && !ipv6 {
				return
			}
			p = p.Masked()
			fam, g := 4, lan.GW4
			if p.Addr().Is6() {
				fam, g = 6, lan.GW6
			}
			key := fmt.Sprintf("%d %s", fam, p)
			if _, ok := routes[key]; ok {
				return
			}
			switch a {
			case Drop:
				routes[key] = "blackhole " + p.String()
			case Direct:
				if g != "" {
					routes[key] = fmt.Sprintf("%s via %s dev %s", p, g, lan.Iface)
				}
			case Proxy:
				t := target
				if t == "" {
					t = primary
				}
				nh, isGroup := "", false
				if nx, ok := e.Tunnels.(Nexthoper); ok && t != "" {
					nh, isGroup = nx.Nexthop(t, p.Addr().Is6())
				}
				switch {
				case isGroup:
					routes[key] = p.String() + " " + nh
				case t != "" && e.Tunnels.Table(t) != 0:
					routes[key] = fmt.Sprintf("%s dev %s", p, t)
				case killSwitch:
					routes[key] = "blackhole " + p.String()
				default:
					// без туннеля и без kill switch — дальше по общим правилам
					routes[key] = "throw " + p.String()
				}
			}
		}
		for _, c := range lan.Connected {
			if c.Addr().Is4() || ipv6 {
				routes[fmt.Sprintf("%d %s", famOf(c), c.Masked())] = "throw " + c.Masked().String() // локальные сети не трогаем
			}
		}
		for a, l := range snap[sc.key] {
			add(netip.PrefixFrom(a, a.BitLen()), l.action, l.target)
		}
		for _, r := range sc.rules {
			switch r.Kind {
			case KindCIDR:
				add(r.Prefix, r.Action, r.Target)
			case KindGeoIP:
				if c := e.geoIPs(r.Value); c != nil && !c.Reverse {
					for _, p := range c.Prefixes {
						add(p, r.Action, r.Target)
					}
				}
			}
		}
		pl := scopePlan{table: sc.table, routes: routes, rules: map[int][]string{}}
		for _, fam := range []int{4, 6} {
			if fam == 6 && !ipv6 {
				continue
			}
			pl.rules[fam] = sc.ipRules(fam, lan.Iface)
		}
		plans = append(plans, pl)
	}
	return plans, notes
}

func famOf(p netip.Prefix) int {
	if p.Addr().Is4() {
		return 4
	}
	return 6
}

// cleanScopeRules убирает все ip rule областей (приоритеты ScopePrioBase…) —
// при первом применении после запуска, когда прежний набор неизвестен.
func (e *Engine) cleanScopeRules(ctx context.Context, fam string) []string {
	out, err := e.Runner.Run(ctx, "ip", fam, "rule", "show")
	if err != nil {
		return nil
	}
	var del []string
	for _, line := range strings.Split(string(out), "\n") {
		prio, rest, ok := strings.Cut(strings.TrimSpace(line), ":")
		n, err := strconv.Atoi(prio)
		if !ok || err != nil || n < ScopePrioBase || n >= ScopePrioBase+MaxScopes {
			continue
		}
		del = append(del, "rule del priority "+prio+" "+strings.TrimSpace(rest))
	}
	return del
}

// dropScopes убирает таблицы и ip rule областей (политика выключена).
func (e *Engine) dropScopes(ctx context.Context, fam string) {
	for _, d := range e.cleanScopeRules(ctx, fam) {
		_, _ = e.Runner.Run(ctx, "ip", append([]string{fam}, strings.Fields(d)...)...)
	}
	for i := 0; i < MaxScopes; i++ {
		_, _ = e.Runner.Run(ctx, "ip", fam, "route", "flush", "table", strconv.Itoa(ScopeTableBase+i))
	}
}
