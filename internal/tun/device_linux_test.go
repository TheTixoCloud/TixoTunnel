//go:build linux

// TUN ownership and cancellation checks by t.me/ir_spoof.
package tun

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestInvalidSettings(t *testing.T) {
	tests := []struct {
		name, local string
		mtu, queues int
		routes      []string
	}{
		{"bad;name", "198.18.0.1/30", 1400, 1, nil},
		{"bhtest0", "198.18.0.1", 1400, 1, nil},
		{"bhtest0", "0.0.0.0/30", 1400, 1, nil},
		{"bhtest0", "198.18.0.1/30", 1400, 0, nil},
		{"bhtest0", "198.18.0.1/30", 1400, 65, nil},
		{"bhtest0", "198.18.0.1/30", 65536, 1, nil},
		{"bhtest0", "fd00::1/64", 1000, 1, nil},
		{"bhtest0", "198.18.0.1/30", 1400, 1, []string{"invalid-route"}},
	}
	for _, test := range tests {
		d, err := openDevice(test.name, test.local, test.mtu, test.queues, test.routes)
		if d != nil || err == nil {
			if d != nil {
				_ = d.Close()
			}
			t.Fatal("invalid settings must fail before device creation")
		}
	}
}

func requireIsolatedLab(t *testing.T) {
	t.Helper()
	if os.Getenv("BACKHAUL_TUN_LAB") != "1" || os.Geteuid() != 0 {
		t.Skip("privileged TUN checks require BACKHAUL_TUN_LAB=1 inside a disposable network namespace")
	}
	self, err := os.Stat("/proc/self/ns/net")
	init, initErr := os.Stat("/proc/1/ns/net")
	if err != nil || initErr != nil || os.SameFile(self, init) {
		t.Skip("refusing privileged checks in the system network namespace")
	}
}

func ipOutput(t *testing.T, args ...string) string {
	t.Helper()
	ip, err := findIP()
	if err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(ip, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("isolated ip command failed: %v", err)
	}
	return string(output)
}

func TestDeviceLifecycleAndPreexistingProtection(t *testing.T) {
	requireIsolatedLab(t)
	d, err := openDevice("bhtest0", "198.18.0.1/30", 1400, 2, []string{"198.19.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	iface, err := net.InterfaceByName(d.Name())
	if err != nil || iface.MTU != 1400 || iface.Flags&net.FlagUp == 0 {
		t.Fatal("new interface must have the configured MTU and be up")
	}
	addresses, err := iface.Addrs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, address := range addresses {
		found = found || address.String() == "198.18.0.1/30"
	}
	if !found {
		t.Fatal("configured local address is absent")
	}
	if !strings.Contains(ipOutput(t, "-4", "route", "show", "198.19.0.0/24"), "dev bhtest0") {
		t.Fatal("new route must use the owned interface")
	}
	other, err := openDevice("bhtest0", "198.18.0.2/30", 1500, 1, nil)
	if err == nil || other != nil {
		t.Fatal("opening a preexisting named interface must fail")
	}
	unchanged, err := net.InterfaceByName("bhtest0")
	if err != nil || unchanged.Index != iface.Index || unchanged.MTU != 1400 {
		t.Fatal("rejecting a preexisting interface must not mutate it")
	}
	if d.Queue(2) != nil || d.Queue(-1) != nil {
		t.Fatal("invalid queue indexes must be rejected")
	}
	if n, err := d.Queue(0).Write(make([]byte, MaxPacketSize+1)); n != 0 || err == nil {
		t.Fatal("oversized packets must not reach the kernel")
	}
	finished := make(chan error, 2)
	for i := 0; i < 2; i++ {
		queue := d.Queue(i)
		go func() {
			buffer := make([]byte, MaxPacketSize)
			for {
				_, err := queue.Read(buffer)
				if err != nil {
					finished <- err
					return
				}
			}
		}()
	}
	time.Sleep(25 * time.Millisecond)
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case err := <-finished:
			if !errors.Is(err, os.ErrClosed) {
				t.Fatalf("closed queue must wake a reader with closed-file error: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("closing TUN must cancel blocked reads")
		}
	}
	if _, err := net.InterfaceByName("bhtest0"); err == nil {
		t.Fatal("closing every queue must remove the nonpersistent interface")
	}
	if strings.Contains(ipOutput(t, "-4", "route", "show", "198.19.0.0/24"), "bhtest0") {
		t.Fatal("owned route must be gone after close")
	}
	if err := d.Close(); err != nil {
		t.Fatal("closing twice must be safe")
	}
}

func TestRollbackKeepsExistingRoute(t *testing.T) {
	requireIsolatedLab(t)
	ipOutput(t, "link", "add", "name", "nxkeep0", "type", "dummy")
	t.Cleanup(func() { ipOutput(t, "link", "delete", "dev", "nxkeep0") })
	ipOutput(t, "link", "set", "dev", "nxkeep0", "up")
	ipOutput(t, "-4", "route", "add", "198.19.1.0/24", "dev", "nxkeep0")
	d, err := openDevice("nxtest1", "198.18.0.5/30", 1400, 1, []string{"198.19.1.0/24"})
	if err == nil || d != nil {
		if d != nil {
			_ = d.Close()
		}
		t.Fatal("route collision must fail without replacing an unrelated route")
	}
	if _, err := net.InterfaceByName("nxtest1"); err == nil {
		t.Fatal("failed configuration must remove its newly created interface")
	}
	if !strings.Contains(ipOutput(t, "-4", "route", "show", "198.19.1.0/24"), "dev nxkeep0") {
		t.Fatal("rollback must retain the preexisting route")
	}
}

func TestPacketReadRejectsUndersizedDestination(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	queue := &packetQueue{file: reader}
	t.Cleanup(func() { _ = queue.Close(); _ = writer.Close() })
	go func() { _, _ = writer.Write(make([]byte, 1000)) }()
	if n, err := queue.Read(make([]byte, 16)); n != 0 || !errors.Is(err, io.ErrShortBuffer) {
		t.Fatal("a packet must not be silently truncated to the caller's buffer")
	}
}

func TestContextCancellationAdapter(t *testing.T) {
	requireIsolatedLab(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d, err := Open(ctx, Config{Name: "bhcancel0", Local: "198.18.0.9/30", Peer: "198.18.0.10", MTU: 1320})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = d.Close() })
	finished := make(chan error, 1)
	go func() {
		buffer := make([]byte, MaxPacketSize)
		for {
			_, err := d.Read(buffer)
			if err != nil {
				finished <- err
				return
			}
		}
	}()
	cancel()
	select {
	case err := <-finished:
		if !errors.Is(err, os.ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("context cancellation failed to unblock TUN reader")
	}
	if _, err := net.InterfaceByName(d.Name()); err == nil {
		t.Fatal("context cancellation retained the owned interface")
	}
}
