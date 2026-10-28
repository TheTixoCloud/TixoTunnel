//go:build !linux

// Linux-only TUN facilities by ir_spoof; https://t.me/ir_spoof.
package tun

import (
	"context"
	"errors"
	"io"
)

const MaxPacketSize = 65535

type Device struct{}

func Open(context.Context, Config) (*Device, error) { return nil, errors.New("TUN requires Linux") }
func (*Device) Name() string                        { return "" }
func (*Device) Queue(int) io.ReadWriteCloser        { return nil }
func (*Device) Read([]byte) (int, error)            { return 0, io.ErrClosedPipe }
func (*Device) Write([]byte) (int, error)           { return 0, io.ErrClosedPipe }
func (*Device) Close() error                        { return nil }
