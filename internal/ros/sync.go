package ros

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Marker — начало комментария у записей, которыми управляет контейнер.
// Чужие записи (без этого комментария) никогда не трогаются.
const Marker = "awg31"

// Entry — желаемая запись address-list.
type Entry struct {
	Comment string
	Timeout time.Duration // 0 — постоянная запись; иначе динамическая (только в памяти роутера)
}

// AddrKey — адрес в виде RouterOS: /32 и /128 без длины префикса.
func AddrKey(p netip.Prefix) string {
	p = p.Masked()
	if p.IsSingleIP() {
		return p.Addr().String()
	}
	return p.String()
}

func parseAddr(s string) (netip.Prefix, bool) {
	if p, err := netip.ParsePrefix(s); err == nil {
		return p.Masked(), true
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return netip.PrefixFrom(a, a.BitLen()), true
	}
	return netip.Prefix{}, false
}

var reROSDur = regexp.MustCompile(`(\d+)([wdhms])`)

// ParseDuration разбирает время RouterOS: «1w2d3h4m5s» или «23:59:59».
func ParseDuration(s string) time.Duration {
	s = strings.TrimSpace(s)
	if strings.Count(s, ":") == 2 {
		var d time.Duration
		days := 0
		if i := strings.Index(s, "d"); i > 0 { // «1d23:59:59»
			days, _ = strconv.Atoi(s[:i])
			s = s[i+1:]
		}
		parts := strings.Split(s, ":")
		for i, unit := range []time.Duration{time.Hour, time.Minute, time.Second} {
			n, _ := strconv.Atoi(parts[i])
			d += time.Duration(n) * unit
		}
		return d + time.Duration(days)*24*time.Hour
	}
	var d time.Duration
	for _, m := range reROSDur.FindAllStringSubmatch(s, -1) {
		n, _ := strconv.Atoi(m[1])
		unit := map[string]time.Duration{"w": 7 * 24 * time.Hour, "d": 24 * time.Hour, "h": time.Hour, "m": time.Minute, "s": time.Second}[m[2]]
		d += time.Duration(n) * unit
	}
	return d
}

// FormatDuration — время в формате RouterOS («1h2m3s»).
func FormatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Second {
		d = time.Second
	}
	h, m, s := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	out := ""
	if h > 0 {
		out += fmt.Sprintf("%dh", h)
	}
	if m > 0 {
		out += fmt.Sprintf("%dm", m)
	}
	if s > 0 || out == "" {
		out += fmt.Sprintf("%ds", s)
	}
	return out
}

// ListPath — путь address-list для семейства адресов.
func ListPath(v6 bool) string {
	if v6 {
		return "/ipv6/firewall/address-list"
	}
	return "/ip/firewall/address-list"
}

// SyncResult — итог синхронизации.
type SyncResult struct {
	Added, Removed, Refreshed, Kept int
}

// SyncAddressList приводит управляемые записи списка list к desired (ключ —
// AddrKey). Записи без комментария Marker не трогаются. Динамическим записям
// продлевается срок, если осталось меньше половины.
func SyncAddressList(ctx context.Context, c *Client, v6 bool, list string, desired map[string]Entry) (SyncResult, error) {
	var res SyncResult
	path := ListPath(v6)
	rows, err := c.Print(ctx, path, map[string]string{"list": list})
	if err != nil {
		return res, err
	}
	have := map[string]Row{}
	for _, r := range rows {
		if !strings.HasPrefix(r["comment"], Marker) {
			continue
		}
		p, ok := parseAddr(r["address"])
		if !ok {
			continue
		}
		key := AddrKey(p)
		if _, dup := have[key]; dup {
			// дубль от прошлых сбоев — убираем
			if err := c.Remove(ctx, path, r[".id"]); err != nil {
				return res, err
			}
			res.Removed++
			continue
		}
		have[key] = r
	}
	for key, r := range have {
		if _, ok := desired[key]; !ok {
			if err := c.Remove(ctx, path, r[".id"]); err != nil {
				return res, err
			}
			res.Removed++
		}
	}
	for key, e := range desired {
		r, ok := have[key]
		if !ok {
			f := map[string]string{"list": list, "address": key, "comment": e.Comment}
			if e.Timeout > 0 {
				f["timeout"] = FormatDuration(e.Timeout)
			}
			if _, err := c.Add(ctx, path, f); err != nil {
				return res, fmt.Errorf("%s: %w", key, err)
			}
			res.Added++
			continue
		}
		if e.Timeout > 0 {
			left := ParseDuration(r["timeout"])
			if r["dynamic"] == "true" && left < e.Timeout/2 {
				if err := c.Set(ctx, path, r[".id"], map[string]string{"timeout": FormatDuration(e.Timeout)}); err != nil {
					return res, err
				}
				res.Refreshed++
				continue
			}
		}
		res.Kept++
	}
	return res, nil
}

// SyncDNSForward приводит управляемые записи /ip/dns/static типа FWD к
// списку доменов: каждый домен (с поддоменами) пересылается на forwardTo.
func SyncDNSForward(ctx context.Context, c *Client, forwardTo string, domains []string) (SyncResult, error) {
	var res SyncResult
	const path = "/ip/dns/static"
	rows, err := c.Print(ctx, path, map[string]string{"type": "FWD"})
	if err != nil {
		return res, err
	}
	want := map[string]bool{}
	for _, d := range domains {
		want[d] = true
	}
	have := map[string]Row{}
	for _, r := range rows {
		if !strings.HasPrefix(r["comment"], Marker) {
			continue
		}
		name := strings.ToLower(r["name"])
		if _, dup := have[name]; dup || !want[name] {
			if err := c.Remove(ctx, path, r[".id"]); err != nil {
				return res, err
			}
			res.Removed++
			continue
		}
		have[name] = r
	}
	for _, d := range domains {
		r, ok := have[d]
		switch {
		case !ok:
			if _, err := c.Add(ctx, path, map[string]string{"name": d, "type": "FWD", "forward-to": forwardTo,
				"match-subdomain": "yes", "comment": Marker + " dns"}); err != nil {
				return res, fmt.Errorf("%s: %w", d, err)
			}
			res.Added++
		case r["forward-to"] != forwardTo:
			if err := c.Set(ctx, path, r[".id"], map[string]string{"forward-to": forwardTo}); err != nil {
				return res, err
			}
			res.Refreshed++
		default:
			res.Kept++
		}
	}
	return res, nil
}

// Cleanup удаляет все управляемые записи (при выключении интеграции).
func Cleanup(ctx context.Context, c *Client, list string) (int, error) {
	n := 0
	for _, v6 := range []bool{false, true} {
		r, err := SyncAddressList(ctx, c, v6, list, nil)
		n += r.Removed
		if err != nil && !v6 {
			return n, err
		}
	}
	r, err := SyncDNSForward(ctx, c, "", nil)
	return n + r.Removed, err
}
