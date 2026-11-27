package web

import (
	"net/http"
	"strconv"
	"time"

	"awg31/internal/monitor"
)

// handleMonitor — история трафика за окно (?window=секунды, по умолчанию 15 минут)
// и состояние системы. Лёгкий запрос: его опрашивают «Обзор» и боковое меню.
func (s *Server) handleMonitor(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Monitor == nil {
		writeJSON(w, http.StatusOK, map[string]any{"traffic": []monitor.Point{}, "system": monitor.System{}, "step": int(monitor.Step.Seconds())})
		return
	}
	win := 15 * time.Minute
	if v, err := strconv.Atoi(r.URL.Query().Get("window")); err == nil && v >= 60 && v <= 3600 {
		win = time.Duration(v) * time.Second
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"traffic": s.cfg.Monitor.Traffic(win),
		"system":  s.cfg.Monitor.System(),
		"step":    int(monitor.Step.Seconds()),
	})
}

// handleMonitorFlows — соединения клиентов по устройствам, правилам и выходам.
func (s *Server) handleMonitorFlows(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Monitor == nil {
		writeJSON(w, http.StatusOK, monitor.Snapshot{Source: "none", Devices: []monitor.Node{}, Rules: []monitor.Node{}, Outs: []monitor.Node{}, Paths: []monitor.Path{}, Conns: []monitor.Conn{}, ByLine: map[string]int{}})
		return
	}
	ctx, cancel := timeout(r, 10*time.Second)
	defer cancel()
	writeJSON(w, http.StatusOK, s.cfg.Monitor.Flows(ctx))
}
