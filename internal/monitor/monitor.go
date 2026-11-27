// Package monitor — мониторинг шлюза: история трафика, состояние системы и
// соединения клиентов LAN из conntrack, разложенные по устройствам, правилам и
// туннелям (страницы «Обзор» и «Мониторинг»).
package monitor

import (
	"bufio"
	"bytes"
	"context"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// Период замера и глубина истории: 5 с × 720 = 1 час.
const (
	Step     = 5 * time.Second
	HistSize = 720
	MaxConns = 300 // соединений в ответе (самые крупные)
)

// Flow — соединение из conntrack (направление «оригинал»: клиент -> сервер).
type Flow struct {
	Proto string
	Src   netip.Addr
	Dst   netip.Addr
	Sport uint16
	Dport uint16
	Bytes uint64 // в обе стороны; 0, если учёт байтов в conntrack выключен
}

// ParseConntrack разбирает /proc/net/nf_conntrack или вывод «conntrack -L -o extended».
func ParseConntrack(data []byte) []Flow {
	var out []Flow
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) < 6 {
			continue
		}
		// «ipv4 2 tcp 6 431999 ESTABLISHED src=…» — протокол третьим полем
		proto := f[2]
		if proto != "tcp" && proto != "udp" && proto != "icmp" && proto != "icmpv6" {
			proto = "other"
		}
		var fl Flow
		fl.Proto = proto
		seen := map[string]int{}
		for _, kv := range f[3:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				continue
			}
			seen[k]++
			first := seen[k] == 1 // первый набор полей — оригинал, второй — ответ
			switch k {
			case "src":
				if first {
					fl.Src, _ = netip.ParseAddr(v)
				}
			case "dst":
				if first {
					fl.Dst, _ = netip.ParseAddr(v)
				}
			case "sport":
				if first {
					n, _ := strconv.Atoi(v)
					fl.Sport = uint16(n)
				}
			case "dport":
				if first {
					n, _ := strconv.Atoi(v)
					fl.Dport = uint16(n)
				}
			case "bytes":
				n, _ := strconv.ParseUint(v, 10, 64)
				fl.Bytes += n
			}
		}
		if fl.Src.IsValid() && fl.Dst.IsValid() {
			fl.Src, fl.Dst = fl.Src.Unmap(), fl.Dst.Unmap()
			out = append(out, fl)
		}
	}
	return out
}

// Point — замер трафика (байт/с). Down/Up — весь трафик клиентов через шлюз,
// VDown/VUp — через туннели.
type Point struct {
	T     int64   `json:"t"`
	Down  float64 `json:"down"`
	Up    float64 `json:"up"`
	VDown float64 `json:"vdown"`
	VUp   float64 `json:"vup"`
}

// System — состояние контейнера.
type System struct {
	CPU       float64 `json:"cpu"`       // загрузка процессора, %
	MemRSS    uint64  `json:"mem_rss"`   // память awgd
	MemCgroup uint64  `json:"mem_group"` // память контейнера (cgroup), 0 — неизвестно
	Uptime    int64   `json:"uptime"`    // секунд с запуска awgd
	Conns     int     `json:"conns"`     // соединений клиентов в последнем замере
	Goroutine int     `json:"goroutines"`
	Source    string  `json:"conntrack"` // proc | tool | none
}

// Tunnels — то, что нужно от менеджера туннелей.
type Tunnels interface {
	Primary() string
	Options() tunnel.Options
}

// Monitor собирает замеры.
type Monitor struct {
	Store   *store.Store
	Policy  *policy.Engine
	Tunnels Tunnels
	Runner  tunnel.Runner
	Ifaces  func() []string // имена поднятых туннелей
	Proc    string          // путь к таблице conntrack; по умолчанию /proc/net/nf_conntrack
	SysNet  string          // по умолчанию /sys/class/net
	Logf    func(string, ...any)

	once    sync.Once
	mu      sync.Mutex
	hist    []Point
	prev    map[string][2]uint64
	prevT   time.Time
	cpuPrev [2]uint64
	sys     System
	start   time.Time
	snap    *Snapshot
	snapAt  time.Time
	source  string
}

func (m *Monitor) init() {
	m.once.Do(func() {
		m.start = time.Now()
		if m.Proc == "" {
			m.Proc = "/proc/net/nf_conntrack"
		}
		if m.SysNet == "" {
			m.SysNet = "/sys/class/net"
		}
		// учёт байтов в conntrack (в своём сетевом пространстве; без прав — просто нет байтов)
		_ = os.WriteFile("/proc/sys/net/netfilter/nf_conntrack_acct", []byte("1"), 0o644)
	})
}

// Run делает замеры каждые Step.
func (m *Monitor) Run(ctx context.Context) {
	m.init()
	t := time.NewTicker(Step)
	defer t.Stop()
	m.sample()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.sample()
		}
	}
}

func (m *Monitor) counters(iface string) (rx, tx uint64, ok bool) {
	read := func(n string) (uint64, bool) {
		b, err := os.ReadFile(filepath.Join(m.SysNet, iface, "statistics", n))
		if err != nil {
			return 0, false
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		return v, err == nil
	}
	rx, ok1 := read("rx_bytes")
	tx, ok2 := read("tx_bytes")
	return rx, tx, ok1 && ok2
}

func (m *Monitor) sample() {
	now := time.Now()
	cur := map[string][2]uint64{}
	lan := m.Tunnels.Options().LanIface
	if rx, tx, ok := m.counters(lan); ok && lan != "" {
		cur["lan"] = [2]uint64{rx, tx}
	}
	var tuns []string
	if m.Ifaces != nil {
		tuns = m.Ifaces()
	}
	for _, n := range tuns {
		if rx, tx, ok := m.counters(n); ok {
			cur["t:"+n] = [2]uint64{rx, tx}
		}
	}
	cpu := readCPU()
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.prevT.IsZero() {
		dt := now.Sub(m.prevT).Seconds()
		rate := func(k string, i int) float64 {
			c, ok1 := cur[k]
			p, ok2 := m.prev[k]
			if !ok1 || !ok2 || c[i] < p[i] || dt <= 0 {
				return 0
			}
			return float64(c[i]-p[i]) / dt
		}
		pt := Point{T: now.Unix()}
		// LAN: принято от клиентов — их исходящий трафик, отдано клиентам — входящий
		pt.Up, pt.Down = rate("lan", 0), rate("lan", 1)
		for k := range cur {
			if strings.HasPrefix(k, "t:") {
				pt.VDown += rate(k, 0)
				pt.VUp += rate(k, 1)
			}
		}
		m.hist = append(m.hist, pt)
		if len(m.hist) > HistSize {
			m.hist = append([]Point(nil), m.hist[len(m.hist)-HistSize:]...)
		}
	}
	m.prev, m.prevT = cur, now
	if cpu[0] > 0 && m.cpuPrev[0] > 0 && cpu[0] > m.cpuPrev[0] {
		total, idle := cpu[0]-m.cpuPrev[0], cpu[1]-m.cpuPrev[1]
		m.sys.CPU = 100 * float64(total-min(idle, total)) / float64(total)
	}
	m.cpuPrev = cpu
}

// readCPU — суммарные и простойные тики из /proc/stat.
func readCPU() [2]uint64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return [2]uint64{}
	}
	line, _, _ := strings.Cut(string(b), "\n")
	f := strings.Fields(line)
	if len(f) < 5 || f[0] != "cpu" {
		return [2]uint64{}
	}
	var total, idle uint64
	for i, v := range f[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i == 3 || i == 4 { // idle + iowait
			idle += n
		}
	}
	return [2]uint64{total, idle}
}

// Traffic возвращает замеры за последние window.
func (m *Monitor) Traffic(window time.Duration) []Point {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()
	n := int(window / Step)
	if n <= 0 || n > len(m.hist) {
		n = len(m.hist)
	}
	return append([]Point{}, m.hist[len(m.hist)-n:]...)
}

// System возвращает состояние контейнера.
func (m *Monitor) System() System {
	m.init()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m.mu.Lock()
	s := m.sys
	m.mu.Unlock()
	s.Uptime = int64(time.Since(m.start).Seconds())
	s.Goroutine = runtime.NumGoroutine()
	s.MemRSS = rss()
	if s.MemRSS == 0 {
		s.MemRSS = ms.Sys
	}
	for _, p := range []string{"/sys/fs/cgroup/memory.current", "/sys/fs/cgroup/memory/memory.usage_in_bytes"} {
		if b, err := os.ReadFile(p); err == nil {
			if v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64); err == nil {
				s.MemCgroup = v
				break
			}
		}
	}
	return s
}

func rss() uint64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "VmRSS:"); ok {
			f := strings.Fields(v)
			if len(f) > 0 {
				n, _ := strconv.ParseUint(f[0], 10, 64)
				return n << 10
			}
		}
	}
	return 0
}

// flows читает conntrack: файл ядра, а если его нет — утилитой conntrack.
func (m *Monitor) flows(ctx context.Context) ([]Flow, string) {
	if b, err := os.ReadFile(m.Proc); err == nil {
		return ParseConntrack(b), "proc"
	}
	if m.Runner == nil {
		return nil, "none"
	}
	var all []byte
	ok := false
	for _, fam := range []string{"ipv4", "ipv6"} {
		out, err := m.Runner.Run(ctx, "conntrack", "-L", "-f", fam, "-o", "extended")
		if err == nil {
			ok = true
			all = append(all, out...)
		}
	}
	if !ok {
		return nil, "none"
	}
	return ParseConntrack(all), "tool"
}

// ownAddrs — адреса самого контейнера (их соединения — не клиентов).
func ownAddrs() map[netip.Addr]bool {
	out := map[netip.Addr]bool{}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if p, err := netip.ParsePrefix(a.String()); err == nil {
			out[p.Addr().Unmap()] = true
		}
	}
	return out
}

// Device, Rule, Out и Link — узлы и связи схемы «устройства → правила → выходы».
type Node struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Sub   string `json:"sub,omitempty"`  // адрес устройства, условие правила, вид выхода
	Kind  string `json:"kind,omitempty"` // тип устройства или выхода (tunnel, group, direct, drop)
	Icon  string `json:"icon,omitempty"`
	Line  int    `json:"line,omitempty"` // строка правила
	Conns int    `json:"conns"`
	Bytes uint64 `json:"bytes"`
}

// Path — поток «устройство → правило → выход»: из них интерфейс строит схему и
// подсвечивает путь выбранного узла.
type Path struct {
	Dev   string `json:"dev"`
	Rule  string `json:"rule"`
	Out   string `json:"out"`
	Conns int    `json:"conns"`
	Bytes uint64 `json:"bytes"`
}

// Conn — одно соединение для таблицы.
type Conn struct {
	Device string `json:"device"`
	Src    string `json:"src"`
	Dst    string `json:"dst"`
	Host   string `json:"host,omitempty"`
	Port   uint16 `json:"port"`
	Proto  string `json:"proto"`
	Rule   string `json:"rule"`
	Out    string `json:"out"`
	Bytes  uint64 `json:"bytes"`
}

// Snapshot — разбор соединений на момент запроса.
type Snapshot struct {
	At      int64          `json:"at"`
	Source  string         `json:"source"` // proc | tool | none
	Total   int            `json:"total"`
	Devices []Node         `json:"devices"`
	Rules   []Node         `json:"rules"`
	Outs    []Node         `json:"outs"`
	Paths   []Path         `json:"paths"`
	Conns   []Conn         `json:"conns"`
	ByLine  map[string]int `json:"by_line"` // строка правила -> соединений (счётчики на странице правил)
	Bytes   bool           `json:"bytes"`   // conntrack считает байты
}

// Flows разбирает соединения клиентов; результат кешируется на 3 секунды.
func (m *Monitor) Flows(ctx context.Context) Snapshot {
	m.init()
	m.mu.Lock()
	if m.snap != nil && time.Since(m.snapAt) < 3*time.Second {
		s := *m.snap
		m.mu.Unlock()
		return s
	}
	m.mu.Unlock()
	flows, source := m.flows(ctx)
	s := m.build(flows, source, ownAddrs())
	m.mu.Lock()
	m.snap, m.snapAt = &s, time.Now()
	m.sys.Conns, m.sys.Source = s.Total, source
	m.mu.Unlock()
	return s
}

func (m *Monitor) build(flows []Flow, source string, own map[netip.Addr]bool) Snapshot {
	st := m.Store.Snapshot()
	type dev struct {
		p netip.Prefix
		d store.Device
	}
	var devs []dev
	for _, d := range st.Devices {
		if p, ok := d.Prefix(); ok {
			devs = append(devs, dev{p, d})
		}
	}
	sort.Slice(devs, func(i, j int) bool { return devs[i].p.Bits() > devs[j].p.Bits() })
	labels := policy.Labels(st.PolicySettings().Rules)
	primary := ""
	if m.Tunnels != nil {
		primary = m.Tunnels.Primary()
	}
	groups := map[string]bool{}
	for _, g := range st.Groups {
		groups[g.Name] = true
	}

	s := Snapshot{At: time.Now().Unix(), Source: source, ByLine: map[string]int{}}
	nodes := map[string]*Node{}
	paths := map[[3]string]*Path{}
	node := func(id string, mk func() Node) *Node {
		n := nodes[id]
		if n == nil {
			v := mk()
			v.ID = id
			n = &v
			nodes[id] = n
		}
		return n
	}
	path := func(d, r, o string, bytes uint64) {
		k := [3]string{d, r, o}
		p := paths[k]
		if p == nil {
			p = &Path{Dev: d, Rule: r, Out: o}
			paths[k] = p
		}
		p.Conns++
		p.Bytes += bytes
	}
	var conns []Conn
	for _, f := range flows {
		if own[f.Src] || own[f.Dst] || f.Src.IsLoopback() || f.Dst.IsLoopback() || f.Dst.IsMulticast() {
			continue // соединения самого контейнера и к нему (консоль, DNS)
		}
		if f.Bytes > 0 {
			s.Bytes = true
		}
		s.Total++
		// устройство
		devID, devName, devSub, devKind := "ip:"+f.Src.String(), f.Src.String(), "", "other"
		for _, d := range devs {
			if d.p.Contains(f.Src) {
				devID, devName, devSub, devKind = "dev:"+d.d.ID, d.d.Name, d.d.Addr, d.d.Kind
				break
			}
		}
		dn := node(devID, func() Node { return Node{Name: devName, Sub: devSub, Kind: devKind} })
		dn.Conns++
		dn.Bytes += f.Bytes
		// правило
		v := policy.Verdict{Action: policy.Proxy, By: "default"}
		if m.Policy != nil {
			v = m.Policy.Classify(f.Src, f.Dst, f.Proto, f.Dport)
		}
		ruleID, ruleName, ruleSub, ruleIcon, ruleLine := "rule:default", "", "", "", 0
		if v.Line > 0 {
			lb, ok := labels[v.Line]
			key := v.Line
			if ok {
				key = lb.Head
			}
			ruleID, ruleName, ruleSub, ruleIcon, ruleLine = "rule:"+strconv.Itoa(key), lb.Name, v.By, lb.Icon, key
			if !ok || ruleName == "" {
				ruleName = v.By
			}
			s.ByLine[strconv.Itoa(v.Line)]++
		}
		rn := node(ruleID, func() Node { return Node{Name: ruleName, Sub: ruleSub, Icon: ruleIcon, Line: ruleLine} })
		rn.Conns++
		rn.Bytes += f.Bytes
		// выход
		outID, outName, outKind := "out:direct", "direct", "direct"
		switch v.Action {
		case policy.Drop:
			outID, outName, outKind = "out:drop", "drop", "drop"
		case policy.Proxy:
			t := v.Target
			if t == "" {
				t = primary
			}
			outKind = "tunnel"
			if groups[t] {
				outKind = "group"
			}
			if t == "" {
				t, outKind = "vpn", "tunnel"
			}
			outID, outName = "out:"+t, t
		}
		on := node(outID, func() Node { return Node{Name: outName, Kind: outKind} })
		on.Conns++
		on.Bytes += f.Bytes
		path(devID, ruleID, outID, f.Bytes)
		host := ""
		if m.Policy != nil {
			host = m.Policy.Name(f.Dst)
		}
		label := ruleName
		if ruleID == "rule:default" {
			label = ""
		}
		conns = append(conns, Conn{Device: devName, Src: f.Src.String(), Dst: f.Dst.String(), Host: host, Port: f.Dport, Proto: f.Proto, Rule: label, Out: outName, Bytes: f.Bytes})
	}
	for _, n := range nodes {
		switch {
		case strings.HasPrefix(n.ID, "dev:"), strings.HasPrefix(n.ID, "ip:"):
			s.Devices = append(s.Devices, *n)
		case strings.HasPrefix(n.ID, "rule:"):
			s.Rules = append(s.Rules, *n)
		default:
			s.Outs = append(s.Outs, *n)
		}
	}
	byConns := func(a []Node) {
		sort.Slice(a, func(i, j int) bool {
			if a[i].Conns != a[j].Conns {
				return a[i].Conns > a[j].Conns
			}
			return a[i].ID < a[j].ID
		})
	}
	byConns(s.Devices)
	byConns(s.Rules)
	byConns(s.Outs)
	for _, p := range paths {
		s.Paths = append(s.Paths, *p)
	}
	sort.Slice(s.Paths, func(i, j int) bool {
		a, b := s.Paths[i], s.Paths[j]
		return a.Dev+"\x00"+a.Rule+"\x00"+a.Out < b.Dev+"\x00"+b.Rule+"\x00"+b.Out
	})
	sort.Slice(conns, func(i, j int) bool {
		if conns[i].Bytes != conns[j].Bytes {
			return conns[i].Bytes > conns[j].Bytes
		}
		return conns[i].Dst < conns[j].Dst
	})
	if len(conns) > MaxConns {
		conns = conns[:MaxConns]
	}
	s.Conns = conns
	if s.Devices == nil {
		s.Devices, s.Rules, s.Outs, s.Paths, s.Conns = []Node{}, []Node{}, []Node{}, []Path{}, []Conn{}
	}
	return s
}

// Preload добавляет замеры в историю (демонстрационный стенд и тесты).
func (m *Monitor) Preload(pts []Point) {
	m.init()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hist = append(m.hist, pts...)
	if len(m.hist) > HistSize {
		m.hist = append([]Point(nil), m.hist[len(m.hist)-HistSize:]...)
	}
}
