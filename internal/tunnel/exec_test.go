package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func script(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "cmd.sh")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// awg-quick оставляет в фоне демон amneziawg-go, который держит каналы вывода.
// Run обязан вернуться, не дожидаясь его (иначе вешается весь веб и менеджер туннелей).
func TestExecRunnerDoesNotWaitForBackgroundDaemon(t *testing.T) {
	p := script(t, "sleep 6 &\necho interface-created")
	start := time.Now()
	out, err := ExecRunner{Timeout: 20 * time.Second, WaitDelay: 300 * time.Millisecond}.Run(context.Background(), p)
	if err != nil {
		t.Fatalf("успешная команда с фоновым процессом не должна давать ошибку: %v", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("Run ждал фоновый процесс %v", d)
	}
	if !strings.Contains(string(out), "interface-created") {
		t.Fatalf("вывод потерян: %q", out)
	}
}

func TestExecRunnerEnvHasNoLogLevel(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), script(t, "env"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	// LOG_LEVEL заставляет демон наследовать stdout/stderr и держать их открытыми.
	if strings.Contains(s, "LOG_LEVEL") {
		t.Fatalf("LOG_LEVEL задавать нельзя:\n%s", s)
	}
	if !strings.Contains(s, "WG_QUICK_USERSPACE_IMPLEMENTATION=amneziawg-go") {
		t.Fatalf("не задан userspace-вариант:\n%s", s)
	}
}

func TestExecRunnerFailureKeepsOutput(t *testing.T) {
	_, err := ExecRunner{}.Run(context.Background(), script(t, "echo boom; exit 3"))
	if err == nil || !strings.Contains(err.Error(), "boom") || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("ошибка должна нести вывод и код: %v", err)
	}
}

func TestExecRunnerTimeoutKillsCommand(t *testing.T) {
	start := time.Now()
	_, err := ExecRunner{Timeout: 300 * time.Millisecond, WaitDelay: 300 * time.Millisecond}.Run(context.Background(), script(t, "exec sleep 10"))
	if err == nil {
		t.Fatal("ожидалась ошибка таймаута")
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("таймаут не сработал: %v", d)
	}
}

func TestExecRunnerLimitsOutput(t *testing.T) {
	out, err := ExecRunner{}.Run(context.Background(), script(t, "head -c 600000 /dev/zero | tr '\\0' x"))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != maxRunOutput {
		t.Fatalf("вывод не ограничен: %d байт", len(out))
	}
}
