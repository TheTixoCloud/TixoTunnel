//go:build linux

// Independent Linux socket tuning by ir_spoof - t.me/ir_spoof.
package transport

import (
	"errors"
	"syscall"
)

func setMSS(c syscall.RawConn, mss int) error {
	var failure error
	if err := c.Control(func(fd uintptr) {
		failure = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_TCP, syscall.TCP_MAXSEG, mss)
	}); err != nil {
		return errors.New("cannot access socket for TCP MSS")
	}
	if failure != nil {
		return errors.New("cannot set TCP MSS")
	}
	return nil
}
