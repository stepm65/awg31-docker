// Package conf разбирает и строго проверяет клиентские .conf для AmneziaWG.
//
// Конфиг в итоге попадает в awg-quick (bash-скрипт), поэтому правила жёсткие:
//   - разрешены только известные ключи, всё остальное — ошибка;
//   - хуки PreUp/PostUp/PreDown/PostDown запрещены полностью, команды для
//     маршрутизации генерирует само приложение (см. Render);
//   - каждое значение проверяется по типу (ключи, CIDR, порты, endpoint).
package conf

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// MaxSize — максимальный размер текста конфига.
const MaxSize = 64 << 10

// Pair — строка «ключ = значение».
type Pair struct {
	Key   string
	Value string
}

// Section — секция [Interface] или [Peer].
type Section struct {
	Name  string
	Pairs []Pair
}

// Values возвращает все значения ключа (в порядке появления).
func (s Section) Values(key string) []string {
	var out []string
	for _, p := range s.Pairs {
		if p.Key == key {
			out = append(out, p.Value)
		}
	}
	return out
}

// First возвращает первое значение ключа или пустую строку.
func (s Section) First(key string) string {
	for _, p := range s.Pairs {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

// Config — разобранный и проверенный конфиг.
type Config struct {
	Interface Section
	Peers     []Section
}

// Summary — несекретное описание профиля для UI.
type Summary struct {
	Endpoints   []string `json:"endpoints"`
	Addresses   []string `json:"addresses"`
	Peers       int      `json:"peers"`
	Obfuscation bool     `json:"obfuscation"`
	HasPSK      bool     `json:"has_psk"`
}

var (
	reDigits   = regexp.MustCompile(`^[0-9]{1,10}$`)
	reHRange   = regexp.MustCompile(`^[0-9]{1,10}(-[0-9]{1,10})?$`)
	reU16Range = regexp.MustCompile(`^[0-9]{1,5}(-[0-9]{1,5})?$`)
	reTable    = regexp.MustCompile(`^([0-9]{1,10}|[a-zA-Z][a-zA-Z0-9_]{0,31})$`)
	reFwMark   = regexp.MustCompile(`^(off|0x[0-9a-fA-F]{1,8}|[0-9]{1,10})$`)
	reHostname = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]{0,251}[A-Za-z0-9])?$`)
)

// Список ключей сверен с разбором конфигов в amneziawg-tools v3.1 (src/config.c):
// всё, что утилита не знает, она отвергает, поэтому лишнее здесь означало бы
// ошибку только при подъёме туннеля.
var interfaceKeys = buildKeys(
	"PrivateKey", "Address", "DNS", "MTU", "ListenPort", "Table", "FwMark",
	// параметры обфускации AmneziaWG 3.1
	"Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4", "H1", "H2", "H3", "H4",
	"I1", "I2", "I3", "I4", "I5",
	"HeaderProtectionKey", "ContentPaddingAddition", "RandomTrailers", "DisableCookies",
	"RekeyAfterTime", "RekeyTimeout", "RejectAfterTime", "KeepaliveTimeout", "MaxHandshakeAttempts",
)

var peerKeys = buildKeys(
	"PublicKey", "PresharedKey", "AllowedIPs", "Endpoint", "PersistentKeepalive", "AdvancedSecurity",
)

var hookKeys = map[string]bool{"preup": true, "postup": true, "predown": true, "postdown": true}

// ключи, которые WireGuard разрешает повторять
var repeatable = map[string]bool{"Address": true, "DNS": true, "AllowedIPs": true}

func buildKeys(names ...string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[strings.ToLower(n)] = n
	}
	return m
}

// Parse разбирает и проверяет текст конфига.
func Parse(text string) (*Config, error) {
	if len(text) > MaxSize {
		return nil, fmt.Errorf("конфиг слишком большой (максимум %d КиБ)", MaxSize>>10)
	}
	if !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		return nil, fmt.Errorf("конфиг должен быть текстом UTF-8 без нулевых байтов")
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")

	var (
		cfg       Config
		cur       *Section
		haveIface bool
	)
	for i, raw := range strings.Split(text, "\n") {
		n := i + 1
		line := raw
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			switch strings.ToLower(line) {
			case "[interface]":
				if haveIface {
					return nil, fmt.Errorf("строка %d: секция [Interface] повторяется", n)
				}
				haveIface = true
				cfg.Interface.Name = "Interface"
				cur = &cfg.Interface
			case "[peer]":
				cfg.Peers = append(cfg.Peers, Section{Name: "Peer"})
				cur = &cfg.Peers[len(cfg.Peers)-1]
			default:
				return nil, fmt.Errorf("строка %d: неизвестная секция %q", n, line)
			}
			continue
		}
		if cur == nil {
			return nil, fmt.Errorf("строка %d: параметр вне секции", n)
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("строка %d: ожидалось «ключ = значение»", n)
		}
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		lk := strings.ToLower(k)
		if hookKeys[lk] {
			return nil, fmt.Errorf("строка %d: хуки (%s) не поддерживаются: команды маршрутизации генерирует приложение", n, k)
		}
		table := interfaceKeys
		if cur != &cfg.Interface {
			table = peerKeys
		}
		canon, known := table[lk]
		if !known {
			return nil, fmt.Errorf("строка %d: неизвестный параметр %q в секции [%s]", n, k, cur.Name)
		}
		if !repeatable[canon] && cur.First(canon) != "" {
			return nil, fmt.Errorf("строка %d: параметр %s повторяется", n, canon)
		}
		if err := validateValue(canon, v); err != nil {
			return nil, fmt.Errorf("строка %d: %s: %w", n, canon, err)
		}
		cur.Pairs = append(cur.Pairs, Pair{Key: canon, Value: v})
	}

	if !haveIface {
		return nil, fmt.Errorf("нет секции [Interface]")
	}
	if cfg.Interface.First("PrivateKey") == "" {
		return nil, fmt.Errorf("[Interface]: не задан PrivateKey")
	}
	if len(cfg.Interface.Values("Address")) == 0 {
		return nil, fmt.Errorf("[Interface]: не задан Address")
	}
	if len(cfg.Peers) == 0 {
		return nil, fmt.Errorf("нет ни одной секции [Peer]")
	}
	for i, p := range cfg.Peers {
		if p.First("PublicKey") == "" {
			return nil, fmt.Errorf("[Peer #%d]: не задан PublicKey", i+1)
		}
		if len(p.Values("AllowedIPs")) == 0 {
			return nil, fmt.Errorf("[Peer #%d]: не задан AllowedIPs", i+1)
		}
	}
	return &cfg, nil
}

func validateValue(key, v string) error {
	if v == "" {
		return fmt.Errorf("пустое значение")
	}
	switch key {
	case "PrivateKey", "PublicKey", "PresharedKey":
		return validKey(v)
	case "Address", "AllowedIPs":
		return validList(v, validPrefixOrAddr)
	case "DNS":
		return validList(v, func(s string) error {
			if _, err := netip.ParseAddr(s); err == nil {
				return nil
			}
			if reHostname.MatchString(s) {
				return nil
			}
			return fmt.Errorf("%q не похоже на IP или имя домена", s)
		})
	case "MTU":
		return validInt(v, 576, 9200)
	case "ListenPort":
		return validInt(v, 0, 65535)
	case "Table":
		if v == "off" || v == "auto" || reTable.MatchString(v) {
			return nil
		}
		return fmt.Errorf("допустимо off, auto, число или имя таблицы")
	case "FwMark":
		if reFwMark.MatchString(v) {
			return nil
		}
		return fmt.Errorf("ожидалось off, число или 0x-значение")
	case "Jc", "Jmin", "Jmax", "S1", "S2", "S3", "S4":
		if reDigits.MatchString(v) {
			return nil
		}
		return fmt.Errorf("ожидалось целое число")
	case "HeaderProtectionKey":
		return validKey(v)
	case "ContentPaddingAddition", "RekeyAfterTime", "RekeyTimeout", "RejectAfterTime",
		"KeepaliveTimeout", "MaxHandshakeAttempts":
		return validU16Range(v)
	case "RandomTrailers", "DisableCookies", "AdvancedSecurity":
		switch strings.ToLower(v) {
		case "on", "off", "0", "1":
			return nil
		}
		return fmt.Errorf("ожидалось on или off")
	case "H1", "H2", "H3", "H4":
		if reHRange.MatchString(v) {
			return nil
		}
		return fmt.Errorf("ожидалось число или диапазон a-b")
	case "I1", "I2", "I3", "I4", "I5":
		if len(v) > 4096 {
			return fmt.Errorf("значение длиннее 4096 символов")
		}
		for _, r := range v {
			if r < 0x20 || r > 0x7e {
				return fmt.Errorf("допустимы только печатные ASCII-символы")
			}
		}
		return nil
	case "Endpoint":
		return validEndpoint(v)
	case "PersistentKeepalive":
		if v == "off" {
			return nil
		}
		return validInt(v, 0, 65535)
	}
	return fmt.Errorf("внутренняя ошибка: нет правила для %s", key)
}

// validU16Range проверяет «число» или «a-b» в пределах uint16, a <= b.
func validU16Range(v string) error {
	if !reU16Range.MatchString(v) {
		return fmt.Errorf("ожидалось число или диапазон a-b (0–65535)")
	}
	lo, hi, _ := strings.Cut(v, "-")
	a, _ := strconv.Atoi(lo)
	b := a
	if hi != "" {
		b, _ = strconv.Atoi(hi)
	}
	if a > 65535 || b > 65535 || a > b {
		return fmt.Errorf("диапазон должен быть в пределах 0–65535 и a <= b")
	}
	return nil
}

func validKey(v string) error {
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(raw) != 32 {
		return fmt.Errorf("ожидался ключ base64 длиной 32 байта")
	}
	return nil
}

func validList(v string, check func(string) error) error {
	for _, item := range strings.Split(v, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return fmt.Errorf("пустой элемент списка")
		}
		if err := check(item); err != nil {
			return err
		}
	}
	return nil
}

func validPrefixOrAddr(s string) error {
	if _, err := netip.ParsePrefix(s); err == nil {
		return nil
	}
	if _, err := netip.ParseAddr(s); err == nil {
		return nil
	}
	return fmt.Errorf("%q не похоже на IP-адрес или CIDR", s)
}

func validInt(v string, lo, hi int) error {
	n, err := strconv.Atoi(v)
	if err != nil || n < lo || n > hi {
		return fmt.Errorf("ожидалось число от %d до %d", lo, hi)
	}
	return nil
}

func validEndpoint(v string) error {
	host, port, err := net.SplitHostPort(v)
	if err != nil {
		return fmt.Errorf("ожидалось host:port")
	}
	if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("порт должен быть от 1 до 65535")
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	if !reHostname.MatchString(host) {
		return fmt.Errorf("хост %q не похож на IP или имя домена", host)
	}
	return nil
}

// Summary возвращает несекретное описание конфига.
func (c *Config) Summary() Summary {
	s := Summary{Peers: len(c.Peers)}
	for _, a := range c.Interface.Values("Address") {
		for _, item := range strings.Split(a, ",") {
			s.Addresses = append(s.Addresses, strings.TrimSpace(item))
		}
	}
	for _, p := range c.Peers {
		if e := p.First("Endpoint"); e != "" {
			s.Endpoints = append(s.Endpoints, e)
		}
		if p.First("PresharedKey") != "" {
			s.HasPSK = true
		}
	}
	for _, k := range []string{"Jc", "S1", "H1", "I1"} {
		if c.Interface.First(k) != "" {
			s.Obfuscation = true
		}
	}
	return s
}

// Hooks — команды, которые приложение само добавляет в [Interface].
type Hooks struct {
	PostUp   []string
	PostDown []string
}

// Render собирает текст конфига для awg-quick. Пользовательских хуков в Config
// быть не может (Parse их отвергает), поэтому единственные хуки — переданные
// вызывающим кодом.
func (c *Config) Render(h Hooks) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	for _, p := range c.Interface.Pairs {
		fmt.Fprintf(&b, "%s = %s\n", p.Key, p.Value)
	}
	for _, cmd := range h.PostUp {
		fmt.Fprintf(&b, "PostUp = %s\n", cmd)
	}
	for _, cmd := range h.PostDown {
		fmt.Fprintf(&b, "PostDown = %s\n", cmd)
	}
	for _, peer := range c.Peers {
		b.WriteString("\n[Peer]\n")
		for _, p := range peer.Pairs {
			fmt.Fprintf(&b, "%s = %s\n", p.Key, p.Value)
		}
	}
	return b.String()
}

// Sanitize готовит конфиг, полученный из подписки: удаляет строки-хуки
// (PreUp/PostUp/PreDown/PostDown) и возвращает список замечаний для оператора.
//
// Панель добавляет такие строки по просьбе администратора для клиентов с
// awg-quick (NAT, clamp MSS, правила маршрутизации). Выполнять чужие команды
// от root контейнер не должен, а NAT и подгонку MSS он делает сам. Если вместе с
// хуками был задан Table (маршрутизация по правилам, которых теперь нет),
// строка Table тоже удаляется: без своих правил она увела бы маршруты в
// таблицу, которой никто не пользуется, и трафик перестал бы идти через туннель.
func Sanitize(text string) (clean string, notes []string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	keep := make([]string, 0, len(lines))
	removed := map[string]int{}
	tableAt := -1
	tableVal := ""
	for _, raw := range lines {
		line := raw
		if idx := strings.IndexByte(line, '#'); idx >= 0 {
			line = line[:idx]
		}
		k, v, ok := strings.Cut(strings.TrimSpace(line), "=")
		lk := strings.ToLower(strings.TrimSpace(k))
		if ok && hookKeys[lk] {
			removed[strings.TrimSpace(k)]++
			continue
		}
		if ok && lk == "table" {
			tableAt, tableVal = len(keep), strings.TrimSpace(v)
		}
		keep = append(keep, raw)
	}
	total := 0
	for _, n := range removed {
		total += n
	}
	if total == 0 {
		return text, nil
	}
	notes = append(notes, fmt.Sprintf("удалено строк-хуков: %d (PreUp/PostUp/PreDown/PostDown не выполняются, NAT и MSS контейнер настраивает сам)", total))
	if tableAt >= 0 && tableVal != "" && !strings.EqualFold(tableVal, "auto") && !strings.EqualFold(tableVal, "off") {
		keep = append(keep[:tableAt], keep[tableAt+1:]...)
		notes = append(notes, fmt.Sprintf("удалена строка Table = %s: она рассчитана на удалённые правила маршрутизации", tableVal))
	}
	return strings.Join(keep, "\n"), notes
}

// IsServer — профиль сервера: задан ListenPort, и ни у одного пира нет
// Endpoint (клиенты подключаются сами). Такой туннель принимает входящие
// подключения и не бывает «основным» для трафика LAN.
func (c *Config) IsServer() bool {
	if c.Interface.First("ListenPort") == "" || len(c.Peers) == 0 {
		return false
	}
	for _, p := range c.Peers {
		if p.First("Endpoint") != "" {
			return false
		}
	}
	return true
}
