package monitor

import (
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

const sample = `ipv4     2 tcp      6 431999 ESTABLISHED src=192.168.51.150 dst=149.154.167.50 sport=50000 dport=443 packets=10 bytes=1500 src=149.154.167.50 dst=10.8.0.2 sport=443 dport=50000 packets=12 bytes=9000 [ASSURED] mark=0 use=1
ipv4     2 udp      17 25 src=192.168.51.10 dst=8.8.8.8 sport=4000 dport=53 packets=1 bytes=60 src=8.8.8.8 dst=192.168.51.10 sport=53 dport=4000 packets=1 bytes=120 mark=0 use=1
ipv4     2 tcp      6 100 TIME_WAIT src=192.168.253.2 dst=1.1.1.1 sport=40000 dport=443 src=1.1.1.1 dst=192.168.253.2 sport=443 dport=40000 [ASSURED] mark=0 use=1
ipv6     10 tcp      6 300 ESTABLISHED src=fd00::5 dst=2001:db8::1 sport=1 dport=80 src=2001:db8::1 dst=fd00::5 sport=80 dport=1 mark=0 use=1
garbage
`

type fakeTun struct{}

func (fakeTun) Primary() string         { return "awg0" }
func (fakeTun) Options() tunnel.Options { return tunnel.Options{LanIface: "eth0"} }

func TestParseConntrack(t *testing.T) {
	fl := ParseConntrack([]byte(sample))
	if len(fl) != 4 {
		t.Fatalf("%d: %+v", len(fl), fl)
	}
	if fl[0].Proto != "tcp" || fl[0].Src.String() != "192.168.51.150" || fl[0].Dst.String() != "149.154.167.50" || fl[0].Dport != 443 || fl[0].Bytes != 10500 {
		t.Fatalf("%+v", fl[0])
	}
	if fl[3].Src.String() != "fd00::5" || fl[3].Bytes != 0 {
		t.Fatalf("ipv6 без учёта байтов: %+v", fl[3])
	}
}

func TestBuildSnapshot(t *testing.T) {
	st, _ := store.Open(t.TempDir())
	_ = st.Update(func(s *store.State) error {
		s.Devices = []store.Device{{ID: "pc", Name: "PC", Addr: "192.168.51.150", Kind: "pc"}}
		return nil
	})
	m := &Monitor{Store: st, Tunnels: fakeTun{}}
	own := map[netip.Addr]bool{netip.MustParseAddr("192.168.253.2"): true}
	s := m.build(ParseConntrack([]byte(sample)), "proc", own)
	if s.Total != 3 || !s.Bytes {
		t.Fatalf("соединения контейнера не считаются: %+v", s)
	}
	if s.Devices[0].ID != "dev:pc" || s.Devices[0].Name != "PC" || s.Devices[0].Bytes != 10500 {
		t.Fatalf("устройства: %+v", s.Devices)
	}
	if len(s.Outs) != 1 || s.Outs[0].ID != "out:awg0" || s.Outs[0].Conns != 3 {
		t.Fatalf("без политики всё идёт в основной туннель: %+v", s.Outs)
	}
	if len(s.Paths) != 3 || s.Paths[0].Out != "out:awg0" { // 3 устройства -> default -> awg0
		t.Fatalf("потоки: %+v", s.Paths)
	}
	if s.Conns[0].Device != "PC" || s.Conns[0].Port != 443 {
		t.Fatalf("соединения по байтам: %+v", s.Conns)
	}
}

func TestSampleRates(t *testing.T) {
	dir := t.TempDir()
	write := func(iface string, rx, tx int) {
		p := filepath.Join(dir, iface, "statistics")
		_ = os.MkdirAll(p, 0o755)
		_ = os.WriteFile(filepath.Join(p, "rx_bytes"), []byte(itoa(rx)), 0o644)
		_ = os.WriteFile(filepath.Join(p, "tx_bytes"), []byte(itoa(tx)), 0o644)
	}
	write("eth0", 1000, 5000)
	write("awg0", 4000, 800)
	m := &Monitor{Tunnels: fakeTun{}, SysNet: dir, Ifaces: func() []string { return []string{"awg0", "nope"} }}
	m.init()
	m.sample()
	write("eth0", 1000+500*5, 5000+2000*5)
	write("awg0", 4000+1800*5, 800+400*5)
	m.prevT = m.prevT.Add(-Step) // будто прошло 5 секунд
	m.sample()
	pts := m.Traffic(0)
	if len(pts) != 1 {
		t.Fatalf("%+v", pts)
	}
	p := pts[0]
	near := func(a, b float64) bool { return a > b*0.8 && a < b*1.2 }
	if !near(p.Up, 500) || !near(p.Down, 2000) || !near(p.VDown, 1800) || !near(p.VUp, 400) {
		t.Fatalf("скорости: %+v", p)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }
