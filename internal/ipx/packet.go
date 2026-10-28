// Six real IPv4 outer codecs by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"encoding/binary"
	"net/netip"
)

// Codec emits complete IPv4 packets. TCP is a raw 20-byte TCP-shaped header;
// there is no kernel TCP handshake or TCP stream reliability at this layer.
type Codec struct{ s *settings }

func NewCodec(c Config) (*Codec, error) {
	s, err := normalized(c)
	if err != nil {
		return nil, err
	}
	return &Codec{s: s}, nil
}

func Checksum(data []byte) uint16 {
	var sum uint64
	for len(data) > 1 {
		sum += uint64(binary.BigEndian.Uint16(data))
		data = data[2:]
	}
	if len(data) > 0 {
		sum += uint64(data[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 65535) + (sum >> 16)
	}
	return ^uint16(sum)
}

func transportChecksum(src, dst netip.Addr, protocol byte, data []byte) uint16 {
	pseudo := make([]byte, 12+len(data))
	a, b := src.As4(), dst.As4()
	copy(pseudo[:4], a[:])
	copy(pseudo[4:8], b[:])
	pseudo[9] = protocol
	binary.BigEndian.PutUint16(pseudo[10:12], uint16(len(data)))
	copy(pseudo[12:], data)
	return Checksum(pseudo)
}

func (c *Codec) Encode(payload []byte, sequence uint32) ([]byte, error) {
	header := 20 + profileHeader(c.s.Profile)
	if len(payload) < HeaderSize || len(payload)+header > c.s.OuterMTU {
		return nil, ErrPacket
	}
	packet := make([]byte, header+len(payload))
	packet[0] = 0x45
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	binary.BigEndian.PutUint16(packet[4:6], uint16(sequence))
	binary.BigEndian.PutUint16(packet[6:8], 0x4000)
	packet[8] = 64
	packet[9] = c.s.protocol
	a, b := c.s.source.As4(), c.s.peer.As4()
	copy(packet[12:16], a[:])
	copy(packet[16:20], b[:])
	p := packet[20:]
	copy(packet[header:], payload)
	switch c.s.Profile {
	case "icmp":
		p[0] = 8
		if c.s.Role == "server" {
			p[0] = 0
		}
		if c.s.ICMPType != nil {
			p[0] = *c.s.ICMPType
		}
		if c.s.ICMPCode != nil {
			p[1] = *c.s.ICMPCode
		}
		binary.BigEndian.PutUint16(p[4:6], 0x4258)
		binary.BigEndian.PutUint16(p[6:8], uint16(sequence))
		binary.BigEndian.PutUint16(p[2:4], Checksum(p))
	case "udp":
		binary.BigEndian.PutUint16(p[:2], uint16(c.s.LocalPort))
		binary.BigEndian.PutUint16(p[2:4], uint16(c.s.PeerPort))
		binary.BigEndian.PutUint16(p[4:6], uint16(len(p)))
		sum := transportChecksum(c.s.source, c.s.peer, 17, p)
		if sum == 0 {
			sum = 65535
		}
		binary.BigEndian.PutUint16(p[6:8], sum)
	case "tcp":
		binary.BigEndian.PutUint16(p[:2], uint16(c.s.LocalPort))
		binary.BigEndian.PutUint16(p[2:4], uint16(c.s.PeerPort))
		binary.BigEndian.PutUint32(p[4:8], sequence)
		binary.BigEndian.PutUint32(p[8:12], 1)
		p[12] = 0x50
		p[13] = 0x18
		binary.BigEndian.PutUint16(p[14:16], 14600)
		binary.BigEndian.PutUint16(p[16:18], transportChecksum(c.s.source, c.s.peer, 6, p))
	case "gre":
		binary.BigEndian.PutUint16(p[2:4], 0x0800)
	}
	binary.BigEndian.PutUint16(packet[10:12], Checksum(packet[:20]))
	return packet, nil
}

func (c *Codec) Decode(packet []byte) ([]byte, error) {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		return nil, ErrPacket
	}
	header := int(packet[0]&15) * 4
	total := int(binary.BigEndian.Uint16(packet[2:4]))
	if header < 20 || header > 60 || header > len(packet) || total < header || total > len(packet) || total > c.s.OuterMTU || Checksum(packet[:header]) != 0 || packet[8] == 0 || binary.BigEndian.Uint16(packet[6:8])&0xbfff != 0 || packet[9] != c.s.protocol {
		return nil, ErrPacket
	}
	a, b := [4]byte{}, [4]byte{}
	copy(a[:], packet[12:16])
	copy(b[:], packet[16:20])
	src, dst := netip.AddrFrom4(a), netip.AddrFrom4(b)
	if src != c.s.expectedSource || dst != c.s.local {
		return nil, ErrPacket
	}
	p := packet[header:total]
	size := profileHeader(c.s.Profile)
	if len(p) < size+HeaderSize {
		return nil, ErrPacket
	}
	switch c.s.Profile {
	case "icmp":
		expected := byte(0)
		if c.s.Role == "server" {
			expected = 8
		}
		if c.s.PeerICMPType != nil {
			expected = *c.s.PeerICMPType
		}
		code := byte(0)
		if c.s.PeerICMPCode != nil {
			code = *c.s.PeerICMPCode
		}
		if p[0] != expected || p[1] != code || binary.BigEndian.Uint16(p[4:6]) != 0x4258 || Checksum(p) != 0 {
			return nil, ErrPacket
		}
	case "udp":
		if int(binary.BigEndian.Uint16(p[:2])) != c.s.PeerPort || int(binary.BigEndian.Uint16(p[2:4])) != c.s.LocalPort || int(binary.BigEndian.Uint16(p[4:6])) != len(p) || binary.BigEndian.Uint16(p[6:8]) == 0 || transportChecksum(src, dst, 17, p) != 0 {
			return nil, ErrPacket
		}
	case "tcp":
		if int(binary.BigEndian.Uint16(p[:2])) != c.s.PeerPort || int(binary.BigEndian.Uint16(p[2:4])) != c.s.LocalPort || p[12] != 0x50 || p[13] != 0x18 || transportChecksum(src, dst, 6, p) != 0 {
			return nil, ErrPacket
		}
	case "gre":
		if binary.BigEndian.Uint16(p[:2]) != 0 || binary.BigEndian.Uint16(p[2:4]) != 0x0800 {
			return nil, ErrPacket
		}
	}
	return p[size:], nil
}
