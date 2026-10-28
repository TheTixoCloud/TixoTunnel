//go:build !linux

// Host-test boundary by ir_spoof - t.me/ir_spoof. Production target is Linux.
package transport

import (
	"errors"
	"syscall"
)

func setMSS(_ syscall.RawConn, _ int) error { return errors.New("TCP MSS tuning requires Linux") }
