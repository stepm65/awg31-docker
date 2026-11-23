package main

import (
	"io"
	"strings"
	"unicode"
)

// asciiWriter приводит вывод к чистому ASCII. Журнал контейнера в RouterOS, терминалы
// SSH и serial не показывают UTF-8 (кириллица, длинные тире, кавычки превращаются в мусор),
// поэтому всё, что осталось не-ASCII (например, русские тексты ошибок из пакетов), пишется
// транслитом, а типографские знаки заменяются ASCII-аналогами.
type asciiWriter struct{ w io.Writer }

func (a asciiWriter) Write(p []byte) (int, error) {
	for _, b := range p {
		if b >= 0x80 {
			_, err := io.WriteString(a.w, toASCII(string(p)))
			return len(p), err
		}
	}
	return a.w.Write(p)
}

var cyrLower = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r",
	'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh",
	'щ': "shch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	// украинские и белорусские буквы встречаются в именах доменов и профилей
	'і': "i", 'ї': "yi", 'є': "ye", 'ґ': "g", 'ў': "u",
}

var punct = map[rune]string{
	'—': "-", '–': "-", '‑': "-", '−': "-", '«': `"`, '»': `"`, '“': `"`, '”': `"`, '„': `"`,
	'‘': "'", '’': "'", '…': "...", '→': "->", '←': "<-", '↔': "<->", '×': "x", '№': "No.",
	'•': "*", '·': "*", ' ': " ", '✓': "ok", '✗': "x",
}

// toASCII заменяет каждую не-ASCII руну: кириллицу транслитом, знаки таблицей, прочее на «?».
func toASCII(s string) string {
	rs := []rune(s)
	var b strings.Builder
	b.Grow(len(s))
	upper := func(i int) bool { return i >= 0 && i < len(rs) && unicode.IsUpper(rs[i]) }
	for i, r := range rs {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case punct[r] != "":
			b.WriteString(punct[r])
		default:
			lr := unicode.ToLower(r)
			t, ok := cyrLower[lr]
			if !ok {
				b.WriteByte('?')
				continue
			}
			if lr == r || t == "" {
				b.WriteString(t)
				continue
			}
			// заглавная буква: «Ш» в слове из заглавных (ОШИБКА) -> SH, в начале слова (Шлюз) -> Sh
			if len(t) > 1 && (upper(i-1) || upper(i+1)) {
				t = strings.ToUpper(t)
			} else {
				t = strings.ToUpper(t[:1]) + t[1:]
			}
			b.WriteString(t)
		}
	}
	return b.String()
}
