// Bounded HTTP Upgrade carriers by ir_spoof - https://t.me/ir_spoof.
package transport

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
)

const maxHeaderBytes = 16 << 10
const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// bufferedConn preserves bytes already read beyond the HTTP headers. In raw
// mux modes these bytes are application records, never WebSocket frame data.
type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func headerBlock(reader *bufio.Reader) ([]byte, error) {
	var header bytes.Buffer
	lineBytes := 0
	for {
		piece, err := reader.ReadSlice('\n')
		if header.Len()+len(piece) > maxHeaderBytes {
			return nil, errors.New("HTTP Upgrade header exceeds limit")
		}
		header.Write(piece)
		lineBytes += len(piece)
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil {
			return nil, errors.New("incomplete HTTP Upgrade header")
		}
		if header.Len() < 2 || header.Bytes()[header.Len()-2] != '\r' {
			return nil, errors.New("HTTP Upgrade requires CRLF headers")
		}
		if lineBytes == 2 {
			return header.Bytes(), nil
		}
		// Obsolete folded fields are unnecessary for this protocol and rejected.
		start := header.Len() - lineBytes
		if header.Bytes()[start] == ' ' || header.Bytes()[start] == '\t' {
			return nil, errors.New("folded HTTP Upgrade fields are not permitted")
		}
		lineBytes = 0
	}
}

func headerToken(header http.Header, key, wanted string) bool {
	for _, value := range header.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), wanted) {
				return true
			}
		}
	}
	return false
}

func acceptKey(key string) string {
	hash := sha1.Sum([]byte(key + websocketGUID)) // RFC6455 challenge, not an authentication hash.
	return base64.StdEncoding.EncodeToString(hash[:])
}

func writeAll(c io.Writer, data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		n, err := c.Write(data)
		if n < 0 || n > len(data) {
			return written, io.ErrShortWrite
		}
		written += n
		data = data[n:]
		if err != nil {
			return written, err
		}
		if n == 0 {
			return written, io.ErrShortWrite
		}
	}
	return written, nil
}

func serverUpgrade(c net.Conn, o Options) (net.Conn, error) {
	reader := bufio.NewReaderSize(c, 4096)
	header, err := headerBlock(reader)
	if err != nil {
		return nil, err
	}
	request, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(header)))
	if err != nil || request.Method != "GET" || request.ProtoMajor != 1 || request.ProtoMinor != 1 ||
		request.RequestURI != upgradePath(o.Kind) || request.Host == "" || request.ContentLength > 0 || len(request.TransferEncoding) != 0 ||
		!headerToken(request.Header, "Connection", "upgrade") || !headerToken(request.Header, "Upgrade", "websocket") ||
		len(request.Header.Values("Sec-WebSocket-Version")) != 1 || request.Header.Get("Sec-WebSocket-Version") != "13" ||
		len(request.Header.Values("Sec-WebSocket-Key")) != 1 {
		return nil, errors.New("invalid HTTP/1.1 WebSocket Upgrade request")
	}
	key := request.Header.Get("Sec-WebSocket-Key")
	decoded, err := base64.StdEncoding.Strict().DecodeString(key)
	if err != nil || len(decoded) != 16 {
		return nil, errors.New("invalid WebSocket challenge")
	}
	response := "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " + acceptKey(key) + "\r\n\r\n"
	if _, err := writeAll(c, []byte(response)); err != nil {
		return nil, errors.New("cannot send HTTP Upgrade response")
	}
	result := &bufferedConn{Conn: c, reader: reader}
	if framed(o.Kind) {
		return newWebsocket(result, true), nil
	}
	return result, nil
}

func clientUpgrade(c net.Conn, o Options) (net.Conn, error) {
	failed := true
	defer func() {
		if failed {
			c.Close()
		}
	}()
	var nonce [16]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, errors.New("cannot generate WebSocket challenge")
	}
	key := base64.StdEncoding.EncodeToString(nonce[:])
	request := "GET " + upgradePath(o.Kind) + " HTTP/1.1\r\nHost: " + o.Peer + "\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: " + key + "\r\nSec-WebSocket-Version: 13\r\n\r\n"
	if _, err := writeAll(c, []byte(request)); err != nil {
		return nil, errors.New("cannot send HTTP Upgrade request")
	}
	reader := bufio.NewReaderSize(c, 4096)
	header, err := headerBlock(reader)
	if err != nil {
		return nil, err
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), nil)
	if err != nil || response.ProtoMajor != 1 || response.ProtoMinor != 1 || response.StatusCode != 101 ||
		!headerToken(response.Header, "Connection", "upgrade") || !headerToken(response.Header, "Upgrade", "websocket") ||
		len(response.Header.Values("Sec-WebSocket-Accept")) != 1 || response.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) ||
		response.Header.Get("Sec-WebSocket-Extensions") != "" || response.Header.Get("Sec-WebSocket-Protocol") != "" {
		return nil, errors.New("invalid HTTP Upgrade response or challenge")
	}
	failed = false
	result := &bufferedConn{Conn: c, reader: reader}
	if framed(o.Kind) {
		return newWebsocket(result, false), nil
	}
	return result, nil
}
