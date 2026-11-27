package web

import (
	"net/http"
	"strings"
	"testing"

	"awg31/internal/store"
)

func TestDevicesAPIAndRuleRefs(t *testing.T) {
	e := policyEnv(t, true)
	e.setup()
	code, m, raw := e.do("PUT", "/api/devices", map[string]any{"devices": []map[string]string{{"name": "PC", "addr": "192.168.51.150", "kind": "pc"}, {"id": "tv", "name": "TV", "addr": "192.168.51.10", "kind": "tv"}}}, nil)
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	if devs := m["devices"].([]any); len(devs) != 2 || devs[0].(map[string]any)["id"] != "d1" {
		t.Fatalf("%v", devs)
	}
	// правило со ссылкой на неизвестное устройство не сохраняется
	p := store.DefaultPolicy()
	p.Rules = "DIRECT example.com src=@ghost\n"
	if code, _, raw := e.do("PUT", "/api/policy", p, nil); code != http.StatusBadRequest || !strings.Contains(raw, "@ghost") {
		t.Fatalf("%d %s", code, raw)
	}
	p.Rules = "DIRECT example.com src=@tv\n"
	if code, _, raw := e.do("PUT", "/api/policy", p, nil); code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	// устройство из правил удалить нельзя
	code, _, raw = e.do("PUT", "/api/devices", map[string]any{"devices": []map[string]string{{"id": "d1", "name": "PC", "addr": "192.168.51.150"}}}, nil)
	if code != http.StatusConflict || !strings.Contains(raw, "@tv") {
		t.Fatalf("%d %s", code, raw)
	}
	// без интеграции с RouterOS аренды DHCP недоступны
	if code, _, _ := e.do("GET", "/api/devices/leases", nil, nil); code != http.StatusConflict {
		t.Fatalf("аренды: %d", code)
	}
	if code, _, raw := e.do("PUT", "/api/devices", map[string]any{"devices": []map[string]string{{"name": "X", "addr": "nope"}}}, nil); code != http.StatusBadRequest {
		t.Fatalf("плохой адрес: %d %s", code, raw)
	}
}
