<!-- Independent implementation: ir_spoof — https://t.me/ir_spoof -->
# IPX implementation

This package creates an exclusive Linux TUN and carries its IP packets through
one of six distinct IPv4 outer profiles: ICMP (1), IPIP (4), raw TCP (6), UDP
(17), GRE (47) or BIP (58). BIP uses the number recovered from the supplied
binary. It can also use an explicitly configured nonconflicting protocol number.
TCP means a real 20-byte TCP header with PSH/ACK and a checksum, without a kernel
TCP handshake or retransmission. UDP and ICMP have their own eight-byte headers;
GRE has the four-byte version-zero `0000 0800` header. These are packet carriers,
not claims of interoperability with kernel IPIP/GRE tunnel devices when the
payload contains encrypted application records.

The package accepts optional local and peer ICMP type/code pointers and freezes
their values at construction. The current application configuration uses numeric
`ipx.icmp_type`/`icmp_code`: zero retains the role default (client type 8, server
type 0, code 0), and a nonzero override applies symmetrically to transmitted and
expected peer headers. Configure the same nonzero override on both peers. This
numeric application format cannot request explicit type 0 on the client; the
lower-level package API can express it with a pointer.

The runtime transmits through a scoped `AF_INET` raw socket with `IP_HDRINCL` and
receives through an owned `AF_PACKET` socket with a socket-local BPF filter. It
does not install firewall rules, enable promiscuous mode, change global sysctls,
contact a license service or query a public-IP website. The real local IPv4 must
already be assigned to the selected active interface.

`SpoofSrcIP` changes the outgoing IPv4 source only. The outgoing destination is
always the real `PeerIP`. `SpoofDstIP` declares the source address expected from
the other peer; it does not redirect outgoing traffic. Both peers must agree on
these settings. `LocalIP` selects an assigned source, while an explicit source
override requests raw source spoofing. Real networks may filter spoofed traffic.

## New authenticated framing

The supplied binary's legacy sequence-derived GCM nonce is deliberately not
reproduced. The safe replacement uses version-two `BHX2` records. It is **not wire
compatible with original IPX peers**: run this implementation at both endpoints.

AES-256-GCM is the default; AES-128-GCM and ChaCha20-Poly1305 are also supported.
The base key uses PBKDF2-HMAC-SHA256 with the recovered public salt
`bifrost-udp-crypto-v1`, followed by separate HKDF-SHA256 keys for each direction.
HKDF binds the new version, algorithm, profile, protocol number, real endpoint
addresses/ports, logical client/server roles and TUN endpoint addresses. Source
spoof overrides do not change the key context. Keys are base64 encodings of
16–128 secret bytes; 32 random bytes are recommended. The default iteration
count is 100,000; accepted explicit counts are 10,000–1,000,000.

Every fragment has an independent full random 96-bit AEAD nonce, random sender
startup epoch and monotonically increasing authenticated sequence. The 64-byte
header is authenticated as additional data:

| Offset | Bytes | Field |
| --- | --- | --- |
| 0 | 4 | `BHX2` magic |
| 4 | 1 | Version 2 |
| 5 | 1 | Flags; bit 0 marks explicit insecure plaintext |
| 6 | 1 | IPv4 outer protocol |
| 7 | 1 | Direction: client 0, server 1 |
| 8 | 8 | Key-context session identifier |
| 16 | 12 | Fresh cryptographic random AEAD nonce |
| 28 | 8 | Random sender startup epoch |
| 36 | 8 | Sequence, beginning at 1 |
| 44 | 8 | Random inner-packet identifier |
| 52 | 2 | Fragment index |
| 54 | 2 | Fragment count |
| 56 | 2 | Complete inner-packet byte length |
| 58 | 2 | This fragment's plaintext byte length |
| 60 | 4 | Reserved zero bytes |

Multibyte integers use network byte order. The encrypted fragment and 16-byte
AEAD tag follow the header. Plaintext is available only through the explicit
`AllowInsecurePlaintext` opt-in; it provides no peer authentication and is not
recommended on untrusted networks.

Inner packets are fragmented before encryption so no emitted outer packet
exceeds `OuterMTU` (default 1500). The TUN MTU defaults to 1320. Reassembly permits
at most 256 fragments per packet, 64 incomplete packets, 65,535 bytes per complete
packet and 4 MiB of retained fragment data. Incomplete packets expire after five
seconds without extending the deadline on duplicates.

Authentication precedes replay state allocation. A 4096-bit sliding sequence
window rejects repeats and excessively old records for each of at most 32
authenticated sender epochs. Epoch state expires after ten idle minutes. Replay
state is memory-only: captured authenticated traffic can become admissible again
after receiver restart or expiry. This is bounded replay protection, not a
persistent anti-replay database or a handshake providing forward secrecy.

TUN devices are nonpersistent and opened with an exclusive name. Routes use an
owned protocol marker, are added without replacing existing routes, and disappear
with the device. Cancellation closes sockets and wakes blocked queue readers.
Multiple TUN queues are read independently; no original queue-zero-only behavior
is copied. NAT and firewall policy remain the administrator's responsibility.
