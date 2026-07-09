#!/usr/bin/env bash
set -euo pipefail

BINARY="${1:-./legacycoind}"

if [[ ! -x "$BINARY" ]]; then
  echo "[verify-mainnet-identity] binary not found or not executable: $BINARY" >&2
  exit 1
fi

params="$("$BINARY" params)"
echo "$params"

expect() {
  local pattern="$1"
  local label="$2"
  if ! grep -Eq "$pattern" <<<"$params"; then
    echo "[verify-mainnet-identity] failed: $label" >&2
    exit 1
  fi
}

expect 'message start:[[:space:]]+a4 ac c6 4d' 'message start'
expect 'genesis hash:[[:space:]]+5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5' 'genesis hash'
expect 'genesis time:[[:space:]]+1779235200' 'genesis time'
expect 'genesis nonce:[[:space:]]+3' 'genesis nonce'
expect 'yespower personalization:[[:space:]]+LegacyCoinPoW' 'yespower personalization'
expect 'yespower backend:[[:space:]]+cgo-c-reference' 'yespower backend'
expect 'p2p port:[[:space:]]+19555' 'p2p port'
expect 'rpc port:[[:space:]]+19556' 'rpc port'

echo "[verify-mainnet-identity] passed"
