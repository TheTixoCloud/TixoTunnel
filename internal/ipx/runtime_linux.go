//go:build linux

// Bidirectional TUN/raw-IP runtime by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"backhaul.local/ir_spoof/internal/tun"
)

type counters struct{ txPackets, rxPackets, txBytes, rxBytes, dropped, replay atomic.Uint64 }

func (c *counters) snapshot() Stats {
	return Stats{TxPackets: c.txPackets.Load(), RxPackets: c.rxPackets.Load(), TxBytes: c.txBytes.Load(), RxBytes: c.rxBytes.Load(), Dropped: c.dropped.Load(), ReplayRejected: c.replay.Load()}
}

// Run owns its packet sockets, exclusive nonpersistent TUN and added routes.
// It performs no public IP lookup, license contact, telemetry or remote update.
func Run(parent context.Context, config Config) error {
	s, err := normalized(config)
	if err != nil {
		return err
	}
	if err = parent.Err(); err != nil {
		return err
	}
	session, err := NewSession(config)
	if err != nil {
		return err
	}
	wire, err := openCarrier(s)
	if err != nil {
		return err
	}
	defer wire.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	device, err := tun.Open(ctx, tun.Config{Name: s.TUNName, Local: s.LocalTUN, Peer: s.PeerTUN, MTU: s.MTU, Queues: s.Queues, Routes: s.Routes})
	if err != nil {
		return err
	}
	defer device.Close()
	stopWire := context.AfterFunc(ctx, func() { _ = wire.Close() })
	defer stopWire()
	var stats counters
	errorsOut := make(chan error, s.Queues+1)
	var workers sync.WaitGroup
	worker := func(fn func() error) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			result := fn()
			if result != nil {
				select {
				case errorsOut <- result:
				default:
				}
			}
			cancel()
		}()
	}
	for index := 0; index < s.Queues; index++ {
		queue := device.Queue(index)
		worker(func() error {
			buffer := make([]byte, MaximumPacket)
			for ctx.Err() == nil {
				n, e := queue.Read(buffer)
				if e != nil {
					return e
				}
				frames, e := session.EncodePacket(buffer[:n])
				if e != nil {
					stats.dropped.Add(1)
					continue
				}
				for _, frame := range frames {
					if e = wire.WriteFrame(frame); e != nil {
						return e
					}
				}
				stats.txPackets.Add(1)
				stats.txBytes.Add(uint64(n))
			}
			return ctx.Err()
		})
	}
	worker(func() error {
		for ctx.Err() == nil {
			frame, e := wire.ReadFrame()
			if e != nil {
				return e
			}
			packet, e := session.DecodePacket(frame, time.Now())
			if e != nil {
				stats.dropped.Add(1)
				if errors.Is(e, ErrReplay) {
					stats.replay.Add(1)
				}
				continue
			}
			if packet == nil {
				continue
			}
			n, e := device.Write(packet)
			if e != nil {
				return e
			}
			if n != len(packet) {
				return io.ErrShortWrite
			}
			stats.rxPackets.Add(1)
			stats.rxBytes.Add(uint64(n))
		}
		return ctx.Err()
	})
	if s.OnReady != nil {
		s.OnReady(Info{TUNName: device.Name(), Profile: s.Profile, LocalTUN: s.LocalTUN, PeerTUN: s.PeerTUN, MTU: s.MTU, OuterMTU: s.OuterMTU})
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
		case <-ticker.C:
			if s.OnStats != nil {
				s.OnStats(stats.snapshot())
			}
		}
	}
	_ = device.Close()
	_ = wire.Close()
	workers.Wait()
	if s.OnStats != nil {
		s.OnStats(stats.snapshot())
	}
	if parent.Err() != nil {
		return parent.Err()
	}
	select {
	case err = <-errorsOut:
		return err
	default:
		return ctx.Err()
	}
}
