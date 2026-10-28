//go:build !linux

// Linux-only packet runtime by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"context"
	"errors"
)

func Run(context.Context, Config) error {
	return errors.New("IPX requires Linux CAP_NET_RAW and TUN CAP_NET_ADMIN")
}
