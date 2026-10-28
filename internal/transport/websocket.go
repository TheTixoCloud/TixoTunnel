// Independent bounded RFC6455 binary stream codec by ir_spoof.
// Attribution: https://t.me/ir_spoof
package transport

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"unicode/utf8"
)

const maxWriteMessage = 32 << 10
const maxReadMessage = 16 << 20

// websocketConn presents binary messages/continuations as a continuous stream.
// It permits one serialized reader and arbitrary serialized writers, handles
// control frames between fragments, and delegates deadlines directly to TCP.
// Text and negotiated compression are deliberately not tunnel payload types.
type websocketConn struct {
	net.Conn
	server             bool
	readMu, writeMu    sync.Mutex
	remaining          uint64
	mask               [4]byte
	maskOffset         uint64
	masked, fragmented bool
	messageBytes       uint64
}

func newWebsocket(c net.Conn, server bool) net.Conn { return &websocketConn{Conn: c, server: server} }

func (c *websocketConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for c.remaining == 0 {
		if err := c.nextFrame(); err != nil {
			c.Conn.Close()
			return 0, err
		}
	}
	if uint64(len(p)) > c.remaining {
		p = p[:int(c.remaining)]
	}
	n, err := c.Conn.Read(p)
	if c.masked {
		for i := 0; i < n; i++ {
			p[i] ^= c.mask[(c.maskOffset+uint64(i))%4]
		}
	}
	c.maskOffset += uint64(n)
	c.remaining -= uint64(n)
	if err == io.EOF && c.remaining != 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (c *websocketConn) nextFrame() error {
	for {
		var header [14]byte
		if _, err := io.ReadFull(c.Conn, header[:2]); err != nil {
			return err
		}
		final, opcode := header[0]&0x80 != 0, header[0]&15
		masked, size := header[1]&0x80 != 0, uint64(header[1]&127)
		if header[0]&0x70 != 0 || masked != c.server {
			return errors.New("invalid WebSocket reserved bits or masking direction")
		}
		if size == 126 {
			if _, err := io.ReadFull(c.Conn, header[2:4]); err != nil {
				return err
			}
			size = uint64(binary.BigEndian.Uint16(header[2:4]))
			if size < 126 {
				return errors.New("noncanonical WebSocket frame length")
			}
		} else if size == 127 {
			if _, err := io.ReadFull(c.Conn, header[2:10]); err != nil {
				return err
			}
			size = binary.BigEndian.Uint64(header[2:10])
			if size < 65536 || size>>63 != 0 {
				return errors.New("invalid WebSocket frame length")
			}
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(c.Conn, mask[:]); err != nil {
				return err
			}
		}
		if opcode >= 8 {
			if !final || size > 125 || opcode > 10 {
				return errors.New("invalid WebSocket control frame")
			}
			payload := make([]byte, int(size))
			if _, err := io.ReadFull(c.Conn, payload); err != nil {
				return err
			}
			if masked {
				for i := range payload {
					payload[i] ^= mask[i%4]
				}
			}
			switch opcode {
			case 8:
				if !validClose(payload) {
					return errors.New("invalid WebSocket close payload")
				}
				if err := c.control(8, payload); err != nil {
					return err
				}
				return io.EOF
			case 9:
				if err := c.control(10, payload); err != nil {
					return err
				}
			case 10:
			}
			continue
		}
		switch opcode {
		case 2:
			if c.fragmented {
				return errors.New("new WebSocket message before continuation completed")
			}
			c.messageBytes = 0
		case 0:
			if !c.fragmented {
				return errors.New("unexpected WebSocket continuation")
			}
		default:
			return errors.New("tunnel accepts only binary WebSocket messages")
		}
		if size > maxReadMessage || c.messageBytes > maxReadMessage-size {
			return errors.New("WebSocket message exceeds tunnel limit")
		}
		c.messageBytes += size
		c.fragmented = !final
		c.remaining, c.mask, c.masked, c.maskOffset = size, mask, masked, 0
		if size > 0 {
			return nil
		}
	}
}

func validClose(payload []byte) bool {
	if len(payload) == 0 {
		return true
	}
	if len(payload) == 1 || !utf8.Valid(payload[2:]) {
		return false
	}
	code := binary.BigEndian.Uint16(payload[:2])
	if code >= 3000 && code <= 4999 {
		return true
	}
	switch code {
	case 1000, 1001, 1002, 1003, 1007, 1008, 1009, 1010, 1011, 1012, 1013, 1014:
		return true
	}
	return false
}

func (c *websocketConn) control(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.writeFrame(opcode, payload)
	return err
}

func (c *websocketConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	written := 0
	for len(p) > 0 {
		size := len(p)
		if size > maxWriteMessage {
			size = maxWriteMessage
		}
		n, err := c.writeFrame(2, p[:size])
		written += n
		if err != nil {
			c.Conn.Close()
			return written, err
		}
		p = p[size:]
	}
	return written, nil
}

func (c *websocketConn) writeFrame(opcode byte, payload []byte) (int, error) {
	var header [14]byte
	header[0] = 0x80 | opcode
	offset := 2
	if len(payload) < 126 {
		header[1] = byte(len(payload))
	} else {
		header[1] = 126
		binary.BigEndian.PutUint16(header[2:4], uint16(len(payload)))
		offset = 4
	}
	data := payload
	if !c.server {
		header[1] |= 0x80
		if _, err := io.ReadFull(rand.Reader, header[offset:offset+4]); err != nil {
			return 0, errors.New("cannot generate WebSocket mask")
		}
		data = append([]byte(nil), payload...)
		for i := range data {
			data[i] ^= header[offset+i%4]
		}
		offset += 4
	}
	if _, err := writeAll(c.Conn, header[:offset]); err != nil {
		return 0, err
	}
	return writeAll(c.Conn, data)
}
