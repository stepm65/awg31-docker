// Команда awgd — управляющий демон AmneziaWG 3.1 в контейнере: WebUI, профили,
// подписки и пробы здоровья.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"awg31/internal/auth"
	"awg31/internal/conf"
	"awg31/internal/dns"
	"awg31/internal/failover"
	"awg31/internal/geo"
	"awg31/internal/group"
	"awg31/internal/monitor"
	"awg31/internal/policy"
	"awg31/internal/ros"
	"awg31/internal/store"
	"awg31/internal/subsync"
	"awg31/internal/tlsutil"
	"awg31/internal/tunnel"
	"awg31/internal/version"
	"awg31/internal/web"
)

func main() {
	// Журнал контейнера RouterOS и терминалы не показывают UTF-8: см. asciiWriter.
	log.SetOutput(asciiWriter{os.Stderr})
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(asciiWriter{os.Stderr}, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		log.Printf("ERROR: %v", err)
		os.Exit(1)
	}
}

// healthURL — адрес /healthz на loopback с тем же портом и схемой, что у WebUI.
func healthURL() string {
	port := "8443"
	if _, p, err := net.SplitHostPort(env("UI_LISTEN", ":8443")); err == nil && p != "" {
		port = p
	}
	scheme := "https"
	if strings.ToLower(env("UI_TLS", "auto")) == "off" {
		scheme = "http"
	}
	return scheme + "://127.0.0.1:" + port + "/healthz"
}

// healthcheck — для HEALTHCHECK в Dockerfile: не требует wget и принимает
// самоподписанный сертификат, потому что ходит только на 127.0.0.1.
func healthcheck() error {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // только loopback
		},
	}
	resp, err := client.Get(healthURL())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz: HTTP %d", resp.StatusCode)
	}
	return nil
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch strings.ToLower(env(key, "")) {
	case "1", "on", "true", "yes":
		return true
	case "0", "off", "false", "no":
		return false
	}
	return def
}

// parsePrefixes разбирает список «10.0.0.0/8, 192.168.1.1»; одиночный адрес — это /32 (/128).
func parsePrefixes(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if p, err := netip.ParsePrefix(item); err == nil {
			out = append(out, p)
			continue
		}
		a, err := netip.ParseAddr(item)
		if err != nil {
			return nil, fmt.Errorf("%q: ожидался IP или CIDR", item)
		}
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	return out, nil
}

// adminPassword читает пароль из ADMIN_PASSWORD_FILE (предпочтительно, docker secrets) или ADMIN_PASSWORD.
func adminPassword() (string, error) {
	if path := env("ADMIN_PASSWORD_FILE", ""); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("ADMIN_PASSWORD_FILE: %w", err)
		}
		return strings.TrimRight(string(raw), "\r\n"), nil
	}
	return os.Getenv("ADMIN_PASSWORD"), nil
}

func run() error {
	logf := log.New(asciiWriter{os.Stdout}, "", log.LstdFlags).Printf
	logf("awg31 %s", version.Version)

	dataDir := env("DATA_DIR", "/data")
	st, err := store.Open(dataDir)
	if err != nil {
		return err
	}

	// Пароль администратора из окружения: один раз при первом запуске.
	if !st.Snapshot().Configured() {
		pw, err := adminPassword()
		if err != nil {
			return err
		}
		if pw != "" {
			user := env("ADMIN_USER", "admin")
			if err := auth.ValidatePassword(pw); err != nil {
				return fmt.Errorf("ADMIN_PASSWORD: %w", err)
			}
			hash, err := auth.HashPassword(pw)
			if err != nil {
				return err
			}
			if err := st.Update(func(s *store.State) error { s.User, s.PassHash = user, hash; return nil }); err != nil {
				return err
			}
			logf("administrator password for %q is set from the environment; change it only through the WebUI from now on", user)
		}
	}

	runner := tunnel.ExecRunner{Timeout: 45 * time.Second}
	topts, err := networkOptions(context.Background(), runner, logf)
	if err != nil {
		return err
	}
	tm := tunnel.New(runner, topts)
	if err := tm.EnsureRouting(context.Background()); err != nil {
		logf("WARNING: %v", err)
	}
	subs := &subsync.Service{Store: st, Tunnels: tm, AllowHTTP: envBool("ALLOW_INSECURE_SUBSCRIPTIONS", false), Logf: logf}
	if subs.AllowHTTP {
		logf("WARNING: ALLOW_INSECURE_SUBSCRIPTIONS=on: subscriptions over plain HTTP are allowed")
	}

	proxies, err := parsePrefixes(env("TRUSTED_PROXIES", ""))
	if err != nil {
		return fmt.Errorf("TRUSTED_PROXIES: %w", err)
	}

	var setupToken string
	if !st.Snapshot().Configured() {
		if setupToken, err = auth.NewToken(); err != nil {
			return err
		}
		logf("================ FIRST-TIME SETUP ================")
		logf("Open the WebUI and enter the setup token (SETUP TOKEN): %s", setupToken)
		logf("The token is single-use and valid until the container restarts.")
		logf("=====================================================")
	}

	tlsMode := strings.ToLower(env("UI_TLS", "auto"))
	gateway := topts.AutoRoute && topts.LanIface != ""
	fo := &failover.Monitor{Store: st, Tunnels: tm, Gateway: gateway, Logf: logf, Exclude: func(name string) bool {
		raw, err := st.ReadProfile(name)
		if err != nil {
			return false
		}
		cfg, err := conf.Parse(raw)
		return err == nil && cfg.IsServer() // сервер не бывает основным туннелем
	}}
	grp := &group.Manager{Store: st, Tunnels: tm, Runner: runner, Logf: logf}
	tm.SetGroups(grp)
	router := group.Router{Manager: tm, G: grp} // туннели и группы под одним именем для политики и DNS
	fo.OnHealth = grp.SetHealth
	fo.Keep = func(name string) bool { return grp.Members()[name] }
	fo.IsGroup = grp.Has
	geoStore := &geo.Store{Dir: filepath.Join(dataDir, "geo")}
	pol := &policy.Engine{Store: st, Geo: geoStore, Tunnels: router, Runner: runner, Dir: topts.RunDir, Logf: logf}
	if err := pol.Reload(); err != nil {
		logf("policy: geo databases: %v", err)
	}
	tm.OnChange(pol.Invalidate) // PROXY@туннель зависит от того, какие туннели подняты, а ядро убирает маршруты исчезнувшего интерфейса
	tm.OnChange(grp.TunnelsChanged)
	grp.OnChange(pol.MarkDirty)
	rosMgr := &ros.Manager{Store: st, Policy: pol, Tunnels: tm, Runner: runner, Logf: logf}
	var dnsSrv *dns.Server
	if gateway && envBool("DNS_ENABLE", true) {
		allow, err := parsePrefixes(env("DNS_ALLOW", ""))
		if err != nil {
			return fmt.Errorf("DNS_ALLOW: %w", err)
		}
		dnsSrv = &dns.Server{Listen: env("DNS_LISTEN", ":53"), Store: st, Policy: pol, Tunnels: router, Allow: allow, Logf: logf}
	}
	if err := applyFailoverEnv(st); err != nil {
		return err
	}

	mon := &monitor.Monitor{Store: st, Policy: pol, Tunnels: tm, Runner: runner, Logf: logf,
		Ifaces: func() []string { n, _ := st.ListProfiles(); return n }}

	srv := web.New(web.Config{
		Store: st, Tunnels: tm, Subs: subs, Failover: fo, Groups: grp, Policy: pol, Geo: geoStore, Router: rosMgr, Monitor: mon, DNSEnabled: dnsSrv != nil,
		Autostart:      func(ctx context.Context) { autostart(ctx, st, tm, logf) },
		Sessions:       auth.NewSessions(24*time.Hour, 64),
		Limiter:        auth.NewLimiter(5, 2*time.Minute, time.Minute, 4096),
		SetupToken:     setupToken,
		TLS:            tlsMode != "off",
		TrustedProxies: proxies,
		AllowHTTPSubs:  subs.AllowHTTP,
		Logf:           logf,
	})

	httpSrv := &http.Server{
		Addr:              env("UI_LISTEN", ":8443"),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}

	var certFile, keyFile string
	switch tlsMode {
	case "auto":
		sans := tlsutil.DefaultSANs(strings.Split(env("UI_TLS_SANS", ""), ",")...)
		cert, fp, err := tlsutil.LoadOrCreate(filepath.Join(dataDir, "tls"), sans)
		if err != nil {
			return fmt.Errorf("сертификат WebUI: %w", err)
		}
		httpSrv.TLSConfig = tlsutil.ServerConfig(cert)
		logf("WebUI over HTTPS: %s", tlsutil.Describe(fp))
	case "files":
		certFile, keyFile = env("UI_TLS_CERT", ""), env("UI_TLS_KEY", "")
		if certFile == "" || keyFile == "" {
			return errors.New("UI_TLS=files требует UI_TLS_CERT и UI_TLS_KEY")
		}
	case "off":
		logf("WARNING: UI_TLS=off: password and cookie travel in clear text; acceptable only behind a trusted TLS proxy")
	default:
		return fmt.Errorf("UI_TLS: неизвестный режим %q (auto|files|off)", tlsMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 2)
	go func() {
		logf("WebUI listening on %s (TLS: %s)", httpSrv.Addr, tlsMode)
		var err error
		if tlsMode == "off" {
			err = httpSrv.ListenAndServe()
		} else {
			err = httpSrv.ListenAndServeTLS(certFile, keyFile)
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var healthSrv *http.Server
	if addr := env("HEALTH_LISTEN", ""); addr != "" {
		maxHS, _ := strconv.Atoi(env("HEALTH_MAX_HANDSHAKE_SEC", "0"))
		healthSrv = &http.Server{
			Addr:              addr,
			Handler:           web.HealthHandler(tm, time.Duration(maxHS)*time.Second),
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      10 * time.Second,
		}
		go func() {
			logf("health probe listening on %s (status code only, no body)", addr)
			if err := healthSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	go subs.Run(ctx)
	go pol.Run(ctx)
	go mon.Run(ctx)
	go rosMgr.Run(ctx)
	go geoUpdater(ctx, st, geoStore, pol, logf)
	if dnsSrv != nil {
		go func() {
			logf("DNS forwarder: %s (UDP/TCP)", dnsSrv.Listen)
			if err := dnsSrv.ListenAndServe(ctx); err != nil {
				logf("DNS forwarder not started: %v (needs NET_BIND_SERVICE for port 53, or DNS_LISTEN with a port above 1024)", err)
			}
		}()
	}
	go func() {
		autostart(ctx, st, tm, logf)
		fo.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		logf("shutdown signal received")
	case err := <-errCh:
		stop()
		logf("server error: %v", err)
		shutdown(httpSrv, healthSrv, st, tm, logf)
		return err
	}
	shutdown(httpSrv, healthSrv, st, tm, logf)
	return nil
}

// autostart поднимает туннели, которые были подняты до перезапуска контейнера.
func autostart(ctx context.Context, st *store.Store, tm *tunnel.Manager, logf func(string, ...any)) {
	snap := st.Snapshot()
	profiles, _ := st.ListProfiles()
	// порядок: сначала список приоритета, потом остальные по имени — так первый
	// поднятый (и ставший основным) туннель предсказуем
	names := failover.Order(snap.Priority, profiles)
	for _, n := range failover.Order(nil, profiles) {
		if !slicesContains(names, n) {
			names = append(names, n)
		}
	}
	for _, name := range names {
		if !snap.Up[name] || ctx.Err() != nil {
			continue
		}
		text, err := st.ReadProfile(name)
		if err != nil {
			logf("autostart %s: %v", name, err)
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		err = tm.Up(cctx, name, text)
		cancel()
		if err != nil && !errors.Is(err, tunnel.ErrAlreadyUp) {
			logf("autostart %s: %v", name, err)
			continue
		}
		logf("autostart: tunnel %s is up", name)
	}
	if p := snap.Primary; p != "" && tm.Primary() != p && (tm.Table(p) != 0 || snap.HasGroup(p)) {
		if err := tm.SetPrimary(ctx, p); err != nil {
			logf("autostart: primary tunnel %s: %v", p, err)
		}
	}
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// applyFailoverEnv задаёт настройки резервирования из окружения при первом
// запуске (AWG_FAILOVER_MODE и др.); дальше они меняются в WebUI.
func applyFailoverEnv(st *store.Store) error {
	if st.Snapshot().Failover != nil || env("AWG_FAILOVER_MODE", "") == "" {
		return nil
	}
	f := store.DefaultFailover()
	f.Mode = env("AWG_FAILOVER_MODE", f.Mode)
	f.Trigger = env("AWG_FAILOVER_TRIGGER", f.Trigger)
	f.ProbeTarget = env("AWG_FAILOVER_PROBE", f.ProbeTarget)
	f.Failback = envBool("AWG_FAILOVER_FAILBACK", false)
	if err := f.Validate(); err != nil {
		return fmt.Errorf("AWG_FAILOVER_*: %w", err)
	}
	return st.Update(func(s *store.State) error { s.Failover = &f; return nil })
}

// shutdown останавливает серверы и опускает туннели, не меняя сохранённое
// состояние «поднят»: при следующем запуске они поднимутся снова.
func shutdown(web, health *http.Server, st *store.Store, tm *tunnel.Manager, logf func(string, ...any)) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = web.Shutdown(ctx)
	if health != nil {
		_ = health.Shutdown(ctx)
	}
	ifaces, _ := tm.Status(ctx)
	for _, i := range ifaces {
		name := i.Name
		if err := tm.Down(ctx, name); err != nil && !errors.Is(err, tunnel.ErrNotUp) {
			logf("stopping %s: %v", name, err)
		}
	}
	logf("stopped")
}

// networkOptions собирает настройки режима шлюза из окружения:
// AWG_AUTO_ROUTE (on), AWG_LAN_IFACE (по маршруту по умолчанию),
// AWG_KILLSWITCH (on), AWG_ENABLE_IPV6 (on, если IPv6 есть в контейнере).
func networkOptions(ctx context.Context, r tunnel.Runner, logf func(string, ...any)) (tunnel.Options, error) {
	o := tunnel.Options{
		RunDir:    env("RUN_DIR", "/run/awg31"),
		AutoRoute: envBool("AWG_AUTO_ROUTE", true),
		Awg:       env("AWG_BIN", ""),       // по умолчанию /usr/bin/awg
		AwgQuick:  env("AWG_QUICK_BIN", ""), // по умолчанию /usr/bin/awg-quick
		Logf:      logf,
	}
	if !o.AutoRoute {
		logf("gateway mode is off (AWG_AUTO_ROUTE=off): routes as in plain awg-quick")
		return o, nil
	}
	o.KillSwitch = envBool("AWG_KILLSWITCH", true)
	if envBool("AWG_ENABLE_IPV6", true) {
		if o.IPv6 = tunnel.IPv6Available(); !o.IPv6 {
			logf("IPv6 is unavailable in the container (disable_ipv6=1): the tunnel works over IPv4 only")
		}
	}
	if v := env("AWG_LAN_IFACE", ""); v != "" {
		if !tunnel.ValidIfaceName(v) {
			return o, fmt.Errorf("AWG_LAN_IFACE: недопустимое имя интерфейса %q", v)
		}
		o.LanIface = v
	} else if v, err := tunnel.DetectLanIface(ctx, r); err == nil {
		o.LanIface = v
	} else {
		logf("LAN interface not found (%v): NAT and MSS clamp only, no LAN routing and no kill switch", err)
	}
	logf("gateway mode: LAN=%s, kill switch=%v, IPv6=%v, table %d", orDash(o.LanIface), o.KillSwitch, o.IPv6, tunnel.RouteTable)
	return o, nil
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// geoUpdater раз в час проверяет, не пора ли обновить базы geo (по
// GeoUpdateHours) и нужны ли они вообще правилам; при отсутствии файлов
// скачивает их сразу.
func geoUpdater(ctx context.Context, st *store.Store, g *geo.Store, pol *policy.Engine, logf func(string, ...any)) {
	check := func() {
		p := st.Snapshot().PolicySettings()
		// дополнительные наборы: недостающие — сразу, остальные — по периоду обновления
		if len(p.GeoExtra) > 0 && p.Enabled {
			g.SetExtra(policy.GeoExtra(p))
			stale := false
			for _, e := range g.ExtraStatus() {
				if !e.File.Present || (p.GeoUpdateHours > 0 && time.Since(e.File.Updated) > time.Duration(p.GeoUpdateHours)*time.Hour) {
					stale = true
				}
			}
			if stale {
				dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
				if err := g.DownloadExtra(dctx, false, false); err != nil {
					logf("geo extra sets: %v", err)
				} else if err := pol.Reload(); err != nil {
					logf("policy: geo databases: %v", err)
				}
				cancel()
			}
		}
		rules, _ := policy.Parse(p.Rules)
		sites, ips := policy.Categories(rules)
		if !p.Enabled || len(sites)+len(ips) == 0 {
			return
		}
		gs := g.Status()
		stale := func(f geo.FileStatus, need bool) bool {
			if !need {
				return false
			}
			if !f.Present {
				return true
			}
			return p.GeoUpdateHours > 0 && time.Since(f.Updated) > time.Duration(p.GeoUpdateHours)*time.Hour
		}
		siteURL, ipURL := "", ""
		if stale(gs.Site, len(sites) > 0) {
			siteURL = p.GeoSiteURL
		}
		if stale(gs.IP, len(ips) > 0) {
			ipURL = p.GeoIPURL
		}
		if siteURL == "" && ipURL == "" {
			return
		}
		dctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		if err := g.Download(dctx, siteURL, ipURL, false); err != nil {
			logf("geo databases: %v", err)
			return
		}
		logf("geo databases updated")
		if err := pol.Reload(); err != nil {
			logf("policy: geo databases: %v", err)
		}
	}
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			check()
			timer.Reset(time.Hour)
		}
	}
}
