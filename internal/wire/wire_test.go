// Session security and end-to-end stream regressions by ir_spoof; https://t.me/ir_spoof.
package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type observedConn struct {
	net.Conn
	mu      sync.Mutex
	written []byte
}

func (c *observedConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	c.mu.Lock()
	c.written = append(c.written, p[:n]...)
	c.mu.Unlock()
	return n, err
}
func (c *observedConn) reset() { c.mu.Lock(); c.written = nil; c.mu.Unlock() }
func (c *observedConn) bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.written...)
}

func testOptions(algorithm string) Options {
	return Options{Token: "ir_spoof independent local session test secret", Transport: "tcp", Algorithm: algorithm,
		Encrypted: true, Timeout: 500 * time.Millisecond}
}

func sessionPair(t *testing.T, options Options) (net.Conn, net.Conn, *observedConn) {
	t.Helper()
	rawClient, rawServer := net.Pipe()
	watched := &observedConn{Conn: rawClient}
	t.Cleanup(func() { rawClient.Close(); rawServer.Close() })
	type result struct {
		conn net.Conn
		err  error
	}
	ready := make(chan result, 1)
	go func() { c, err := Handshake(rawServer, false, options); ready <- result{c, err} }()
	client, err := Handshake(watched, true, options)
	if err != nil {
		t.Fatal("client handshake:", err)
	}
	var server net.Conn
	select {
	case outcome := <-ready:
		if outcome.err != nil {
			t.Fatal("server handshake:", outcome.err)
		}
		server = outcome.conn
	case <-time.After(time.Second):
		t.Fatal("server handshake did not finish")
	}
	client.SetDeadline(time.Now().Add(3 * time.Second))
	server.SetDeadline(time.Now().Add(3 * time.Second))
	watched.reset()
	return client, server, watched
}

func TestAuthenticatedAlgorithmsPreserveHalfCloseReplies(t *testing.T) {
	for _, algorithm := range []string{"aes-128-gcm", "aes-256-gcm", "chacha20-poly1305"} {
		t.Run(algorithm, func(t *testing.T) {
			client, server, watched := sessionPair(t, testOptions(algorithm))
			request := bytes.Repeat([]byte("ir_spoof request body/"), 6000)
			response := bytes.Repeat([]byte("response only after request EOF/"), 3000)
			completion := make(chan error, 1)
			go func() {
				got, err := io.ReadAll(server)
				if err != nil {
					completion <- err
					return
				}
				if !bytes.Equal(got, request) {
					completion <- errors.New("request body differs")
					return
				}
				// A protocol EOF remains EOF. A second read must not consume
				// or wait for an unrelated future record after the half-close.
				var one [1]byte
				if n, err := server.Read(one[:]); n != 0 || err != io.EOF {
					completion <- errors.New("EOF was not persistent")
					return
				}
				if _, err := server.Write(response); err != nil {
					completion <- err
					return
				}
				completion <- server.(interface{ CloseWrite() error }).CloseWrite()
			}()
			sender := make(chan error, 1)
			go func() {
				for position := 0; position < len(request); {
					end := position + 509
					if end > len(request) {
						end = len(request)
					}
					if _, err := client.Write(request[position:end]); err != nil {
						sender <- err
						return
					}
					position = end
				}
				sender <- client.(interface{ CloseWrite() error }).CloseWrite()
			}()
			got, err := io.ReadAll(client)
			if err != nil || !bytes.Equal(got, response) {
				t.Fatalf("half-close response lost: bytes=%d error=%v", len(got), err)
			}
			if err := <-sender; err != nil {
				t.Fatal("sender:", err)
			}
			if err := <-completion; err != nil {
				t.Fatal("server:", err)
			}
			if bytes.Contains(watched.bytes(), []byte("ir_spoof request body/")) {
				t.Fatal("plaintext request was visible on the encrypted carrier")
			}
			if _, err := client.Write([]byte("after EOF")); err == nil {
				t.Fatal("write after local half-close was accepted")
			}
		})
	}
}

func TestConcurrentWritesRemainWholeAndBounded(t *testing.T) {
	client, server, _ := sessionPair(t, testOptions("aes-256-gcm"))
	const writers = 8
	size := MaxRecord*2 + 17
	result := make(chan []byte, 1)
	readError := make(chan error, 1)
	go func() { data, err := io.ReadAll(server); result <- data; readError <- err }()
	var group sync.WaitGroup
	failures := make(chan error, writers)
	for i := 0; i < writers; i++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			data := bytes.Repeat([]byte{byte(id + 1)}, size)
			n, err := client.Write(data)
			if err != nil {
				failures <- err
			} else if n != len(data) {
				failures <- io.ErrShortWrite
			}
		}(i)
	}
	group.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	if err := client.(interface{ CloseWrite() error }).CloseWrite(); err != nil {
		t.Fatal(err)
	}
	data := <-result
	if err := <-readError; err != nil {
		t.Fatal(err)
	}
	if len(data) != writers*size {
		t.Fatalf("got %d bytes; expected %d", len(data), writers*size)
	}
	seen := map[byte]bool{}
	for offset := 0; offset < len(data); offset += size {
		id := data[offset]
		if id < 1 || id > writers || seen[id] || !bytes.Equal(data[offset:offset+size], bytes.Repeat([]byte{id}, size)) {
			t.Fatal("concurrent writes interleaved, duplicated, or changed")
		}
		seen[id] = true
	}
}

func TestHandshakeRejectsDifferentSecretsOptionsAndRoles(t *testing.T) {
	for _, name := range []string{"token", "transport", "algorithm", "encryption", "both-client", "both-server"} {
		t.Run(name, func(t *testing.T) {
			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()
			a, b := testOptions("aes-256-gcm"), testOptions("aes-256-gcm")
			leftRole, rightRole := true, false
			switch name {
			case "token":
				b.Token = "another unrelated long local session secret"
			case "transport":
				b.Transport = "tcpmux"
			case "algorithm":
				b.Algorithm = "aes-128-gcm"
			case "encryption":
				b.Encrypted = false
			case "both-client":
				rightRole = true
			case "both-server":
				leftRole = false
			}
			results := make(chan error, 2)
			go func() { _, err := Handshake(left, leftRole, a); left.Close(); results <- err }()
			go func() { _, err := Handshake(right, rightRole, b); right.Close(); results <- err }()
			for i := 0; i < 2; i++ {
				select {
				case err := <-results:
					if err == nil {
						t.Fatal("incompatible handshake succeeded")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("incompatible handshake did not fail within its deadline")
				}
			}
		})
	}
}

func TestSeparateSessionsUseFreshCiphertext(t *testing.T) {
	data := bytes.Repeat([]byte("same application bytes"), 200)
	var captures [][]byte
	for i := 0; i < 2; i++ {
		client, server, watched := sessionPair(t, testOptions("chacha20-poly1305"))
		done := make(chan error, 1)
		go func() { _, err := client.Write(data); done <- err }()
		got := make([]byte, len(data))
		if _, err := io.ReadFull(server, got); err != nil || !bytes.Equal(got, data) {
			t.Fatal("independent session transfer failed")
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		captures = append(captures, watched.bytes())
		client.Close()
		server.Close()
	}
	if len(captures[0]) == 0 || bytes.Equal(captures[0], captures[1]) {
		t.Fatal("separate sessions reused the same ciphertext")
	}
}

type alteredConn struct {
	net.Conn
	mu       sync.Mutex
	alter    bool
	original []byte
}

func (c *alteredConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	active := c.alter
	if active {
		c.original = append(c.original, p...)
	}
	c.mu.Unlock()
	data := p
	if active && len(p) > 4 {
		data = append([]byte(nil), p...)
		data[len(data)-1] ^= 1
	}
	return c.Conn.Write(data)
}

func TestAuthenticationFailurePermanentlyStopsThatStream(t *testing.T) {
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	modified := &alteredConn{Conn: left}
	options := testOptions("aes-256-gcm")
	type outcome struct {
		conn net.Conn
		err  error
	}
	accepted := make(chan outcome, 1)
	go func() { conn, err := Handshake(right, false, options); accepted <- outcome{conn, err} }()
	client, err := Handshake(modified, true, options)
	if err != nil {
		t.Fatal(err)
	}
	server := <-accepted
	if server.err != nil {
		t.Fatal(server.err)
	}
	client.SetDeadline(time.Now().Add(time.Second))
	server.conn.SetDeadline(time.Now().Add(time.Second))
	modified.mu.Lock()
	modified.alter = true
	modified.mu.Unlock()
	sent := make(chan error, 1)
	go func() { _, err := client.Write([]byte("authenticated application request")); sent <- err }()
	buffer := make([]byte, 128)
	if n, err := server.conn.Read(buffer); n != 0 || err == nil {
		t.Fatal("damaged authenticated data was accepted")
	}
	<-sent
	// Replay the unmodified first record after the authentication error. A
	// caller retry must never recover application plaintext from a poisoned
	// stream, even when that later record would otherwise authenticate.
	modified.mu.Lock()
	original := append([]byte(nil), modified.original...)
	modified.mu.Unlock()
	replayed := make(chan struct{})
	go func() { left.Write(original); close(replayed) }()
	if n, err := server.conn.Read(buffer); n != 0 || err == nil {
		t.Fatal("stream resumed after authentication failure")
	}
	left.Close()
	select {
	case <-replayed:
	case <-time.After(time.Second):
		t.Fatal("replay writer did not terminate")
	}
}

func assertPoisoned(t *testing.T, conn net.Conn, first error) {
	t.Helper()
	if first == nil || first == io.EOF {
		t.Fatal("expected a terminal protocol error")
	}
	var buffer [1]byte
	if n, err := conn.Read(buffer[:]); n != 0 || err != first {
		t.Fatalf("read lost terminal error: n=%d err=%v first=%v", n, err, first)
	}
	if n, err := conn.Write([]byte("retry")); n != 0 || err != first {
		t.Fatalf("write lost terminal error: n=%d err=%v first=%v", n, err, first)
	}
	if err := conn.(interface{ CloseWrite() error }).CloseWrite(); err != first {
		t.Fatal("half-close lost terminal error")
	}
}

func TestInvalidRecordLengthsFailClosed(t *testing.T) {
	for _, size := range []uint32{0, 15, MaxRecord + 17, ^uint32(0)} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			client, server, _ := sessionPair(t, testOptions("aes-256-gcm"))
			var header [4]byte
			binary.BigEndian.PutUint32(header[:], size)
			done := make(chan struct{})
			go func() { client.(*recordConn).Conn.Write(header[:]); close(done) }()
			var buffer [1]byte
			n, err := server.Read(buffer[:])
			if n != 0 {
				t.Fatal("malformed record produced plaintext")
			}
			assertPoisoned(t, server, err)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("malformed writer was not unblocked")
			}
		})
	}
}

func TestTransportTruncationCannotBecomeAuthenticatedEOF(t *testing.T) {
	for _, name := range []string{"before-header", "partial-header", "partial-body"} {
		t.Run(name, func(t *testing.T) {
			client, server, _ := sessionPair(t, testOptions("chacha20-poly1305"))
			done := make(chan struct{})
			go func() {
				raw := client.(*recordConn).Conn
				switch name {
				case "partial-header":
					raw.Write([]byte{0, 0})
				case "partial-body":
					raw.Write([]byte{0, 0, 0, 20})
					raw.Write([]byte{1, 2, 3})
				}
				raw.Close()
				close(done)
			}()
			var buffer [1]byte
			n, err := server.Read(buffer[:])
			if n != 0 || err != io.ErrUnexpectedEOF {
				t.Fatalf("truncation became clean EOF/plaintext: n=%d err=%v", n, err)
			}
			assertPoisoned(t, server, err)
			<-done
		})
	}
}

var errPartialWrite = errors.New("injected carrier write failure")

type partialWriteConn struct {
	net.Conn
	remaining int
	zero      bool
}

func (c *partialWriteConn) Write(p []byte) (int, error) {
	if c.zero {
		return 0, nil
	}
	if len(p) <= c.remaining {
		n, err := c.Conn.Write(p)
		c.remaining -= n
		return n, err
	}
	if c.remaining == 0 {
		return 0, errPartialWrite
	}
	n, err := c.Conn.Write(p[:c.remaining])
	c.remaining -= n
	if err != nil {
		return n, err
	}
	return n, errPartialWrite
}

func TestPartialRecordWritesPermanentlyCloseBothDirections(t *testing.T) {
	for _, name := range []string{"partial-header", "partial-body", "zero-progress"} {
		t.Run(name, func(t *testing.T) {
			client, server, _ := sessionPair(t, testOptions("aes-128-gcm"))
			r := client.(*recordConn)
			fault := &partialWriteConn{Conn: r.Conn, remaining: 2}
			if name == "partial-body" {
				fault.remaining = 7
			}
			if name == "zero-progress" {
				fault.zero = true
			}
			r.Conn = fault
			finished := make(chan error, 1)
			go func() { _, err := io.ReadAll(server); finished <- err }()
			n, err := client.Write(bytes.Repeat([]byte("body"), 200))
			if n != 0 {
				t.Fatal("partial encrypted record reported complete plaintext")
			}
			assertPoisoned(t, client, err)
			select {
			case err := <-finished:
				if err == nil || err == io.EOF {
					t.Fatal("peer accepted a partial record as EOF")
				}
			case <-time.After(time.Second):
				t.Fatal("peer read stayed blocked after write failure")
			}
		})
	}
}

func TestCounterExhaustionIsTerminal(t *testing.T) {
	for _, direction := range []string{"read", "write"} {
		t.Run(direction, func(t *testing.T) {
			client, _, _ := sessionPair(t, testOptions("aes-256-gcm"))
			r := client.(*recordConn)
			var err error
			if direction == "read" {
				r.rx = ^uint64(0)
				_, err = r.Read(make([]byte, 1))
			} else {
				r.tx = ^uint64(0)
				_, err = r.Write([]byte("no wrapped nonce"))
			}
			assertPoisoned(t, client, err)
		})
	}
}

type untouchedConn struct{ touched bool }

func (c *untouchedConn) Read([]byte) (int, error)         { c.touched = true; return 0, io.EOF }
func (c *untouchedConn) Write([]byte) (int, error)        { c.touched = true; return 0, io.ErrClosedPipe }
func (c *untouchedConn) Close() error                     { c.touched = true; return nil }
func (c *untouchedConn) LocalAddr() net.Addr              { c.touched = true; return nil }
func (c *untouchedConn) RemoteAddr() net.Addr             { c.touched = true; return nil }
func (c *untouchedConn) SetDeadline(time.Time) error      { c.touched = true; return nil }
func (c *untouchedConn) SetReadDeadline(time.Time) error  { c.touched = true; return nil }
func (c *untouchedConn) SetWriteDeadline(time.Time) error { c.touched = true; return nil }

func TestConstructorRejectsInvalidOptionsBeforeCarrierIO(t *testing.T) {
	for _, name := range []string{"short-token", "oversized-token", "algorithm", "transport", "negative-timeout", "oversized-timeout"} {
		t.Run(name, func(t *testing.T) {
			o := testOptions("aes-256-gcm")
			switch name {
			case "short-token":
				o.Token = "short"
			case "oversized-token":
				o.Token = string(bytes.Repeat([]byte{'x'}, 4097))
			case "algorithm":
				o.Algorithm = "aes-gcm-typo"
			case "transport":
				o.Transport = "unknown"
			case "negative-timeout":
				o.Timeout = -time.Second
			case "oversized-timeout":
				o.Timeout = 6 * time.Minute
			}
			for _, role := range []bool{true, false} {
				conn := new(untouchedConn)
				if _, err := Handshake(conn, role, o); err == nil {
					t.Fatal("invalid constructor input was accepted")
				}
				if conn.touched {
					t.Fatal("invalid options touched the carrier")
				}
			}
		})
	}
	if _, err := Handshake(nil, true, testOptions("aes-256-gcm")); err == nil {
		t.Fatal("nil carrier was accepted")
	}
}
