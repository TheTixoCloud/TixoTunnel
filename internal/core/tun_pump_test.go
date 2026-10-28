// Persistent TUN reader/reconnect regression tests by ir_spoof; https://t.me/ir_spoof.
package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"backhaul.local/ir_spoof/internal/ipx"
	"backhaul.local/ir_spoof/internal/wire"
)

type fakeTUN struct {
	input, output       chan []byte
	closed              chan struct{}
	once                sync.Once
	readers, maxReaders atomic.Int64
}

func (f *fakeTUN) Read(buffer []byte) (int, error) {
	active := f.readers.Add(1)
	defer f.readers.Add(-1)
	for {
		old := f.maxReaders.Load()
		if old >= active || f.maxReaders.CompareAndSwap(old, active) {
			break
		}
	}
	select {
	case <-f.closed:
		return 0, io.ErrClosedPipe
	case packet := <-f.input:
		return copy(buffer, packet), nil
	}
}
func (f *fakeTUN) Write(packet []byte) (int, error) {
	select {
	case <-f.closed:
		return 0, io.ErrClosedPipe
	case f.output <- append([]byte(nil), packet...):
		return len(packet), nil
	}
}
func (f *fakeTUN) Close() error { f.once.Do(func() { close(f.closed) }); return nil }

func testPacket(size int, id uint16) []byte {
	p := make([]byte, size)
	p[0] = 0x45
	p[8] = 64
	p[9] = 17
	binary.BigEndian.PutUint16(p[2:4], uint16(size))
	binary.BigEndian.PutUint16(p[4:6], id)
	binary.BigEndian.PutUint16(p[10:12], ipx.Checksum(p[:20]))
	return p
}
func pumpEngine(t *testing.T) (*Engine, *fakeTUN) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{ctx: ctx, cancel: cancel, closers: make(map[io.Closer]bool)}
	e.cfg.TUN.MTU = 1500
	f := &fakeTUN{input: make(chan []byte, 512), output: make(chan []byte, 32), closed: make(chan struct{})}
	pump := e.startTUNPump(f)
	t.Cleanup(func() { pump.stopping.Store(true); cancel(); _ = f.Close(); e.wg.Wait() })
	return e, f
}
func eventually(t *testing.T, test func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if test() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition did not complete")
}
func startDuplex(t *testing.T, e *Engine, f *fakeTUN) (net.Conn, <-chan struct{}) {
	t.Helper()
	local, remote := net.Pipe()
	done := make(chan struct{})
	go func() { e.tunDuplex(f, local); close(done) }()
	eventually(t, func() bool { return e.tunPump.generation.Load()&1 == 1 })
	_ = remote.SetDeadline(time.Now().Add(2 * time.Second))
	return remote, done
}
func joined(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("per-carrier TUN workers were not joined")
	}
}

func TestTUNPersistentReaderAcrossReconnects(t *testing.T) {
	e, f := pumpEngine(t)
	for index := 0; index < 16; index++ {
		remote, done := startDuplex(t, e, f)
		outgoing := testPacket(100, uint16(index+1))
		f.input <- outgoing
		got, err := wire.ReadFrame(remote, 1500)
		if err != nil || !bytes.Equal(got, outgoing) {
			t.Fatal("outgoing packet lost on reconnect", err)
		}
		incoming := testPacket(80, uint16(index+100))
		if err = wire.WriteFrame(remote, incoming); err != nil {
			t.Fatal(err)
		}
		select {
		case got = <-f.output:
			if !bytes.Equal(got, incoming) {
				t.Fatal("incoming packet corrupted")
			}
		case <-time.After(time.Second):
			t.Fatal("incoming packet did not reach TUN")
		}
		_ = remote.Close()
		joined(t, done)
		select {
		case <-f.closed:
			t.Fatal("peer disconnect closed persistent TUN")
		default:
		}
	}
	if f.maxReaders.Load() != 1 {
		t.Fatal("more than one TUN reader survived reconnect", f.maxReaders.Load())
	}
}

func TestTUNOfflineAndPriorGenerationPacketsAreDropped(t *testing.T) {
	e, f := pumpEngine(t)
	f.input <- testPacket(60, 1)
	eventually(t, func() bool { return e.tunPump.dropped.Load() > 0 })
	remote, done := startDuplex(t, e, f)
	oldGeneration := e.tunPump.generation.Load()
	_ = remote.Close()
	joined(t, done)
	remote, done = startDuplex(t, e, f)
	e.tunPump.packets <- tunPacket{data: testPacket(60, 2), generation: oldGeneration}
	fresh := testPacket(60, 3)
	f.input <- fresh
	got, err := wire.ReadFrame(remote, 1500)
	if err != nil || !bytes.Equal(got, fresh) {
		t.Fatal("stale packet crossed reconnect boundary", err)
	}
	_ = remote.Close()
	joined(t, done)
}

func TestTUNCancellationJoinsIdleCarrierWorkers(t *testing.T) {
	e, f := pumpEngine(t)
	remote, done := startDuplex(t, e, f)
	defer remote.Close()
	e.cancel()
	joined(t, done)
	if f.maxReaders.Load() > 1 {
		t.Fatal("concurrent persistent readers")
	}
}

func TestTUNMalformedAndOversizeInboundFramesCloseCarrier(t *testing.T) {
	for _, oversize := range []bool{false, true} {
		t.Run(map[bool]string{false: "malformed", true: "oversize"}[oversize], func(t *testing.T) {
			e, f := pumpEngine(t)
			remote, done := startDuplex(t, e, f)
			defer remote.Close()
			packet := testPacket(20, 1)
			if oversize {
				packet = testPacket(1501, 2)
			} else {
				packet[0] = 0x35
			}
			_ = wire.WriteFrame(remote, packet)
			joined(t, done)
			select {
			case <-f.output:
				t.Fatal("invalid frame reached TUN")
			default:
			}
		})
	}
}

func TestTUNPumpQueueIsBoundedAndReaderFailureCancels(t *testing.T) {
	e, f := pumpEngine(t)
	e.tunPump.generation.Store(1)
	for index := 0; index < cap(e.tunPump.packets)+8; index++ {
		f.input <- testPacket(20, uint16(index))
	}
	eventually(t, func() bool { return e.tunPump.dropped.Load() >= 8 })
	if len(e.tunPump.packets) != cap(e.tunPump.packets) {
		t.Fatal("bounded queue overflow was not retained/dropped correctly")
	}
	_ = f.Close()
	select {
	case <-e.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("unexpected TUN reader failure did not cancel engine")
	}
}
