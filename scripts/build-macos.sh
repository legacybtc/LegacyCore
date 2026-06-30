#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARCH="${1:-amd64}"
VERSION="${2:-v1.0.20}"

echo ""
echo "======================================================"
echo "  Legacy Core Wallet - macOS Build Script"
echo "  Version $VERSION ($ARCH)"
echo "======================================================"
echo ""

# Step 1: Check prerequisites
echo "[1/6] Checking prerequisites..."
if ! command -v go &>/dev/null; then echo "Missing Go"; exit 1; fi
if ! command -v node &>/dev/null; then echo "Missing Node.js"; exit 1; fi
if ! command -v npm &>/dev/null; then echo "Missing npm"; exit 1; fi
echo "  Go:    $(go version)"
echo "  Node:  $(node --version)"
echo "  npm:   $(npm --version)"

# Step 2: Find C compiler (Apple clang)
echo "[2/6] Finding C compiler..."
case "$ARCH" in
  amd64) export CC="clang -arch x86_64" ;;
  arm64) export CC="clang -arch arm64" ;;
  *)
    echo "Unsupported arch: $ARCH (expected amd64|arm64)"
    exit 1
    ;;
esac
export CGO_ENABLED=1
echo "  CC:    $CC"

# Step 3: Install frontend dependencies
echo "[3/6] Installing frontend dependencies..."
cd "$ROOT_DIR/cmd/legacywallet/frontend"
if [ ! -d "node_modules" ]; then
  npm ci
fi
cd "$ROOT_DIR"

# Step 4: Build frontend
echo "[4/6] Building frontend..."
cd "$ROOT_DIR/cmd/legacywallet/frontend"
npm run build
cd "$ROOT_DIR"

# Step 5: Build headless binaries
echo "[5/6] Building binaries..."
export GOOS=darwin
export GOARCH="$ARCH"

rm -f legacycoind legacycoin-cli

go build -trimpath -o legacycoind ./cmd/legacycoind
echo "  legacycoind    - built"

go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
echo "  legacycoin-cli - built"

# Verify yespower backend
if ./legacycoind params 2>/dev/null | grep -q "yespower backend:"; then
  ./legacycoind params 2>/dev/null | grep "yespower backend:"
fi

# Step 6: Build wallet (Wails)
echo "[6/6] Building desktop wallet..."
if command -v wails &>/dev/null; then
  rm -rf "$ROOT_DIR/cmd/legacywallet/frontend/dist" 2>/dev/null || true
  cd "$ROOT_DIR/cmd/legacywallet"
  wails build -platform "darwin/$ARCH" -trimpath -ldflags "-s -w"
  cd "$ROOT_DIR"
  if [ -f "cmd/legacywallet/build/bin/LegacyWallet" ]; then
    echo "  LegacyWallet   - built (darwin/$ARCH)"
  elif [ -f "cmd/legacywallet/build/bin/LegacyWallet.app/Contents/MacOS/LegacyWallet" ]; then
    echo "  LegacyWallet.app - built (darwin/$ARCH)"
  else
    echo "  WARNING: Wails build may have completed but binary not found at expected path"
  fi
else
  echo "  Wails not found. Core + CLI built successfully."
  echo "  Install Wails: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  echo "  Then run: scripts/build-macos.sh"
fi

echo ""
echo "======================================================"
echo "  BUILD COMPLETE"
echo "======================================================"
ls -la legacycoind legacycoin-cli 2>/dev/null || true
if [ -d "cmd/legacywallet/build" ]; then
  find cmd/legacywallet/build -name "LegacyWallet*" -type f -o -name "*.app" -type d 2>/dev/null
fi
echo ""
