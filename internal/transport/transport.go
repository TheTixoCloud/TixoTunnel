// Package transport implements independent Backhaul carriers by ir_spoof.
// Attribution: https://t.me/ir_spoof
// Carrier compatibility does not imply the original licensed application wire.
package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	utls "github.com/refraction-networking/utls"
)

// Options describes one physical carrier. LocalAddr selects an assigned local
// address, not a spoofed source. Peer remains the HTTP Host/TLS identity when
// EdgeIP changes the physical destination. No environment proxy is consulted.
type Options struct {
	Kind, Listen, Peer, LocalAddr, EdgeIP, SNI, CertFile, KeyFile, CAFile string
	InsecureSkipVerify, NoDelay                                           bool
	DialTimeout, KeepAlive                                                time.Duration
	ReadBuffer, WriteBuffer, MSS                                          int
}

const handshakeSlots = 64

func validate(o Options) error {
	switch o.Kind {
	case "tcp", "tcpmux", "xtcpmux", "ws", "wss", "wsmux", "wssmux", "xwsmux", "anytls", "tun":
	default:
		return errors.New("unsupported carrier")
	}
	if o.DialTimeout < 0 || o.KeepAlive < 0 || o.ReadBuffer < 0 || o.WriteBuffer < 0 || o.MSS < 0 || o.MSS > 65535 {
		return errors.New("invalid carrier timeout or socket tuning")
	}
	if strings.ContainsAny(o.SNI, "\r\n\x00\t") || len(o.SNI) > 253 {
		return errors.New("invalid TLS identity")
	}
	return nil
}

func timeout(o Options) time.Duration {
	if o.DialTimeout == 0 {
		return 10 * time.Second
	}
	return o.DialTimeout
}

func needsTLS(kind string) bool { return kind == "wss" || kind == "wssmux" || kind == "anytls" }
func needsUpgrade(kind string) bool {
	return kind == "ws" || kind == "wss" || kind == "wsmux" || kind == "wssmux" || kind == "xwsmux"
}
func framed(kind string) bool { return kind == "ws" || kind == "wss" }
func upgradePath(kind string) string {
	if kind == "xwsmux" {
		return "/data"
	}
	return "/tunnel"
}

func socketControl(o Options) func(string, string, syscall.RawConn) error {
	if o.MSS == 0 {
		return nil
	}
	return func(_, _ string, c syscall.RawConn) error { return setMSS(c, o.MSS) }
}

func tune(c net.Conn, o Options) error {
	x, ok := c.(*net.TCPConn)
	if !ok {
		return errors.New("carrier requires a TCP socket")
	}
	if err := x.SetNoDelay(o.NoDelay); err != nil {
		return errors.New("cannot set TCP delay policy")
	}
	if err := x.SetKeepAlive(o.KeepAlive > 0); err != nil {
		return errors.New("cannot set TCP keepalive")
	}
	if o.KeepAlive > 0 {
		if err := x.SetKeepAlivePeriod(o.KeepAlive); err != nil {
			return errors.New("cannot set keepalive period")
		}
	}
	if o.ReadBuffer > 0 {
		if err := x.SetReadBuffer(o.ReadBuffer); err != nil {
			return errors.New("cannot set socket read buffer")
		}
	}
	if o.WriteBuffer > 0 {
		if err := x.SetWriteBuffer(o.WriteBuffer); err != nil {
			return errors.New("cannot set socket write buffer")
		}
	}
	return nil
}

func peerParts(value string) (string, string, error) {
	if strings.ContainsAny(value, "\r\n\x00\t /?#@") {
		return "", "", errors.New("invalid peer endpoint")
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil || host == "" || len(host) > 253 {
		return "", "", errors.New("peer requires HOST:PORT")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", "", errors.New("invalid peer port")
	}
	return host, port, nil
}

// assignedLocal validates a literal IP against interface addresses without DNS.
// It runs on each Dial, since addresses can be assigned/removed after -check.
func assignedLocal(value string) (*net.TCPAddr, error) {
	if value == "" {
		return nil, nil
	}
	host, port, zone := value, 0, ""
	if net.ParseIP(strings.Split(value, "%")[0]) == nil {
		var p string
		var err error
		host, p, err = net.SplitHostPort(value)
		if err != nil {
			return nil, errors.New("local bind requires a literal IP or IP:port")
		}
		port, err = strconv.Atoi(p)
		if err != nil || port < 0 || port > 65535 {
			return nil, errors.New("invalid local bind port")
		}
	}
	if index := strings.LastIndex(host, "%"); index >= 0 {
		zone, host = host[index+1:], host[:index]
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return nil, errors.New("local bind must be a specific literal unicast IP")
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.New("cannot inspect locally assigned addresses")
	}
	for _, device := range interfaces {
		if device.Flags&net.FlagUp == 0 || zone != "" && zone != device.Name && zone != strconv.Itoa(device.Index) {
			continue
		}
		addresses, err := device.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			local, _, err := net.ParseCIDR(address.String())
			if err == nil && local.Equal(ip) {
				return &net.TCPAddr{IP: ip, Port: port, Zone: zone}, nil
			}
		}
	}
	return nil, errors.New("requested local source IP is not assigned to an active interface")
}

func clientTLS(o Options) (*tls.Config, error) {
	host, _, err := peerParts(o.Peer)
	if err != nil {
		return nil, err
	}
	name := o.SNI
	if name == "" {
		name = strings.Split(host, "%")[0]
	}
	config := &tls.Config{ServerName: name, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}, InsecureSkipVerify: o.InsecureSkipVerify}
	if o.CAFile != "" {
		info, err := os.Stat(o.CAFile)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return nil, errors.New("cannot read local TLS trust file")
		}
		data, err := os.ReadFile(o.CAFile)
		if err != nil {
			return nil, errors.New("cannot read local TLS trust file")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(data) {
			return nil, errors.New("local trust file contains no certificates")
		}
		config.RootCAs = roots
	}
	return config, nil
}

func serverTLS(o Options) (*tls.Config, error) {
	if o.CertFile == "" || o.KeyFile == "" {
		return nil, errors.New("TLS listener requires a local certificate and private key")
	}
	pair, err := tls.LoadX509KeyPair(o.CertFile, o.KeyFile)
	if err != nil {
		return nil, errors.New("cannot load listener TLS certificate/key")
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}}, nil
}

// Dial establishes the selected carrier with a bounded TLS/Upgrade handshake.
// After Dial returns, the caller owns the connection and its lifetime.
func Dial(ctx context.Context, o Options) (net.Conn, error) {
	if err := validate(o); err != nil {
		return nil, err
	}
	_, port, err := peerParts(o.Peer)
	if err != nil {
		return nil, err
	}
	local, err := assignedLocal(o.LocalAddr)
	if err != nil {
		return nil, err
	}
	address := o.Peer
	if o.EdgeIP != "" {
		if strings.ContainsAny(o.EdgeIP, "\r\n\x00\t /?#@[]") || len(o.EdgeIP) > 253 {
			return nil, errors.New("invalid edge destination")
		}
		address = net.JoinHostPort(o.EdgeIP, port)
	}
	dialer := net.Dialer{Timeout: timeout(o), LocalAddr: local, Control: socketControl(o)}
	c, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, errors.New("cannot connect to carrier peer")
	}
	if err = tune(c, o); err != nil {
		c.Close()
		return nil, err
	}
	if !needsTLS(o.Kind) && !needsUpgrade(o.Kind) {
		return c, nil
	}
	handshakeCtx, cancel := context.WithTimeout(ctx, timeout(o))
	defer cancel()
	deadline, _ := handshakeCtx.Deadline()
	c.SetDeadline(deadline)
	raw := c
	stop := context.AfterFunc(handshakeCtx, func() { raw.Close() })
	defer stop()
	if needsTLS(o.Kind) {
		config, e := clientTLS(o)
		if e != nil {
			c.Close()
			return nil, e
		}
		if o.Kind == "anytls" {
			// Upstream's label refers to a browser-shaped TLS carrier, not the
			// independent AnyTLS authentication/session protocol.
			x := utls.UClient(c, &utls.Config{ServerName: config.ServerName, RootCAs: config.RootCAs, InsecureSkipVerify: config.InsecureSkipVerify,
				MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}}, utls.HelloChrome_Auto)
			err = x.HandshakeContext(handshakeCtx)
			c = x
		} else {
			x := tls.Client(c, config)
			err = x.HandshakeContext(handshakeCtx)
			c = x
		}
		if err != nil {
			c.Close()
			return nil, errors.New("TLS peer verification or handshake failed")
		}
	}
	if needsUpgrade(o.Kind) {
		c, err = clientUpgrade(c, o)
		if err != nil {
			return nil, err
		}
	}
	if err = c.SetDeadline(time.Time{}); err != nil {
		c.Close()
		return nil, errors.New("cannot clear carrier handshake deadline")
	}
	if err = handshakeCtx.Err(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// Listen bounds concurrent pre-authentication carrier handshakes to 64.
// Slow TLS/HTTP peers do not serialize the Accept loop. Application-level
// authentication and connection limits remain the caller's responsibility.
func Listen(ctx context.Context, o Options) (net.Listener, error) {
	if err := validate(o); err != nil {
		return nil, err
	}
	if o.Listen == "" {
		return nil, errors.New("carrier listener requires an explicit bind endpoint")
	}
	var config *tls.Config
	var err error
	if needsTLS(o.Kind) {
		config, err = serverTLS(o)
		if err != nil {
			return nil, err
		}
	}
	raw, err := (&net.ListenConfig{Control: socketControl(o)}).Listen(ctx, "tcp", o.Listen)
	if err != nil {
		return nil, errors.New("cannot create carrier listener")
	}
	child, cancel := context.WithCancel(ctx)
	l := &listener{raw: raw, options: o, tls: config, ctx: child, cancel: cancel,
		done: make(chan struct{}), ready: make(chan net.Conn, handshakeSlots), slots: make(chan struct{}, handshakeSlots), pending: make(map[net.Conn]struct{})}
	go l.acceptLoop()
	go func() {
		select {
		case <-child.Done():
			l.Close()
		case <-l.done:
		}
	}()
	return l, nil
}

type listener struct {
	raw     net.Listener
	options Options
	tls     *tls.Config
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	ready   chan net.Conn
	slots   chan struct{}
	mu      sync.Mutex
	pending map[net.Conn]struct{}
	closed  bool
	once    sync.Once
	workers sync.WaitGroup
}

func (l *listener) Addr() net.Addr { return l.raw.Addr() }

func (l *listener) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, net.ErrClosed
	case c := <-l.ready:
		l.mu.Lock()
		closed := l.closed
		l.mu.Unlock()
		if closed {
			c.Close()
			return nil, net.ErrClosed
		}
		return c, nil
	}
}

func (l *listener) Close() error {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.done)
		l.cancel()
		l.raw.Close()
		for c := range l.pending {
			c.Close()
		}
		l.mu.Unlock()
		l.workers.Wait()
		for {
			select {
			case c := <-l.ready:
				c.Close()
			default:
				return
			}
		}
	})
	return nil
}

func (l *listener) acceptLoop() {
	for {
		c, err := l.raw.Accept()
		if err != nil {
			l.Close()
			return
		}
		if err := tune(c, l.options); err != nil {
			c.Close()
			continue
		}
		select {
		case l.slots <- struct{}{}:
		default:
			c.Close()
			continue
		}
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			c.Close()
			<-l.slots
			return
		}
		l.pending[c] = struct{}{}
		l.workers.Add(1)
		l.mu.Unlock()
		go l.prepare(c)
	}
}

func (l *listener) prepare(raw net.Conn) {
	defer l.workers.Done()
	defer func() { <-l.slots }()
	c := raw
	var err error
	if l.tls != nil || needsUpgrade(l.options.Kind) {
		ctx, cancel := context.WithTimeout(l.ctx, timeout(l.options))
		defer cancel()
		deadline, _ := ctx.Deadline()
		raw.SetDeadline(deadline)
		if l.tls != nil {
			x := tls.Server(raw, l.tls)
			err = x.HandshakeContext(ctx)
			c = x
		}
		if err == nil && needsUpgrade(l.options.Kind) {
			c, err = serverUpgrade(c, l.options)
		}
		if err == nil {
			err = c.SetDeadline(time.Time{})
		}
	}
	l.mu.Lock()
	delete(l.pending, raw)
	closed := l.closed
	l.mu.Unlock()
	if err != nil || closed {
		raw.Close()
		return
	}
	select {
	case l.ready <- c:
	case <-l.done:
		c.Close()
	}
}
