package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"awg31/internal/geo"
	"awg31/internal/policy"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func policyEnv(t *testing.T, gateway bool) *env {
	o := tunnel.Options{RunDir: t.TempDir()}
	if gateway {
		o.AutoRoute, o.LanIface = true, "eth0"
	}
	return newEnvOpts(t, o, func(c *Config) {
		c.Geo = &geo.Store{Dir: filepath.Join(t.TempDir(), "geo")}
		c.Policy = &policy.Engine{Store: c.Store, Geo: c.Geo, Tunnels: c.Tunnels}
		c.AllowHTTPSubs = true
	})
}

func TestPolicyAPI(t *testing.T) {
	e := policyEnv(t, true)
	if code, _, _ := e.do("GET", "/api/policy", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", code)
	}
	e.setup()
	p := store.DefaultPolicy()
	p.Enabled = true
	p.Rules = "PROXY youtube.com\nFORWARD x\nDIRECT 10.0.0.0/8\nPROXY@bad;x a.com\n"
	p.ProxyDNS = "dns.google"
	code, m, _ := e.do("PUT", "/api/policy", p, nil)
	if code != http.StatusBadRequest {
		t.Fatalf("ошибки должны отвергаться: %d", code)
	}
	errs := m["errors"].([]any)
	joined := ""
	for _, x := range errs {
		joined += x.(string) + "\n"
	}
	if len(errs) != 3 || !strings.Contains(joined, "строка 2") || !strings.Contains(joined, "строка 4") || !strings.Contains(joined, "DNS через VPN") {
		t.Fatalf("все ошибки сразу, с номерами строк: %v", errs)
	}
	p.Rules = "PROXY youtube.com\nDIRECT 10.0.0.0/8\n"
	p.ProxyDNS = "1.1.1.1, https://dns.google/dns-query"
	code, m, raw := e.do("PUT", "/api/policy", p, nil)
	if code != 200 {
		t.Fatalf("сохранение: %d %s", code, raw)
	}
	if st := m["stats"].(map[string]any); st["rules"].(float64) != 2 || st["enabled"] != true {
		t.Fatalf("статистика: %v", st)
	}
	code, m, _ = e.do("POST", "/api/policy/test", map[string]string{"query": "www.YouTube.com"}, nil)
	if code != 200 || m["action"] != "PROXY" || m["matched_by"] != "domain:youtube.com" {
		t.Fatalf("проверка имени: %d %v", code, m)
	}
	if _, m, _ = e.do("POST", "/api/policy/test", map[string]string{"query": "10.1.2.3"}, nil); m["action"] != "DIRECT" {
		t.Fatalf("проверка адреса: %v", m)
	}
}

func TestPolicyNeedsGateway(t *testing.T) {
	e := policyEnv(t, false)
	e.setup()
	p := store.DefaultPolicy()
	p.Enabled = true
	if code, _, _ := e.do("PUT", "/api/policy", p, nil); code != http.StatusConflict {
		t.Fatalf("без режима шлюза политика не включается: %d", code)
	}
	p.Enabled = false
	if code, _, _ := e.do("PUT", "/api/policy", p, nil); code != 200 {
		t.Fatalf("выключенная сохраняется: %d", code)
	}
}

func TestGeoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("youtube.com\n"))
	}))
	defer srv.Close()
	e := policyEnv(t, true)
	e.setup()
	p := store.DefaultPolicy()
	p.Enabled, p.Rules, p.GeoSiteURL, p.GeoIPURL = true, "PROXY geosite:youtube\n", srv.URL+"/geosite.dat", ""
	if code, _, raw := e.do("PUT", "/api/policy", p, nil); code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	code, m, raw := e.do("POST", "/api/geo/update", nil, nil)
	if code != 200 {
		t.Fatalf("обновление баз: %d %s", code, raw)
	}
	g := m["geo"].(map[string]any)
	if g["geosite"].(map[string]any)["present"] != true || g["site_loaded"].(map[string]any)["youtube"].(float64) != 1 {
		t.Fatalf("база скачана и категория загружена: %v", g)
	}
	if _, m, _ = e.do("POST", "/api/policy/test", map[string]string{"query": "music.youtube.com"}, nil); m["matched_by"] != "geosite:youtube" {
		t.Fatalf("geosite после обновления: %v", m)
	}
}

// Включение правил с geosite на свежем контейнере сразу скачивает недостающие
// базы, без ошибки «no such file or directory».
func TestPolicySaveFetchesMissingGeo(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte("youtube.com\n"))
	}))
	defer srv.Close()
	e := policyEnv(t, true)
	e.setup()
	p := store.DefaultPolicy()
	p.Enabled, p.Rules, p.GeoSiteURL, p.GeoIPURL = true, "PROXY geosite:youtube\n", srv.URL+"/geosite.dat", srv.URL+"/geoip.dat"
	code, m, raw := e.do("PUT", "/api/policy", p, nil)
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	if errs, _ := m["errors"].([]any); len(errs) != 0 {
		t.Fatalf("ошибок быть не должно: %v", errs)
	}
	if hits != 1 {
		t.Fatalf("качается только geosite (geoip правилам не нужна): %d запросов", hits)
	}
	g := m["geo"].(map[string]any)
	if g["geosite"].(map[string]any)["present"] != true || g["site_loaded"].(map[string]any)["youtube"].(float64) != 1 {
		t.Fatalf("база скачана и загружена: %v", g)
	}
	// повторное сохранение уже имеющиеся базы не качает
	if code, _, raw = e.do("PUT", "/api/policy", p, nil); code != 200 || hits != 1 {
		t.Fatalf("повторное сохранение: %d %s, запросов %d", code, raw, hits)
	}
}

// Если скачать не удалось, правила всё равно сохранены, а ошибка понятная.
func TestPolicySaveGeoDownloadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "нет", http.StatusInternalServerError)
	}))
	defer srv.Close()
	e := policyEnv(t, true)
	e.setup()
	p := store.DefaultPolicy()
	p.Enabled, p.Rules, p.GeoSiteURL = true, "PROXY geosite:youtube\n", srv.URL+"/geosite.dat"
	code, m, raw := e.do("PUT", "/api/policy", p, nil)
	if code != 200 {
		t.Fatalf("правила сохраняются и при сбое загрузки: %d %s", code, raw)
	}
	errs, _ := m["errors"].([]any)
	if len(errs) != 1 || !strings.Contains(errs[0].(string), "не удалось скачать базы geo") {
		t.Fatalf("ожидалась ошибка загрузки: %v", errs)
	}
}

func TestGeoCategoriesSearch(t *testing.T) {
	e := policyEnv(t, true)
	if code, _, _ := e.do("GET", "/api/geo/categories?kind=geosite&q=you", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", code)
	}
	e.setup()
	dir := e.srv.cfg.Geo.Dir
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	// geosite.dat в двоичном виде: три категории
	pb := func(num int, v []byte) []byte {
		n := len(v)
		l := []byte{byte(num<<3 | 2)}
		for n >= 0x80 {
			l = append(l, byte(n)|0x80)
			n >>= 7
		}
		return append(append(l, byte(n)), v...)
	}
	var data []byte
	for _, c := range []string{"YOUTUBE", "my-youtube-mirror", "telegram"} {
		data = append(data, pb(1, pb(1, []byte(c)))...)
	}
	if err := os.WriteFile(filepath.Join(dir, "geosite.dat"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	items := func(q string) []any {
		code, m, raw := e.do("GET", "/api/geo/categories?kind=geosite&q="+q, nil, nil)
		if code != 200 {
			t.Fatalf("%s: %d %s", q, code, raw)
		}
		return m["items"].([]any)
	}
	if got := items("you"); len(got) != 2 || got[0] != "youtube" || got[1] != "my-youtube-mirror" {
		t.Fatalf("сначала совпадения с начала имени, потом из середины: %v", got)
	}
	if got := items("y"); len(got) != 0 {
		t.Fatalf("короче двух символов не ищем: %v", got)
	}
	if got := items("nothing"); len(got) != 0 {
		t.Fatalf("нет совпадений: %v", got)
	}
	if code, _, _ := e.do("GET", "/api/geo/categories?kind=bogus&q=you", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("неизвестный вид: %d", code)
	}
	// all=1 отдаёт весь список (для каталога IPList), без поиска и без лимита подсказок
	code, m, raw := e.do("GET", "/api/geo/categories?kind=geosite&all=1", nil, nil)
	if got, _ := m["items"].([]any); code != 200 || len(got) != 3 {
		t.Fatalf("весь список: %d %s", code, raw)
	}
	// базы geoip нет: пустой список, а не null и не ошибка
	code, m, raw = e.do("GET", "/api/geo/categories?kind=geoip&all=1", nil, nil)
	if got, ok := m["items"].([]any); code != 200 || !ok || len(got) != 0 {
		t.Fatalf("пустой список: %d %s", code, raw)
	}
}
