package ros

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sort"
	"sync"
	"time"
)

// Минимальный BGP-4 спикер (RFC 4271, AS4 — RFC 6793): только анонсирует
// IPv4-сети роутеру, маршруты от роутера не принимает (UPDATE от него
// игнорируются). Контейнер сам подключается к роутеру на порт 179.

const (
	bgpOpen         = 1
	bgpUpdate       = 2
	bgpNotification = 3
	bgpKeepalive    = 4
	bgpMaxMsg       = 4096
	asTrans         = 23456
)

// BGPConfig — параметры сессии.
type BGPConfig struct {
	Peer     netip.AddrPort // роутер, обычно шлюз LAN:179
	LocalAS  uint32
	RemoteAS uint32
	RouterID netip.Addr // IPv4 контейнера
	NextHop  netip.Addr // IPv4 контейнера
	HoldTime uint16     // 0 — 90 секунд
}

func (c BGPConfig) valid() error {
	switch {
	case !c.Peer.IsValid() || !c.Peer.Addr().Is4():
		return errors.New("BGP: адрес роутера — IPv4")
	case c.LocalAS == 0 || c.RemoteAS == 0:
		return errors.New("BGP: укажите номера AS")
	case !c.RouterID.Is4() || !c.NextHop.Is4():
		return errors.New("BGP: нет IPv4-адреса контейнера")
	}
	return nil
}

// BGPStatus — состояние для UI.
type BGPStatus struct {
	State     string    `json:"state"` // idle | connecting | established | error
	Announced int       `json:"announced"`
	Since     time.Time `json:"since,omitempty"`
	LastError string    `json:"last_error,omitempty"`
}

// Speaker — BGP-спикер с переподключением.
type Speaker struct {
	Logf func(format string, args ...any)
	Dial func(ctx context.Context, addr string) (net.Conn, error)

	mu      sync.Mutex
	cfg     *BGPConfig
	cancel  context.CancelFunc
	done    chan struct{}
	desired map[netip.Prefix]bool
	notify  chan struct{}
	status  BGPStatus
}

func (s *Speaker) logf(f string, a ...any) {
	if s.Logf != nil {
		s.Logf(f, a...)
	}
}

// Configure запускает, перезапускает (при изменении) или останавливает (nil) сессию.
func (s *Speaker) Configure(cfg *BGPConfig) error {
	if cfg != nil {
		if err := cfg.valid(); err != nil {
			return err
		}
	}
	s.mu.Lock()
	if s.notify == nil {
		s.notify = make(chan struct{}, 1)
	}
	same := (cfg == nil && s.cfg == nil) || (cfg != nil && s.cfg != nil && *cfg == *s.cfg)
	if same {
		s.mu.Unlock()
		return nil
	}
	cancel, done := s.cancel, s.done
	s.cfg, s.cancel, s.done = nil, nil, nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		<-done
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if cfg == nil {
		s.status = BGPStatus{State: "idle"}
		return nil
	}
	c := *cfg
	ctx, cancelFn := context.WithCancel(context.Background())
	s.cfg, s.cancel, s.done = &c, cancelFn, make(chan struct{})
	go s.run(ctx, c, s.done)
	return nil
}

// Close останавливает спикер.
func (s *Speaker) Close() { _ = s.Configure(nil) }

// SetPrefixes задаёт анонсируемые сети (IPv6 пропускаются).
func (s *Speaker) SetPrefixes(list []netip.Prefix) {
	m := map[netip.Prefix]bool{}
	for _, p := range list {
		if p.Addr().Is4() && p.Bits() > 0 {
			m[p.Masked()] = true
		}
	}
	s.mu.Lock()
	s.desired = m
	if s.notify == nil {
		s.notify = make(chan struct{}, 1)
	}
	ch := s.notify
	s.mu.Unlock()
	select {
	case ch <- struct{}{}:
	default:
	}
}

// Status возвращает состояние.
func (s *Speaker) Status() BGPStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status
	if st.State == "" {
		st.State = "idle"
	}
	return st
}

func (s *Speaker) setStatus(f func(*BGPStatus)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.status)
}

func (s *Speaker) run(ctx context.Context, cfg BGPConfig, done chan struct{}) {
	defer close(done)
	backoff := 5 * time.Second
	for ctx.Err() == nil {
		s.setStatus(func(st *BGPStatus) { st.State = "connecting" })
		err := s.session(ctx, cfg)
		if ctx.Err() != nil {
			return
		}
		s.setStatus(func(st *BGPStatus) { st.State, st.LastError, st.Announced = "error", err.Error(), 0 })
		s.logf("BGP: %v, повтор через %s", err, backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
}

type bgpMsg struct {
	typ  byte
	body []byte
	err  error
}

func (s *Speaker) session(ctx context.Context, cfg BGPConfig) error {
	dial := s.Dial
	if dial == nil {
		dial = func(ctx context.Context, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, "tcp", addr)
		}
	}
	conn, err := dial(ctx, cfg.Peer.String())
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() { <-ctx.Done(); conn.Close() }()

	hold := cfg.HoldTime
	if hold == 0 {
		hold = 90
	}
	if _, err := conn.Write(BuildOpen(cfg.LocalAS, hold, cfg.RouterID)); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	typ, body, err := ReadMessage(conn)
	if err != nil {
		return fmt.Errorf("ожидался OPEN: %w", err)
	}
	if typ == bgpNotification {
		return notificationErr(body)
	}
	if typ != bgpOpen {
		return fmt.Errorf("ожидался OPEN, получен тип %d", typ)
	}
	peer, err := ParseOpen(body)
	if err != nil {
		return err
	}
	if peer.AS != cfg.RemoteAS {
		_, _ = conn.Write(frame(bgpNotification, []byte{2, 2})) // Bad Peer AS
		return fmt.Errorf("роутер представился AS %d, ожидался %d", peer.AS, cfg.RemoteAS)
	}
	if peer.Hold != 0 && peer.Hold < hold {
		hold = peer.Hold
	}
	if _, err := conn.Write(frame(bgpKeepalive, nil)); err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Time{})

	msgs := make(chan bgpMsg, 8)
	quit := make(chan struct{})
	defer close(quit)
	go func() {
		for {
			if hold > 0 {
				_ = conn.SetReadDeadline(time.Now().Add(time.Duration(hold) * time.Second))
			}
			t, b, err := ReadMessage(conn)
			select {
			case msgs <- bgpMsg{t, b, err}:
			case <-quit:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	established := false
	sent := map[netip.Prefix]bool{}
	ibgp := cfg.LocalAS == cfg.RemoteAS
	keep := time.NewTicker(time.Duration(max(hold/3, 1)) * time.Second)
	defer keep.Stop()
	s.mu.Lock()
	notify := s.notify
	s.mu.Unlock()
	push := func() error {
		s.mu.Lock()
		want := make(map[netip.Prefix]bool, len(s.desired))
		for p := range s.desired {
			want[p] = true
		}
		s.mu.Unlock()
		var add, del []netip.Prefix
		for p := range want {
			if !sent[p] {
				add = append(add, p)
			}
		}
		for p := range sent {
			if !want[p] {
				del = append(del, p)
			}
		}
		for _, m := range BuildUpdates(add, del, cfg.LocalAS, cfg.NextHop, peer.AS4, ibgp) {
			if _, err := conn.Write(m); err != nil {
				return err
			}
		}
		for _, p := range add {
			sent[p] = true
		}
		for _, p := range del {
			delete(sent, p)
		}
		s.setStatus(func(st *BGPStatus) { st.Announced = len(sent) })
		return nil
	}
	for {
		select {
		case <-ctx.Done():
			_, _ = conn.Write(frame(bgpNotification, []byte{6, 2})) // Cease: Administrative Shutdown
			return ctx.Err()
		case m := <-msgs:
			if m.err != nil {
				if errors.Is(m.err, io.EOF) {
					return errors.New("роутер закрыл соединение")
				}
				return m.err
			}
			switch m.typ {
			case bgpNotification:
				return notificationErr(m.body)
			case bgpKeepalive:
				if !established {
					established = true
					s.setStatus(func(st *BGPStatus) { st.State, st.Since, st.LastError = "established", time.Now(), "" })
					s.logf("BGP: сессия с %s установлена (AS %d)", cfg.Peer, peer.AS)
					if err := push(); err != nil {
						return err
					}
				}
			case bgpUpdate:
				// маршруты от роутера не нужны
			}
		case <-notify:
			if established {
				if err := push(); err != nil {
					return err
				}
			}
		case <-keep.C:
			if _, err := conn.Write(frame(bgpKeepalive, nil)); err != nil {
				return err
			}
		}
	}
}

func notificationErr(b []byte) error {
	if len(b) < 2 {
		return errors.New("роутер прислал NOTIFICATION")
	}
	return fmt.Errorf("роутер прислал NOTIFICATION %d/%d", b[0], b[1])
}

func frame(typ byte, body []byte) []byte {
	m := make([]byte, 19+len(body))
	for i := 0; i < 16; i++ {
		m[i] = 0xFF
	}
	binary.BigEndian.PutUint16(m[16:18], uint16(19+len(body)))
	m[18] = typ
	copy(m[19:], body)
	return m
}

// ReadMessage читает одно сообщение BGP.
func ReadMessage(r io.Reader) (byte, []byte, error) {
	var h [19]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return 0, nil, err
	}
	for i := 0; i < 16; i++ {
		if h[i] != 0xFF {
			return 0, nil, errors.New("BGP: неверный маркер")
		}
	}
	n := int(binary.BigEndian.Uint16(h[16:18]))
	if n < 19 || n > bgpMaxMsg {
		return 0, nil, fmt.Errorf("BGP: неверная длина %d", n)
	}
	body := make([]byte, n-19)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return h[18], body, nil
}

// BuildOpen — OPEN с возможностями «IPv4 unicast» и «4-байтовые AS».
func BuildOpen(as uint32, hold uint16, id netip.Addr) []byte {
	short := uint16(as)
	if as > 0xFFFF {
		short = asTrans
	}
	caps := []byte{1, 4, 0, 1, 0, 1} // MP: AFI 1 (IPv4), SAFI 1 (unicast)
	caps = append(caps, 65, 4)
	caps = binary.BigEndian.AppendUint32(caps, as)
	b := []byte{4}
	b = binary.BigEndian.AppendUint16(b, short)
	b = binary.BigEndian.AppendUint16(b, hold)
	b = append(b, id.AsSlice()...)
	b = append(b, byte(2+len(caps)), 2, byte(len(caps)))
	b = append(b, caps...)
	return frame(bgpOpen, b)
}

// PeerOpen — разобранный OPEN роутера.
type PeerOpen struct {
	AS   uint32
	Hold uint16
	AS4  bool
}

// ParseOpen разбирает тело OPEN.
func ParseOpen(b []byte) (PeerOpen, error) {
	var p PeerOpen
	if len(b) < 10 || b[0] != 4 {
		return p, errors.New("BGP: неверный OPEN (нужна версия 4)")
	}
	p.AS = uint32(binary.BigEndian.Uint16(b[1:3]))
	p.Hold = binary.BigEndian.Uint16(b[3:5])
	if p.Hold == 1 || p.Hold == 2 {
		return p, errors.New("BGP: недопустимое время удержания")
	}
	optLen := int(b[9])
	opts := b[10:]
	if optLen > len(opts) {
		return p, errors.New("BGP: обрезанный OPEN")
	}
	opts = opts[:optLen]
	for len(opts) >= 2 {
		t, l := opts[0], int(opts[1])
		if 2+l > len(opts) {
			return p, errors.New("BGP: обрезанный параметр OPEN")
		}
		v := opts[2 : 2+l]
		opts = opts[2+l:]
		if t != 2 {
			continue
		}
		for len(v) >= 2 {
			code, cl := v[0], int(v[1])
			if 2+cl > len(v) {
				break
			}
			if code == 65 && cl == 4 {
				p.AS = binary.BigEndian.Uint32(v[2:6])
				p.AS4 = true
			}
			v = v[2+cl:]
		}
	}
	return p, nil
}

func encodePrefix(p netip.Prefix) []byte {
	a := p.Masked().Addr().As4()
	n := (p.Bits() + 7) / 8
	return append([]byte{byte(p.Bits())}, a[:n]...)
}

func pathAttrs(as uint32, nh netip.Addr, as4, ibgp bool) []byte {
	var b []byte
	b = append(b, 0x40, 1, 1, 0) // ORIGIN IGP
	if ibgp {
		b = append(b, 0x40, 2, 0) // пустой AS_PATH
	} else if as4 {
		b = append(b, 0x40, 2, 6, 2, 1)
		b = binary.BigEndian.AppendUint32(b, as)
	} else {
		short := uint16(as)
		if as > 0xFFFF {
			short = asTrans
		}
		b = append(b, 0x40, 2, 4, 2, 1)
		b = binary.BigEndian.AppendUint16(b, short)
	}
	n4 := nh.As4()
	b = append(b, 0x40, 3, 4)
	b = append(b, n4[:]...)
	if ibgp {
		b = append(b, 0x40, 5, 4, 0, 0, 0, 100) // LOCAL_PREF 100
	}
	return b
}

// BuildUpdates — сообщения UPDATE: отзывы и анонсы, разбитые по 4096 байт.
func BuildUpdates(add, del []netip.Prefix, as uint32, nh netip.Addr, as4, ibgp bool) [][]byte {
	sortP := func(l []netip.Prefix) {
		sort.Slice(l, func(i, j int) bool {
			if c := l[i].Addr().Compare(l[j].Addr()); c != 0 {
				return c < 0
			}
			return l[i].Bits() < l[j].Bits()
		})
	}
	sortP(add)
	sortP(del)
	var out [][]byte
	const room = bgpMaxMsg - 19 - 4 - 64 // заголовок, две длины, атрибуты
	for len(del) > 0 {
		var w []byte
		for len(del) > 0 && len(w)+5 <= room {
			w = append(w, encodePrefix(del[0])...)
			del = del[1:]
		}
		b := binary.BigEndian.AppendUint16(nil, uint16(len(w)))
		b = append(b, w...)
		b = append(b, 0, 0)
		out = append(out, frame(bgpUpdate, b))
	}
	attrs := pathAttrs(as, nh, as4, ibgp)
	for len(add) > 0 {
		var nlri []byte
		for len(add) > 0 && len(nlri)+5 <= room {
			nlri = append(nlri, encodePrefix(add[0])...)
			add = add[1:]
		}
		b := []byte{0, 0}
		b = binary.BigEndian.AppendUint16(b, uint16(len(attrs)))
		b = append(b, attrs...)
		b = append(b, nlri...)
		out = append(out, frame(bgpUpdate, b))
	}
	return out
}

// ParseUpdate разбирает UPDATE (для тестов и журнала): отзывы и анонсы IPv4.
func ParseUpdate(b []byte) (withdrawn, nlri []netip.Prefix, attrs []byte, err error) {
	rd := func(buf []byte) ([]netip.Prefix, error) {
		var out []netip.Prefix
		for len(buf) > 0 {
			bits := int(buf[0])
			n := (bits + 7) / 8
			if bits > 32 || 1+n > len(buf) {
				return nil, errors.New("BGP: неверный префикс")
			}
			var a [4]byte
			copy(a[:], buf[1:1+n])
			out = append(out, netip.PrefixFrom(netip.AddrFrom4(a), bits))
			buf = buf[1+n:]
		}
		return out, nil
	}
	if len(b) < 4 {
		return nil, nil, nil, errors.New("BGP: короткий UPDATE")
	}
	wl := int(binary.BigEndian.Uint16(b[0:2]))
	if 2+wl+2 > len(b) {
		return nil, nil, nil, errors.New("BGP: обрезанный UPDATE")
	}
	if withdrawn, err = rd(b[2 : 2+wl]); err != nil {
		return
	}
	al := int(binary.BigEndian.Uint16(b[2+wl : 4+wl]))
	if 4+wl+al > len(b) {
		return nil, nil, nil, errors.New("BGP: обрезанные атрибуты")
	}
	attrs = b[4+wl : 4+wl+al]
	nlri, err = rd(b[4+wl+al:])
	return
}
