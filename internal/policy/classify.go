package policy

import (
	"net/netip"

	"awg31/internal/geo"
	"awg31/internal/store"
	"strings"
	"time"
)

// Verdict — куда по правилам идёт соединение (для мониторинга и счётчиков).
type Verdict struct {
	Line   int    `json:"line"` // строка правила; 0 — действие по умолчанию
	Action Action `json:"action"`
	Target string `json:"target,omitempty"`
	By     string `json:"by"`
}

// Classify повторяет решение ядра для соединения src -> dst:port: сначала области
// (правила с устройствами и портами) по порядку, затем общая таблица.
func (e *Engine) Classify(src, dst netip.Addr, proto string, dport uint16) Verdict {
	e.init()
	src, dst = src.Unmap(), dst.Unmap()
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.enabled {
		return Verdict{Action: Proxy, By: "default"}
	}
	now := e.Now()
	for _, sc := range e.scopes {
		if !sc.matches(src, proto, dport) {
			continue
		}
		if l, ok := e.learnedScoped[sc.key][dst]; ok && l.expires.After(now) {
			return Verdict{l.line, l.action, l.target, l.by}
		}
		if r, ok := e.bestPrefix(sc.rules, dst); ok {
			return Verdict{r.Line, r.Action, r.Target, string(r.Kind) + ":" + r.Value}
		}
	}
	if l, ok := e.learned[dst]; ok && l.expires.After(now) {
		return Verdict{l.line, l.action, l.target, l.by}
	}
	if r, ok := e.bestPrefix(e.rules, dst); ok {
		return Verdict{r.Line, r.Action, r.Target, string(r.Kind) + ":" + r.Value}
	}
	return Verdict{Action: e.def, By: "default"}
}

// bestPrefix — правило сетей или geoip с самым точным префиксом для адреса.
func (e *Engine) bestPrefix(rules []Rule, a netip.Addr) (Rule, bool) {
	best, bits := Rule{}, -1
	for _, r := range rules {
		switch r.Kind {
		case KindCIDR:
			if r.Prefix.Contains(a) && r.Prefix.Bits() > bits {
				best, bits = r, r.Prefix.Bits()
			}
		case KindGeoIP:
			if c := e.geoIPs(r.Value); c != nil && !c.Reverse {
				for _, p := range c.Prefixes {
					if p.Contains(a) && p.Bits() > bits {
						best, bits = r, p.Bits()
					}
				}
			}
		}
	}
	return best, bits >= 0
}

const maxNames = 20000

type nameEntry struct {
	name    string
	expires time.Time
}

// NoteNames запоминает, какое имя DNS вернуло для адресов (для мониторинга).
func (e *Engine) NoteNames(name string, addrs []netip.Addr, ttl time.Duration) {
	e.init()
	h := NormalizeHost(name)
	if h == "" || len(addrs) == 0 {
		return
	}
	ttl = max(10*time.Minute, min(ttl, LearnMax))
	exp := e.Now().Add(ttl)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.names == nil {
		e.names = map[netip.Addr]nameEntry{}
	}
	if len(e.names) >= maxNames {
		now := e.Now()
		for a, n := range e.names {
			if !n.expires.After(now) {
				delete(e.names, a)
			}
		}
	}
	for _, a := range addrs {
		a = a.Unmap()
		if _, ok := e.names[a]; !ok && len(e.names) >= maxNames {
			continue
		}
		e.names[a] = nameEntry{h, exp}
	}
}

// Name — последнее имя из DNS для адреса или "".
func (e *Engine) Name(a netip.Addr) string {
	e.init()
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n, ok := e.names[a.Unmap()]; ok && n.expires.After(e.Now()) {
		return n.name
	}
	return ""
}

// Label — подпись правила: имя блока «#@ Имя | значок» или само условие.
type Label struct {
	Name string `json:"name"`
	Icon string `json:"icon,omitempty"`
	Head int    `json:"head,omitempty"` // строка заголовка блока (одинакова у всех его строк)
}

// Labels сопоставляет строкам правил подписи блоков так же, как редактор: заголовок
// «#@» действует до пустой строки.
func Labels(text string) map[int]Label {
	out := map[int]Label{}
	cur, curLine := Label{}, 0
	for i, raw := range strings.Split(text, "\n") {
		n, tr := i+1, strings.TrimSpace(raw)
		switch {
		case tr == "":
			cur, curLine = Label{}, 0
		case strings.HasPrefix(tr, "#@"):
			name, icon, _ := strings.Cut(strings.TrimPrefix(tr, "#@"), "|")
			cur, curLine = Label{Name: strings.TrimSpace(name), Icon: strings.TrimSpace(icon)}, n
		case strings.HasPrefix(tr, "#"):
		default:
			if curLine != 0 {
				l := cur
				l.Head = curLine
				out[n] = l
			}
		}
	}
	return out
}

// GeoExtra переводит наборы из настроек в вид пакета geo.
func GeoExtra(p store.Policy) []geo.Extra {
	out := make([]geo.Extra, 0, len(p.GeoExtra))
	for _, e := range p.GeoExtra {
		out = append(out, geo.Extra{Tag: e.Tag, URL: e.URL, Kind: e.Kind})
	}
	return out
}

// TestFor — «куда пойдёт» с учётом устройства, протокола и порта. src может быть
// пустым: тогда правила с устройствами не подходят.
func (e *Engine) TestFor(q string, src netip.Addr, proto string, port uint16) TestResult {
	e.init()
	q = strings.TrimSpace(q)
	if a, err := netip.ParseAddr(q); err == nil {
		v := e.Classify(src, a, proto, port)
		return TestResult{Query: q, Action: v.Action, Target: v.Target, By: v.By, Line: v.Line}
	}
	h := NormalizeHost(q)
	e.mu.RLock()
	scopes, enabled := e.scopes, e.enabled
	e.mu.RUnlock()
	if enabled {
		for _, sc := range scopes {
			if !sc.matches(src.Unmap(), proto, port) {
				continue
			}
			for _, r := range sc.rules {
				if e.matchHost(r, h) {
					return TestResult{Query: h, Action: r.Action, Target: r.Target, By: string(r.Kind) + ":" + r.Value, Line: r.Line}
				}
			}
		}
	}
	act, tgt, by := e.MatchDomain(h)
	line := 0
	e.mu.RLock()
	for _, r := range e.rules {
		if string(r.Kind)+":"+r.Value == by {
			line = r.Line
			break
		}
	}
	e.mu.RUnlock()
	return TestResult{Query: h, Action: act, Target: tgt, By: by, Line: line}
}
