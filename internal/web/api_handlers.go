package web

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"awg31/internal/conf"
	"awg31/internal/store"
	"awg31/internal/subfetch"
	"awg31/internal/subsync"
	"awg31/internal/tunnel"
	"awg31/internal/version"
)

// SubView — настройки подписки без секретов. У панели amneziawg31-installer токен
// лежит прямо в пути ссылки, поэтому URL отдаётся только в замаскированном виде.
type SubView struct {
	URL         string    `json:"url"`
	TokenSet    bool      `json:"token_set"`
	PinSHA256   string    `json:"pin_sha256,omitempty"`
	InsecureTLS bool      `json:"insecure_tls"`
	IntervalMin int       `json:"interval_min"`
	LastSync    time.Time `json:"last_sync,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	Notice      string    `json:"notice,omitempty"`
}

// ProfileView — профиль для UI. Приватные ключи и PSK никогда не включаются.
type ProfileView struct {
	Name      string        `json:"name"`
	Summary   *conf.Summary `json:"summary,omitempty"`
	Error     string        `json:"error,omitempty"`
	Up        bool          `json:"up"`
	Primary   bool          `json:"primary"`
	Autostart bool          `json:"autostart"`
	Iface     *tunnel.Iface `json:"iface,omitempty"`
	Sub       *SubView      `json:"subscription,omitempty"`
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, user string) {
	ctx, cancel := timeout(r, 15*time.Second)
	defer cancel()

	o := s.cfg.Tunnels.Options()
	resp := map[string]any{"version": version.Version, "user": user, "network": map[string]any{
		"gateway":     o.AutoRoute,
		"lan_iface":   o.LanIface,
		"kill_switch": o.AutoRoute && o.LanIface != "" && o.KillSwitch,
		"ipv6":        o.IPv6,
		"table":       tunnel.RouteTable,
		"error":       s.cfg.Tunnels.RoutingError(),
	}}
	ifaces, err := s.cfg.Tunnels.Status(ctx)
	if err != nil {
		resp["tunnel_error"] = err.Error()
	}
	byName := map[string]tunnel.Iface{}
	for _, i := range ifaces {
		byName[i.Name] = i
	}

	names, err := s.cfg.Store.ListProfiles()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	st := s.cfg.Store.Snapshot()
	subNames := map[string]bool{}
	for n := range st.Subs {
		subNames[n] = true
	}
	for _, n := range names {
		delete(subNames, n)
	}
	for n := range subNames { // подписки, по которым профиль ещё не создан
		names = append(names, n)
	}
	sort.Strings(names)

	primary := s.cfg.Tunnels.Primary()
	resp["primary"] = primary
	profiles := make([]ProfileView, 0, len(names))
	for _, n := range names {
		pv := ProfileView{Name: n, Autostart: st.Up[n], Primary: n == primary}
		if text, err := s.cfg.Store.ReadProfile(n); err == nil {
			if cfg, err := conf.Parse(text); err == nil {
				sum := cfg.Summary()
				pv.Summary = &sum
			} else {
				pv.Error = err.Error()
			}
		}
		if i, ok := byName[n]; ok {
			pv.Up = true
			iface := i
			pv.Iface = &iface
		}
		if sub, ok := st.Subs[n]; ok {
			pv.Sub = &SubView{
				URL: subfetch.MaskURL(sub.URL), TokenSet: sub.Token != "", PinSHA256: sub.PinSHA256,
				InsecureTLS: sub.InsecureTLS, IntervalMin: sub.IntervalMin,
				LastSync: sub.LastSync, LastError: sub.LastError, Notice: sub.LastNotice,
			}
		}
		profiles = append(profiles, pv)
	}
	resp["profiles"] = profiles
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handlePutProfile(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	var body struct {
		Content string `json:"content"`
	}
	if !decode(w, r, &body, conf.MaxSize+4096) {
		return
	}
	if err := s.cfg.Store.WriteProfile(name, body.Content); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := timeout(r, 10*time.Second)
	defer cancel()
	running := s.isUp(ctx, name)
	s.cfg.Logf("profile %s saved", name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "restart_required": running})
}

func (s *Server) isUp(ctx context.Context, name string) bool {
	ifaces, err := s.cfg.Tunnels.Status(ctx)
	if err != nil {
		return false
	}
	for _, i := range ifaces {
		if i.Name == name {
			return true
		}
	}
	return false
}

func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !store.ValidName(name) {
		writeErr(w, http.StatusBadRequest, "недопустимое имя профиля")
		return
	}
	ctx, cancel := timeout(r, 30*time.Second)
	defer cancel()
	if err := s.cfg.Tunnels.Down(ctx, name); err != nil && !errors.Is(err, tunnel.ErrNotUp) {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.cfg.Store.DeleteProfile(name); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			code = http.StatusNotFound
		}
		writeErr(w, code, err.Error())
		return
	}
	s.cfg.Logf("profile %s deleted", name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleUp(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	text, err := s.cfg.Store.ReadProfile(name)
	if err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			code = http.StatusNotFound
		}
		writeErr(w, code, err.Error())
		return
	}
	ctx, cancel := timeout(r, 60*time.Second)
	defer cancel()
	if err := s.cfg.Tunnels.Up(ctx, name, text); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, tunnel.ErrAlreadyUp) {
			code = http.StatusConflict
		}
		writeErr(w, code, err.Error())
		return
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { st.Up[name] = true; return nil })
	s.cfg.Logf("tunnel %s up", name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDown(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !store.ValidName(name) {
		writeErr(w, http.StatusBadRequest, "недопустимое имя профиля")
		return
	}
	ctx, cancel := timeout(r, 60*time.Second)
	defer cancel()
	err := s.cfg.Tunnels.Down(ctx, name)
	if err != nil && !errors.Is(err, tunnel.ErrNotUp) {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { delete(st.Up, name); return nil })
	s.cfg.Logf("tunnel %s down", name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePutSub(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !store.ValidName(name) {
		writeErr(w, http.StatusBadRequest, "недопустимое имя профиля")
		return
	}
	var body struct {
		URL         string `json:"url"`
		Token       string `json:"token"`       // пусто — оставить прежний
		ClearToken  bool   `json:"clear_token"` // удалить сохранённый токен
		PinSHA256   string `json:"pin_sha256"`
		InsecureTLS bool   `json:"insecure_tls"`
		IntervalMin int    `json:"interval_min"`
	}
	if !decode(w, r, &body, 8192) {
		return
	}
	if strings.TrimSpace(body.URL) == "" { // пусто — оставить прежнюю ссылку (её секретная часть не показывается)
		body.URL = s.cfg.Store.Snapshot().Subs[name].URL
		if body.URL == "" {
			writeErr(w, http.StatusBadRequest, "укажите ссылку подписки")
			return
		}
	}
	if _, err := subfetch.ValidateURL(body.URL, s.cfg.AllowHTTPSubs); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pin, err := subfetch.NormalizePin(body.PinSHA256)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.IntervalMin != 0 && (body.IntervalMin < subsync.MinInterval || body.IntervalMin > subsync.MaxInterval) {
		writeErr(w, http.StatusBadRequest, "интервал: 0 (выключено) или от 5 до 10080 минут")
		return
	}
	if len(body.Token) > 512 {
		writeErr(w, http.StatusBadRequest, "токен слишком длинный")
		return
	}
	err = s.cfg.Store.Update(func(st *store.State) error {
		sub := st.Subs[name]
		if sub.URL != body.URL || sub.PinSHA256 != pin {
			sub.ETag = "" // источник изменился — условные запросы больше недействительны
		}
		sub.URL, sub.PinSHA256, sub.InsecureTLS, sub.IntervalMin = body.URL, pin, body.InsecureTLS, body.IntervalMin
		switch {
		case body.ClearToken:
			sub.Token = ""
		case body.Token != "":
			sub.Token = body.Token
		}
		st.Subs[name] = sub
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if body.InsecureTLS && pin == "" {
		s.cfg.Logf("WARNING: TLS verification is disabled for subscription %s", name)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteSub(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !store.ValidName(name) {
		writeErr(w, http.StatusBadRequest, "недопустимое имя профиля")
		return
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { delete(st.Subs, name); return nil })
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSyncSub(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	ctx, cancel := timeout(r, 60*time.Second)
	defer cancel()
	changed, err := s.cfg.Subs.Sync(ctx, name)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error(), "changed": changed})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "changed": changed})
}
