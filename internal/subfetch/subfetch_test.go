package subfetch

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func pinOf(s *httptest.Server) string {
	sum := sha256.Sum256(s.Certificate().Raw)
	return hex.EncodeToString(sum[:])
}

func newSrv(t *testing.T, h http.HandlerFunc) *httptest.Server {
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	return s
}

func opts(url string) Options { return Options{URL: url, AllowLoopback: true} }

func TestValidateURL(t *testing.T) {
	for _, bad := range []string{
		"http://example.org/sub", // открытый HTTP
		"ftp://example.org/x",
		"https://user:pass@example.org/x",
		"https:///nohost",
		"example.org/sub",
		"",
	} {
		if _, err := ValidateURL(bad, false); err == nil {
			t.Errorf("%q должен отвергаться", bad)
		}
	}
	if _, err := ValidateURL("https://example.org/sub/tok", false); err != nil {
		t.Errorf("корректный https-URL отвергнут: %v", err)
	}
	if _, err := ValidateURL("http://example.org/sub", true); err != nil {
		t.Errorf("http при явном разрешении должен проходить: %v", err)
	}
}

func TestFetchRejectsPlainHTTPByDefault(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) }))
	defer s.Close()
	if _, _, _, err := Fetch(context.Background(), opts(s.URL), ""); err == nil {
		t.Fatal("открытый HTTP должен отвергаться по умолчанию")
	}
	o := opts(s.URL)
	o.AllowHTTP = true
	if _, _, _, err := Fetch(context.Background(), o, ""); err != nil {
		t.Fatalf("с явным разрешением HTTP должен работать: %v", err)
	}
}

func TestSelfSignedRejectedWithoutPin(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("cfg")) })
	if _, _, _, err := Fetch(context.Background(), opts(s.URL), ""); err == nil {
		t.Fatal("самоподписанный сертификат без отпечатка должен отвергаться: проверка TLS включена по умолчанию")
	}
}

func TestPinAcceptsMatchingAndRejectsWrong(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("cfg")) })
	o := opts(s.URL)
	o.PinSHA256 = pinOf(s)
	body, _, _, err := Fetch(context.Background(), o, "")
	if err != nil || body != "cfg\n" {
		t.Fatalf("верный отпечаток: %q %v", body, err)
	}
	// формат с двоеточиями тоже поддерживается
	var parts []string
	for i := 0; i < len(o.PinSHA256); i += 2 {
		parts = append(parts, strings.ToUpper(o.PinSHA256[i:i+2]))
	}
	o.PinSHA256 = strings.Join(parts, ":")
	if _, _, _, err := Fetch(context.Background(), o, ""); err != nil {
		t.Fatalf("отпечаток с двоеточиями: %v", err)
	}
	o.PinSHA256 = strings.Repeat("ab", 32)
	if _, _, _, err := Fetch(context.Background(), o, ""); err == nil {
		t.Fatal("неверный отпечаток должен отвергаться")
	}
}

func TestInsecureTLSIsExplicit(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("cfg")) })
	o := opts(s.URL)
	o.InsecureTLS = true
	if _, _, _, err := Fetch(context.Background(), o, ""); err != nil {
		t.Fatalf("при явном InsecureTLS запрос должен проходить: %v", err)
	}
}

func TestTokenInHeaderNotInURLAndETag(t *testing.T) {
	var gotAuth, gotPath string
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.String()
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte("cfg"))
	})
	o := opts(s.URL + "/sub")
	o.PinSHA256 = pinOf(s)
	o.Token = "secret-token"
	_, etag, nm, err := Fetch(context.Background(), o, "")
	if err != nil || nm || etag != "v1" {
		t.Fatalf("первый запрос: etag=%q nm=%v err=%v", etag, nm, err)
	}
	if gotAuth != "Bearer secret-token" || strings.Contains(gotPath, "secret-token") {
		t.Fatalf("токен должен идти в заголовке, а не в URL: auth=%q path=%q", gotAuth, gotPath)
	}
	_, etag2, nm, err := Fetch(context.Background(), o, etag)
	if err != nil || !nm || etag2 != "v1" {
		t.Fatalf("условный запрос: nm=%v err=%v", nm, err)
	}
}

func TestStatusSizeAndEmpty(t *testing.T) {
	o := func(h http.HandlerFunc) (string, error) {
		s := newSrv(t, h)
		op := opts(s.URL)
		op.PinSHA256 = pinOf(s)
		b, _, _, err := Fetch(context.Background(), op, "")
		return b, err
	}
	if _, err := o(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }); err == nil {
		t.Error("HTTP 500 должен давать ошибку")
	}
	if _, err := o(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("  \n")) }); err == nil {
		t.Error("пустой ответ должен давать ошибку")
	}
	if _, err := o(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", MaxBody+10)))
	}); err == nil {
		t.Error("слишком большой ответ должен давать ошибку")
	}
}

func TestLoopbackForbiddenByDefault(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("cfg")) })
	o := Options{URL: s.URL, PinSHA256: pinOf(s)} // AllowLoopback не задан
	if _, _, _, err := Fetch(context.Background(), o, ""); err == nil {
		t.Fatal("loopback должен быть запрещён по умолчанию")
	}
}

func TestRedirectToHTTPBlocked(t *testing.T) {
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("leak")) }))
	defer plain.Close()
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, plain.URL, http.StatusFound) })
	o := opts(s.URL)
	o.PinSHA256 = pinOf(s)
	if _, _, _, err := Fetch(context.Background(), o, ""); err == nil {
		t.Fatal("перенаправление с https на http должно блокироваться")
	}
}

func TestNormalizePin(t *testing.T) {
	if p, err := NormalizePin(""); p != "" || err != nil {
		t.Fatal("пустой отпечаток допустим")
	}
	for _, bad := range []string{"abc", strings.Repeat("zz", 32)} {
		if _, err := NormalizePin(bad); err == nil {
			t.Errorf("%q должен отвергаться", bad)
		}
	}
}

func TestErrorDoesNotLeakURLWithToken(t *testing.T) {
	o := Options{URL: "https://127.0.0.1:1/sub/awg_tok_SECRET", AllowLoopback: true}
	_, _, _, err := Fetch(context.Background(), o, "")
	if err == nil {
		t.Fatal("ожидалась ошибка соединения")
	}
	if strings.Contains(err.Error(), "awg_tok_SECRET") {
		t.Fatalf("токен из пути URL попал в текст ошибки: %v", err)
	}
}

func spkiPinOf(s *httptest.Server) string {
	sum := sha256.Sum256(s.Certificate().RawSubjectPublicKeyInfo)
	return "sha256//" + base64.StdEncoding.EncodeToString(sum[:])
}

func TestSPKIPinLikeCurlPinnedPubkey(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("cfg")) })
	o := opts(s.URL)
	o.PinSHA256 = spkiPinOf(s)
	pin, err := NormalizePin(o.PinSHA256)
	if err != nil {
		t.Fatal(err)
	}
	o.PinSHA256 = pin
	if _, _, _, err := Fetch(context.Background(), o, ""); err != nil {
		t.Fatalf("верный пин открытого ключа должен приниматься: %v", err)
	}
	wrong := sha256.Sum256([]byte("другой ключ"))
	o.PinSHA256 = "sha256//" + base64.StdEncoding.EncodeToString(wrong[:])
	if _, _, _, err := Fetch(context.Background(), o, ""); err == nil {
		t.Fatal("чужой пин открытого ключа должен отвергаться")
	}
	// пин сертификата в hex не подходит к пину ключа и наоборот
	o.PinSHA256 = pinOf(s)
	if _, _, _, err := Fetch(context.Background(), o, ""); err != nil {
		t.Fatalf("hex-пин сертификата тоже должен работать: %v", err)
	}
}

func TestNormalizePinSPKI(t *testing.T) {
	good := "sha256//" + base64.StdEncoding.EncodeToString(make([]byte, 32))
	if got, err := NormalizePin("  SHA256//" + good[len("sha256//"):] + " "); err != nil || got != good {
		t.Fatalf("канонизация: %q, %v", got, err)
	}
	for _, bad := range []string{"sha256//", "sha256//AAAA", "sha256//" + base64.StdEncoding.EncodeToString(make([]byte, 31)), "sha256//не-base64!"} {
		if _, err := NormalizePin(bad); err == nil {
			t.Errorf("%q должен отвергаться", bad)
		}
	}
}

func TestMaskURLHidesPanelToken(t *testing.T) {
	const tok = "dGhpcy1pcy1hLXNlY3JldC10b2tlbi0xMjM0NTY3OA"
	got := MaskURL("https://vps.example.org:8443/sub/awg0/" + tok + "?k=" + tok)
	if strings.Contains(got, tok) || strings.Contains(got, "k="+tok[:4]) {
		t.Fatalf("токен остался в маске: %s", got)
	}
	if !strings.HasPrefix(got, "https://vps.example.org:8443/sub/awg0/") {
		t.Fatalf("хост и короткие сегменты должны сохраниться: %s", got)
	}
	if MaskURL("не url") != "" {
		t.Fatal("некорректный URL не должен возвращаться")
	}
}

func TestMaskURLHidesShortTokenAndUserinfo(t *testing.T) {
	got := MaskURL("https://user:pass@vps.example.org/sub/awg0/abc123")
	if strings.Contains(got, "abc123") || strings.Contains(got, "pass") || strings.Contains(got, "user") {
		t.Fatalf("короткий токен или учётные данные остались в маске: %s", got)
	}
	if got != "https://vps.example.org/sub/awg0/••••••" {
		t.Fatalf("неожиданная маска: %s", got)
	}
	if got := MaskURL("https://h.example/sub/awg0/abc123/"); strings.Contains(got, "abc123") {
		t.Fatalf("токен перед завершающим слэшем остался: %s", got)
	}
	// Секрет не всегда в конце пути: длинный сегмент скрывается где бы он ни стоял.
	const mid = "dGhpcy1pcy1hLXNlY3JldC10b2tlbi0xMjM0NTY3OA"
	if got := MaskURL("https://h.example/" + mid + "/awg0"); strings.Contains(got, mid) {
		t.Fatalf("длинный сегмент в середине пути остался: %s", got)
	}
}

func TestPanelStyleTokenInPathAndStatusCodes(t *testing.T) {
	const tok = "dGhpcy1pcy1hLXNlY3JldC10b2tlbi0xMjM0NTY3OA"
	var gotAuth string
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/sub/awg0/" + tok:
			if r.Header.Get("If-None-Match") == `"v1"` {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("ETag", `"v1"`)
			_, _ = w.Write([]byte("[Interface]\n"))
		case "/sub/awg0/off":
			http.Error(w, "The client is switched off", http.StatusForbidden)
		case "/sub/awg0/many":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
	o := opts(s.URL + "/sub/awg0/" + tok)
	o.PinSHA256 = spkiPinOf(s)
	o.PinSHA256, _ = NormalizePin(o.PinSHA256)
	body, etag, nm, err := Fetch(context.Background(), o, "")
	if err != nil || nm || etag != "v1" || !strings.Contains(body, "[Interface]") {
		t.Fatalf("первый запрос: %q %q %v %v", body, etag, nm, err)
	}
	if gotAuth != "" {
		t.Fatalf("без токена в поле Token заголовок Authorization отправляться не должен: %q", gotAuth)
	}
	if _, _, nm, err = Fetch(context.Background(), o, etag); err != nil || !nm {
		t.Fatalf("условный запрос должен дать 304: %v %v", nm, err)
	}
	for path, want := range map[string]int{"/sub/awg0/off": 403, "/sub/awg0/many": 429, "/sub/awg0/" + "x" + tok: 404} {
		o2 := o
		o2.URL = s.URL + path
		_, _, _, err := Fetch(context.Background(), o2, "")
		var se *StatusError
		if !errors.As(err, &se) || se.Code != want {
			t.Fatalf("%s: ожидался StatusError %d, получено %v", path, want, err)
		}
		if strings.Contains(err.Error(), tok) {
			t.Fatalf("токен попал в текст ошибки: %v", err)
		}
	}
}

func TestHintForTypicalNetworkFailures(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"отмена", context.Canceled, "masquerade"},
		{"DNS", &net.DNSError{Err: "no such host", Name: "x.invalid"}, "по имени"},
		{"отказ", syscall.ECONNREFUSED, "адрес и порт"},
		{"нет маршрута", syscall.ENETUNREACH, "нет маршрута"},
		{"таймаут", context.DeadlineExceeded, "не ответил вовремя"},
		{"таймаут соединения", &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded}, "не ответил вовремя"},
	} {
		got := explain("h:8443", c.err)
		if !strings.Contains(got.Error(), "запрос к h:8443 не удался") || !strings.Contains(got.Error(), c.want) {
			t.Errorf("%s: %q не содержит %q", c.name, got, c.want)
		}
		if !errors.Is(got, c.err) {
			t.Errorf("%s: исходная ошибка потеряна из цепочки", c.name)
		}
	}
	if h := hint(errors.New("что-то своё")); h != "" {
		t.Errorf("для неизвестной ошибки подсказки быть не должно: %q", h)
	}
}

func TestFetchCancelledAndTimeoutGiveReadableErrors(t *testing.T) {
	s := newSrv(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := Fetch(ctx, Options{URL: s.URL, AllowLoopback: true, PinSHA256: pinOf(s)}, "")
	if err == nil || !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "выход в интернет") {
		t.Fatalf("отмена: %v", err)
	}
	_, _, _, err = Fetch(context.Background(), Options{URL: s.URL, AllowLoopback: true, PinSHA256: pinOf(s), Timeout: 200 * time.Millisecond}, "")
	if err == nil || !strings.Contains(err.Error(), "не ответил вовремя") {
		t.Fatalf("таймаут: %v", err)
	}
}
