//go:build linux

// Owned raw IPv4 sockets by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

type carrier struct {
	codec           *Codec
	rx, tx, wake    int
	closed          atomic.Bool
	readMu, writeMu sync.Mutex
	destination     *unix.SockaddrInet4
	sequence        uint32
}

func htons(v uint16) uint16 {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	return binary.NativeEndian.Uint16(b[:])
}

func localInterface(s *settings) (*net.Interface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range interfaces {
		if s.Interface != "" && iface.Name != s.Interface {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, value := range addresses {
			prefix, e := netip.ParsePrefix(value.String())
			if e == nil && prefix.Addr().Unmap() == s.local {
				if iface.MTU < s.OuterMTU {
					return nil, errors.New("IPX outer MTU exceeds selected interface MTU")
				}
				copy := iface
				return &copy, nil
			}
		}
	}
	return nil, errors.New("IPX real local IPv4 must already be assigned to the selected active interface")
}

func openCarrier(s *settings) (*carrier, error) {
	iface, err := localInterface(s)
	if err != nil {
		return nil, err
	}
	c := &carrier{codec: &Codec{s: s}, rx: -1, tx: -1, wake: -1, destination: &unix.SockaddrInet4{Addr: s.peer.As4()}}
	ok := false
	defer func() {
		if !ok {
			_ = c.Close()
		}
	}()
	c.rx, err = unix.Socket(unix.AF_PACKET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(htons(unix.ETH_P_IP)))
	if err != nil {
		return nil, fmt.Errorf("open scoped IPX packet receiver: %w", err)
	}
	if err = unix.Bind(c.rx, &unix.SockaddrLinklayer{Protocol: htons(unix.ETH_P_IP), Ifindex: iface.Index}); err != nil {
		return nil, err
	}
	// Socket-local BPF drops unrelated traffic before allocating Go buffers. It
	// does not request promiscuous membership or change shared firewall rules.
	filter := []unix.SockFilter{}
	guard := func(code uint16, offset, value uint32) {
		filter = append(filter, unix.SockFilter{Code: code, K: offset}, unix.SockFilter{Code: unix.BPF_JMP | unix.BPF_JEQ | unix.BPF_K, Jt: 1, K: value}, unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: 0})
	}
	guard(unix.BPF_LD|unix.BPF_B|unix.BPF_ABS, 9, uint32(s.protocol))
	src, dst := s.expectedSource.As4(), s.local.As4()
	guard(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 12, binary.BigEndian.Uint32(src[:]))
	guard(unix.BPF_LD|unix.BPF_W|unix.BPF_ABS, 16, binary.BigEndian.Uint32(dst[:]))
	filter = append(filter, unix.SockFilter{Code: unix.BPF_RET | unix.BPF_K, K: uint32(s.OuterMTU)})
	program := unix.SockFprog{Len: uint16(len(filter)), Filter: &filter[0]}
	if err = unix.SetsockoptSockFprog(c.rx, unix.SOL_SOCKET, unix.SO_ATTACH_FILTER, &program); err != nil {
		return nil, err
	}
	if err = unix.SetsockoptInt(c.rx, unix.SOL_SOCKET, unix.SO_RCVBUF, 4*1024*1024); err != nil {
		return nil, err
	}
	c.tx, err = unix.Socket(unix.AF_INET, unix.SOCK_RAW|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(s.protocol))
	if err != nil {
		return nil, fmt.Errorf("open IPX raw transmitter: %w", err)
	}
	if err = unix.SetsockoptInt(c.tx, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		return nil, err
	}
	if err = unix.SetsockoptString(c.tx, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, iface.Name); err != nil {
		return nil, err
	}
	if err = unix.Bind(c.tx, &unix.SockaddrInet4{Addr: s.local.As4()}); err != nil {
		return nil, err
	}
	if err = unix.SetsockoptInt(c.tx, unix.SOL_SOCKET, unix.SO_SNDBUF, 4*1024*1024); err != nil {
		return nil, err
	}
	c.wake, err = unix.Eventfd(0, unix.EFD_CLOEXEC|unix.EFD_NONBLOCK)
	if err != nil {
		return nil, err
	}
	var seed [4]byte
	if err = randomFull(seed[:]); err != nil {
		return nil, err
	}
	c.sequence = binary.BigEndian.Uint32(seed[:])
	ok = true
	return c, nil
}

func (c *carrier) wait(fd int, events int16) error {
	for !c.closed.Load() {
		fds := []unix.PollFd{{Fd: int32(fd), Events: events}, {Fd: int32(c.wake), Events: unix.POLLIN}}
		_, err := unix.Poll(fds, 1000)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if c.closed.Load() || fds[1].Revents != 0 {
			return io.ErrClosedPipe
		}
		if fds[0].Revents&events != 0 {
			return nil
		}
		if fds[0].Revents&(unix.POLLERR|unix.POLLHUP|unix.POLLNVAL) != 0 {
			return io.ErrUnexpectedEOF
		}
	}
	return io.ErrClosedPipe
}

func (c *carrier) ReadFrame() ([]byte, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	buffer := make([]byte, c.codec.s.OuterMTU+1)
	for !c.closed.Load() {
		if err := c.wait(c.rx, unix.POLLIN); err != nil {
			return nil, err
		}
		n, from, err := unix.Recvfrom(c.rx, buffer, unix.MSG_TRUNC)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if n > c.codec.s.OuterMTU {
			continue
		}
		link, ok := from.(*unix.SockaddrLinklayer)
		if !ok || link.Pkttype == unix.PACKET_OUTGOING {
			continue
		}
		frame, err := c.codec.Decode(buffer[:n])
		if err != nil {
			continue
		}
		return append([]byte(nil), frame...), nil
	}
	return nil, io.ErrClosedPipe
}

func (c *carrier) WriteFrame(frame []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.closed.Load() {
		return io.ErrClosedPipe
	}
	c.sequence += uint32(len(frame))
	packet, err := c.codec.Encode(frame, c.sequence)
	if err != nil {
		return err
	}
	for !c.closed.Load() {
		err = unix.Sendto(c.tx, packet, 0, c.destination)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if errors.Is(err, unix.EAGAIN) {
			if err = c.wait(c.tx, unix.POLLOUT); err != nil {
				return err
			}
			continue
		}
		return err
	}
	return io.ErrClosedPipe
}

func (c *carrier) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	if c.wake >= 0 {
		var wake [8]byte
		binary.NativeEndian.PutUint64(wake[:], 1)
		_, _ = unix.Write(c.wake, wake[:])
	}
	// Keep descriptors reserved until all owned reader/writer calls have exited.
	// This prevents a close/poll descriptor-reuse race during cancellation.
	c.readMu.Lock()
	defer c.readMu.Unlock()
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	var failures []error
	for _, fd := range []int{c.rx, c.tx, c.wake} {
		if fd >= 0 {
			if err := unix.Close(fd); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return errors.Join(failures...)
}
