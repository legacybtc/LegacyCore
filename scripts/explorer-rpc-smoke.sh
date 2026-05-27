#!/usr/bin/env bash
set -euo pipefail

CLI="${1:-./legacycoin-cli}"
DATADIR="${2:-}"
RPCPORT="${3:-19556}"

run_cli() {
  local args=()
  if [[ -n "$DATADIR" ]]; then
    args+=("-datadir=$DATADIR")
  fi
  if [[ -n "$RPCPORT" ]]; then
    args+=("-rpcport=$RPCPORT")
  fi
  args+=("$@")
  echo "[explorer-smoke] $CLI ${args[*]}"
  "$CLI" "${args[@]}"
}

best_hash_json="$(run_cli getbestblockhash)"
best_hash="$(printf '%s\n' "$best_hash_json" | sed -n 's/.*"result"[[:space:]]*:[[:space:]]*"\([^"]\+\)".*/\1/p' | head -n1)"
height_json="$(run_cli getblockcount)"
height="$(printf '%s\n' "$height_json" | sed -n 's/.*"result"[[:space:]]*:[[:space:]]*\([0-9]\+\).*/\1/p' | head -n1)"

run_cli getblockchaininfo
run_cli getmempoolinfo
run_cli getrawmempool
run_cli getblockhash "$height"
run_cli getblock "$best_hash"
run_cli getblockheader "$best_hash"
run_cli getnetworkhashps

echo "[explorer-smoke] completed"
