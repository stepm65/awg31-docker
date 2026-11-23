package geo

import (
	"context"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pbVarint(v uint64) []byte {
	var b []byte
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}
func pbBytes(num int, v []byte) []byte {
	return append(append(pbVarint(uint64(num<<3|2)), pbVarint(uint64(len(v)))...), v...)
}
func pbInt(num int, v uint64) []byte { return append(pbVarint(uint64(num<<3)), pbVarint(v)...) }

func site(code string, domains ...[2]string) []byte {
	e := pbBytes(1, []byte(code))
	kinds := map[string]uint64{"keyword": 0, "regex": 1, "domain": 2, "full": 3}
	for _, d := range domains {
		e = append(e, pbBytes(2, append(pbInt(1, kinds[d[0]]), pbBytes(2, []byte(d[1]))...))...)
	}
	return pbBytes(1, e)
}

func geoip(code string, reverse bool, cidrs ...string) []byte {
	e := pbBytes(1, []byte(code))
	for _, c := range cidrs {
		p := netip.MustParsePrefix(c)
		e = append(e, pbBytes(2, append(pbBytes(1, p.Addr().AsSlice()), pbInt(2, uint64(p.Bits()))...))...)
	}
	if reverse {
		e = append(e, pbInt(3, 1)...)
	}
	return pbBytes(1, e)
}

func TestParseSitesOnlyNeeded(t *testing.T) {
	data := append(site("YOUTUBE", [2]string{"domain", "youtube.com"}, [2]string{"full", "youtu.be"}, [2]string{"keyword", "googlevideo"}, [2]string{"regex", `^yt\d+\.example$`}, [2]string{"regex", `(?<=x)y`}),
		site("cn", [2]string{"domain", "baidu.com"})...)
	got, err := ParseSites(data, map[string]bool{"youtube": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["youtube"] == nil {
		t.Fatalf("загружаются только нужные категории: %v", got)
	}
	s := got["youtube"]
	for host, want := range map[string]bool{
		"youtube.com": true, "www.youtube.com": true, "youtu.be": true, "x.youtu.be": false,
		"r1---sn.googlevideo.com": true, "yt12.example": true, "notyoutube.com": false, "baidu.com": false,
	} {
		if s.Match(host) != want {
			t.Errorf("%s: ожидалось %v", host, want)
		}
	}
	if s.Count != 4 {
		t.Fatalf("регулярка, которую не понимает RE2, пропускается: %d", s.Count)
	}
}

func TestParseIPsAndReverse(t *testing.T) {
	data := append(geoip("RU", false, "5.8.0.0/16", "2a00:1450::/32"), geoip("private", true, "10.0.0.0/8")...)
	got, err := ParseIPs(data, map[string]bool{"ru": true, "private": true})
	if err != nil {
		t.Fatal(err)
	}
	ru := got["ru"]
	if !ru.Match(netip.MustParseAddr("5.8.1.1")) || !ru.Match(netip.MustParseAddr("2a00:1450::1")) || ru.Match(netip.MustParseAddr("1.1.1.1")) {
		t.Fatalf("geoip:ru %v", ru.Prefixes)
	}
	if !ru.Match(netip.MustParseAddr("::ffff:5.8.1.1")) {
		t.Fatal("IPv4 в IPv6-записи должен совпадать")
	}
	if got["private"].Match(netip.MustParseAddr("10.1.1.1")) || !got["private"].Match(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("reverse_match инвертирует совпадение")
	}
}

func TestTextLists(t *testing.T) {
	s := ParseSiteText([]byte("# список\nexample.com\nfull:exact.org\nkeyword:torrent\n*.wild.net\n"))
	if !s.Match("a.example.com") || !s.Match("exact.org") || s.Match("a.exact.org") || !s.Match("mytorrents.io") || !s.Match("x.wild.net") {
		t.Fatal("текстовый список доменов")
	}
	ip := ParseIPText([]byte("1.2.3.0/24\n9.9.9.9 # одиночный\nмусор\n"))
	if len(ip.Prefixes) != 2 || !ip.Match(netip.MustParseAddr("9.9.9.9")) {
		t.Fatalf("текстовый список сетей: %v", ip.Prefixes)
	}
}

func TestGarbageNeverPanics(t *testing.T) {
	r := rand.New(rand.NewSource(2))
	base := append(site("a", [2]string{"domain", "x.com"}), geoip("a", false, "1.0.0.0/8")...)
	for i := 0; i < 5000; i++ {
		b := append([]byte(nil), base...)
		for j := 0; j < 3; j++ {
			b[r.Intn(len(b))] = byte(r.Intn(256))
		}
		b = b[:r.Intn(len(b)+1)]
		_, _ = ParseSites(b, map[string]bool{"a": true})
		_, _ = ParseIPs(b, map[string]bool{"a": true})
	}
}

func TestStoreDownloadValidatesAndReplacesAtomically(t *testing.T) {
	good := site("youtube", [2]string{"domain", "youtube.com"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/geosite.dat":
			_, _ = w.Write(good)
		case "/broken.dat":
			_, _ = w.Write([]byte{0x0a, 0xff, 0xff, 0xff, 0x0f, 0x00}) // длина больше данных
		case "/big.dat":
			_, _ = w.Write([]byte(strings.Repeat("example.com\n", 200))) // правильный текстовый список, но больше предела
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	s := &Store{Dir: t.TempDir(), MaxSize: 1024}
	ctx := context.Background()
	if err := s.Download(ctx, srv.URL+"/geosite.dat", "", false); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("http без разрешения запрещён: %v", err)
	}
	if err := s.Download(ctx, srv.URL+"/geosite.dat", "", true); err != nil {
		t.Fatal(err)
	}
	if err := s.Load([]string{"youtube", "netflix"}, nil); err != nil {
		t.Fatal(err)
	}
	if !s.MatchSite("youtube", "m.youtube.com") {
		t.Fatal("категория из скачанной базы")
	}
	st := s.Status()
	if !st.Site.Present || st.SiteLoaded["youtube"] != 1 || len(st.Missing) != 1 || st.Missing[0] != "geosite:netflix" || st.Site.Source == "" {
		t.Fatalf("статус: %+v", st)
	}
	before, _ := os.ReadFile(filepath.Join(s.Dir, "geosite.dat"))
	for _, path := range []string{"/broken.dat", "/big.dat", "/missing.dat"} {
		if err := s.Download(ctx, srv.URL+path, "", true); err == nil {
			t.Fatalf("%s: ожидалась ошибка", path)
		}
	}
	after, _ := os.ReadFile(filepath.Join(s.Dir, "geosite.dat"))
	if string(before) != string(after) {
		t.Fatal("неудачное скачивание не должно портить рабочий файл")
	}
	if ValidURL("https://user:pw@example.com/x", false) == nil || ValidURL("ftp://example.com/x", false) == nil {
		t.Fatal("ValidURL")
	}
}

func TestCategoriesListCachedAndRefreshedOnChange(t *testing.T) {
	dir := t.TempDir()
	st := &Store{Dir: dir}
	if got := st.Categories("geosite"); got != nil {
		t.Fatalf("без файла категорий нет: %v", got)
	}
	data := append(site("YouTube", [2]string{"domain", "youtube.com"}), site("telegram", [2]string{"domain", "t.me"})...)
	data = append(data, site("youtube", [2]string{"domain", "youtu.be"})...) // повтор в другом регистре
	if err := os.WriteFile(filepath.Join(dir, "geosite.dat"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	got := st.Categories("geosite")
	if len(got) != 2 || got[0] != "telegram" || got[1] != "youtube" {
		t.Fatalf("имена в нижнем регистре, без повторов, по алфавиту: %v", got)
	}
	// файл заменили: кэш обязан обновиться (размер другой)
	more := append(data, site("netflix", [2]string{"domain", "netflix.com"})...)
	if err := os.WriteFile(filepath.Join(dir, "geosite.dat"), more, 0o600); err != nil {
		t.Fatal(err)
	}
	if got = st.Categories("geosite"); len(got) != 3 {
		t.Fatalf("после замены файла список должен обновиться: %v", got)
	}
	// текстовый список и неизвестный вид категорий не имеют
	if err := os.WriteFile(filepath.Join(dir, "geoip.dat"), []byte("10.0.0.0/8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if st.Categories("geoip") != nil || st.Categories("other") != nil {
		t.Fatal("у текстового списка и неизвестного вида категорий нет")
	}
}
