// Package ros — интеграция с RouterOS 7 (этап 4, по мотивам
// a-amneziawg-mikrotik): контейнер сам сообщает роутеру, какой трафик
// отправлять в VPN — списком адресов (address-list), маршрутами по BGP и
// DNS-перенаправлением доменов на контейнер (static FWD).
package ros

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"awg31/internal/subfetch"
)

// Client — клиент REST API RouterOS 7 (/rest).
type Client struct {
	base *url.URL
	user string
	pass string
	http *http.Client
}

// Conn — параметры подключения.
type Conn struct {
	URL       string // https://192.168.253.1 (http — только для частных адресов)
	User      string
	Pass      string
	PinSHA256 string // sha256//base64 или hex: для самоподписанного сертификата роутера
	Insecure  bool   // не проверять сертификат (не рекомендуется)
}

// ValidateURL проверяет адрес роутера: https, или http только для частного IP
// (пароль по http в открытую допустим лишь на внутреннем veth).
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return nil, errors.New("адрес роутера: https://IP или https://имя (без пути и логина)")
	}
	switch u.Scheme {
	case "https":
	case "http":
		a, err := netip.ParseAddr(u.Hostname())
		if err != nil || !(a.IsPrivate() || a.IsLoopback()) {
			return nil, errors.New("http допускается только для частного IP роутера; иначе — https")
		}
	default:
		return nil, errors.New("адрес роутера: https:// или http://")
	}
	u.Path = ""
	return u, nil
}

// NewClient создаёт клиента.
func NewClient(c Conn) (*Client, error) {
	u, err := ValidateURL(c.URL)
	if err != nil {
		return nil, err
	}
	if c.User == "" {
		return nil, errors.New("укажите пользователя RouterOS")
	}
	tc := &tls.Config{MinVersion: tls.VersionTLS12}
	if c.PinSHA256 != "" {
		pin, err := subfetch.NormalizePin(c.PinSHA256)
		if err != nil {
			return nil, err
		}
		// с пином цепочка не проверяется (сертификат роутера самоподписанный),
		// но сертификат обязан совпасть с пином
		tc.InsecureSkipVerify = true
		tc.VerifyPeerCertificate = func(raw [][]byte, _ [][]*x509.Certificate) error {
			if len(raw) == 0 {
				return errors.New("роутер не прислал сертификат")
			}
			cert, err := x509.ParseCertificate(raw[0])
			if err != nil {
				return err
			}
			if !subfetch.PinMatches(pin, cert) {
				return errors.New("сертификат роутера не совпадает с пином")
			}
			return nil
		}
	} else if c.Insecure {
		tc.InsecureSkipVerify = true
	}
	return &Client{base: u, user: c.User, pass: c.Pass, http: &http.Client{
		Timeout:   20 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tc, MaxIdleConns: 4, IdleConnTimeout: 60 * time.Second},
		// редиректы не нужны и могли бы увести пароль
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// Error — ответ RouterOS с ошибкой.
type Error struct {
	Code   int
	Detail string
}

func (e *Error) Error() string {
	switch e.Code {
	case 401:
		return "RouterOS: неверный логин или пароль (401)"
	case 403:
		return "RouterOS: у пользователя нет прав (403): нужна группа с policy=read,write,api,rest-api"
	case 404:
		return "RouterOS: путь не найден (404): нужен RouterOS 7 и включённая служба www-ssl/www"
	}
	if strings.Contains(e.Detail, "not allowed") {
		return fmt.Sprintf("RouterOS %d: %s — у группы пользователя не хватает прав: /user/group/set awg31 policy=read,write,api,rest-api", e.Code, e.Detail)
	}
	if e.Detail != "" {
		return fmt.Sprintf("RouterOS %d: %s", e.Code, e.Detail)
	}
	return fmt.Sprintf("RouterOS: ответ %d", e.Code)
}

// Row — запись RouterOS (все значения строками).
type Row map[string]string

func (c *Client) do(ctx context.Context, method, path string, body any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	u := *c.base
	u.Path = "/rest" + path
	req, err := http.NewRequestWithContext(ctx, method, u.String(), rd)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.pass)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		var e struct {
			Detail  string `json:"detail"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		d := e.Detail
		if d == "" {
			d = e.Message
		}
		if len(d) > 200 {
			d = d[:200]
		}
		return nil, &Error{resp.StatusCode, d}
	}
	return data, nil
}

// Print читает записи: path вида /ip/firewall/address-list, query — фильтр «ключ=значение».
func (c *Client) Print(ctx context.Context, path string, query map[string]string) ([]Row, error) {
	body := map[string]any{}
	if len(query) > 0 {
		// несколько условий RouterOS объединяет по «и»
		var q []string
		for k, v := range query {
			q = append(q, k+"="+v)
		}
		sort.Strings(q)
		body[".query"] = q
	}
	data, err := c.do(ctx, http.MethodPost, path+"/print", body)
	if err != nil {
		return nil, err
	}
	var raw []map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("RouterOS: неожиданный ответ: %w", err)
	}
	out := make([]Row, 0, len(raw))
	for _, r := range raw {
		row := Row{}
		for k, v := range r {
			row[k] = fmt.Sprint(v)
		}
		out = append(out, row)
	}
	return out, nil
}

// Add создаёт запись и возвращает её .id.
func (c *Client) Add(ctx context.Context, path string, fields map[string]string) (string, error) {
	data, err := c.do(ctx, http.MethodPut, path, fields)
	if err != nil {
		return "", err
	}
	var r map[string]any
	_ = json.Unmarshal(data, &r)
	return fmt.Sprint(r[".id"]), nil
}

var reID = regexp.MustCompile(`^\*[0-9A-Fa-f]{1,16}$`)

// Set меняет запись по .id.
func (c *Client) Set(ctx context.Context, path, id string, fields map[string]string) error {
	if !reID.MatchString(id) {
		return fmt.Errorf("RouterOS: неожиданный .id %q", id)
	}
	_, err := c.do(ctx, http.MethodPatch, path+"/"+id, fields)
	return err
}

// Remove удаляет запись по .id.
func (c *Client) Remove(ctx context.Context, path, id string) error {
	if !reID.MatchString(id) {
		return fmt.Errorf("RouterOS: неожиданный .id %q", id)
	}
	_, err := c.do(ctx, http.MethodDelete, path+"/"+id, nil)
	return err
}

// Identity возвращает имя и версию роутера (проверка подключения).
func (c *Client) Identity(ctx context.Context) (string, string, error) {
	data, err := c.do(ctx, http.MethodGet, "/system/identity", nil)
	if err != nil {
		return "", "", err
	}
	var id struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(data, &id)
	data, err = c.do(ctx, http.MethodGet, "/system/resource", nil)
	if err != nil {
		return id.Name, "", err
	}
	var res struct {
		Version string `json:"version"`
		Arch    string `json:"architecture-name"`
	}
	_ = json.Unmarshal(data, &res)
	return id.Name, strings.TrimSpace(res.Version + " " + res.Arch), nil
}
