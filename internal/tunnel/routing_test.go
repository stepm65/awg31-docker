package tunnel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dualConf = "[Interface]\nPrivateKey = KEY\nAddress = 10.0.0.2/32, fd00:77::2/128\nDNS = 10.8.0.1, fd00:77::1\nMTU = 1200\nTable = auto\n\n[Peer]\nPublicKey = KEY\nAllowedIPs = 0.0.0.0/0, ::/0\nEndpoint = 192.0.2.1:51820\n"

func gwMgr(t *testing.T, f *fakeRunner, o Options) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	o.RunDir = dir
	o.AutoRoute = true
	return New(f, o), dir
}

func upAndRead(t *testing.T, m *Manager, dir, text string) string {
	t.Helper()
	if err := m.Up(context.Background(), "awg0", strings.ReplaceAll(text, "KEY", key)); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "awg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func calls(f *fakeRunner) string {
	var got []string
	for _, c := range f.calls {
		got = append(got, c.name+" "+strings.Join(c.args, " "))
	}
	return strings.Join(got, "\n")
}

func TestGatewayRoutesLanTrafficIntoOwnTable(t *testing.T) {
	f := &fakeRunner{}
	m, dir := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true, IPv6: true})
	text := upAndRead(t, m, dir, dualConf)
	for _, want := range []string{
		"Table = 110",
		"ip -4 rule add priority 1110 fwmark 0x3100006e lookup 110",
		"ip -6 rule add priority 1110 fwmark 0x3100006e lookup 110",
		"PostDown = ip -4 rule del priority 1110",
		"ip6tables -t nat -C POSTROUTING -o %i -j MASQUERADE",
		"Address = 10.0.0.2/32, fd00:77::2/128",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"MTU = 1280",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("нет %q в рабочем конфиге:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Table = auto") || strings.Contains(text, "DNS =") {
		t.Fatalf("Table из профиля и DNS не должны попадать в конфиг шлюза:\n%s", text)
	}
	// с kill switch правило не удаляется при остановке туннеля
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "PostDown") && (strings.Contains(line, "priority 300") || strings.Contains(line, "blackhole")) {
			t.Fatalf("при kill switch правило LAN и blackhole должны оставаться после остановки: %s", line)
		}
	}
	// первый поднятый туннель становится основным: default в общей таблице
	all := calls(f)
	if !strings.Contains(all, "ip -4 route replace default dev awg0 table 103") || !strings.Contains(all, "ip -6 route replace default dev awg0 table 103") {
		t.Fatalf("трафик LAN не направлен в первый туннель:\n%s", all)
	}
	if m.Primary() != "awg0" {
		t.Fatalf("основной туннель %q", m.Primary())
	}
}

func TestGatewayWithoutKillSwitchRemovesRuleOnDown(t *testing.T) {
	f := &fakeRunner{}
	m, dir := gwMgr(t, f, Options{LanIface: "eth0", IPv6: false})
	text := upAndRead(t, m, dir, dualConf)
	if strings.Contains(text, "blackhole") || strings.Contains(text, "ip -6") || strings.Contains(text, "ip6tables") {
		t.Fatalf("без kill switch и IPv6 лишние команды:\n%s", text)
	}
	if !strings.Contains(calls(f), "ip -4 rule add priority 300 from all iif eth0 lookup 103") {
		t.Fatalf("без kill switch правило LAN ставится вместе с основным туннелем:\n%s", calls(f))
	}
	f.calls = nil
	if err := m.Down(context.Background(), "awg0"); err != nil {
		t.Fatal(err)
	}
	all := calls(f)
	if !strings.Contains(all, "ip -4 rule del priority 300") || strings.Contains(all, "rule add") {
		t.Fatalf("без kill switch правило LAN снимается с последним туннелем:\n%s", all)
	}
	if m.Primary() != "" || m.Table("awg0") != 0 {
		t.Fatal("после остановки туннель не должен оставаться основным и держать таблицу")
	}
}

func TestIPv6OffStripsV6FromRuntimeConfig(t *testing.T) {
	var logs []string
	m, dir := gwMgr(t, &fakeRunner{}, Options{LanIface: "eth0", Logf: func(f string, a ...any) { logs = append(logs, f) }})
	text := upAndRead(t, m, dir, dualConf)
	if strings.Contains(text, "fd00:77") || strings.Contains(text, "::/0") {
		t.Fatalf("IPv6 выключен, но адреса остались:\n%s", text)
	}
	if !strings.Contains(text, "Address = 10.0.0.2/32") || !strings.Contains(text, "AllowedIPs = 0.0.0.0/0") {
		t.Fatalf("IPv4 должен остаться:\n%s", text)
	}
	if !strings.Contains(text, "MTU = 1200") {
		t.Fatalf("без IPv6 MTU не трогается:\n%s", text)
	}
	if len(logs) == 0 {
		t.Fatal("о вырезанном IPv6 и DNS нужно написать в журнал")
	}
}

func TestIPv6OnlyProfileFailsClearlyWhenIPv6Off(t *testing.T) {
	m, _ := gwMgr(t, &fakeRunner{}, Options{LanIface: "eth0"})
	only6 := strings.ReplaceAll("[Interface]\nPrivateKey = KEY\nAddress = fd00:77::2/128\n\n[Peer]\nPublicKey = KEY\nAllowedIPs = ::/0\nEndpoint = 192.0.2.1:51820\n", "KEY", key)
	err := m.Up(context.Background(), "awg0", only6)
	if err == nil || !strings.Contains(err.Error(), "IPv6") {
		t.Fatalf("ожидалась понятная ошибка про IPv6, получено %v", err)
	}
}

func TestPlainModeKeepsProfileRouting(t *testing.T) {
	// Без LAN-интерфейса (не шлюз) таблица и DNS из профиля сохраняются.
	m, dir := gwMgr(t, &fakeRunner{}, Options{IPv6: true})
	text := upAndRead(t, m, dir, dualConf)
	if !strings.Contains(text, "Table = auto") || !strings.Contains(text, "DNS = 10.8.0.1, fd00:77::1") {
		t.Fatalf("без режима шлюза профиль не должен меняться:\n%s", text)
	}
	if strings.Contains(text, "rule add") {
		t.Fatalf("без LAN-интерфейса правил маршрутизации быть не должно:\n%s", text)
	}
}

func TestLanIfaceIsValidatedBeforeReachingHooks(t *testing.T) {
	for _, bad := range []string{"eth0;reboot", "$(id)", "eth 0", "", "averyveryverylongname"} {
		if ValidIfaceName(bad) {
			t.Errorf("%q не должно проходить проверку", bad)
		}
		m, _ := gwMgr(t, &fakeRunner{}, Options{LanIface: bad, KillSwitch: true})
		if err := m.EnsureRouting(context.Background()); err == nil && bad != "" {
			t.Fatalf("с недопустимым именем %q маршрутизация настраиваться не должна", bad)
		}
	}
	for _, good := range []string{"eth0", "veth-awg31", "br0.10"} {
		if !ValidIfaceName(good) {
			t.Errorf("%q должно проходить проверку", good)
		}
	}
}

func TestEnsureRoutingKillSwitch(t *testing.T) {
	f := &fakeRunner{}
	m, _ := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true, IPv6: true})
	if err := m.EnsureRouting(context.Background()); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range f.calls {
		got = append(got, c.name+" "+strings.Join(c.args, " "))
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{
		"ip -4 rule add priority 300 from all iif eth0 lookup 103",
		"ip -4 route replace blackhole default table 103 metric 254",
		"ip -6 rule add priority 300 from all iif eth0 lookup 103",
		"ip -6 route replace blackhole default table 103 metric 254",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("нет команды %q:\n%s", want, all)
		}
	}

	f2 := &fakeRunner{}
	m2, _ := gwMgr(t, f2, Options{LanIface: "eth0"})
	_ = m2.EnsureRouting(context.Background())
	for _, c := range f2.calls {
		line := c.name + " " + strings.Join(c.args, " ")
		if strings.Contains(line, "rule add priority 300") || strings.Contains(line, "replace blackhole") {
			t.Fatalf("без kill switch постоянных правил LAN быть не должно: %s", line)
		}
	}
	if !strings.Contains(calls(f2), "ip -4 rule add priority 1103 fwmark 0x31000067 lookup 103") {
		t.Fatalf("правило метки «через основной туннель» нужно всегда:\n%s", calls(f2))
	}
}

func TestDetectLanIface(t *testing.T) {
	for out, want := range map[string]string{
		"default via 192.168.253.1 dev eth0 proto static\n": "eth0",
		"default via 172.18.0.1 dev veth1@if5 metric 10\n":  "veth1",
	} {
		f := &routeRunner{out: out}
		got, err := DetectLanIface(context.Background(), f)
		if err != nil || got != want {
			t.Errorf("%q: получено %q, %v", out, got, err)
		}
	}
	if _, err := DetectLanIface(context.Background(), &routeRunner{out: "default via 1.1.1.1 dev $(id)\n"}); err == nil {
		t.Fatal("подозрительное имя интерфейса должно отвергаться")
	}
	if _, err := DetectLanIface(context.Background(), &routeRunner{out: ""}); err == nil {
		t.Fatal("без маршрута по умолчанию нужна ошибка")
	}
}

type routeRunner struct{ out string }

func (r *routeRunner) Run(_ context.Context, _ string, _ ...string) ([]byte, error) {
	return []byte(r.out), nil
}

func TestIPv6Available(t *testing.T) {
	old := IPv6Sysctl
	defer func() { IPv6Sysctl = old }()
	dir := t.TempDir()
	IPv6Sysctl = filepath.Join(dir, "disable_ipv6")
	if IPv6Available() {
		t.Fatal("без файла IPv6 считается недоступным")
	}
	_ = os.WriteFile(IPv6Sysctl, []byte("1\n"), 0o600)
	if IPv6Available() {
		t.Fatal("disable_ipv6=1")
	}
	_ = os.WriteFile(IPv6Sysctl, []byte("0\n"), 0o600)
	if !IPv6Available() {
		t.Fatal("disable_ipv6=0")
	}
}

func TestMultiTunnelTablesAndFastSwitch(t *testing.T) {
	f := &fakeRunner{}
	m, dir := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true})
	ctx := context.Background()
	c := strings.ReplaceAll(goodConf, "Address = 10.0.0.2/32", "Address = 10.0.0.2/32")
	for _, n := range []string{"a1", "b2", "c3"} {
		if err := m.Up(ctx, n, c); err != nil {
			t.Fatal(err)
		}
	}
	if m.Table("a1") != 110 || m.Table("b2") != 111 || m.Table("c3") != 112 {
		t.Fatalf("таблицы: %d %d %d", m.Table("a1"), m.Table("b2"), m.Table("c3"))
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "b2.conf"))
	if !strings.Contains(string(raw), "Table = 111") || !strings.Contains(string(raw), "fwmark 0x3100006f lookup 111") {
		t.Fatalf("у второго туннеля своя таблица и правило пробы:\n%s", raw)
	}
	if m.Primary() != "a1" {
		t.Fatalf("основным должен остаться первый: %q", m.Primary())
	}
	f.calls = nil
	if err := m.SetPrimary(ctx, "c3"); err != nil {
		t.Fatal(err)
	}
	all := calls(f)
	if all != "ip -4 route replace default dev c3 table 103" {
		t.Fatalf("быстрое переключение — одна команда без перезапуска туннелей:\n%s", all)
	}
	if err := m.SetPrimary(ctx, "zz"); err != ErrNotUp {
		t.Fatalf("основным может быть только поднятый туннель: %v", err)
	}
	// основной остановлен — трафик переходит на туннель, поднятый раньше других
	f.calls = nil
	if err := m.Down(ctx, "c3"); err != nil {
		t.Fatal(err)
	}
	if m.Primary() != "a1" || !strings.Contains(calls(f), "ip -4 route replace default dev a1 table 103") {
		t.Fatalf("после остановки основного трафик должен перейти на a1: %q\n%s", m.Primary(), calls(f))
	}
	// таблица c3 освободилась и выдаётся следующему
	if err := m.Up(ctx, "d4", c); err != nil {
		t.Fatal(err)
	}
	if m.Table("d4") != 112 {
		t.Fatalf("освободившаяся таблица должна переиспользоваться: %d", m.Table("d4"))
	}
}

func TestPlainModeHasNoPrimary(t *testing.T) {
	m, _ := gwMgr(t, &fakeRunner{}, Options{})
	if err := m.Up(context.Background(), "awg0", goodConf); err != nil {
		t.Fatal(err)
	}
	if m.Primary() != "" || m.Table("awg0") != 0 {
		t.Fatal("без режима шлюза нет ни таблиц, ни основного туннеля")
	}
	if err := m.SetPrimary(context.Background(), "awg0"); err == nil {
		t.Fatal("без режима шлюза выбор основного туннеля невозможен")
	}
}

func TestServerProfileStaysInMainAndIsNeverPrimary(t *testing.T) {
	f := &fakeRunner{}
	var logs []string
	m, dir := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true, Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }})
	srv := strings.ReplaceAll("[Interface]\nPrivateKey = KEY\nAddress = 10.8.0.1/24\nListenPort = 51820\n\n[Peer]\nPublicKey = KEY\nAllowedIPs = 10.8.0.2/32\n", "KEY", key)
	if err := m.Up(context.Background(), "srv0", srv); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "srv0.conf"))
	text := string(raw)
	if strings.Contains(text, "Table =") || strings.Contains(text, "fwmark") {
		t.Fatalf("сервер: маршруты клиентов в main, без своей таблицы:\n%s", text)
	}
	if !strings.Contains(text, "iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE") {
		t.Fatalf("клиенты сервера выходят через LAN с подменой адреса:\n%s", text)
	}
	if m.Primary() != "" || m.Table("srv0") != 0 {
		t.Fatal("сервер не становится основным туннелем")
	}
	if len(logs) != 0 {
		t.Fatalf("сервер не должен даже пытаться стать основным: %q", logs)
	}
	// обычный туннель после сервера становится основным
	if err := m.Up(context.Background(), "awg0", goodConf); err != nil {
		t.Fatal(err)
	}
	if m.Primary() != "awg0" {
		t.Fatalf("основной: %q", m.Primary())
	}
}

type fakeGroups struct {
	groups map[string]string // имя -> маршрут IPv4 ("" — исправных участников нет)
	v6     string
}

func (g *fakeGroups) Has(n string) bool { _, ok := g.groups[n]; return ok }
func (g *fakeGroups) Nexthop(n string, v6 bool) (string, bool) {
	if v6 {
		return g.v6, g.v6 != ""
	}
	s := g.groups[n]
	return s, s != ""
}

func TestGroupAsPrimaryKeepsKillSwitchAndSurvivesTunnelEvents(t *testing.T) {
	f := &fakeRunner{}
	m, _ := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true, IPv6: true})
	ctx := context.Background()
	g := &fakeGroups{groups: map[string]string{"pool": "nexthop dev a1 weight 1 nexthop dev b2 weight 1"}, v6: "dev a1"}
	m.SetGroups(g)
	for _, n := range []string{"a1", "b2"} {
		if err := m.Up(ctx, n, goodConf); err != nil {
			t.Fatal(err)
		}
	}
	f.calls = nil
	if err := m.SetPrimary(ctx, "pool"); err != nil {
		t.Fatal(err)
	}
	all := calls(f)
	if m.Primary() != "pool" || !m.PrimaryIsGroup() {
		t.Fatalf("основной — группа: %q", m.Primary())
	}
	if !strings.Contains(all, "ip -4 route replace default nexthop dev a1 weight 1 nexthop dev b2 weight 1 table 103") ||
		!strings.Contains(all, "ip -6 route replace default dev a1 table 103") {
		t.Fatalf("default таблицы 103 — маршрут группы (IPv6 через одного):\n%s", all)
	}
	// остановка участника не должна менять основной на одиночный туннель
	if err := m.Down(ctx, "a1"); err != nil {
		t.Fatal(err)
	}
	if m.Primary() != "pool" {
		t.Fatalf("основной остаётся группой: %q", m.Primary())
	}
	// участников не осталось: прежний маршрут заменяется blackhole, а не удаляется
	// (удаление «default» без метрики могло бы снести и kill switch с метрикой 254)
	g.groups["pool"] = ""
	g.v6 = ""
	f.calls = nil
	m.RefreshPrimary(ctx)
	all = calls(f)
	if !strings.Contains(all, "ip -4 route replace blackhole default table 103") || strings.Contains(all, "route del") {
		t.Fatalf("пустая группа — blackhole:\n%s", all)
	}
	// группу удалили: трафик возвращается на поднятый туннель
	delete(g.groups, "pool")
	f.calls = nil
	m.RefreshPrimary(ctx)
	if m.Primary() != "b2" || !strings.Contains(calls(f), "ip -4 route replace default dev b2 table 103") {
		t.Fatalf("после удаления группы: %q\n%s", m.Primary(), calls(f))
	}
	if m.PrimaryIsGroup() {
		t.Fatal("основной снова туннель")
	}
}

func TestGroupPrimaryWithoutKillSwitchDropsLanRuleWhileEmpty(t *testing.T) {
	f := &fakeRunner{}
	m, _ := gwMgr(t, f, Options{LanIface: "eth0"})
	ctx := context.Background()
	g := &fakeGroups{groups: map[string]string{"pool": ""}}
	m.SetGroups(g)
	if err := m.Up(ctx, "a1", goodConf); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	if err := m.SetPrimary(ctx, "pool"); err != nil {
		t.Fatal(err)
	}
	if all := calls(f); !strings.Contains(all, "ip -4 rule del priority 300") || strings.Contains(all, "rule add priority 300") {
		t.Fatalf("без kill switch и без исправных участников правило LAN снимается:\n%s", all)
	}
	g.groups["pool"] = "dev a1"
	f.calls = nil
	m.RefreshPrimary(ctx)
	if all := calls(f); !strings.Contains(all, "ip -4 rule add priority 300 from all iif eth0 lookup 103") {
		t.Fatalf("участник появился — правило возвращается:\n%s", all)
	}
}

func TestRefreshPrimaryIgnoresTunnelPrimary(t *testing.T) {
	f := &fakeRunner{}
	m, _ := gwMgr(t, f, Options{LanIface: "eth0", KillSwitch: true})
	m.SetGroups(&fakeGroups{groups: map[string]string{}})
	if err := m.Up(context.Background(), "a1", goodConf); err != nil {
		t.Fatal(err)
	}
	f.calls = nil
	m.RefreshPrimary(context.Background())
	if len(f.calls) != 0 || m.Primary() != "a1" {
		t.Fatalf("основной туннель не трогается: %v", f.calls)
	}
}
