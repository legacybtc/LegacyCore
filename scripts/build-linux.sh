#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR" && pwd)"
cd "$ROOT_DIR"

echo ""
echo "======================================================"
echo "  Legacy Core Server - Linux Build Script"
echo "  Version 1.0.6"
echo "======================================================"
echo ""

# ---- STEP 1: Check prerequisites ----
echo "[1/4] Checking prerequisites..."

MISSING=""

if ! command -v gcc >/dev/null 2>&1; then
    MISSING="$MISSING gcc"
fi
if ! command -v go >/dev/null 2>&1; then
    MISSING="$MISSING go"
fi
if ! command -v git >/dev/null 2>&1; then
    MISSING="$MISSING git"
fi

if [[ -n "$MISSING" ]]; then
    echo ""
    echo "  MISSING:$MISSING"
    echo ""
    echo "  Install with:"
    echo "    apt update && apt install -y gcc golang-go git"
    echo ""
    echo "  Or install Go manually from https://go.dev/dl/"
    echo ""
    exit 1
fi

echo "  gcc:     $(gcc --version | head -1)"
echo "  go:      $(go version)"
echo "  git:     $(git --version)"

# ---- STEP 2: Run tests ----
echo "[2/4] Running tests..."

export CGO_ENABLED=1

if ! go test -short ./...; then
    echo "Tests failed"
    exit 1
fi

# ---- STEP 3: Build binaries ----
echo "[3/4] Building binaries..."

go build -trimpath -ldflags "-s -w" -o legacycoind ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o legacycoin-cli ./cmd/legacycoin-cli

echo "  legacycoind       - built"
echo "  legacycoin-cli     - built"

# ---- STEP 4: Verify identity ----
echo "[4/4] Verifying identity..."

./legacycoind params > params.txt

check_param() {
    if ! grep -q "$2" params.txt; then
        echo "  IDENTITY FAILED: $1"
        exit 1
    fi
}

check_param "genesis"    "genesis hash: 5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
check_param "yespower"   "yespower backend: cgo-c-reference"
check_param "p2p port"   "p2p port: 19555"
check_param "rpc port"   "rpc port: 19556"
check_param "msg start"  "message start: a4 ac c6 4d"

rm -f params.txt

echo "  Identity: verified"
echo ""
echo "======================================================"
echo "  BUILD COMPLETE"
echo "======================================================"
echo ""
echo "  Binaries:"
echo "    $(pwd)/legacycoind"
echo "    $(pwd)/legacycoin-cli"
echo ""
echo "  To start the node:"
echo "    ./legacycoind"
echo ""
echo "  To install as systemd service:"
echo "    sudo bash scripts/server/update-node.sh"
echo ""
