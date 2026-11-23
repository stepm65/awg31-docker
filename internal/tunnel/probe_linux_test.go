//go:build linux

package tunnel

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
	"testing"
)

func TestProbeMarksAndBindsSocket(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	m := New(&fakeRunner{}, Options{})
	if _, err := m.Probe(context.Background(), "lo", ln.Addr().String()); !errors.Is(err, ErrNotUp) {
		t.Fatalf("проба через не поднятый туннель: %v", err)
	}
	m.tables["lo"] = TunnelTableMin // «туннель» — loopback, чтобы проверить сокет без NET_ADMIN-стенда
	rtt, err := m.Probe(context.Background(), "lo", ln.Addr().String())
	if err != nil {
		if skipSockopt(err) {
			t.Skip("сокет-опции недоступны в этой среде:", err)
		}
		t.Fatal(err)
	}
	if rtt <= 0 {
		t.Fatalf("время соединения %v", rtt)
	}
	// сокет, привязанный к другому интерфейсу, до loopback-адреса не дойдёт
	m.tables["nonexistent0"] = TunnelTableMin + 1
	if _, err := m.Probe(context.Background(), "nonexistent0", ln.Addr().String()); err == nil {
		t.Fatal("привязка к интерфейсу должна действовать")
	}
}

func TestMarkAndBindSetsSocketOptions(t *testing.T) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(fd)
	if err := markAndBind(fd, ProbeMark(111), "lo"); err != nil {
		if skipSockopt(err) {
			t.Skip("сокет-опции недоступны в этой среде:", err)
		}
		t.Fatal(err)
	}
	got, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK)
	if err != nil || got != ProbeMark(111) {
		t.Fatalf("метка сокета %#x, ожидалась %#x (%v)", got, ProbeMark(111), err)
	}
}

// skipSockopt: нет прав (EPERM) или эмулятор QEMU user-mode, который не
// поддерживает SO_MARK (ENOPROTOOPT) — на настоящем Linux такого не бывает.
func skipSockopt(err error) bool {
	return errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOPROTOOPT) ||
		strings.Contains(err.Error(), "operation not permitted") || strings.Contains(err.Error(), "protocol not available")
}
