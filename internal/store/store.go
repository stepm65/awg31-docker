// Package store хранит состояние приложения и профили туннелей на диске.
//
// Все файлы создаются с правами 0600, каталоги — 0700, запись атомарная
// (временный файл + fsync + rename), поэтому обрыв питания роутера не оставляет
// наполовину записанный конфиг.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"awg31/internal/conf"
)

// ErrNotFound — профиль не найден.
var ErrNotFound = errors.New("профиль не найден")

// MaxNameLen — ограничение Linux на имя интерфейса (IFNAMSIZ-1); awg-quick берёт имя из файла.
const MaxNameLen = 15

var reName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,15}$`)

// ValidName проверяет имя профиля (оно же имя интерфейса).
func ValidName(name string) bool { return reName.MatchString(name) }

// Sub — настройки подписки профиля. Токен наружу не отдаётся.
type Sub struct {
	URL         string    `json:"url"`
	Token       string    `json:"token,omitempty"`
	PinSHA256   string    `json:"pin_sha256,omitempty"`
	InsecureTLS bool      `json:"insecure_tls,omitempty"`
	IntervalMin int       `json:"interval_min"`
	ETag        string    `json:"etag,omitempty"`
	LastSync    time.Time `json:"last_sync,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
	// LastNotice — замечание после успешной синхронизации (например, что из
	// конфига удалены хуки). Не ошибка.
	LastNotice string `json:"last_notice,omitempty"`
}

// Режимы резервирования туннелей.
const (
	FailoverOff     = "off"     // переключение только вручную
	FailoverStandby = "standby" // все туннели из списка подняты, основной меняется мгновенно
	FailoverSwitch  = "switch"  // поднят только основной, при сбое поднимается следующий
)

// Failover — настройки резервирования (этап 2: Multi-Tunnel + failover).
type Failover struct {
	Mode            string `json:"mode"`              // off | standby | switch
	Trigger         string `json:"trigger"`           // handshake | probe | hybrid
	IntervalSec     int    `json:"interval_sec"`      // период проверки
	Threshold       int    `json:"threshold"`         // столько неудач подряд — туннель неисправен
	CooldownSec     int    `json:"cooldown_sec"`      // пауза для неисправного туннеля
	HandshakeMaxAge int    `json:"handshake_max_age"` // рукопожатие старше — неудача
	GraceSec        int    `json:"grace_sec"`         // столько секунд после подъёма ждём первого рукопожатия
	ProbeTarget     string `json:"probe_target"`      // host:port для TCP-пробы через туннель
	Failback        bool   `json:"failback"`          // возвращаться к более приоритетному туннелю
}

// DefaultFailover — значения по умолчанию (как в a-amneziawg-mikrotik).
func DefaultFailover() Failover {
	return Failover{Mode: FailoverOff, Trigger: "hybrid", IntervalSec: 15, Threshold: 3, CooldownSec: 180,
		HandshakeMaxAge: 190, GraceSec: 35, ProbeTarget: "1.1.1.1:443"}
}

// Validate проверяет настройки резервирования.
func (f Failover) Validate() error {
	switch f.Mode {
	case FailoverOff, FailoverStandby, FailoverSwitch:
	default:
		return fmt.Errorf("режим: off, standby или switch")
	}
	switch f.Trigger {
	case "handshake", "probe", "hybrid":
	default:
		return fmt.Errorf("признак сбоя: handshake, probe или hybrid")
	}
	switch {
	case f.IntervalSec < 5 || f.IntervalSec > 3600:
		return fmt.Errorf("период проверки: 5–3600 секунд")
	case f.Threshold < 1 || f.Threshold > 20:
		return fmt.Errorf("порог неудач: 1–20")
	case f.CooldownSec < 0 || f.CooldownSec > 86400:
		return fmt.Errorf("пауза: 0–86400 секунд")
	case f.HandshakeMaxAge < 130 || f.HandshakeMaxAge > 3600:
		// рукопожатие обновляется раз в ~2 минуты, меньший порог даёт ложные сбои
		return fmt.Errorf("максимальный возраст рукопожатия: 130–3600 секунд")
	case f.GraceSec < 5 || f.GraceSec > 600:
		return fmt.Errorf("ожидание первого рукопожатия: 5–600 секунд")
	}
	if f.Trigger != "handshake" {
		if err := ValidProbeTarget(f.ProbeTarget); err != nil {
			return err
		}
	}
	return nil
}

// ValidProbeTarget проверяет адрес пробы: IP:порт (имя хоста не допускается —
// его пришлось бы разрешать через неисправный туннель).
func ValidProbeTarget(t string) error {
	ap, err := netip.ParseAddrPort(strings.TrimSpace(t))
	if err != nil || ap.Port() == 0 || ap.Addr().IsUnspecified() {
		return fmt.Errorf("цель пробы: IP-адрес и порт, например 1.1.1.1:443")
	}
	return nil
}

// Policy — маршрутизация по правилам и DNS (этап 3). Проверяется пакетом policy.
type Policy struct {
	Enabled        bool   `json:"enabled"`
	DefaultAction  string `json:"default_action"` // proxy | direct
	Rules          string `json:"rules"`
	ProxyDNS       string `json:"proxy_dns"`  // через VPN: IP[:порт] или https://…/dns-query, через запятую
	DirectDNS      string `json:"direct_dns"` // мимо VPN
	GeoSiteURL     string `json:"geosite_url"`
	GeoIPURL       string `json:"geoip_url"`
	GeoUpdateHours int    `json:"geo_update_hours"` // 0 — обновлять только вручную
}

// DefaultPolicy — значения по умолчанию (как в a-amneziawg-mikrotik).
func DefaultPolicy() Policy {
	return Policy{DefaultAction: "proxy", ProxyDNS: "1.1.1.1, 8.8.8.8", DirectDNS: "77.88.8.8, 1.1.1.1",
		GeoSiteURL:     "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
		GeoIPURL:       "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
		GeoUpdateHours: 24}
}

// PolicySettings возвращает действующие настройки политики.
func (s State) PolicySettings() Policy {
	if s.Policy == nil {
		return DefaultPolicy()
	}
	return *s.Policy
}

// Режимы интеграции с RouterOS.
const (
	RouterOff         = "off"
	RouterAddressList = "address_list" // сети PROXY и выученные адреса — в address-list роутера
	RouterBGP         = "bgp"          // сети PROXY — маршрутами по BGP
)

// Router — интеграция с RouterOS 7 через REST API (этап 4).
type Router struct {
	Mode        string `json:"mode"`
	URL         string `json:"url"`
	User        string `json:"user"`
	Pass        string `json:"pass,omitempty"` // никогда не отдаётся через API
	PinSHA256   string `json:"pin_sha256,omitempty"`
	Insecure    bool   `json:"insecure,omitempty"`
	AddressList string `json:"address_list"`
	SyncDNS     bool   `json:"sync_dns"`               // static FWD на контейнер для доменных правил
	ForwardTo   string `json:"forward_to,omitempty"`   // IP контейнера для FWD и BGP (пусто — сам)
	StaticLimit int    `json:"static_limit"`           // предел сетей из правил и geoip
	DynamicMin  int    `json:"dynamic_minutes"`        // срок выученных адресов в address-list
	DomainLimit int    `json:"domain_limit"`           // предел имён для FWD
	BGPLocalAS  uint32 `json:"bgp_local_as,omitempty"` // AS контейнера
	BGPRemoteAS uint32 `json:"bgp_remote_as,omitempty"`
	BGPPeer     string `json:"bgp_peer,omitempty"` // IP роутера (пусто — шлюз LAN)
}

// DefaultRouter — значения по умолчанию.
func DefaultRouter() Router {
	return Router{Mode: RouterOff, URL: "https://192.168.253.1", User: "awg31", AddressList: "awg31",
		StaticLimit: 5000, DynamicMin: 60, DomainLimit: 1000, BGPLocalAS: 65031, BGPRemoteAS: 65030}
}

// RouterSettings возвращает действующие настройки интеграции.
func (s State) RouterSettings() Router {
	if s.Router == nil {
		return DefaultRouter()
	}
	return *s.Router
}

// State — содержимое state.json.
type State struct {
	User     string          `json:"user,omitempty"`
	PassHash string          `json:"pass_hash,omitempty"`
	Subs     map[string]Sub  `json:"subs,omitempty"`
	Up       map[string]bool `json:"up,omitempty"` // какие туннели поднимать при старте

	// Failover — настройки резервирования; nil — значения по умолчанию.
	Failover *Failover `json:"failover,omitempty"`
	// Priority — порядок туннелей для резервирования (первый — самый
	// приоритетный). Пусто — все профили по имени.
	Priority []string `json:"priority,omitempty"`
	// Primary — основной туннель, выбранный вручную или последним переключением.
	Primary string `json:"primary,omitempty"`
	// Policy — маршрутизация по правилам и DNS; nil — значения по умолчанию.
	Policy *Policy `json:"policy,omitempty"`
	// Router — интеграция с RouterOS; nil — выключена.
	Router *Router `json:"router,omitempty"`
	// Groups — группы туннелей (балансировка и резерв по порядку).
	Groups []Group `json:"groups,omitempty"`
	// Devices — справочник устройств LAN (правила по устройствам, мониторинг).
	Devices []Device `json:"devices,omitempty"`
}

// FailoverSettings возвращает действующие настройки резервирования.
func (s State) FailoverSettings() Failover {
	if s.Failover == nil {
		return DefaultFailover()
	}
	return *s.Failover
}

// Configured сообщает, задан ли пароль администратора.
func (s State) Configured() bool { return s.User != "" && s.PassHash != "" }

func (s State) clone() State {
	c := State{User: s.User, PassHash: s.PassHash, Subs: map[string]Sub{}, Up: map[string]bool{},
		Priority: append([]string(nil), s.Priority...), Primary: s.Primary, Groups: cloneGroups(s.Groups),
		Devices: append([]Device(nil), s.Devices...)}
	if s.Failover != nil {
		f := *s.Failover
		c.Failover = &f
	}
	if s.Policy != nil {
		p := *s.Policy
		c.Policy = &p
	}
	if s.Router != nil {
		r := *s.Router
		c.Router = &r
	}
	for k, v := range s.Subs {
		c.Subs[k] = v
	}
	for k, v := range s.Up {
		c.Up[k] = v
	}
	return c
}

// Store — хранилище в каталоге dir.
type Store struct {
	dir string
	mu  sync.RWMutex
	st  State
}

// Open открывает (и при необходимости создаёт) хранилище.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "profiles"), 0o700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, st: State{Subs: map[string]Sub{}, Up: map[string]bool{}}}
	raw, err := os.ReadFile(s.statePath())
	switch {
	case err == nil:
		if err := json.Unmarshal(raw, &s.st); err != nil {
			return nil, fmt.Errorf("state.json повреждён: %w", err)
		}
		if s.st.Subs == nil {
			s.st.Subs = map[string]Sub{}
		}
		if s.st.Up == nil {
			s.st.Up = map[string]bool{}
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return nil, err
	}
	return s, nil
}

func (s *Store) statePath() string { return filepath.Join(s.dir, "state.json") }

// Dir возвращает корневой каталог данных.
func (s *Store) Dir() string { return s.dir }

// Snapshot возвращает копию состояния.
func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.st.clone()
}

// Update применяет fn к копии состояния и сохраняет результат. При ошибке fn
// или записи состояние не меняется.
func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.st.clone()
	if err := fn(&next); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(s.statePath(), raw); err != nil {
		return err
	}
	s.st = next
	return nil
}

func (s *Store) profilePath(name string) string {
	return filepath.Join(s.dir, "profiles", name+".conf")
}

// ListProfiles возвращает имена профилей по алфавиту.
func (s *Store) ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, "profiles"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		n, ok := strings.CutSuffix(e.Name(), ".conf")
		if ok && e.Type().IsRegular() && ValidName(n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// ReadProfile возвращает текст профиля.
func (s *Store) ReadProfile(name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("недопустимое имя профиля (разрешено [a-zA-Z0-9_-], до %d символов)", MaxNameLen)
	}
	raw, err := os.ReadFile(s.profilePath(name))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNotFound
	}
	return string(raw), err
}

// WriteProfile проверяет конфиг и атомарно сохраняет его.
func (s *Store) WriteProfile(name, content string) error {
	if !ValidName(name) {
		return fmt.Errorf("недопустимое имя профиля (разрешено [a-zA-Z0-9_-], до %d символов)", MaxNameLen)
	}
	if _, err := conf.Parse(content); err != nil {
		return err
	}
	if _, ok := s.Snapshot().Group(name); ok {
		return fmt.Errorf("имя «%s» занято группой туннелей", name)
	}
	return writeFileAtomic(s.profilePath(name), []byte(content))
}

// DeleteProfile удаляет профиль и связанные с ним записи состояния.
func (s *Store) DeleteProfile(name string) error {
	if !ValidName(name) {
		return fmt.Errorf("недопустимое имя профиля")
	}
	if err := os.Remove(s.profilePath(name)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	return s.Update(func(st *State) error {
		delete(st.Subs, name)
		delete(st.Up, name)
		kept := st.Priority[:0]
		for _, n := range st.Priority {
			if n != name {
				kept = append(kept, n)
			}
		}
		st.Priority = kept
		if st.Primary == name {
			st.Primary = ""
		}
		for i := range st.Groups {
			kept := st.Groups[i].Members[:0:0]
			for _, m := range st.Groups[i].Members {
				if m.Name != name {
					kept = append(kept, m)
				}
			}
			st.Groups[i].Members = kept
		}
		return nil
	})
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
