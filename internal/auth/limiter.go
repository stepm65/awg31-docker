package auth

import (
	"sync"
	"time"
)

type attempt struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

// Limiter блокирует ключ (обычно IP) после max неудач за window на время block.
type Limiter struct {
	mu     sync.Mutex
	m      map[string]*attempt
	max    int
	window time.Duration
	block  time.Duration
	cap    int
	now    func() time.Time
}

// NewLimiter создаёт ограничитель. cap — максимум отслеживаемых ключей.
func NewLimiter(max int, window, block time.Duration, cap int) *Limiter {
	return &Limiter{m: make(map[string]*attempt), max: max, window: window, block: block, cap: cap, now: time.Now}
}

// Check сообщает, заблокирован ли ключ сейчас, и сколько ждать.
func (l *Limiter) Check(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	a, ok := l.m[key]
	if !ok {
		return false, 0
	}
	now := l.now()
	if now.Before(a.blockedUntil) {
		return true, a.blockedUntil.Sub(now)
	}
	return false, 0
}

// Fail фиксирует неудачную попытку. Возвращает true, если ключ теперь заблокирован.
func (l *Limiter) Fail(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if len(l.m) >= l.cap {
		for k, a := range l.m {
			if now.After(a.blockedUntil) && now.Sub(a.windowStart) > l.window {
				delete(l.m, k)
			}
		}
		if len(l.m) >= l.cap { // всё ещё переполнено: безопаснее начать заново, чем расти без предела
			l.m = make(map[string]*attempt)
		}
	}
	a := l.m[key]
	if a == nil || now.Sub(a.windowStart) > l.window {
		a = &attempt{windowStart: now}
		l.m[key] = a
	}
	a.count++
	if a.count >= l.max {
		a.blockedUntil = now.Add(l.block)
		a.count = 0
		a.windowStart = now
		return true, l.block
	}
	return false, 0
}

// Reset снимает учёт неудач (после успешного входа).
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}
