package web

import (
	"net/http"
	"testing"

	"awg31/internal/failover"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func gatewayEnv(t *testing.T) *env {
	return newEnvOpts(t, tunnel.Options{RunDir: t.TempDir(), AutoRoute: true, LanIface: "eth0", KillSwitch: true}, func(c *Config) {
		c.Failover = &failover.Monitor{Store: c.Store, Tunnels: c.Tunnels, Gateway: true}
	})
}

func TestFailoverAPIRequiresAuthAndValidates(t *testing.T) {
	e := gatewayEnv(t)
	if code, _, _ := e.do("GET", "/api/failover", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", code)
	}
	e.setup()
	for _, n := range []string{"a", "b"} {
		if code, _, raw := e.do("PUT", "/api/profiles/"+n, map[string]string{"content": goodConf()}, nil); code != 200 {
			t.Fatalf("профиль %s: %d %s", n, code, raw)
		}
	}
	good := store.DefaultFailover()
	good.Mode = store.FailoverStandby
	bad := good
	bad.ProbeTarget = "example.com:443"
	if code, _, _ := e.do("PUT", "/api/failover", map[string]any{"settings": bad}, nil); code != http.StatusBadRequest {
		t.Fatalf("имя хоста в цели пробы должно отвергаться: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/failover", map[string]any{"settings": good, "priority": []string{"a", "a"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("повтор в приоритете: %d", code)
	}
	if code, _, _ := e.do("PUT", "/api/failover", map[string]any{"settings": good, "priority": []string{"../x"}}, nil); code != http.StatusBadRequest {
		t.Fatalf("недопустимое имя в приоритете: %d", code)
	}
	code, m, raw := e.do("PUT", "/api/failover", map[string]any{"settings": good, "priority": []string{"b", "a"}}, nil)
	if code != 200 {
		t.Fatalf("сохранение: %d %s", code, raw)
	}
	if order := m["order"].([]any); len(order) != 2 || order[0] != "b" {
		t.Fatalf("порядок: %v", m["order"])
	}
	snap := e.st.Snapshot()
	if snap.FailoverSettings().Mode != store.FailoverStandby || len(snap.Priority) != 2 {
		t.Fatalf("настройки не сохранились: %+v %v", snap.Failover, snap.Priority)
	}
	if code, m, _ := e.do("POST", "/api/failover/check", nil, nil); code != 200 || m["gateway"] != true {
		t.Fatalf("проверка сейчас: %d %v", code, m)
	}
}

func TestFailoverNeedsGatewayMode(t *testing.T) {
	e := newEnv(t) // без LAN-интерфейса
	e.setup()
	f := store.DefaultFailover()
	f.Mode = store.FailoverStandby
	if code, _, _ := e.do("PUT", "/api/failover", map[string]any{"settings": f}, nil); code != http.StatusConflict {
		t.Fatalf("без режима шлюза резервирование включаться не должно: %d", code)
	}
	f.Mode = store.FailoverOff
	if code, _, _ := e.do("PUT", "/api/failover", map[string]any{"settings": f}, nil); code != 200 {
		t.Fatalf("режим off сохраняется всегда: %d", code)
	}
}

func TestManualPrimary(t *testing.T) {
	e := gatewayEnv(t)
	e.setup()
	for _, n := range []string{"a", "b"} {
		e.do("PUT", "/api/profiles/"+n, map[string]string{"content": goodConf()}, nil)
	}
	if code, _, _ := e.do("POST", "/api/profiles/b/primary", nil, nil); code != http.StatusConflict {
		t.Fatalf("основным может быть только поднятый туннель: %d", code)
	}
	for _, n := range []string{"a", "b"} {
		if code, _, raw := e.do("POST", "/api/profiles/"+n+"/up", nil, nil); code != 200 {
			t.Fatalf("подъём %s: %d %s", n, code, raw)
		}
	}
	_, m, _ := e.do("GET", "/api/status", nil, nil)
	if m["primary"] != "a" {
		t.Fatalf("первый поднятый — основной: %v", m["primary"])
	}
	if code, _, raw := e.do("POST", "/api/profiles/b/primary", nil, nil); code != 200 {
		t.Fatalf("ручной выбор: %d %s", code, raw)
	}
	_, m, _ = e.do("GET", "/api/status", nil, nil)
	if m["primary"] != "b" || e.st.Snapshot().Primary != "b" {
		t.Fatalf("основной после выбора: %v / %q", m["primary"], e.st.Snapshot().Primary)
	}
	if code, _, _ := e.do("POST", "/api/profiles/..%2fx/primary", nil, nil); code != http.StatusBadRequest && code != http.StatusNotFound {
		t.Fatalf("недопустимое имя: %d", code)
	}
}
