package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestToASCII(t *testing.T) {
	for in, want := range map[string]string{
		"ОШИБКА: нет доступа":             "OSHIBKA: net dostupa",
		"Шлюз включён":                    "Shlyuz vklyuchyon",
		"Щука, Юла, Яма, Хата":            "Shchuka, Yula, Yama, Khata",
		"объект":                          "obekt",
		"a — b «c» → d…":                  `a - b "c" -> d...`,
		"подписка wg0: запрос к h:8443":   "podpiska wg0: zapros k h:8443",
		"ВНИМАНИЕ: UI_TLS=off":            "VNIMANIE: UI_TLS=off",
		"emoji \U0001F600 stays a mark":   "emoji ? stays a mark",
		"plain ASCII 100% [ok] {x} \"q\"": "plain ASCII 100% [ok] {x} \"q\"",
	} {
		if got := toASCII(in); got != want {
			t.Errorf("toASCII(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

func TestAsciiWriterOnlyRewritesNonASCII(t *testing.T) {
	var buf bytes.Buffer
	w := asciiWriter{&buf}
	n, err := w.Write([]byte("только ASCII? нет: Привет\n"))
	if err != nil || n != len("только ASCII? нет: Привет\n") {
		t.Fatalf("Write вернул n=%d err=%v: должен вернуть длину исходных данных", n, err)
	}
	for _, b := range buf.Bytes() {
		if b >= 0x80 {
			t.Fatalf("в выводе остались не-ASCII байты: %q", buf.String())
		}
	}
	if !strings.Contains(buf.String(), "Privet") {
		t.Fatalf("транслит не применён: %q", buf.String())
	}
	buf.Reset()
	_, _ = w.Write([]byte("plain line\n"))
	if buf.String() != "plain line\n" {
		t.Fatalf("ASCII-строка изменена: %q", buf.String())
	}
}
