// Package geo читает базы v2ray geosite.dat и geoip.dat (protobuf) или
// простые текстовые списки и хранит только нужные категории — на роутерах с
// небольшой памятью вся база (десятки мегабайт) не загружается целиком.
package geo

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Виды доменных правил geosite (enum Domain.Type в v2ray).
const (
	kindKeyword = 0 // Plain: подстрока
	kindRegex   = 1
	kindDomain  = 2 // RootDomain: домен и поддомены
	kindFull    = 3
)

// Site — категория доменов.
type Site struct {
	full     map[string]bool
	suffixes map[string]bool
	keywords []string
	regexps  []*regexp.Regexp
	Count    int
}

// Match проверяет имя хоста (нижний регистр, без точки в конце).
func (s *Site) Match(host string) bool {
	if s == nil || host == "" {
		return false
	}
	if s.full[host] || s.suffixes[host] {
		return true
	}
	for h := host; ; {
		i := strings.IndexByte(h, '.')
		if i < 0 {
			break
		}
		h = h[i+1:]
		if s.suffixes[h] {
			return true
		}
	}
	for _, k := range s.keywords {
		if strings.Contains(host, k) {
			return true
		}
	}
	for _, re := range s.regexps {
		if re.MatchString(host) {
			return true
		}
	}
	return false
}

func (s *Site) add(kind int, v string) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return
	}
	switch kind {
	case kindFull:
		s.full[v] = true
	case kindDomain:
		s.suffixes[strings.TrimPrefix(v, ".")] = true
	case kindKeyword:
		s.keywords = append(s.keywords, v)
	case kindRegex:
		// регулярные выражения Go (RE2) работают за линейное время; то, что RE2
		// не понимает, пропускается
		re, err := regexp.Compile(v)
		if err != nil || len(v) > 512 {
			return
		}
		s.regexps = append(s.regexps, re)
	default:
		return
	}
	s.Count++
}

func newSite() *Site { return &Site{full: map[string]bool{}, suffixes: map[string]bool{}} }

// IP — категория сетей.
type IP struct {
	Prefixes []netip.Prefix
	Reverse  bool // reverse_match: категория означает «всё, кроме»
}

// Match проверяет адрес.
func (c *IP) Match(a netip.Addr) bool {
	if c == nil {
		return false
	}
	a = a.Unmap()
	hit := false
	for _, p := range c.Prefixes {
		if p.Contains(a) {
			hit = true
			break
		}
	}
	return hit != c.Reverse
}

// --- protobuf ---

func varint(b []byte) (uint64, int) {
	var v uint64
	for i := 0; i < len(b) && i < 10; i++ {
		v |= uint64(b[i]&0x7F) << (7 * i)
		if b[i] < 0x80 {
			return v, i + 1
		}
	}
	return 0, 0
}

// field читает одно поле: номер, тип, байты (для типа 2) или значение varint.
func field(b []byte) (num, wt int, val []byte, n uint64, rest []byte, ok bool) {
	key, k := varint(b)
	if k == 0 {
		return
	}
	b = b[k:]
	num, wt = int(key>>3), int(key&7)
	switch wt {
	case 0:
		v, m := varint(b)
		if m == 0 {
			return
		}
		return num, wt, nil, v, b[m:], true
	case 2:
		l, m := varint(b)
		if m == 0 || l > uint64(len(b)-m) {
			return
		}
		return num, wt, b[m : m+int(l)], 0, b[m+int(l):], true
	case 1:
		if len(b) < 8 {
			return
		}
		return num, wt, nil, 0, b[8:], true
	case 5:
		if len(b) < 4 {
			return
		}
		return num, wt, nil, 0, b[4:], true
	}
	return
}

var errFormat = errors.New("неверный формат базы")

// eachEntry обходит записи списка (поле 1) и отдаёт код категории (поле 1
// записи) с телом записи.
func eachEntry(data []byte, fn func(code string, entry []byte) error) error {
	for len(data) > 0 {
		num, wt, val, _, rest, ok := field(data)
		if !ok {
			return errFormat
		}
		data = rest
		if num != 1 || wt != 2 {
			continue
		}
		code := ""
		for e := val; len(e) > 0; {
			n, w, v, _, r, ok := field(e)
			if !ok {
				return errFormat
			}
			e = r
			if n == 1 && w == 2 {
				code = strings.ToLower(string(v))
				break
			}
		}
		if err := fn(code, val); err != nil {
			return err
		}
	}
	return nil
}

// ParseSites читает geosite.dat, оставляя только категории need (нижний регистр).
func ParseSites(data []byte, need map[string]bool) (map[string]*Site, error) {
	out := map[string]*Site{}
	err := eachEntry(data, func(code string, entry []byte) error {
		if !need[code] {
			return nil
		}
		s, err := parseSiteEntry(entry)
		if err == nil {
			out[code] = s
		}
		return err
	})
	return out, err
}

// parseSiteEntry разбирает тело одной записи geosite.dat.
func parseSiteEntry(entry []byte) (*Site, error) {
	s := newSite()
	for len(entry) > 0 {
		n, w, v, _, r, ok := field(entry)
		if !ok {
			return nil, errFormat
		}
		entry = r
		if n != 2 || w != 2 {
			continue
		}
		kind, value := 0, ""
		for d := v; len(d) > 0; {
			dn, dw, dv, dint, dr, ok := field(d)
			if !ok {
				return nil, errFormat
			}
			d = dr
			switch {
			case dn == 1 && dw == 0:
				kind = int(dint)
			case dn == 2 && dw == 2:
				value = string(dv)
			}
		}
		s.add(kind, value)
	}
	return s, nil
}

// ParseIPs читает geoip.dat, оставляя только категории need.
func ParseIPs(data []byte, need map[string]bool) (map[string]*IP, error) {
	out := map[string]*IP{}
	err := eachEntry(data, func(code string, entry []byte) error {
		if !need[code] {
			return nil
		}
		c, err := parseIPEntry(entry)
		if err == nil {
			out[code] = c
		}
		return err
	})
	return out, err
}

// parseIPEntry разбирает тело одной записи geoip.dat.
func parseIPEntry(entry []byte) (*IP, error) {
	c := &IP{}
	for len(entry) > 0 {
		n, w, v, iv, r, ok := field(entry)
		if !ok {
			return nil, errFormat
		}
		entry = r
		switch {
		case n == 3 && w == 0:
			c.Reverse = iv != 0
		case n == 2 && w == 2:
			var ip []byte
			bits := -1
			for d := v; len(d) > 0; {
				dn, dw, dv, dint, dr, ok := field(d)
				if !ok {
					return nil, errFormat
				}
				d = dr
				switch {
				case dn == 1 && dw == 2:
					ip = dv
				case dn == 2 && dw == 0:
					bits = int(dint)
				}
			}
			a, ok := netip.AddrFromSlice(ip)
			if !ok || bits < 0 || bits > a.BitLen() {
				continue
			}
			c.Prefixes = append(c.Prefixes, netip.PrefixFrom(a.Unmap(), bits).Masked())
		}
	}
	return c, nil
}

// ParseSiteText читает текстовый список: «domain:x», «full:x», «keyword:x»,
// «regexp:x» или просто домен; # — комментарий.
func ParseSiteText(data []byte) *Site {
	s := newSite()
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		kind := kindDomain
		if k, v, ok := strings.Cut(line, ":"); ok {
			switch strings.ToLower(k) {
			case "full":
				kind, line = kindFull, v
			case "domain":
				line = v
			case "keyword":
				kind, line = kindKeyword, v
			case "regexp":
				kind, line = kindRegex, v
			}
		}
		s.add(kind, strings.TrimPrefix(line, "*."))
	}
	return s
}

// ParseIPText читает текстовый список сетей (CIDR или адрес на строке).
func ParseIPText(data []byte) *IP {
	c := &IP{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "#")
		line = strings.TrimSpace(line)
		if p, err := netip.ParsePrefix(line); err == nil {
			c.Prefixes = append(c.Prefixes, p.Masked())
		} else if a, err := netip.ParseAddr(line); err == nil {
			c.Prefixes = append(c.Prefixes, netip.PrefixFrom(a, a.BitLen()))
		}
	}
	return c
}

// isText — данные похожи на текстовый список, а не на protobuf.
func isText(data []byte) bool {
	n := min(len(data), 4096)
	for _, b := range data[:n] {
		if b == 0 || (b < 0x09) || (b > 0x0D && b < 0x20) {
			return false
		}
	}
	return n > 0
}

// --- хранилище ---

// FileStatus — состояние одного файла базы.
type FileStatus struct {
	Present bool      `json:"present"`
	Size    int64     `json:"size"`
	Updated time.Time `json:"updated,omitempty"`
	SHA256  string    `json:"sha256,omitempty"`
	Source  string    `json:"source,omitempty"`
}

// Status — состояние баз для UI.
type Status struct {
	Site       FileStatus     `json:"geosite"`
	IP         FileStatus     `json:"geoip"`
	SiteLoaded map[string]int `json:"site_loaded"` // категория -> правил
	IPLoaded   map[string]int `json:"ip_loaded"`   // категория -> сетей
	Missing    []string       `json:"missing,omitempty"`
	LastError  string         `json:"last_error,omitempty"`
}

// Store — базы на диске и загруженные категории.
type Store struct {
	Dir string
	// Client для скачивания (по умолчанию — с таймаутом 2 минуты).
	Client *http.Client
	// MaxSize — предел размера файла базы.
	MaxSize int64

	mu      sync.RWMutex
	catMu   sync.Mutex
	cats    map[string]catCache
	sites   map[string]*Site
	ips     map[string]*IP
	missing []string
	lastErr string
	sources map[string]string
	extra   []Extra
}

func (s *Store) sitePath() string { return filepath.Join(s.Dir, "geosite.dat") }
func (s *Store) ipPath() string   { return filepath.Join(s.Dir, "geoip.dat") }

// Load загружает из файлов нужные категории (имена в нижнем регистре).
func (s *Store) Load(needSites, needIPs []string) error {
	sites, ips := map[string]*Site{}, map[string]*IP{}
	var missing []string
	var errs []string
	allSites, allIPs := needSites, needIPs
	needSites, siteTags := s.splitExtra("geosite", needSites)
	needIPs, ipTags := s.splitExtra("geoip", needIPs)
	if len(needSites) > 0 {
		var err error
		if sites, err = LoadSitesFile(s.sitePath(), needSites); err != nil {
			errs = append(errs, "geosite.dat: "+err.Error())
		}
	}
	if len(needIPs) > 0 {
		var err error
		if ips, err = LoadIPsFile(s.ipPath(), needIPs); err != nil {
			errs = append(errs, "geoip.dat: "+err.Error())
		}
	}
	if sites == nil {
		sites = map[string]*Site{}
	}
	if ips == nil {
		ips = map[string]*IP{}
	}
	errs = append(errs, s.loadExtra(sites, ips, siteTags, ipTags)...)
	for _, n := range allSites {
		if sites[n] == nil {
			missing = append(missing, "geosite:"+n)
		}
	}
	for _, n := range allIPs {
		if ips[n] == nil {
			missing = append(missing, "geoip:"+n)
		}
	}
	s.mu.Lock()
	s.sites, s.ips, s.missing = sites, ips, missing
	s.lastErr = strings.Join(errs, "; ")
	s.mu.Unlock()
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func set(list []string) map[string]bool {
	m := map[string]bool{}
	for _, v := range list {
		m[strings.ToLower(v)] = true
	}
	return m
}

// MatchSite — host входит в категорию cat.
func (s *Store) MatchSite(cat, host string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sites[cat].Match(host)
}

// IPs — сети категории (nil — не загружена).
func (s *Store) IPs(cat string) *IP {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ips[cat]
}

// Status возвращает состояние баз.
func (s *Store) Status() Status {
	st := Status{Site: fileStatus(s.sitePath()), IP: fileStatus(s.ipPath()), SiteLoaded: map[string]int{}, IPLoaded: map[string]int{}}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for k, v := range s.sites {
		st.SiteLoaded[k] = v.Count
	}
	for k, v := range s.ips {
		st.IPLoaded[k] = len(v.Prefixes)
	}
	st.Missing = append([]string(nil), s.missing...)
	sort.Strings(st.Missing)
	st.LastError = s.lastErr
	if src := s.readSources(); src != nil {
		st.Site.Source, st.IP.Source = src["geosite"], src["geoip"]
	}
	return st
}

func fileStatus(path string) FileStatus {
	fi, err := os.Stat(path)
	if err != nil {
		return FileStatus{}
	}
	fs := FileStatus{Present: true, Size: fi.Size(), Updated: fi.ModTime().UTC()}
	if f, err := os.Open(path); err == nil {
		h := sha256.New()
		if _, err := io.Copy(h, f); err == nil {
			fs.SHA256 = hex.EncodeToString(h.Sum(nil))
		}
		_ = f.Close()
	}
	return fs
}

func (s *Store) readSources() map[string]string {
	raw, err := os.ReadFile(filepath.Join(s.Dir, "sources.json"))
	if err != nil {
		return nil
	}
	var m map[string]string
	_ = json.Unmarshal(raw, &m)
	return m
}

// ValidURL проверяет адрес базы: только https (http — лишь для тестов через AllowHTTP).
func ValidURL(raw string, allowHTTP bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && !(allowHTTP && u.Scheme == "http")) {
		return fmt.Errorf("адрес базы: только https://, без логина в адресе")
	}
	return nil
}

// Download скачивает базы (пустой URL — пропустить) и атомарно заменяет файлы.
// Файл, который не удалось скачать или который не разбирается, не трогается.
func (s *Store) Download(ctx context.Context, siteURL, ipURL string, allowHTTP bool) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	src := s.readSources()
	if src == nil {
		src = map[string]string{}
	}
	var errs []string
	for _, f := range []struct{ name, url, path string }{{"geosite", siteURL, s.sitePath()}, {"geoip", ipURL, s.ipPath()}} {
		if f.url == "" {
			continue
		}
		if err := s.fetch(ctx, f.url, f.path, allowHTTP); err != nil {
			errs = append(errs, f.name+": "+err.Error())
			continue
		}
		src[f.name] = f.url
		if f.name == "geosite" {
			s.PruneIcons()
		}
	}
	if raw, err := json.Marshal(src); err == nil {
		_ = os.WriteFile(filepath.Join(s.Dir, "sources.json"), raw, 0o600)
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (s *Store) fetch(ctx context.Context, rawURL, dst string, allowHTTP bool) error {
	if err := ValidURL(rawURL, allowHTTP); err != nil {
		return err
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}
	limit := s.MaxSize
	if limit <= 0 {
		limit = 128 << 20
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ответ %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp(s.Dir, ".geo-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(resp.Body, limit+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > limit {
		return fmt.Errorf("файл больше %d МБ", limit>>20)
	}
	if n == 0 {
		return errors.New("пустой файл")
	}
	// проверка формата до замены рабочего файла
	if _, err := scanFile(tmp.Name(), nil, nil); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}

// Domains — имена категории (точные и доменные, без ключевых слов и регулярных
// выражений), не больше limit, по алфавиту. Нужны для DNS-перенаправления на роутере.
func (s *Site) Domains(limit int) []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.full)+len(s.suffixes))
	for d := range s.suffixes {
		out = append(out, d)
	}
	for d := range s.full {
		if !s.suffixes[d] {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// SiteDomains — имена загруженной категории (см. Site.Domains).
func (s *Store) SiteDomains(cat string, limit int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sites[cat].Domains(limit)
}

// catCache — список категорий файла базы; действителен, пока не изменились размер и время файла.
type catCache struct {
	size int64
	mod  time.Time
	list []string
}

// Categories — имена категорий из файла базы (kind: "geosite" или "geoip") по алфавиту.
// Список кэшируется до смены файла: разбор базы на десять с лишним мегабайт не нужен при
// каждом нажатии клавиши в поиске. Текстовый список категорий не имеет: результат пуст.
func (s *Store) Categories(kind string) []string {
	switch kind {
	case "geosite":
		return s.cachedCategories(kind, s.sitePath())
	case "geoip":
		return s.cachedCategories(kind, s.ipPath())
	}
	return nil
}

// cachedCategories — категории файла path, кэш под ключом key до смены размера или времени файла.
func (s *Store) cachedCategories(key, path string) []string {
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	s.catMu.Lock()
	defer s.catMu.Unlock()
	if c, ok := s.cats[key]; ok && c.size == st.Size() && c.mod.Equal(st.ModTime()) {
		return c.list
	}
	list := fileCategories(path)
	if s.cats == nil {
		s.cats = map[string]catCache{}
	}
	s.cats[key] = catCache{size: st.Size(), mod: st.ModTime(), list: list}
	return list
}
