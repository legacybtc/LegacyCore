#!/usr/bin/env bash
set -euo pipefail

DAEMON="${1:-./legacycoind}"
CLI="${2:-./legacycoin-cli}"
ROOT="${3:-/tmp/legacy-chaos-ci-smoke}"

NODE_A="$ROOT/nodeA"
NODE_B="$ROOT/nodeB"
rm -rf "$ROOT"
mkdir -p "$NODE_A" "$NODE_B"

P2P_A=29755
RPC_A=29756
P2P_B=29757
RPC_B=29758

run_cli() {
  local datadir="$1"
  local port="$2"
  shift 2
  "$CLI" -datadir "$datadir" -rpcport "$port" "$@"
}

wait_rpc() {
  local datadir="$1"
  local port="$2"
  for _ in $(seq 1 60); do
    if run_cli "$datadir" "$port" getblockcount >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

extract_result_number() {
  sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1
}

extract_chain_id() {
  sed -n 's/.*"chain_id":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

"$DAEMON" run -datadir "$NODE_A" -p2pport "$P2P_A" -rpcport "$RPC_A" -seed-peers >/tmp/legacy-chaosA.log 2>&1 &
PID_A=$!
cleanup() {
  run_cli "$NODE_B" "$RPC_B" stop >/dev/null 2>&1 || true
  run_cli "$NODE_A" "$RPC_A" stop >/dev/null 2>&1 || true
  sleep 2
  kill "$PID_B" >/dev/null 2>&1 || true
  kill "$PID_A" >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_rpc "$NODE_A" "$RPC_A"

"$DAEMON" run -datadir "$NODE_B" -p2pport "$P2P_B" -rpcport "$RPC_B" -connect "127.0.0.1:$P2P_A" >/tmp/legacy-chaosB.log 2>&1 &
PID_B=$!
wait_rpc "$NODE_B" "$RPC_B"

connected=0
for _ in $(seq 1 30); do
  CONN_A="$(run_cli "$NODE_A" "$RPC_A" getconnectioncount | extract_result_number)"
  CONN_B="$(run_cli "$NODE_B" "$RPC_B" getconnectioncount | extract_result_number)"
  if [ "${CONN_A:-0}" -gt 0 ] && [ "${CONN_B:-0}" -gt 0 ]; then
    connected=1
    break
  fi
  sleep 1
done
[ "$connected" -eq 1 ] || { echo "[chaos-ci-smoke] connect failed" >&2; exit 1; }

CHAIN_A="$(run_cli "$NODE_A" "$RPC_A" getchainparams | extract_chain_id)"
CHAIN_B="$(run_cli "$NODE_B" "$RPC_B" getchainparams | extract_chain_id)"
[ -n "$CHAIN_A" ] && [ "$CHAIN_A" = "$CHAIN_B" ] || { echo "[chaos-ci-smoke] chain id mismatch" >&2; exit 1; }

run_cli "$NODE_B" "$RPC_B" stop >/dev/null 2>&1 || true
sleep 2
kill "$PID_B" >/dev/null 2>&1 || true
"$DAEMON" run -datadir "$NODE_B" -p2pport "$P2P_B" -rpcport "$RPC_B" -connect "127.0.0.1:$P2P_A" >/tmp/legacy-chaosB.log 2>&1 &
PID_B=$!
wait_rpc "$NODE_B" "$RPC_B"

reconnected=0
for _ in $(seq 1 30); do
  CONN_B="$(run_cli "$NODE_B" "$RPC_B" getconnectioncount | extract_result_number)"
  if [ "${CONN_B:-0}" -gt 0 ]; then
    reconnected=1
    break
  fi
  sleep 1
done
[ "$reconnected" -eq 1 ] || { echo "[chaos-ci-smoke] reconnect failed" >&2; exit 1; }

echo "[chaos-ci-smoke] PASS"
