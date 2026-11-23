package web

import (
	"net/http"
	"time"

	"awg31/internal/tunnel"
)

// HealthHandler — проба для /tool/netwatch RouterOS и HEALTHCHECK. Отвечает
// только кодом (200/503) и пустым телом, поэтому без авторизации ничего не раскрывает.
// Если maxHandshake > 0, то свежее рукопожатие обязательно для каждого пира с endpoint.
func HealthHandler(tm *tunnel.Manager, maxHandshake time.Duration) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := timeout(r, 5*time.Second)
		defer cancel()
		ifaces, err := tm.Status(ctx)
		if err != nil || len(ifaces) == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if maxHandshake > 0 {
			now := time.Now().Unix()
			for _, i := range ifaces {
				for _, p := range i.Peers {
					if p.Endpoint == "" {
						continue
					}
					if p.LatestHandshake == 0 || now-p.LatestHandshake > int64(maxHandshake/time.Second) {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}
