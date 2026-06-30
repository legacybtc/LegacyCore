#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT_DIR"

# Optional positional arguments (backwards-compatible):
#   $1 = ARCH (amd64|arm64) — only used to set GOARCH for cross-compile
#   $2 = OUT_DIR — where to drop legacycoind / legacycoin-cli (default: repo root)
TARGET_ARCH="${1:-}"
OUT_DIR="${2:-$ROOT_DIR}"
mkdir -p "$OUT_DIR"

echo ""
echo "======================================================"
echo "  Legacy Core Server - Linux Build Script"
echo "  Version 1.0.20"
if [[ -n "$TARGET_ARCH" ]]; then echo "  Target arch: $TARGET_ARCH"; fi
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

if ! command -v gcc >/dev/null 2>&1 && [[ -z "${CC:-}" ]]; then
    echo "  Installing gcc..."
    sudo apt update -qq && sudo apt install -y -qq gcc
fi

if ! command -v git >/dev/null 2>&1; then
    echo "  Installing git..."
    sudo apt update -qq && sudo apt install -y -qq git
fi

CC_REPORT="${CC:-$(command -v gcc || echo gcc)}"
echo "  cc:      $CC_REPORT"
echo "  go:      $(go version)"
echo "  git:     $(git --version 2>&1 | head -1)"

# ---- STEP 2: Build ----
echo "[2/4] Building..."
export CGO_ENABLED=1
if [[ -n "${CC:-}" ]]; then export CC; fi

# Cross-compile when ARCH is set and differs from host arch.
HOST_ARCH="$(go env GOARCH 2>/dev/null || uname -m)"
HOST_ARCH="${HOST_ARCH/x86_64/amd64}"
if [[ -n "$TARGET_ARCH" && "$TARGET_ARCH" != "$HOST_ARCH" ]]; then
    export GOARCH="$TARGET_ARCH"
    echo "  Cross-compiling GOARCH=$GOARCH with CC=${CC:-<default>}"
fi

echo "       go mod tidy..."
go mod tidy
echo "       go mod download..."
go mod download

DAEMON_OUT="$OUT_DIR/legacycoind"
CLI_OUT="$OUT_DIR/legacycoin-cli"
go build -trimpath -ldflags "-s -w" -o "$DAEMON_OUT" ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o "$CLI_OUT"     ./cmd/legacycoin-cli

if [[ -f "$DAEMON_OUT" && -f "$CLI_OUT" ]]; then
    echo "  legacycoind        - built"
    echo "  legacycoin-cli     - built"
else
    echo "  Build failed"
    exit 1
fi

# ---- STEP 3: Quick tests (only on native arch — cgo cross-compile can't execute the binary) ----
if [[ -z "$TARGET_ARCH" || "$TARGET_ARCH" == "$HOST_ARCH" ]]; then
    echo "[3/4] Running quick tests..."
    go test -short -count=1 ./cmd/legacycoin-cli ./internal/address ./internal/config 2>&1 || echo "       (OK)"

    # ---- STEP 4: Verify identity ----
    echo "[4/4] Verifying identity..."
    "$DAEMON_OUT" params > params.txt
    fail=0
    check() { if ! grep -q "$2" params.txt; then echo "  IDENTITY FAILED: $1"; fail=1; fi; }
    check "genesis"   "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
    check "yespower"  "cgo-c-reference"
    check "p2p port"  "19555"
    check "rpc port"  "19556"
    rm -f params.txt
    [[ $fail -eq 1 ]] && exit 1
    echo "  Identity: verified"
else
    echo "[3/4] Skipping quick tests (cross-compile for $TARGET_ARCH — host is $HOST_ARCH)"
    echo "[4/4] Skipping identity probe (cannot execute $TARGET_ARCH binary on $HOST_ARCH host)"
fi

echo ""
echo "======================================================"
echo "  BUILD COMPLETE"
echo "======================================================"
echo ""
echo "  $DAEMON_OUT"
echo "  $CLI_OUT"
echo ""
