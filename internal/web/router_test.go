package web

import (
	"net/http"
	"strings"
	"testing"

	"awg31/internal/ros"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func routerEnv(t *testing.T, gateway bool) *env {
	o := tunnel.Options{RunDir: t.TempDir()}
	if gateway {
		o.AutoRoute, o.LanIface = true, "eth0"
	}
	return newEnvOpts(t, o, func(c *Config) {
		c.Router = &ros.Manager{Store: c.Store, Tunnels: c.Tunnels}
	})
}

func TestRouterPasswordNeverReturned(t *testing.T) {
	e := routerEnv(t, true)
	if code, _, _ := e.do("GET", "/api/router", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", code)
	}
	e.setup()
	r := store.DefaultRouter()
	r.Mode, r.Pass = store.RouterAddressList, "Sup3r-Secret!"
	code, _, raw := e.do("PUT", "/api/router", r, nil)
	if code != 200 || strings.Contains(raw, "Sup3r-Secret!") {
		t.Fatalf("сохранение: %d, пароль в ответе: %v", code, strings.Contains(raw, "Sup3r-Secret!"))
	}
	_, m, raw := e.do("GET", "/api/router", nil, nil)
	if strings.Contains(raw, "Sup3r-Secret!") || m["settings"].(map[string]any)["pass_set"] != true {
		t.Fatalf("GET не отдаёт пароль, но сообщает, что он задан: %s", raw)
	}
	// пустой пароль при сохранении — прежний остаётся
	r.Pass, r.AddressList = "", "vpn"
	if code, _, _ := e.do("PUT", "/api/router", r, nil); code != 200 {
		t.Fatal(code)
	}
	if got := e.st.Snapshot().RouterSettings(); got.Pass != "Sup3r-Secret!" || got.AddressList != "vpn" {
		t.Fatalf("пароль должен сохраниться: %+v", got)
	}
	r.URL = "http://203.0.113.1"
	code, m, _ = e.do("PUT", "/api/router", r, nil)
	if code != http.StatusBadRequest || !strings.Contains(m["error"].(string), "http") {
		t.Fatalf("http на публичный адрес: %d %v", code, m)
	}
}

func TestRouterNeedsGateway(t *testing.T) {
	e := routerEnv(t, false)
	e.setup()
	r := store.DefaultRouter()
	r.Mode = store.RouterBGP
	if code, _, _ := e.do("PUT", "/api/router", r, nil); code != http.StatusConflict {
		t.Fatalf("без режима шлюза: %d", code)
	}
	r.Mode = store.RouterOff
	if code, _, _ := e.do("PUT", "/api/router", r, nil); code != 200 {
		t.Fatalf("off сохраняется: %d", code)
	}
}
