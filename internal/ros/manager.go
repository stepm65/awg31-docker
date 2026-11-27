package ros

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Validate проверяет настройки интеграции (пароль может быть пустым: тогда
// сохраняется прежний).
func Validate(r store.Router) []string {
	var errs []string
	switch r.Mode {
	case store.RouterOff, store.RouterAddressList, store.RouterBGP:
	default:
		errs = append(errs, "режим: off, address_list или bgp")
	}
	if r.Mode == store.RouterOff || len(errs) > 0 {
		return errs
	}
	if _, err := ValidateURL(r.URL); err != nil {
		errs = append(errs, err.Error())
	}
	if !reName.MatchString(r.User) {
		errs = append(errs, "пользователь RouterOS: латиница, цифры, _ . -")
	}
	if !reName.MatchString(r.AddressList) {
		errs = append(errs, "имя address-list: латиница, цифры, _ . -")
	}
	if r.ForwardTo != "" {
		if a, err := netip.ParseAddr(r.ForwardTo); err != nil || !a.Is4() {
			errs = append(errs, "IP контейнера: IPv4-адрес")
		}
	}
	if r.StaticLimit < 0 || r.StaticLimit > 50000 {
		errs = append(errs, "предел сетей: 0–50000")
	}
	if r.DynamicMin < 5 || r.DynamicMin > 1440 {
		errs = append(errs, "срок выученных адресов: 5–1440 минут")
	}
	if r.DomainLimit < 0 || r.DomainLimit > 5000 {
		errs = append(errs, "предел доменов: 0–5000")
	}
	if r.Mode == store.RouterBGP {
		if r.BGPLocalAS == 0 || r.BGPRemoteAS == 0 {
			errs = append(errs, "BGP: укажите оба номера AS")
		}
		if r.BGPPeer != "" {
			if a, err := netip.ParseAddr(r.BGPPeer); err != nil || !a.Is4() {
				errs = append(errs, "BGP: адрес роутера — IPv4")
			}
		}
	}
	return errs
}

var reName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,64}$`)

// Status — состояние интеграции для UI.
type Status struct {
	Mode       string    `json:"mode"`
	Router     string    `json:"router,omitempty"` // имя и версия
	LastSync   time.Time `json:"last_sync,omitempty"`
	LastError  string    `json:"last_error,omitempty"`
	Static     int       `json:"static"`
	Dynamic    int       `json:"dynamic"`
	Domains    int       `json:"domains"`
	Dropped    int       `json:"dropped,omitempty"`
	Added      int       `json:"added"`
	Removed    int       `json:"removed"`
	ForwardTo  string    `json:"forward_to,omitempty"`
	BGP        BGPStatus `json:"bgp"`
	ContainerV string    `json:"container_ip,omitempty"`
}

// Manager — синхронизация с роутером.
type Manager struct {
	Store   *store.Store
	Policy  *policy.Engine
	Tunnels interface{ Options() tunnel.Options }
	Runner  tunnel.Runner
	Logf    func(format string, args ...any)
	// NewClient — подменяется в тестах.
	NewClient func(Conn) (*Client, error)

	bgp      Speaker
	mu       sync.Mutex
	syncMu   sync.Mutex
	status   Status
	prevMode string
	prevList string
	prevConn Conn
	wake     chan struct{}
	once     sync.Once
}

func (m *Manager) init() {
	m.once.Do(func() {
		m.wake = make(chan struct{}, 1)
		m.bgp.Logf = m.Logf
		if m.NewClient == nil {
			m.NewClient = NewClient
		}
	})
}

func (m *Manager) logf(f string, a ...any) {
	if m.Logf != nil {
		m.Logf(f, a...)
	}
}

// Wake просит синхронизировать при ближайшей возможности.
func (m *Manager) Wake() {
	m.init()
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Status возвращает состояние.
func (m *Manager) Status() Status {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.status
	st.BGP = m.bgp.Status()
	return st
}

func connOf(r store.Router) Conn {
	return Conn{URL: r.URL, User: r.User, Pass: r.Pass, PinSHA256: r.PinSHA256, Insecure: r.Insecure}
}

// Test проверяет подключение к роутеру.
func (m *Manager) Test(ctx context.Context, r store.Router) (string, error) {
	m.init()
	c, err := m.NewClient(connOf(r))
	if err != nil {
		return "", err
	}
	name, ver, err := c.Identity(ctx)
	if err != nil {
		return "", err
	}
	// права на запись проверятся при первой синхронизации; чтение списка — сейчас
	if _, err := c.Print(ctx, ListPath(false), map[string]string{"list": r.AddressList}); err != nil {
		return "", err
	}
	return strings.TrimSpace(name + " · RouterOS " + ver), nil
}

// containerIP — IPv4 контейнера на LAN-интерфейсе.
func (m *Manager) containerIP(ctx context.Context, r store.Router) (netip.Addr, error) {
	if r.ForwardTo != "" {
		return netip.ParseAddr(r.ForwardTo)
	}
	lan := m.Tunnels.Options().LanIface
	if lan == "" || m.Runner == nil {
		return netip.Addr{}, errors.New("LAN-интерфейс не найден: укажите IP контейнера")
	}
	out, err := m.Runner.Run(ctx, "ip", "-o", "-4", "addr", "show", "dev", lan)
	if err != nil {
		return netip.Addr{}, err
	}
	f := strings.Fields(string(out))
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "inet" {
			if p, err := netip.ParsePrefix(f[i+1]); err == nil {
				return p.Addr(), nil
			}
		}
	}
	return netip.Addr{}, fmt.Errorf("нет IPv4 на %s: укажите IP контейнера", lan)
}

func (m *Manager) gatewayIP(ctx context.Context) netip.Addr {
	lan := m.Tunnels.Options().LanIface
	if lan == "" || m.Runner == nil {
		return netip.Addr{}
	}
	out, err := m.Runner.Run(ctx, "ip", "-o", "-4", "route", "show", "default", "dev", lan)
	if err != nil {
		return netip.Addr{}
	}
	f := strings.Fields(string(out))
	for i := 0; i+1 < len(f); i++ {
		if f[i] == "via" {
			a, _ := netip.ParseAddr(f[i+1])
			return a
		}
	}
	return netip.Addr{}
}

// Sync выполняет одну синхронизацию с роутером.
func (m *Manager) Sync(ctx context.Context) error {
	m.init()
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	r := m.Store.Snapshot().RouterSettings()
	st := Status{Mode: r.Mode}
	err := m.sync(ctx, r, &st)
	st.LastSync = time.Now()
	if err != nil {
		st.LastError = tunnel.Redact(err.Error())
		m.logf("RouterOS: %v", err)
	}
	m.mu.Lock()
	if st.Router == "" {
		st.Router = m.status.Router
	}
	m.status = st
	m.mu.Unlock()
	return err
}

func (m *Manager) sync(ctx context.Context, r store.Router, st *Status) error {
	prevMode, prevList, prevConn := m.prevMode, m.prevList, m.prevConn
	// смена режима или списка: убрать записи прежнего режима
	if prevMode == store.RouterAddressList && (r.Mode != store.RouterAddressList || r.AddressList != prevList) {
		if c, err := m.NewClient(prevConn); err == nil {
			n, err := Cleanup(ctx, c, prevList)
			m.logf("RouterOS: previous mode switched off, entries removed: %d", n)
			if err != nil {
				return fmt.Errorf("очистка прежних записей: %w", err)
			}
		}
	}
	if prevMode == store.RouterBGP && r.Mode != store.RouterBGP {
		m.bgp.Close()
	}
	if r.Mode == store.RouterOff {
		if prevMode == store.RouterBGP || r.SyncDNS {
			// записи DNS FWD при выключении тоже убираются
			if c, err := m.NewClient(connOf(r)); err == nil && r.URL != "" && r.User != "" {
				_, _ = SyncDNSForward(ctx, c, "", nil)
			}
		}
		m.prevMode, m.prevList = r.Mode, r.AddressList
		return nil
	}
	o := m.Tunnels.Options()
	if !o.AutoRoute || o.LanIface == "" {
		return errors.New("интеграция работает только в режиме шлюза")
	}
	c, err := m.NewClient(connOf(r))
	if err != nil {
		return err
	}
	if name, ver, err := c.Identity(ctx); err == nil {
		st.Router = strings.TrimSpace(name + " · RouterOS " + ver)
	} else {
		return err
	}
	ex := m.Policy.Export(r.StaticLimit, r.DomainLimit)
	st.Static, st.Dropped = len(ex.Prefixes), ex.Dropped
	cip, cerr := m.containerIP(ctx, r)
	if cerr == nil {
		st.ContainerV = cip.String()
	}

	switch r.Mode {
	case store.RouterAddressList:
		dyn := time.Duration(r.DynamicMin) * time.Minute
		desired := map[bool]map[string]Entry{false: {}, true: {}}
		for _, p := range ex.Prefixes {
			desired[p.Addr().Is6()][AddrKey(p)] = Entry{Comment: Marker + " static"}
		}
		now := time.Now()
		for _, l := range ex.Learned {
			p := netip.PrefixFrom(l.Addr, l.Addr.BitLen())
			key := AddrKey(p)
			if _, static := desired[l.Addr.Is6()][key]; static {
				continue
			}
			ttl := min(l.Expires.Sub(now), dyn)
			if ttl < time.Minute {
				continue
			}
			desired[l.Addr.Is6()][key] = Entry{Comment: Marker + " dns " + l.By, Timeout: ttl}
			st.Dynamic++
		}
		for _, v6 := range []bool{false, true} {
			res, err := SyncAddressList(ctx, c, v6, r.AddressList, desired[v6])
			st.Added += res.Added
			st.Removed += res.Removed
			if err != nil {
				if v6 && strings.Contains(err.Error(), "404") {
					continue // пакет ipv6 выключен на роутере
				}
				return err
			}
		}
	case store.RouterBGP:
		if cerr != nil {
			return cerr
		}
		peer := m.gatewayIP(ctx)
		if r.BGPPeer != "" {
			peer, _ = netip.ParseAddr(r.BGPPeer)
		}
		if !peer.IsValid() {
			return errors.New("BGP: не найден адрес роутера, укажите его")
		}
		var list []netip.Prefix
		list = append(list, ex.Prefixes...)
		for _, l := range ex.Learned {
			list = append(list, netip.PrefixFrom(l.Addr, l.Addr.BitLen()))
			st.Dynamic++
		}
		if err := m.bgp.Configure(&BGPConfig{Peer: netip.AddrPortFrom(peer, 179), LocalAS: r.BGPLocalAS, RemoteAS: r.BGPRemoteAS,
			RouterID: cip, NextHop: cip}); err != nil {
			return err
		}
		m.bgp.SetPrefixes(list)
	}

	if r.SyncDNS {
		if cerr != nil {
			return cerr
		}
		st.ForwardTo = cip.String()
		res, err := SyncDNSForward(ctx, c, cip.String(), ex.Domains)
		st.Domains = res.Added + res.Kept + res.Refreshed
		st.Added += res.Added
		st.Removed += res.Removed
		if err != nil {
			return err
		}
	} else if prevMode != "" {
		if _, err := SyncDNSForward(ctx, c, "", nil); err != nil {
			return err
		}
	}
	m.prevMode, m.prevList, m.prevConn = r.Mode, r.AddressList, connOf(r)
	return nil
}

// Run синхронизирует при изменениях политики (не чаще раза в 10 секунд) и
// каждые 5 минут.
func (m *Manager) Run(ctx context.Context) {
	m.init()
	if m.Policy != nil {
		m.Policy.OnChange(m.Wake)
	}
	defer m.bgp.Close()
	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()
	first := time.After(15 * time.Second)
	last := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-first:
		case <-tick.C:
		case <-m.wake:
			if wait := 10*time.Second - time.Since(last); wait > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(wait):
				}
			}
		}
		if m.Store.Snapshot().RouterSettings().Mode == store.RouterOff && m.prevMode == "" {
			continue
		}
		last = time.Now()
		sctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		_ = m.Sync(sctx)
		cancel()
	}
}

// Lease — аренда DHCP роутера (для справочника устройств).
type Lease struct {
	Addr     string `json:"addr"`
	MAC      string `json:"mac,omitempty"`
	HostName string `json:"host_name,omitempty"`
	Comment  string `json:"comment,omitempty"`
	Dynamic  bool   `json:"dynamic"`
}

// Leases читает аренды DHCP-сервера роутера (/ip/dhcp-server/lease). Нужны права
// на чтение; интеграция при этом может быть выключена, важно лишь подключение.
func (m *Manager) Leases(ctx context.Context, r store.Router) ([]Lease, error) {
	m.init()
	c, err := m.NewClient(connOf(r))
	if err != nil {
		return nil, err
	}
	rows, err := c.Print(ctx, "/ip/dhcp-server/lease", nil)
	if err != nil {
		return nil, err
	}
	out := make([]Lease, 0, len(rows))
	for _, row := range rows {
		a, err := netip.ParseAddr(row["address"])
		if err != nil {
			continue
		}
		out = append(out, Lease{Addr: a.String(), MAC: row["mac-address"], HostName: row["host-name"], Comment: row["comment"], Dynamic: row["dynamic"] == "true"})
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := netip.ParseAddr(out[i].Addr)
		b, _ := netip.ParseAddr(out[j].Addr)
		return a.Less(b)
	})
	return out, nil
}
