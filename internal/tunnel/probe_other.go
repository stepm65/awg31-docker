//go:build !linux

package tunnel

import (
	"context"
	"errors"
	"time"
)

// Probe через конкретный туннель есть только в Linux.
func (m *Manager) Probe(context.Context, string, string) (time.Duration, error) {
	return 0, errors.New("проба через туннель работает только в Linux")
}
