// Fresh authenticated sessions by ir_spoof. https://t.me/ir_spoof
// BHC4 intentionally does not reproduce the original commercial wire protocol.
package wire

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
)

const MaxRecord = 32768

type Options struct {
	Token, Transport, Algorithm string
	Encrypted                   bool
	Timeout                     time.Duration
}

func validateOptions(o Options) error {
	if len(o.Token) < 16 || len(o.Token) > 4096 {
		return errors.New("session token must contain 16..4096 bytes")
	}
	switch o.Transport {
	case "tcp", "tcpmux", "xtcpmux", "ws", "wss", "wsmux", "wssmux", "xwsmux", "anytls", "tun":
	default:
		return errors.New("unsupported session transport")
	}
	switch o.Algorithm {
	case "aes-128-gcm", "aes-256-gcm", "chacha20-poly1305":
	default:
		return errors.New("unsupported session algorithm")
	}
	if o.Timeout < 0 || o.Timeout > 5*time.Minute {
		return errors.New("session handshake timeout outside bounds")
	}
	return nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if n < 0 || n > len(p) {
			return io.ErrShortWrite
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

func mac(key string, parts ...[]byte) []byte {
	m := hmac.New(sha256.New, []byte(key))
	for _, p := range parts {
		m.Write(p)
	}
	return m.Sum(nil)
}

func aead(algorithm string, key []byte) (cipher.AEAD, error) {
	switch algorithm {
	case "chacha20-poly1305":
		return chacha20poly1305.New(key)
	case "aes-128-gcm":
		if len(key) < 16 {
			return nil, errors.New("invalid AES128 key length")
		}
		key = key[:16]
	case "aes-256-gcm":
		if len(key) != 32 {
			return nil, errors.New("invalid AES256 key length")
		}
	default:
		return nil, errors.New("unsupported session algorithm")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func Handshake(conn net.Conn, client bool, o Options) (net.Conn, error) {
	// Reject ambiguous/unsafe constructor input before touching the carrier.
	if err := validateOptions(o); err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("session requires a carrier")
	}
	completed := false
	defer func() {
		if !completed {
			conn.Close()
		}
	}()
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	public := private.PublicKey().Bytes()
	context := []byte("ir_spoof Backhaul BHC4/" + o.Transport + "/" + o.Algorithm)
	if o.Encrypted {
		context = append(context, 1)
	} else {
		context = append(context, 0)
	}
	var cp, sp []byte
	if client {
		cp = public
		hello := append([]byte("BHC4"), cp...)
		hello = append(hello, mac(o.Token, context, []byte("client"), cp)...)
		if err = writeAll(conn, hello); err != nil {
			return nil, err
		}
		reply := make([]byte, 68)
		if _, err = io.ReadFull(conn, reply); err != nil {
			return nil, err
		}
		sp = reply[4:36]
		if string(reply[:4]) != "BHS4" || !hmac.Equal(reply[36:], mac(o.Token, context, []byte("server"), cp, sp)) {
			return nil, errors.New("peer authentication failed")
		}
		if err = writeAll(conn, mac(o.Token, context, []byte("finish"), cp, sp)); err != nil {
			return nil, err
		}
	} else {
		hello := make([]byte, 68)
		if _, err = io.ReadFull(conn, hello); err != nil {
			return nil, err
		}
		cp, sp = hello[4:36], public
		if string(hello[:4]) != "BHC4" || !hmac.Equal(hello[36:], mac(o.Token, context, []byte("client"), cp)) {
			return nil, errors.New("peer authentication failed")
		}
		reply := append([]byte("BHS4"), sp...)
		reply = append(reply, mac(o.Token, context, []byte("server"), cp, sp)...)
		if err = writeAll(conn, reply); err != nil {
			return nil, err
		}
		finish := make([]byte, 32)
		if _, err = io.ReadFull(conn, finish); err != nil {
			return nil, err
		}
		if !hmac.Equal(finish, mac(o.Token, context, []byte("finish"), cp, sp)) {
			return nil, errors.New("peer authentication failed")
		}
	}
	peer := cp
	if client {
		peer = sp
	}
	peerKey, err := ecdh.X25519().NewPublicKey(peer)
	if err != nil {
		return nil, err
	}
	shared, err := private.ECDH(peerKey)
	if err != nil {
		return nil, err
	}
	// Domain-separated extract/expand with fresh X25519 contributions on both sides.
	extract := hmac.New(sha256.New, mac(o.Token, context, cp, sp))
	extract.Write(shared)
	base := extract.Sum(nil)
	derive := func(label string) []byte {
		m := hmac.New(sha256.New, base)
		m.Write(context)
		m.Write([]byte(label))
		m.Write([]byte{1})
		return m.Sum(nil)
	}
	c2s, err := aead(o.Algorithm, derive("client-to-server"))
	if err != nil {
		return nil, err
	}
	s2c, err := aead(o.Algorithm, derive("server-to-client"))
	if err != nil {
		return nil, err
	}
	send, receive := s2c, c2s
	if client {
		send, receive = c2s, s2c
	}
	if err = conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	completed = true
	return &recordConn{Conn: conn, send: send, receive: receive, encrypted: o.Encrypted}, nil
}

type recordConn struct {
	net.Conn
	send, receive           cipher.AEAD
	encrypted               bool
	rm, wm                  sync.Mutex
	stateMu                 sync.Mutex
	terminal                error
	rx, tx                  uint64
	pending                 []byte
	writeClosed, readClosed bool
}

func nonce(counter uint64) []byte {
	n := make([]byte, 12)
	binary.BigEndian.PutUint64(n[4:], counter)
	return n
}

func (r *recordConn) terminalError() error {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	return r.terminal
}

// A broken record cannot be retried or resynchronized. Preserve the first
// terminal error for both directions, close the carrier to unblock concurrent
// I/O, and never acquire the opposite direction's lock while doing so.
func (r *recordConn) poison(err error) error {
	r.stateMu.Lock()
	first := r.terminal == nil
	if first {
		r.terminal = err
	}
	saved := r.terminal
	r.stateMu.Unlock()
	if first {
		r.Conn.Close()
	}
	return saved
}

func (r *recordConn) Close() error {
	r.poison(net.ErrClosed)
	return nil
}

func (r *recordConn) Read(p []byte) (int, error) {
	r.rm.Lock()
	defer r.rm.Unlock()
	if err := r.terminalError(); err != nil {
		return 0, err
	}
	if len(p) == 0 {
		return 0, nil
	}
	if r.readClosed {
		return 0, io.EOF
	}
	if len(r.pending) == 0 {
		if r.rx == ^uint64(0) {
			return 0, r.poison(errors.New("record counter exhausted"))
		}
		var head [4]byte
		if _, err := io.ReadFull(r.Conn, head[:]); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, r.poison(err)
		}
		size := int(binary.BigEndian.Uint32(head[:]))
		limit, minimum := MaxRecord, 0
		if r.encrypted {
			minimum = r.receive.Overhead()
			limit += minimum
		}
		if size < minimum || size > limit {
			return 0, r.poison(errors.New("invalid record length"))
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(r.Conn, data); err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return 0, r.poison(err)
		}
		if r.encrypted {
			var err error
			data, err = r.receive.Open(nil, nonce(r.rx), data, head[:])
			if err != nil {
				return 0, r.poison(errors.New("record authentication failed"))
			}
		}
		r.rx++
		if len(data) == 0 {
			// This authenticated logical EOF closes only the read direction.
			// The peer may still receive a response after its request EOF.
			r.readClosed = true
			return 0, io.EOF
		}
		r.pending = data
	}
	n := copy(p, r.pending)
	r.pending = r.pending[n:]
	return n, nil
}

func (r *recordConn) writeRecord(p []byte) error {
	if err := r.terminalError(); err != nil {
		return err
	}
	if r.tx == ^uint64(0) {
		return r.poison(errors.New("record counter exhausted"))
	}
	size := len(p)
	if r.encrypted {
		size += r.send.Overhead()
	}
	var head [4]byte
	binary.BigEndian.PutUint32(head[:], uint32(size))
	body := p
	if r.encrypted {
		body = r.send.Seal(nil, nonce(r.tx), p, head[:])
	}
	if err := writeAll(r.Conn, head[:]); err != nil {
		return r.poison(err)
	}
	if err := writeAll(r.Conn, body); err != nil {
		return r.poison(err)
	}
	r.tx++
	return nil
}

func (r *recordConn) Write(p []byte) (int, error) {
	r.wm.Lock()
	defer r.wm.Unlock()
	if err := r.terminalError(); err != nil {
		return 0, err
	}
	if r.writeClosed {
		return 0, net.ErrClosed
	}
	written := 0
	for len(p) > 0 {
		n := len(p)
		if n > MaxRecord {
			n = MaxRecord
		}
		if err := r.writeRecord(p[:n]); err != nil {
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

func (r *recordConn) CloseWrite() error {
	r.wm.Lock()
	defer r.wm.Unlock()
	if err := r.terminalError(); err != nil {
		return err
	}
	if r.writeClosed {
		return nil
	}
	r.writeClosed = true
	return r.writeRecord(nil)
}

func WriteFrame(w io.Writer, p []byte) error {
	if len(p) > 65535 {
		return errors.New("frame too large")
	}
	var h [2]byte
	binary.BigEndian.PutUint16(h[:], uint16(len(p)))
	if err := writeAll(w, h[:]); err != nil {
		return err
	}
	return writeAll(w, p)
}

func ReadFrame(r io.Reader, max int) ([]byte, error) {
	var h [2]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(h[:]))
	if n > max {
		return nil, errors.New("frame exceeds limit")
	}
	p := make([]byte, n)
	_, err := io.ReadFull(r, p)
	return p, err
}
