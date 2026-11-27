package policy

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"awg31/internal/store"
)

func TestParseScopeModifiers(t *testing.T) {
	rules, errs := Parse("PROXY@awg0 geosite:steam src=@pc,192.168.51.0/28 port=443,27000-27100 net=udp,tcp\nDIRECT example.com src=@tv\nPROXY,geosite:x,comment\nDROP 1.2.3.4 port=80\n")
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	r := rules[0]
	if !r.Scoped() || strings.Join(r.Src, " ") != "@pc 192.168.51.0/28" || len(r.Ports) != 2 || r.Ports[1] != (PortRange{27000, 27100}) || strings.Join(r.Net, ",") != "tcp,udp" {
		t.Fatalf("%+v", r)
	}
	if got := r.String(); got != "PROXY@awg0 geosite:steam src=@pc,192.168.51.0/28 port=443,27000-27100 net=tcp,udp" {
		t.Fatalf("канонический вид: %s", got)
	}
	if rules[2].Scoped() || rules[2].Value != "x" {
		t.Fatalf("формат через запятую: %+v", rules[2])
	}
	if got := DeviceRefs(rules); strings.Join(got, ",") != "pc,tv" {
		t.Fatalf("ссылки: %v", got)
	}
	for _, bad := range []string{"PROXY x.com src=pc", "PROXY x.com port=0", "PROXY x.com port=9-8", "PROXY x.com net=icmp", "PROXY x.com foo=1", "PROXY x.com src=@a src=@b", "PROXY x.com port"} {
		if _, errs := Parse(bad); len(errs) == 0 {
			t.Errorf("%q должна быть ошибкой", bad)
		}
	}
}

func TestScopeIPRules(t *testing.T) {
	devs := map[string]netip.Prefix{"pc": netip.MustParsePrefix("192.168.51.150/32"), "v6": netip.MustParsePrefix("fd00::5/128")}
	rules, _ := Parse("PROXY a.com src=@pc,@v6 port=443\nDIRECT b.com src=@pc,@v6 port=443\nDROP c.com\nPROXY d.com net=udp\nPROXY e.com src=@ghost\n")
	scopes, notes := buildScopes(rules, devs)
	if len(scopes) != 2 || len(scopes[0].rules) != 2 || scopes[0].table != ScopeTableBase || scopes[1].prio != ScopePrioBase+1 {
		t.Fatalf("области: %+v", scopes)
	}
	if len(notes) != 2 || !strings.Contains(strings.Join(notes, " "), "@ghost") {
		t.Fatalf("заметки: %v", notes)
	}
	got := scopes[0].ipRules(4, "eth0")
	want := []string{
		"priority 200 from 192.168.51.150/32 iif eth0 ipproto tcp dport 443 lookup 300",
		"priority 200 from 192.168.51.150/32 iif eth0 ipproto udp dport 443 lookup 300",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("ip rule v4:\n%s", strings.Join(got, "\n"))
	}
	if got := scopes[0].ipRules(6, "eth0"); len(got) != 2 || !strings.Contains(got[0], "from fd00::5/128") {
		t.Fatalf("v6: %v", got)
	}
	if got := scopes[1].ipRules(4, "eth0"); len(got) != 1 || got[0] != "priority 201 from all iif eth0 ipproto udp lookup 301" {
		t.Fatalf("только протокол: %v", got)
	}
	sc := scopes[0]
	if !sc.matches(netip.MustParseAddr("192.168.51.150"), "udp", 443) || sc.matches(netip.MustParseAddr("192.168.51.151"), "tcp", 443) || sc.matches(netip.MustParseAddr("192.168.51.150"), "tcp", 80) {
		t.Fatal("matches")
	}
}

// primTunnels — fakeTunnels с основным туннелем (policy.Primarier).
type primTunnels struct {
	*fakeTunnels
	primary string
}

func (p *primTunnels) Primary() string { return p.primary }

func TestApplyScopedRules(t *testing.T) {
	e, r, ft, st := env(t, "PROXY 203.0.113.0/24 src=@pc\nDIRECT 198.51.100.0/24 src=@pc\nDROP 192.0.2.0/24\nPROXY steam.example port=27015 net=udp\n", "proxy")
	pt := &primTunnels{fakeTunnels: ft, primary: "awg0"}
	e.Tunnels = pt
	_ = st.Update(func(s *store.State) error {
		s.Devices = []store.Device{{ID: "pc", Name: "PC", Addr: "192.168.51.150", Kind: "pc"}}
		return nil
	})
	if err := e.Reload(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b := strings.Join(r.batches, "\n")
	for _, want := range []string{
		"route replace blackhole 192.0.2.0/24 table 210",
		"route flush table 300",
		"route replace 203.0.113.0/24 dev awg0 table 300", // PROXY области — прямо в основной туннель
		"route replace 198.51.100.0/24 via 192.168.253.1 dev eth0 table 300",
		"route replace throw 192.168.253.0/30 table 300",
		"rule add priority 200 from 192.168.51.150/32 iif eth0 lookup 300",
		"rule add priority 201 from all iif eth0 ipproto udp dport 27015 lookup 301",
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("нет «%s»:\n%s", want, b)
		}
	}
	if strings.Contains(b, "203.0.113.0/24 table 210") {
		t.Fatal("суженное правило попало в общую таблицу")
	}
	// DNS: адреса доменного правила области учатся в её таблицу
	r.batches = nil
	e.LearnScoped("cdn.steam.example", []netip.Addr{netip.MustParseAddr("8.8.4.4"), netip.MustParseAddr("10.0.0.1")}, time.Minute)
	if err := e.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	b = strings.Join(r.batches, "\n")
	if !strings.Contains(b, "route replace 8.8.4.4/32 dev awg0 table 301") || strings.Contains(b, "10.0.0.1") || strings.Contains(b, "rule add") {
		t.Fatalf("выученный адрес области (без повторного добавления правил):\n%s", b)
	}
	// основной туннель сменился — маршруты PROXY областей переезжают
	ft.tables["awg1"] = 111
	pt.primary = "awg1"
	r.batches = nil
	_ = e.Apply(ctx)
	if b = strings.Join(r.batches, "\n"); !strings.Contains(b, "route replace 203.0.113.0/24 dev awg1 table 300") {
		t.Fatalf("смена основного:\n%s", b)
	}
	// правило с устройством убрано — правило и таблица уходят
	p := st.Snapshot().PolicySettings()
	p.Rules = "DROP 192.0.2.0/24\n"
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	_ = e.Reload()
	r.batches = nil
	_ = e.Apply(ctx)
	b = strings.Join(r.batches, "\n")
	if !strings.Contains(b, "rule del priority 200 from 192.168.51.150/32 iif eth0 lookup 300") || !strings.Contains(b, "route del 203.0.113.0/24 dev awg1 table 300") {
		t.Fatalf("удаление области:\n%s", b)
	}
	// без основного туннеля и с kill switch — blackhole, а не мимо VPN
	pt.primary = ""
	p.Rules = "PROXY 203.0.113.0/24 src=@pc\n"
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	_ = e.Reload()
	r.batches = nil
	_ = e.Apply(ctx)
	if b = strings.Join(r.batches, "\n"); !strings.Contains(b, "route replace blackhole 203.0.113.0/24 table 300") {
		t.Fatalf("kill switch:\n%s", b)
	}
}

func TestScopedRuleWithUnknownDeviceIsNotApplied(t *testing.T) {
	e, r, _, _ := env(t, "PROXY 203.0.113.0/24 src=@ghost\n", "proxy")
	_ = e.Apply(context.Background())
	b := strings.Join(r.batches, "\n")
	if strings.Contains(b, "203.0.113.0/24") || strings.Contains(b, "rule add") {
		t.Fatalf("правило с неизвестным устройством не должно стать общим:\n%s", b)
	}
	if !strings.Contains(e.Stats().LastError, "@ghost") {
		t.Fatalf("заметка: %q", e.Stats().LastError)
	}
}

func TestClassify(t *testing.T) {
	e, _, ft, st := env(t, "#@ Steam | steam\nDIRECT 203.0.113.0/24 src=@pc port=443\n\n#@ Сеть\nPROXY@awg0 203.0.113.0/24\nDROP 198.51.100.0/24\n", "direct")
	e.Tunnels = &primTunnels{fakeTunnels: ft, primary: "awg0"}
	_ = st.Update(func(s *store.State) error {
		s.Devices = []store.Device{{ID: "pc", Name: "PC", Addr: "192.168.51.150"}}
		return nil
	})
	_ = e.Reload()
	pc, tv := netip.MustParseAddr("192.168.51.150"), netip.MustParseAddr("192.168.51.10")
	dst := netip.MustParseAddr("203.0.113.7")
	if v := e.Classify(pc, dst, "tcp", 443); v.Line != 2 || v.Action != Direct {
		t.Fatalf("область: %+v", v)
	}
	if v := e.Classify(pc, dst, "tcp", 80); v.Line != 5 || v.Action != Proxy || v.Target != "awg0" {
		t.Fatalf("другой порт — общее правило: %+v", v)
	}
	if v := e.Classify(tv, netip.MustParseAddr("198.51.100.1"), "udp", 53); v.Line != 6 || v.Action != Drop {
		t.Fatalf("%+v", v)
	}
	if v := e.Classify(tv, netip.MustParseAddr("8.8.8.8"), "udp", 53); v.Line != 0 || v.Action != Direct || v.By != "default" {
		t.Fatalf("по умолчанию: %+v", v)
	}
	e.LearnScoped("x", nil, 0) // пустой ответ ничего не ломает
	lb := Labels("#@ Steam | steam\nDIRECT a.example\n#~ DIRECT b.example\nDIRECT c.example\n\nPROXY d.example\n")
	if lb[2].Name != "Steam" || lb[2].Head != 1 || lb[4].Head != 1 || lb[3].Name != "" || lb[6].Name != "" {
		t.Fatalf("подписи: %+v", lb)
	}
	e.NoteNames("WWW.Example.org.", []netip.Addr{netip.MustParseAddr("192.0.2.1")}, time.Minute)
	if e.Name(netip.MustParseAddr("192.0.2.1")) != "www.example.org" {
		t.Fatal("имя из DNS")
	}
}
