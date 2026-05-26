#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist/linux-amd64"
TMP_ROOT="$ROOT_DIR/.tmp-linux-build"

mkdir -p "$DIST_DIR"
mkdir -p "$TMP_ROOT/gocache" "$TMP_ROOT/gotmp"
cd "$ROOT_DIR"

export CGO_ENABLED=1
export GOCACHE="$TMP_ROOT/gocache"
export GOTMPDIR="$TMP_ROOT/gotmp"
export TMPDIR="$TMP_ROOT/gotmp"
export TMP="$TMP_ROOT/gotmp"
export TEMP="$TMP_ROOT/gotmp"

# Run tests/vet with host toolchain even when cross-compilers are configured for linux builds.
HOST_CC="${CC:-}"
HOST_CXX="${CXX:-}"
unset CC
unset CXX

echo "[build-linux] running tests"
go test ./internal/p2p ./internal/rpc ./internal/wallet ./internal/mempool
go test ./cmd/... ./internal/...

echo "[build-linux] running vet"
go vet ./internal/p2p ./internal/rpc ./internal/wallet ./internal/mempool
go vet ./cmd/... ./internal/...

export GOOS=linux
export GOARCH=amd64
if [[ -n "${LINUX_CC:-}" ]]; then
  export CC="$LINUX_CC"
elif [[ -n "$HOST_CC" ]]; then
  export CC="$HOST_CC"
fi
if [[ -n "${LINUX_CXX:-}" ]]; then
  export CXX="$LINUX_CXX"
elif [[ -n "$HOST_CXX" ]]; then
  export CXX="$HOST_CXX"
fi

echo "[build-linux] building binaries"
go build -trimpath -ldflags "-s -w" -o "$DIST_DIR/legacycoind" ./cmd/legacycoind
go build -trimpath -ldflags "-s -w" -o "$DIST_DIR/legacycoin-cli" ./cmd/legacycoin-cli

if command -v strip >/dev/null 2>&1; then
  echo "[build-linux] stripping binaries"
  strip "$DIST_DIR/legacycoind" "$DIST_DIR/legacycoin-cli"
fi

echo "[build-linux] binary path leak check"
if command -v strings >/dev/null 2>&1; then
  BINARY_SENSITIVE_RE="C:/Users|C:\\\\Users|MAX/AppData|Co""dex|go-build"
  if strings -a "$DIST_DIR/legacycoind" "$DIST_DIR/legacycoin-cli" | grep -E "$BINARY_SENSITIVE_RE" >/dev/null; then
    echo "[build-linux] error: sensitive path-like pattern found in linux binaries" >&2
    exit 1
  fi
fi

echo "[build-linux] params verification"
if [[ "$(go env GOHOSTOS)" == "linux" ]]; then
  "$DIST_DIR/legacycoind" params
else
  echo "[build-linux] host is not linux; skipping direct params execution for linux binary."
  if command -v file >/dev/null 2>&1; then
    file "$DIST_DIR/legacycoind" "$DIST_DIR/legacycoin-cli"
  fi
fi
