#!/usr/bin/env bash
set -euo pipefail

CLI="${1:-./legacycoin-cli}"
DATADIR="${2:-}"
RPCPORT="${3:-19556}"

run_cli() {
  local args=()
  if [[ -n "$DATADIR" ]]; then
    args+=("-datadir" "$DATADIR")
  fi
  if [[ -n "$RPCPORT" ]]; then
    args+=("-rpcport" "$RPCPORT")
  fi
  args+=("$@")
  echo "[exchange-smoke] $CLI ${args[*]}"
  "$CLI" "${args[@]}"
}

run_cli getblockchaininfo
run_cli getnetworkinfo
run_cli validateaddress Laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
run_cli getrawmempool
run_cli getwalletinfo
run_cli listunspent 0
run_cli backupwallet exchange-smoke-backup.json

echo "[exchange-smoke] completed"
