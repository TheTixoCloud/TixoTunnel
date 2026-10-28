// Independent reconstruction by ir_spoof. https://t.me/ir_spoof
package config

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type Listener struct {
	BindAddr  string   `toml:"bind_addr" json:"bind_addr"`
	BindAddrs []string `toml:"bind_addrs" json:"bind_addrs"`
}
type Dialer struct {
	RemoteAddr    string   `toml:"remote_addr" json:"remote_addr"`
	RemoteAddrs   []string `toml:"remote_addrs" json:"remote_addrs"`
	LocalAddr     string   `toml:"local_addr" json:"local_addr"`
	LocalAddrs    []string `toml:"local_addrs" json:"local_addrs"`
	EdgeIP        string   `toml:"edge_ip" json:"edge_ip"`
	DialTimeout   int      `toml:"dial_timeout" json:"dial_timeout"`
	RetryInterval int      `toml:"retry_interval" json:"retry_interval"`
}
type Transport struct {
	Type              string `toml:"type" json:"type"`
	NoDelay           bool   `toml:"nodelay" json:"nodelay"`
	KeepalivePeriod   int    `toml:"keepalive_period" json:"keepalive_period"`
	AcceptUDP         bool   `toml:"accept_udp" json:"accept_udp"`
	ProxyProtocol     bool   `toml:"proxy_protocol" json:"proxy_protocol"`
	ConnectionPool    int    `toml:"connection_pool" json:"connection_pool"`
	HeartbeatInterval int    `toml:"heartbeat_interval" json:"heartbeat_interval"`
	HeartbeatTimeout  int    `toml:"heartbeat_timeout" json:"heartbeat_timeout"`
}
type Security struct {
	Token                  string `toml:"token" json:"token"`
	EnableEncryption       *bool  `toml:"enable_encryption" json:"enable_encryption"`
	Algorithm              string `toml:"algorithm" json:"algorithm"`
	PSK                    string `toml:"psk" json:"psk"`
	KDFIterations          int    `toml:"kdf_iterations" json:"kdf_iterations"`
	AllowInsecurePlaintext bool   `toml:"allow_insecure_plaintext" json:"allow_insecure_plaintext"`
}
type Mux struct {
	Version       int `toml:"mux_version" json:"mux_version"`
	FrameSize     int `toml:"mux_framesize" json:"mux_framesize"`
	ReceiveBuffer int `toml:"mux_recievebuffer" json:"mux_recievebuffer"`
	StreamBuffer  int `toml:"mux_streambuffer" json:"mux_streambuffer"`
	Concurrency   int `toml:"mux_concurrency" json:"mux_concurrency"`
}
type TUN struct {
	Encapsulation string   `toml:"encapsulation" json:"encapsulation"`
	Name          string   `toml:"name" json:"name"`
	LocalAddr     string   `toml:"local_addr" json:"local_addr"`
	RemoteAddr    string   `toml:"remote_addr" json:"remote_addr"`
	HealthPort    int      `toml:"health_port" json:"health_port"`
	MTU           int      `toml:"mtu" json:"mtu"`
	Routes        []string `toml:"routes" json:"routes"`
}
type TLS struct {
	SNI                string `toml:"sni" json:"sni"`
	CertFile           string `toml:"tls_cert" json:"tls_cert"`
	KeyFile            string `toml:"tls_key" json:"tls_key"`
	CAFile             string `toml:"ca_file" json:"ca_file"`
	InsecureSkipVerify bool   `toml:"insecure_skip_verify" json:"insecure_skip_verify"`
}
type Tuning struct {
	AutoTuning     bool   `toml:"auto_tuning" json:"auto_tuning"`
	Profile        string `toml:"tuning_profile" json:"tuning_profile"`
	Workers        int    `toml:"workers" json:"workers"`
	ChannelSize    int    `toml:"channel_size" json:"channel_size"`
	MSS            int    `toml:"tcp_mss" json:"tcp_mss"`
	ReceiveBuffer  int    `toml:"so_rcvbuf" json:"so_rcvbuf"`
	SendBuffer     int    `toml:"so_sndbuf" json:"so_sndbuf"`
	BufferProfile  string `toml:"buffer_profile" json:"buffer_profile"`
	TPacketProfile string `toml:"tpacket_profile" json:"tpacket_profile"`
	MaxConnections int    `toml:"max_connections" json:"max_connections"`
	BatchSize      int    `toml:"batch_size" json:"batch_size"`
	ReadTimeout    int    `toml:"read_timeout" json:"read_timeout"`
}
type Logging struct {
	Level string `toml:"log_level" json:"log_level"`
}
type Ports struct {
	Forwarder       string   `toml:"forwarder" json:"forwarder"`
	Mapping         []string `toml:"mapping" json:"mapping"`
	BackendSourceIP string   `toml:"backend_source_ip" json:"backend_source_ip"`
}
type UDP struct {
	RingSize        int `toml:"ring_size" json:"ring_size"`
	FrameSize       int `toml:"frame_size" json:"frame_size"`
	PeerIdleTimeout int `toml:"peer_idle_timeout_s" json:"peer_idle_timeout_s"`
	WriteTimeoutMS  int `toml:"write_timeout_ms" json:"write_timeout_ms"`
}
type IPX struct {
	Mode         string `toml:"mode" json:"mode"`
	Profile      string `toml:"profile" json:"profile"`
	Protocol     uint8  `toml:"protocol" json:"protocol"`
	ListenIP     string `toml:"listen_ip" json:"listen_ip"`
	DstIP        string `toml:"dst_ip" json:"dst_ip"`
	SpoofSrcIP   string `toml:"spoof_src_ip" json:"spoof_src_ip"`
	SpoofDstIP   string `toml:"spoof_dst_ip" json:"spoof_dst_ip"`
	Interface    string `toml:"interface" json:"interface"`
	ICMPType     uint8  `toml:"icmp_type" json:"icmp_type"`
	ICMPCode     uint8  `toml:"icmp_code" json:"icmp_code"`
	CustomPacket bool   `toml:"custom_packet" json:"custom_packet"`
	LocalPort    int    `toml:"local_port" json:"local_port"`
	PeerPort     int    `toml:"peer_port" json:"peer_port"`
	OuterMTU     int    `toml:"outer_mtu" json:"outer_mtu"`
	Queues       int    `toml:"queues" json:"queues"`
}
type Config struct {
	Listener  Listener  `toml:"listener" json:"listener"`
	Dialer    Dialer    `toml:"dialer" json:"dialer"`
	Transport Transport `toml:"transport" json:"transport"`
	Security  Security  `toml:"security" json:"security"`
	Mux       Mux       `toml:"mux" json:"mux"`
	TUN       TUN       `toml:"tun" json:"tun"`
	TLS       TLS       `toml:"tls" json:"tls"`
	Tuning    Tuning    `toml:"tuning" json:"tuning"`
	Logging   Logging   `toml:"logging" json:"logging"`
	Ports     Ports     `toml:"ports" json:"ports"`
	UDP       UDP       `toml:"accept_udp" json:"accept_udp"`
	IPX       IPX       `toml:"ipx" json:"ipx"`
}

func Load(path string) (Config, error) {
	var c Config
	f, err := os.Open(path)
	if err != nil {
		return c, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil {
		return c, err
	}
	if len(data) > 1024*1024 {
		return c, errors.New("configuration exceeds 1 MiB")
	}
	if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		d := json.NewDecoder(strings.NewReader(string(data)))
		d.DisallowUnknownFields()
		err = d.Decode(&c)
		if err == nil {
			var extra any
			if d.Decode(&extra) != io.EOF {
				err = errors.New("trailing configuration content")
			}
		}
	} else {
		var meta toml.MetaData
		meta, err = toml.Decode(string(data), &c)
		if err == nil && len(meta.Undecoded()) > 0 {
			err = fmt.Errorf("unknown configuration field: %s", meta.Undecoded()[0].String())
		}
	}
	if err != nil {
		return c, err
	}
	c.Defaults()
	return c, c.Validate()
}
func (c *Config) Defaults() {
	if c.Transport.Type == "" {
		c.Transport.Type = "tcp"
	}
	if c.Transport.ConnectionPool == 0 {
		c.Transport.ConnectionPool = 4
	}
	if c.Transport.KeepalivePeriod == 0 {
		c.Transport.KeepalivePeriod = 40
	}
	if c.Transport.HeartbeatInterval == 0 {
		c.Transport.HeartbeatInterval = 10
	}
	if c.Transport.HeartbeatTimeout == 0 {
		c.Transport.HeartbeatTimeout = 25
	}
	if c.Dialer.DialTimeout == 0 {
		c.Dialer.DialTimeout = 10
	}
	if c.Dialer.RetryInterval == 0 {
		c.Dialer.RetryInterval = 3
	}
	if c.Security.Algorithm == "" {
		c.Security.Algorithm = "aes-256-gcm"
	}
	if c.Security.KDFIterations == 0 {
		c.Security.KDFIterations = 100000
	}
	if c.Mux.Version == 0 {
		c.Mux.Version = 2
	}
	if c.Mux.FrameSize == 0 {
		c.Mux.FrameSize = 32768
	}
	if c.Mux.ReceiveBuffer == 0 {
		c.Mux.ReceiveBuffer = 4194304
	}
	if c.Mux.StreamBuffer == 0 {
		c.Mux.StreamBuffer = 2097152
	}
	if c.Mux.Concurrency == 0 {
		c.Mux.Concurrency = 64
	}
	if c.Tuning.ChannelSize == 0 {
		c.Tuning.ChannelSize = 4096
	}
	if c.Tuning.MaxConnections == 0 {
		c.Tuning.MaxConnections = 4096
	}
	if c.Tuning.ReadTimeout == 0 {
		c.Tuning.ReadTimeout = 120
	}
	if c.UDP.PeerIdleTimeout == 0 {
		c.UDP.PeerIdleTimeout = 120
	}
	if c.UDP.RingSize == 0 {
		c.UDP.RingSize = 64
	}
	if c.UDP.FrameSize == 0 {
		c.UDP.FrameSize = 2048
	}
	if c.UDP.WriteTimeoutMS == 0 {
		c.UDP.WriteTimeoutMS = 3
	}
	if c.TUN.Name == "" {
		c.TUN.Name = "backhaul"
	}
	if c.TUN.MTU == 0 {
		c.TUN.MTU = 1500
		if c.IsIPX() {
			c.TUN.MTU = 1320
		}
	}
	if c.TUN.Encapsulation == "" {
		c.TUN.Encapsulation = "tcp"
	}
	if c.Ports.Forwarder == "" {
		c.Ports.Forwarder = "backhaul"
	}
	if c.IPX.Profile == "" {
		c.IPX.Profile = "tcp"
	}
	if c.IPX.LocalPort == 0 {
		c.IPX.LocalPort = 443
	}
	if c.IPX.PeerPort == 0 {
		c.IPX.PeerPort = 443
	}
	if c.IPX.OuterMTU == 0 {
		c.IPX.OuterMTU = 1500
	}
	if c.IPX.Queues == 0 {
		c.IPX.Queues = 1
	}
}
func (c Config) IsIPX() bool { return c.Transport.Type == "tun" && c.TUN.Encapsulation == "ipx" }
func (c Config) Server() bool {
	if c.IsIPX() {
		return c.IPX.Mode == "server"
	}
	return c.Listener.BindAddr != "" || len(c.Listener.BindAddrs) > 0
}
func (c Config) Encrypted() bool {
	return c.Security.EnableEncryption == nil || *c.Security.EnableEncryption
}
func (c Config) Multiplexed() bool { return strings.HasSuffix(c.Transport.Type, "mux") }
func (c Config) ListenAddresses() []string {
	r := append([]string{}, c.Listener.BindAddrs...)
	if c.Listener.BindAddr != "" {
		r = append(r, c.Listener.BindAddr)
	}
	return r
}
func (c Config) PeerAddresses() []string {
	r := append([]string{}, c.Dialer.RemoteAddrs...)
	if c.Dialer.RemoteAddr != "" {
		r = append(r, c.Dialer.RemoteAddr)
	}
	return r
}
func (c Config) LocalAddresses() []string {
	r := append([]string{}, c.Dialer.LocalAddrs...)
	if c.Dialer.LocalAddr != "" {
		r = append(r, c.Dialer.LocalAddr)
	}
	if len(r) == 0 {
		r = []string{""}
	}
	return r
}
func validEndpoint(s string) error {
	h, p, e := net.SplitHostPort(s)
	if e != nil {
		return errors.New("endpoint must be host:port (IPv6 in brackets)")
	}
	if strings.ContainsAny(h, "\r\n\x00") {
		return errors.New("invalid endpoint host")
	}
	n, err := strconv.ParseUint(p, 10, 16)
	if err != nil || n == 0 {
		return errors.New("port outside 1..65535")
	}
	return nil
}
func (c Config) Validate() error {
	switch c.Transport.Type {
	case "tcp", "tcpmux", "xtcpmux", "ws", "wss", "wsmux", "wssmux", "xwsmux", "anytls", "tun":
	default:
		return errors.New("unsupported transport")
	}
	if c.Tuning.AutoTuning {
		return errors.New("auto_tuning is disabled in this rewrite; use false (no global kernel changes)")
	}
	for _, n := range []int{c.Tuning.Workers, c.Tuning.MSS, c.Tuning.ReceiveBuffer, c.Tuning.SendBuffer, c.Tuning.BatchSize} {
		if n < 0 {
			return errors.New("negative tuning value")
		}
	}
	if c.Tuning.MSS > 65535 || c.Tuning.ReceiveBuffer > 64<<20 || c.Tuning.SendBuffer > 64<<20 {
		return errors.New("socket tuning exceeds bounds")
	}
	if c.Tuning.TPacketProfile != "" {
		return errors.New("original TPACKET performance profiles are not reproduced")
	}
	if c.Tuning.ReadTimeout < 1 || c.Tuning.ReadTimeout > 86400 || c.Transport.KeepalivePeriod < 1 || c.Transport.KeepalivePeriod > 86400 {
		return errors.New("read timeout and keepalive must be within 1..86400 seconds")
	}
	if c.Tuning.Workers > 256 || c.Tuning.BatchSize > 65536 {
		return errors.New("worker or batch tuning exceeds bounds")
	}
	switch c.Tuning.BufferProfile {
	case "", "balanced", "extreme_low_cpu", "ultra_low_cpu", "low_cpu", "low_memory":
	default:
		return errors.New("unsupported buffer profile")
	}
	switch c.Tuning.Profile {
	case "", "balanced", "fast", "latency", "resource":
	default:
		return errors.New("unsupported tuning profile")
	}
	if c.Transport.ConnectionPool < 1 || c.Transport.ConnectionPool > 128 || c.Tuning.MaxConnections < 1 || c.Tuning.MaxConnections > 65536 || c.Tuning.ChannelSize < 1 || c.Tuning.ChannelSize > 65536 {
		return errors.New("pool or connection limits outside bounds")
	}
	if c.Dialer.DialTimeout < 1 || c.Dialer.DialTimeout > 300 || c.Dialer.RetryInterval < 1 || c.Dialer.RetryInterval > 300 || c.Transport.HeartbeatInterval < 1 || c.Transport.HeartbeatTimeout < c.Transport.HeartbeatInterval || c.Transport.HeartbeatTimeout > 600 {
		return errors.New("invalid timeout configuration")
	}
	if c.TUN.HealthPort < 0 || c.TUN.HealthPort > 65535 || c.TUN.MTU < 576 || c.TUN.MTU > 65535 {
		return errors.New("invalid health port or MTU")
	}
	if c.UDP.PeerIdleTimeout < 1 || c.UDP.PeerIdleTimeout > 86400 {
		return errors.New("UDP idle timeout outside bounds")
	}
	if c.UDP.RingSize < 1 || c.UDP.RingSize > 4096 || c.UDP.FrameSize < 512 || c.UDP.FrameSize > 65535 || c.UDP.WriteTimeoutMS < 1 || c.UDP.WriteTimeoutMS > 300000 {
		return errors.New("UDP queue, buffer or write timeout exceeds bounds")
	}
	if c.Mux.Version != 1 && c.Mux.Version != 2 {
		return errors.New("SMUX version must be 1 or 2")
	}
	if c.Mux.FrameSize < 512 || c.Mux.FrameSize > 65535 || c.Mux.StreamBuffer < 65536 || c.Mux.ReceiveBuffer < c.Mux.StreamBuffer || c.Mux.ReceiveBuffer > 64<<20 || c.Mux.Concurrency < 1 || c.Mux.Concurrency > 4096 {
		return errors.New("invalid SMUX bounds")
	}
	if c.IsIPX() {
		if c.IPX.Mode != "server" && c.IPX.Mode != "client" {
			return errors.New("ipx.mode must be server or client")
		}
		if c.IPX.CustomPacket {
			return errors.New("malformed custom_packet checksum bypass is not supported")
		}
		if len(c.ListenAddresses())+len(c.PeerAddresses()) != 0 {
			return errors.New("IPX uses ipx.listen_ip/dst_ip instead of listener/dialer")
		}
	} else {
		if c.Server() == (len(c.PeerAddresses()) > 0) {
			return errors.New("configure exactly one listener or dialer role")
		}
		addrs := append(c.ListenAddresses(), c.PeerAddresses()...)
		if len(addrs) > 32 {
			return errors.New("too many endpoints")
		}
		seen := map[string]bool{}
		for _, s := range addrs {
			if e := validEndpoint(s); e != nil {
				return e
			}
			if seen[s] {
				return errors.New("duplicate endpoint")
			}
			seen[s] = true
		}
		if len(c.Security.Token) < 16 || len(c.Security.Token) > 4096 {
			return errors.New("security.token must contain 16..4096 bytes; generate a unique local secret")
		}
		for _, source := range c.LocalAddresses() {
			if source == "" {
				continue
			}
			host := source
			if net.ParseIP(host) == nil {
				h, p, err := net.SplitHostPort(source)
				if err != nil {
					return errors.New("local source must be a literal IP or IP:port")
				}
				host = h
				if _, err = strconv.ParseUint(p, 10, 16); err != nil {
					return errors.New("invalid local source port")
				}
			}
			if net.ParseIP(host) == nil {
				return errors.New("local source must be a literal IP")
			}
		}
	}
	switch c.Security.Algorithm {
	case "aes-256-gcm", "aes-128-gcm", "chacha20-poly1305":
	default:
		return errors.New("unsupported encryption algorithm; original unused musix label is not a cipher")
	}
	if !c.Encrypted() && !c.Security.AllowInsecurePlaintext {
		return errors.New("plaintext requires explicit security.allow_insecure_plaintext=true")
	}
	if c.Transport.AcceptUDP && c.Transport.Type != "tcp" {
		return errors.New("accept_udp is supported by tcp; choose TUN for IP/UDP routing")
	}
	if c.Transport.Type == "tun" {
		if c.TUN.Encapsulation != "tcp" && c.TUN.Encapsulation != "ipx" {
			return errors.New("tun.encapsulation must be tcp or ipx")
		}
		if _, _, e := net.ParseCIDR(c.TUN.LocalAddr); e != nil {
			return errors.New("invalid tun.local_addr CIDR")
		}
		if _, _, e := net.ParseCIDR(c.TUN.RemoteAddr); e != nil {
			return errors.New("invalid tun.remote_addr CIDR")
		}
		if c.TUN.Encapsulation == "tcp" && len(c.ListenAddresses())+len(c.PeerAddresses()) != 1 {
			return errors.New("TCP TUN requires exactly one peer or listener")
		}
	}
	if c.Ports.BackendSourceIP != "" && net.ParseIP(c.Ports.BackendSourceIP) == nil {
		return errors.New("backend_source_ip must be a literal assigned IP")
	}
	tlsMode := c.Transport.Type == "wss" || c.Transport.Type == "wssmux" || c.Transport.Type == "anytls"
	if tlsMode && c.Server() {
		if c.TLS.CertFile == "" || c.TLS.KeyFile == "" {
			return errors.New("TLS listener requires local tls_cert and tls_key")
		}
		if _, err := tls.LoadX509KeyPair(c.TLS.CertFile, c.TLS.KeyFile); err != nil {
			return errors.New("local TLS certificate or private key is invalid")
		}
	}
	if tlsMode && !c.Server() && c.TLS.CAFile != "" {
		data, err := os.ReadFile(c.TLS.CAFile)
		if err != nil || len(data) > 8<<20 || !x509.NewCertPool().AppendCertsFromPEM(data) {
			return errors.New("local TLS trust certificate is invalid")
		}
	}
	if !c.Server() && len(c.Ports.Mapping) > 0 {
		return errors.New("forwarding mappings belong to server role")
	}
	if c.Ports.Forwarder != "backhaul" && c.Ports.Forwarder != "iptables" {
		return errors.New("unknown port forwarder")
	}
	if c.Ports.Forwarder == "iptables" {
		return errors.New("global iptables forwarding is replaced by owned userspace forwarding; choose backhaul")
	}
	if c.Transport.ProxyProtocol && c.Transport.Type == "tun" {
		return errors.New("PROXY protocol is a TCP forwarding option")
	}
	return nil
}
