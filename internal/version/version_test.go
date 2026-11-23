package version

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Версия живёт в одном файле VERSION. Тест не даёт ей разойтись с README:
// именно такое расхождение (код 1.7.6, документация 1.7.7) нашлось в оригинале.
func TestVersionFileIsSemverAndMatchesReadmes(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	v := strings.TrimSpace(string(raw))
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(v) {
		t.Fatalf("VERSION %q не похожа на MAJOR.MINOR.PATCH", v)
	}
	// Манифесты MikroTik ссылаются на образ с тем же тегом.
	for _, name := range []string{"routeros/app.yaml", "routeros/install-container.rsc"} {
		doc, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(doc), "awg31-docker:"+v+"\"") && !strings.Contains(string(doc), "awg31-docker:"+v+"\n") {
			t.Errorf("%s: образ должен иметь тег %s", name, v)
		}
	}
	for _, name := range []string{"README.md", "README.ru.md"} {
		doc, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(doc), "**"+v+"**") {
			t.Errorf("%s: не найдена отметка версии **%s**", name, v)
		}
	}
}
