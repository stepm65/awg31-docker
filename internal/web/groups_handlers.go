package web

import (
	"context"
	"errors"
	"net/http"
	"time"

	"awg31/internal/conf"
	"awg31/internal/group"
	"awg31/internal/store"
	"awg31/internal/tunnel"
)

// groupsView — группы туннелей для UI: настройки (то, что редактируется),
// состояние маршрутов и здоровье туннелей.
type groupsView struct {
	Groups   []store.Group  `json:"groups"`
	Status   []group.Status `json:"status"`
	Profiles []string       `json:"profiles"` // профили, которые можно добавить (не серверные)
	Health   map[string]struct {
		State  string `json:"state"`
		Reason string `json:"reason,omitempty"`
	} `json:"health"`
	Primary    string `json:"primary"`
	Gateway    bool   `json:"gateway"`
	HashPolicy string `json:"hash_policy"` // fib_multipath_hash_policy IPv4 ("1" — по портам)
}

func (s *Server) groupsView() groupsView {
	st := s.cfg.Store.Snapshot()
	v := groupsView{Groups: append([]store.Group{}, st.Groups...), Primary: s.cfg.Tunnels.Primary(), HashPolicy: group.HashPolicy()}
	o := s.cfg.Tunnels.Options()
	v.Gateway = o.AutoRoute && o.LanIface != ""
	if s.cfg.Groups != nil {
		v.Status = s.cfg.Groups.Status()
	}
	v.Status = append([]group.Status{}, v.Status...)
	profiles, _ := s.cfg.Store.ListProfiles()
	v.Profiles = []string{}
	for _, n := range profiles {
		if !s.isServerProfile(n) {
			v.Profiles = append(v.Profiles, n)
		}
	}
	v.Health = map[string]struct {
		State  string `json:"state"`
		Reason string `json:"reason,omitempty"`
	}{}
	if s.cfg.Failover != nil {
		for _, h := range s.cfg.Failover.Status().Tunnels {
			v.Health[h.Name] = struct {
				State  string `json:"state"`
				Reason string `json:"reason,omitempty"`
			}{h.State, h.Reason}
		}
	}
	return v
}

func (s *Server) isServerProfile(name string) bool {
	raw, err := s.cfg.Store.ReadProfile(name)
	if err != nil {
		return false
	}
	cfg, err := conf.Parse(raw)
	return err == nil && cfg.IsServer()
}

func (s *Server) handleGetGroups(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.groupsView())
}

func (s *Server) handlePutGroups(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Groups []store.Group `json:"groups"`
	}
	if !decode(w, r, &body, 64<<10) {
		return
	}
	profiles, _ := s.cfg.Store.ListProfiles()
	if err := store.ValidateGroups(body.Groups, profiles); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	for _, g := range body.Groups {
		for _, m := range g.Members {
			if s.isServerProfile(m.Name) {
				writeErr(w, http.StatusBadRequest, "группа «"+g.Name+"»: профиль «"+m.Name+"» — сервер, в группу его добавить нельзя")
				return
			}
		}
	}
	if o := s.cfg.Tunnels.Options(); len(body.Groups) > 0 && (!o.AutoRoute || o.LanIface == "") {
		writeErr(w, http.StatusConflict, "группы работают только в режиме шлюза (AWG_AUTO_ROUTE=on и найден LAN-интерфейс)")
		return
	}
	groups := make([]store.Group, len(body.Groups))
	for i, g := range body.Groups {
		groups[i] = store.Group{Name: g.Name, Mode: g.Mode, Members: append([]store.GroupMember{}, g.Members...)}
	}
	prev := s.cfg.Store.Snapshot().Groups
	if err := s.cfg.Store.Update(func(st *store.State) error {
		st.Groups = groups
		if st.Primary != "" && !containsGroup(groups, st.Primary) && isGroupName(st.Primary, prev) {
			st.Primary = "" // основной была группа, которую удалили
		}
		return nil
	}); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// участники группы нужны поднятыми: недостающие поднимаются сразу
	ctx, cancel := timeout(r, 150*time.Second)
	defer cancel()
	var warnings []string
	for _, g := range groups {
		for _, m := range g.Members {
			if err := s.ensureUp(ctx, m.Name); err != nil {
				warnings = append(warnings, m.Name+": "+err.Error())
			}
		}
	}
	if s.cfg.Groups != nil {
		s.cfg.Groups.Update(ctx) // сам переводит основной, если основной была удалённая группа
	}
	if s.cfg.Failover != nil {
		s.cfg.Failover.Wake() // здоровье новых участников — немедленно
	}
	if s.cfg.Policy != nil {
		s.cfg.Policy.MarkDirty()
	}
	s.cfg.Logf("группы туннелей: %d", len(groups))
	v := s.groupsView()
	writeJSON(w, http.StatusOK, map[string]any{"view": v, "warnings": warnings})
}

func isGroupName(name string, gs []store.Group) bool { return containsGroup(gs, name) }

func containsGroup(gs []store.Group, name string) bool {
	for _, g := range gs {
		if g.Name == name {
			return true
		}
	}
	return false
}

// ensureUp поднимает туннель, если он ещё не поднят.
func (s *Server) ensureUp(ctx context.Context, name string) error {
	text, err := s.cfg.Store.ReadProfile(name)
	if err != nil {
		return err
	}
	if err := s.cfg.Tunnels.Up(ctx, name, text); err != nil {
		if errors.Is(err, tunnel.ErrAlreadyUp) {
			return nil
		}
		return err
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { st.Up[name] = true; return nil })
	return nil
}

func (s *Server) handleGroupPrimary(w http.ResponseWriter, r *http.Request, _ string) {
	name := r.PathValue("name")
	if !s.cfg.Store.Snapshot().HasGroup(name) {
		writeErr(w, http.StatusNotFound, "такой группы нет")
		return
	}
	ctx, cancel := timeout(r, 15*time.Second)
	defer cancel()
	if err := s.cfg.Tunnels.SetPrimary(ctx, name); err != nil {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	_ = s.cfg.Store.Update(func(st *store.State) error { st.Primary = name; return nil })
	s.cfg.Logf("основной: группа %s (вручную)", name)
	if s.cfg.Failover != nil {
		s.cfg.Failover.Wake()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
