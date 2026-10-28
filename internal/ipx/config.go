// Independent authenticated IPX reconstruction by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	MaximumPacket    = 65535
	DefaultOuterMTU  = 1500
	DefaultMTU       = 1320
	HeaderSize       = 64
	TagSize          = 16
	MaximumFragments = 256
	ReassemblyTTL    = 5 * time.Second
	ReplayTTL        = 10 * time.Minute
)

var (
	ErrPacket = errors.New("IPX: malformed or unadmitted packet")
	ErrReplay = errors.New("IPX: duplicate authenticated datagram")
	ErrLimits = errors.New("IPX: bounded reassembly or replay capacity reached")
)

// Config uses literal outer IPv4 addresses. The actual local IP must be assigned
// to Interface. SpoofSrcIP affects only transmitted source; SpoofDstIP declares
// the expected peer source, while the destination always remains PeerIP.
type Config struct {
	Profile, Role, Interface, LocalIP, PeerIP, SpoofSrcIP, SpoofDstIP string
	LocalPort, PeerPort                                               int
	Protocol                                                          uint8
	ICMPType, ICMPCode, PeerICMPType, PeerICMPCode                    *uint8
	TUNName, LocalTUN, PeerTUN                                        string
	Routes                                                            []string
	MTU, OuterMTU, Queues                                             int
	Algorithm, Key                                                    string
	KDFIterations                                                     int
	AllowInsecurePlaintext                                            bool
	OnReady                                                           func(Info)
	OnStats                                                           func(Stats)
}

type Info struct {
	TUNName, Profile, LocalTUN, PeerTUN string
	MTU, OuterMTU                       int
}
type Stats struct {
	TxPackets, RxPackets, TxBytes, RxBytes, Dropped, ReplayRejected uint64
}

type settings struct {
	Config
	local, peer, source, expectedSource netip.Addr
	localTun, peerTun                   netip.Addr
	secret                              []byte
	protocol                            uint8
	direction                           byte
	context                             string
}

func address(value string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(value)
	if err != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() || ip == netip.AddrFrom4([4]byte{255, 255, 255, 255}) {
		return netip.Addr{}, errors.New("IPX requires literal unicast outer IPv4 addresses")
	}
	return ip, nil
}
func tunAddress(value string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		p, e := netip.ParsePrefix(value)
		if e != nil {
			return netip.Addr{}, e
		}
		ip = p.Addr()
	}
	if ip.Is4In6() || ip.IsUnspecified() || ip.IsMulticast() {
		return netip.Addr{}, ErrPacket
	}
	return ip, nil
}

func interfaceName(value string) bool {
	if len(value) < 1 || len(value) > 15 {
		return false
	}
	for index, ch := range []byte(value) {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
			continue
		}
		if index > 0 && ((ch >= '0' && ch <= '9') || ch == '_' || ch == '-' || ch == '.') {
			continue
		}
		return false
	}
	return true
}

func normalized(c Config) (*settings, error) {
	s := &settings{Config: c}
	s.Routes = append([]string(nil), c.Routes...)
	cloneByte := func(value *uint8) *uint8 {
		if value == nil {
			return nil
		}
		copy := *value
		return &copy
	}
	s.ICMPType, s.ICMPCode = cloneByte(c.ICMPType), cloneByte(c.ICMPCode)
	s.PeerICMPType, s.PeerICMPCode = cloneByte(c.PeerICMPType), cloneByte(c.PeerICMPCode)
	s.Profile = strings.ToLower(strings.TrimSpace(c.Profile))
	s.Role = strings.ToLower(strings.TrimSpace(c.Role))
	if s.Role != "client" && s.Role != "server" {
		return nil, errors.New("IPX role must be client or server")
	}
	protocols := map[string]uint8{"icmp": 1, "ipip": 4, "tcp": 6, "udp": 17, "gre": 47, "bip": 58}
	s.protocol = protocols[s.Profile]
	if s.protocol == 0 {
		return nil, errors.New("IPX supports icmp, ipip, tcp, udp, gre and bip")
	}
	if s.Profile != "icmp" && (s.ICMPType != nil || s.ICMPCode != nil || s.PeerICMPType != nil || s.PeerICMPCode != nil) {
		return nil, errors.New("ICMP overrides require the icmp profile")
	}
	if c.Protocol != 0 && c.Protocol != s.protocol {
		if s.Profile != "bip" {
			return nil, errors.New("a custom IP protocol is supported only with the explicit bip profile")
		}
		if c.Protocol == 1 || c.Protocol == 4 || c.Protocol == 6 || c.Protocol == 17 || c.Protocol == 47 {
			return nil, errors.New("custom bip protocol conflicts with a structured carrier")
		}
		s.protocol = c.Protocol
	}
	if s.MTU == 0 {
		s.MTU = DefaultMTU
	}
	if s.OuterMTU == 0 {
		s.OuterMTU = DefaultOuterMTU
	}
	if s.Queues == 0 {
		s.Queues = 1
	}
	if s.MTU < 576 || s.MTU > MaximumPacket || s.OuterMTU < 576 || s.OuterMTU > 9000 || s.Queues < 1 || s.Queues > 64 {
		return nil, errors.New("invalid IPX MTU or queue count")
	}
	if !interfaceName(s.TUNName) || (s.Interface != "" && !interfaceName(s.Interface)) || len(s.Routes) > 256 {
		return nil, errors.New("invalid IPX interface name")
	}
	if s.LocalPort == 0 {
		s.LocalPort = 443
	}
	if s.PeerPort == 0 {
		s.PeerPort = 443
	}
	if s.LocalPort < 1 || s.LocalPort > 65535 || s.PeerPort < 1 || s.PeerPort > 65535 {
		return nil, errors.New("invalid IPX port")
	}
	var err error
	if s.local, err = address(c.LocalIP); err != nil {
		return nil, err
	}
	if s.peer, err = address(c.PeerIP); err != nil {
		return nil, err
	}
	if s.local == s.peer {
		return nil, errors.New("IPX real endpoints must differ")
	}
	s.source, s.expectedSource = s.local, s.peer
	if c.SpoofSrcIP != "" {
		if s.source, err = address(c.SpoofSrcIP); err != nil {
			return nil, err
		}
	}
	if c.SpoofDstIP != "" {
		if s.expectedSource, err = address(c.SpoofDstIP); err != nil {
			return nil, err
		}
	}
	localPrefix, err := netip.ParsePrefix(c.LocalTUN)
	if err != nil {
		return nil, errors.New("IPX local TUN address needs a CIDR prefix")
	}
	s.localTun = localPrefix.Addr()
	if s.peerTun, err = tunAddress(c.PeerTUN); err != nil || s.localTun.Is4In6() || s.localTun.IsUnspecified() || s.localTun.IsMulticast() || s.localTun == s.peerTun || s.localTun.Is4() != s.peerTun.Is4() {
		return nil, errors.New("IPX needs distinct local/peer TUN addresses in one address family")
	}
	if s.localTun.Is6() && s.MTU < 1280 {
		return nil, errors.New("IPv6 TUN requires MTU at least 1280")
	}
	for _, r := range c.Routes {
		p, e := netip.ParsePrefix(r)
		if e != nil || p.Addr().Is4In6() {
			return nil, errors.New("invalid IPX route")
		}
	}
	s.Algorithm = strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(c.Algorithm))
	switch s.Algorithm {
	case "", "aes256", "aes256gcm", "aesgcm":
		s.Algorithm = "aes256"
	case "aes128", "aes128gcm":
		s.Algorithm = "aes128"
	case "chacha20", "chacha20poly1305", "chacha":
		s.Algorithm = "chacha20poly1305"
	default:
		return nil, errors.New("IPX algorithm must be aes256, aes128 or chacha20poly1305")
	}
	if s.KDFIterations == 0 {
		s.KDFIterations = 100000
	}
	if s.KDFIterations < 10000 || s.KDFIterations > 1000000 {
		return nil, errors.New("IPX KDF iterations must be 10000..1000000")
	}
	if !c.AllowInsecurePlaintext || c.Key != "" {
		s.secret, err = base64.StdEncoding.DecodeString(c.Key)
		if err != nil {
			s.secret, err = base64.RawStdEncoding.DecodeString(c.Key)
		}
		if err != nil || len(s.secret) < 16 || len(s.secret) > 128 {
			return nil, errors.New("IPX key must be base64 encoding 16..128 secret bytes; 32 bytes recommended")
		}
	}
	// Endpoints are canonicalized as pairs so swapped role, endpoint or TUN
	// configuration cannot silently share a directional key context.
	localTuple := s.local.String() + ":" + strconv.Itoa(s.LocalPort) + "/" + s.localTun.String()
	peerTuple := s.peer.String() + ":" + strconv.Itoa(s.PeerPort) + "/" + s.peerTun.String()
	clientTuple, serverTuple := localTuple, peerTuple
	if s.Role == "server" {
		clientTuple, serverTuple = peerTuple, localTuple
		s.direction = 1
	}
	s.context = fmt.Sprintf("backhaul-ir_spoof-ipx-v2|%s|%d|%s|client=%s|server=%s", s.Profile, s.protocol, s.Algorithm, clientTuple, serverTuple)
	return s, nil
}

func Validate(c Config) error { _, err := normalized(c); return err }

func profileHeader(profile string) int {
	switch profile {
	case "icmp", "udp":
		return 8
	case "tcp":
		return 20
	case "gre":
		return 4
	default:
		return 0
	}
}
