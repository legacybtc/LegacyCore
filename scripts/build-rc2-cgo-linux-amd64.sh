#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST="$ROOT/dist/rc2"

export CGO_ENABLED=1
export GOOS=linux
export GOARCH=amd64

mkdir -p "$DIST"

echo "== RC2 CGO yespower verification =="
go test ./internal/pow -v

echo "== Wallet frontend assets =="
(cd "$ROOT/cmd/legacywallet/frontend" && npm install && npm run build)

echo "== Full test suite =="
go test ./...
go vet ./...

echo "== Core =="
go build -trimpath -o "$DIST/legacycoind" ./cmd/legacycoind
go build -trimpath -o "$DIST/legacycoin-cli" ./cmd/legacycoin-cli

echo "== Pool =="
(cd "$ROOT/legacy-pool" && go test ./... && go vet ./... && go build -trimpath -o "$DIST/legacypool" ./cmd/legacypool)

echo "== Explorer / Launchpad =="
go build -trimpath -o "$DIST/legacylaunchpad" ./cmd/legacysite

echo "== Backend check =="
"$DIST/legacycoind" params | tee "$DIST/legacycoind-params.txt"
grep -q "yespower backend: cgo-c-reference" "$DIST/legacycoind-params.txt"
if grep -q "legacy-mainnet-rc2-pending" "$DIST/legacycoind-params.txt"; then
	echo "refusing to build package: public genesis identity is still pending" >&2
	exit 1
fi
if grep -q "genesis hash: $" "$DIST/legacycoind-params.txt"; then
	echo "refusing to build package: genesis hash is empty" >&2
	exit 1
fi

echo "== SHA256 =="
(cd "$DIST" && sha256sum legacycoind legacycoin-cli legacypool legacylaunchpad > SHA256SUMS.txt && cat SHA256SUMS.txt)

echo "RC2 CGO Linux amd64 artifacts written to $DIST"
