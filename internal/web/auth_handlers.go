package web

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"time"

	"awg31/internal/auth"
	"awg31/internal/store"
)

var (
	reUser            = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	errAlreadyCfgured = errors.New("пароль уже задан")
)

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	st := s.cfg.Store.Snapshot()
	user, authed := s.sessionUser(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"configured":    st.Configured(),
		"authenticated": authed,
		"user":          user,
	})
}

func (s *Server) limited(w http.ResponseWriter, ip string) bool {
	if blocked, wait := s.cfg.Limiter.Check(ip); blocked {
		sec := int(wait/time.Second) + 1
		w.Header().Set("Retry-After", fmt.Sprint(sec))
		writeErr(w, http.StatusTooManyRequests, fmt.Sprintf("слишком много попыток, повторите через %d с", sec))
		return true
	}
	return false
}

// handleSetup задаёт пароль администратора при первом запуске. Требует
// одноразовый токен из журнала контейнера, поэтому первый, кто «успел» открыть
// порт, не получает управление.
func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if s.limited(w, ip) {
		return
	}
	var body struct {
		SetupToken string `json:"setup_token"`
		User       string `json:"user"`
		Password   string `json:"password"`
	}
	if !decode(w, r, &body, 4096) {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.Store.Snapshot().Configured() || s.setupToken == "" {
		writeErr(w, http.StatusConflict, "первичная настройка уже выполнена")
		return
	}
	if !auth.TokenEqual(body.SetupToken, s.setupToken) {
		s.cfg.Limiter.Fail(ip)
		s.cfg.Logf("first-time setup: wrong token, ip=%s", ip)
		writeErr(w, http.StatusForbidden, "неверный токен первичной настройки (он выведен в журнале контейнера)")
		return
	}
	if !reUser.MatchString(body.User) {
		writeErr(w, http.StatusBadRequest, "имя пользователя: 1–64 символа [A-Za-z0-9._-]")
		return
	}
	if err := auth.ValidatePassword(body.Password); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "не удалось обработать пароль")
		return
	}
	err = s.cfg.Store.Update(func(st *store.State) error {
		if st.Configured() {
			return errAlreadyCfgured
		}
		st.User, st.PassHash = body.User, hash
		return nil
	})
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, errAlreadyCfgured) {
			code = http.StatusConflict
		}
		writeErr(w, code, err.Error())
		return
	}
	s.setupToken = "" // токен одноразовый
	if err := s.setSession(w, body.User); err != nil {
		writeErr(w, http.StatusInternalServerError, "не удалось создать сессию")
		return
	}
	s.cfg.Logf("first-time setup done, administrator %q", body.User)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": body.User})
}

// SetupToken возвращает действующий токен первичной настройки (для журнала).
func (s *Server) SetupToken() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setupToken
}

func (s *Server) dummy() string {
	s.dummyOnce.Do(func() { s.dummyHash, _ = auth.HashPassword("dummy-password-for-timing") })
	return s.dummyHash
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if s.limited(w, ip) {
		return
	}
	var body struct {
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body, 4096) {
		return
	}
	st := s.cfg.Store.Snapshot()
	if !st.Configured() {
		writeErr(w, http.StatusConflict, "сначала выполните первичную настройку")
		return
	}
	// Пароль проверяется всегда, чтобы время ответа не выдавало верность логина.
	userOK := subtle.ConstantTimeCompare([]byte(body.User), []byte(st.User)) == 1
	passOK := auth.VerifyPassword(st.PassHash, body.Password)
	if !userOK || !passOK {
		blocked, wait := s.cfg.Limiter.Fail(ip)
		s.cfg.Logf("failed login, ip=%s", ip)
		if blocked {
			writeErr(w, http.StatusTooManyRequests, fmt.Sprintf("неверный логин или пароль; адрес заблокирован на %d с", int(wait/time.Second)))
			return
		}
		writeErr(w, http.StatusUnauthorized, "неверный логин или пароль")
		return
	}
	s.cfg.Limiter.Reset(ip)
	if err := s.setSession(w, st.User); err != nil {
		writeErr(w, http.StatusInternalServerError, "не удалось создать сессию")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": st.User})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cookieName()); err == nil {
		s.cfg.Sessions.Revoke(c.Value)
	}
	s.clearSession(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handlePassword(w http.ResponseWriter, r *http.Request, user string) {
	ip := s.clientIP(r)
	if s.limited(w, ip) {
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decode(w, r, &body, 4096) {
		return
	}
	st := s.cfg.Store.Snapshot()
	if !auth.VerifyPassword(st.PassHash, body.CurrentPassword) {
		s.cfg.Limiter.Fail(ip)
		writeErr(w, http.StatusUnauthorized, "неверный текущий пароль")
		return
	}
	if err := auth.ValidatePassword(body.NewPassword); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "не удалось обработать пароль")
		return
	}
	if err := s.cfg.Store.Update(func(st *store.State) error { st.PassHash = hash; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cfg.Limiter.Reset(ip)
	s.cfg.Sessions.RevokeAll() // старые сессии (в том числе украденные) перестают действовать
	_ = s.setSession(w, user)
	s.cfg.Logf("administrator password changed, ip=%s", ip)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
