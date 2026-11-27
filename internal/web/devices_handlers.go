package web

import (
	"net/http"
	"time"

	"awg31/internal/policy"
	"awg31/internal/store"
)

func (s *Server) devicesView() map[string]any {
	devs := s.cfg.Store.Snapshot().Devices
	if devs == nil {
		devs = []store.Device{}
	}
	return map[string]any{"devices": devs, "kinds": store.DeviceKinds}
}

func (s *Server) handleGetDevices(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.devicesView())
}

// handlePutDevices сохраняет справочник целиком. Устройство, на которое ссылаются
// правила (src=@id), удалить нельзя: правило молча стало бы общим.
func (s *Server) handlePutDevices(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Devices []store.Device `json:"devices"`
	}
	if !decode(w, r, &body, 128<<10) {
		return
	}
	devs, err := store.NormalizeDevices(body.Devices)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	keep := map[string]bool{}
	for _, d := range devs {
		keep[d.ID] = true
	}
	for _, id := range deviceRefs(s.cfg.Store.Snapshot().PolicySettings().Rules) {
		if !keep[id] {
			writeErr(w, http.StatusConflict, "устройство @"+id+" используется в правилах маршрутизации: сначала уберите его из правил")
			return
		}
	}
	if err := s.cfg.Store.Update(func(st *store.State) error { st.Devices = devs; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if s.cfg.Policy != nil {
		_ = s.cfg.Policy.Reload() // адреса устройств входят в правила
	}
	s.cfg.Logf("devices: %d saved", len(devs))
	writeJSON(w, http.StatusOK, s.devicesView())
}

// handleDeviceLeases отдаёт аренды DHCP роутера, чтобы добавить устройства из них.
func (s *Server) handleDeviceLeases(w http.ResponseWriter, r *http.Request, _ string) {
	rs := s.cfg.Store.Snapshot().RouterSettings()
	if s.cfg.Router == nil || rs.Pass == "" {
		writeErr(w, http.StatusConflict, "нужна интеграция с RouterOS: на странице «RouterOS» задайте адрес, пользователя и пароль")
		return
	}
	ctx, cancel := timeout(r, 20*time.Second)
	defer cancel()
	leases, err := s.cfg.Router.Leases(ctx, rs)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "RouterOS: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"leases": leases})
}

// deviceRefs — устройства, на которые ссылаются сохранённые правила.
func deviceRefs(text string) []string {
	rules, _ := policy.Parse(text)
	return policy.DeviceRefs(rules)
}
