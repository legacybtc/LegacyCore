#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_ROOT="$ROOT_DIR/dist"
VERSION="${1:-v1.0.5}"
ARCH="${2:-amd64}"

case "$ARCH" in
  amd64|arm64) ;;
  *)
    echo "[package-macos] unsupported arch: $ARCH (expected amd64|arm64)" >&2
    exit 1
    ;;
esac

PKG_DIR="$DIST_ROOT/macos-${ARCH}"
PKG_NAME="LegacyCore-LBTC-mainnet-macos-${ARCH}-${VERSION}.tar.gz"
PKG_PATH="$DIST_ROOT/$PKG_NAME"
TMP_TAR="$DIST_ROOT/LegacyCore-LBTC-mainnet-macos-${ARCH}-${VERSION}.tar"

mkdir -p "$PKG_DIR"
cd "$ROOT_DIR"

export CGO_ENABLED=1
export GOOS=darwin
export GOARCH="$ARCH"

HOST_OS="$(go env GOHOSTOS)"
HOST_ARCH="$(go env GOARCH)"

if [[ "$HOST_OS" != "darwin" ]]; then
  echo "[package-macos] non-mac host detected. Set CC for darwin cross-compile (for example osxcross/zig)." >&2
  exit 2
fi

# Pick the right C compiler for native or cross-arch CGO on macOS.
# Apple's clang supports both amd64 and arm64 via -arch.
case "$ARCH" in
  amd64) export CC="clang -arch x86_64" ;;
  arm64) export CC="clang -arch arm64" ;;
esac
echo "[package-macos] using CC=$CC"

if [[ "$HOST_ARCH" != "$ARCH" ]]; then
  echo "[package-macos] cross-compiling darwin/$ARCH on darwin/$HOST_ARCH"
fi

echo "[package-macos] building darwin/$ARCH binaries"
go build -trimpath -ldflags "-s -w" -o "$PKG_DIR/legacycoind"   ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o "$PKG_DIR/legacycoin-cli" ./cmd/legacycoin-cli

cp "$ROOT_DIR/LICENSE" "$PKG_DIR/LICENSE"
cp "$ROOT_DIR/NOTICE" "$PKG_DIR/NOTICE"
cp "$ROOT_DIR/configs/legacycoin-pretty.conf.example" "$PKG_DIR/legacycoin.conf.example"

cat > "$PKG_DIR/README_FIRST.txt" <<'EOF'
Legacy Core macOS Headless Quick Start

1) chmod +x legacycoind legacycoin-cli
2) ./legacycoind params
3) ./legacycoind run -seed-peers

Second terminal:
  ./legacycoin-cli getblockcount
  ./legacycoin-cli getsyncstatus
  ./legacycoin-cli getpeerinfo
  ./legacycoin-cli getblocktemplate

Security:
- P2P port 19555 can be public.
- RPC port 19556 must stay private/firewalled.
EOF

chmod 755 "$PKG_DIR/legacycoind" "$PKG_DIR/legacycoin-cli"
chmod 644 "$PKG_DIR/README_FIRST.txt" "$PKG_DIR/LICENSE" "$PKG_DIR/NOTICE" "$PKG_DIR/legacycoin.conf.example"

bash "$ROOT_DIR/scripts/generate-sha256s.sh" "$PKG_DIR"

(
  cd "$DIST_ROOT"
  rm -f "$TMP_TAR" "$PKG_PATH"
  tar --owner=0 --group=0 --numeric-owner --mode='0755' -cf "$TMP_TAR" \
    "macos-${ARCH}/legacycoind" \
    "macos-${ARCH}/legacycoin-cli"
  tar --owner=0 --group=0 --numeric-owner --mode='0644' -rf "$TMP_TAR" \
    "macos-${ARCH}/legacycoin.conf.example" \
    "macos-${ARCH}/LICENSE" \
    "macos-${ARCH}/NOTICE" \
    "macos-${ARCH}/README_FIRST.txt" \
    "macos-${ARCH}/SHA256SUMS.txt"
  gzip -n -c "$TMP_TAR" > "$PKG_PATH"
  rm -f "$TMP_TAR"
)

echo "[package-macos] created $PKG_PATH"
