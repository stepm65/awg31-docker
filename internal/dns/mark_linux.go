//go:build linux

package dns

import (
	"fmt"
	"syscall"
)

func setMark(fd, mark int) error {
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_MARK, mark); err != nil {
		return fmt.Errorf("SO_MARK: %w", err)
	}
	return nil
}
