//go:build !linux

package dns

import "errors"

func setMark(int, int) error {
	return errors.New("метки сокетов есть только в Linux")
}
