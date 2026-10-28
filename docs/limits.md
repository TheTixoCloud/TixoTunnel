<!-- Scope and limitations by ir_spoof - https://t.me/ir_spoof -->
# Scope and intentional differences

This is independent source written from recovered configuration and behavior,
not the original source or a byte-identical rebuild. The original Go compiler
version and exact dependency set were unavailable. Its embedded
`v2.0.0-hotfix8` is an application version.

- Both endpoints must use this rebuild. `BHC4` stream sessions and `BHX2` raw
  records intentionally differ from original premium framing and encryption.
  Carrier names and TOML compatibility do not establish original-peer
  interoperability.
- Ten manager-visible transport labels and six IPX profiles are implemented.
  A server-side QUIC selector observed in the original is not a complete
  recovered bidirectional QUIC feature and is not exposed here.
- Backhaul's AnyTLS label means a browser-shaped TLS carrier. This project does
  not claim implementation of the separate AnyTLS application's authentication
  and session protocol.
- `auto_tuning=true` is rejected. There are no global sysctl, automatic NIC,
  kernel forwarding, NAT or firewall optimizations. Original TPACKET performance
  profiles and malformed-packet checksum bypasses are not reproduced.
- `ports.forwarder="iptables"` is rejected. Use owned userspace forwarding
  (`backhaul`), or configure separate administrator-owned routing policy. This
  is an intentional behavior difference, not identical firewall migration.
- Buffer/worker/performance settings describe this implementation's bounded
  behavior. They do not reproduce every premium scheduling, allocator,
  batching, logging or diagnostic policy. Consult the current core validation
  and measured report before relying on nondefault tuning.
- UDP `accept_udp.ring_size`, `frame_size` and `write_timeout_ms` are admitted
  compatibility metadata, not active ring, frame-buffer or write-deadline
  controls. Direct forwarding uses a 65,507-byte datagram buffer, the shared
  `max_connections` admission limit, `peer_idle_timeout_s` idle expiry and
  `dialer.dial_timeout` for flow writes. Their original performance semantics
  are not reproduced.
- Local source binding requires an already assigned active-interface address.
  It does not add an address or change a provider's NAT. IPX spoof overrides
  require correctly paired peers and permitted return routing; they do not
  hide the actual endpoints or guarantee Internet delivery.
- Raw IPX requires Linux raw/TUN privileges and suitable network support. Raw
  TCP lacks an outer kernel TCP handshake/retransmission. Encrypted IPIP/GRE
  payloads do not establish interoperability with ordinary kernel tunnels.
- Replay/reassembly state is bounded and memory-only. IPX replay state can be
  lost on receiver restart or expiry; it is not a persistent anti-replay
  database or a forward-secret handshake.
- Plaintext is rejected unless explicitly opted into through
  `security.allow_insecure_plaintext=true` and encryption disabled. It should
  not be used on untrusted networks. The manager's convenience builder enables
  encryption and generates private peer secrets.
- TLS verification defaults on. Private/self-signed deployments need a trusted
  local certificate/CA. Setting an arbitrary SNI or disabling verification does
  not provide identity assurance.
- The built release targets the architecture named in its filename. Source
  can be rebuilt for another supported architecture, but that does not imply
  an untested binary received the same acceptance run.

Measured results are listed in [test-report.md](test-report.md) and
[test-results.json](test-results.json). No static audit, compiler setting or
finite test matrix proves that every bug or malicious behavior is absent.
Capture destination checks cover the observed outer headers within each
admitted capture filter. They cannot inspect encrypted payloads or prove that
every possible network attempt was recorded; VM interface/default-route
isolation and static source review provide separate evidence.
