package auth

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

// hashWithIter строит корректный хеш с произвольным числом итераций — так
// проверяется именно граница, а не «мусорный» формат.
func hashWithIter(t *testing.T, pw string, iter int) string {
	t.Helper()
	salt := []byte("0123456789abcdef")
	dk, err := pbkdf2.Key(sha256.New, pw, salt, iter, 32)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("%s$%d$%s$%s", scheme, iter,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk))
}

func TestVerifyEnforcesIterationBounds(t *testing.T) {
	const pw = "correct horse battery"
	if !VerifyPassword(hashWithIter(t, pw, minIter), pw) {
		t.Fatal("хеш на нижней границе должен проходить")
	}
	if VerifyPassword(hashWithIter(t, pw, minIter-1), pw) {
		t.Fatal("слишком мало итераций: такой хеш не должен приниматься")
	}
	if testing.Short() {
		t.Skip("проверка верхней границы тяжёлая")
	}
	if VerifyPassword(hashWithIter(t, pw, maxIter+1), pw) {
		t.Fatal("слишком много итераций (DoS): такой хеш не должен приниматься")
	}
}

func TestPasswordRoundTrip(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "pbkdf2-sha256$600000$") {
		t.Fatalf("неожиданный формат: %s", h)
	}
	if !VerifyPassword(h, "correct horse battery") {
		t.Fatal("верный пароль не прошёл")
	}
	if VerifyPassword(h, "wrong password!!") {
		t.Fatal("неверный пароль прошёл")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	good, _ := HashPassword("correct horse battery")
	parts := strings.Split(good, "$")
	bad := []string{
		"",
		"plain",
		"bcrypt$1$a$b",
		strings.Join([]string{parts[0], "1", parts[2], parts[3]}, "$"),         // мало итераций
		strings.Join([]string{parts[0], "999999999", parts[2], parts[3]}, "$"), // DoS по итерациям
		strings.Join([]string{parts[0], parts[1], "!!!", parts[3]}, "$"),       // не base64
		strings.Join([]string{parts[0], parts[1], parts[2], "AA"}, "$"),        // слишком короткий хеш
		strings.Join([]string{parts[0], "abc", parts[2], parts[3]}, "$"),       // итерации не число
	}
	for _, b := range bad {
		if VerifyPassword(b, "correct horse battery") {
			t.Errorf("испорченный хеш принят: %q", b)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	if ValidatePassword("short") == nil {
		t.Error("короткий пароль принят")
	}
	if ValidatePassword("          ") == nil {
		t.Error("пароль из пробелов принят")
	}
	if ValidatePassword(strings.Repeat("a", MaxPasswordLen+1)) == nil {
		t.Error("слишком длинный пароль принят")
	}
	if err := ValidatePassword("long enough password"); err != nil {
		t.Errorf("нормальный пароль отвергнут: %v", err)
	}
}

func TestSessions(t *testing.T) {
	now := time.Now()
	s := NewSessions(time.Hour, 2)
	s.now = func() time.Time { return now }

	tok, err := s.Issue("admin")
	if err != nil {
		t.Fatal(err)
	}
	if u, ok := s.Validate(tok); !ok || u != "admin" {
		t.Fatal("сессия должна быть валидной")
	}
	if _, ok := s.Validate("deadbeef"); ok {
		t.Fatal("чужой токен принят")
	}
	if _, ok := s.Validate(""); ok {
		t.Fatal("пустой токен принят")
	}
	for k := range s.m {
		if k == tok {
			t.Fatal("в карте не должен лежать сырой токен")
		}
	}

	now = now.Add(2 * time.Hour)
	if _, ok := s.Validate(tok); ok {
		t.Fatal("просроченная сессия принята")
	}
}

func TestSessionsEvictOldestAndRevoke(t *testing.T) {
	now := time.Now()
	s := NewSessions(time.Hour, 2)
	s.now = func() time.Time { return now }
	t1, _ := s.Issue("a")
	now = now.Add(time.Minute)
	t2, _ := s.Issue("b")
	now = now.Add(time.Minute)
	t3, _ := s.Issue("c") // вытесняет t1 (самую старую)
	if _, ok := s.Validate(t1); ok {
		t.Fatal("самая старая сессия должна быть вытеснена")
	}
	if _, ok := s.Validate(t2); !ok {
		t.Fatal("t2 должна остаться")
	}
	s.Revoke(t2)
	if _, ok := s.Validate(t2); ok {
		t.Fatal("отозванная сессия принята")
	}
	s.RevokeAll()
	if _, ok := s.Validate(t3); ok {
		t.Fatal("после RevokeAll сессий быть не должно")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Now()
	l := NewLimiter(3, time.Minute, 30*time.Second, 10)
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if blocked, _ := l.Fail("1.1.1.1"); blocked {
			t.Fatal("блокировка слишком рано")
		}
	}
	if blocked, d := l.Fail("1.1.1.1"); !blocked || d != 30*time.Second {
		t.Fatal("третья неудача должна блокировать")
	}
	if blocked, _ := l.Check("1.1.1.1"); !blocked {
		t.Fatal("ключ должен быть заблокирован")
	}
	if blocked, _ := l.Check("2.2.2.2"); blocked {
		t.Fatal("другой ключ не должен блокироваться")
	}
	now = now.Add(31 * time.Second)
	if blocked, _ := l.Check("1.1.1.1"); blocked {
		t.Fatal("блокировка должна истечь")
	}
	l.Fail("3.3.3.3")
	l.Reset("3.3.3.3")
	if _, ok := l.m["3.3.3.3"]; ok {
		t.Fatal("Reset должен снимать учёт")
	}
}

func TestLimiterBoundedMemory(t *testing.T) {
	l := NewLimiter(5, time.Minute, time.Minute, 4)
	for i := 0; i < 100; i++ {
		l.Fail(string(rune('a' + i%50)))
	}
	if len(l.m) > 4 {
		t.Fatalf("таблица выросла сверх лимита: %d", len(l.m))
	}
}

func TestToken(t *testing.T) {
	a, _ := NewToken()
	b, _ := NewToken()
	if len(a) != 32 || a == b {
		t.Fatal("токен должен быть случайным, 32 hex-символа")
	}
	if !TokenEqual(a, a) || TokenEqual(a, b) {
		t.Fatal("TokenEqual работает неверно")
	}
}
