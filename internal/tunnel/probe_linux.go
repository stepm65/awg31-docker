//go:build linux

package tunnel

import (
	"context"
	"fmt"
	"net"
	"syscall"
	"time"
)

// Probe открывает TCP-соединение с target через туннель name: сокет получает
// метку ProbeMark(таблица туннеля), правило fwmark из хуков направляет его в
// таблицу этого туннеля, а SO_BINDTODEVICE не даёт пакету уйти мимо.
// Возвращает время установки соединения.
func (m *Manager) Probe(ctx context.Context, name, target string) (time.Duration, error) {
	table := m.Table(name)
	if table == 0 {
		return 0, ErrNotUp
	}
	mark := ProbeMark(table)
	d := net.Dialer{Timeout: 5 * time.Second, Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		if err := c.Control(func(fd uintptr) { serr = markAndBind(int(fd), mark, name) }); err != nil {
			return err
		}
		return serr
	}}
	start := time.Now()
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return 0, err
	}
	_ = conn.Close()
	return time.Since(start), nil
}

// markAndBind ставит сокету метку пробы и привязывает его к интерфейсу туннеля.
func markAndBind(fd, mark int, iface string) error {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, mark); err != nil {
		return fmt.Errorf("SO_MARK: %w", err)
	}
	if err := syscall.SetsockoptString(fd, syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface); err != nil {
		return fmt.Errorf("SO_BINDTODEVICE: %w", err)
	}
	return nil
}
