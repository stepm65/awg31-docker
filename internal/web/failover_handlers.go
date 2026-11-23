package web

import (
	"errors"
	"net/http"
	"time"

	"awg31/internal/failover"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// failoverView — настройки, порядок и состояние резервирования для UI.
type failoverView struct {
	failover.Status
	Priority []string `json:"priority"` // явный список ([] — все профили по имени)
	Order    []string `json:"order"`    // действующий порядок
}

func (s *Server) failoverView() failoverView {
	st := s.cfg.Store.Snapshot()
	profiles, _ := s.cfg.Store.ListProfiles()
	var status failover.Status
	if s.cfg.Failover != nil {
		status = s.cfg.Failover.Status()
	}
	status.Settings = st.FailoverSettings()
	status.Primary = s.cfg.Tunnels.Primary()
	o := s.cfg.Tunnels.Options()
	status.Gateway = o.AutoRoute && o.LanIface != ""
	return failoverView{Status: status, Priority: append([]string{}, st.Priority...), Order: failover.Order(st.Priority, profiles)}
}

func (s *Server) handleGetFailover(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.failoverView())
}

func (s *Server) handlePutFailover(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Settings store.Failover `json:"settings"`
		Priority []string       `json:"priority"`
	}
	if !decode(w, r, &body, 16<<10) {
		return
	}
	if err := body.Settings.Validate(); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.Priority) > 64 {
		writeErr(w, http.StatusBadRequest, "слишком длинный список приоритета")
		return
	}
	seen := map[string]bool{}
	for _, n := range body.Priority {
		if !store.ValidName(n) || seen[n] {
			writeErr(w, http.StatusBadRequest, "список приоритета: недопустимое или повторяющееся имя профиля")
			return
		}
		seen[n] = true
	}
	if body.Settings.Mode != store.FailoverOff {
		if o := s.cfg.Tunnels.Options(); !o.AutoRoute || o.LanIface == "" {
			writeErr(w, http.StatusConflict, "резервирование работает только в режиме шлюза (AWG_AUTO_ROUTE=on и найден LAN-интерфейс)")
			return
		}
	}
	settings, prio := body.Settings, append([]string{}, body.Priority...)
	if err := s.cfg.Store.Update(func(st *store.State) error {
		st.Failover, st.Priority = &settings, prio
		return nil
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.cfg.Failover != nil {
		s.cfg.Failover.Wake()
	}
	s.cfg.Logf("failover: mode %s, trigger %s, priority %v", settings.Mode, settings.Trigger, prio)
	writeJSON(w, http.StatusOK, s.failoverView())
}

func (s *Server) handleCheckFailover(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Failover == nil {
		writeErr(w, http.StatusNotImplemented, "монитор резервирования не запущен")
		return
	}
	ctx, cancel := timeout(r, 90*time.Second)
	defer cancel()
	s.cfg.Failover.Check(ctx)
	writeJSON(w, http.StatusOK, s.failoverView())
}

func (s *Server) handlePrimary(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !store.ValidName(name) {
		writeErr(w, http.StatusBadRequest, "недопустимое имя профиля")
		return
	}
	ctx, cancel := timeout(r, 15*time.Second)
	defer cancel()
	if err := s.cfg.Tunnels.SetPrimary(ctx, name); err != nil {
		code := http.StatusConflict
		if !errors.Is(err, tunnel.ErrNotUp) && s.cfg.Tunnels.Options().LanIface != "" {
			code = http.StatusInternalServerError
		}
		if errors.Is(err, tunnel.ErrNotUp) {
			writeErr(w, code, "сначала поднимите туннель "+name)
			return
		}
		writeErr(w, code, err.Error())
		return
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { st.Primary = name; return nil })
	s.cfg.Logf("primary tunnel: %s (manual)", name)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
