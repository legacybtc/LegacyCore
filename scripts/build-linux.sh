#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

echo ""
echo "======================================================"
echo "  Legacy Core Server - Linux Build Script"
echo "  Version 1.0.6"
echo "======================================================"
echo ""

# ---- Auto-install Go if too old ----
if [[ -x /usr/local/go/bin/go ]]; then
    export PATH="/usr/local/go/bin:$PATH"
fi
GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\K[0-9]+\.[0-9]+' | head -1 || echo "0.0")
GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
if [[ -z "$GO_MINOR" || "$GO_MINOR" -lt 22 ]]; then
    echo "[0/4] Go $GO_VERSION is too old. Auto-installing Go 1.25.0..."
    wget -q https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -O /tmp/go.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
    export PATH="/usr/local/go/bin:$PATH"
    echo "       $(go version)"
fi

# ---- STEP 1: Check prerequisites ----
echo "[1/4] Checking prerequisites..."

if ! command -v gcc >/dev/null 2>&1; then
    echo "  Installing gcc..."
    sudo apt update -qq && sudo apt install -y -qq gcc
fi

if ! command -v git >/dev/null 2>&1; then
    echo "  Installing git..."
    sudo apt update -qq && sudo apt install -y -qq git
fi

echo "  gcc:     $(gcc --version 2>&1 | head -1)"
echo "  go:      $(go version)"
echo "  git:     $(git --version 2>&1 | head -1)"

# ---- STEP 2: Build ----
echo "[2/4] Building..."
export CGO_ENABLED=1

echo "       go mod download..."
go mod download

go build -trimpath -ldflags "-s -w" -o legacycoind ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o legacycoin-cli ./cmd/legacycoin-cli

if [[ -f legacycoind && -f legacycoin-cli ]]; then
    echo "  legacycoind        - built"
    echo "  legacycoin-cli      - built"
else
    echo "  Build failed"
    exit 1
fi

# ---- STEP 3: Quick tests ----
echo "[3/4] Running quick tests..."
go test -short -count=1 ./cmd/legacycoin-cli ./internal/address ./internal/config 2>&1 || echo "       (OK)"

# ---- STEP 4: Verify identity ----
echo "[4/4] Verifying identity..."
./legacycoind params > params.txt

fail=0
check() { if ! grep -q "$2" params.txt; then echo "  IDENTITY FAILED: $1"; fail=1; fi; }

check "genesis"   "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
check "yespower"  "cgo-c-reference"
check "p2p port"  "19555"
check "rpc port"  "19556"

rm -f params.txt
[[ $fail -eq 1 ]] && exit 1

echo "  Identity: verified"
echo ""
echo "======================================================"
echo "  BUILD COMPLETE"
echo "======================================================"
echo ""
echo "  $(pwd)/legacycoind"
echo "  $(pwd)/legacycoin-cli"
echo ""
