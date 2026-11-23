package policy

import (
	"context"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awg31/internal/geo"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func TestParseRules(t *testing.T) {
	rules, errs := Parse(`# комментарий
PROXY geosite:YouTube
DIRECT, geoip:ru, российские сети
PROXY@awg1 domain:*.Example.COM.
DROP keyword:ads
direct 192.168.50.0/24
block 2001:db8::1
VPN full:api.example.org # хвост
tunnel youtube.com
`)
	if len(errs) != 0 {
		t.Fatalf("ошибки: %v", errs)
	}
	got := []string{}
	for _, r := range rules {
		got = append(got, r.String())
	}
	want := []string{"PROXY geosite:youtube", "DIRECT geoip:ru", "PROXY@awg1 domain:example.com", "DROP keyword:ads",
		"DIRECT cidr:192.168.50.0/24", "DROP cidr:2001:db8::1/128", "PROXY full:api.example.org", "PROXY domain:youtube.com"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("\nполучено %v\nожидалось %v", got, want)
	}
	_, errs = Parse("PROXY\nFORWARD x.com\nPROXY@bad;name x.com\nDIRECT@awg1 x.com\nPROXY geosite:a/b\nPROXY bad_host!\nPROXY foo:bar\nPROXY 0.0.0.0/0\n")
	if len(errs) != 8 {
		t.Fatalf("ожидалось 8 ошибок, получено %d: %v", len(errs), errs)
	}
	if errs[0].Line != 1 || errs[2].Line != 3 {
		t.Fatalf("номера строк: %v", errs)
	}
	if _, errs := Parse(strings.Repeat("x", MaxRulesBytes+1)); len(errs) != 1 {
		t.Fatal("предел размера")
	}
	if _, errs := Parse(strings.Repeat("PROXY a.com\n", MaxRules+1)); len(errs) != 1 {
		t.Fatal("предел числа правил")
	}
	sites, ips := Categories(rules)
	if strings.Join(sites, ",") != "youtube" || strings.Join(ips, ",") != "ru" {
		t.Fatalf("категории: %v %v", sites, ips)
	}
}

type fakeTunnels struct {
	tables map[string]int
	opts   tunnel.Options
}

func (f *fakeTunnels) Table(n string) int      { return f.tables[n] }
func (f *fakeTunnels) Options() tunnel.Options { return f.opts }

type rec struct {
	calls   []string
	batches []string
}

func (r *rec) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	line := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, line)
	if len(args) >= 2 && args[len(args)-2] == "-batch" {
		b, _ := os.ReadFile(args[len(args)-1])
		r.batches = append(r.batches, string(b))
	}
	switch line {
	case "ip -o -4 route show default dev eth0":
		return []byte("default via 192.168.253.1 dev eth0 proto static\n"), nil
	case "ip -o -4 route show dev eth0 scope link":
		return []byte("192.168.253.0/30 proto kernel scope link src 192.168.253.2\n"), nil
	}
	return nil, nil
}

func env(t *testing.T, rules, def string) (*Engine, *rec, *fakeTunnels, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := store.DefaultPolicy()
	p.Enabled, p.Rules, p.DefaultAction = true, rules, def
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	// базы geo: текстовые списки
	gdir := filepath.Join(dir, "geo")
	_ = os.MkdirAll(gdir, 0o700)
	_ = os.WriteFile(filepath.Join(gdir, "geosite.dat"), []byte("youtube.com\nfull:youtu.be\n"), 0o600)
	_ = os.WriteFile(filepath.Join(gdir, "geoip.dat"), []byte("5.8.0.0/16\n77.88.0.0/18\n"), 0o600)
	ft := &fakeTunnels{tables: map[string]int{"awg0": 110}, opts: tunnel.Options{AutoRoute: true, LanIface: "eth0", KillSwitch: true}}
	r := &rec{}
	e := &Engine{Store: st, Geo: &geo.Store{Dir: gdir}, Tunnels: ft, Runner: r, Dir: t.TempDir()}
	if err := e.Reload(); err != nil {
		t.Fatal(err)
	}
	return e, r, ft, st
}

func TestMatching(t *testing.T) {
	e, _, _, _ := env(t, "DIRECT full:direct.youtube.com\nPROXY geosite:youtube\nDIRECT geoip:ru\nPROXY 77.88.8.0/24\nDROP keyword:tracker\nPROXY@awg1 example.org\n", "direct")
	for q, want := range map[string]string{
		"direct.youtube.com": "DIRECT full:direct.youtube.com",
		"www.youtube.com":    "PROXY geosite:youtube",
		"youtu.be":           "PROXY geosite:youtube",
		"a.tracker.net":      "DROP keyword:tracker",
		"cdn.example.org":    "PROXY@awg1 domain:example.org",
		"other.net":          "DIRECT default",
		"5.8.1.1":            "DIRECT geoip:ru",
		"77.88.8.8":          "PROXY cidr:77.88.8.0/24", // точнее, чем geoip 77.88.0.0/18
		"77.88.9.9":          "DIRECT geoip:ru",
		"1.1.1.1":            "DIRECT default",
	} {
		r := e.Test(q)
		got := string(r.Action)
		if r.Target != "" {
			got += "@" + r.Target
		}
		got += " " + r.By
		if got != want {
			t.Errorf("%s: %s, ожидалось %s", q, got, want)
		}
	}
}

func TestLearnFromDNS(t *testing.T) {
	e, _, _, _ := env(t, "PROXY geosite:youtube\n", "direct")
	now := time.Unix(1_800_000_000, 0)
	e.Now = func() time.Time { return now }
	addrs := []netip.Addr{netip.MustParseAddr("142.250.1.1"), netip.MustParseAddr("192.168.1.10"), netip.MustParseAddr("2a00:1450::1")}
	e.Learn(Proxy, "", "default", addrs, time.Minute)
	if e.Stats().Learned != 0 {
		t.Fatal("ответы для правила по умолчанию не выучиваются")
	}
	e.Learn(Proxy, "", "geosite:youtube", addrs, 30*time.Second)
	if e.Stats().Learned != 2 {
		t.Fatalf("частные адреса не выучиваются: %d", e.Stats().Learned)
	}
	if r := e.Test("142.250.1.1"); r.Action != Proxy || !strings.Contains(r.By, "DNS") {
		t.Fatalf("выученный адрес: %+v", r)
	}
	now = now.Add(LearnMin - time.Second) // TTL 30 с поднят до LearnMin
	if e.Expire() != 0 {
		t.Fatal("раньше LearnMin не истекает")
	}
	now = now.Add(2 * time.Second)
	if e.Expire() != 2 || e.Test("142.250.1.1").Action != Direct {
		t.Fatal("после истечения адрес забывается")
	}
}

func TestPlanRoutes(t *testing.T) {
	e, _, ft, _ := env(t, "DROP 10.9.0.0/16\nPROXY@awg0 203.0.113.0/24\nPROXY@awg9 198.51.100.0/24\nPROXY 2001:db8::/32\nDIRECT geoip:ru\n", "direct")
	lan := LAN{Iface: "eth0", GW4: "192.168.253.1", Connected: []netip.Prefix{netip.MustParsePrefix("192.168.253.0/30")}}
	routes, notes := e.Plan(lan, true, true)
	for key, want := range map[string]string{
		"4 192.168.253.0/30": "throw 192.168.253.0/30",
		"4 10.9.0.0/16":      "blackhole 10.9.0.0/16",
		"4 203.0.113.0/24":   "203.0.113.0/24 dev awg0",
		"4 198.51.100.0/24":  "blackhole 198.51.100.0/24", // awg9 не поднят + kill switch
		"6 2001:db8::/32":    "throw 2001:db8::/32",
		"4 5.8.0.0/16":       "5.8.0.0/16 via 192.168.253.1 dev eth0",
		"4 default":          "default via 192.168.253.1 dev eth0",
	} {
		if routes[key] != want {
			t.Errorf("%s: %q, ожидалось %q", key, routes[key], want)
		}
	}
	if _, ok := routes["6 default"]; ok || len(notes) != 1 || !strings.Contains(notes[0], "IPv6") {
		t.Fatalf("без IPv6-шлюза DIRECT для IPv6 не применяется: %v", notes)
	}
	ft.opts.KillSwitch = false
	routes, _ = e.Plan(lan, false, false)
	if routes["4 198.51.100.0/24"] != "throw 198.51.100.0/24" {
		t.Fatal("без kill switch PROXY@неподнятый уходит в основной туннель")
	}
	if _, ok := routes["6 2001:db8::/32"]; ok {
		t.Fatal("без IPv6 IPv6-маршрутов нет")
	}
}

func TestApplyDiffAndDisable(t *testing.T) {
	e, r, _, st := env(t, "PROXY 203.0.113.0/24\nDIRECT 198.51.100.0/24\n", "proxy")
	ctx := context.Background()
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if len(r.batches) != 1 || !strings.HasPrefix(r.batches[0], "route flush table 210\n") ||
		!strings.Contains(r.batches[0], "route replace throw 203.0.113.0/24 table 210") ||
		!strings.Contains(r.batches[0], "route replace 198.51.100.0/24 via 192.168.253.1 dev eth0 table 210") {
		t.Fatalf("первое применение — полная заливка:\n%v", r.batches)
	}
	all := strings.Join(r.calls, "\n")
	if !strings.Contains(all, "ip -4 rule add priority 250 from all iif eth0 lookup 210") || !strings.Contains(all, "iptables -t nat -C POSTROUTING -o eth0 -j MASQUERADE") {
		t.Fatalf("правило и MASQUERADE:\n%s", all)
	}
	// изменение — только разница
	p := st.Snapshot().PolicySettings()
	p.Rules = "PROXY 203.0.113.0/24\nDROP 192.0.2.0/24\n"
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	_ = e.Reload()
	r.batches = nil
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b := r.batches[0]
	if strings.Contains(b, "flush") || strings.Contains(b, "203.0.113.0/24") ||
		!strings.Contains(b, "route del 198.51.100.0/24 via 192.168.253.1 dev eth0 table 210") || !strings.Contains(b, "route replace blackhole 192.0.2.0/24 table 210") {
		t.Fatalf("второе применение — только изменения:\n%s", b)
	}
	// выключение — правило и таблица убираются
	p.Enabled = false
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	_ = e.Reload()
	r.calls = nil
	_ = e.Apply(ctx)
	all = strings.Join(r.calls, "\n")
	if !strings.Contains(all, "ip -4 rule del priority 250") || !strings.Contains(all, "ip -4 route flush table 210") {
		t.Fatalf("выключение:\n%s", all)
	}
}

func TestApplyNeedsGateway(t *testing.T) {
	e, r, ft, _ := env(t, "PROXY 203.0.113.0/24\n", "proxy")
	ft.opts.LanIface = ""
	_ = e.Apply(context.Background())
	if len(r.batches) != 0 {
		t.Fatal("без режима шлюза таблица не заполняется")
	}
}

// groupTunnels — fakeTunnels с группами туннелей (policy.Nexthoper).
type groupTunnels struct {
	*fakeTunnels
	hops map[string]string // группа -> маршрут IPv4; "" — исправных участников нет
	v6   string
}

func (g *groupTunnels) Nexthop(n string, v6 bool) (string, bool) {
	s, ok := g.hops[n]
	if v6 {
		s = g.v6
	}
	return s, ok && s != ""
}

func TestPlanProxyToGroup(t *testing.T) {
	e, _, ft, _ := env(t, "PROXY@pool 203.0.113.0/24\nPROXY@pool 2001:db8::/32\nPROXY@empty 198.51.100.0/24\nPROXY@awg0 192.0.2.0/24\n", "proxy")
	gt := &groupTunnels{fakeTunnels: ft, hops: map[string]string{
		"pool": "nexthop dev awg0 weight 2 nexthop dev awg1 weight 1", "empty": ""}, v6: "dev awg0"}
	e.Tunnels = gt
	lan := LAN{Iface: "eth0", GW4: "192.168.253.1"}
	routes, _ := e.Plan(lan, true, true)
	for key, want := range map[string]string{
		"4 203.0.113.0/24":  "203.0.113.0/24 nexthop dev awg0 weight 2 nexthop dev awg1 weight 1",
		"6 2001:db8::/32":   "2001:db8::/32 dev awg0",    // IPv6 — через одного участника
		"4 198.51.100.0/24": "blackhole 198.51.100.0/24", // в группе нет исправных + kill switch
		"4 192.0.2.0/24":    "192.0.2.0/24 dev awg0",     // обычный туннель работает как раньше
	} {
		if routes[key] != want {
			t.Errorf("%s: %q, ожидалось %q", key, routes[key], want)
		}
	}
	routes, _ = e.Plan(lan, false, true)
	if routes["4 198.51.100.0/24"] != "throw 198.51.100.0/24" {
		t.Fatalf("без kill switch пустая группа уходит в основной туннель: %q", routes["4 198.51.100.0/24"])
	}
	// выученные адреса домена с правилом PROXY@группа тоже идут через группу
	e.Learn(Proxy, "pool", "domain:example.com", []netip.Addr{netip.MustParseAddr("198.18.5.5")}, time.Hour)
	routes, _ = e.Plan(lan, true, true)
	if routes["4 198.18.5.5/32"] != "198.18.5.5/32 nexthop dev awg0 weight 2 nexthop dev awg1 weight 1" {
		t.Fatalf("выученный адрес: %q", routes["4 198.18.5.5/32"])
	}
}

func TestGroupMembershipChangeReplacesOnlyGroupRoutes(t *testing.T) {
	e, r, ft, _ := env(t, "PROXY@pool 203.0.113.0/24\nDIRECT 198.51.100.0/24\n", "proxy")
	gt := &groupTunnels{fakeTunnels: ft, hops: map[string]string{"pool": "nexthop dev awg0 weight 1 nexthop dev awg1 weight 1"}}
	e.Tunnels = gt
	ctx := context.Background()
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	r.batches = nil
	gt.hops["pool"] = "dev awg0" // awg1 вышел из группы
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b := r.batches[0]
	if !strings.Contains(b, "route replace 203.0.113.0/24 dev awg0 table 210") || strings.Contains(b, "198.51.100.0/24") || strings.Contains(b, "flush") {
		t.Fatalf("изменился только маршрут группы:\n%s", b)
	}
}

func TestInvalidateReplacesEverythingEvenIfPlanIsUnchanged(t *testing.T) {
	e, r, _, _ := env(t, "PROXY@awg0 203.0.113.0/24\nDIRECT 198.51.100.0/24\n", "proxy")
	ctx := context.Background()
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	r.batches = nil
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if len(r.batches) != 0 {
		t.Fatalf("без изменений таблица не трогается: %v", r.batches)
	}
	// туннель перезапустился: ядро убрало маршруты через его интерфейс, план тот же
	e.Invalidate()
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if len(r.batches) != 1 || !strings.Contains(r.batches[0], "route replace 203.0.113.0/24 dev awg0 table 210") ||
		!strings.Contains(r.batches[0], "route replace 198.51.100.0/24 via 192.168.253.1 dev eth0 table 210") || strings.Contains(r.batches[0], "flush") {
		t.Fatalf("после Invalidate все маршруты кладутся заново, но без flush (окна без политики нет):\n%v", r.batches)
	}
	r.batches = nil
	_ = e.Apply(ctx)
	if len(r.batches) != 0 {
		t.Fatal("принудительная перезаливка разовая")
	}
}
