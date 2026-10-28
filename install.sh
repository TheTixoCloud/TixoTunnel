#!/usr/bin/env bash
# TixoTunnel installer for the open-source Go core (cmd/backhaul), published as
# the "tixotunnel" binary inside GitHub Release assets named
# TixoTunnel-linux-<arch>.tar.gz. This installer is independent of the legacy
# TixoTunnel.sh console/AETHER-X1 engine bundle and never touches its files.
#
# Usage:
#   ./install.sh [--version vX.Y.Z] [--install-dir /usr/local/bin]
#
# The script never executes a downloaded script, and never restarts any
# running tunnel service. Review the archive and checksum before running with
# elevated privileges if you have any doubt about the source.
set -euo pipefail

REPO="TheTixoCloud/TixoTunnel"
VERSION="latest"
INSTALL_DIR="/usr/local/bin"
BIN_NAME="tixotunnel"

usage() {
    cat <<EOF
Usage: $0 [--version vX.Y.Z] [--install-dir DIR]

  --version vX.Y.Z   Install a specific release tag instead of the latest.
  --install-dir DIR  Install location for the "tixotunnel" binary (default: $INSTALL_DIR).
  -h, --help          Show this help.
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            VERSION="$2"; shift 2 ;;
        --install-dir)
            INSTALL_DIR="$2"; shift 2 ;;
        -h|--help)
            usage; exit 0 ;;
        *)
            echo "error: unknown argument: $1" >&2
            usage
            exit 1 ;;
    esac
done

err() { echo "error: $*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || err "curl is required but not installed."
command -v sha256sum >/dev/null 2>&1 || err "sha256sum is required but not installed."
command -v tar >/dev/null 2>&1 || err "tar is required but not installed."

case "$(uname -s)" in
    Linux) ;;
    *) err "this installer only supports Linux." ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) GOARCH="amd64" ;;
    aarch64|arm64) GOARCH="arm64" ;;
    *) err "unsupported CPU architecture: $ARCH (supported: amd64, arm64)" ;;
esac

ASSET="TixoTunnel-linux-${GOARCH}.tar.gz"

if [[ "$VERSION" == "latest" ]]; then
    BASE_URL="https://github.com/${REPO}/releases/latest/download"
else
    BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
fi

ASSET_URL="${BASE_URL}/${ASSET}"
SUMS_URL="${BASE_URL}/SHA256SUMS"

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

echo "Detected architecture: ${GOARCH}"
echo "Downloading ${ASSET_URL}"
curl -fL --retry 3 --connect-timeout 15 -o "${WORKDIR}/${ASSET}" "$ASSET_URL" \
    || err "failed to download release asset. Check that version '${VERSION}' exists."

echo "Downloading ${SUMS_URL}"
curl -fL --retry 3 --connect-timeout 15 -o "${WORKDIR}/SHA256SUMS" "$SUMS_URL" \
    || err "failed to download SHA256SUMS."

echo "Verifying checksum..."
EXPECTED="$(grep " ${ASSET}\$" "${WORKDIR}/SHA256SUMS" | awk '{print $1}')"
[[ -n "$EXPECTED" ]] || err "no checksum entry found for ${ASSET} in SHA256SUMS."
ACTUAL="$(sha256sum "${WORKDIR}/${ASSET}" | awk '{print $1}')"
[[ "$EXPECTED" == "$ACTUAL" ]] || err "checksum mismatch for ${ASSET} (expected ${EXPECTED}, got ${ACTUAL}). Aborting."
echo "Checksum OK: ${ACTUAL}"

echo "Extracting..."
tar -xzf "${WORKDIR}/${ASSET}" -C "$WORKDIR"
[[ -f "${WORKDIR}/${BIN_NAME}" ]] || err "expected executable '${BIN_NAME}' not found in archive."
chmod 0755 "${WORKDIR}/${BIN_NAME}"

if [[ ! -w "$INSTALL_DIR" ]] && [[ "$(id -u)" -ne 0 ]]; then
    err "no write permission to ${INSTALL_DIR}. Re-run as root or pass --install-dir."
fi
mkdir -p "$INSTALL_DIR"

TARGET="${INSTALL_DIR}/${BIN_NAME}"
if [[ -f "$TARGET" ]]; then
    echo "Existing installation found at ${TARGET}; upgrading in place (atomic replace)."
fi

TMP_TARGET="${TARGET}.new.$$"
cp "${WORKDIR}/${BIN_NAME}" "$TMP_TARGET"
mv -f "$TMP_TARGET" "$TARGET"

echo "Installed ${TARGET}"
"$TARGET" -version || true

cat <<EOF

Installation complete.

- Existing configuration files were not touched by this installer.
- No tunnel service was started, enabled, or restarted.
- To run manually:        $TARGET -c /path/to/config.toml
- To validate a config:   $TARGET -c /path/to/config.toml -check
EOF
