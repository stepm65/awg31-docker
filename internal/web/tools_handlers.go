package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"awg31/internal/auth"
	"awg31/internal/conf"
	"awg31/internal/store"
	"awg31/internal/tunnel"
	"awg31/internal/version"
)

// Формат резервной копии: настройки (без логина и пароля администратора) и профили.
const backupFormat = "awg31-backup"

type backupFile struct {
	Format   string            `json:"format"`
	Version  string            `json:"version"`
	Created  time.Time         `json:"created"`
	State    store.State       `json:"state"`
	Profiles map[string]string `json:"profiles"`
}

func attachment(w http.ResponseWriter, name, ctype string) {
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
}

// handleExportRules отдаёт текст правил маршрутизации файлом.
func (s *Server) handleExportRules(w http.ResponseWriter, _ *http.Request, _ string) {
	attachment(w, "awg31-rules-"+time.Now().Format("20060102")+".txt", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(s.cfg.Store.Snapshot().PolicySettings().Rules))
}

// handleBackup отдаёт резервную копию: в ней закрытые ключи профилей и пароль роутера.
func (s *Server) handleBackup(w http.ResponseWriter, _ *http.Request, _ string) {
	st := s.cfg.Store.Snapshot()
	st.User, st.PassHash = "", ""
	b := backupFile{Format: backupFormat, Version: version.Version, Created: time.Now().UTC(), State: st, Profiles: map[string]string{}}
	names, _ := s.cfg.Store.ListProfiles()
	for _, n := range names {
		if raw, err := s.cfg.Store.ReadProfile(n); err == nil {
			b.Profiles[n] = raw
		}
	}
	data, err := json.MarshalIndent(b, "", " ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg.Logf("backup downloaded (%d profiles)", len(b.Profiles))
	attachment(w, "awg31-backup-"+time.Now().Format("20060102-1504")+".json", "application/json")
	_, _ = w.Write(data)
}

// handleRestore восстанавливает копию: туннели опускаются, профили и настройки
// заменяются (логин и пароль администратора остаются текущими), затем поднимаются
// туннели, отмеченные в копии. Нужен текущий пароль.
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request, _ string) {
	ip := s.clientIP(r)
	if s.limited(w, ip) {
		return
	}
	var body struct {
		Password string          `json:"password"`
		Backup   json.RawMessage `json:"backup"`
	}
	if !decode(w, r, &body, 8<<20) {
		return
	}
	if !auth.VerifyPassword(s.cfg.Store.Snapshot().PassHash, body.Password) {
		s.cfg.Limiter.Fail(ip)
		writeErr(w, http.StatusForbidden, "неверный пароль администратора")
		return
	}
	var b backupFile
	if err := json.Unmarshal(body.Backup, &b); err != nil || b.Format != backupFormat {
		writeErr(w, http.StatusBadRequest, "это не резервная копия awg31")
		return
	}
	if err := checkBackup(b); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := timeout(r, 4*time.Minute)
	defer cancel()
	if err := s.restore(ctx, b); err != nil {
		writeErr(w, http.StatusInternalServerError, "восстановление: "+err.Error())
		return
	}
	s.cfg.Logf("backup restored: %d profiles from %s (v%s)", len(b.Profiles), b.Created.Format(time.RFC3339), b.Version)
	writeJSON(w, http.StatusOK, map[string]any{"profiles": len(b.Profiles), "version": b.Version})
}

func checkBackup(b backupFile) error {
	if len(b.Profiles) > 64 {
		return errors.New("в копии больше 64 профилей")
	}
	names := make([]string, 0, len(b.Profiles))
	for n, raw := range b.Profiles {
		if !store.ValidName(n) {
			return fmt.Errorf("недопустимое имя профиля «%s»", n)
		}
		if _, err := conf.Parse(raw); err != nil {
			return fmt.Errorf("профиль %s: %w", n, err)
		}
		names = append(names, n)
	}
	if err := store.ValidateGroups(b.State.Groups, names); err != nil {
		return err
	}
	if _, err := store.NormalizeDevices(b.State.Devices); err != nil {
		return err
	}
	return nil
}

func (s *Server) restore(ctx context.Context, b backupFile) error {
	// 1. все туннели вниз
	if ifs, err := s.cfg.Tunnels.Status(ctx); err == nil {
		for _, i := range ifs {
			if err := s.cfg.Tunnels.Down(ctx, i.Name); err != nil && !errors.Is(err, tunnel.ErrNotUp) {
				s.cfg.Logf("restore: %s: %v", i.Name, err)
			}
		}
	}
	// 2. профили: лишние удаляются, из копии — записываются
	cur, _ := s.cfg.Store.ListProfiles()
	for _, n := range cur {
		if _, keep := b.Profiles[n]; !keep {
			if err := s.cfg.Store.DeleteProfile(n); err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
		}
	}
	// группы с именами профилей мешают записи: сначала настройки без групп
	if err := s.cfg.Store.Update(func(st *store.State) error { st.Groups = nil; return nil }); err != nil {
		return err
	}
	names := make([]string, 0, len(b.Profiles))
	for n := range b.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err := s.cfg.Store.WriteProfile(n, b.Profiles[n]); err != nil {
			return fmt.Errorf("профиль %s: %w", n, err)
		}
	}
	// 3. настройки целиком, кроме входа администратора
	if err := s.cfg.Store.Update(func(st *store.State) error {
		user, hash := st.User, st.PassHash
		*st = b.State
		st.User, st.PassHash = user, hash
		if st.Subs == nil {
			st.Subs = map[string]store.Sub{}
		}
		if st.Up == nil {
			st.Up = map[string]bool{}
		}
		return nil
	}); err != nil {
		return err
	}
	if s.cfg.Policy != nil {
		_ = s.cfg.Policy.Reload()
	}
	if s.cfg.Router != nil {
		s.cfg.Router.Wake()
	}
	// 4. туннели из копии — вверх (как при запуске контейнера)
	if s.cfg.Autostart != nil {
		s.cfg.Autostart(ctx)
	}
	return nil
}
