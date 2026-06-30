#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_ROOT="$ROOT_DIR/dist"
VERSION="${1:-v1.0.13}"
ARCH="${2:-amd64}"
PKG_DIR="$DIST_ROOT/linux-${ARCH}"
PKG_NAME="LegacyCore-LBTC-mainnet-linux-${ARCH}-${VERSION}.tar.gz"
PKG_PATH="$DIST_ROOT/$PKG_NAME"
PKG_TAR_TMP="$DIST_ROOT/LegacyCore-LBTC-mainnet-linux-${ARCH}-${VERSION}.tar"

mkdir -p "$PKG_DIR"

# Build directly into the staging dir; pass ARCH for cross-compile and let
# build-linux.sh honor the CC / LINUX_CC env exported by release.yml.
bash "$ROOT_DIR/scripts/build-linux.sh" "$ARCH" "$PKG_DIR"

cp "$ROOT_DIR/LICENSE" "$PKG_DIR/LICENSE"
cp "$ROOT_DIR/NOTICE" "$PKG_DIR/NOTICE"
cp "$ROOT_DIR/configs/legacycoin-pretty.conf.example" "$PKG_DIR/legacycoin.conf.example"

cat > "$PKG_DIR/README_FIRST.txt" <<'EOF'
Legacy Core Linux Headless Quick Start

1) chmod +x legacycoind legacycoin-cli
2) ./legacycoind params
3) ./legacycoind run -seed-peers

Second terminal:
  ./legacycoin-cli getblockcount
  ./legacycoin-cli getsyncstatus
  ./legacycoin-cli getpeerinfo
  ./legacycoin-cli getblocktemplate
  ./legacycoin-cli getminerstatus

Security:
- P2P port 19555 can be public.
- RPC port 19556 must stay private/firewalled.
- Back up wallet data before mining or holding funds.
EOF

chmod 755 "$PKG_DIR/legacycoind" "$PKG_DIR/legacycoin-cli"
chmod 644 "$PKG_DIR/README_FIRST.txt" "$PKG_DIR/LICENSE" "$PKG_DIR/NOTICE" "$PKG_DIR/legacycoin.conf.example"

(
  cd "$PKG_DIR"
  sha256sum legacycoind legacycoin-cli README_FIRST.txt LICENSE NOTICE legacycoin.conf.example > SHA256SUMS.txt
  chmod 644 SHA256SUMS.txt
)

(
  cd "$DIST_ROOT"
  rm -f "$PKG_TAR_TMP" "$PKG_PATH"
  tar --owner=0 --group=0 --numeric-owner --mode='0755' -cf "$PKG_TAR_TMP" \
    "linux-${ARCH}/legacycoind" \
    "linux-${ARCH}/legacycoin-cli"
  tar --owner=0 --group=0 --numeric-owner --mode='0644' -rf "$PKG_TAR_TMP" \
    "linux-${ARCH}/legacycoin.conf.example" \
    "linux-${ARCH}/LICENSE" \
    "linux-${ARCH}/NOTICE" \
    "linux-${ARCH}/README_FIRST.txt" \
    "linux-${ARCH}/SHA256SUMS.txt"
  gzip -n -c "$PKG_TAR_TMP" > "$PKG_PATH"
  rm -f "$PKG_TAR_TMP"
)

echo "[package-linux] created $PKG_PATH"
sha256sum "$PKG_PATH"

echo "[package-linux] sensitive scan"
SENSITIVE_RE="MAX/|C:\\\\Users|Co""dex|wallet\\.dat|config\\.local\\.json|/home/""maxgor|server""2|root""@"
if tar -tvf "$PKG_PATH" | grep -E "$SENSITIVE_RE" >/dev/null; then
  echo "[package-linux] error: sensitive pattern found in archive metadata/listing" >&2
  exit 1
fi
if command -v strings >/dev/null 2>&1; then
  BINARY_SENSITIVE_RE="C:/Users|C:\\\\Users|MAX/AppData|Co""dex|go-build"
  if strings -a "$PKG_DIR/legacycoind" "$PKG_DIR/legacycoin-cli" | grep -E "$BINARY_SENSITIVE_RE" >/dev/null; then
    echo "[package-linux] error: sensitive path-like pattern found in linux binaries" >&2
    exit 1
  fi
fi
