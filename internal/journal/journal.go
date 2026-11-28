// Package journal хранит последние сообщения awgd в памяти для страницы «Журнал».
//
// В журнал контейнера (stdout) строки уходят транслитом (RouterOS не показывает UTF-8),
// а здесь лежат как есть. Записи живут до перезапуска контейнера.
package journal

import (
	"bytes"
	"strings"
	"sync"
	"time"
)

// Уровни записи.
const (
	Info  = "info"
	Warn  = "warn"
	Error = "error"
)

// Entry — одна строка журнала.
type Entry struct {
	Seq    uint64    `json:"seq"`
	Time   time.Time `json:"t"`
	Level  string    `json:"level"`
	Source string    `json:"source"`
	Text   string    `json:"text"`
}

// MaxText — длиннее строки обрезаются (вывод команд бывает большим).
const MaxText = 2000

// Journal — кольцевой буфер записей. Нулевое значение не годится: см. New.
type Journal struct {
	mu   sync.Mutex
	buf  []Entry
	next int // куда писать следующую
	full bool
	seq  uint64
	now  func() time.Time
}

// New создаёт журнал на size записей.
func New(size int) *Journal {
	if size < 1 {
		size = 1
	}
	return &Journal{buf: make([]Entry, size), now: time.Now}
}

// Add добавляет запись; уровень и источник угадываются по тексту.
func (j *Journal) Add(text string) {
	j.AddAt(j.now(), text)
}

// AddAt — то же с заданным временем (строки entrypoint, записанные до старта awgd).
func (j *Journal) AddAt(t time.Time, text string) {
	text = strings.TrimRight(text, "\r\n")
	if strings.TrimSpace(text) == "" {
		return
	}
	// одноразовый токен первичной настройки не показываем и в журнале консоли
	if i := strings.Index(text, "(SETUP TOKEN): "); i >= 0 {
		text = text[:i] + "(SETUP TOKEN): ***"
	}
	if len(text) > MaxText {
		text = text[:MaxText] + "…"
	}
	src, lvl := Classify(text)
	j.mu.Lock()
	defer j.mu.Unlock()
	j.seq++
	j.buf[j.next] = Entry{Seq: j.seq, Time: t.UTC(), Level: lvl, Source: src, Text: text}
	j.next++
	if j.next == len(j.buf) {
		j.next, j.full = 0, true
	}
}

// Since возвращает записи с номером больше after (по возрастанию), не больше limit
// последних, и номер последней записи в журнале.
func (j *Journal) Since(after uint64, limit int) ([]Entry, uint64) {
	j.mu.Lock()
	defer j.mu.Unlock()
	var all []Entry
	if j.full {
		all = append(all, j.buf[j.next:]...)
	}
	all = append(all, j.buf[:j.next]...)
	i := 0
	for i < len(all) && all[i].Seq <= after {
		i++
	}
	out := all[i:]
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return append([]Entry(nil), out...), j.seq
}

// Classify угадывает источник (первое слово до двоеточия) и уровень записи.
func Classify(text string) (source, level string) {
	level = Info
	l := strings.ToLower(text)
	switch {
	case strings.HasPrefix(l, "error") || strings.Contains(l, " error:") || strings.Contains(l, "panic") ||
		strings.Contains(l, "ошибка") || strings.Contains(l, "failed") || strings.Contains(l, "не удалось"):
		level = Error
	case strings.HasPrefix(l, "warning") || strings.Contains(l, "warning:") || strings.Contains(l, "was missing") ||
		strings.Contains(l, "unhealthy") || strings.Contains(l, "no healthy") || strings.Contains(l, "does not respond") ||
		strings.Contains(l, "could not") || strings.Contains(l, "retry in") || strings.Contains(l, "errors in the rules") ||
		strings.Contains(l, "не применяется") || strings.Contains(l, "предупрежд"):
		level = Warn
	}
	t := text
	if strings.HasPrefix(t, "[") {
		if i := strings.IndexByte(t, ']'); i > 1 {
			return t[1:i], level
		}
	}
	if i := strings.IndexByte(t, ':'); i > 0 && i <= 24 {
		head := t[:i]
		if f := strings.Fields(head); len(f) > 0 && len(f) <= 2 && !strings.ContainsAny(head, "/=\"'") {
			if s := strings.ToLower(f[0]); s != "warning" && s != "error" {
				return s, level
			}
		}
	}
	return "", level
}

// Writer — io.Writer для log.Logger: каждая строка становится записью.
type Writer struct {
	J *Journal

	mu  sync.Mutex
	acc []byte
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.acc = append(w.acc, p...)
	for {
		i := bytes.IndexByte(w.acc, '\n')
		if i < 0 {
			break
		}
		w.J.Add(stripStdTime(string(w.acc[:i])))
		w.acc = w.acc[i+1:]
	}
	if len(w.acc) > 64<<10 { // строка без перевода: не копим бесконечно
		w.J.Add(string(w.acc))
		w.acc = w.acc[:0]
	}
	return len(p), nil
}

// stripStdTime убирает метку времени log.LstdFlags («2006/01/02 15:04:05 »): у записи своя.
func stripStdTime(s string) string {
	const n = len("2006/01/02 15:04:05 ")
	if len(s) >= n && s[4] == '/' && s[7] == '/' && s[10] == ' ' && s[13] == ':' && s[16] == ':' && s[19] == ' ' {
		return s[n:]
	}
	return s
}
