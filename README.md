# TixoTunnel

TixoTunnel is a Linux TCP/UDP reverse-tunnel core, independently written in Go
(module `backhaul.local/ir_spoof`, originally published by `ir_spoof` —
https://t.me/ir_spoof), plus the legacy `TixoTunnel.sh` console/engine bundle
distributed under `core/`. This README documents the **Go source** in `cmd/`
and `internal/`, which now ships alongside the existing console bundle in this
repository.

> فارسی: [README.fa.md](README.fa.md)

## What this is, precisely

This is an independent reconstruction of Backhaul-style tunneling behavior. It
is not the original Backhaul source and makes no claim of wire compatibility
with the original premium core. Both tunnel endpoints must run this same
implementation. See [`docs/core.md`](docs/core.md) and
[`docs/limits.md`](docs/limits.md) for the full list of intentional
differences and scope limits — read them before relying on any specific
feature.

## Features verified from source

- **Server/client TCP reverse tunnel** with port mappings (`8080=9000`,
  explicit `bind:target` pairs, and bounded port ranges up to 256 listeners) —
  `internal/core`.
- **Transports**: `tcp`, `tcpmux`, `xtcpmux`, `ws`, `wss`, `wsmux`, `wssmux`,
  `xwsmux`, `anytls`, `tun` — `internal/transport`, `internal/wire`.
- **UDP forwarding** over the authenticated stream, with per-peer state, a
  65,507-byte datagram buffer, and idle-timeout expiry
  (`accept_udp`, `peer_idle_timeout_s`) — `internal/core/udp.go`.
- **Mutual authentication and encryption**: an HMAC-SHA256 + X25519 handshake
  (`BHC4`) establishes directional keys for AES-256-GCM, AES-128-GCM, or
  ChaCha20-Poly1305 framed records — `internal/wire`.
- **TUN/IPX raw tunneling** between two servers you control, over six raw
  IPv4 profiles (ICMP, IPIP, UDP, TCP, GRE, BIP) with its own encrypted
  envelope (`BHX2`), replay window, and reassembly — `internal/ipx`,
  `internal/tun`. Requires `CAP_NET_ADMIN`/root, `/dev/net/tun`, and raw
  sockets.
- **Source-IP selection**: binding an outbound dial to a specific IP already
  assigned to the server (`dialer.local_addr`), separate from the IPX raw
  source-address override (`ipx.spoof_src_ip` / `ipx.spoof_dst_ip`) used only
  between two cooperating, pre-configured IPX peers — see
  [`docs/source-ip-fa.txt`](docs/source-ip-fa.txt) and the security note
  below.
- **TOML or JSON configuration**, capped at 1 MiB, unknown fields rejected,
  `-check` validates a config with no network or interface changes —
  `internal/config`.
- **PROXY protocol** support on TCP forwards to backends configured to
  understand it.

TCP and UDP forwarding, TLS-carried transports, and the TUN/IPX raw path are
all implemented and covered by the test suite in `internal/`.

## Supported platforms

- **Runtime**: Linux only (amd64, arm64). The binary refuses to start the
  tunnel engine on any other OS; `-check` and `-version` work everywhere Go
  can build it.
- **Build**: any OS with Go 1.26+ installed (per `go.mod`); this repository
  is built and tested on Go 1.27.

## Installation

Download a release and verify its checksum with the installer:

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/TheTixoCloud/TixoTunnel/main/install.sh
chmod +x install.sh
./install.sh            # installs to /usr/local/bin/tixotunnel
```

The installer detects amd64/arm64, downloads the matching
`TixoTunnel-linux-<arch>.tar.gz` release asset, verifies it against
`SHA256SUMS`, and performs an atomic in-place upgrade if a previous
installation exists. It never runs a downloaded script, never restarts a
running service, and never touches your config files. Review `install.sh`
before running it with elevated privileges.

Alternatively, download assets manually from the
[Releases page](https://github.com/TheTixoCloud/TixoTunnel/releases), verify
`SHA256SUMS` yourself, and extract the `tixotunnel` binary.

> This installer is unrelated to the legacy `TixoTunnel.sh` console, which
> continues to manage the separate `core/tixotunnel-core.engine` bundle and is
> unaffected by this change.

## Build from source

```bash
git clone https://github.com/TheTixoCloud/TixoTunnel.git
cd TixoTunnel
make build          # -> bin/tixotunnel
make test           # go test ./...
make release        # -> dist/TixoTunnel-linux-{amd64,arm64}.tar.gz + SHA256SUMS
```

Requires Go 1.26 or newer. Dependencies are vendored under `vendor/`, so
`make build` works offline from a clean checkout.

## Configuration

Minimal TOML server example (`server.toml`):

```toml
[listener]
bind_addr = "0.0.0.0:3080"

[ports]
mapping = ["8080=9000", "8081=9001"]

[security]
token = "REPLACE_WITH_A_UNIQUE_32+_BYTE_SECRET"
algorithm = "aes-256-gcm"
```

Matching client:

```toml
[dialer]
remote_addr = "203.0.113.10:3080"

[security]
token = "REPLACE_WITH_A_UNIQUE_32+_BYTE_SECRET"
algorithm = "aes-256-gcm"
```

Validate before deploying:

```bash
tixotunnel -c server.toml -check
```

### TCP vs UDP

Set `transport.accept_udp = true` on a `tcp`/`tcpmux`/`ws...` transport to
forward UDP datagrams alongside TCP through the same authenticated stream. No
separate listener configuration is required.

### TUN/IPX (raw tunnel between two servers you control)

See [`docs/setup-fa.txt`](docs/setup-fa.txt) and
[`docs/source-ip-fa.txt`](docs/source-ip-fa.txt) for the full walkthrough
(generating a PSK, choosing real interface IPs on both ends, and the optional
raw source-IP override). Summary:

```bash
# generate a 32-byte PSK and share it over a trusted channel, mode 0600
openssl rand -base64 32 > /etc/tixotunnel/ipx-demo.psk
chmod 600 /etc/tixotunnel/ipx-demo.psk
```

```toml
[tun]
encapsulation = "ipx"
name = "tun-a"
local_addr = "10.200.0.1/30"
remote_addr = "10.200.0.2/30"

[ipx]
# profile, mode, interface, listen/peer IP, secret_file, etc. — see docs/setup-fa.txt
```

## Security considerations

- **Source-IP override (`ipx.spoof_src_ip`/`spoof_dst_ip`) is not anonymity
  and is not a general-purpose spoofing tool.** It only changes the raw
  outer-IP source used between two IPX peers that are *both* already
  configured, over a route and PSK you control. It requires raw-socket
  privileges on a Linux box you administer, does not bypass a provider's
  anti-spoofing filters, does not change your public egress IP, and does not
  hide either endpoint from each other. Use it only on networks you own or
  are explicitly authorized to test. See
  [`docs/security.md`](docs/security.md) and
  [`docs/source-ip-fa.txt`](docs/source-ip-fa.txt).
- Plaintext is rejected unless you explicitly set
  `security.allow_insecure_plaintext = true` *and* disable encryption — do not
  do this on an untrusted network.
- TLS verification is on by default for `wss`/`anytls`; private deployments
  need a trusted local CA/certificate.
- Tokens and PSKs are local secrets, not telemetry. Generate unique values
  per deployment, keep secret files at mode `0600`, and never commit them to
  a repository, screenshot, or command-line history.
- This project performs no telemetry, license checks, or automatic update
  downloads. See [`docs/security.md`](docs/security.md) for the full review.

## Troubleshooting

- `configuration invalid: ...` from `-check` — the error names the offending
  field; fix it before starting the tunnel.
- Tunnel starts but no traffic flows — confirm the token/PSK match on both
  ends, confirm firewall rules allow the listener/dialer ports, and for IPX,
  confirm the raw protocol (ICMP/GRE/etc.) isn't being dropped by the network
  path.
- IPX/TUN requires root (or `CAP_NET_ADMIN` + `CAP_NET_RAW`) and
  `/dev/net/tun`.

## Updating

```bash
./install.sh            # re-run to upgrade an existing install atomically
```

or rebuild from a newer `git pull` with `make build`.

## Downloading releases

Prebuilt `linux/amd64` and `linux/arm64` binaries, plus `SHA256SUMS`, are
published on the [Releases page](https://github.com/TheTixoCloud/TixoTunnel/releases)
for every tagged version. Always verify the checksum before running a
downloaded binary.

## Legacy console (`TixoTunnel.sh` / AETHER-X1 engine)

This repository also distributes a separate, pre-existing bash management
console and prebuilt engine, unrelated to the Go source above:

- `TixoTunnel.sh` — installation and management console.
- `core/tixotunnel-core` — service launcher and core wrapper.
- `core/tixotunnel-core.engine` — AETHER-X1 engine and Advanced Spoof Tester
  executable.

Install it as `root` on a supported Debian/Ubuntu server:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/TheTixoCloud/TixoTunnel/main/TixoTunnel.sh)
```

After installation, launch the console with `tixotunnel`. Its unified
updater downloads the console and both core files from this repository's
`main` branch and creates a rollback archive before replacing installed
files. Release identity: console `NEXUS-1.0.0`, engine `AETHER-X1`, tag
`v1.0.0`. The Advanced Spoof Tester uses the dedicated
`core/tixotunnel-spoof-tester` binary; the tunnel engine remains
`core/tixotunnel-core.engine` — the two are intentionally separate.

As with the Go core above: use tunneling, forwarding, diagnostics,
benchmarking, and spoof-testing features only on systems and networks you
own or are explicitly authorized to test.

## License

The Go source in `cmd/` and `internal/` is MIT-licensed — see [LICENSE](LICENSE).
Vendored dependencies under `vendor/` retain their own licenses.
