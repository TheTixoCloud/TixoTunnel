// Genuine outer-profile/checksum tests by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestInternetChecksumKnownVectors(t *testing.T) {
	for _, test := range []struct {
		data     []byte
		expected uint16
	}{
		{[]byte{}, 0xffff},
		{[]byte{0, 1, 0xf2, 3, 0xf4, 0xf5, 0xf6, 0xf7}, 0x220d},
		{[]byte("123456789"), 0xf62a},
	} {
		if actual := Checksum(test.data); actual != test.expected {
			t.Fatalf("checksum=%04x, expected=%04x", actual, test.expected)
		}
	}
}

func TestSixOuterProfilesAndStrictAdmission(t *testing.T) {
	protocols := map[string]byte{"icmp": 1, "ipip": 4, "tcp": 6, "udp": 17, "gre": 47, "bip": 58}
	for profile, protocol := range protocols {
		t.Run(profile, func(t *testing.T) {
			a, b := pair(profile, "aes256")
			tx, err := NewCodec(a)
			if err != nil {
				t.Fatal(err)
			}
			rx, err := NewCodec(b)
			if err != nil {
				t.Fatal(err)
			}
			payload := bytes.Repeat([]byte{0xa5}, 100)
			packet, err := tx.Encode(payload, 37)
			if err != nil {
				t.Fatal(err)
			}
			if packet[9] != protocol || Checksum(packet[:20]) != 0 {
				t.Fatal("wrong IPv4 profile or checksum")
			}
			if len(packet) != 20+profileHeader(profile)+len(payload) {
				t.Fatal("outer subheader missing")
			}
			value, err := rx.Decode(packet)
			if err != nil || !bytes.Equal(value, payload) {
				t.Fatal("outer record roundtrip failed", err)
			}
			for _, index := range []int{0, 2, 6, 9, 10, 12, 16} {
				bad := append([]byte(nil), packet...)
				bad[index] ^= 1
				if _, err = rx.Decode(bad); err == nil {
					t.Fatalf("bad IPv4 field %d accepted", index)
				}
			}
			if profile == "tcp" || profile == "udp" || profile == "icmp" {
				bad := append([]byte(nil), packet...)
				bad[len(bad)-1] ^= 1
				if _, err = rx.Decode(bad); err == nil {
					t.Fatal("bad transport checksum accepted")
				}
			}
			if profile == "gre" && binary.BigEndian.Uint32(packet[20:24]) != 0x00000800 {
				t.Fatal("GRE header not real version-zero IPv4 encapsulation")
			}
		})
	}
}

func TestPairedSpoofUsesRealDestinationAndExpectedPeerSource(t *testing.T) {
	a, b := pair("tcp", "aes256")
	a.SpoofSrcIP = "198.51.100.9"
	b.SpoofDstIP = a.SpoofSrcIP
	b.SpoofSrcIP = "203.0.113.9"
	a.SpoofDstIP = b.SpoofSrcIP
	tx, _ := NewCodec(a)
	rx, _ := NewCodec(b)
	p, _ := tx.Encode(bytes.Repeat([]byte{7}, 100), 3)
	if !bytes.Equal(p[12:16], []byte{198, 51, 100, 9}) || !bytes.Equal(p[16:20], []byte{192, 0, 2, 2}) {
		t.Fatal("spoof changed real destination")
	}
	if _, err := rx.Decode(p); err != nil {
		t.Fatal("paired expected spoof was not admitted", err)
	}
	b.SpoofDstIP = ""
	wrong, _ := NewCodec(b)
	if _, err := wrong.Decode(p); err == nil {
		t.Fatal("unconfigured spoof source admitted")
	}
}

func TestBIPProtocolAndConfigurationValidation(t *testing.T) {
	a, _ := pair("bip", "aes256")
	a.Protocol = 99
	c, err := NewCodec(a)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := c.Encode(bytes.Repeat([]byte{1}, 80), 1)
	if p[9] != 99 {
		t.Fatal("custom BIP protocol ignored")
	}
	a.Protocol = 17
	if err = Validate(a); err == nil {
		t.Fatal("BIP structured-protocol collision admitted")
	}
	a, _ = pair("udp", "aes256")
	a.Protocol = 99
	if err = Validate(a); err == nil {
		t.Fatal("malformed UDP protocol override admitted")
	}
	for _, change := range []func(*Config){func(c *Config) { c.LocalIP = "0.0.0.0" }, func(c *Config) { c.PeerIP = "example.com" }, func(c *Config) { c.LocalIP = c.PeerIP }, func(c *Config) { c.LocalTUN = "198.18.0.1" }, func(c *Config) { c.Role = "both" }, func(c *Config) { c.Key = "password" }, func(c *Config) { c.KDFIterations = 1 }, func(c *Config) { c.OuterMTU = 10 }, func(c *Config) { c.Algorithm = "none" }} {
		copy := a
		copy.Protocol = 0
		change(&copy)
		if Validate(copy) == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}

func TestExplicitICMPTypeCodeAndImmutablePointers(t *testing.T) {
	a, b := pair("icmp", "aes256")
	typeValue, codeValue := uint8(13), uint8(7)
	a.ICMPType, a.ICMPCode = &typeValue, &codeValue
	b.PeerICMPType, b.PeerICMPCode = &typeValue, &codeValue
	tx, err := NewCodec(a)
	if err != nil {
		t.Fatal(err)
	}
	rx, err := NewCodec(b)
	if err != nil {
		t.Fatal(err)
	}
	typeValue, codeValue = 12, 8
	packet, err := tx.Encode(bytes.Repeat([]byte{5}, 100), 1)
	if err != nil {
		t.Fatal(err)
	}
	if packet[20] != 13 || packet[21] != 7 || Checksum(packet[20:]) != 0 {
		t.Fatal("explicit ICMP header or immutable configuration failed")
	}
	if _, err = rx.Decode(packet); err != nil {
		t.Fatal("configured peer ICMP override rejected", err)
	}
	_, defaultPeer := pair("icmp", "aes256")
	defaultCodec, _ := NewCodec(defaultPeer)
	if _, err = defaultCodec.Decode(packet); err == nil {
		t.Fatal("unexpected ICMP type/code admitted")
	}
}
