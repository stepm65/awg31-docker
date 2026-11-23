package web

import (
	"net/http"
	"strings"
	"time"

	"awg31/internal/ros"
	"awg31/internal/store"
)

// routerSettingsView — настройки без пароля.
type routerSettingsView struct {
	store.Router
	Pass    string `json:"pass"`
	PassSet bool   `json:"pass_set"`
}

func (s *Server) routerView() map[string]any {
	r := s.cfg.Store.Snapshot().RouterSettings()
	v := map[string]any{"settings": routerSettingsView{Router: r, PassSet: r.Pass != ""}}
	if s.cfg.Router != nil {
		v["status"] = s.cfg.Router.Status()
	}
	o := s.cfg.Tunnels.Options()
	v["gateway"] = o.AutoRoute && o.LanIface != ""
	return v
}

// mergeRouter принимает настройки из запроса: пустой пароль — оставить прежний.
func (s *Server) mergeRouter(w http.ResponseWriter, r *http.Request) (store.Router, bool) {
	var in store.Router
	if !decode(w, r, &in, 16<<10) {
		return in, false
	}
	in.Mode, in.URL = strings.TrimSpace(in.Mode), strings.TrimSpace(in.URL)
	if in.Pass == "" {
		in.Pass = s.cfg.Store.Snapshot().RouterSettings().Pass
	}
	if errs := ros.Validate(in); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": errs[0], "errors": errs})
		return in, false
	}
	return in, true
}

func (s *Server) handleGetRouter(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.routerView())
}

func (s *Server) handlePutRouter(w http.ResponseWriter, r *http.Request, _ string) {
	in, ok := s.mergeRouter(w, r)
	if !ok {
		return
	}
	if in.Mode != store.RouterOff {
		if o := s.cfg.Tunnels.Options(); !o.AutoRoute || o.LanIface == "" {
			writeErr(w, http.StatusConflict, "интеграция с RouterOS работает только в режиме шлюза")
			return
		}
	}
	if err := s.cfg.Store.Update(func(st *store.State) error { st.Router = &in; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.cfg.Router != nil {
		s.cfg.Router.Wake()
	}
	s.cfg.Logf("RouterOS: mode %s, address %s", in.Mode, in.URL)
	writeJSON(w, http.StatusOK, s.routerView())
}

func (s *Server) handleTestRouter(w http.ResponseWriter, r *http.Request, _ string) {
	in, ok := s.mergeRouter(w, r)
	if !ok || s.cfg.Router == nil {
		return
	}
	ctx, cancel := timeout(r, 30*time.Second)
	defer cancel()
	who, err := s.cfg.Router.Test(ctx, in)
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"router": who})
}

func (s *Server) handleSyncRouter(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Router == nil {
		writeErr(w, http.StatusNotImplemented, "интеграция не запущена")
		return
	}
	ctx, cancel := timeout(r, 3*time.Minute)
	defer cancel()
	if err := s.cfg.Router.Sync(ctx); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.routerView())
}
