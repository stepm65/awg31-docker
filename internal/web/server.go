// Package web — HTTP API и встроенный интерфейс управления.
package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"awg31/internal/auth"
	"awg31/internal/failover"
	"awg31/internal/geo"
	"awg31/internal/group"
	"awg31/internal/policy"
	"awg31/internal/ros"
	"awg31/internal/store"
	"awg31/internal/subsync"
	"awg31/internal/tunnel"
)

//go:embed static
var staticFS embed.FS

// Config — зависимости и настройки сервера.
type Config struct {
	Store    *store.Store
	Tunnels  *tunnel.Manager
	Subs     *subsync.Service
	Failover *failover.Monitor
	Groups   *group.Manager
	Policy   *policy.Engine
	Router   *ros.Manager
	Geo      *geo.Store
	Sessions *auth.Sessions
	Limiter  *auth.Limiter

	// SetupToken — одноразовый токен первичной настройки. Пустой, если пароль уже задан.
	SetupToken string
	// TLS — интерфейс отдаётся по HTTPS (включает Secure-cookie и HSTS).
	TLS bool
	// TrustedProxies — прокси, чьему заголовку X-Forwarded-For можно верить.
	TrustedProxies []netip.Prefix
	// AllowHTTPSubs разрешает http:// в URL подписок (по умолчанию запрещено).
	AllowHTTPSubs bool
	// DNSEnabled — DNS-форвардер запущен (для UI).
	DNSEnabled bool
	Logf       func(format string, args ...any)
}

// Server — HTTP-сервер приложения.
type Server struct {
	cfg Config

	mu         sync.Mutex // защищает setupToken и сериализует первичную настройку
	setupToken string

	dummyOnce sync.Once
	dummyHash string
}

// New создаёт сервер.
func New(cfg Config) *Server {
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	return &Server{cfg: cfg, setupToken: cfg.SetupToken}
}

func (s *Server) cookieName() string {
	if s.cfg.TLS {
		return "__Host-awg_session" // префикс требует Secure, Path=/ и запрещает Domain
	}
	return "awg_session"
}

// Handler собирает маршруты и middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(staticFS, "static")
	files := http.FileServerFS(sub)

	// index.html отдаём напрямую: http.FileServer редиректит любой путь на
	// /index.html в "./", и переписанный «/» зацикливался бы.
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("web: во встроенной статике нет index.html: " + err.Error())
	}
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
	})
	mux.Handle("GET /app.js", files)
	mux.Handle("GET /app.css", files)
	mux.Handle("GET /icon.svg", files)
	// Шаблон с одним сегментом: каталог /fonts/ не листается, отдаются только файлы по имени.
	mux.Handle("GET /fonts/{file}", cacheFor(files, "public, max-age=86400"))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		// Без деталей: ни версии, ни имён интерфейсов.
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	mux.HandleFunc("GET /api/auth/status", s.handleAuthStatus)
	mux.HandleFunc("POST /api/auth/setup", s.handleSetup)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.handleLogout)
	mux.HandleFunc("POST /api/auth/password", s.requireAuth(s.handlePassword))

	mux.HandleFunc("GET /api/status", s.requireAuth(s.handleStatus))
	mux.HandleFunc("PUT /api/profiles/{name}", s.requireAuth(s.handlePutProfile))
	mux.HandleFunc("DELETE /api/profiles/{name}", s.requireAuth(s.handleDeleteProfile))
	mux.HandleFunc("GET /api/groups", s.requireAuth(s.handleGetGroups))
	mux.HandleFunc("PUT /api/groups", s.requireAuth(s.handlePutGroups))
	mux.HandleFunc("POST /api/groups/{name}/primary", s.requireAuth(s.handleGroupPrimary))
	mux.HandleFunc("POST /api/profiles/{name}/up", s.requireAuth(s.handleUp))
	mux.HandleFunc("POST /api/profiles/{name}/down", s.requireAuth(s.handleDown))
	mux.HandleFunc("PUT /api/subscriptions/{name}", s.requireAuth(s.handlePutSub))
	mux.HandleFunc("DELETE /api/subscriptions/{name}", s.requireAuth(s.handleDeleteSub))
	mux.HandleFunc("POST /api/subscriptions/{name}/sync", s.requireAuth(s.handleSyncSub))
	mux.HandleFunc("POST /api/profiles/{name}/primary", s.requireAuth(s.handlePrimary))
	mux.HandleFunc("GET /api/failover", s.requireAuth(s.handleGetFailover))
	mux.HandleFunc("GET /api/policy", s.requireAuth(s.handleGetPolicy))
	mux.HandleFunc("PUT /api/policy", s.requireAuth(s.handlePutPolicy))
	mux.HandleFunc("POST /api/policy/test", s.requireAuth(s.handleTestPolicy))
	mux.HandleFunc("POST /api/geo/update", s.requireAuth(s.handleGeoUpdate))
	mux.HandleFunc("GET /api/router", s.requireAuth(s.handleGetRouter))
	mux.HandleFunc("PUT /api/router", s.requireAuth(s.handlePutRouter))
	mux.HandleFunc("POST /api/router/test", s.requireAuth(s.handleTestRouter))
	mux.HandleFunc("POST /api/router/sync", s.requireAuth(s.handleSyncRouter))
	mux.HandleFunc("PUT /api/failover", s.requireAuth(s.handlePutFailover))
	mux.HandleFunc("POST /api/failover/check", s.requireAuth(s.handleCheckFailover))

	return s.securityHeaders(s.csrf(mux))
}

// ---------- middleware ----------

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		} else {
			h.Set("Cache-Control", "no-cache")
		}
		if s.cfg.TLS {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}

// csrf отвергает изменяющие запросы с чужого origin и формы вместо JSON.
func (s *Server) csrf(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			writeErr(w, http.StatusForbidden, "запрос с другого origin заблокирован (защита от CSRF)")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(u.Host, r.Host) {
				writeErr(w, http.StatusForbidden, "запрос с другого origin заблокирован (защита от CSRF)")
				return
			}
		}
		if r.ContentLength != 0 {
			ct := strings.ToLower(strings.TrimSpace(strings.SplitN(r.Header.Get("Content-Type"), ";", 2)[0]))
			if ct != "application/json" {
				writeErr(w, http.StatusUnsupportedMediaType, "требуется Content-Type: application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next func(http.ResponseWriter, *http.Request, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.sessionUser(r)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "требуется вход")
			return
		}
		next(w, r, user)
	}
}

func (s *Server) sessionUser(r *http.Request) (string, bool) {
	c, err := r.Cookie(s.cookieName())
	if err != nil {
		return "", false
	}
	return s.cfg.Sessions.Validate(c.Value)
}

func (s *Server) setSession(w http.ResponseWriter, user string) error {
	token, err := s.cfg.Sessions.Issue(user)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName(), Value: token, Path: "/", MaxAge: 24 * 3600,
		HttpOnly: true, Secure: s.cfg.TLS, SameSite: http.SameSiteStrictMode,
	})
	return nil
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName(), Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: s.cfg.TLS, SameSite: http.SameSiteStrictMode,
	})
}

// clientIP возвращает адрес клиента. X-Forwarded-For учитывается только от
// доверенных прокси и читается справа налево.
func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil || len(s.cfg.TrustedProxies) == 0 || !s.trusted(addr) {
		return host
	}
	parts := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		a, err := netip.ParseAddr(strings.TrimSpace(parts[i]))
		if err != nil {
			continue
		}
		if !s.trusted(a) {
			return a.String()
		}
	}
	return host
}

func (s *Server) trusted(a netip.Addr) bool {
	for _, p := range s.cfg.TrustedProxies {
		if p.Contains(a.Unmap()) {
			return true
		}
	}
	return false
}

// ---------- JSON ----------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// decode читает ровно одно JSON-значение ограниченного размера без неизвестных полей.
func decode(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный JSON: "+err.Error())
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		writeErr(w, http.StatusBadRequest, "после JSON найдены лишние данные")
		return false
	}
	return true
}

func timeout(r *http.Request, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), d)
}

// cacheFor добавляет Cache-Control к ответам next.
func cacheFor(next http.Handler, value string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", value)
		next.ServeHTTP(w, r)
	})
}
