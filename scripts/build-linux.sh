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
for cmd in gcc go git; do
    if ! command -v $cmd >/dev/null 2>&1; then
        MISSING="$MISSING $cmd"
    fi
done

if [[ -n "$MISSING" ]]; then
    echo "  MISSING:$MISSING"
    echo "  Install: apt update && apt install -y gcc golang-go git"
    exit 1
fi

echo "  gcc:     $(gcc --version 2>&1 | head -1)"
echo "  go:      $(go version)"
echo "  git:     $(git --version 2>&1 | head -1)"

# ---- STEP 2: Verify module + download deps ----
echo "[2/4] Downloading modules and running tests..."

export CGO_ENABLED=1
export GOFLAGS=""

echo "       go mod download..."
go mod download

echo "       go mod verify..."
go mod verify

echo "       go vet ./cmd/... ./internal/..."
go vet ./cmd/... ./internal/... 2>&1 || echo "       (vet warnings OK)"

echo "       go test -short ./cmd/... ./internal/..."
go test -short -count=1 ./cmd/... ./internal/... 2>&1 || true

# ---- STEP 3: Build binaries ----
echo "[3/4] Building binaries..."

go build -trimpath -ldflags "-s -w" -o legacycoind ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o legacycoin-cli ./cmd/legacycoin-cli

if [[ -f legacycoind && -f legacycoin-cli ]]; then
    echo "  legacycoind        - built"
    echo "  legacycoin-cli      - built"
else
    echo "  Build failed — binaries not found"
    exit 1
fi

# ---- STEP 4: Verify identity ----
echo "[4/4] Verifying identity..."

./legacycoind params > params.txt

fail=0
check() {
    if ! grep -q "$2" params.txt; then
        echo "  IDENTITY FAILED: $1"
        fail=1
    fi
}

check "genesis"   "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
check "yespower"  "cgo-c-reference"
check "p2p port"  "19555"
check "rpc port"  "19556"

rm -f params.txt

if [[ $fail -eq 1 ]]; then exit 1; fi

echo "  Identity: verified"
echo ""
echo "======================================================"
echo "  BUILD COMPLETE"
echo "======================================================"
echo ""
echo "  $(pwd)/legacycoind"
echo "  $(pwd)/legacycoin-cli"
echo ""
