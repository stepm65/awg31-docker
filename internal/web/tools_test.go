package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"awg31/internal/store"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	e := policyEnv(t, true)
	e.setup()
	if err := e.st.WriteProfile("main", goodConf()); err != nil {
		t.Fatal(err)
	}
	_ = e.st.Update(func(s *store.State) error {
		s.Devices = []store.Device{{ID: "pc", Name: "PC", Addr: "10.0.0.2", Kind: "pc"}}
		p := store.DefaultPolicy()
		p.Rules = "DIRECT example.com src=@pc\n"
		s.Policy = &p
		return nil
	})
	resp, err := e.client.Get(e.ts.URL + "/api/backup")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(resp.Header.Get("Content-Disposition"), "awg31-backup-") || strings.Contains(string(raw), "pass_hash") {
		t.Fatalf("%d %s\n%s", resp.StatusCode, resp.Header.Get("Content-Disposition"), raw)
	}
	// всё поменялось после копии
	_ = e.st.DeleteProfile("main")
	_ = e.st.WriteProfile("other", goodConf())
	_ = e.st.Update(func(s *store.State) error { s.Devices = nil; s.Policy = nil; return nil })

	var backup json.RawMessage = raw
	if code, _, _ := e.do("POST", "/api/backup/restore", map[string]any{"password": "wrong password!!", "backup": backup}, nil); code != http.StatusForbidden {
		t.Fatalf("без пароля: %d", code)
	}
	if code, _, raw := e.do("POST", "/api/backup/restore", map[string]any{"password": "correct horse battery", "backup": json.RawMessage(`{"format":"x"}`)}, nil); code != http.StatusBadRequest {
		t.Fatalf("не копия: %d %s", code, raw)
	}
	code, _, body := e.do("POST", "/api/backup/restore", map[string]any{"password": "correct horse battery", "backup": backup}, nil)
	if code != 200 {
		t.Fatalf("%d %s", code, body)
	}
	names, _ := e.st.ListProfiles()
	snap := e.st.Snapshot()
	if strings.Join(names, ",") != "main" || len(snap.Devices) != 1 || !strings.Contains(snap.PolicySettings().Rules, "src=@pc") || !snap.Configured() {
		t.Fatalf("профили %v, состояние %+v", names, snap)
	}
	// вход администратора не сбит восстановлением
	if code, _, _ := e.do("GET", "/api/status", nil, nil); code != 200 {
		t.Fatalf("сессия: %d", code)
	}
	// экспорт правил
	resp, _ = e.client.Get(e.ts.URL + "/api/policy/export")
	txt, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(txt) != "DIRECT example.com src=@pc\n" || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/plain") {
		t.Fatalf("%q", txt)
	}
}

func TestRouteTestWithDevice(t *testing.T) {
	e := policyEnv(t, true)
	e.setup()
	_ = e.st.Update(func(s *store.State) error {
		s.Devices = []store.Device{{ID: "pc", Name: "PC", Addr: "10.0.0.2", Kind: "pc"}}
		p := store.DefaultPolicy()
		p.Enabled, p.DefaultAction = true, "proxy"
		p.Rules = "#@ Игры\nDIRECT example.com src=@pc port=443\n\n#@ Общее\nDROP example.com\n"
		s.Policy = &p
		return nil
	})
	_ = e.srv.cfg.Policy.Reload()
	_, m, _ := e.do("POST", "/api/policy/test", map[string]any{"query": "www.example.com", "src": "@pc", "port": 443}, nil)
	if m["action"] != "DIRECT" || m["rule"] != "Игры" {
		t.Fatalf("%v", m)
	}
	_, m, _ = e.do("POST", "/api/policy/test", map[string]any{"query": "www.example.com"}, nil)
	if m["action"] != "DROP" || m["rule"] != "Общее" {
		t.Fatalf("%v", m)
	}
	if code, _, _ := e.do("POST", "/api/policy/test", map[string]any{"query": "x.com", "src": "@nope"}, nil); code != http.StatusBadRequest {
		t.Fatalf("%d", code)
	}
}
