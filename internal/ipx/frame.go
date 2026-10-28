// Versioned authenticated datagrams by ir_spoof; https://t.me/ir_spoof.
package ipx

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

const plainFlag = 1

// Session owns fresh cryptographic entropy and replay/reassembly state. It is
// safe for concurrent packet encoders; decode is serialized by its own mutex.
type Session struct {
	s             *settings
	tx, rx        cipher.AEAD
	sid           [8]byte
	epoch         [8]byte
	sequence      uint64
	txMu, rxMu    sync.Mutex
	replay        map[[8]byte]*replayWindow
	fragments     map[fragmentKey]*fragmentSet
	fragmentBytes int
}

type replayWindow struct {
	highest  uint64
	bits     [64]uint64
	deadline time.Time
}
type fragmentKey struct {
	epoch [8]byte
	id    [8]byte
}
type fragmentSet struct {
	parts                  [][]byte
	total, received, bytes int
	deadline               time.Time
}

func randomFull(p []byte) error { _, err := io.ReadFull(rand.Reader, p); return err }

func NewSession(c Config) (*Session, error) {
	s, err := normalized(c)
	if err != nil {
		return nil, err
	}
	v := &Session{s: s, replay: make(map[[8]byte]*replayWindow), fragments: make(map[fragmentKey]*fragmentSet)}
	if err = randomFull(v.epoch[:]); err != nil {
		return nil, err
	}
	if s.AllowInsecurePlaintext {
		digest := sha256.Sum256([]byte(s.context))
		copy(v.sid[:], digest[:8])
		return v, nil
	}
	// Retain the public original PBKDF2 salt, then separate every new key with
	// a versioned directional HKDF context. This is not legacy wire compatibility.
	base := pbkdf2.Key(s.secret, []byte("bifrost-udp-crypto-v1"), s.KDFIterations, 32, sha256.New)
	defer clear(base)
	mac := hmac.New(sha256.New, base)
	_, _ = mac.Write([]byte("session-id|" + s.context))
	copy(v.sid[:], mac.Sum(nil)[:8])
	makeCipher := func(direction byte) (cipher.AEAD, error) {
		length := 32
		if s.Algorithm == "aes128" {
			length = 16
		}
		key := make([]byte, length)
		reader := hkdf.New(sha256.New, base, []byte(s.context), append([]byte("IPX-v2-direction|"), direction))
		if _, e := io.ReadFull(reader, key); e != nil {
			return nil, e
		}
		defer clear(key)
		if s.Algorithm == "chacha20poly1305" {
			return chacha20poly1305.New(key)
		}
		block, e := aes.NewCipher(key)
		if e != nil {
			return nil, e
		}
		return cipher.NewGCM(block)
	}
	if v.tx, err = makeCipher(s.direction); err != nil {
		return nil, err
	}
	if v.rx, err = makeCipher(s.direction ^ 1); err != nil {
		return nil, err
	}
	return v, nil
}

// ValidInnerPacket accepts exactly one complete IPv4 or IPv6 packet. IP options,
// IPv6 extension headers and inner fragments stay opaque to the TUN carrier.
func ValidInnerPacket(packet []byte) bool {
	if len(packet) < 1 || len(packet) > MaximumPacket {
		return false
	}
	switch packet[0] >> 4 {
	case 4:
		return len(packet) >= 20 && int(packet[0]&15)*4 >= 20 && int(packet[0]&15)*4 <= len(packet) && int(binary.BigEndian.Uint16(packet[2:4])) == len(packet)
	case 6:
		return len(packet) >= 40 && int(binary.BigEndian.Uint16(packet[4:6]))+40 == len(packet)
	default:
		return false
	}
}

// EncodePacket fragments before encryption. Every fragment gets a full fresh
// random 96-bit nonce, authenticated epoch/sequence and authenticated metadata.
func (v *Session) EncodePacket(packet []byte) ([][]byte, error) {
	if !ValidInnerPacket(packet) {
		return nil, ErrPacket
	}
	v.txMu.Lock()
	defer v.txMu.Unlock()
	maximum := v.s.OuterMTU - 20 - profileHeader(v.s.Profile) - HeaderSize
	if v.tx != nil {
		maximum -= v.tx.Overhead()
	}
	count := (len(packet) + maximum - 1) / maximum
	if maximum < 1 || count < 1 || count > MaximumFragments || uint64(count) > math.MaxUint64-v.sequence {
		return nil, ErrLimits
	}
	var id [8]byte
	if err := randomFull(id[:]); err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, count)
	for index := 0; index < count; index++ {
		part := packet[index*maximum : min((index+1)*maximum, len(packet))]
		h := make([]byte, HeaderSize)
		copy(h, "BHX2")
		h[4] = 2
		h[6] = v.s.protocol
		h[7] = v.s.direction
		copy(h[8:16], v.sid[:])
		if v.tx == nil {
			h[5] = plainFlag
		}
		if err := randomFull(h[16:28]); err != nil {
			return nil, err
		}
		copy(h[28:36], v.epoch[:])
		v.sequence++
		binary.BigEndian.PutUint64(h[36:44], v.sequence)
		copy(h[44:52], id[:])
		binary.BigEndian.PutUint16(h[52:54], uint16(index))
		binary.BigEndian.PutUint16(h[54:56], uint16(count))
		binary.BigEndian.PutUint16(h[56:58], uint16(len(packet)))
		binary.BigEndian.PutUint16(h[58:60], uint16(len(part)))
		if v.tx != nil {
			frames = append(frames, v.tx.Seal(h, h[16:28], part, h))
		} else {
			frames = append(frames, append(h, part...))
		}
	}
	return frames, nil
}

func (w *replayWindow) accept(sequence uint64) bool {
	if sequence == 0 {
		return false
	}
	if sequence > w.highest {
		shift := sequence - w.highest
		if shift >= 4096 {
			clear(w.bits[:])
		} else {
			words, bits := int(shift/64), uint(shift%64)
			for index := len(w.bits) - 1; index >= 0; index-- {
				var next uint64
				if index >= words {
					next = w.bits[index-words] << bits
					if bits > 0 && index > words {
						next |= w.bits[index-words-1] >> (64 - bits)
					}
				}
				w.bits[index] = next
			}
		}
		w.highest = sequence
		w.bits[0] |= 1
		return true
	}
	gap := w.highest - sequence
	if gap >= 4096 {
		return false
	}
	mask := uint64(1) << uint(gap%64)
	if w.bits[gap/64]&mask != 0 {
		return false
	}
	w.bits[gap/64] |= mask
	return true
}

func (v *Session) expire(now time.Time) {
	for key, set := range v.fragments {
		if !now.Before(set.deadline) {
			v.fragmentBytes -= set.bytes
			delete(v.fragments, key)
		}
	}
	for key, window := range v.replay {
		if !now.Before(window.deadline) {
			delete(v.replay, key)
		}
	}
}

// DecodePacket authenticates first, then applies the replay window and bounded
// reassembly. Incomplete fragmented packets return nil,nil. Duplicate fragments
// and invalid metadata never extend deadlines or allocate unbounded memory.
func (v *Session) DecodePacket(frame []byte, now time.Time) ([]byte, error) {
	if len(frame) < HeaderSize || len(frame) > v.s.OuterMTU-20-profileHeader(v.s.Profile) {
		return nil, ErrPacket
	}
	h := frame[:HeaderSize]
	if !bytes.Equal(h[:4], []byte("BHX2")) || h[4] != 2 || h[5]&^byte(plainFlag) != 0 || h[6] != v.s.protocol || h[7] != v.s.direction^1 || !hmac.Equal(h[8:16], v.sid[:]) || !bytes.Equal(h[60:64], []byte{0, 0, 0, 0}) {
		return nil, ErrPacket
	}
	if (v.rx == nil) != (h[5] == plainFlag) {
		return nil, ErrPacket
	}
	index, count, total, size := int(binary.BigEndian.Uint16(h[52:54])), int(binary.BigEndian.Uint16(h[54:56])), int(binary.BigEndian.Uint16(h[56:58])), int(binary.BigEndian.Uint16(h[58:60]))
	if count < 1 || count > MaximumFragments || index >= count || total < 20 || total > MaximumPacket || size < 1 || size > total || count > total {
		return nil, ErrPacket
	}
	if count == 1 && (index != 0 || size != total) {
		return nil, ErrPacket
	}
	var payload []byte
	var err error
	if v.rx != nil {
		payload, err = v.rx.Open(nil, h[16:28], frame[HeaderSize:], h)
		if err != nil {
			return nil, ErrPacket
		}
	} else {
		payload = append([]byte(nil), frame[HeaderSize:]...)
	}
	if len(payload) != size {
		return nil, ErrPacket
	}
	v.rxMu.Lock()
	defer v.rxMu.Unlock()
	v.expire(now)
	var epoch [8]byte
	copy(epoch[:], h[28:36])
	sequence := binary.BigEndian.Uint64(h[36:44])
	window := v.replay[epoch]
	if window == nil {
		if len(v.replay) >= 32 {
			return nil, ErrLimits
		}
		window = &replayWindow{deadline: now.Add(ReplayTTL)}
		v.replay[epoch] = window
	}
	if !window.accept(sequence) {
		return nil, ErrReplay
	}
	window.deadline = now.Add(ReplayTTL)
	if count == 1 {
		if !ValidInnerPacket(payload) {
			return nil, ErrPacket
		}
		return payload, nil
	}
	var id [8]byte
	copy(id[:], h[44:52])
	key := fragmentKey{epoch: epoch, id: id}
	set := v.fragments[key]
	if set == nil {
		if len(v.fragments) >= 64 || v.fragmentBytes+size > 4*1024*1024 {
			return nil, ErrLimits
		}
		set = &fragmentSet{parts: make([][]byte, count), total: total, deadline: now.Add(ReassemblyTTL)}
		v.fragments[key] = set
	}
	if len(set.parts) != count || set.total != total || set.parts[index] != nil || set.bytes+size > total {
		v.fragmentBytes -= set.bytes
		delete(v.fragments, key)
		return nil, ErrPacket
	}
	if v.fragmentBytes+size > 4*1024*1024 {
		return nil, ErrLimits
	}
	set.parts[index] = payload
	set.received++
	set.bytes += size
	v.fragmentBytes += size
	if set.received != count {
		return nil, nil
	}
	v.fragmentBytes -= set.bytes
	delete(v.fragments, key)
	if set.bytes != total {
		return nil, ErrPacket
	}
	packet := make([]byte, 0, total)
	for _, part := range set.parts {
		packet = append(packet, part...)
	}
	if !ValidInnerPacket(packet) {
		return nil, ErrPacket
	}
	return packet, nil
}

func IsDroppedPacket(err error) bool {
	return errors.Is(err, ErrPacket) || errors.Is(err, ErrReplay) || errors.Is(err, ErrLimits)
}
