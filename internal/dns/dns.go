// Package dns — DNS-форвардер для клиентов LAN (этап 3). Каждый запрос
// проверяется правилами политики: DROP — NXDOMAIN; DIRECT — к серверам «мимо
// VPN»; PROXY — к серверам «через VPN», и сам запрос уходит через основной
// (или указанный в правиле) туннель по метке SO_MARK. Поддерживаются обычный
// DNS (UDP с переходом на TCP) и DNS-over-HTTPS (RFC 8484). Адреса из ответов
// для доменных правил передаются политике, чтобы трафик к ним шёл тем же путём.
package dns

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"awg31/internal/dnswire"
	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Upstream — сервер DNS: Addr (IP:порт) или DoH (https-адрес).
type Upstream struct {
	Addr string
	DoH  *url.URL
}

func (u Upstream) String() string {
	if u.DoH != nil {
		return u.DoH.String()
	}
	return u.Addr
}

// ParseUpstreams разбирает список через запятую или пробел:
// «1.1.1.1», «8.8.8.8:53», «[2606:4700:4700::1111]:53», «https://dns.google/dns-query».
func ParseUpstreams(s string) ([]Upstream, error) {
	var out []Upstream
	for _, item := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == '\n' || r == ';' }) {
		if strings.HasPrefix(strings.ToLower(item), "https://") {
			u, err := url.Parse(item)
			if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
				return nil, fmt.Errorf("DoH-сервер «%s»: нужен адрес вида https://хост/dns-query", item)
			}
			if u.Path == "" {
				u.Path = "/dns-query"
			}
			out = append(out, Upstream{DoH: u})
			continue
		}
		if a, err := netip.ParseAddr(item); err == nil {
			out = append(out, Upstream{Addr: netip.AddrPortFrom(a, 53).String()})
			continue
		}
		ap, err := netip.ParseAddrPort(item)
		if err != nil || ap.Port() == 0 {
			return nil, fmt.Errorf("DNS-сервер «%s»: нужен IP, IP:порт или https://…/dns-query", item)
		}
		out = append(out, Upstream{Addr: ap.String()})
	}
	if len(out) == 0 {
		return nil, errors.New("нужен хотя бы один DNS-сервер")
	}
	if len(out) > 8 {
		return nil, errors.New("не больше 8 DNS-серверов")
	}
	return out, nil
}

// DefaultAllow — от кого принимаются запросы: частные сети и loopback.
// Форвардер не должен становиться открытым резолвером.
var DefaultAllow = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"), netip.MustParsePrefix("::1/128"),
}

// Tunnels — что нужно от tunnel.Manager.
type Tunnels interface {
	Primary() string
	Table(name string) int
	Options() tunnel.Options
}

// Exchanger отправляет запрос серверу (подменяется в тестах).
type Exchanger func(ctx context.Context, query []byte, up Upstream, mark int) ([]byte, error)

// Server — форвардер.
type Server struct {
	Listen  string // адрес, например ":53"
	Store   *store.Store
	Policy  *policy.Engine
	Tunnels Tunnels
	Allow   []netip.Prefix
	Logf    func(format string, args ...any)
	// Exchange по умолчанию — настоящий обмен по сети (UDP/TCP/DoH).
	Exchange Exchanger
	// TLSConfig для DoH (nil — системные корневые сертификаты; задаётся в тестах).
	TLSConfig *tls.Config

	sem  chan struct{}
	once sync.Once
	doh  *dohClients
}

func (s *Server) init() {
	s.once.Do(func() {
		s.sem = make(chan struct{}, 256)
		if s.Allow == nil {
			s.Allow = DefaultAllow
		}
		s.doh = &dohClients{m: map[int]*http.Client{}, tls: s.TLSConfig}
		if s.Exchange == nil {
			s.Exchange = s.exchange
		}
	})
}

func (s *Server) logf(f string, a ...any) {
	if s.Logf != nil {
		s.Logf(f, a...)
	}
}

func (s *Server) allowed(a netip.Addr) bool {
	s.init()
	a = a.Unmap()
	for _, p := range s.Allow {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Route — решение по запросу (для тестов и журнала).
type Route struct {
	Action policy.Action
	Target string
	By     string
	Mark   int
	Ups    []Upstream
	Refuse byte // ненулевой rcode — ответить сразу
}

// Decide выбирает путь для имени.
func (s *Server) Decide(name string) Route {
	s.init()
	p := s.Store.Snapshot().PolicySettings()
	r := Route{Action: policy.Proxy, By: "default"}
	if s.Policy != nil && s.Policy.Enabled() {
		r.Action, r.Target, r.By = s.Policy.MatchDomain(name)
	}
	o := s.Tunnels.Options()
	gateway := o.AutoRoute && o.LanIface != ""
	switch r.Action {
	case policy.Drop:
		r.Refuse = dnswire.RcodeNXDomain
		return r
	case policy.Direct:
		r.Ups, _ = ParseUpstreams(p.DirectDNS)
		return r
	}
	r.Ups, _ = ParseUpstreams(p.ProxyDNS)
	if !gateway {
		return r // без режима шлюза туннели не привязаны к таблицам: обычный маршрут
	}
	if r.Target != "" {
		if t := s.Tunnels.Table(r.Target); t != 0 {
			r.Mark = tunnel.ProbeMark(t)
			return r
		}
		if o.KillSwitch {
			r.Refuse = dnswire.RcodeServFail // туннеля нет — запрос не уходит мимо VPN
			return r
		}
	}
	// основной туннель: метка ведёт в таблицу 103, где при kill switch без
	// туннеля только blackhole — запрос не уйдёт мимо VPN
	r.Mark = tunnel.PrimaryMark
	return r
}

// Handle отвечает на один запрос.
func (s *Server) Handle(ctx context.Context, query []byte) []byte {
	s.init()
	name, _, _, ok := dnswire.Question(query)
	if !ok {
		return dnswire.ErrorReply(query, 1) // FORMERR
	}
	r := s.Decide(name)
	if s.Policy != nil {
		s.Policy.Note(r.Action)
	}
	if r.Refuse != 0 {
		return dnswire.ErrorReply(query, r.Refuse)
	}
	var lastErr error
	for _, up := range r.Ups {
		qctx, cancel := context.WithTimeout(ctx, 4*time.Second)
		resp, err := s.Exchange(qctx, query, up, r.Mark)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		if !dnswire.IsResponseTo(resp, dnswire.ID(query)) {
			lastErr = errors.New("ответ не на этот запрос")
			continue
		}
		if s.Policy != nil && s.Policy.Enabled() {
			ans := dnswire.Answers(resp)
			addrs := make([]netip.Addr, 0, len(ans))
			ttl := uint32(0)
			for i, a := range ans {
				addrs = append(addrs, a.Addr)
				if i == 0 || a.TTL < ttl {
					ttl = a.TTL
				}
			}
			if r.By != "default" {
				s.Policy.Learn(r.Action, r.Target, r.By, addrs, time.Duration(ttl)*time.Second)
			}
			// правила с устройствами и портами учатся отдельно: у них свои таблицы
			s.Policy.LearnScoped(name, addrs, time.Duration(ttl)*time.Second)
		}
		return resp
	}
	if lastErr != nil {
		s.logf("DNS %s (%s): %v", name, r.Action, lastErr)
	}
	return dnswire.ErrorReply(query, dnswire.RcodeServFail)
}

// ListenAndServe слушает UDP и TCP до отмены ctx.
func (s *Server) ListenAndServe(ctx context.Context) error {
	s.init()
	pc, err := net.ListenPacket("udp", s.Listen)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", s.Listen)
	if err != nil {
		_ = pc.Close()
		return err
	}
	go func() {
		<-ctx.Done()
		_ = pc.Close()
		_ = ln.Close()
	}()
	go s.serveTCP(ctx, ln)
	s.serveUDP(ctx, pc)
	return nil
}

func (s *Server) serveUDP(ctx context.Context, pc net.PacketConn) {
	buf := make([]byte, 4096)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		ua, ok := addr.(*net.UDPAddr)
		if !ok || !s.allowed(ua.AddrPort().Addr()) || n < 12 {
			continue
		}
		q := append([]byte(nil), buf[:n]...)
		select {
		case s.sem <- struct{}{}:
		default:
			continue // перегрузка: клиент повторит
		}
		go func() {
			defer func() { <-s.sem }()
			resp := s.Handle(ctx, q)
			if len(resp) > 512 && !hasEDNS(q) {
				resp = truncated(q)
			}
			_, _ = pc.WriteTo(resp, addr)
		}()
	}
}

// hasEDNS — в запросе есть OPT-запись (клиент принимает ответы длиннее 512 байт).
func hasEDNS(q []byte) bool { return len(q) >= 12 && binary.BigEndian.Uint16(q[10:12]) > 0 }

// truncated — ответ с флагом TC: клиент повторит запрос по TCP.
func truncated(q []byte) []byte {
	r := dnswire.ErrorReply(q, 0)
	if len(r) >= 3 {
		r[2] |= 0x02
	}
	return r
}

func (s *Server) serveTCP(ctx context.Context, ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		ta, ok := c.RemoteAddr().(*net.TCPAddr)
		if !ok || !s.allowed(ta.AddrPort().Addr()) {
			_ = c.Close()
			continue
		}
		select {
		case s.sem <- struct{}{}:
		default:
			_ = c.Close()
			continue
		}
		go func() {
			defer func() { <-s.sem }()
			defer c.Close()
			for i := 0; i < 16; i++ {
				_ = c.SetDeadline(time.Now().Add(10 * time.Second))
				q, err := readTCP(c)
				if err != nil {
					return
				}
				if err := writeTCP(c, s.Handle(ctx, q)); err != nil {
					return
				}
			}
		}()
	}
}

func readTCP(r io.Reader) ([]byte, error) {
	var l [2]byte
	if _, err := io.ReadFull(r, l[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(l[:]))
	if n < 12 {
		return nil, errors.New("короткое сообщение")
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}

func writeTCP(w io.Writer, msg []byte) error {
	if len(msg) > 0xFFFF {
		return errors.New("слишком длинный ответ")
	}
	b := make([]byte, 2+len(msg))
	binary.BigEndian.PutUint16(b, uint16(len(msg)))
	copy(b[2:], msg)
	_, err := w.Write(b)
	return err
}

// --- обмен с серверами ---

func dialer(mark int) *net.Dialer {
	d := &net.Dialer{Timeout: 4 * time.Second}
	if mark != 0 {
		d.Control = func(_, _ string, c syscall.RawConn) error {
			var serr error
			if err := c.Control(func(fd uintptr) { serr = setMark(int(fd), mark) }); err != nil {
				return err
			}
			return serr
		}
	}
	return d
}

func (s *Server) exchange(ctx context.Context, q []byte, up Upstream, mark int) ([]byte, error) {
	if up.DoH != nil {
		return s.doh.exchange(ctx, q, up.DoH, mark)
	}
	resp, err := exchangeUDP(ctx, q, up.Addr, mark)
	if err == nil && len(resp) > 2 && resp[2]&0x02 != 0 { // TC: повтор по TCP
		return exchangeTCP(ctx, q, up.Addr, mark)
	}
	return resp, err
}

func exchangeUDP(ctx context.Context, q []byte, addr string, mark int) ([]byte, error) {
	c, err := dialer(mark).DialContext(ctx, "udp", addr)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	if _, err := c.Write(q); err != nil {
		return nil, err
	}
	buf := make([]byte, 65535)
	for {
		n, err := c.Read(buf)
		if err != nil {
			return nil, err
		}
		if dnswire.IsResponseTo(buf[:n], dnswire.ID(q)) {
			return append([]byte(nil), buf[:n]...), nil
		}
	}
}

func exchangeTCP(ctx context.Context, q []byte, addr string, mark int) ([]byte, error) {
	c, err := dialer(mark).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = c.SetDeadline(dl)
	}
	if err := writeTCP(c, q); err != nil {
		return nil, err
	}
	return readTCP(c)
}

// dohClients — по клиенту на метку (у каждого свои соединения).
type dohClients struct {
	tls   *tls.Config
	mu    sync.Mutex
	m     map[int]*http.Client
	cache sync.Map // хост -> resolved
}

type resolved struct {
	addr    string
	expires time.Time
}

func (d *dohClients) client(mark int) *http.Client {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c := d.m[mark]; c != nil {
		return c
	}
	dl := dialer(mark)
	tr := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ip, err := d.resolve(ctx, host, mark)
			if err != nil {
				return nil, err
			}
			return dl.DialContext(ctx, network, net.JoinHostPort(ip, port))
		},
		TLSClientConfig:     d.tlsConfig(),
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        8,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 4 * time.Second,
	}
	c := &http.Client{Transport: tr, Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	d.m[mark] = c
	return c
}

func (d *dohClients) tlsConfig() *tls.Config {
	if d.tls != nil {
		return d.tls.Clone()
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}
}

// resolve находит адрес DoH-сервера: IP — как есть, имя — запросом A к 1.1.1.1
// через тот же путь (та же метка), без системного резолвера.
func (d *dohClients) resolve(ctx context.Context, host string, mark int) (string, error) {
	if a, err := netip.ParseAddr(host); err == nil {
		return a.String(), nil
	}
	key := host + "|" + strconv.Itoa(mark)
	if v, ok := d.cache.Load(key); ok && v.(resolved).expires.After(time.Now()) {
		return v.(resolved).addr, nil
	}
	q, ok := dnswire.Query(uint16(time.Now().UnixNano()), host, dnswire.TypeA)
	if !ok {
		return "", fmt.Errorf("недопустимое имя DoH-сервера %q", host)
	}
	var lastErr error
	for _, boot := range []string{"1.1.1.1:53", "8.8.8.8:53"} {
		resp, err := exchangeUDP(ctx, q, boot, mark)
		if err != nil {
			lastErr = err
			continue
		}
		for _, a := range dnswire.Answers(resp) {
			if a.Addr.Is4() {
				d.cache.Store(key, resolved{a.Addr.String(), time.Now().Add(10 * time.Minute)})
				return a.Addr.String(), nil
			}
		}
		lastErr = fmt.Errorf("нет адреса для %s", host)
	}
	return "", lastErr
}

func (d *dohClients) exchange(ctx context.Context, q []byte, u *url.URL, mark int) ([]byte, error) {
	// RFC 8484: ID = 0 повышает попадания в кэш; вернём исходный ID в ответе
	id := dnswire.ID(q)
	body := append([]byte(nil), q...)
	body[0], body[1] = 0, 0
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/dns-message")
	req.Header.Set("Accept", "application/dns-message")
	resp, err := d.client(mark).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DoH: ответ %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/dns-message") {
		return nil, fmt.Errorf("DoH: неожиданный тип %q", ct)
	}
	out, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return nil, err
	}
	if len(out) < 12 {
		return nil, errors.New("DoH: короткий ответ")
	}
	binary.BigEndian.PutUint16(out[0:2], id)
	return out, nil
}
