// Package auth содержит хеширование паролей, сессии и ограничитель попыток входа.
package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// Iterations — число итераций PBKDF2-HMAC-SHA256 для новых хешей (OWASP, 2023+).
	Iterations = 600_000
	minIter    = 100_000
	maxIter    = 10_000_000
	scheme     = "pbkdf2-sha256"

	MinPasswordLen = 10
	MaxPasswordLen = 256
)

// ValidatePassword проверяет требования к новому паролю.
func ValidatePassword(pw string) error {
	if len(pw) < MinPasswordLen {
		return fmt.Errorf("пароль должен содержать минимум %d символов", MinPasswordLen)
	}
	if len(pw) > MaxPasswordLen {
		return fmt.Errorf("пароль длиннее %d символов", MaxPasswordLen)
	}
	if strings.TrimSpace(pw) == "" {
		return errors.New("пароль не может состоять из пробелов")
	}
	return nil
}

// HashPassword возвращает строку вида pbkdf2-sha256$<iter>$<salt>$<hash>.
func HashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk, err := pbkdf2.Key(sha256.New, pw, salt, Iterations, 32)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s$%d$%s$%s", scheme, Iterations,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk)), nil
}

// VerifyPassword сравнивает пароль с хешем за постоянное время.
func VerifyPassword(encoded, pw string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != scheme {
		return false
	}
	iter, err := strconv.Atoi(parts[1])
	if err != nil || iter < minIter || iter > maxIter {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil || len(salt) < 8 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(want) < 16 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// NewToken возвращает случайный токен из 16 байт в hex (для токена первичной настройки).
func NewToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

// TokenEqual сравнивает токены за постоянное время.
func TokenEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
