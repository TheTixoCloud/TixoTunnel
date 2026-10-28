// Independent carrier and privacy regressions by ir_spoof - t.me/ir_spoof.
package transport

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func certificates(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		DNSNames: []string{"localhost", "edge-identity.invalid"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	private, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	certPath, keyPath := filepath.Join(folder, "cert.pem"), filepath.Join(folder, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: private}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func echoListener(t *testing.T, o Options) net.Listener {
	t.Helper()
	listener, err := Listen(context.Background(), o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); io.Copy(c, c) }()
		}
	}()
	return listener
}

func exchange(t *testing.T, c net.Conn, payload []byte) {
	t.Helper()
	if err := c.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	written := make(chan error, 1)
	go func() { _, err := writeAll(c, payload); written <- err }()
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(c, got); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, got) {
		t.Fatal("echo content changed")
	}
}

func TestAllTenCarriersEcho(t *testing.T) {
	cert, key := certificates(t)
	for _, kind := range []string{"tcp", "tcpmux", "xtcpmux", "ws", "wss", "wsmux", "wssmux", "xwsmux", "anytls", "tun"} {
		t.Run(kind, func(t *testing.T) {
			server := echoListener(t, Options{Kind: kind, Listen: "127.0.0.1:0", CertFile: cert, KeyFile: key, NoDelay: true, DialTimeout: 2 * time.Second})
			c, err := Dial(context.Background(), Options{Kind: kind, Peer: server.Addr().String(), CAFile: cert, SNI: "localhost", LocalAddr: "127.0.0.1:0", NoDelay: true, DialTimeout: 2 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			exchange(t, c, bytes.Repeat([]byte("carrier-binary-\x00\xff"), 8192))
		})
	}
}

func TestTLSVerificationAndEdgeIdentity(t *testing.T) {
	cert, key := certificates(t)
	for _, kind := range []string{"wss", "wssmux", "anytls"} {
		t.Run(kind, func(t *testing.T) {
			server := echoListener(t, Options{Kind: kind, Listen: "127.0.0.1:0", CertFile: cert, KeyFile: key})
			_, port, _ := net.SplitHostPort(server.Addr().String())
			base := Options{Kind: kind, Peer: net.JoinHostPort("edge-identity.invalid", port), EdgeIP: "127.0.0.1", DialTimeout: time.Second}
			if c, err := Dial(context.Background(), base); err == nil {
				c.Close()
				t.Fatal("untrusted self-signed certificate accepted")
			}
			base.CAFile = cert
			base.SNI = "wrong-name.invalid"
			if c, err := Dial(context.Background(), base); err == nil {
				c.Close()
				t.Fatal("wrong TLS hostname accepted")
			}
			base.SNI = ""
			c, err := Dial(context.Background(), base)
			if err != nil {
				t.Fatal("edge dial lost peer TLS identity:", err)
			}
			defer c.Close()
			exchange(t, c, []byte("edge routes locally, identity stays the peer"))
		})
	}
}

func TestSlowTLSHandshakesDoNotSerializeAccept(t *testing.T) {
	cert, key := certificates(t)
	server := echoListener(t, Options{Kind: "anytls", Listen: "127.0.0.1:0", CertFile: cert, KeyFile: key, DialTimeout: 3 * time.Second})
	for i := 0; i < 4; i++ {
		c, err := net.Dial("tcp", server.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
	}
	start := time.Now()
	c, err := Dial(context.Background(), Options{Kind: "anytls", Peer: server.Addr().String(), CAFile: cert, DialTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	exchange(t, c, []byte("valid peer bypasses stalled handshakes"))
	if time.Since(start) > 2*time.Second {
		t.Fatal("stalled handshakes serialized the listener")
	}
}

func TestAssignedSourceValidationAndLiteralOnly(t *testing.T) {
	for _, value := range []string{"localhost", "0.0.0.0", "224.0.0.1", "203.0.113.254", "127.0.0.1:65536", "127.0.0.1:bad"} {
		t.Run(value, func(t *testing.T) {
			if _, err := assignedLocal(value); err == nil {
				t.Fatal("invalid/unassigned source accepted")
			}
		})
	}
	for _, value := range []string{"127.0.0.1", "127.0.0.1:0"} {
		got, err := assignedLocal(value)
		if err != nil || !got.IP.Equal(net.ParseIP("127.0.0.1")) {
			t.Fatal("assigned loopback rejected")
		}
	}
}

func TestStrictCarrierOptionsAndListenerCancellation(t *testing.T) {
	for _, o := range []Options{{Kind: "anything"}, {Kind: "tcp"}, {Kind: "tcp", MSS: 65536}, {Kind: "ws", ReadBuffer: -1}} {
		if _, err := Listen(context.Background(), o); err == nil {
			t.Fatal("invalid carrier options accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	l, err := Listen(ctx, Options{Kind: "wsmux", Listen: "127.0.0.1:0", DialTimeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	stalled, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Close()
	result := make(chan error, 1)
	go func() { _, err := l.Accept(); result <- err }()
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("context did not unblock Accept")
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func fixedRequest(path string) string {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 16))
	return "GET " + path + " HTTP/1.1\r\nHost: example.invalid:8443\r\nUpgrade: websocket\r\nConnection: keep-alive, Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: " + key + "\r\n\r\n"
}

func TestRawServerUpgradePreservesPipelinedApplicationBytes(t *testing.T) {
	for _, kind := range []string{"wsmux", "wssmux", "xwsmux"} {
		t.Run(kind, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			a.SetDeadline(time.Now().Add(time.Second))
			b.SetDeadline(time.Now().Add(time.Second))
			payload := []byte("BH4\x00pipelined-unframed-record")
			errCh := make(chan error, 1)
			go func() {
				c, err := serverUpgrade(a, Options{Kind: kind})
				if err != nil {
					errCh <- err
					return
				}
				got := make([]byte, len(payload))
				_, err = io.ReadFull(c, got)
				if err == nil && !bytes.Equal(got, payload) {
					err = errors.New("pipelined application bytes lost")
				}
				errCh <- err
			}()
			if _, err := b.Write(append([]byte(fixedRequest(upgradePath(kind))), payload...)); err != nil {
				t.Fatal(err)
			}
			if _, err := headerBlock(bufio.NewReader(b)); err != nil {
				t.Fatal(err)
			}
			if err := <-errCh; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestClientUpgradePreservesServerLookaheadAndHost(t *testing.T) {
	for _, kind := range []string{"ws", "wsmux", "xwsmux"} {
		t.Run(kind, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			a.SetDeadline(time.Now().Add(time.Second))
			b.SetDeadline(time.Now().Add(time.Second))
			payload := []byte("read-after-101")
			errCh := make(chan error, 1)
			go func() {
				reader := bufio.NewReader(b)
				header, err := headerBlock(reader)
				if err != nil {
					errCh <- err
					return
				}
				req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(header)))
				if err != nil {
					errCh <- err
					return
				}
				if req.Host != "edge-identity.invalid:8443" || req.RequestURI != upgradePath(kind) {
					errCh <- errors.New("peer HTTP identity/path changed")
					return
				}
				response := []byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + acceptKey(req.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n")
				if framed(kind) {
					response = append(response, 0x82, byte(len(payload)))
				}
				_, err = b.Write(append(response, payload...))
				errCh <- err
			}()
			c, err := clientUpgrade(a, Options{Kind: kind, Peer: "edge-identity.invalid:8443"})
			if err != nil {
				t.Fatal(err)
			}
			got := make([]byte, len(payload))
			if _, err = io.ReadFull(c, got); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, payload) {
				t.Fatal("server lookahead lost")
			}
			if err = <-errCh; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUpgradeRejectsInvalidChallengePathAndOversizedHeaders(t *testing.T) {
	for _, request := range []string{
		fixedRequest("/wrong"), strings.Replace(fixedRequest("/tunnel"), "Sec-WebSocket-Version: 13", "Sec-WebSocket-Version: 12", 1),
		strings.Replace(fixedRequest("/tunnel"), "Connection: keep-alive, Upgrade", "Connection: keep-alive", 1),
		strings.Replace(fixedRequest("/tunnel"), "\r\n\r\n", "\r\nX-Large: "+strings.Repeat("x", maxHeaderBytes)+"\r\n\r\n", 1),
		strings.Replace(fixedRequest("/tunnel"), "Sec-WebSocket-Key: ", "Sec-WebSocket-Key: invalid", 1),
	} {
		a, b := net.Pipe()
		a.SetDeadline(time.Now().Add(time.Second))
		b.SetDeadline(time.Now().Add(time.Second))
		result := make(chan error, 1)
		go func() { _, err := serverUpgrade(a, Options{Kind: "wsmux"}); a.Close(); result <- err }()
		b.Write([]byte(request))
		b.Close()
		if err := <-result; err == nil {
			t.Fatal("invalid Upgrade accepted")
		}
	}
}

func TestHeaderBlockCRLFAcrossReadBufferAndPreservesTail(t *testing.T) {
	header := "X: " + strings.Repeat("x", 4092) + "\r\n\r\n"
	reader := bufio.NewReaderSize(strings.NewReader(header+"TAIL"), 4096)
	got, err := headerBlock(reader)
	if err != nil || string(got) != header {
		t.Fatal("valid CRLF split across reader buffers was rejected")
	}
	tail, err := io.ReadAll(reader)
	if err != nil || string(tail) != "TAIL" {
		t.Fatal("header parser lost lookahead")
	}
}

func TestWebSocketCloseHandshakeWithGorilla(t *testing.T) {
	server := echoListener(t, Options{Kind: "ws", Listen: "127.0.0.1:0"})
	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	c, _, err := dialer.Dial("ws://"+server.Addr().String()+"/tunnel", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(time.Second))
	if err := c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "finished"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	_, _, err = c.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseNormalClosure) {
		t.Fatal("normal RFC6455 close was not returned", err)
	}
}

func TestFramedWebSocketInteroperatesWithGorillaClients(t *testing.T) {
	server := echoListener(t, Options{Kind: "ws", Listen: "127.0.0.1:0", NoDelay: true})
	dialer := websocket.Dialer{HandshakeTimeout: time.Second, WriteBufferSize: 256}
	c, _, err := dialer.Dial("ws://"+server.Addr().String()+"/tunnel", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	payload := bytes.Repeat([]byte("fragmented-binary-content"), 500)
	ponged := false
	c.SetPongHandler(func(data string) error {
		if data == "ping-test" {
			ponged = true
		}
		return nil
	})
	w, err := c.NextWriter(websocket.BinaryMessage)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(payload[:len(payload)/2]); err != nil {
		t.Fatal(err)
	}
	if err = c.WriteControl(websocket.PingMessage, []byte("ping-test"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write(payload[len(payload)/2:]); err != nil {
		t.Fatal(err)
	}
	if err = w.Close(); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for len(got) < len(payload) {
		kind, data, err := c.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		if kind != websocket.BinaryMessage {
			t.Fatal("nonbinary application output")
		}
		got = append(got, data...)
	}
	if !bytes.Equal(got, payload) || !ponged {
		t.Fatal("fragment handling or ping/pong failed")
	}
}

func TestFramedWebSocketInteroperatesWithGorillaServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	upgrader := websocket.Upgrader{}
	server := http.Server{ReadHeaderTimeout: time.Second, MaxHeaderBytes: maxHeaderBytes, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tunnel" {
			http.NotFound(w, r)
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			kind, payload, err := c.ReadMessage()
			if err != nil {
				return
			}
			if kind != websocket.BinaryMessage || len(payload) > maxWriteMessage {
				return
			}
			if c.WriteMessage(kind, payload) != nil {
				return
			}
		}
	})}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	c, err := Dial(context.Background(), Options{Kind: "ws", Peer: listener.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	exchange(t, c, bytes.Repeat([]byte("32k-bounded-frames"), 8192))
}

func TestWebSocketRejectsWrongMaskingTextAndOversize(t *testing.T) {
	for _, header := range [][]byte{{0x82, 0x01, 'x'}, {0x81, 0x80, 0, 0, 0, 0}, {0x82, 0xFE, 0, 1, 0, 0, 0, 0}, {0x82, 0xFF, 0, 0, 0, 0, 1, 0, 0, 1, 0, 0, 0, 0}} {
		a, b := net.Pipe()
		a.SetDeadline(time.Now().Add(time.Second))
		b.SetDeadline(time.Now().Add(time.Second))
		c := newWebsocket(a, true)
		go func() { b.Write(header); b.Close() }()
		if _, err := c.Read(make([]byte, 1)); err == nil {
			t.Fatal("invalid frame accepted")
		}
		a.Close()
	}
}

func TestWebSocketDeadlineAndConcurrentWrites(t *testing.T) {
	deadlineA, deadlineB := net.Pipe()
	deadlineConn := newWebsocket(deadlineA, true)
	deadlineConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
	if _, err := deadlineConn.Read(make([]byte, 1)); err == nil {
		t.Fatal("read deadline ignored")
	}
	deadlineA.Close()
	deadlineB.Close()
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	client, server := newWebsocket(a, false), newWebsocket(b, true)
	server.SetReadDeadline(time.Now().Add(3 * time.Second))
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(value byte) {
			defer group.Done()
			for j := 0; j < 10; j++ {
				if _, err := client.Write(bytes.Repeat([]byte{value}, 100)); err != nil {
					return
				}
			}
		}(byte(i + 1))
	}
	got := make([]byte, 8000)
	if _, err := io.ReadFull(server, got); err != nil {
		t.Fatal(err)
	}
	group.Wait()
	for value := byte(1); value <= 8; value++ {
		if bytes.Count(got, []byte{value}) != 1000 {
			t.Fatal("concurrent frames corrupted")
		}
	}
	client.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	if _, err := client.Write([]byte("blocked")); err == nil {
		t.Fatal("write deadline ignored")
	}
}
