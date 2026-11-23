// Package tunnel поднимает и опускает туннели через awg-quick и читает статус из awg.
//
// Команды запускаются напрямую (без shell), с минимальным окружением. Хуки для
// awg-quick генерирует только этот пакет и только из констант, поэтому внедрить
// команду через конфиг или имя профиля нельзя.
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"awg31/internal/conf"
)

var (
	// ErrAlreadyUp — туннель уже поднят.
	ErrAlreadyUp = errors.New("туннель уже поднят")
	// ErrNotUp — туннель не поднят.
	ErrNotUp = errors.New("туннель не поднят")

	reSecret = regexp.MustCompile(`[A-Za-z0-9+/]{43}=`)
)

// Runner запускает внешние команды (подменяется в тестах).
type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// ExecRunner — реальный запуск процессов.
type ExecRunner struct {
	Timeout time.Duration
	// WaitDelay — сколько ждать закрытия каналов вывода после выхода команды
	// (по умолчанию 3 с). awg-quick запускает демон amneziawg-go, и тот может
	// унаследовать каналы вывода: без этого предела Run не вернулся бы, пока жив демон.
	WaitDelay time.Duration
}

// maxRunOutput — сколько вывода команды храним.
const maxRunOutput = 256 << 10

// syncBuf — буфер вывода, безопасный для записи из горутины, которую остановил WaitDelay.
type syncBuf struct {
	mu sync.Mutex
	b  []byte
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if room := maxRunOutput - len(s.b); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		s.b = append(s.b, p[:room]...)
	}
	return len(p), nil
}

func (s *syncBuf) bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.b...)
}

// Run выполняет команду без shell и возвращает её вывод.
func (r ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	// LOG_LEVEL не задаём: при нём демон amneziawg-go, запущенный awg-quick, наследует
	// наши каналы stdout/stderr и держит их открытыми, пока работает туннель.
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go",
	}
	cmd.WaitDelay = r.WaitDelay
	if cmd.WaitDelay <= 0 {
		cmd.WaitDelay = 3 * time.Second
	}
	var buf syncBuf
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		err = nil // команда завершилась успешно, каналы держит оставшийся в фоне демон
	}
	out := buf.bytes()
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, firstArg(args), err, Redact(string(out)))
	}
	return out, nil
}

func firstArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// Redact скрывает в тексте всё, похожее на ключ WireGuard, и ограничивает длину.
func Redact(s string) string {
	s = reSecret.ReplaceAllString(s, "[ключ скрыт]")
	s = strings.TrimSpace(s)
	if len(s) > 2048 {
		s = s[:2048] + "…"
	}
	return s
}

// Options — настройки Manager.
type Options struct {
	RunDir    string // каталог для рабочих копий конфигов (tmpfs)
	AutoRoute bool   // режим шлюза: NAT, MSS-clamp и маршрутизация трафика LAN
	AwgQuick  string // путь к awg-quick
	Awg       string // путь к awg

	// LanIface — интерфейс, через который приходит трафик LAN (veth/eth0).
	// Пусто — только NAT и MSS-clamp, маршруты как у обычного awg-quick.
	LanIface   string
	KillSwitch bool // трафик LAN без туннеля отбрасывается (см. routing.go)
	IPv6       bool // IPv6 в туннеле: NAT66, правила и маршруты -6
	Logf       func(format string, args ...any)
}

// Manager управляет туннелями.
type Manager struct {
	r    Runner
	opts Options
	mu   sync.Mutex

	routingErr string         // последняя ошибка EnsureRouting (для UI)
	onChange   []func()       // вызываются после подъёма и остановки туннеля
	tables     map[string]int // собственные таблицы поднятых туннелей (режим шлюза)
	primary    string         // основной туннель или группа (режим шлюза)
	groups     GroupRouter    // группы туннелей (nil — нет)
}

// OnChange регистрирует функцию, вызываемую после подъёма или остановки
// туннеля (вне блокировки Manager).
func (m *Manager) OnChange(fn func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onChange = append(m.onChange, fn)
}

func (m *Manager) changed() {
	m.mu.Lock()
	fns := append([]func(){}, m.onChange...)
	m.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// RoutingError возвращает ошибку настройки маршрутизации шлюза или "".
func (m *Manager) RoutingError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.routingErr
}

// New создаёт Manager.
func New(r Runner, o Options) *Manager {
	if o.AwgQuick == "" {
		o.AwgQuick = "/usr/bin/awg-quick"
	}
	if o.Awg == "" {
		o.Awg = "/usr/bin/awg"
	}
	if o.RunDir == "" {
		o.RunDir = "/run/awg31"
	}
	return &Manager{r: r, opts: o, tables: map[string]int{}}
}

func (m *Manager) logf(format string, args ...any) {
	if m.opts.Logf != nil {
		m.opts.Logf(format, args...)
	}
}

// Options возвращает действующие настройки (для UI).
func (m *Manager) Options() Options { return m.opts }

func (m *Manager) runPath(name string) string {
	return filepath.Join(m.opts.RunDir, name+".conf")
}

// Up поднимает туннель по тексту конфига.
func (m *Manager) Up(ctx context.Context, name, content string) error {
	err := m.up(ctx, name, content)
	if err == nil {
		m.changed()
	}
	return err
}

func (m *Manager) up(ctx context.Context, name, content string) error {
	cfg, err := conf.Parse(content)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	ifaces, err := m.statusLocked(ctx)
	if err != nil {
		return err
	}
	for _, i := range ifaces {
		if i.Name == name {
			return ErrAlreadyUp
		}
	}
	table := 0
	server := cfg.IsServer()
	if m.gateway() && !server {
		if table, err = m.allocTableLocked(name); err != nil {
			return err
		}
	}
	release := func() {
		if table > 0 {
			delete(m.tables, name)
		}
	}
	run, notes := m.prepare(cfg, table)
	if run.Interface.First("Address") == "" {
		release()
		return fmt.Errorf("в профиле не осталось адресов: только IPv6, а IPv6 в контейнере выключен")
	}
	for _, n := range notes {
		m.logf("tunnel %s: %s", name, n)
	}
	var hooks conf.Hooks
	if m.opts.AutoRoute {
		hooks = RouteHooks(m.opts, table)
		if server && m.gateway() {
			// клиенты сервера выходят в интернет через LAN (роутер): подмена адреса
			h := "iptables -t nat -C POSTROUTING -o " + m.opts.LanIface + " -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o " + m.opts.LanIface + " -j MASQUERADE"
			hooks.PostUp = append(hooks.PostUp, h)
		}
	}
	if err := os.MkdirAll(m.opts.RunDir, 0o700); err != nil {
		release()
		return err
	}
	path := m.runPath(name)
	if err := os.WriteFile(path, []byte(run.Render(hooks)), 0o600); err != nil {
		release()
		return err
	}
	if _, err := m.r.Run(ctx, m.opts.AwgQuick, "up", path); err != nil {
		_ = os.Remove(path)
		release()
		return err
	}
	if m.gateway() && m.primary == "" && !server {
		// первый поднятый туннель становится основным
		if err := m.setPrimaryLocked(ctx, name); err != nil {
			m.logf("tunnel %s is up, but LAN traffic is not routed into it: %v", name, err)
		}
	}
	return nil
}

// Down опускает туннель.
func (m *Manager) Down(ctx context.Context, name string) error {
	err := m.down(ctx, name)
	if !errors.Is(err, ErrNotUp) {
		m.changed()
	}
	return err
}

func (m *Manager) down(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := m.runPath(name)
	if _, err := os.Stat(path); err != nil {
		return ErrNotUp
	}
	_, err := m.r.Run(ctx, m.opts.AwgQuick, "down", path)
	if rmErr := os.Remove(path); err == nil && rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		err = rmErr
	}
	delete(m.tables, name)
	if m.primary == name {
		// Маршрут в RouteTable исчез вместе с интерфейсом. Если работает другой
		// туннель, трафик LAN переводится на него (с наименьшей таблицей —
		// поднятый раньше); монитор резервирования поправит выбор по приоритету.
		m.primary = ""
		next, best := "", TunnelTableMax+1
		for n, t := range m.tables {
			if t < best {
				next, best = n, t
			}
		}
		if next != "" {
			if perr := m.setPrimaryLocked(ctx, next); perr != nil {
				m.logf("switching to tunnel %s failed: %v", next, perr)
			}
		} else if m.gateway() && !m.opts.KillSwitch {
			m.lanRuleLocked(ctx, false)
		}
	}
	return err
}

// Peer — пир туннеля (без приватных данных: PSK и приватный ключ не читаются).
type Peer struct {
	PublicKey       string   `json:"public_key"`
	Endpoint        string   `json:"endpoint"`
	AllowedIPs      []string `json:"allowed_ips"`
	LatestHandshake int64    `json:"latest_handshake"` // unix-время, 0 — рукопожатия не было
	RxBytes         uint64   `json:"rx_bytes"`
	TxBytes         uint64   `json:"tx_bytes"`
}

// Iface — состояние интерфейса.
type Iface struct {
	Name       string `json:"name"`
	ListenPort int    `json:"listen_port"`
	Peers      []Peer `json:"peers"`
}

// Status возвращает все поднятые интерфейсы awg.
func (m *Manager) Status(ctx context.Context) ([]Iface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked(ctx)
}

func (m *Manager) statusLocked(ctx context.Context) ([]Iface, error) {
	out, err := m.r.Run(ctx, m.opts.Awg, "show", "interfaces")
	if err != nil {
		return nil, err
	}
	var res []Iface
	for _, name := range strings.Fields(string(out)) {
		dump, err := m.r.Run(ctx, m.opts.Awg, "show", name, "dump")
		if err != nil {
			return nil, err
		}
		iface, err := ParseDump(name, string(dump))
		if err != nil {
			return nil, err
		}
		res = append(res, iface)
	}
	return res, nil
}

// ParseDump разбирает вывод `awg show <iface> dump`: первая строка — интерфейс
// (у AmneziaWG в ней больше полей, чем у WireGuard), остальные — пиры по 8 полей.
func ParseDump(name, dump string) (Iface, error) {
	lines := strings.Split(strings.TrimSpace(dump), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return Iface{}, fmt.Errorf("пустой вывод awg show для %s", name)
	}
	head := strings.Split(lines[0], "\t")
	if len(head) < 3 {
		return Iface{}, fmt.Errorf("неожиданная строка интерфейса в выводе awg show")
	}
	iface := Iface{Name: name}
	iface.ListenPort, _ = strconv.Atoi(head[2])
	for _, line := range lines[1:] {
		f := strings.Split(line, "\t")
		if len(f) < 8 {
			continue
		}
		p := Peer{PublicKey: f[0]}
		if f[2] != "(none)" {
			p.Endpoint = f[2]
		}
		if f[3] != "(none)" {
			p.AllowedIPs = strings.Split(f[3], ",")
		}
		p.LatestHandshake, _ = strconv.ParseInt(f[4], 10, 64)
		p.RxBytes, _ = strconv.ParseUint(f[5], 10, 64)
		p.TxBytes, _ = strconv.ParseUint(f[6], 10, 64)
		iface.Peers = append(iface.Peers, p)
	}
	return iface, nil
}
