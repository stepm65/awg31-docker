package web

import (
	"net/http"
	"net/http/httptest"
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
