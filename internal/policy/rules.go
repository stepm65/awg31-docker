// Package policy — маршрутизация трафика LAN по правилам (этап 3, по мотивам
// a-amneziawg-mikrotik): домены, geosite, geoip и сети решают, идёт ли трафик
// в VPN (PROXY), мимо него (DIRECT) или отбрасывается (DROP).
package policy

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// Action — что делать с совпавшим трафиком.
type Action string

const (
	Proxy  Action = "PROXY"
	Direct Action = "DIRECT"
	Drop   Action = "DROP"
)

// Kind — вид условия.
type Kind string

const (
	KindDomain  Kind = "domain"  // домен и поддомены
	KindFull    Kind = "full"    // только это имя
	KindKeyword Kind = "keyword" // подстрока имени
	KindGeoSite Kind = "geosite"
	KindGeoIP   Kind = "geoip"
	KindCIDR    Kind = "cidr"
)

// Rule — одно правило.
type Rule struct {
	Line   int          `json:"line"`
	Action Action       `json:"action"`
	Target string       `json:"target,omitempty"` // туннель для PROXY@имя
	Kind   Kind         `json:"kind"`
	Value  string       `json:"value"`
	Prefix netip.Prefix `json:"-"`
}

// String — правило в каноническом виде.
func (r Rule) String() string {
	a := string(r.Action)
	if r.Target != "" {
		a += "@" + r.Target
	}
	return a + " " + string(r.Kind) + ":" + r.Value
}

// LineError — ошибка в строке правил.
type LineError struct {
	Line int
	Msg  string
}

func (e LineError) Error() string { return fmt.Sprintf("строка %d: %s", e.Line, e.Msg) }

var (
	reTarget   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,15}$`)
	reCategory = regexp.MustCompile(`^[a-z0-9!@_.:-]{1,64}$`)
	reHost     = regexp.MustCompile(`^[a-z0-9_]([a-z0-9_-]{0,62}\.)*[a-z0-9_-]{0,63}$`)
)

// Limits — ограничения на текст правил.
const (
	MaxRulesBytes = 64 << 10
	MaxRules      = 2000
)

// Parse разбирает правила. Формат строки: «ДЕЙСТВИЕ УСЛОВИЕ [комментарий]»
// или «ДЕЙСТВИЕ,УСЛОВИЕ[,комментарий]»; # — комментарий до конца строки.
// Действия: PROXY (VPN), PROXY@туннель, DIRECT, DROP. Условия: domain:,
// full:, keyword:, geosite:, geoip:, cidr: или просто домен / IP / сеть.
// Ошибки возвращаются все сразу, с номерами строк.
func Parse(text string) ([]Rule, []LineError) {
	if len(text) > MaxRulesBytes {
		return nil, []LineError{{0, fmt.Sprintf("правила длиннее %d КБ", MaxRulesBytes>>10)}}
	}
	var rules []Rule
	var errs []LineError
	for i, raw := range strings.Split(text, "\n") {
		n := i + 1
		line, _, _ := strings.Cut(raw, "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var act, cond string
		if strings.Contains(line, ",") {
			parts := strings.SplitN(line, ",", 3)
			act = strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				cond = strings.TrimSpace(parts[1])
			}
		} else {
			f := strings.Fields(line)
			act = f[0]
			if len(f) > 1 {
				cond = f[1]
			}
		}
		r := Rule{Line: n}
		var err string
		if r.Action, r.Target, err = parseAction(act); err != "" {
			errs = append(errs, LineError{n, err})
			continue
		}
		if cond == "" {
			errs = append(errs, LineError{n, "нет условия (домен, geosite:, geoip: или сеть)"})
			continue
		}
		if r.Kind, r.Value, r.Prefix, err = parseCond(cond); err != "" {
			errs = append(errs, LineError{n, err})
			continue
		}
		rules = append(rules, r)
		if len(rules) > MaxRules {
			return nil, []LineError{{n, fmt.Sprintf("больше %d правил", MaxRules)}}
		}
	}
	return rules, errs
}

func parseAction(s string) (Action, string, string) {
	head, target, hasTarget := strings.Cut(s, "@")
	if !hasTarget {
		head, target, hasTarget = strings.Cut(s, ":")
	}
	switch strings.ToUpper(strings.TrimSpace(head)) {
	case "PROXY", "VPN", "TUNNEL":
		if hasTarget {
			target = strings.TrimSuffix(strings.TrimSpace(target), ".conf")
			if !reTarget.MatchString(target) {
				return "", "", "недопустимое имя туннеля в «" + s + "»"
			}
			return Proxy, target, ""
		}
		return Proxy, "", ""
	case "DIRECT", "BYPASS":
		if hasTarget {
			return "", "", "у DIRECT не бывает туннеля"
		}
		return Direct, "", ""
	case "DROP", "BLOCK", "REJECT":
		if hasTarget {
			return "", "", "у DROP не бывает туннеля"
		}
		return Drop, "", ""
	}
	return "", "", "неизвестное действие «" + s + "»: PROXY, PROXY@туннель, DIRECT или DROP"
}

// NormalizeHost приводит имя к виду для сравнения.
func NormalizeHost(h string) string {
	h = strings.ToLower(strings.TrimSpace(h))
	h = strings.TrimPrefix(h, "*.")
	h = strings.TrimPrefix(h, ".")
	return strings.TrimSuffix(h, ".")
}

func parseCond(c string) (Kind, string, netip.Prefix, string) {
	k, v, has := strings.Cut(c, ":")
	low := strings.ToLower(k)
	if has {
		switch low {
		case "domain", "suffix", "domain-suffix":
			return host(KindDomain, v)
		case "full":
			return host(KindFull, v)
		case "keyword", "domain-keyword":
			v = strings.ToLower(strings.TrimSpace(v))
			if v == "" || len(v) > 64 || strings.ContainsAny(v, " \t") {
				return "", "", netip.Prefix{}, "пустое или слишком длинное ключевое слово"
			}
			return KindKeyword, v, netip.Prefix{}, ""
		case "geosite", "geoip":
			v = strings.ToLower(strings.TrimSpace(v))
			if !reCategory.MatchString(v) {
				return "", "", netip.Prefix{}, "недопустимая категория «" + v + "»"
			}
			return Kind(low), v, netip.Prefix{}, ""
		case "cidr", "ip-cidr", "ip":
			c = v
		}
	}
	if p, ok := prefixOf(c); ok {
		return KindCIDR, p.String(), p, ""
	}
	if p, err := netip.ParsePrefix(strings.TrimSpace(c)); err == nil && p.Bits() == 0 {
		return "", "", netip.Prefix{}, "сеть /0 не нужна: для всего остального есть действие по умолчанию"
	}
	if has && !strings.Contains(c, ".") {
		return "", "", netip.Prefix{}, "неизвестное условие «" + c + "»"
	}
	return host(KindDomain, c)
}

func host(kind Kind, v string) (Kind, string, netip.Prefix, string) {
	v = NormalizeHost(v)
	if v == "" || len(v) > 253 || !reHost.MatchString(v) {
		return "", "", netip.Prefix{}, "недопустимое имя «" + v + "»"
	}
	return kind, v, netip.Prefix{}, ""
}

func prefixOf(s string) (netip.Prefix, bool) {
	s = strings.TrimSpace(s)
	if p, err := netip.ParsePrefix(s); err == nil {
		return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()).Masked(), p.Bits() > 0
	}
	if a, err := netip.ParseAddr(s); err == nil && !a.Is4In6() {
		return netip.PrefixFrom(a, a.BitLen()), true
	}
	return netip.Prefix{}, false
}

// Categories возвращает категории geosite и geoip, которые нужны правилам.
func Categories(rules []Rule) (sites, ips []string) {
	seen := map[string]bool{}
	for _, r := range rules {
		key := string(r.Kind) + ":" + r.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		switch r.Kind {
		case KindGeoSite:
			sites = append(sites, r.Value)
		case KindGeoIP:
			ips = append(ips, r.Value)
		}
	}
	return sites, ips
}
