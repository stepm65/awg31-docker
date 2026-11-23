package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type sessionEntry struct {
	user    string
	expires time.Time
}

// Sessions — хранилище сессий в памяти. В карте лежит только SHA-256 токена,
// поэтому дамп памяти не раскрывает действующие cookie.
type Sessions struct {
	mu  sync.Mutex
	m   map[string]sessionEntry
	ttl time.Duration
	max int
	now func() time.Time
}

// NewSessions создаёт хранилище с временем жизни ttl и лимитом max активных сессий.
func NewSessions(ttl time.Duration, max int) *Sessions {
	return &Sessions{m: make(map[string]sessionEntry), ttl: ttl, max: max, now: time.Now}
}

func hashToken(t string) string {
	sum := sha256.Sum256([]byte(t))
	return hex.EncodeToString(sum[:])
}

// Issue создаёт сессию и возвращает токен для cookie.
func (s *Sessions) Issue(user string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.m {
		if now.After(e.expires) {
			delete(s.m, k)
		}
	}
	// при переполнении вытесняем самую старую (с ближайшим сроком), а не случайную
	for len(s.m) >= s.max {
		var oldestKey string
		var oldest time.Time
		for k, e := range s.m {
			if oldestKey == "" || e.expires.Before(oldest) {
				oldestKey, oldest = k, e.expires
			}
		}
		delete(s.m, oldestKey)
	}
	s.m[hashToken(token)] = sessionEntry{user: user, expires: now.Add(s.ttl)}
	return token, nil
}

// Validate возвращает пользователя по токену, если сессия действует.
func (s *Sessions) Validate(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	key := hashToken(token)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[key]
	if !ok {
		return "", false
	}
	if s.now().After(e.expires) {
		delete(s.m, key)
		return "", false
	}
	return e.user, true
}

// Revoke завершает одну сессию.
func (s *Sessions) Revoke(token string) {
	s.mu.Lock()
	delete(s.m, hashToken(token))
	s.mu.Unlock()
}

// RevokeAll завершает все сессии (например, после смены пароля).
func (s *Sessions) RevokeAll() {
	s.mu.Lock()
	s.m = make(map[string]sessionEntry)
	s.mu.Unlock()
}
