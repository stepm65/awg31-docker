package store

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var key = base64.StdEncoding.EncodeToString(make([]byte, 32))

var goodConf = "[Interface]\nPrivateKey = " + key + "\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = " + key + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 192.0.2.1:51820\n"

func TestValidName(t *testing.T) {
	for _, ok := range []string{"a", "wg0", "my-vpn_1", "123456789012345"} {
		if !ValidName(ok) {
			t.Errorf("%q должно быть допустимым", ok)
		}
	}
	for _, bad := range []string{"", "../etc/passwd", "a/b", "a.conf", "a b", "1234567890123456", "имя", "a\x00", ".hidden", "a;rm"} {
		if ValidName(bad) {
			t.Errorf("%q не должно быть допустимым", bad)
		}
	}
}

func TestProfileLifecycleAndPermissions(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.WriteProfile("wg0", goodConf); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "profiles", "wg0.conf"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("права профиля %v, ожидалось 0600", info.Mode().Perm())
	}
	got, err := s.ReadProfile("wg0")
	if err != nil || got != goodConf {
		t.Fatalf("чтение: %v", err)
	}
	names, _ := s.ListProfiles()
	if len(names) != 1 || names[0] != "wg0" {
		t.Fatalf("список: %v", names)
	}
	if err := s.DeleteProfile("wg0"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadProfile("wg0"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ожидался ErrNotFound, получено %v", err)
	}
	if err := s.DeleteProfile("wg0"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("повторное удаление: %v", err)
	}
}

func TestWriteRejectsTraversalAndBadConfig(t *testing.T) {
	s, _ := Open(t.TempDir())
	for _, name := range []string{"../evil", "a/b", ""} {
		if s.WriteProfile(name, goodConf) == nil {
			t.Errorf("имя %q должно отвергаться", name)
		}
	}
	if s.WriteProfile("ok", "[Interface]\nPostUp = rm -rf /\n") == nil {
		t.Error("конфиг с хуком должен отвергаться")
	}
	if names, _ := s.ListProfiles(); len(names) != 0 {
		t.Errorf("после отказов профилей быть не должно: %v", names)
	}
}

func TestStatePersistsAndIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	if err := s.Update(func(st *State) error { st.User = "admin"; st.PassHash = "h"; st.Up["wg0"] = true; return nil }); err != nil {
		t.Fatal(err)
	}
	// ошибка в fn не должна менять состояние
	_ = s.Update(func(st *State) error { st.User = "hacker"; return errors.New("отмена") })
	if s.Snapshot().User != "admin" {
		t.Fatal("состояние изменилось несмотря на ошибку")
	}
	info, _ := os.Stat(filepath.Join(dir, "state.json"))
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("права state.json %v", info.Mode().Perm())
	}
	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := s2.Snapshot()
	if !st.Configured() || !st.Up["wg0"] {
		t.Fatalf("состояние не восстановлено: %+v", st)
	}
	// временных файлов не остаётся
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) > 4 && e.Name()[:5] == ".tmp-" {
			t.Fatalf("остался временный файл %s", e.Name())
		}
	}
}

func TestDeleteProfileCleansState(t *testing.T) {
	s, _ := Open(t.TempDir())
	_ = s.WriteProfile("wg0", goodConf)
	_ = s.Update(func(st *State) error {
		st.Subs["wg0"] = Sub{URL: "https://x"}
		st.Up["wg0"] = true
		return nil
	})
	_ = s.DeleteProfile("wg0")
	st := s.Snapshot()
	if _, ok := st.Subs["wg0"]; ok || st.Up["wg0"] {
		t.Fatalf("записи состояния должны удаляться вместе с профилем: %+v", st)
	}
}

func TestOpenCorruptedState(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "state.json"), []byte("{not json"), 0o600)
	if _, err := Open(dir); err == nil {
		t.Fatal("повреждённый state.json должен давать ошибку, а не молча сбрасывать пароль")
	}
}

func TestValidateGroups(t *testing.T) {
	profiles := []string{"a", "b", "c"}
	ok := []Group{{Name: "pool", Mode: GroupBalance, Members: []GroupMember{{Name: "a", Weight: 3}, {Name: "b"}}}}
	if err := ValidateGroups(ok, profiles); err != nil {
		t.Fatal(err)
	}
	cases := map[string][]Group{
		"имя совпадает с профилем": {{Name: "a", Mode: GroupBalance}},
		"недопустимое имя":         {{Name: "bad name", Mode: GroupBalance}},
		"режим":                    {{Name: "g", Mode: "random"}},
		"повтор группы":            {{Name: "g", Mode: GroupBalance}, {Name: "G", Mode: GroupOrdered}},
		"повтор участника":         {{Name: "g", Mode: GroupBalance, Members: []GroupMember{{Name: "a"}, {Name: "a"}}}},
		"нет такого профиля":       {{Name: "g", Mode: GroupBalance, Members: []GroupMember{{Name: "zzz"}}}},
		"вес больше предела":       {{Name: "g", Mode: GroupBalance, Members: []GroupMember{{Name: "a", Weight: MaxGroupWeight + 1}}}},
		"отрицательный вес":        {{Name: "g", Mode: GroupBalance, Members: []GroupMember{{Name: "a", Weight: -1}}}},
		"имя участника":            {{Name: "g", Mode: GroupBalance, Members: []GroupMember{{Name: "../x"}}}},
	}
	for name, g := range cases {
		if err := ValidateGroups(g, profiles); err == nil {
			t.Errorf("%s: ошибка должна быть", name)
		}
	}
	var many []Group
	for i := 0; i <= MaxGroups; i++ {
		many = append(many, Group{Name: "g" + string(rune('a'+i)), Mode: GroupOrdered})
	}
	if ValidateGroups(many, profiles) == nil {
		t.Error("слишком много групп")
	}
	var members []GroupMember
	for i := 0; i <= MaxGroupMembers; i++ {
		members = append(members, GroupMember{Name: "m" + string(rune('a'+i))})
	}
	if ValidateGroups([]Group{{Name: "g", Mode: GroupOrdered, Members: members}}, nil) == nil {
		t.Error("слишком много участников")
	}
}

func TestGroupsAreIsolatedFromProfilesAndCleanedOnDelete(t *testing.T) {
	st, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"a", "b"} {
		if err := st.WriteProfile(n, goodConf); err != nil {
			t.Fatal(err)
		}
	}
	_ = st.Update(func(s *State) error {
		s.Groups = []Group{{Name: "pool", Mode: GroupBalance, Members: []GroupMember{{Name: "a"}, {Name: "b"}}}}
		s.Primary = "pool"
		return nil
	})
	if err := st.WriteProfile("pool", goodConf); err == nil {
		t.Fatal("профиль не может занять имя группы")
	}
	snap := st.Snapshot()
	snap.Groups[0].Members[0].Name = "hacked" // снимок — копия
	if st.Snapshot().Groups[0].Members[0].Name != "a" {
		t.Fatal("Snapshot должен отдавать копию групп")
	}
	if err := st.DeleteProfile("a"); err != nil {
		t.Fatal(err)
	}
	g, ok := st.Snapshot().Group("pool")
	if !ok || len(g.Members) != 1 || g.Members[0].Name != "b" {
		t.Fatalf("удалённый профиль уходит из групп: %+v", g)
	}
	if st.Snapshot().Primary != "pool" {
		t.Fatal("основная группа не сбрасывается при удалении профиля")
	}
	if !st.Snapshot().HasGroup("pool") || st.Snapshot().HasGroup("a") {
		t.Fatal("HasGroup")
	}
}
