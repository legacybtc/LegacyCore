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
  echo "[pool-smoke] $CLI ${args[*]}"
  "$CLI" "${args[@]}"
}

run_cli getblockchaininfo
run_cli getnetworkinfo
run_cli getnetworkhashps
run_cli getblocktemplate
run_cli submitblock 00
run_cli validateaddress Laaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa

echo "[pool-smoke] completed (submitblock invalid-path call expected to return structured rejection)"
