package dns

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"awg31/internal/dnswire"
	"awg31/internal/geo"
	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func TestParseUpstreams(t *testing.T) {
	ups, err := ParseUpstreams("1.1.1.1, 8.8.8.8:5353 [2606:4700:4700::1111]:53;https://dns.google")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, u := range ups {
		got = append(got, u.String())
	}
	if strings.Join(got, "|") != "1.1.1.1:53|8.8.8.8:5353|[2606:4700:4700::1111]:53|https://dns.google/dns-query" {
		t.Fatalf("%v", got)
	}
	for _, bad := range []string{"", "dns.google", "1.1.1.1:0", "https://user:pw@x/dns-query", "http://1.1.1.1/dns-query", strings.Repeat("1.1.1.1,", 9)} {
		if _, err := ParseUpstreams(bad); err == nil {
			t.Errorf("%q должно отвергаться", bad)
		}
	}
}

type fakeTunnels struct {
	primary string
	tables  map[string]int
	opts    tunnel.Options
}

func (f *fakeTunnels) Primary() string         { return f.primary }
func (f *fakeTunnels) Table(n string) int      { return f.tables[n] }
func (f *fakeTunnels) Options() tunnel.Options { return f.opts }

func setup(t *testing.T, rules string) (*Server, *policy.Engine, *fakeTunnels) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := store.DefaultPolicy()
	p.Enabled, p.Rules, p.DefaultAction = true, rules, "direct"
	p.ProxyDNS, p.DirectDNS = "10.0.0.53, 10.0.0.54", "192.168.253.1"
	_ = st.Update(func(s *store.State) error { s.Policy = &p; return nil })
	gdir := filepath.Join(dir, "geo")
	_ = os.MkdirAll(gdir, 0o700)
	_ = os.WriteFile(filepath.Join(gdir, "geosite.dat"), []byte("youtube.com\n"), 0o600)
	ft := &fakeTunnels{primary: "awg0", tables: map[string]int{"awg0": 110, "awg1": 111}, opts: tunnel.Options{AutoRoute: true, LanIface: "eth0", KillSwitch: true}}
	eng := &policy.Engine{Store: st, Geo: &geo.Store{Dir: gdir}, Tunnels: ft}
	if err := eng.Reload(); err != nil {
		t.Fatal(err)
	}
	return &Server{Store: st, Policy: eng, Tunnels: ft}, eng, ft
}

func reply(q []byte, addrs ...string) []byte {
	_, _, end, _ := dnswire.Question(q)
	r := append([]byte(nil), q[:end]...)
	r[2] |= 0x80
	binary.BigEndian.PutUint16(r[6:8], uint16(len(addrs)))
	for _, a := range addrs {
		ip := netip.MustParseAddr(a).As4()
		r = append(r, 0xC0, 12, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4)
		r = append(r, ip[:]...)
	}
	return r
}

func TestDecide(t *testing.T) {
	s, _, ft := setup(t, "PROXY geosite:youtube\nDROP ads.example\nPROXY@awg1 work.example\nPROXY@awg9 down.example\n")
	cases := map[string]func(Route) bool{
		"m.youtube.com": func(r Route) bool {
			return r.Action == policy.Proxy && r.Mark == tunnel.PrimaryMark && r.Ups[0].Addr == "10.0.0.53:53"
		},
		"x.ads.example":    func(r Route) bool { return r.Refuse == dnswire.RcodeNXDomain },
		"git.work.example": func(r Route) bool { return r.Mark == tunnel.ProbeMark(111) },
		"down.example":     func(r Route) bool { return r.Refuse == dnswire.RcodeServFail }, // kill switch
		"other.org": func(r Route) bool {
			return r.Action == policy.Direct && r.Mark == 0 && r.Ups[0].Addr == "192.168.253.1:53"
		},
	}
	for name, ok := range cases {
		if r := s.Decide(name); !ok(r) {
			t.Errorf("%s: %+v", name, r)
		}
	}
	ft.opts.KillSwitch = false
	if r := s.Decide("down.example"); r.Refuse != 0 || r.Mark != tunnel.PrimaryMark {
		t.Fatalf("без kill switch PROXY@неподнятый идёт через основной: %+v", r)
	}
	ft.opts.LanIface = ""
	if r := s.Decide("m.youtube.com"); r.Mark != 0 {
		t.Fatal("без режима шлюза метки нет")
	}
}

func TestHandleFallbackAndLearning(t *testing.T) {
	s, eng, _ := setup(t, "PROXY geosite:youtube\n")
	var mu sync.Mutex
	var seen []string
	s.Exchange = func(_ context.Context, q []byte, up Upstream, mark int) ([]byte, error) {
		mu.Lock()
		seen = append(seen, up.Addr)
		mu.Unlock()
		if up.Addr == "10.0.0.53:53" {
			return nil, errors.New("таймаут")
		}
		r := reply(q, "142.250.74.14")
		if up.Addr == "192.168.253.1:53" {
			r[0] ^= 0xFF // чужой ID
		}
		return r, nil
	}
	q, _ := dnswire.Query(0x4242, "www.youtube.com", dnswire.TypeA)
	resp := s.Handle(context.Background(), q)
	if !dnswire.IsResponseTo(resp, 0x4242) || len(dnswire.Answers(resp)) != 1 {
		t.Fatalf("ответ второго сервера: % x", resp)
	}
	if strings.Join(seen, ",") != "10.0.0.53:53,10.0.0.54:53" {
		t.Fatalf("переход к следующему серверу: %v", seen)
	}
	if r := eng.Test("142.250.74.14"); r.Action != policy.Proxy || !strings.Contains(r.By, "DNS") {
		t.Fatalf("адрес из ответа выучен: %+v", r)
	}
	// ответ с чужим ID не принимается
	q2, _ := dnswire.Query(0x1111, "other.org", dnswire.TypeA)
	if r := s.Handle(context.Background(), q2); r[3]&0x0F != dnswire.RcodeServFail {
		t.Fatal("ответ с чужим ID — SERVFAIL")
	}
	if st := eng.Stats(); st.Queries != 2 || st.Proxy != 1 || st.Direct != 1 {
		t.Fatalf("счётчики: %+v", st)
	}
	if r := s.Handle(context.Background(), []byte{1, 2, 3}); r != nil && len(r) >= 4 && r[3]&0x0F == 0 {
		t.Fatal("мусор не должен давать успешный ответ")
	}
}

// fakeUpstream — DNS-сервер на loopback: по UDP отвечает усечённо (TC), по TCP — полностью.
func fakeUpstream(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	pc, err := net.ListenPacket("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close(); pc.Close() })
	go func() {
		buf := make([]byte, 4096)
		for {
			n, a, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			r := reply(buf[:n])
			r[2] |= 0x02 // TC
			_, _ = pc.WriteTo(r, a)
		}
	}()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			q, err := readTCP(c)
			if err == nil {
				_ = writeTCP(c, reply(q, "198.51.100.7"))
			}
			c.Close()
		}
	}()
	return addr
}

func TestExchangeTruncatedFallsBackToTCP(t *testing.T) {
	up := fakeUpstream(t)
	s := &Server{}
	s.init()
	q, _ := dnswire.Query(7, "big.example", dnswire.TypeA)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := s.exchange(ctx, q, Upstream{Addr: up}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if a := dnswire.Answers(resp); len(a) != 1 || a[0].Addr.String() != "198.51.100.7" {
		t.Fatalf("после TC ответ берётся по TCP: %v", a)
	}
}

func TestDoH(t *testing.T) {
	var gotCT string
	var gotID uint16
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		q, _ := io.ReadAll(r.Body)
		gotID = dnswire.ID(q)
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(reply(q, "203.0.113.9"))
	}))
	defer srv.Close()
	pool := srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	s := &Server{TLSConfig: &tls.Config{RootCAs: pool}}
	s.init()
	u, _ := url.Parse(srv.URL + "/dns-query")
	q, _ := dnswire.Query(0xABCD, "doh.example", dnswire.TypeA)
	resp, err := s.exchange(context.Background(), q, Upstream{DoH: u}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if gotCT != "application/dns-message" || gotID != 0 {
		t.Fatalf("RFC 8484: тип %q, ID в запросе %d", gotCT, gotID)
	}
	if dnswire.ID(resp) != 0xABCD || len(dnswire.Answers(resp)) != 1 {
		t.Fatal("ответ DoH с исходным ID")
	}
	bad := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/html") // тело — настоящий DNS-ответ, но тип не тот
		_, _ = w.Write(reply(q, "203.0.113.9"))
	}))
	defer bad.Close()
	ub, _ := url.Parse(bad.URL + "/dns-query")
	s2 := &Server{TLSConfig: &tls.Config{RootCAs: bad.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}}
	s2.init()
	if _, err := s2.exchange(context.Background(), q, Upstream{DoH: ub}, 0); err == nil {
		t.Fatal("ответ не application/dns-message должен отвергаться")
	}
	// без доверенного сертификата — ошибка
	s3 := &Server{}
	s3.init()
	if _, err := s3.exchange(context.Background(), q, Upstream{DoH: u}, 0); err == nil {
		t.Fatal("сертификат DoH-сервера проверяется")
	}
}

func TestServeOnlyPrivateClients(t *testing.T) {
	s, _, _ := setup(t, "")
	s.Exchange = func(_ context.Context, q []byte, _ Upstream, _ int) ([]byte, error) {
		return reply(q, "192.0.2.1"), nil
	}
	if s.allowed(netip.MustParseAddr("8.8.8.8")) || !s.allowed(netip.MustParseAddr("192.168.88.10")) || !s.allowed(netip.MustParseAddr("::ffff:10.1.1.1")) {
		t.Fatal("принимаются только частные адреса")
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	s.Listen = addr
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = s.ListenAndServe(ctx) }()
	time.Sleep(100 * time.Millisecond)
	q, _ := dnswire.Query(99, "a.example", dnswire.TypeA)
	for _, network := range []string{"udp", "tcp"} {
		c, err := net.DialTimeout(network, addr, time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		var resp []byte
		if network == "udp" {
			_, _ = c.Write(q)
			buf := make([]byte, 1500)
			n, err := c.Read(buf)
			if err != nil {
				t.Fatal(err)
			}
			resp = buf[:n]
		} else {
			_ = writeTCP(c, q)
			if resp, err = readTCP(c); err != nil {
				t.Fatal(err)
			}
		}
		c.Close()
		if !dnswire.IsResponseTo(resp, 99) || len(dnswire.Answers(resp)) != 1 {
			t.Fatalf("%s: % x", network, resp)
		}
	}
}

func TestDialerSetsMark(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	c, err := dialer(tunnel.PrimaryMark).Dial("udp", pc.LocalAddr().String())
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "protocol not available") {
			t.Skip("SO_MARK недоступен в этой среде:", err)
		}
		t.Fatal(err)
	}
	defer c.Close()
	raw, err := c.(*net.UDPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var mark int
	var gerr error
	_ = raw.Control(func(fd uintptr) { mark, gerr = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_MARK) })
	if gerr != nil || mark != tunnel.PrimaryMark {
		t.Fatalf("метка сокета %#x, ожидалась %#x (%v)", mark, tunnel.PrimaryMark, gerr)
	}
}
