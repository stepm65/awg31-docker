package geo

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func testPNG(w, h uint32) []byte {
	b := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 13, 'I', 'H', 'D', 'R'}, make([]byte, 8)...)
	binary.BigEndian.PutUint32(b[16:], w)
	binary.BigEndian.PutUint32(b[20:], h)
	return append(b, make([]byte, 20)...)
}

func TestIconBase(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://iplist.opencck.org/?format=geosite&data=domains", "https://iplist.opencck.org/"},
		{"https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", ""},
		{"https://example.org/lists/?format=geosite", ""}, // не корень
		{"http://x.test/?format=geosite", ""},             // http без разрешения
		{"", ""},
	} {
		if got := IconBase(c.in, false); got != c.want {
			t.Errorf("IconBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	if got := IconBase("http://127.0.0.1:1/?format=geosite", true); got != "http://127.0.0.1:1/" {
		t.Errorf("http разрешён: %q", got)
	}
	for tag, want := range map[string]bool{"telegram.org": true, "jetbrains@cdn": false, "messengers": false, "../x.com": false, "a.b": true} {
		if IconSite(tag) != want {
			t.Errorf("IconSite(%q) != %v", tag, want)
		}
	}
}

func TestIconFetchCacheAndRejects(t *testing.T) {
	var hits int32
	good := testPNG(32, 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.URL.Path != "/favicon" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Query().Get("site") {
		case "good.org":
			w.Header().Set("Content-Type", "image/x-icon") // заголовку не верим, смотрим на содержимое
			_, _ = w.Write(good)
		case "page.org":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(bytes.Repeat([]byte("<!DOCTYPE html><html>"), 5))
		case "blank.org":
			_, _ = w.Write(testPNG(1, 1))
		case "gone.org":
			http.NotFound(w, r)
		default:
			http.Error(w, "boom", http.StatusBadGateway)
		}
	}))
	defer srv.Close()
	st := &Store{Dir: t.TempDir()}
	ctx := context.Background()

	data, ct, err := st.Icon(ctx, srv.URL+"/", "good.org", true)
	if err != nil || ct != "image/png" || !bytes.Equal(data, good) {
		t.Fatalf("значок: %v %q", err, ct)
	}
	if _, _, err = st.Icon(ctx, srv.URL+"/", "good.org", true); err != nil || atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("второй раз из кеша: %v, запросов %d", err, hits)
	}
	for _, site := range []string{"page.org", "blank.org", "gone.org"} {
		if _, _, err = st.Icon(ctx, srv.URL+"/", site, true); !errors.Is(err, ErrNoIcon) {
			t.Fatalf("%s: ждали ErrNoIcon, вышло %v", site, err)
		}
	}
	n := atomic.LoadInt32(&hits)
	if _, _, err = st.Icon(ctx, srv.URL+"/", "page.org", true); !errors.Is(err, ErrNoIcon) || atomic.LoadInt32(&hits) != n {
		t.Fatalf("отсутствие значка запоминается на сутки: %v, запросов %d→%d", err, n, hits)
	}
	// по прошествии суток поиск повторяется
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(filepath.Join(st.Dir, "icons", "page.org.none"), old, old)
	if _, _, err = st.Icon(ctx, srv.URL+"/", "page.org", true); !errors.Is(err, ErrNoIcon) || atomic.LoadInt32(&hits) != n+1 {
		t.Fatalf("повторная проверка: %v, запросов %d", err, hits)
	}
	// сбой сервера не запоминается как «значка нет»
	if _, _, err = st.Icon(ctx, srv.URL+"/", "err.org", true); err == nil || errors.Is(err, ErrNoIcon) {
		t.Fatalf("сбой должен остаться ошибкой: %v", err)
	}
	if _, serr := os.Stat(filepath.Join(st.Dir, "icons", "err.org.none")); serr == nil {
		t.Fatal("при сбое .none не создаётся")
	}
	if _, _, err = st.Icon(ctx, srv.URL+"/", "../etc.passwd", true); !errors.Is(err, ErrNoIcon) {
		t.Fatalf("недопустимое имя: %v", err)
	}
	if _, _, err = st.Icon(ctx, "ftp://x/", "good2.org", false); err == nil {
		t.Fatal("адрес сервера проверяется")
	}
}

func TestPruneIcons(t *testing.T) {
	st := &Store{Dir: t.TempDir()}
	dir := filepath.Join(st.Dir, "icons")
	_ = os.MkdirAll(dir, 0o700)
	for _, n := range []string{"keep.org.img", "keep2.org.none", "gone.org.img", "gone2.org.none", "junk.txt"} {
		_ = os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600)
	}
	st.PruneIcons() // базы нет — ничего не удаляется
	if ents, _ := os.ReadDir(dir); len(ents) != 5 {
		t.Fatalf("без базы кеш не трогаем: %d", len(ents))
	}
	pb := func(v []byte) []byte { return append([]byte{0x0a, byte(len(v))}, v...) }
	var data []byte
	for _, c := range []string{"KEEP.ORG", "KEEP2.ORG", "MESSENGERS"} {
		e := pb([]byte(c))
		data = append(data, append([]byte{0x0a, byte(len(e))}, e...)...)
	}
	if err := os.WriteFile(filepath.Join(st.Dir, "geosite.dat"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	st.PruneIcons()
	ents, _ := os.ReadDir(dir)
	var left []string
	for _, e := range ents {
		left = append(left, e.Name())
	}
	if len(left) != 2 || left[0] != "keep.org.img" || left[1] != "keep2.org.none" {
		t.Fatalf("остались: %v", left)
	}
}
