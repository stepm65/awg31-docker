package geo

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ErrNoIcon — у сайта нет пригодного значка (или сервер списков его не отдал).
var ErrNoIcon = errors.New("значка нет")

const (
	maxIconSize   = 256 << 10
	iconRetryAt   = 24 * time.Hour // как часто повторять поиск значка, которого не нашли
	iconFetchSlot = 4              // сколько значков качается одновременно
)

var (
	iconSem     = make(chan struct{}, iconFetchSlot)
	iconSiteRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*(\.[a-z0-9-]+)+$`)
	iconCacheMu sync.Mutex
)

// IconBase выводит адрес сервера списков из ссылки на geosite.dat: у IPList она вида
// https://host/?format=geosite&data=domains, и значки лежат там же, в /favicon?site=<сайт>.
// Для любой другой ссылки (Loyalsoldier и т. п.) значков нет, и возвращается пустая строка.
func IconBase(geositeURL string, allowHTTP bool) string {
	u, err := url.Parse(strings.TrimSpace(geositeURL))
	if err != nil || ValidURL(geositeURL, allowHTTP) != nil || (u.Path != "" && u.Path != "/") || u.Query().Get("format") != "geosite" {
		return ""
	}
	return u.Scheme + "://" + u.Host + "/"
}

// IconSite говорит, похож ли тег на имя сайта (telegram.org), а не на группу (messengers).
func IconSite(tag string) bool { return len(tag) <= 64 && iconSiteRe.MatchString(tag) }

// Icon возвращает значок сайта с сервера списков и его тип. Значки кешируются на диске в
// <Dir>/icons; найденное хранится постоянно, ненайденное перепроверяется раз в сутки. Скачивается
// не больше четырёх значков одновременно, размер ограничен, принимаются только растровые картинки
// (png, jpeg, gif, webp, ico): страница-заглушка или svg вместо значка отбрасывается.
func (s *Store) Icon(ctx context.Context, base, site string, allowHTTP bool) ([]byte, string, error) {
	if !IconSite(site) {
		return nil, "", ErrNoIcon
	}
	dir := filepath.Join(s.Dir, "icons")
	img, none := filepath.Join(dir, site+".img"), filepath.Join(dir, site+".none")
	if data, err := os.ReadFile(img); err == nil {
		if ct := iconType(data); ct != "" {
			return data, ct, nil
		}
	}
	if st, err := os.Stat(none); err == nil && time.Since(st.ModTime()) < iconRetryAt {
		return nil, "", ErrNoIcon
	}
	if err := ValidURL(base, allowHTTP); err != nil {
		return nil, "", err
	}
	select {
	case iconSem <- struct{}{}:
		defer func() { <-iconSem }()
	case <-ctx.Done():
		return nil, "", ctx.Err()
	}
	data, err := s.fetchIcon(ctx, strings.TrimRight(base, "/")+"/favicon?site="+url.QueryEscape(site))
	if err != nil && !errors.Is(err, ErrNoIcon) {
		return nil, "", err // сбой сети: ничего не запоминаем, в следующий раз попробуем снова
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, "", err
	}
	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()
	if err != nil {
		_ = os.WriteFile(none, nil, 0o600)
		return nil, "", ErrNoIcon
	}
	_ = os.Remove(none)
	if err := os.WriteFile(img, data, 0o600); err != nil {
		return nil, "", err
	}
	return data, iconType(data), nil
}

func (s *Store) fetchIcon(ctx context.Context, rawURL string) ([]byte, error) {
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNoIcon
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ответ %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxIconSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIconSize || iconType(data) == "" {
		return nil, ErrNoIcon
	}
	return data, nil
}

// iconType определяет тип картинки по содержимому, а не по заголовку сервера: у сервера
// списков встречаются страницы вместо значков. Пустая строка — не картинка или заглушка 1×1.
func iconType(data []byte) string {
	if len(data) < 24 {
		return ""
	}
	ct := http.DetectContentType(data)
	switch ct {
	case "image/png":
		if binary.BigEndian.Uint32(data[16:20]) <= 1 && binary.BigEndian.Uint32(data[20:24]) <= 1 {
			return "" // прозрачный пиксель вместо значка
		}
	case "image/jpeg", "image/gif", "image/webp", "image/x-icon", "image/vnd.microsoft.icon":
	default:
		return ""
	}
	return ct
}
