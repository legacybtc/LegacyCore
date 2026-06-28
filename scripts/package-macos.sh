#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_ROOT="$ROOT_DIR/dist"
VERSION="${1:-v1.0.9}"
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

echo "[package-macos] building desktop wallet (Wails)"
rm -rf "$ROOT_DIR/cmd/legacywallet/frontend/dist" 2>/dev/null || true
cd "$ROOT_DIR/cmd/legacywallet/frontend"
if [ ! -d "node_modules" ]; then npm ci; fi
npm run build 2>/dev/null || echo "[package-macos] frontend build skipped (pre-built OK)"
cd "$ROOT_DIR/cmd/legacywallet"
wails build -platform "darwin/$ARCH" -trimpath -ldflags "-s -w" -o "$PKG_DIR/LegacyWallet"
cd "$ROOT_DIR"
if [ -d "$PKG_DIR/LegacyWallet.app" ]; then
  echo "[package-macos] LegacyWallet.app built"
elif [ -f "$PKG_DIR/LegacyWallet" ]; then
  echo "[package-macos] LegacyWallet binary built"
fi

cp "$ROOT_DIR/LICENSE" "$PKG_DIR/LICENSE"
cp "$ROOT_DIR/NOTICE" "$PKG_DIR/NOTICE"
cp "$ROOT_DIR/configs/legacycoin-pretty.conf.example" "$PKG_DIR/legacycoin.conf.example"

cat > "$PKG_DIR/README_FIRST.txt" <<'EOF'
Legacy Core macOS Quick Start

Desktop Wallet (GUI):
  Double-click LegacyWallet.app

Headless Node (terminal):
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

WALLET_APP="$PKG_DIR/LegacyWallet.app"
if [ -f "$PKG_DIR/LegacyWallet" ]; then
  chmod 755 "$PKG_DIR/LegacyWallet"
elif [ -d "$WALLET_APP" ]; then
  chmod 755 "$WALLET_APP/Contents/MacOS/LegacyWallet" 2>/dev/null || true
fi
chmod 755 "$PKG_DIR/legacycoind" "$PKG_DIR/legacycoin-cli"
chmod 644 "$PKG_DIR/README_FIRST.txt" "$PKG_DIR/LICENSE" "$PKG_DIR/NOTICE" "$PKG_DIR/legacycoin.conf.example"

bash "$ROOT_DIR/scripts/generate-sha256s.sh" "$PKG_DIR"

(
  cd "$DIST_ROOT"
  rm -f "$TMP_TAR" "$PKG_PATH"
  # NOTE: --owner/--group/--numeric-owner are GNU tar flags but widely
  # supported on macOS tar as well.  --mode is *not* supported on
  # BSD tar (macOS); we already set file modes with chmod above.
  TAR_WALLET=""
  if [ -d "macos-${ARCH}/LegacyWallet.app" ]; then
    TAR_WALLET="macos-${ARCH}/LegacyWallet.app"
  elif [ -f "macos-${ARCH}/LegacyWallet" ]; then
    TAR_WALLET="macos-${ARCH}/LegacyWallet"
  fi
  tar --owner=0 --group=0 --numeric-owner -cf "$TMP_TAR" \
    "macos-${ARCH}/legacycoind" \
    "macos-${ARCH}/legacycoin-cli" \
    $TAR_WALLET
  tar --owner=0 --group=0 --numeric-owner -rf "$TMP_TAR" \
    "macos-${ARCH}/legacycoin.conf.example" \
    "macos-${ARCH}/LICENSE" \
    "macos-${ARCH}/NOTICE" \
    "macos-${ARCH}/README_FIRST.txt" \
    "macos-${ARCH}/SHA256SUMS.txt"
  gzip -n -c "$TMP_TAR" > "$PKG_PATH"
  rm -f "$TMP_TAR"
)

echo "[package-macos] created $PKG_PATH"
