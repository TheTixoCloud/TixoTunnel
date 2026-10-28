// Authenticated IPX security and parser tests by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func pair(profile, algorithm string) (Config, Config) {
	a := Config{Profile: profile, Role: "client", Interface: "lab0", LocalIP: "192.0.2.1", PeerIP: "192.0.2.2", LocalPort: 443, PeerPort: 444, TUNName: "bh0", LocalTUN: "198.18.0.1/30", PeerTUN: "198.18.0.2", MTU: 1320, OuterMTU: 1500, Key: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x42}, 32)), Algorithm: algorithm, KDFIterations: 10000}
	b := a
	b.Role = "server"
	b.LocalIP, b.PeerIP = a.PeerIP, a.LocalIP
	b.LocalPort, b.PeerPort = a.PeerPort, a.LocalPort
	b.LocalTUN = "198.18.0.2/30"
	b.PeerTUN = "198.18.0.1"
	return a, b
}
func inner(size int) []byte {
	p := make([]byte, size)
	p[0] = 0x45
	binary.BigEndian.PutUint16(p[2:4], uint16(size))
	p[8] = 64
	p[9] = 17
	copy(p[12:20], []byte{198, 18, 0, 1, 198, 18, 0, 2})
	for i := 20; i < size; i++ {
		p[i] = byte(i*31 + i/251)
	}
	binary.BigEndian.PutUint16(p[10:12], Checksum(p[:20]))
	return p
}

func TestAlgorithmsFragmentAndBidirectionalRecords(t *testing.T) {
	for _, algorithm := range []string{"aes128", "aes256", "chacha20poly1305"} {
		t.Run(algorithm, func(t *testing.T) {
			a, b := pair("udp", algorithm)
			tx, err := NewSession(a)
			if err != nil {
				t.Fatal(err)
			}
			rx, err := NewSession(b)
			if err != nil {
				t.Fatal(err)
			}
			for _, size := range []int{20, 1320, 65535} {
				original := inner(size)
				frames, e := tx.EncodePacket(original)
				if e != nil {
					t.Fatal(e)
				}
				seen := map[string]bool{}
				var decoded []byte
				for index := len(frames) - 1; index >= 0; index-- {
					frame := frames[index]
					if len(frame)+28 > 1500 {
						t.Fatal("outer MTU exceeded")
					}
					nonce := string(frame[16:28])
					if seen[nonce] {
						t.Fatal("nonce reused")
					}
					seen[nonce] = true
					decoded, e = rx.DecodePacket(frame, time.Now())
					if e != nil {
						t.Fatal(e)
					}
				}
				if !bytes.Equal(decoded, original) {
					t.Fatal("fragmented packet did not round trip")
				}
				if _, e = rx.DecodePacket(frames[0], time.Now()); !errors.Is(e, ErrReplay) {
					t.Fatal("authenticated replay admitted")
				}
			}
			reply := inner(1024)
			frames, e := rx.EncodePacket(reply)
			if e != nil {
				t.Fatal(e)
			}
			decoded, e := tx.DecodePacket(frames[0], time.Now())
			if e != nil || !bytes.Equal(decoded, reply) {
				t.Fatal("reverse directional key failed", e)
			}
		})
	}
}

func TestAuthenticationAndDirectionRejectBeforeReplayAllocation(t *testing.T) {
	a, b := pair("icmp", "aes256")
	tx, _ := NewSession(a)
	rx, _ := NewSession(b)
	frames, _ := tx.EncodePacket(inner(100))
	frame := frames[0]
	for _, index := range []int{0, 4, 5, 6, 7, 8, 16, 28, 36, 44, 52, 54, 56, 58, 60, HeaderSize, len(frame) - 1} {
		copyFrame := append([]byte(nil), frame...)
		copyFrame[index] ^= 1
		if _, err := rx.DecodePacket(copyFrame, time.Now()); err == nil {
			t.Fatalf("tampered field %d accepted", index)
		}
	}
	if len(rx.replay) != 0 || len(rx.fragments) != 0 {
		t.Fatal("unauthenticated data allocated replay/reassembly state")
	}
	wrong := b
	wrong.Key = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x41}, 32))
	wrongSession, _ := NewSession(wrong)
	if _, err := wrongSession.DecodePacket(frame, time.Now()); err == nil {
		t.Fatal("wrong secret accepted")
	}
	if _, err := tx.DecodePacket(frame, time.Now()); err == nil {
		t.Fatal("reflected sender direction accepted")
	}
	wrongRole := b
	wrongRole.Role = "client"
	bad, _ := NewSession(wrongRole)
	if _, err := bad.DecodePacket(frame, time.Now()); err == nil {
		t.Fatal("same role peer accepted")
	}
}

func TestIncompletePacketSetBound(t *testing.T) {
	a, b := pair("udp", "aes256")
	tx, _ := NewSession(a)
	rx, _ := NewSession(b)
	now := time.Now()
	for index := 0; index < 65; index++ {
		frames, err := tx.EncodePacket(inner(3000))
		if err != nil {
			t.Fatal(err)
		}
		_, err = rx.DecodePacket(frames[0], now)
		if index < 64 && err != nil {
			t.Fatal(err)
		}
		if index == 64 && !errors.Is(err, ErrLimits) {
			t.Fatal("incomplete packet limit was not enforced", err)
		}
	}
	if len(rx.fragments) != 64 || rx.fragmentBytes > 4*1024*1024 {
		t.Fatal("retained fragment memory exceeded bounds")
	}
}

func TestFreshSessionNonceAndEpoch(t *testing.T) {
	a, _ := pair("bip", "aes256")
	first, _ := NewSession(a)
	second, _ := NewSession(a)
	f, _ := first.EncodePacket(inner(50))
	g, _ := second.EncodePacket(inner(50))
	if bytes.Equal(f[0][16:28], g[0][16:28]) || bytes.Equal(f[0][28:36], g[0][28:36]) {
		t.Fatal("fresh process state reused")
	}
	if bytes.Equal(f[0], g[0]) {
		t.Fatal("identical ciphertext across fresh sessions")
	}
}

func TestReassemblyExpiryAndCapacity(t *testing.T) {
	a, b := pair("gre", "aes128")
	tx, _ := NewSession(a)
	rx, _ := NewSession(b)
	now := time.Now()
	frames, _ := tx.EncodePacket(inner(4000))
	if value, err := rx.DecodePacket(frames[0], now); err != nil || value != nil {
		t.Fatal("first fragment should wait", err)
	}
	if len(rx.fragments) != 1 {
		t.Fatal("fragment not retained")
	}
	for _, frame := range frames[1:] {
		if result, err := rx.DecodePacket(frame, now.Add(ReassemblyTTL+time.Second)); err != nil || result != nil {
			t.Fatal("expired packet reconstructed", err)
		}
	}
	if rx.fragmentBytes > MaximumPacket {
		t.Fatal("expiry retained old fragment bytes")
	}
	// Complete authenticated records from separate fresh sender epochs prove the
	// bounded epoch table refuses capacity growth rather than evicting live state.
	for index := 0; index < 32; index++ {
		sender, _ := NewSession(a)
		list, _ := sender.EncodePacket(inner(20))
		_, err := rx.DecodePacket(list[0], now)
		if index < 31 && err != nil {
			t.Fatal(err)
		}
		if index == 31 && !errors.Is(err, ErrLimits) {
			t.Fatal("replay epoch capacity was not bounded", err)
		}
	}
}

func TestReplaySlidingWindowReordering(t *testing.T) {
	w := new(replayWindow)
	for _, value := range []uint64{5000, 4999, 4998, 5002, 5001, 4096, 9050} {
		if !w.accept(value) {
			t.Fatal("fresh sequence rejected", value)
		}
	}
	for _, value := range []uint64{0, 9050, 5000, 4999, 1} {
		if w.accept(value) {
			t.Fatal("duplicate or old sequence admitted", value)
		}
	}
}

func TestPlaintextRequiresExplicitOptInAndCannotDowngrade(t *testing.T) {
	a, b := pair("ipip", "aes256")
	a.Key = ""
	if _, err := NewSession(a); err == nil {
		t.Fatal("implicit plaintext allowed")
	}
	a.AllowInsecurePlaintext = true
	b.AllowInsecurePlaintext = true
	b.Key = ""
	tx, _ := NewSession(a)
	rx, _ := NewSession(b)
	frames, _ := tx.EncodePacket(inner(100))
	p, err := rx.DecodePacket(frames[0], time.Now())
	if err != nil || !bytes.Equal(p, inner(100)) {
		t.Fatal(err)
	}
	secureA, secureB := pair("ipip", "aes256")
	secure, _ := NewSession(secureB)
	secureFrames, _ := NewSession(secureA)
	list, _ := secureFrames.EncodePacket(inner(100))
	list[0][5] = plainFlag
	if _, err = secure.DecodePacket(list[0], time.Now()); err == nil {
		t.Fatal("plaintext downgrade accepted")
	}
}
