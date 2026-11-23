// Package subsync обновляет профили из подписок: по расписанию и по запросу.
package subsync

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"awg31/internal/conf"
	"awg31/internal/store"
	"awg31/internal/subfetch"
	"awg31/internal/tunnel"
)

const (
	// MinInterval — минимальный интервал автообновления в минутах.
	MinInterval = 5
	// MaxInterval — максимальный интервал (7 суток).
	MaxInterval = 7 * 24 * 60
)

// DefaultStaleAfter — через сколько секунд без рукопожатия поднятый туннель
// считается неработающим (как в скрипте awg-sub.sh панели).
const DefaultStaleAfter = 180 * time.Second

// Service выполняет синхронизацию подписок.
type Service struct {
	Store         *store.Store
	Tunnels       *tunnel.Manager
	AllowHTTP     bool
	AllowLoopback bool // только для тестов
	StaleAfter    time.Duration
	Logf          func(format string, args ...any)

	mu  sync.Mutex // одна синхронизация за раз
	now func() time.Time
}

func (s *Service) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s *Service) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// brokenTunnel сообщает, что туннель name поднят, но рукопожатий нет дольше
// StaleAfter. Такой туннель мог унести маршрут по умолчанию (например, после
// смены обфускации на сервере), и адрес подписки через него не достать.
func (s *Service) brokenTunnel(ctx context.Context, name string) bool {
	if s.Tunnels == nil {
		return false
	}
	ifaces, err := s.Tunnels.Status(ctx)
	if err != nil {
		return false
	}
	limit := s.StaleAfter
	if limit <= 0 {
		limit = DefaultStaleAfter
	}
	for _, i := range ifaces {
		if i.Name != name {
			continue
		}
		var newest int64
		for _, p := range i.Peers {
			if p.LatestHandshake > newest {
				newest = p.LatestHandshake
			}
		}
		return newest == 0 || s.clock().Unix()-newest > int64(limit/time.Second)
	}
	return false
}

// Sync скачивает конфиг подписки name, проверяет его и обновляет профиль.
//
//   - Строки-хуки в полученном конфиге удаляются (conf.Sanitize), о чём остаётся замечание.
//   - Если конфиг изменился и туннель поднят, туннель перезапускается; если новый
//     конфиг не поднялся, возвращается прежний профиль и прежний туннель.
//   - Если адрес подписки недоступен, а туннель поднят и не отвечает, туннель
//     на время запроса опускается и затем поднимается снова (с новым или прежним конфигом).
func (s *Service) Sync(ctx context.Context, name string) (changed bool, err error) {
	if !store.ValidName(name) {
		return false, errors.New("недопустимое имя профиля")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.Store.Snapshot()
	sub, ok := st.Subs[name]
	if !ok {
		return false, errors.New("для профиля не настроена подписка")
	}

	record := func(newETag, notice string, syncErr error) {
		_ = s.Store.Update(func(st *store.State) error {
			x, ok := st.Subs[name]
			if !ok {
				return nil
			}
			x.LastSync = s.clock()
			x.LastError = ""
			if syncErr != nil {
				x.LastError = syncErr.Error()
			} else {
				x.ETag = newETag
				x.LastNotice = notice
			}
			st.Subs[name] = x
			return nil
		})
	}

	cur, rerr := s.Store.ReadProfile(name)
	if rerr != nil && !errors.Is(rerr, store.ErrNotFound) {
		record("", "", rerr)
		return false, rerr
	}

	fetchOpts := subfetch.Options{
		URL: sub.URL, Token: sub.Token, PinSHA256: sub.PinSHA256, InsecureTLS: sub.InsecureTLS,
		AllowHTTP: s.AllowHTTP, AllowLoopback: s.AllowLoopback,
	}
	body, etag, notModified, err := subfetch.Fetch(ctx, fetchOpts, sub.ETag)

	// Туннель опущен ради запроса мимо него: любой выход ниже обязан его поднять.
	reopened := false
	var se *subfetch.StatusError
	if err != nil && !errors.As(err, &se) && ctx.Err() == nil && cur != "" && s.brokenTunnel(ctx, name) {
		s.logf("подписка %s: туннель не отвечает, запрашиваю конфиг в обход него", name)
		if derr := s.Tunnels.Down(ctx, name); derr == nil {
			reopened = true
			body, etag, notModified, err = subfetch.Fetch(ctx, fetchOpts, sub.ETag)
		}
	}
	raise := func(content string) error {
		if !reopened {
			return nil
		}
		reopened = false
		// Отмена запроса (закрыли страницу) не должна оставить туннель опущенным.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
		defer cancel()
		return s.Tunnels.Up(rctx, name, content)
	}
	fail := func(e error) (bool, error) {
		if uerr := raise(cur); uerr != nil {
			e = fmt.Errorf("%w; прежний туннель не поднялся: %v", e, uerr)
		}
		record("", "", e)
		return false, e
	}

	if err != nil {
		return fail(err)
	}
	if notModified {
		if uerr := raise(cur); uerr != nil {
			record("", "", uerr)
			return false, uerr
		}
		record(sub.ETag, sub.LastNotice, nil)
		return false, nil
	}

	clean, notes := conf.Sanitize(body)
	if _, err := conf.Parse(clean); err != nil {
		return fail(fmt.Errorf("сервер подписки вернул недопустимый конфиг: %w", err))
	}
	notice := strings.Join(notes, "; ")

	changed = cur != clean
	if changed {
		if err := s.Store.WriteProfile(name, clean); err != nil {
			return fail(err)
		}
	}

	needUp := reopened || (changed && st.Up[name])
	if needUp && s.Tunnels != nil {
		wasReopened := reopened
		reopened = false
		if err := s.restart(ctx, name, clean, !wasReopened); err != nil {
			// Новый конфиг не поднялся: возвращаем прежний профиль и туннель.
			if changed && cur != "" {
				_ = s.Store.WriteProfile(name, cur)
			}
			msg := fmt.Errorf("новый конфиг не поднялся (%v)", err)
			if cur != "" {
				if rerr := s.restart(ctx, name, cur, false); rerr != nil && !errors.Is(rerr, tunnel.ErrAlreadyUp) {
					msg = fmt.Errorf("%w; прежний конфиг тоже не поднялся: %v", msg, rerr)
				} else {
					msg = fmt.Errorf("%w; возвращён прежний конфиг", msg)
				}
			}
			s.logf("подписка %s: %v", name, msg)
			record("", "", msg)
			return false, msg
		}
	}
	record(etag, notice, nil)
	return changed, nil
}

// restart перезапускает туннель с содержимым content. Если опускать нечего
// (туннель уже опущен), передайте down=false.
func (s *Service) restart(ctx context.Context, name, content string, down bool) error {
	if s.Tunnels == nil {
		return nil
	}
	if down {
		if err := s.Tunnels.Down(ctx, name); err != nil && !errors.Is(err, tunnel.ErrNotUp) {
			return err
		}
	}
	return s.Tunnels.Up(ctx, name, content)
}

// Run раз в минуту проверяет, какие подписки пора обновить, пока ctx не отменён.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.syncDue(ctx)
		}
	}
}

func (s *Service) syncDue(ctx context.Context) {
	now := s.clock()
	for name, sub := range s.Store.Snapshot().Subs {
		if sub.IntervalMin <= 0 {
			continue
		}
		if !sub.LastSync.IsZero() && now.Sub(sub.LastSync) < time.Duration(sub.IntervalMin)*time.Minute {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, time.Minute)
		changed, err := s.Sync(cctx, name)
		cancel()
		switch {
		case err != nil:
			s.logf("подписка %s: ошибка: %v", name, err)
		case changed:
			s.logf("подписка %s: профиль обновлён", name)
		}
	}
}
