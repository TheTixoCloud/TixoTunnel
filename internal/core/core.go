// Independent two-peer tunnel engine by ir_spoof. https://t.me/ir_spoof
package core

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"backhaul.local/ir_spoof/internal/config"
	"backhaul.local/ir_spoof/internal/ipx"
	carrier "backhaul.local/ir_spoof/internal/transport"
	"backhaul.local/ir_spoof/internal/tun"
	"backhaul.local/ir_spoof/internal/wire"
	"github.com/xtaci/smux"
)

type Stats struct {
	Sessions atomic.Int64
	Streams  atomic.Int64
	Rejected atomic.Uint64
	Bytes    atomic.Uint64
	Ready    atomic.Bool
}
type Engine struct {
	cfg       config.Config
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mu        sync.Mutex
	closers   map[io.Closer]bool
	pool      []*peer
	notify    chan struct{}
	slots     chan struct{}
	peerSlots chan struct{}
	stats     Stats
	tunPump   *tunPacketPump
}
type peer struct {
	raw     net.Conn
	conn    net.Conn
	session *smux.Session
	active  atomic.Int64
	retired bool
}
type command struct {
	Kind   string `json:"kind"`
	Target string `json:"target,omitempty"`
	Source string `json:"source,omitempty"`
	Proxy  bool   `json:"proxy,omitempty"`
}
type countedConn struct {
	net.Conn
	done func()
	once sync.Once
}

func (c *countedConn) Close() error { e := c.Conn.Close(); c.once.Do(c.done); return e }
func (c *countedConn) CloseWrite() error {
	if v, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return v.CloseWrite()
	}
	return c.Conn.Close()
}

func Run(ctx context.Context, c config.Config) error {
	c.Defaults()
	if e := c.Validate(); e != nil {
		return e
	}
	if _, e := ParseMappings(c.Ports.Mapping); e != nil {
		return e
	}
	if c.Tuning.Workers > 0 {
		if c.Tuning.Workers > 256 {
			return errors.New("workers exceeds 256")
		}
		runtime.GOMAXPROCS(c.Tuning.Workers)
	}
	child, cancel := context.WithCancel(ctx)
	e := &Engine{cfg: c, ctx: child, cancel: cancel, closers: map[io.Closer]bool{}, notify: make(chan struct{}, 1), slots: make(chan struct{}, c.Tuning.MaxConnections), peerSlots: make(chan struct{}, min(c.Tuning.MaxConnections, 512))}
	defer func() { cancel(); e.closeAll(); e.wg.Wait() }()
	if err := e.health(); err != nil {
		return err
	}
	if c.IsIPX() {
		return e.runIPX()
	}
	if c.Transport.Type == "tun" {
		return e.runTUN()
	}
	if c.Server() {
		if err := e.startCarriers(); err != nil {
			return err
		}
		if err := e.startForwarders(false); err != nil {
			return err
		}
	} else {
		e.startDialers(false)
	}
	e.stats.Ready.Store(true)
	<-child.Done()
	return nil
}
func (e *Engine) launch(f func()) { e.wg.Add(1); go func() { defer e.wg.Done(); f() }() }
func (e *Engine) track(c io.Closer) {
	e.mu.Lock()
	if e.ctx.Err() != nil {
		e.mu.Unlock()
		c.Close()
		return
	}
	e.closers[c] = true
	e.mu.Unlock()
}
func (e *Engine) release(c io.Closer) { c.Close(); e.mu.Lock(); delete(e.closers, c); e.mu.Unlock() }
func (e *Engine) closeAll() {
	e.mu.Lock()
	list := make([]io.Closer, 0, len(e.closers))
	for c := range e.closers {
		list = append(list, c)
	}
	e.mu.Unlock()
	for _, c := range list {
		c.Close()
	}
}
func (e *Engine) signal() {
	select {
	case e.notify <- struct{}{}:
	default:
	}
}
func (e *Engine) options(address, local string) carrier.Options {
	c := e.cfg
	o := carrier.Options{Kind: c.Transport.Type, LocalAddr: local, EdgeIP: c.Dialer.EdgeIP, SNI: c.TLS.SNI, CertFile: c.TLS.CertFile, KeyFile: c.TLS.KeyFile, CAFile: c.TLS.CAFile, InsecureSkipVerify: c.TLS.InsecureSkipVerify, NoDelay: c.Transport.NoDelay, DialTimeout: time.Duration(c.Dialer.DialTimeout) * time.Second, KeepAlive: time.Duration(c.Transport.KeepalivePeriod) * time.Second, ReadBuffer: c.Tuning.ReceiveBuffer, WriteBuffer: c.Tuning.SendBuffer, MSS: c.Tuning.MSS}
	if c.Server() {
		o.Listen = address
	} else {
		o.Peer = address
	}
	return o
}
func (e *Engine) auth(c net.Conn, client bool) (net.Conn, error) {
	stop := context.AfterFunc(e.ctx, func() { _ = c.Close() })
	defer stop()
	return wire.Handshake(c, client, wire.Options{Token: e.cfg.Security.Token, Transport: e.cfg.Transport.Type, Algorithm: e.cfg.Security.Algorithm, Encrypted: e.cfg.Encrypted(), Timeout: time.Duration(e.cfg.Dialer.DialTimeout) * time.Second})
}
func (e *Engine) muxConfig() *smux.Config {
	c := smux.DefaultConfig()
	c.Version = e.cfg.Mux.Version
	c.MaxFrameSize = e.cfg.Mux.FrameSize
	c.MaxReceiveBuffer = e.cfg.Mux.ReceiveBuffer
	c.MaxStreamBuffer = e.cfg.Mux.StreamBuffer
	c.KeepAliveInterval = time.Duration(e.cfg.Transport.HeartbeatInterval) * time.Second
	c.KeepAliveTimeout = time.Duration(e.cfg.Transport.HeartbeatTimeout) * time.Second
	return c
}
func (e *Engine) startCarriers() error {
	for _, address := range e.cfg.ListenAddresses() {
		listener, err := carrier.Listen(e.ctx, e.options(address, ""))
		if err != nil {
			return errors.New("carrier listener could not start")
		}
		e.track(listener)
		e.launch(func() {
			for {
				c, err := listener.Accept()
				if err != nil {
					return
				}
				select {
				case e.peerSlots <- struct{}{}:
				default:
					e.stats.Rejected.Add(1)
					c.Close()
					continue
				}
				e.track(c)
				e.launch(func() { defer func() { <-e.peerSlots }(); e.acceptCarrier(c) })
			}
		})
	}
	return nil
}
func (e *Engine) acceptCarrier(raw net.Conn) {
	c, err := e.auth(raw, false)
	if err != nil {
		e.stats.Rejected.Add(1)
		e.release(raw)
		return
	}
	p := &peer{raw: raw, conn: c}
	if e.cfg.Multiplexed() {
		p.session, err = smux.Server(c, e.muxConfig())
		if err != nil {
			e.release(raw)
			return
		}
	}
	e.mu.Lock()
	if e.ctx.Err() != nil {
		e.mu.Unlock()
		e.release(raw)
		return
	}
	if len(e.pool) >= e.cfg.Tuning.ChannelSize {
		e.mu.Unlock()
		e.stats.Rejected.Add(1)
		e.release(raw)
		return
	}
	e.pool = append(e.pool, p)
	e.mu.Unlock()
	e.stats.Sessions.Add(1)
	e.signal()
	if p.session != nil {
		select {
		case <-e.ctx.Done():
		case <-p.session.CloseChan():
		}
		e.retire(p)
		e.release(raw)
	}
	// Nonmultiplexed authenticated connections wait in the bounded spare pool.
}
func (e *Engine) retire(p *peer) {
	e.mu.Lock()
	if !p.retired {
		p.retired = true
		for i, v := range e.pool {
			if v == p {
				e.pool = append(e.pool[:i], e.pool[i+1:]...)
				break
			}
		}
		e.stats.Sessions.Add(-1)
	}
	e.mu.Unlock()
	if p.session != nil {
		p.session.Close()
	}
	p.conn.Close()
	if p.raw != nil {
		e.release(p.raw)
	}
	e.signal()
}
func (e *Engine) open(cmd command) (net.Conn, error) {
	deadline := time.Now().Add(time.Duration(e.cfg.Dialer.DialTimeout) * time.Second)
	timeout := time.NewTimer(time.Until(deadline))
	defer timeout.Stop()
	for {
		if err := e.ctx.Err(); err != nil {
			return nil, err
		}
		if !time.Now().Before(deadline) {
			return nil, errors.New("authenticated peer deadline exceeded")
		}
		e.mu.Lock()
		var chosen *peer
		for _, p := range e.pool {
			if p.retired || (p.session != nil && p.session.IsClosed()) {
				continue
			}
			if p.active.Load() >= int64(e.cfg.Mux.Concurrency) {
				continue
			}
			if chosen == nil || p.active.Load() < chosen.active.Load() {
				chosen = p
			}
		}
		if chosen != nil {
			chosen.active.Add(1)
			if chosen.session == nil {
				for i, p := range e.pool {
					if p == chosen {
						e.pool = append(e.pool[:i], e.pool[i+1:]...)
						break
					}
				}
			}
			e.mu.Unlock()
			c := chosen.conn
			var err error
			if chosen.session != nil {
				c, err = chosen.session.OpenStream()
			}
			if err != nil {
				chosen.active.Add(-1)
				e.retire(chosen)
				continue
			}
			c.SetWriteDeadline(deadline)
			data, err := json.Marshal(cmd)
			if err == nil {
				err = wire.WriteFrame(c, data)
			}
			c.SetWriteDeadline(time.Time{})
			if err != nil {
				c.Close()
				chosen.active.Add(-1)
				if chosen.session == nil {
					e.retire(chosen)
				}
				continue
			}
			e.stats.Streams.Add(1)
			wrapped := &countedConn{Conn: c, done: func() {
				e.stats.Streams.Add(-1)
				chosen.active.Add(-1)
				if chosen.session == nil {
					e.retire(chosen)
				}
				e.signal()
			}}
			e.track(wrapped)
			return wrapped, nil
		}
		e.mu.Unlock()
		select {
		case <-e.ctx.Done():
			return nil, e.ctx.Err()
		case <-timeout.C:
			return nil, errors.New("no authenticated peer available")
		case <-e.notify:
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func (e *Engine) startDialers(tunMode bool) {
	locals := e.cfg.LocalAddresses()
	for _, address := range e.cfg.PeerAddresses() {
		for i := 0; i < e.cfg.Transport.ConnectionPool; i++ {
			address, local := address, locals[i%len(locals)]
			e.launch(func() {
				for e.ctx.Err() == nil {
					e.dialOnce(address, local, tunMode)
					select {
					case <-e.ctx.Done():
						return
					case <-time.After(time.Duration(e.cfg.Dialer.RetryInterval) * time.Second):
					}
				}
			})
		}
	}
}
func (e *Engine) dialOnce(address, local string, tunMode bool) {
	raw, err := carrier.Dial(e.ctx, e.options(address, local))
	if err != nil {
		return
	}
	e.track(raw)
	defer e.release(raw)
	c, err := e.auth(raw, true)
	if err != nil {
		e.stats.Rejected.Add(1)
		return
	}
	e.stats.Sessions.Add(1)
	defer e.stats.Sessions.Add(-1)
	if e.cfg.Multiplexed() {
		s, err := smux.Client(c, e.muxConfig())
		if err != nil {
			return
		}
		defer s.Close()
		for {
			stream, err := s.AcceptStream()
			if err != nil {
				return
			}
			select {
			case e.slots <- struct{}{}:
			default:
				stream.Close()
				continue
			}
			e.track(stream)
			e.launch(func() { defer func() { <-e.slots }(); defer e.release(stream); e.handle(stream) })
		}
	} else {
		e.handle(c)
	}
}
func (e *Engine) handle(c net.Conn) {
	data, err := wire.ReadFrame(c, 4096)
	if err != nil {
		return
	}
	var cmd command
	if err = json.Unmarshal(data, &cmd); err != nil {
		return
	}
	switch cmd.Kind {
	case "tcp":
		e.forwardTCP(c, cmd)
	case "udp":
		e.forwardUDP(c, cmd)
	}
}
func (e *Engine) backend(network, target string) (net.Conn, error) {
	d := net.Dialer{Timeout: time.Duration(e.cfg.Dialer.DialTimeout) * time.Second, KeepAlive: time.Duration(e.cfg.Transport.KeepalivePeriod) * time.Second}
	if e.cfg.Ports.BackendSourceIP != "" {
		ip := net.ParseIP(e.cfg.Ports.BackendSourceIP)
		if ip == nil {
			return nil, errors.New("invalid backend source IP")
		}
		if network == "tcp" {
			d.LocalAddr = &net.TCPAddr{IP: ip}
		} else {
			d.LocalAddr = &net.UDPAddr{IP: ip}
		}
	}
	return d.DialContext(e.ctx, network, target)
}
func (e *Engine) forwardTCP(c net.Conn, cmd command) {
	backend, err := e.backend("tcp", cmd.Target)
	if err != nil {
		return
	}
	e.track(backend)
	defer e.release(backend)
	if cmd.Proxy && cmd.Source != "" {
		if err = proxyHeader(backend, cmd.Source, backend.RemoteAddr().String()); err != nil {
			return
		}
	}
	e.duplex(c, backend)
}
func halfClose(c net.Conn) {
	c.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer c.SetWriteDeadline(time.Time{})
	if v, ok := c.(interface{ CloseWrite() error }); ok {
		v.CloseWrite()
	} else {
		c.Close()
	}
}
func (e *Engine) bufferSize() int {
	switch e.cfg.Tuning.BufferProfile {
	case "extreme_low_cpu":
		return 1 << 20
	case "ultra_low_cpu":
		return 256 << 10
	case "low_cpu":
		return 128 << 10
	case "low_memory":
		return 16 << 10
	default:
		return 32 << 10
	}
}
func (e *Engine) duplex(a, b net.Conn) {
	finished := make(chan struct{}, 2)
	copyTo := func(dst, src net.Conn) {
		n, _ := io.CopyBuffer(dst, src, make([]byte, e.bufferSize()))
		if n > 0 {
			e.stats.Bytes.Add(uint64(n))
		}
		halfClose(dst)
		finished <- struct{}{}
	}
	go copyTo(a, b)
	go copyTo(b, a)
	select {
	case <-finished:
	case <-e.ctx.Done():
		a.Close()
		b.Close()
		<-finished
	}
	select {
	case <-finished:
	case <-time.After(time.Duration(e.cfg.Tuning.ReadTimeout) * time.Second):
		a.Close()
		b.Close()
		<-finished
	case <-e.ctx.Done():
		a.Close()
		b.Close()
		<-finished
	}
}
func proxyHeader(w io.Writer, source, target string) error {
	sh, sp, e := net.SplitHostPort(source)
	if e != nil {
		return e
	}
	dh, dp, e := net.SplitHostPort(target)
	if e != nil {
		return e
	}
	if _, err := strconv.ParseUint(sp, 10, 16); err != nil {
		return errors.New("invalid PROXY source port")
	}
	if _, err := strconv.ParseUint(dp, 10, 16); err != nil {
		return errors.New("invalid PROXY target port")
	}
	proto := "TCP4"
	if net.ParseIP(sh) == nil || net.ParseIP(dh) == nil {
		return errors.New("invalid PROXY endpoint")
	}
	if strings.Contains(sh, ":") {
		proto = "TCP6"
	}
	_, e = fmt.Fprintf(w, "PROXY %s %s %s %s %s\r\n", proto, sh, dh, sp, dp)
	return e
}
func (e *Engine) startForwarders(overTUN bool) error {
	maps, err := ParseMappings(e.cfg.Ports.Mapping)
	if err != nil {
		return err
	}
	for _, mapping := range maps {
		mapping := mapping
		l, err := net.Listen("tcp", mapping.Bind)
		if err != nil {
			return errors.New("forwarding listener could not start")
		}
		e.track(l)
		e.launch(func() {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				select {
				case e.slots <- struct{}{}:
				default:
					c.Close()
					continue
				}
				e.track(c)
				e.launch(func() {
					defer func() { <-e.slots }()
					defer e.release(c)
					var upstream net.Conn
					var err error
					if overTUN {
						upstream, err = e.backend("tcp", e.tunTarget(mapping.Target))
					} else {
						upstream, err = e.open(command{Kind: "tcp", Target: mapping.Target, Source: c.RemoteAddr().String(), Proxy: e.cfg.Transport.ProxyProtocol})
					}
					if err != nil {
						return
					}
					defer e.release(upstream)
					e.duplex(c, upstream)
				})
			}
		})
		if e.cfg.Transport.AcceptUDP || overTUN {
			if err = e.startUDP(mapping, overTUN); err != nil {
				return err
			}
		}
	}
	return nil
}
func (e *Engine) tunTarget(target string) string {
	h, p, err := net.SplitHostPort(target)
	if err != nil {
		return target
	}
	if h == "127.0.0.1" || h == "::1" {
		peer, _, err := net.ParseCIDR(e.cfg.TUN.RemoteAddr)
		if err == nil {
			return net.JoinHostPort(peer.String(), p)
		}
	}
	return target
}
func (e *Engine) forwardUDP(c net.Conn, cmd command) {
	backend, err := e.backend("udp", cmd.Target)
	if err != nil {
		return
	}
	e.track(backend)
	defer e.release(backend)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := make([]byte, 65507)
		for {
			backend.SetReadDeadline(time.Now().Add(time.Duration(e.cfg.UDP.PeerIdleTimeout) * time.Second))
			n, err := backend.Read(buffer)
			if err != nil {
				return
			}
			if err = wire.WriteFrame(c, buffer[:n]); err != nil {
				return
			}
		}
	}()
	for {
		c.SetReadDeadline(time.Now().Add(time.Duration(e.cfg.UDP.PeerIdleTimeout) * time.Second))
		p, err := wire.ReadFrame(c, 65507)
		if err != nil {
			break
		}
		if _, err = backend.Write(p); err != nil {
			break
		}
	}
	backend.Close()
	c.Close()
	<-done
}
func (e *Engine) health() error {
	if e.cfg.TUN.HealthPort == 0 {
		return nil
	}
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(e.cfg.TUN.HealthPort)))
	if err != nil {
		return errors.New("loopback health listener could not start")
	}
	e.track(l)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ready": e.stats.Ready.Load(), "sessions": e.stats.Sessions.Load(), "streams": e.stats.Streams.Load(), "rejected": e.stats.Rejected.Load(), "bytes": e.stats.Bytes.Load(), "channel": "ir_spoof"})
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 4096}
	e.track(server)
	e.launch(func() { server.Serve(l) })
	return nil
}
func (e *Engine) runIPX() error {
	c := e.cfg
	startErrors := make(chan error, 1)
	options := ipx.Config{Profile: c.IPX.Profile, Role: c.IPX.Mode, Interface: c.IPX.Interface, LocalIP: c.IPX.ListenIP, PeerIP: c.IPX.DstIP, SpoofSrcIP: c.IPX.SpoofSrcIP, SpoofDstIP: c.IPX.SpoofDstIP, LocalPort: c.IPX.LocalPort, PeerPort: c.IPX.PeerPort, Protocol: c.IPX.Protocol, TUNName: c.TUN.Name, LocalTUN: c.TUN.LocalAddr, PeerTUN: c.TUN.RemoteAddr, Routes: c.TUN.Routes, MTU: c.TUN.MTU, OuterMTU: c.IPX.OuterMTU, Queues: c.IPX.Queues, Key: c.Security.PSK, Algorithm: c.Security.Algorithm, KDFIterations: c.Security.KDFIterations, AllowInsecurePlaintext: c.Security.AllowInsecurePlaintext && !c.Encrypted(),
		OnReady: func(ipx.Info) {
			if c.Server() {
				if err := e.startForwarders(true); err != nil {
					startErrors <- err
					e.cancel()
					return
				}
			}
			e.stats.Ready.Store(true)
			e.stats.Sessions.Store(1)
		}, OnStats: func(s ipx.Stats) { e.stats.Bytes.Store(s.TxBytes + s.RxBytes) }}
	// The original numeric configuration reserves zero as the role default.
	// Explicit nonzero overrides are symmetric: both peers must configure them.
	if c.IPX.ICMPType != 0 {
		value := c.IPX.ICMPType
		options.ICMPType = &value
		options.PeerICMPType = &value
	}
	if c.IPX.ICMPCode != 0 {
		value := c.IPX.ICMPCode
		options.ICMPCode = &value
		options.PeerICMPCode = &value
	}
	err := ipx.Run(e.ctx, options)
	select {
	case startErr := <-startErrors:
		return startErr
	default:
		return err
	}
}

type tunPacket struct {
	data       []byte
	generation uint64
}
type tunPacketPump struct {
	packets    chan tunPacket
	generation atomic.Uint64
	dropped    atomic.Uint64
	stopping   atomic.Bool
	done       chan struct{}
	errors     chan error
}
type tunPacketDevice interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
}

// startTUNPump is the only reader of this device during its entire lifetime.
// Odd generations identify a connected authenticated carrier; even generations
// are offline. Stale queued packets can never cross a reconnect boundary.
func (e *Engine) startTUNPump(d io.Reader) *tunPacketPump {
	capacity := min(256, max(1, (4<<20)/e.cfg.TUN.MTU))
	pump := &tunPacketPump{packets: make(chan tunPacket, capacity), done: make(chan struct{}), errors: make(chan error, 1)}
	e.tunPump = pump
	e.launch(func() {
		defer close(pump.done)
		buffer := make([]byte, tun.MaxPacketSize)
		for e.ctx.Err() == nil {
			n, err := d.Read(buffer)
			if err != nil {
				if !pump.stopping.Load() && e.ctx.Err() == nil {
					pump.errors <- err
					e.cancel()
				}
				return
			}
			if n == 0 {
				continue
			}
			if n > e.cfg.TUN.MTU || !ipx.ValidInnerPacket(buffer[:n]) {
				pump.dropped.Add(1)
				continue
			}
			generation := pump.generation.Load()
			if generation&1 == 0 {
				pump.dropped.Add(1)
				continue
			}
			packet := tunPacket{data: append([]byte(nil), buffer[:n]...), generation: generation}
			select {
			case <-e.ctx.Done():
				return
			case pump.packets <- packet:
			default:
				pump.dropped.Add(1)
			}
		}
	})
	return pump
}
func (p *tunPacketPump) drain() {
	for {
		select {
		case <-p.packets:
			p.dropped.Add(1)
		default:
			return
		}
	}
}

func (e *Engine) runTUN() (result error) {
	addresses, peers, locals := e.cfg.ListenAddresses(), e.cfg.PeerAddresses(), e.cfg.LocalAddresses()
	if e.cfg.Server() {
		if len(addresses) != 1 {
			return errors.New("TCP TUN requires exactly one listener")
		}
	} else {
		if len(peers) != 1 || len(locals) != 1 {
			return errors.New("TCP TUN requires exactly one peer and one local source address")
		}
	}
	d, err := tun.Open(e.ctx, tun.Config{Name: e.cfg.TUN.Name, Local: e.cfg.TUN.LocalAddr, Peer: e.cfg.TUN.RemoteAddr, MTU: e.cfg.TUN.MTU, Routes: e.cfg.TUN.Routes, Queues: 1})
	if err != nil {
		return err
	}
	e.track(d)
	pump := e.startTUNPump(d)
	defer func() {
		pump.stopping.Store(true)
		e.release(d)
		<-pump.done
		select {
		case readErr := <-pump.errors:
			if result == nil {
				result = readErr
			}
		default:
		}
	}()
	if e.cfg.Server() {
		if err = e.startForwarders(true); err != nil {
			return err
		}
		listener, err := carrier.Listen(e.ctx, e.options(addresses[0], ""))
		if err != nil {
			return err
		}
		e.track(listener)
		e.stats.Ready.Store(true)
		for {
			raw, err := listener.Accept()
			if err != nil {
				if e.ctx.Err() != nil {
					return nil
				}
				return err
			}
			e.track(raw)
			c, err := e.auth(raw, false)
			if err == nil {
				e.stats.Sessions.Store(1)
				e.tunDuplex(d, c)
				e.stats.Sessions.Store(0)
			} else {
				e.stats.Rejected.Add(1)
			}
			e.release(raw)
			if e.ctx.Err() != nil {
				return nil
			}
		}
	}
	e.stats.Ready.Store(true)
	for e.ctx.Err() == nil {
		raw, err := carrier.Dial(e.ctx, e.options(peers[0], locals[0]))
		if err == nil {
			e.track(raw)
			c, authErr := e.auth(raw, true)
			if authErr == nil {
				e.stats.Sessions.Store(1)
				e.tunDuplex(d, c)
				e.stats.Sessions.Store(0)
			} else {
				e.stats.Rejected.Add(1)
			}
			e.release(raw)
		}
		select {
		case <-e.ctx.Done():
			return nil
		case <-time.After(time.Duration(e.cfg.Dialer.RetryInterval) * time.Second):
		}
	}
	return nil
}

// Both per-carrier workers are joined before this method returns. They never
// read the persistent TUN directly and never close it on peer disconnect.
func (e *Engine) tunDuplex(d tunPacketDevice, c net.Conn) {
	ctx, cancel := context.WithCancel(e.ctx)
	pump := e.tunPump
	pump.drain()
	generation := pump.generation.Add(1)
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-ctx.Done():
				return
			case packet := <-pump.packets:
				if packet.generation != generation {
					pump.dropped.Add(1)
					continue
				}
				if ctx.Err() != nil {
					return
				}
				if err := wire.WriteFrame(c, packet.data); err != nil {
					return
				}
				e.stats.Bytes.Add(uint64(len(packet.data)))
			}
		}
	}()
	go func() {
		defer func() { done <- struct{}{} }()
		for ctx.Err() == nil {
			packet, err := wire.ReadFrame(c, e.cfg.TUN.MTU)
			if err != nil {
				return
			}
			if !ipx.ValidInnerPacket(packet) {
				e.stats.Rejected.Add(1)
				return
			}
			n, err := d.Write(packet)
			if err != nil || n != len(packet) {
				return
			}
			e.stats.Bytes.Add(uint64(n))
		}
	}()
	completed := 0
	select {
	case <-done:
		completed = 1
	case <-ctx.Done():
	}
	pump.generation.Add(1)
	cancel()
	_ = c.Close()
	for completed < 2 {
		<-done
		completed++
	}
	pump.drain()
}

// Parse the same length prefix in native tests without exposing any secret.
func packetLength(p []byte) int {
	if len(p) < 2 {
		return -1
	}
	return int(binary.BigEndian.Uint16(p[:2]))
}
