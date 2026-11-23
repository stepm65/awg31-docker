package web

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"awg31/internal/policy"
)

// Редактор маршрутов читает и пишет тот же текст правил, что и движок. Здесь модель из
// static/rules.js сверяется с policy.Parse: включённые строки, порядок и назначение
// должны совпадать после любых действий в редакторе. Без node тест пропускается.
func TestRulesJSMatchesEngine(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node не найден")
	}
	if out, err := exec.Command(node, "../../scripts/test-rules.mjs").CombinedOutput(); err != nil {
		t.Fatalf("scripts/test-rules.mjs: %v\n%s", err, out)
	}
	raw, err := exec.Command(node, "../../scripts/test-rules.mjs", "--dump").Output()
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text     string `json:"text"`
		Variants []struct {
			Name      string   `json:"name"`
			Text      string   `json:"text"`
			Effective []string `json:"effective"`
		} `json:"variants"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("пустой корпус")
	}
	// Правила в каноническом виде; строки с ошибками движок пропускает, как и редактор.
	canon := func(text string) []string {
		rules, _ := policy.Parse(text)
		out := make([]string, 0, len(rules))
		for _, r := range rules {
			out = append(out, r.String())
		}
		return out
	}
	for _, c := range cases {
		want := canon(c.Text)
		for _, v := range c.Variants {
			if v.Name == "orig" {
				if got := canon(v.Text); strings.Join(got, "\n") != strings.Join(want, "\n") {
					t.Errorf("%q: после разбора и записи правила изменились:\n было %q\n стало %q", c.Text, want, got)
				}
			}
			// То, что редактор считает включённым, движок должен прочитать так же.
			if got, eff := canon(v.Text), canon(strings.Join(v.Effective, "\n")); strings.Join(got, "\n") != strings.Join(eff, "\n") {
				t.Errorf("%q, %s: движок видит %q, редактор ожидает %q", c.Text, v.Name, got, eff)
			}
		}
	}
}
