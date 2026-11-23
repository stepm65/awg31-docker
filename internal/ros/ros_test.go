package ros

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// fakeROS — REST API RouterOS 7 в памяти: таблицы по путям, .id вида *1.
type fakeROS struct {
	mu     sync.Mutex
	tables map[string][]map[string]string
	next   int
	calls  []string
}

func newFakeROS() *fakeROS { return &fakeROS{tables: map[string][]map[string]string{}} }

func (f *fakeROS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, p, _ := r.BasicAuth(); u != "awg31" || p != "secret" {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":401,"message":"Unauthorized"}`))
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/rest")
	f.calls = append(f.calls, r.Method+" "+path)
	write := func(v any) { w.Header().Set("Content-Type", "application/json"); _ = json.NewEncoder(w).Encode(v) }
	switch {
	case path == "/system/identity":
		write(map[string]string{"name": "hAP"})
	case path == "/system/resource":
		write(map[string]string{"version": "7.21", "architecture-name": "arm64"})
	case r.Method == http.MethodPost && strings.HasSuffix(path, "/print"):
		var body struct {
			Query []string `json:".query"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		var out []map[string]string
	rows:
		for _, row := range f.tables[strings.TrimSuffix(path, "/print")] {
			for _, q := range body.Query {
				k, v, _ := strings.Cut(q, "=")
				if row[k] != v {
					continue rows
				}
			}
			out = append(out, row)
		}
		if out == nil {
			out = []map[string]string{}
		}
		write(out)
	case r.Method == http.MethodPut:
		var row map[string]string
		_ = json.NewDecoder(r.Body).Decode(&row)
		f.next++
		row[".id"] = fmt.Sprintf("*%X", f.next)
		if row["timeout"] != "" {
			row["dynamic"] = "true"
		}
		f.tables[path] = append(f.tables[path], row)
		write(row)
	case r.Method == http.MethodPatch || r.Method == http.MethodDelete:
		i := strings.LastIndex(path, "/")
		table, id := path[:i], path[i+1:]
		rows := f.tables[table]
		for j, row := range rows {
			if row[".id"] == id {
				if r.Method == http.MethodDelete {
					f.tables[table] = append(rows[:j], rows[j+1:]...)
				} else {
					var upd map[string]string
					_ = json.NewDecoder(r.Body).Decode(&upd)
					for k, v := range upd {
						row[k] = v
					}
				}
				write(map[string]string{})
				return
			}
		}
		w.WriteHeader(404)
		_, _ = w.Write([]byte(`{"error":404,"message":"Not Found","detail":"no such item"}`))
	default:
		w.WriteHeader(404)
	}
}

func (f *fakeROS) list(table, field string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.tables[table] {
		out = append(out, r[field])
	}
	sort.Strings(out)
	return out
}

func tlsServer(t *testing.T, h http.Handler) (*httptest.Server, string) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(srv.Certificate().RawSubjectPublicKeyInfo)
	return srv, "sha256//" + base64.StdEncoding.EncodeToString(sum[:])
}

func TestClientAuthPinAndURL(t *testing.T) {
	f := newFakeROS()
	srv, pin := tlsServer(t, f)
	c, err := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "secret", PinSHA256: pin})
	if err != nil {
		t.Fatal(err)
	}
	name, ver, err := c.Identity(context.Background())
	if err != nil || name != "hAP" || !strings.Contains(ver, "7.21") {
		t.Fatalf("identity: %q %q %v", name, ver, err)
	}
	bad, _ := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "wrong", PinSHA256: pin})
	if _, _, err := bad.Identity(context.Background()); err == nil || !strings.Contains(err.Error(), "пароль") {
		t.Fatalf("401: %v", err)
	}
	other := "sha256//" + base64.StdEncoding.EncodeToString(make([]byte, 32))
	wrongPin, _ := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "secret", PinSHA256: other})
	if _, _, err := wrongPin.Identity(context.Background()); err == nil || !strings.Contains(err.Error(), "пином") {
		t.Fatalf("чужой сертификат должен отвергаться: %v", err)
	}
	noPin, _ := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "secret"})
	if _, _, err := noPin.Identity(context.Background()); err == nil {
		t.Fatal("без пина самоподписанный сертификат не принимается")
	}
	for raw, ok := range map[string]bool{
		"https://192.168.253.1": true, "http://192.168.253.1": true, "http://203.0.113.1": false,
		"https://user:pw@router": false, "https://router/rest": false, "ftp://router": false,
	} {
		if _, err := ValidateURL(raw); (err == nil) != ok {
			t.Errorf("%s: %v", raw, err)
		}
	}
}

func TestSyncAddressListKeepsForeignEntries(t *testing.T) {
	f := newFakeROS()
	path := ListPath(false)
	f.tables[path] = []map[string]string{
		{".id": "*A1", "list": "awg31", "address": "8.8.8.8", "comment": "мой вручную"},
		{".id": "*A2", "list": "awg31", "address": "198.51.100.0/24", "comment": "awg31 static"},
		{".id": "*A3", "list": "awg31", "address": "203.0.113.5", "comment": "awg31 dns", "timeout": "2m", "dynamic": "true"},
		{".id": "*A4", "list": "awg31", "address": "203.0.113.5/32", "comment": "awg31 dns", "timeout": "50m", "dynamic": "true"},
		{".id": "*A5", "list": "other", "address": "1.1.1.1", "comment": "awg31 static"},
	}
	srv, pin := tlsServer(t, f)
	c, _ := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "secret", PinSHA256: pin})
	desired := map[string]Entry{
		"203.0.113.5":  {Comment: "awg31 dns", Timeout: time.Hour},
		"192.0.2.0/24": {Comment: "awg31 static"},
		"192.0.2.9":    {Comment: "awg31 dns", Timeout: 30 * time.Minute},
	}
	res, err := SyncAddressList(context.Background(), c, false, "awg31", desired)
	if err != nil {
		t.Fatal(err)
	}
	got := f.list(path, "address")
	want := []string{"1.1.1.1", "192.0.2.0/24", "192.0.2.9", "203.0.113.5", "8.8.8.8"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("список: %v, ожидалось %v", got, want)
	}
	if res.Added != 2 || res.Removed != 2 || res.Refreshed != 1 {
		t.Fatalf("итог: %+v (дубль и устаревшая сеть убраны, срок продлён)", res)
	}
	for _, r := range f.tables[path] {
		if r["address"] == "203.0.113.5" && r["timeout"] != "1h" {
			t.Fatalf("срок динамической записи продлён: %v", r)
		}
		if r["address"] == "192.0.2.9" && r["timeout"] != "30m" {
			t.Fatalf("новая динамическая запись со сроком: %v", r)
		}
	}
	// повторная синхронизация ничего не меняет
	res, err = SyncAddressList(context.Background(), c, false, "awg31", desired)
	if err != nil || res.Added+res.Removed+res.Refreshed != 0 {
		t.Fatalf("повтор: %+v %v", res, err)
	}
}

func TestSyncDNSForward(t *testing.T) {
	f := newFakeROS()
	f.tables["/ip/dns/static"] = []map[string]string{
		{".id": "*1", "name": "mine.example", "type": "FWD", "forward-to": "9.9.9.9", "comment": "свой"},
		{".id": "*2", "name": "old.example", "type": "FWD", "forward-to": "192.168.253.2", "comment": "awg31 dns"},
		{".id": "*3", "name": "youtube.com", "type": "FWD", "forward-to": "192.168.253.9", "comment": "awg31 dns"},
	}
	srv, pin := tlsServer(t, f)
	c, _ := NewClient(Conn{URL: srv.URL, User: "awg31", Pass: "secret", PinSHA256: pin})
	res, err := SyncDNSForward(context.Background(), c, "192.168.253.2", []string{"youtube.com", "work.example"})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.list("/ip/dns/static", "name"), ","); got != "mine.example,work.example,youtube.com" {
		t.Fatalf("имена: %s", got)
	}
	if res.Added != 1 || res.Removed != 1 || res.Refreshed != 1 {
		t.Fatalf("итог: %+v", res)
	}
	for _, r := range f.tables["/ip/dns/static"] {
		if r["name"] != "mine.example" && (r["forward-to"] != "192.168.253.2" || r["match-subdomain"] == "no") {
			t.Fatalf("запись: %v", r)
		}
	}
}

func TestDurations(t *testing.T) {
	for s, want := range map[string]time.Duration{"1h2m3s": time.Hour + 2*time.Minute + 3*time.Second, "1w": 7 * 24 * time.Hour,
		"23:59:59": 24*time.Hour - time.Second, "1d02:00:00": 26 * time.Hour, "": 0} {
		if got := ParseDuration(s); got != want {
			t.Errorf("%q: %v", s, got)
		}
	}
	if FormatDuration(90*time.Minute) != "1h30m" || FormatDuration(0) != "1s" {
		t.Fatal("FormatDuration")
	}
}

// fakePeer — BGP-роутер: принимает соединение, отвечает OPEN и KEEPALIVE, собирает UPDATE.
type fakePeer struct {
	as      uint32
	mu      sync.Mutex
	adv     map[netip.Prefix]bool
	attrs   []byte
	updates int
	gotAS   uint32
}

func (p *fakePeer) serve(t *testing.T, ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go func(c net.Conn) {
			defer c.Close()
			typ, body, err := ReadMessage(c)
			if err != nil || typ != bgpOpen {
				return
			}
			o, _ := ParseOpen(body)
			p.mu.Lock()
			p.gotAS = o.AS
			p.mu.Unlock()
			_, _ = c.Write(BuildOpen(p.as, 90, netip.MustParseAddr("192.168.253.1")))
			_, _ = c.Write(frame(bgpKeepalive, nil))
			for {
				typ, body, err := ReadMessage(c)
				if err != nil {
					return
				}
				if typ == bgpUpdate {
					w, n, attrs, err := ParseUpdate(body)
					if err != nil {
						t.Errorf("UPDATE: %v", err)
						return
					}
					p.mu.Lock()
					p.updates++
					for _, x := range w {
						delete(p.adv, x)
					}
					for _, x := range n {
						p.adv[x] = true
					}
					if len(attrs) > 0 {
						p.attrs = attrs
					}
					p.mu.Unlock()
				}
			}
		}(c)
	}
}

func (p *fakePeer) prefixes() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []string
	for x := range p.adv {
		out = append(out, x.String())
	}
	sort.Strings(out)
	return out
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for i := 0; i < 200; i++ {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("не дождались: %s", what)
}

func TestBGPAnnounceAndWithdraw(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	peer := &fakePeer{as: 65030, adv: map[netip.Prefix]bool{}}
	go peer.serve(t, ln)
	s := &Speaker{}
	defer s.Close()
	s.SetPrefixes([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("198.51.100.7/32"), netip.MustParsePrefix("2001:db8::/32")})
	cfg := &BGPConfig{Peer: netip.MustParseAddrPort(ln.Addr().String()), LocalAS: 4200000001, RemoteAS: 65030,
		RouterID: netip.MustParseAddr("192.168.253.2"), NextHop: netip.MustParseAddr("192.168.253.2")}
	if err := s.Configure(cfg); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return len(peer.prefixes()) == 2 }, "анонс двух IPv4-сетей")
	if got := strings.Join(peer.prefixes(), ","); got != "198.51.100.7/32,203.0.113.0/24" {
		t.Fatalf("анонсы: %s", got)
	}
	if st := s.Status(); st.State != "established" || st.Announced != 2 {
		t.Fatalf("состояние: %+v", st)
	}
	peer.mu.Lock()
	attrs, gotAS := peer.attrs, peer.gotAS
	peer.mu.Unlock()
	if gotAS != 4200000001 {
		t.Fatalf("4-байтовый AS в OPEN: %d", gotAS)
	}
	// NEXT_HOP — IP контейнера, AS_PATH — 4 байта (роутер поддерживает AS4)
	if !strings.Contains(string(attrs), string([]byte{0x40, 3, 4, 192, 168, 253, 2})) || !strings.Contains(string(attrs), string([]byte{0x40, 2, 6, 2, 1, 0xFA, 0x56, 0xEA, 0x01})) {
		t.Fatalf("атрибуты: % x", attrs)
	}
	s.SetPrefixes([]netip.Prefix{netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("192.0.2.0/25")})
	waitFor(t, func() bool { return strings.Join(peer.prefixes(), ",") == "192.0.2.0/25,203.0.113.0/24" }, "отзыв и новый анонс")
}

func TestBGPRejectsWrongPeerAS(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go (&fakePeer{as: 64999, adv: map[netip.Prefix]bool{}}).serve(t, ln)
	s := &Speaker{}
	defer s.Close()
	_ = s.Configure(&BGPConfig{Peer: netip.MustParseAddrPort(ln.Addr().String()), LocalAS: 65031, RemoteAS: 65030,
		RouterID: netip.MustParseAddr("192.168.253.2"), NextHop: netip.MustParseAddr("192.168.253.2")})
	waitFor(t, func() bool { return s.Status().State == "error" }, "ошибка сессии")
	if !strings.Contains(s.Status().LastError, "64999") {
		t.Fatalf("причина: %s", s.Status().LastError)
	}
}

func TestBGPMessagesAreBounded(t *testing.T) {
	var many []netip.Prefix
	for i := 0; i < 3000; i++ {
		many = append(many, netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i >> 8), byte(i), 0}), 24))
	}
	msgs := BuildUpdates(many, many[:1500], 65031, netip.MustParseAddr("192.168.253.2"), true, false)
	total := 0
	for _, m := range msgs {
		if len(m) > bgpMaxMsg {
			t.Fatalf("сообщение %d байт больше 4096", len(m))
		}
		typ, body, err := ReadMessage(strings.NewReader(string(m)))
		if err != nil || typ != bgpUpdate {
			t.Fatal(err)
		}
		_, n, _, err := ParseUpdate(body)
		if err != nil {
			t.Fatal(err)
		}
		total += len(n)
	}
	if total != 3000 {
		t.Fatalf("анонсировано %d из 3000", total)
	}
	if _, err := ParseOpen([]byte{3, 0, 1, 0, 90, 1, 2, 3, 4, 0}); err == nil {
		t.Fatal("версия BGP 3 не поддерживается")
	}
	if _, _, err := ReadMessage(strings.NewReader(strings.Repeat("\x00", 19))); err == nil {
		t.Fatal("неверный маркер")
	}
	_ = io.EOF
}

type rosTunnels struct{ o tunnel.Options }

func (r rosTunnels) Options() tunnel.Options { return r.o }

type rosRunner struct{}

func (rosRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	switch strings.Join(args, " ") {
	case "-o -4 addr show dev eth0":
		return []byte("2: eth0    inet 192.168.253.2/30 brd 192.168.253.3 scope global eth0\n"), nil
	case "-o -4 route show default dev eth0":
		return []byte("default via 192.168.253.1 dev eth0\n"), nil
	}
	return nil, nil
}

func TestManagerAddressListModeAndCleanup(t *testing.T) {
	f := newFakeROS()
	srv, pin := tlsServer(t, f)
	st, _ := store.Open(t.TempDir())
	p := store.DefaultPolicy()
	p.Enabled, p.Rules = true, "PROXY 203.0.113.0/24\nDIRECT 198.51.100.0/24\nPROXY youtube.com\n"
	r := store.DefaultRouter()
	r.Mode, r.URL, r.Pass, r.PinSHA256, r.SyncDNS = store.RouterAddressList, srv.URL, "secret", pin, true
	_ = st.Update(func(s *store.State) error { s.Policy, s.Router = &p, &r; return nil })
	tun := rosTunnels{tunnel.Options{AutoRoute: true, LanIface: "eth0"}}
	eng := &policy.Engine{Store: st, Tunnels: fakePolTunnels{tun.o}}
	_ = eng.Reload()
	eng.Learn(policy.Proxy, "", "domain:youtube.com", []netip.Addr{netip.MustParseAddr("142.250.1.1")}, time.Hour)
	m := &Manager{Store: st, Policy: eng, Tunnels: tun, Runner: rosRunner{}}
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(f.list(ListPath(false), "address"), ","); got != "142.250.1.1,203.0.113.0/24" {
		t.Fatalf("address-list: %s (только PROXY и выученные адреса)", got)
	}
	if got := strings.Join(f.list("/ip/dns/static", "forward-to"), ","); got != "192.168.253.2" {
		t.Fatalf("DNS FWD на IP контейнера: %s", got)
	}
	s := m.Status()
	if s.Static != 1 || s.Dynamic != 1 || s.Domains != 1 || !strings.Contains(s.Router, "hAP") {
		t.Fatalf("состояние: %+v", s)
	}
	// выключение — записи контейнера убираются
	r.Mode, r.SyncDNS = store.RouterOff, false
	_ = st.Update(func(s *store.State) error { s.Router = &r; return nil })
	if err := m.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(f.list(ListPath(false), "address")) + len(f.list("/ip/dns/static", "name")); n != 0 {
		t.Fatalf("после выключения осталось записей: %d", n)
	}
}

type fakePolTunnels struct{ o tunnel.Options }

func (f fakePolTunnels) Table(string) int        { return 0 }
func (f fakePolTunnels) Options() tunnel.Options { return f.o }

func TestValidate(t *testing.T) {
	r := store.DefaultRouter()
	r.Mode = store.RouterBGP
	if errs := Validate(r); len(errs) != 0 {
		t.Fatal(errs)
	}
	r.URL, r.User, r.AddressList, r.BGPLocalAS, r.DynamicMin = "http://8.8.8.8", "a b", "x;y", 0, 1
	if errs := Validate(r); len(errs) != 5 {
		t.Fatalf("ожидалось 5 ошибок: %v", errs)
	}
	r.Mode = "magic"
	if len(Validate(r)) != 1 {
		t.Fatal("режим")
	}
}

func TestSetRemoveRejectUnexpectedID(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits++; w.Write([]byte("{}")) }))
	defer srv.Close()
	c, err := NewClient(Conn{URL: srv.URL, User: "u", Pass: "p"})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"", "1", "*1/../../user", "*zz", "*1?x=1"} {
		if err := c.Remove(context.Background(), "/ip/firewall/address-list", id); err == nil {
			t.Fatalf("Remove(%q) должен отказать", id)
		}
		if err := c.Set(context.Background(), "/ip/firewall/address-list", id, map[string]string{"timeout": "1h"}); err == nil {
			t.Fatalf("Set(%q) должен отказать", id)
		}
	}
	if hits != 0 {
		t.Fatalf("запросы с неверным .id не должны уходить на роутер: %d", hits)
	}
	if err := c.Remove(context.Background(), "/ip/firewall/address-list", "*1A"); err != nil || hits != 1 {
		t.Fatalf("корректный .id: %v, запросов %d", err, hits)
	}
}
