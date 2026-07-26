# TixoTunnel NEXUS 1.0.0

TixoTunnel is a Bash-based Linux tunnel management console powered by the **AETHER-X1** engine.

## Components

- `TixoTunnel.sh` — installation and management console
- `core/tixotunnel-core` — service launcher and core wrapper
- `core/tixotunnel-core.engine` — AETHER-X1 engine and Advanced Spoof Tester executable

## Installation

Run as `root` on a supported Debian/Ubuntu server:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/TheTixoCloud/TixoTunnel/main/TixoTunnel.sh)
```

After installation, launch the console with:

```bash
tixotunnel
```

## Update behavior

The unified updater downloads the console and both core files directly from the repository's `main` branch. A rollback archive is created before installed files are replaced.

## Release identity

- Console: `NEXUS-1.0.0`
- Engine: `AETHER-X1`
- Version tag: `v1.0.0`

## Important

Use tunneling, forwarding, diagnostics, benchmarking, and spoof-testing features only on systems and networks you own or are explicitly authorized to test.


## Advanced Spoof Tester

The tester uses the dedicated `core/tixotunnel-spoof-tester` binary. The tunnel engine remains `core/tixotunnel-core.engine`; the two binaries are intentionally separate.
