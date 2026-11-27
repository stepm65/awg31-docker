package geo

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Потоковое чтение файла даёт то же, что разбор целиком, включая запись, у которой
// код лежит дальше первых codePrefix байт и тело длиннее буфера.
func TestStreamMatchesWholeFile(t *testing.T) {
	big := make([][2]string, 0, 3000)
	for i := 0; i < 3000; i++ {
		big = append(big, [2]string{"domain", strings.Repeat("x", 20) + string(rune('a'+i%26)) + ".example"})
	}
	// запись с кодом после длинного поля 2: код не в первых 512 байтах
	late := pbBytes(1, append(pbBytes(2, append(pbInt(1, 2), pbBytes(2, []byte(strings.Repeat("z", 700)+".com"))...)), pbBytes(1, []byte("LATE"))...))
	data := append(site("ru-blocked", big...), site("youtube", [2]string{"domain", "youtube.com"})...)
	data = append(data, late...)
	data = append(data, site("skip", [2]string{"domain", "skip.com"})...)
	p := filepath.Join(t.TempDir(), "geosite.dat")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	need := []string{"ru-blocked", "youtube", "late"}
	got, err := LoadSitesFile(p, need)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := ParseSites(data, set(need))
	for _, n := range need {
		if got[n] == nil || len(got[n].Domains(0)) != len(want[n].Domains(0)) {
			t.Fatalf("%s: got %v want %v", n, got[n], want[n])
		}
	}
	if got["skip"] != nil || !got["youtube"].Match("m.youtube.com") {
		t.Fatal("skip/match")
	}
	cats := fileCategories(p)
	if strings.Join(cats, ",") != "late,ru-blocked,skip,youtube" {
		t.Fatalf("cats %v", cats)
	}
}

func TestStreamGarbageNeverPanics(t *testing.T) {
	r := rand.New(rand.NewSource(3))
	base := append(site("a", [2]string{"domain", "x.com"}), geoip("a", false, "1.0.0.0/8")...)
	p := filepath.Join(t.TempDir(), "x.dat")
	for i := 0; i < 2000; i++ {
		b := append([]byte(nil), base...)
		for j := 0; j < 3; j++ {
			b[r.Intn(len(b))] = byte(r.Intn(256))
		}
		b = b[:r.Intn(len(b)+1)]
		_ = os.WriteFile(p, b, 0o600)
		_, _ = LoadSitesFile(p, []string{"a"})
		_, _ = LoadIPsFile(p, []string{"a"})
		_ = fileCategories(p)
	}
}
