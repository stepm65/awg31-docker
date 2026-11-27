package web

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"awg31/internal/dns"
	"awg31/internal/geo"
	"awg31/internal/policy"
	"awg31/internal/store"
)

type policyView struct {
	Settings store.Policy `json:"settings"`
	Stats    policy.Stats `json:"stats"`
	Geo      *geo.Status  `json:"geo,omitempty"`
	Gateway  bool         `json:"gateway"`
	DNS      bool         `json:"dns"`
	Errors   []string     `json:"errors,omitempty"`
}

func (s *Server) policyView() policyView {
	o := s.cfg.Tunnels.Options()
	v := policyView{Settings: s.cfg.Store.Snapshot().PolicySettings(), Gateway: o.AutoRoute && o.LanIface != "", DNS: s.cfg.DNSEnabled}
	if s.cfg.Policy != nil {
		v.Stats = s.cfg.Policy.Stats()
	}
	if s.cfg.Geo != nil {
		g := s.cfg.Geo.Status()
		v.Geo = &g
	}
	return v
}

// validatePolicy проверяет настройки и возвращает все ошибки сразу.
func validatePolicy(p store.Policy, allowHTTP bool) []string {
	var errs []string
	if p.DefaultAction != "proxy" && p.DefaultAction != "direct" {
		errs = append(errs, "действие по умолчанию: proxy или direct")
	}
	_, lineErrs := policy.Parse(p.Rules)
	for i, e := range lineErrs {
		if i == 20 {
			errs = append(errs, fmt.Sprintf("… и ещё %d ошибок в правилах", len(lineErrs)-20))
			break
		}
		errs = append(errs, e.Error())
	}
	if _, err := dns.ParseUpstreams(p.ProxyDNS); err != nil {
		errs = append(errs, "DNS через VPN: "+err.Error())
	}
	if _, err := dns.ParseUpstreams(p.DirectDNS); err != nil {
		errs = append(errs, "DNS мимо VPN: "+err.Error())
	}
	for _, u := range []string{p.GeoSiteURL, p.GeoIPURL} {
		if strings.TrimSpace(u) != "" {
			if err := geo.ValidURL(u, allowHTTP); err != nil {
				errs = append(errs, err.Error())
			}
		}
	}
	if p.GeoUpdateHours < 0 || p.GeoUpdateHours > 720 {
		errs = append(errs, "период обновления баз: 0–720 часов")
	}
	return errs
}

func (s *Server) handleGetPolicy(w http.ResponseWriter, _ *http.Request, _ string) {
	writeJSON(w, http.StatusOK, s.policyView())
}

func (s *Server) handlePutPolicy(w http.ResponseWriter, r *http.Request, _ string) {
	var p store.Policy
	if !decode(w, r, &p, policy.MaxRulesBytes+16<<10) {
		return
	}
	p.DefaultAction = strings.ToLower(strings.TrimSpace(p.DefaultAction))
	if errs := validatePolicy(p, s.cfg.AllowHTTPSubs); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": errs[0], "errors": errs})
		return
	}
	if p.Enabled {
		if o := s.cfg.Tunnels.Options(); !o.AutoRoute || o.LanIface == "" {
			writeErr(w, http.StatusConflict, "маршрутизация по правилам работает только в режиме шлюза (AWG_AUTO_ROUTE=on и найден LAN-интерфейс)")
			return
		}
	}
	if err := s.cfg.Store.Update(func(st *store.State) error { st.Policy = &p; return nil }); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Включили правила с geosite/geoip, а баз ещё нет (свежий контейнер) —
	// скачиваем сразу, не дожидаясь фонового обновления.
	var fetchErr error
	if p.Enabled {
		fetchErr = s.fetchMissingGeo(r, p)
	}
	var reloadErr error
	if s.cfg.Policy != nil {
		reloadErr = s.cfg.Policy.Reload()
	}
	v := s.policyView()
	switch {
	case fetchErr != nil:
		v.Errors = append(v.Errors, "не удалось скачать базы geo: "+fetchErr.Error()+" — проверьте доступ контейнера в интернет и повторите кнопкой «Обновить базы»")
	case reloadErr != nil:
		v.Errors = append(v.Errors, "базы geo: "+reloadErr.Error())
	}
	s.cfg.Logf("policy: %s, default %s, rules %d", map[bool]string{true: "enabled", false: "disabled"}[p.Enabled], p.DefaultAction, v.Stats.Rules)
	writeJSON(w, http.StatusOK, v)
}

// fetchMissingGeo скачивает те базы geo, которые нужны правилам, но ещё не лежат
// на диске. Уже имеющиеся файлы не трогает (их обновляет geoUpdater и кнопка).
func (s *Server) fetchMissingGeo(r *http.Request, p store.Policy) error {
	if s.cfg.Geo == nil {
		return nil
	}
	rules, _ := policy.Parse(p.Rules)
	sites, ips := policy.Categories(rules)
	gs := s.cfg.Geo.Status()
	siteURL, ipURL := "", ""
	if len(sites) > 0 && !gs.Site.Present {
		siteURL = p.GeoSiteURL
	}
	if len(ips) > 0 && !gs.IP.Present {
		ipURL = p.GeoIPURL
	}
	if siteURL == "" && ipURL == "" {
		return nil
	}
	ctx, cancel := timeout(r, 3*time.Minute)
	defer cancel()
	if err := s.cfg.Geo.Download(ctx, siteURL, ipURL, s.cfg.AllowHTTPSubs); err != nil {
		s.cfg.Logf("geo databases: %v", err)
		return err
	}
	s.cfg.Logf("geo databases downloaded")
	return nil
}

// handleGeoIcon отдаёт значок сайта из списка IPList. Браузер панели наружу не ходит (CSP), поэтому
// значок берёт и кеширует контейнер. Подходят только сайты из загруженной базы и только когда
// источник баз — сервер вида IPList; для остальных источников значков нет (404).
func (s *Server) handleGeoIcon(w http.ResponseWriter, r *http.Request, _ string) {
	site := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("site")))
	if s.cfg.Geo == nil || !geo.IconSite(site) {
		writeErr(w, http.StatusNotFound, "значка нет")
		return
	}
	base := geo.IconBase(s.cfg.Store.Snapshot().PolicySettings().GeoSiteURL, s.cfg.AllowHTTPSubs)
	cats := s.cfg.Geo.Categories("geosite")
	if i := sort.SearchStrings(cats, site); base == "" || i >= len(cats) || cats[i] != site {
		writeErr(w, http.StatusNotFound, "значка нет")
		return
	}
	ctx, cancel := timeout(r, 25*time.Second)
	defer cancel()
	data, ct, err := s.cfg.Geo.Icon(ctx, base, site, s.cfg.AllowHTTPSubs)
	if errors.Is(err, geo.ErrNoIcon) {
		writeErr(w, http.StatusNotFound, "значка нет")
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	_, _ = w.Write(data)
}

func (s *Server) handleTestPolicy(w http.ResponseWriter, r *http.Request, _ string) {
	var body struct {
		Query string `json:"query"`
	}
	if !decode(w, r, &body, 1024) {
		return
	}
	q := strings.TrimSpace(body.Query)
	if q == "" || len(q) > 253 || s.cfg.Policy == nil {
		writeErr(w, http.StatusBadRequest, "укажите домен или IP-адрес")
		return
	}
	writeJSON(w, http.StatusOK, s.cfg.Policy.Test(q))
}

func (s *Server) handleGeoUpdate(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Geo == nil {
		writeErr(w, http.StatusNotImplemented, "базы geo не настроены")
		return
	}
	ctx, cancel := timeout(r, 5*time.Minute)
	defer cancel()
	p := s.cfg.Store.Snapshot().PolicySettings()
	if err := s.cfg.Geo.Download(ctx, p.GeoSiteURL, p.GeoIPURL, s.cfg.AllowHTTPSubs); err != nil {
		s.cfg.Logf("geo databases: %v", err)
		writeErr(w, http.StatusBadGateway, "базы geo: "+err.Error())
		return
	}
	var reloadErr error
	if s.cfg.Policy != nil {
		reloadErr = s.cfg.Policy.Reload()
	}
	v := s.policyView()
	if reloadErr != nil {
		v.Errors = append(v.Errors, reloadErr.Error())
	}
	s.cfg.Logf("geo databases updated")
	writeJSON(w, http.StatusOK, v)
}

// handleGeoCategories ищет категории в загруженных базах: нужен редактору правил, чтобы
// подсказать имена geosite/geoip вместо угадывания. Сначала совпадения с начала имени.
func (s *Server) handleGeoCategories(w http.ResponseWriter, r *http.Request, _ string) {
	if s.cfg.Geo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []string{}})
		return
	}
	kind := r.URL.Query().Get("kind")
	if kind != "geosite" && kind != "geoip" {
		writeErr(w, http.StatusBadRequest, "kind: geosite или geoip")
		return
	}
	// all=1 — весь список категорий загруженной базы: из него редактор строит каталог IPList.
	if r.URL.Query().Get("all") == "1" {
		all := s.cfg.Geo.Categories(kind)
		if len(all) > 5000 {
			all = all[:5000]
		}
		if all == nil {
			all = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "items": all})
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v < limit {
		limit = v
	}
	items := []string{}
	if len(q) >= 2 && len(q) <= 64 {
		var prefix, inner []string
		for _, c := range s.cfg.Geo.Categories(kind) {
			switch {
			case strings.HasPrefix(c, q):
				prefix = append(prefix, c)
			case strings.Contains(c, q):
				inner = append(inner, c)
			}
		}
		items = append(append(items, prefix...), inner...) // не nil: в JSON должен быть [], не null
		if len(items) > limit {
			items = items[:limit]
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"kind": kind, "items": items})
}
