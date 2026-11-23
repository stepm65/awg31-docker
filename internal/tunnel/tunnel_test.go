package tunnel

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var key = base64.StdEncoding.EncodeToString(make([]byte, 32))

var goodConf = "[Interface]\nPrivateKey = " + key + "\nAddress = 10.0.0.2/32\nJc = 5\n\n[Peer]\nPublicKey = " + key + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 192.0.2.1:51820\n"

type call struct {
	name string
	args []string
}

type fakeRunner struct {
	calls      []call
	interfaces string // что вернёт `awg show interfaces`
	dump       string
	failUp     bool
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{name, args})
	switch {
	case strings.HasSuffix(name, "awg-quick") && args[0] == "up" && f.failUp:
		return []byte("boom " + key), errors.New("exit status 1")
	case strings.HasSuffix(name, "awg") && args[0] == "show" && args[1] == "interfaces":
		return []byte(f.interfaces), nil
	case strings.HasSuffix(name, "awg") && args[0] == "show" && args[2] == "dump":
		return []byte(f.dump), nil
	}
	return nil, nil
}

func newMgr(t *testing.T, f *fakeRunner, auto bool) (*Manager, string) {
	dir := t.TempDir()
	return New(f, Options{RunDir: dir, AutoRoute: auto}), dir
}

func TestUpWritesRenderedConfigWithGeneratedHooks(t *testing.T) {
	f := &fakeRunner{}
	m, dir := newMgr(t, f, true)
	if err := m.Up(context.Background(), "wg0", goodConf); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "wg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Count(text, "PostUp =") != 2 || strings.Count(text, "PostDown =") != 2 {
		t.Fatalf("ожидались 2 PostUp и 2 PostDown:\n%s", text)
	}
	if !strings.Contains(text, "-t mangle -A FORWARD") || !strings.Contains(text, "-t nat -A POSTROUTING") {
		t.Fatalf("TCPMSS должен идти в таблице mangle, NAT — в nat:\n%s", text)
	}
	info, _ := os.Stat(filepath.Join(dir, "wg0.conf"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("права рабочего конфига %v", info.Mode().Perm())
	}
	last := f.calls[len(f.calls)-1]
	if last.args[0] != "up" || last.args[1] != filepath.Join(dir, "wg0.conf") {
		t.Fatalf("неверный вызов awg-quick: %+v", last)
	}
}

func TestUpWithoutAutoRouteHasNoHooks(t *testing.T) {
	f := &fakeRunner{}
	m, dir := newMgr(t, f, false)
	if err := m.Up(context.Background(), "wg0", goodConf); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "wg0.conf"))
	if strings.Contains(string(raw), "PostUp") {
		t.Fatal("без AutoRoute хуков быть не должно")
	}
}

func TestUpRejectsUserHooksAndAlreadyUp(t *testing.T) {
	f := &fakeRunner{}
	m, _ := newMgr(t, f, true)
	evil := strings.Replace(goodConf, "Jc = 5", "PostUp = touch /pwned", 1)
	if err := m.Up(context.Background(), "wg0", evil); err == nil {
		t.Fatal("конфиг с хуком должен отвергаться")
	}
	for _, c := range f.calls {
		if len(c.args) > 0 && c.args[0] == "up" {
			t.Fatal("awg-quick не должен вызываться для отвергнутого конфига")
		}
	}
	f.interfaces = "wg0\n"
	f.dump = "priv\tpub\t51820\toff\n"
	if err := m.Up(context.Background(), "wg0", goodConf); !errors.Is(err, ErrAlreadyUp) {
		t.Fatalf("ожидался ErrAlreadyUp, получено %v", err)
	}
}

func TestFailedUpCleansRuntimeConfigAndRedactsKeys(t *testing.T) {
	f := &fakeRunner{failUp: true}
	m, dir := newMgr(t, f, false)
	err := m.Up(context.Background(), "wg0", goodConf)
	if err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "wg0.conf")); statErr == nil {
		t.Fatal("рабочий конфиг с приватным ключом должен удаляться после неудачи")
	}
}

func TestDown(t *testing.T) {
	f := &fakeRunner{}
	m, dir := newMgr(t, f, false)
	if err := m.Down(context.Background(), "wg0"); !errors.Is(err, ErrNotUp) {
		t.Fatalf("ожидался ErrNotUp, получено %v", err)
	}
	if err := m.Up(context.Background(), "wg0", goodConf); err != nil {
		t.Fatal(err)
	}
	if err := m.Down(context.Background(), "wg0"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "wg0.conf")); err == nil {
		t.Fatal("рабочий конфиг должен удаляться после Down")
	}
}

func TestParseDumpWithAmneziaExtraFields(t *testing.T) {
	// У AmneziaWG в строке интерфейса дополнительные поля обфускации.
	dump := "PRIV\tPUB\t51820\toff\t5\t10\t50\t30\t40\n" +
		"PEERPUB\tPSK\t203.0.113.5:51820\t0.0.0.0/0,::/0\t1700000000\t1234\t5678\t25\n" +
		"PEER2\t(none)\t(none)\t(none)\t0\t0\t0\toff\n"
	i, err := ParseDump("wg0", dump)
	if err != nil {
		t.Fatal(err)
	}
	if i.ListenPort != 51820 || len(i.Peers) != 2 {
		t.Fatalf("неверный разбор: %+v", i)
	}
	p := i.Peers[0]
	if p.Endpoint != "203.0.113.5:51820" || len(p.AllowedIPs) != 2 || p.LatestHandshake != 1700000000 || p.RxBytes != 1234 || p.TxBytes != 5678 {
		t.Fatalf("неверный пир: %+v", p)
	}
	if i.Peers[1].Endpoint != "" || len(i.Peers[1].AllowedIPs) != 0 {
		t.Fatalf("(none) должен превращаться в пустое значение: %+v", i.Peers[1])
	}
	if _, err := ParseDump("wg0", ""); err == nil {
		t.Fatal("пустой вывод должен давать ошибку")
	}
}

func TestStatusNeverExposesSecrets(t *testing.T) {
	dump := "PRIVATEKEYVALUE\tPUB\t51820\toff\nPEERPUB\tPRESHARED\t(none)\t10.0.0.0/8\t0\t0\t0\toff\n"
	i, _ := ParseDump("wg0", dump)
	flat := strings.Join([]string{i.Name, i.Peers[0].PublicKey, i.Peers[0].Endpoint, strings.Join(i.Peers[0].AllowedIPs, ",")}, " ")
	if strings.Contains(flat, "PRIVATEKEYVALUE") || strings.Contains(flat, "PRESHARED") {
		t.Fatal("приватный ключ и PSK не должны попадать в Iface")
	}
}

func TestRedact(t *testing.T) {
	out := Redact("error with " + key + " inside")
	if strings.Contains(out, key) {
		t.Fatalf("ключ не скрыт: %s", out)
	}
	if len(Redact(strings.Repeat("x", 5000))) > 2100 {
		t.Fatal("вывод должен усекаться")
	}
}

func TestRouteHooksAreConstant(t *testing.T) {
	h := RouteHooks(Options{AutoRoute: true}, 0)
	if len(h.PostUp) != 2 || len(h.PostDown) != 2 {
		t.Fatalf("неожиданное число хуков: %+v", h)
	}
	for _, c := range append(append([]string{}, h.PostUp...), h.PostDown...) {
		if strings.Contains(c, "%s") || strings.Contains(c, "%!") {
			t.Fatalf("ошибка форматирования в хуке: %s", c)
		}
		if !strings.Contains(c, "%i") {
			t.Fatalf("в хуке должно быть %%i для имени интерфейса: %s", c)
		}
	}
}
