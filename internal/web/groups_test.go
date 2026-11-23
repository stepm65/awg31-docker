package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"awg31/internal/failover"
	"awg31/internal/group"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

func groupsEnv(t *testing.T, gateway bool) *env {
	t.Helper()
	dir := t.TempDir()
	group.HashPolicyV4, group.HashPolicyV6 = filepath.Join(dir, "v4"), filepath.Join(dir, "v6")
	_ = os.WriteFile(group.HashPolicyV4, []byte("0\n"), 0o644)
	_ = os.WriteFile(group.HashPolicyV6, []byte("0\n"), 0o644)
	o := tunnel.Options{RunDir: t.TempDir()}
	if gateway {
		o = tunnel.Options{RunDir: t.TempDir(), AutoRoute: true, LanIface: "eth0", KillSwitch: true}
	}
	return newEnvOpts(t, o, func(c *Config) {
		g := &group.Manager{Store: c.Store, Tunnels: c.Tunnels, Runner: &fakeRunner{}}
		c.Tunnels.SetGroups(g)
		c.Groups = g
		c.Failover = &failover.Monitor{Store: c.Store, Tunnels: c.Tunnels, Gateway: gateway, OnHealth: g.SetHealth}
	})
}

func serverConf() string {
	return "[Interface]\nPrivateKey = " + secretKey + "\nAddress = 10.8.0.1/24\nListenPort = 51820\n\n[Peer]\nPublicKey = " + pubKey + "\nAllowedIPs = 10.8.0.2/32\n"
}

func TestGroupsAPIValidationAndGatewayRequirement(t *testing.T) {
	e := groupsEnv(t, true)
	if code, _, _ := e.do("GET", "/api/groups", nil, nil); code != http.StatusUnauthorized {
		t.Fatalf("без входа: %d", code)
	}
	e.setup()
	for n, c := range map[string]string{"a": goodConf(), "b": goodConf(), "srv": serverConf()} {
		if code, _, raw := e.do("PUT", "/api/profiles/"+n, map[string]string{"content": c}, nil); code != 200 {
			t.Fatalf("профиль %s: %d %s", n, code, raw)
		}
	}
	put := func(g ...store.Group) int {
		code, _, _ := e.do("PUT", "/api/groups", map[string]any{"groups": g}, nil)
		return code
	}
	mem := func(n ...string) []store.GroupMember {
		var out []store.GroupMember
		for _, x := range n {
			out = append(out, store.GroupMember{Name: x})
		}
		return out
	}
	if put(store.Group{Name: "a", Mode: store.GroupBalance, Members: mem("b")}) != http.StatusBadRequest {
		t.Error("имя группы совпадает с профилем")
	}
	if put(store.Group{Name: "pool", Mode: "random", Members: mem("a")}) != http.StatusBadRequest {
		t.Error("недопустимый режим")
	}
	if put(store.Group{Name: "pool", Mode: store.GroupBalance, Members: mem("nope")}) != http.StatusBadRequest {
		t.Error("несуществующий участник")
	}
	if put(store.Group{Name: "pool", Mode: store.GroupBalance, Members: mem("a", "srv")}) != http.StatusBadRequest {
		t.Error("серверный профиль в группу добавить нельзя")
	}
	if len(e.st.Snapshot().Groups) != 0 {
		t.Fatal("после отказов ничего не сохраняется")
	}
	// профиль не может занять имя группы
	if put(store.Group{Name: "pool", Mode: store.GroupBalance, Members: mem("a", "b")}) != 200 {
		t.Fatal("корректная группа")
	}
	if code, _, _ := e.do("PUT", "/api/profiles/pool", map[string]string{"content": goodConf()}, nil); code != http.StatusBadRequest {
		t.Fatalf("имя занято группой: %d", code)
	}
	// без режима шлюза группы не включаются
	e2 := groupsEnv(t, false)
	e2.setup()
	e2.do("PUT", "/api/profiles/a", map[string]string{"content": goodConf()}, nil)
	code, _, _ := e2.do("PUT", "/api/groups", map[string]any{"groups": []store.Group{{Name: "pool", Mode: store.GroupBalance, Members: mem("a")}}}, nil)
	if code != http.StatusConflict {
		t.Fatalf("без режима шлюза: %d", code)
	}
	if code, _, _ := e2.do("PUT", "/api/groups", map[string]any{"groups": []store.Group{}}, nil); code != 200 {
		t.Fatalf("пустой список сохраняется всегда: %d", code)
	}
}

func TestGroupsLifecycleHealthAndPrimary(t *testing.T) {
	e := groupsEnv(t, true)
	e.setup()
	for _, n := range []string{"a", "b"} {
		e.do("PUT", "/api/profiles/"+n, map[string]string{"content": goodConf()}, nil)
	}
	g := []store.Group{{Name: "pool", Mode: store.GroupBalance, Members: []store.GroupMember{{Name: "a", Weight: 2}, {Name: "b"}}}}
	code, m, raw := e.do("PUT", "/api/groups", map[string]any{"groups": g}, nil)
	if code != 200 {
		t.Fatalf("сохранение: %d %s", code, raw)
	}
	if !e.run.up["a"] || !e.run.up["b"] {
		t.Fatalf("участники группы поднимаются сразу: %v", e.run.up)
	}
	// таблица группы выделяется сразу при сохранении, не дожидаясь проверки здоровья
	first := m["view"].(map[string]any)["status"].([]any)[0].(map[string]any)
	if tbl, _ := first["table"].(float64); tbl < float64(tunnel.GroupTableMin) || tbl > float64(tunnel.GroupTableMax) {
		t.Fatalf("после сохранения у группы нет таблицы: %v", first)
	}
	if st := e.st.Snapshot(); !st.Up["a"] || !st.Up["b"] {
		t.Fatalf("и запоминаются для автозапуска: %v", st.Up)
	}
	// здоровье приходит от монитора: рукопожатие свежее
	e.run.handshake = time.Now().Unix()
	f := store.DefaultFailover()
	f.Trigger = "handshake"
	_ = e.st.Update(func(s *store.State) error { s.Failover = &f; return nil })
	if code, _, raw := e.do("POST", "/api/failover/check", nil, nil); code != 200 {
		t.Fatalf("проверка: %d %s", code, raw)
	}
	_, m, _ = e.do("GET", "/api/groups", nil, nil)
	status := m["status"].([]any)[0].(map[string]any)
	if status["active"] != float64(2) || status["route"] != "nexthop dev a weight 2 nexthop dev b weight 1" {
		t.Fatalf("после проверки в группе оба участника: %v", status)
	}
	if m["health"].(map[string]any)["a"].(map[string]any)["state"] != "healthy" {
		t.Fatalf("здоровье: %v", m["health"])
	}
	if m["hash_policy"] != "1" {
		t.Fatalf("для balance включается хеш по портам: %v", m["hash_policy"])
	}
	if profiles := m["profiles"].([]any); len(profiles) != 2 {
		t.Fatalf("кандидаты: %v", profiles)
	}
	// группа — основной
	if code, _, raw := e.do("POST", "/api/groups/pool/primary", nil, nil); code != 200 {
		t.Fatalf("основная группа: %d %s", code, raw)
	}
	if e.st.Snapshot().Primary != "pool" {
		t.Fatalf("основной сохранён: %q", e.st.Snapshot().Primary)
	}
	_, m, _ = e.do("GET", "/api/groups", nil, nil)
	if m["primary"] != "pool" || !m["status"].([]any)[0].(map[string]any)["primary"].(bool) {
		t.Fatalf("основной в статусе: %v", m["primary"])
	}
	if code, _, _ := e.do("POST", "/api/groups/nope/primary", nil, nil); code != http.StatusNotFound {
		t.Fatalf("нет такой группы: %d", code)
	}
	// ручной выбор туннеля снова делает основным туннель
	if code, _, raw := e.do("POST", "/api/profiles/b/primary", nil, nil); code != 200 {
		t.Fatalf("туннель основным: %d %s", code, raw)
	}
	if e.srv.cfg.Tunnels.Primary() != "b" {
		t.Fatalf("основной: %q", e.srv.cfg.Tunnels.Primary())
	}
	// удаление группы, бывшей основной: основной возвращается на туннель
	e.do("POST", "/api/groups/pool/primary", nil, nil)
	if code, _, raw := e.do("PUT", "/api/groups", map[string]any{"groups": []store.Group{}}, nil); code != 200 {
		t.Fatalf("удаление: %d %s", code, raw)
	}
	if p := e.srv.cfg.Tunnels.Primary(); p != "a" && p != "b" {
		t.Fatalf("после удаления группы основным должен стать туннель: %q", p)
	}
	if e.st.Snapshot().Primary != "" && strings.HasPrefix(e.st.Snapshot().Primary, "pool") {
		t.Fatalf("сохранённый основной не должен указывать на удалённую группу: %q", e.st.Snapshot().Primary)
	}
}
