package geo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
)

func TestExtraSets(t *testing.T) {
	slim := append(site("RU-AVAILABLE-ONLY-INSIDE", [2]string{"domain", "gosuslugi.ru"}), site("OTHER", [2]string{"domain", "x.example"})...)
	ipdat := geoip("RU-BLOCKED", false, "203.0.113.0/24")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "ip") {
			_, _ = w.Write(ipdat)
			return
		}
		_, _ = w.Write(slim)
	}))
	defer srv.Close()
	s := &Store{Dir: t.TempDir()}
	list := []Extra{{Tag: "slim", URL: srv.URL + "/site.dat", Kind: "geosite"}, {Tag: "slim", URL: srv.URL + "/ip.dat", Kind: "geoip"}}
	if err := ValidateExtra(list, true); err != nil {
		t.Fatal(err)
	}
	s.SetExtra(list)
	// до скачивания — понятная ошибка, категория отсутствует
	if err := s.Load([]string{"slim:ru-available-only-inside"}, nil); err == nil || !strings.Contains(err.Error(), "не скачан") {
		t.Fatalf("без файла: %v", err)
	}
	if err := s.DownloadExtra(context.Background(), true, true); err != nil {
		t.Fatal(err)
	}
	if err := s.Load([]string{"slim:ru-available-only-inside"}, []string{"slim:ru-blocked"}); err != nil {
		t.Fatal(err)
	}
	if !s.MatchSite("slim:ru-available-only-inside", "www.gosuslugi.ru") || s.MatchSite("slim:ru-available-only-inside", "x.example") {
		t.Fatal("категория набора")
	}
	if c := s.IPs("slim:ru-blocked"); c == nil || !c.Match(netip.MustParseAddr("203.0.113.9")) {
		t.Fatal("geoip набора")
	}
	st := s.ExtraStatus()
	if len(st) != 2 || !st[0].File.Present || st[0].Categories != 2 {
		t.Fatalf("%+v", st)
	}
	if got := s.ExtraCategories("geosite", "slim"); len(got) != 2 || got[1] != "slim:ru-available-only-inside" {
		t.Fatalf("%v", got)
	}
	// набор убран — его файл удаляется при следующем скачивании
	s.SetExtra(list[:1])
	_ = s.DownloadExtra(context.Background(), true, true)
	if _, err := os.Stat(s.extraPath("geoip", "slim")); err == nil {
		t.Fatal("файл удалённого набора остался")
	}
	for _, bad := range [][]Extra{{{Tag: "A B", URL: "https://x", Kind: "geosite"}}, {{Tag: "a", URL: "http://x", Kind: "geosite"}}, {{Tag: "a", URL: "https://x", Kind: "dns"}}} {
		if ValidateExtra(bad, false) == nil {
			t.Errorf("%+v должен быть ошибкой", bad)
		}
	}
}
