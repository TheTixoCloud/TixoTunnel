<!-- Independent core documentation by ir_spoof - https://t.me/ir_spoof -->
# How the core works

The entry server listens for tunnel peers and exposes the selected forwarding
ports. The client dials that server and opens destination service connections.
A mapping such as `8080=9000` exposes entry port 8080 and targets the client's
local service on port 9000. A range expands to at most 256 owned listeners.
Explicit bind/target addresses can be imported through TOML, for example
`127.0.0.1:8080=127.0.0.1:9000`.

The configuration loader accepts TOML or JSON, caps input at 1 MiB and rejects
unknown fields. The manager imports TOML and serializes typed values without
shell expansion. `-check` parses/validates without starting a listener, contacting
a peer or changing interfaces. Runtime checks also verify prerequisites such as
assigned source IPs and available device names.

## Sessions, pools and records

The physical carrier layer supplies a byte stream. The new `BHC4` exchange
authenticates both endpoints with HMAC-SHA256 keyed by a shared local token and
fresh X25519 public contributions. The authenticated context binds the carrier,
cipher and encryption selection. Direction-specific keys protect length-prefixed
records with AES-256-GCM, AES-128-GCM or ChaCha20-Poly1305. Counters are separate
for each direction; each record is at most 32 KiB before encryption. An
authenticated empty record marks write-side completion so response data can
continue in the opposite direction.

Both endpoints must run this independent protocol. It does not reuse the
premium core's fixed token/framing or licensing handshake, and matching a
transport name does not establish original-core interoperability.

Names ending in `mux` use bounded SMUX sessions and multiplex forwarding streams
over the session pool. Plain `tcp`, `ws`, `wss` and `anytls` use a pool of spare
authenticated physical connections, consumed by individual forwards and refilled
by the client. Worker counts, buffers and bounds are the independent
implementation's policy, not a reproduction of the original allocator or
performance tuning.

TCP forwarding pumps both directions and supports a write-side close while
waiting for the peer's response. Optional PROXY protocol sends a source header
to a backend that is explicitly configured to understand it. The TCP transport's
UDP option creates per-peer forwarding state and sends length-prefixed datagrams
through the authenticated stream; idle state is bounded and expires. Each flow
reserves a slot from the engine's shared connection limit. The implementation
forwards datagrams directly, with a 65,507-byte payload buffer and
`peer_idle_timeout_s` for idle expiry. Flow writes use `dialer.dial_timeout` in
seconds. The admitted compatibility fields `ring_size`, `frame_size` and
`write_timeout_ms` do **not** select a UDP ring, change that payload buffer or set
the active write deadline. Do not interpret their presence in an imported
premium configuration as implementation of the original queue/timing policy.

The [carrier documentation](../internal/transport/README.md) distinguishes RFC
6455 framing from raw post-Upgrade mux streams and explains TLS verification,
header bounds and connection deadlines. Backhaul's `anytls` label is a
browser-shaped TLS carrier, not the distinct AnyTLS application protocol.

## TUN and IPX

TCP TUN transports IP packets from an owned Linux TUN device through
authenticated framed streams. The IPX implementation carries those packets in
one of six raw IPv4 profiles: ICMP, IPIP, UDP, TCP, GRE or the recovered BIP
selector. A raw TCP outer header does not create a kernel TCP connection or add
carrier retransmission. Packet loss and filtering remain relevant.

The new `BHX2` IPX envelope intentionally replaces the original sequence-derived
GCM nonce construction. Each fragment has an independent random 96-bit AEAD
nonce, authenticated direction/context, sender epoch and sequence. Encryption
uses a local base64 PSK with PBKDF2/HKDF-derived directional keys. Reassembly and
replay windows are bounded in memory. This differs from the original IPX wire
format and has no claim of interoperability with an original premium peer.

An assigned local bind selects a real interface address. IPX source overrides
instead change the outgoing raw IPv4 source, while the real peer remains the
destination. The other endpoint must be configured to expect that spoofed
source. These settings do not configure an Internet return route or bypass a
provider's filtering. The [IPX design](../internal/ipx/README.md) describes exact
packet headers, replay/reassembly limits and routing ownership.

TUN interfaces and route entries are scoped to owned resources. The core does
not enable global kernel forwarding, replace global firewall rules, configure
NAT, or modify sysctls. Those are explicit administrator network decisions.

## Manager and release boundaries

The Python manager verifies a local ELF and an expected SHA-256 before install,
then checks the independent build identity. It tracks core, manager, config and
unit hashes, refuses unrecognized replacements and uses atomic writes with
rollback for failed updates. Configs and secret files use mode 0600. Systemd
units run the local core directly, with no shell or external probes.

`change-source` validates a candidate, stops an active owned tunnel, replaces its
config atomically and restarts it. Startup failure restores the previous config
and state. A successful startup check is distinct from a successful end-to-end
tunnel; test the actual destination service after a route or source change.

Optional health output is bound to loopback and exposes readiness/counters,
not a list of endpoints, paths or secrets. The manager disables HTTP proxies and
redirects for these local checks. The program contains no vendor-IP lookup,
license request or analytics sender. The final [test report](test-report.md)
records measured behavior and limitations for the delivered build.
