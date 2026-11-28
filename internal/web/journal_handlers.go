package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"awg31/internal/journal"
)

// handleLog — записи журнала после ?after=N (не больше ?limit, по умолчанию 1000).
// Страница «Журнал» опрашивает его, передавая номер последней полученной записи.
func (s *Server) handleLog(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Journal == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []journal.Entry{}, "last": 0})
		return
	}
	after, _ := strconv.ParseUint(r.URL.Query().Get("after"), 10, 64)
	limit := 1000
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 5000 {
		limit = v
	}
	entries, last := s.cfg.Journal.Since(after, limit)
	if entries == nil {
		entries = []journal.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "last": last})
}

// handleLogDownload — весь журнал текстом (UTF-8), для пересылки при разборе проблем.
func (s *Server) handleLogDownload(w http.ResponseWriter, _ *http.Request, _ string) {
	var b strings.Builder
	if s.cfg.Journal != nil {
		entries, _ := s.cfg.Journal.Since(0, 0)
		for _, e := range entries {
			fmt.Fprintf(&b, "%s %-5s %s\n", e.Time.Format(time.RFC3339), e.Level, e.Text)
		}
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="awg31-log-`+time.Now().UTC().Format("20060102-150405")+`.txt"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}
