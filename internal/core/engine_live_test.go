// Live independent-peer forwarding regressions by ir_spoof; https://t.me/ir_spoof.
// These tests run only the new Engine against owned loopback sockets.
package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"backhaul.local/ir_spoof/internal/config"
)

type liveEngineResult struct {
	done chan struct{}
	err  error
}

func livePorts(t *testing.T, count int) []string {
	t.Helper()
	var listeners []net.Listener
	var addresses []string
	defer func() {
		for _, listener := range listeners {
			listener.Close()
		}
	}()
	for i := 0; i < count; i++ {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		listeners = append(listeners, listener)
		addresses = append(addresses, listener.Addr().String())
	}
	return addresses
}

func liveConfigPair(t *testing.T, transport, backend string, udp bool) (config.Config, config.Config, string, []string) {
	t.Helper()
	addresses := livePorts(t, 4)
	makeConfig := func() config.Config {
		c := config.Config{}
		c.Defaults()
		c.Security.Token = "ir_spoof independent Engine loopback test secret"
		c.Transport.Type = transport
		c.Transport.ConnectionPool = 1
		c.Transport.NoDelay = true
		c.Transport.HeartbeatInterval = 1
		c.Transport.HeartbeatTimeout = 5
		c.Dialer.DialTimeout = 3
		c.Dialer.RetryInterval = 1
		c.Tuning.MaxConnections = 1
		c.Tuning.ChannelSize = 2
		c.Tuning.ReadTimeout = 3
		c.Mux.Concurrency = 1
		return c
	}
	server, client := makeConfig(), makeConfig()
	server.Listener.BindAddr = addresses[0]
	server.Ports.Mapping = []string{addresses[1] + "=" + backend}
	server.Transport.AcceptUDP = udp
	client.Dialer.RemoteAddr = addresses[0]
	fmt.Sscanf(addresses[2], "127.0.0.1:%d", &server.TUN.HealthPort)
	fmt.Sscanf(addresses[3], "127.0.0.1:%d", &client.TUN.HealthPort)
	for _, c := range []config.Config{server, client} {
		if err := c.Validate(); err != nil {
			t.Fatal("live fixture configuration:", err)
		}
	}
	return server, client, addresses[1], addresses[2:]
}

func liveRun(t *testing.T, ctx context.Context, c config.Config) *liveEngineResult {
	t.Helper()
	outcome := &liveEngineResult{done: make(chan struct{})}
	go func() { outcome.err = Run(ctx, c); close(outcome.done) }()
	return outcome
}

func liveWaitPeers(t *testing.T, addresses []string, engines ...*liveEngineResult) {
	t.Helper()
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		for _, engine := range engines {
			select {
			case <-engine.done:
				t.Fatalf("Engine exited before ready: %v", engine.err)
			default:
			}
		}
		ready := true
		for _, address := range addresses {
			response, err := client.Get("http://" + address + "/health")
			if err != nil {
				ready = false
				break
			}
			var state struct {
				Ready    bool
				Sessions int
			}
			err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&state)
			response.Body.Close()
			if err != nil || response.StatusCode != 200 || !state.Ready || state.Sessions < 1 {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("independent peers did not establish an authenticated session")
}

func liveStop(t *testing.T, cancel context.CancelFunc, engines ...*liveEngineResult) {
	t.Helper()
	cancel()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for _, engine := range engines {
		select {
		case <-engine.done:
			if engine.err != nil {
				t.Errorf("Engine cancellation returned error: %v", engine.err)
			}
		case <-deadline.C:
			t.Error("Engine Run did not join its workers after cancellation")
			return
		}
	}
}

func TestEngineLiveHalfCloseWithSinglePeerAndFlowSlot(t *testing.T) {
	for _, transport := range []string{"tcp", "tcpmux"} {
		t.Run(transport, func(t *testing.T) {
			backend, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer backend.Close()
			request := bytes.Repeat([]byte("request must finish before response/"), 1700)
			response := bytes.Repeat([]byte("response-after-request-EOF/"), 3100)
			if len(response) < 74000 {
				t.Fatal("fixture response should cross multiple encrypted record boundaries")
			}
			served := make(chan error, 1)
			go func() {
				conn, err := backend.Accept()
				if err != nil {
					served <- err
					return
				}
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(6 * time.Second))
				got, err := io.ReadAll(conn)
				if err != nil {
					served <- err
					return
				}
				if !bytes.Equal(got, request) {
					served <- fmt.Errorf("backend request changed: %d bytes", len(got))
					return
				}
				// No reply is produced until the backend observes real TCP EOF.
				n, err := conn.Write(response)
				if err == nil && n != len(response) {
					err = io.ErrShortWrite
				}
				served <- err
			}()
			serverConfig, clientConfig, frontend, health := liveConfigPair(t, transport, backend.Addr().String(), false)
			ctx, cancel := context.WithCancel(context.Background())
			server := liveRun(t, ctx, serverConfig)
			client := liveRun(t, ctx, clientConfig)
			t.Cleanup(func() { liveStop(t, cancel, client, server) })
			liveWaitPeers(t, health, server, client)
			conn, err := net.DialTimeout("tcp", frontend, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(6 * time.Second))
			if _, err = conn.Write(request); err != nil {
				t.Fatal(err)
			}
			if err = conn.(*net.TCPConn).CloseWrite(); err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(conn)
			if err != nil || !bytes.Equal(got, response) {
				t.Fatalf("full backend response was lost after request half-close: bytes=%d err=%v", len(got), err)
			}
			if err := <-served; err != nil {
				t.Fatal("backend:", err)
			}
			conn.Close()
			// Reestablishing the spare peer also proves the single flow slot
			// was released, and gives cancellation a live idle carrier to join.
			liveWaitPeers(t, health, server, client)
			liveStop(t, cancel, client, server)
		})
	}
}

func TestEngineLiveUDPZeroAndLargeDatagramWithOneFlow(t *testing.T) {
	backend, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	served := make(chan error, 1)
	go func() {
		buffer := make([]byte, 65535)
		backend.SetDeadline(time.Now().Add(8 * time.Second))
		for i := 0; i < 2; i++ {
			n, source, err := backend.ReadFromUDP(buffer)
			if err != nil {
				served <- err
				return
			}
			sent, err := backend.WriteToUDP(buffer[:n], source)
			if err != nil {
				served <- err
				return
			}
			if sent != n {
				served <- io.ErrShortWrite
				return
			}
		}
		served <- nil
	}()
	serverConfig, clientConfig, frontend, health := liveConfigPair(t, "tcp", backend.LocalAddr().String(), true)
	serverConfig.UDP.FrameSize = 65535
	clientConfig.UDP.FrameSize = 65535
	ctx, cancel := context.WithCancel(context.Background())
	server := liveRun(t, ctx, serverConfig)
	client := liveRun(t, ctx, clientConfig)
	t.Cleanup(func() { liveStop(t, cancel, client, server) })
	liveWaitPeers(t, health, server, client)
	address, err := net.ResolveUDPAddr("udp", frontend)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialUDP("udp", nil, address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(6 * time.Second))
	for _, size := range []int{0, 60000} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i*37 + i/251)
		}
		if n, err := conn.Write(data); err != nil || n != len(data) {
			t.Fatalf("UDP send %d: n=%d err=%v", size, n, err)
		}
		buffer := make([]byte, 65535)
		n, err := conn.Read(buffer)
		if err != nil || !bytes.Equal(buffer[:n], data) {
			t.Fatalf("UDP echo %d changed: n=%d err=%v", size, n, err)
		}
	}
	if err := <-served; err != nil {
		t.Fatal("UDP backend:", err)
	}
	// The UDP association remains active here. Cancellation must close its
	// carrier, backend and reply worker without waiting for the idle timeout.
	liveStop(t, cancel, client, server)
}
