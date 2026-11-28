package tunnel

import (
	"context"
	"strings"
	"testing"
	"time"
)

// slowRunner держит «awg-quick up» до сигнала release.
type slowRunner struct {
	entered chan struct{}
	release chan struct{}
}

func (s *slowRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if strings.HasSuffix(name, "awg-quick") && args[0] == "up" {
		close(s.entered)
		<-s.release
	}
	return nil, nil
}

// Пока туннель поднимается (awg-quick может работать секунды), статус и основной туннель
// отдаются сразу, а второй подъём ждёт своей очереди.
func TestStatusDoesNotWaitForSlowUp(t *testing.T) {
	r := &slowRunner{entered: make(chan struct{}), release: make(chan struct{})}
	m := New(r, Options{RunDir: t.TempDir(), AutoRoute: true, LanIface: "eth0", KillSwitch: true})
	done := make(chan error, 1)
	go func() { done <- m.Up(context.Background(), "wg0", goodConf) }()
	<-r.entered

	fast := make(chan struct{})
	go func() {
		_, _ = m.Status(context.Background())
		_ = m.Primary()
		_ = m.Table("wg0")
		_ = m.RoutingError()
		close(fast)
	}()
	select {
	case <-fast:
	case <-time.After(2 * time.Second):
		t.Fatal("Status/Primary ждут, пока поднимается туннель")
	}

	second := make(chan error, 1)
	go func() { second <- m.Down(context.Background(), "wg0") }()
	select {
	case <-second:
		t.Fatal("Down выполнился, не дождавшись Up")
	case <-time.After(100 * time.Millisecond):
	}
	close(r.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-second; err != nil {
		t.Fatal(err)
	}
}
