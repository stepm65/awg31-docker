package geo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Дополнительные наборы geodata (например, geo-slim или russia-blocked): файл .dat
// с тегом; в правилах категория пишется с тегом через двоеточие —
// geosite:тег:категория, geoip:тег:категория.
const MaxExtra = 8

var reExtraTag = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,23}$`)

// Extra — дополнительный набор.
type Extra struct {
	Tag  string `json:"tag"`
	URL  string `json:"url"`
	Kind string `json:"kind"` // geosite | geoip
}

// ExtraStatus — состояние набора для UI.
type ExtraStatus struct {
	Extra
	File       FileStatus `json:"file"`
	Categories int        `json:"categories"`
}

// ValidateExtra проверяет список наборов.
func ValidateExtra(list []Extra, allowHTTP bool) error {
	if len(list) > MaxExtra {
		return fmt.Errorf("дополнительных наборов не больше %d", MaxExtra)
	}
	seen := map[string]bool{}
	for _, e := range list {
		switch {
		case !reExtraTag.MatchString(e.Tag):
			return fmt.Errorf("тег набора «%s»: латиница, цифры, _ и -, до 24 символов", e.Tag)
		case seen[e.Kind+":"+e.Tag]:
			return fmt.Errorf("тег «%s» у %s повторяется", e.Tag, e.Kind)
		case e.Kind != "geosite" && e.Kind != "geoip":
			return fmt.Errorf("набор «%s»: тип geosite или geoip", e.Tag)
		}
		if err := ValidURL(e.URL, allowHTTP); err != nil {
			return fmt.Errorf("набор «%s»: %w", e.Tag, err)
		}
		seen[e.Kind+":"+e.Tag] = true
	}
	return nil
}

func (s *Store) extraPath(kind, tag string) string {
	return filepath.Join(s.Dir, "ext-"+kind+"-"+tag+".dat")
}

// SetExtra задаёт наборы (их категории подхватит следующий Load).
func (s *Store) SetExtra(list []Extra) {
	s.mu.Lock()
	s.extra = append([]Extra(nil), list...)
	s.mu.Unlock()
}

func (s *Store) extraList() []Extra {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Extra(nil), s.extra...)
}

// splitExtra делит категории на основные и наборов: «тег:категория» с известным
// тегом этого вида уходит в набор.
func (s *Store) splitExtra(kind string, need []string) (main []string, byTag map[string][]string) {
	tags := map[string]bool{}
	for _, e := range s.extraList() {
		if e.Kind == kind {
			tags[e.Tag] = true
		}
	}
	byTag = map[string][]string{}
	for _, n := range need {
		if tag, cat, ok := strings.Cut(n, ":"); ok && tags[tag] {
			byTag[tag] = append(byTag[tag], cat)
			continue
		}
		main = append(main, n)
	}
	return main, byTag
}

// loadExtra загружает категории наборов: в sites/ips они попадают как «тег:категория».
func (s *Store) loadExtra(sites map[string]*Site, ips map[string]*IP, siteTags, ipTags map[string][]string) []string {
	var errs []string
	for tag, cats := range siteTags {
		got, err := LoadSitesFile(s.extraPath("geosite", tag), cats)
		if errors.Is(err, os.ErrNotExist) {
			errs = append(errs, "набор "+tag+": файл ещё не скачан")
			continue
		} else if err != nil {
			errs = append(errs, "набор "+tag+": "+err.Error())
			continue
		}
		for c, v := range got {
			sites[tag+":"+c] = v
		}
	}
	for tag, cats := range ipTags {
		got, err := LoadIPsFile(s.extraPath("geoip", tag), cats)
		if errors.Is(err, os.ErrNotExist) {
			errs = append(errs, "набор "+tag+": файл ещё не скачан")
			continue
		} else if err != nil {
			errs = append(errs, "набор "+tag+": "+err.Error())
			continue
		}
		for c, v := range got {
			ips[tag+":"+c] = v
		}
	}
	return errs
}

// DownloadExtra скачивает наборы; onlyMissing — только те, чьих файлов ещё нет.
func (s *Store) DownloadExtra(ctx context.Context, onlyMissing, allowHTTP bool) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	var errs []string
	for _, e := range s.extraList() {
		p := s.extraPath(e.Kind, e.Tag)
		if _, err := os.Stat(p); onlyMissing && err == nil {
			continue
		}
		if err := s.fetch(ctx, e.URL, p, allowHTTP); err != nil {
			errs = append(errs, "набор "+e.Tag+": "+err.Error())
		}
	}
	s.pruneExtra()
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// pruneExtra удаляет файлы наборов, которых больше нет в списке.
func (s *Store) pruneExtra() {
	keep := map[string]bool{}
	for _, e := range s.extraList() {
		keep[filepath.Base(s.extraPath(e.Kind, e.Tag))] = true
	}
	ents, _ := os.ReadDir(s.Dir)
	for _, d := range ents {
		if n := d.Name(); strings.HasPrefix(n, "ext-") && strings.HasSuffix(n, ".dat") && !keep[n] {
			_ = os.Remove(filepath.Join(s.Dir, n))
		}
	}
}

// ExtraStatus возвращает состояние наборов.
func (s *Store) ExtraStatus() []ExtraStatus {
	out := []ExtraStatus{}
	for _, e := range s.extraList() {
		st := ExtraStatus{Extra: e, File: fileStatus(s.extraPath(e.Kind, e.Tag))}
		if st.File.Present {
			st.Categories = len(s.extraCategories(e))
		}
		out = append(out, st)
	}
	return out
}

func (s *Store) extraCategories(e Extra) []string {
	return s.cachedCategories("extra:"+e.Kind+":"+e.Tag, s.extraPath(e.Kind, e.Tag))
}

// ExtraCategories — категории набора с тегом tag вида kind, уже с префиксом «тег:».
func (s *Store) ExtraCategories(kind, tag string) []string {
	for _, e := range s.extraList() {
		if e.Kind == kind && e.Tag == tag {
			cats := s.extraCategories(e)
			out := make([]string, len(cats))
			for i, c := range cats {
				out[i] = tag + ":" + c
			}
			return out
		}
	}
	return nil
}
