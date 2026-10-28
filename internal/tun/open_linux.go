//go:build linux

// Independent TUN lifetime adapter by ir_spoof; https://t.me/ir_spoof.
package tun

import (
	"context"
	"errors"
	"io"
	"net/netip"
)

// Open creates and owns an exclusive interface; cancellation closes it. This
// never enables global forwarding or changes global firewall/sysctl settings.
func Open(ctx context.Context, config Config) (*Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if config.MTU == 0 {
		config.MTU = 1320
	}
	if config.Queues == 0 {
		config.Queues = 1
	}
	routes := append([]string(nil), config.Routes...)
	if config.Peer != "" {
		peer, err := netip.ParseAddr(config.Peer)
		if err != nil {
			prefix, parseErr := netip.ParsePrefix(config.Peer)
			if parseErr != nil {
				return nil, errors.New("invalid TUN peer address")
			}
			peer = prefix.Addr()
		}
		local, err := netip.ParsePrefix(config.Local)
		if err != nil || peer.Is4In6() || peer.IsUnspecified() || peer.IsMulticast() || peer.Is4() != local.Addr().Is4() || peer == local.Addr() {
			return nil, errors.New("TUN peer must be a distinct unicast address in the local address family")
		}
		bits := 128
		if peer.Is4() {
			bits = 32
		}
		routes = append(routes, netip.PrefixFrom(peer, bits).String())
	}
	d, err := openDevice(config.Name, config.Local, config.MTU, config.Queues, routes)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = d.Close() })
	d.contextMu.Lock()
	if d.closed {
		stop()
	} else {
		d.stopContext = stop
	}
	d.contextMu.Unlock()
	if ctx.Err() != nil {
		_ = d.Close()
		return nil, ctx.Err()
	}
	return d, nil
}

func (d *Device) Read(p []byte) (int, error) {
	if d == nil || len(d.queues) == 0 {
		return 0, io.ErrClosedPipe
	}
	return d.queues[0].Read(p)
}

func (d *Device) Write(p []byte) (int, error) {
	if d == nil || len(d.queues) == 0 {
		return 0, io.ErrClosedPipe
	}
	return d.queues[0].Write(p)
}
