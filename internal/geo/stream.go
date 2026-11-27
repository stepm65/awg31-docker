package geo

import (
	"bufio"
	"errors"
	"io"
	"os"
	"sort"
	"strings"
)

// Базы вроде runetfreedom весят под сотню мегабайт (geosite.dat ~90 МБ), а на роутере у
// контейнера GOMEMLIMIT 64 МиБ. Поэтому файл не читается целиком: записи обходятся
// потоком, и в память попадают только тела нужных категорий.

// codePrefix — сколько байт начала записи читается, чтобы узнать её код (поле 1).
const codePrefix = 512

// scanFile обходит записи protobuf-файла path. need решает по коду, нужна ли запись
// целиком (nil — не нужна никакая: только проверка формата); fn получает тело нужной.
// Для текстового списка возвращает text=true и ничего не обходит.
func scanFile(path string, need func(code string) bool, fn func(code string, entry []byte) error) (text bool, err error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return false, err
	}
	r := bufio.NewReaderSize(f, 64<<10)
	head, _ := r.Peek(4096)
	if isText(head) {
		return true, nil
	}
	size := uint64(st.Size())
	for {
		key, err := readVarint(r)
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, errFormat
		}
		num, wt := key>>3, key&7
		switch wt {
		case 0:
			if _, err := readVarint(r); err != nil {
				return false, errFormat
			}
			continue
		case 1, 5:
			n := 8
			if wt == 5 {
				n = 4
			}
			if _, err := r.Discard(n); err != nil {
				return false, errFormat
			}
			continue
		case 2:
		default:
			return false, errFormat
		}
		l, err := readVarint(r)
		if err != nil || l > size {
			return false, errFormat
		}
		if num != 1 {
			if _, err := r.Discard(int(l)); err != nil {
				return false, errFormat
			}
			continue
		}
		pre := make([]byte, min(l, codePrefix))
		if _, err := io.ReadFull(r, pre); err != nil {
			return false, errFormat
		}
		code, found := entryCode(pre)
		rest := int(l) - len(pre)
		if !found && rest > 0 {
			// код дальше начала записи: читаем её целиком
			full := make([]byte, l)
			copy(full, pre)
			if _, err := io.ReadFull(r, full[len(pre):]); err != nil {
				return false, errFormat
			}
			pre, rest = full, 0
			code, _ = entryCode(pre)
		}
		if need == nil || !need(code) {
			if _, err := r.Discard(rest); err != nil {
				return false, errFormat
			}
			continue
		}
		entry := pre
		if rest > 0 {
			entry = make([]byte, l)
			copy(entry, pre)
			if _, err := io.ReadFull(r, entry[len(pre):]); err != nil {
				return false, errFormat
			}
		}
		if err := fn(code, entry); err != nil {
			return false, err
		}
	}
}

// entryCode ищет код категории (поле 1, строка) в начале тела записи.
func entryCode(b []byte) (string, bool) {
	for len(b) > 0 {
		n, w, v, _, r, ok := field(b)
		if !ok {
			return "", false
		}
		if n == 1 && w == 2 {
			return strings.ToLower(string(v)), true
		}
		b = r
	}
	return "", false
}

func readVarint(r io.ByteReader) (uint64, error) {
	var v uint64
	for i := 0; i < 10; i++ {
		b, err := r.ReadByte()
		if err != nil {
			if i == 0 && err == io.EOF {
				return 0, io.EOF
			}
			return 0, errFormat
		}
		v |= uint64(b&0x7F) << (7 * i)
		if b < 0x80 {
			return v, nil
		}
	}
	return 0, errFormat
}

// LoadSitesFile загружает из geosite-файла категории need (нижний регистр). Текстовый
// список отдаётся под каждым запрошенным именем.
func LoadSitesFile(path string, need []string) (map[string]*Site, error) {
	want := set(need)
	out := map[string]*Site{}
	text, err := scanFile(path, func(c string) bool { return want[c] }, func(c string, e []byte) error {
		s, err := parseSiteEntry(e)
		if err == nil {
			out[c] = s
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if text {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		t := ParseSiteText(data)
		for _, n := range need {
			out[n] = t
		}
	}
	return out, nil
}

// LoadIPsFile — то же для geoip-файла.
func LoadIPsFile(path string, need []string) (map[string]*IP, error) {
	want := set(need)
	out := map[string]*IP{}
	text, err := scanFile(path, func(c string) bool { return want[c] }, func(c string, e []byte) error {
		ip, err := parseIPEntry(e)
		if err == nil {
			out[c] = ip
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	if text {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		t := ParseIPText(data)
		for _, n := range need {
			out[n] = t
		}
	}
	return out, nil
}

// fileCategories — коды категорий файла по алфавиту (у текстового списка их нет).
func fileCategories(path string) []string {
	seen := map[string]bool{}
	var list []string
	_, err := scanFile(path, func(c string) bool {
		if c != "" && !seen[c] {
			seen[c] = true
			list = append(list, c)
		}
		return false
	}, nil)
	if err != nil && !errors.Is(err, os.ErrNotExist) && len(list) == 0 {
		return nil
	}
	sort.Strings(list)
	return list
}
