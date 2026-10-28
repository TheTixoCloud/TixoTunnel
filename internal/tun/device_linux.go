//go:build linux

// Independent Linux TUN implementation by t.me/ir_spoof.
package tun

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// MaxPacketSize bounds one complete packet, including its IP header.
const MaxPacketSize = 65535

// Locally added routes have a numeric protocol marker so cleanup cannot delete
// an unrelated route with the same prefix. Route addition never uses replace.
const ownedRouteProtocol = "253"

var validName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,14}$`)

type ownedRoute struct {
	family string
	prefix string
}

// Device owns only its newly created, nonpersistent interface and queue files.
// It must stay in the network namespace in which Open was called.
type Device struct {
	name        string
	index       int
	ipCommand   string
	queues      []*packetQueue
	routes      []ownedRoute
	closeOnce   sync.Once
	closeErr    error
	stopContext func() bool
	contextMu   sync.Mutex
	closed      bool
}

type packetQueue struct {
	file      *os.File
	readMu    sync.Mutex
	buffer    [MaxPacketSize]byte
	closeOnce sync.Once
	closeErr  error
}

// Open creates a new Linux TUN with no packet-info header. Existing interface
// names are rejected, including creation races caught by IFF_TUN_EXCL. All
// configuration is validated before opening a device or invoking ip.
func openDevice(name, local string, mtu, queues int, routes []string) (*Device, error) {
	if !validName.MatchString(name) {
		return nil, errors.New("invalid TUN name: use 1..15 ASCII letters, digits, dots, underscores or hyphens, beginning with a letter")
	}
	address, err := netip.ParsePrefix(local)
	if err != nil || address.Addr().Is4In6() || address.Addr().IsUnspecified() || address.Addr().IsMulticast() {
		return nil, errors.New("TUN local address must be an IP address with a prefix")
	}
	if mtu < 576 || mtu > MaxPacketSize || address.Addr().Is6() && mtu < 1280 {
		return nil, errors.New("invalid TUN MTU; IPv4 requires 576..65535 and IPv6 requires 1280..65535")
	}
	if queues < 1 || queues > 64 || len(routes) > 256 {
		return nil, errors.New("TUN allows 1..64 queues and at most 256 routes")
	}
	var normalized []ownedRoute
	seen := make(map[string]bool)
	for _, value := range routes {
		prefix, parseErr := netip.ParsePrefix(value)
		if parseErr != nil || prefix.Addr().Is4In6() {
			return nil, errors.New("invalid TUN route prefix")
		}
		prefix = prefix.Masked()
		// ip address add owns the directly connected prefix automatically.
		if prefix == address.Masked() || seen[prefix.String()] {
			continue
		}
		seen[prefix.String()] = true
		family := "-6"
		if prefix.Addr().Is4() {
			family = "-4"
		}
		normalized = append(normalized, ownedRoute{family: family, prefix: prefix.String()})
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.New("cannot inspect local interfaces")
	}
	for _, iface := range interfaces {
		if iface.Name == name {
			return nil, errors.New("TUN interface name already exists; choose an unused name")
		}
	}
	ipCommand, err := findIP()
	if err != nil {
		return nil, err
	}
	d := &Device{name: name, ipCommand: ipCommand}
	rollback := func(cause error) (*Device, error) {
		return nil, errors.Join(cause, d.Close())
	}
	for i := 0; i < queues; i++ {
		flags := uint16(unix.IFF_TUN | unix.IFF_NO_PI)
		if queues > 1 {
			flags |= unix.IFF_MULTI_QUEUE
		}
		if i == 0 {
			flags |= unix.IFF_TUN_EXCL
		}
		fd, openErr := unix.Open("/dev/net/tun", unix.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
		if openErr != nil {
			return rollback(fmt.Errorf("open Linux TUN device: %w", openErr))
		}
		request, requestErr := unix.NewIfreq(name)
		if requestErr != nil {
			_ = unix.Close(fd)
			return rollback(errors.New("invalid TUN ioctl request"))
		}
		request.SetUint16(flags)
		if ioctlErr := unix.IoctlIfreq(fd, unix.TUNSETIFF, request); ioctlErr != nil {
			_ = unix.Close(fd)
			return rollback(fmt.Errorf("create Linux TUN queue: %w", ioctlErr))
		}
		file := os.NewFile(uintptr(fd), "/dev/net/tun")
		if file == nil {
			_ = unix.Close(fd)
			return rollback(errors.New("cannot initialize TUN queue"))
		}
		d.queues = append(d.queues, &packetQueue{file: file})
		iface, lookupErr := net.InterfaceByName(name)
		if lookupErr != nil || request.Name() != name {
			return rollback(errors.New("cannot verify newly created TUN interface"))
		}
		if i == 0 {
			d.index = iface.Index
		} else if iface.Index != d.index {
			return rollback(errors.New("TUN interface changed during queue creation"))
		}
	}
	if err = d.ip("link", "set", "dev", name, "mtu", strconv.Itoa(mtu)); err != nil {
		return rollback(fmt.Errorf("configure TUN MTU: %w", err))
	}
	if err = d.ip("address", "add", address.String(), "dev", name); err != nil {
		return rollback(fmt.Errorf("configure TUN address: %w", err))
	}
	if err = d.ip("link", "set", "dev", name, "up"); err != nil {
		return rollback(fmt.Errorf("activate TUN interface: %w", err))
	}
	for _, route := range normalized {
		if err = d.ip(route.family, "route", "add", route.prefix, "dev", name, "proto", ownedRouteProtocol); err != nil {
			return rollback(fmt.Errorf("add TUN route without replacing an existing route: %w", err))
		}
		d.routes = append(d.routes, route)
	}
	return d, nil
}

// Name returns the requested, verified interface name.
func (d *Device) Name() string {
	if d == nil {
		return ""
	}
	return d.name
}

// Queue returns nil for an invalid index. Reads and writes preserve packet
// boundaries; every Write must contain one complete IP packet.
func (d *Device) Queue(i int) io.ReadWriteCloser {
	if d == nil || i < 0 || i >= len(d.queues) {
		return nil
	}
	return d.queues[i]
}

// Close immediately wakes blocked queue readers. The nonpersistent device
// disappears when its last descriptor closes; Linux removes its connected and
// device-bound routes. If another privileged process attached to this owned
// interface, cleanup deletes only routes explicitly added by this Device and
// only while the original interface index still matches.
func (d *Device) Close() error {
	if d == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.contextMu.Lock()
		d.closed = true
		stop := d.stopContext
		d.contextMu.Unlock()
		if stop != nil {
			stop()
		}
		var failures []error
		for _, queue := range d.queues {
			if err := queue.Close(); err != nil {
				failures = append(failures, err)
			}
		}
		iface, err := net.InterfaceByName(d.name)
		if err == nil && iface.Index == d.index {
			for _, route := range d.routes {
				if err := d.ip(route.family, "route", "del", route.prefix, "dev", d.name, "proto", ownedRouteProtocol); err != nil {
					failures = append(failures, fmt.Errorf("delete owned TUN route: %w", err))
				}
			}
		}
		d.closeErr = errors.Join(failures...)
	})
	return d.closeErr
}

func (q *packetQueue) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	q.readMu.Lock()
	defer q.readMu.Unlock()
	n, err := q.file.Read(q.buffer[:])
	if n > len(p) {
		return 0, io.ErrShortBuffer
	}
	copy(p, q.buffer[:n])
	return n, err
}

func (q *packetQueue) Write(p []byte) (int, error) {
	if len(p) > MaxPacketSize {
		return 0, errors.New("TUN packet exceeds 65535 bytes")
	}
	if len(p) == 0 {
		return 0, nil
	}
	n, err := q.file.Write(p)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}

func (q *packetQueue) Close() error {
	q.closeOnce.Do(func() { q.closeErr = q.file.Close() })
	return q.closeErr
}

func (d *Device) ip(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, d.ipCommand, args...)
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	return command.Run()
}

func findIP() (string, error) {
	// Use only conventional system paths, never a user-controlled PATH entry.
	for _, path := range []string{"/usr/sbin/ip", "/sbin/ip", "/usr/bin/ip", "/bin/ip"} {
		info, err := os.Stat(path)
		if err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			return path, nil
		}
	}
	return "", errors.New("Linux iproute2 is required for TUN configuration")
}
