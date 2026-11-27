// Package subfetch скачивает конфиг по URL подписки, не позволяя открытый HTTP,
// молчаливое отключение проверки TLS и обращение к loopback/link-local адресам.
package subfetch

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// MaxBody — максимальный размер ответа.
const MaxBody = 1 << 20

// Options — параметры загрузки.
type Options struct {
	URL   string
	Token string // отправляется как Authorization: Bearer, а не в URL
	// PinSHA256 — отпечаток сервера: либо SHA-256 листового сертификата (64 hex),
	// либо SHA-256 его открытого ключа в формате curl «sha256//base64» (так его
	// выдаёт панель amneziawg31-installer). Если задан, он заменяет проверку
	// цепочки (удобно для самоподписанных сертификатов).
	PinSHA256 string
	// InsecureTLS отключает проверку сертификата. Только явным решением оператора.
	InsecureTLS bool
	// AllowHTTP разрешает схему http:// (только явным решением оператора).
	AllowHTTP bool
	// AllowLoopback разрешает loopback-адреса (нужно только в тестах).
	AllowLoopback bool
	Timeout       time.Duration
}

// spkiPrefix — префикс пина открытого ключа, как у curl --pinnedpubkey.
const spkiPrefix = "sha256//"

// NormalizePin приводит отпечаток к каноническому виду: 64 hex-символа в нижнем
// регистре (сертификат) или «sha256//» + base64 от 32 байт (открытый ключ).
func NormalizePin(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	if len(s) >= len(spkiPrefix) && strings.EqualFold(s[:len(spkiPrefix)], spkiPrefix) {
		raw, err := base64.StdEncoding.DecodeString(s[len(spkiPrefix):])
		if err != nil || len(raw) != sha256.Size {
			return "", errors.New("пин sha256//… должен содержать base64 от 32 байт (SHA-256 открытого ключа)")
		}
		return spkiPrefix + base64.StdEncoding.EncodeToString(raw), nil
	}
	s = strings.ToLower(strings.NewReplacer(":", "", " ", "", "-", "").Replace(s))
	if len(s) != 64 {
		return "", errors.New("отпечаток SHA-256 должен содержать 64 hex-символа (или иметь вид sha256//base64)")
	}
	if _, err := hex.DecodeString(s); err != nil {
		return "", errors.New("отпечаток SHA-256 должен быть шестнадцатеричным")
	}
	return s, nil
}

// pinMatches сверяет сертификат сервера с каноническим пином.
func pinMatches(pin string, cert *x509.Certificate) bool {
	var got string
	if strings.HasPrefix(pin, spkiPrefix) {
		sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
		got = spkiPrefix + base64.StdEncoding.EncodeToString(sum[:])
	} else {
		sum := sha256.Sum256(cert.Raw)
		got = hex.EncodeToString(sum[:])
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(pin)) == 1
}

// MaskURL скрывает секретную часть ссылки подписки: у панели токен лежит прямо в
// пути (/sub/awg0/<токен>), поэтому весь URL — секрет. Остаются схема, хост и
// короткие сегменты пути; длинные сегменты и строка запроса заменяются точками.
func MaskURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return ""
	}
	segs := strings.Split(u.Path, "/")
	last := -1
	for i, seg := range segs {
		if seg != "" {
			last = i
		}
	}
	for i, seg := range segs {
		// Токен панели лежит в последнем сегменте пути, каким бы коротким он
		// ни был; длинные сегменты скрываются где бы они ни стояли.
		if i == last || len(seg) >= 16 {
			segs[i] = "••••••"
		}
	}
	out := u.Scheme + "://" + u.Host + strings.Join(segs, "/")
	if u.RawQuery != "" {
		out += "?••••"
	}
	return out
}

// StatusError — ответ сервера подписки с неожиданным кодом.
type StatusError struct {
	Code int
	Host string
}

func (e *StatusError) Error() string {
	switch e.Code {
	case http.StatusForbidden:
		return fmt.Sprintf("сервер %s: клиент отключён в панели (HTTP 403)", e.Host)
	case http.StatusNotFound:
		return fmt.Sprintf("сервер %s: ссылка подписки недействительна или перевыпущена (HTTP 404), запросите новую", e.Host)
	case http.StatusTooManyRequests:
		return fmt.Sprintf("сервер %s: слишком частые запросы (HTTP 429), повторите позже", e.Host)
	}
	return fmt.Sprintf("сервер %s ответил HTTP %d", e.Host, e.Code)
}

// ValidateURL проверяет URL подписки.
func ValidateURL(raw string, allowHTTP bool) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("некорректный URL: %w", err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !allowHTTP {
			return nil, errors.New("подписки по открытому HTTP запрещены: приватный ключ и токен ушли бы по сети без шифрования (используйте https:// или ALLOW_INSECURE_SUBSCRIPTIONS=on)")
		}
	default:
		return nil, errors.New("допустима только схема https://")
	}
	if u.Host == "" || u.Hostname() == "" {
		return nil, errors.New("в URL не указан хост")
	}
	if u.User != nil {
		return nil, errors.New("логин и пароль в URL не поддерживаются: используйте поле токена")
	}
	return u, nil
}

func forbidden(ip netip.Addr, allowLoopback bool) bool {
	ip = ip.Unmap()
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	return ip.IsLoopback() && !allowLoopback
}

// Fetch скачивает конфиг. etag — предыдущий ETag для условного запроса.
// Возвращает тело, новый ETag и признак «не изменилось».
func Fetch(ctx context.Context, o Options, etag string) (body, newETag string, notModified bool, err error) {
	u, err := ValidateURL(o.URL, o.AllowHTTP)
	if err != nil {
		return "", "", false, err
	}
	pin, err := NormalizePin(o.PinSHA256)
	if err != nil {
		return "", "", false, err
	}
	timeout := o.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case pin != "":
		tlsCfg.InsecureSkipVerify = true // доверие даёт отпечаток, а не цепочка
		tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("сервер не прислал сертификат")
			}
			if !pinMatches(pin, cs.PeerCertificates[0]) {
				return errors.New("отпечаток сертификата не совпадает с ожидаемым")
			}
			return nil
		}
	case o.InsecureTLS:
		tlsCfg.InsecureSkipVerify = true
	}

	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		// Control вызывается для реального адреса соединения, поэтому подмена DNS
		// (rebinding) не позволяет обойти запрет.
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if forbidden(ip, o.AllowLoopback) {
				return fmt.Errorf("адрес %s запрещён для подписок", ip)
			}
			return nil
		},
	}
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         dialer.DialContext,
			TLSClientConfig:     tlsCfg,
			TLSHandshakeTimeout: 10 * time.Second,
			DisableKeepAlives:   true,
			Proxy:               nil,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("слишком много перенаправлений")
			}
			if req.URL.Scheme != "https" && !o.AllowHTTP {
				return errors.New("перенаправление на открытый HTTP запрещено")
			}
			return nil
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("User-Agent", "awg31")
	req.Header.Set("Accept", "text/plain")
	if o.Token != "" {
		req.Header.Set("Authorization", "Bearer "+o.Token)
	}
	if e := strings.Trim(strings.TrimSpace(etag), `"`); e != "" {
		req.Header.Set("If-None-Match", `"`+e+`"`)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", false, explain(u.Host, stripURL(err))
	}
	defer resp.Body.Close()
	// запрос отменили, пока шёл ответ: результат уже никому не нужен, и отмена
	// должна дойти до вызывающего как отмена, а не как «пустой ответ»
	if err := req.Context().Err(); err != nil {
		return "", "", false, err
	}

	if resp.StatusCode == http.StatusNotModified {
		return "", etag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", false, &StatusError{Code: resp.StatusCode, Host: u.Host}
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, MaxBody+1))
	if err != nil {
		return "", "", false, fmt.Errorf("ошибка чтения ответа: %w", stripURL(err))
	}
	if len(raw) > MaxBody {
		return "", "", false, fmt.Errorf("ответ больше %d КиБ", MaxBody>>10)
	}
	if err := req.Context().Err(); err != nil {
		return "", "", false, err
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		return "", "", false, errors.New("сервер вернул пустой ответ")
	}
	return text + "\n", strings.Trim(resp.Header.Get("ETag"), `" `), false, nil
}

// explain оформляет сетевую ошибку запроса: к исходному тексту добавляется подсказка,
// что проверить. Сама ошибка остаётся в цепочке (errors.Is/As работают).
func explain(host string, err error) error {
	if h := hint(err); h != "" {
		return fmt.Errorf("запрос к %s не удался: %w (%s)", host, err, h)
	}
	return fmt.Errorf("запрос к %s не удался: %w", host, err)
}

// hint — пояснение к типичным сбоям сети; пустая строка, если добавить нечего.
func hint(err error) string {
	const noNet = "проверьте, что у контейнера есть выход в интернет (на MikroTik — masquerade для адреса контейнера в srcnat) и работает DNS"
	var dnsErr *net.DNSError
	switch {
	case errors.Is(err, context.Canceled):
		return "запрос прерван, не дождавшись ответа: страницу обновили или закрыли, либо контейнер не смог связаться с сервером; " + noNet
	case errors.As(err, &dnsErr):
		return "не удалось найти адрес сервера по имени; " + noNet
	case errors.Is(err, syscall.ECONNREFUSED):
		return "сервер отказал в соединении: проверьте адрес и порт ссылки"
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return "до сервера нет маршрута; " + noNet
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &ne) && ne.Timeout()) {
		return "сервер не ответил вовремя; " + noNet + ", а сервер доступен с адреса вашего роутера"
	}
	return ""
}

// stripURL убирает из ошибки url.Error полный URL (в нём может быть токен в пути).
func stripURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// PinMatches проверяет сертификат по нормализованному пину (sha256//… или hex).
func PinMatches(pin string, cert *x509.Certificate) bool { return pinMatches(pin, cert) }
