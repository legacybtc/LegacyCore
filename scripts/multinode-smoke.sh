#!/usr/bin/env bash
set -euo pipefail

DAEMON="${1:-./legacycoind}"
CLI="${2:-./legacycoin-cli}"
ROOT="${3:-./.tmp-multinode-smoke}"

NODE_A="$ROOT/nodeA"
NODE_B="$ROOT/nodeB"
rm -rf "$ROOT"
mkdir -p "$NODE_A" "$NODE_B"

P2P_A=29655
RPC_A=29656
P2P_B=29657
RPC_B=29658

run_cli() {
  local datadir="$1"
  local port="$2"
  shift
  shift
  "$CLI" "-datadir" "$datadir" "-rpcport" "$port" "$@"
}

wait_rpc() {
  local datadir="$1"
  local port="$2"
  local i
  for i in $(seq 1 60); do
    if run_cli "$datadir" "$port" getblockcount >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

result_number() {
  local datadir="$1"
  local port="$2"
  shift 2
  run_cli "$datadir" "$port" "$@" | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1
}

result_string() {
  local datadir="$1"
  local port="$2"
  shift 2
  run_cli "$datadir" "$port" "$@" | sed -n 's/.*"result":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

wait_same_tip() {
  local want_height="$1"
  local want_hash="$2"
  local height_b=""
  local hash_b=""
  for _ in $(seq 1 90); do
    height_b="$(result_number "$NODE_B" "$RPC_B" getblockcount || true)"
    hash_b="$(result_string "$NODE_B" "$RPC_B" getbestblockhash || true)"
    if [ "$height_b" = "$want_height" ] && [ "$hash_b" = "$want_hash" ]; then
      return 0
    fi
    sleep 1
  done
  echo "[multinode-smoke] nodeB did not sync to nodeA tip (nodeA=$want_height/$want_hash nodeB=$height_b/$hash_b)" >&2
  run_cli "$NODE_B" "$RPC_B" getsyncstatus >&2 || true
  exit 1
}

"$DAEMON" run "-datadir" "$NODE_A" "-p2pport" "$P2P_A" "-rpcport" "$RPC_A" -seed-peers >/tmp/legacy-nodeA.log 2>&1 &
PID_A=$!
PID_B=""
cleanup() {
  run_cli "$NODE_B" "$RPC_B" stop >/dev/null 2>&1 || true
  run_cli "$NODE_A" "$RPC_A" stop >/dev/null 2>&1 || true
  sleep 2
  [ -n "${PID_B:-}" ] && kill "$PID_B" >/dev/null 2>&1 || true
  [ -n "${PID_A:-}" ] && kill "$PID_A" >/dev/null 2>&1 || true
}
trap cleanup EXIT

wait_rpc "$NODE_A" "$RPC_A"
echo "[multinode-smoke] nodeA ready (rpc=$RPC_A, p2p=$P2P_A)"

"$DAEMON" run "-datadir" "$NODE_B" "-p2pport" "$P2P_B" "-rpcport" "$RPC_B" "-connect" "127.0.0.1:$P2P_A" >/tmp/legacy-nodeB.log 2>&1 &
PID_B=$!
wait_rpc "$NODE_B" "$RPC_B"
echo "[multinode-smoke] nodeB ready (rpc=$RPC_B, p2p=$P2P_B)"

connected=0
for _ in $(seq 1 30); do
  CONN_A="$(run_cli "$NODE_A" "$RPC_A" getconnectioncount | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1)"
  CONN_B="$(run_cli "$NODE_B" "$RPC_B" getconnectioncount | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1)"
  if [ "${CONN_A:-0}" -gt 0 ] && [ "${CONN_B:-0}" -gt 0 ]; then
    connected=1
    break
  fi
  sleep 1
done
if [ "$connected" -ne 1 ]; then
  echo "[multinode-smoke] nodes did not establish peer connection within timeout" >&2
  exit 1
fi

HEIGHT_A="$(result_number "$NODE_A" "$RPC_A" getblockcount)"
HEIGHT_B="$(result_number "$NODE_B" "$RPC_B" getblockcount)"
if [ -z "$HEIGHT_A" ] || [ -z "$HEIGHT_B" ] || [ "$HEIGHT_A" -ne "$HEIGHT_B" ]; then
  echo "[multinode-smoke] height mismatch after initial sync (nodeA=$HEIGHT_A nodeB=$HEIGHT_B)" >&2
  exit 1
fi

HASH_A="$(result_string "$NODE_A" "$RPC_A" getbestblockhash)"
HASH_B="$(result_string "$NODE_B" "$RPC_B" getbestblockhash)"
if [ -z "$HASH_A" ] || [ -z "$HASH_B" ] || [ "$HASH_A" != "$HASH_B" ]; then
  echo "[multinode-smoke] best hash mismatch after initial sync (nodeA=$HASH_A nodeB=$HASH_B)" >&2
  exit 1
fi
echo "[multinode-smoke] initial sync aligned: height=$HEIGHT_A hash=$HASH_A"

run_cli "$NODE_A" "$RPC_A" generate 1 4 true >/dev/null
HEIGHT_A="$(result_number "$NODE_A" "$RPC_A" getblockcount)"
HASH_A="$(result_string "$NODE_A" "$RPC_A" getbestblockhash)"
if [ -z "$HEIGHT_A" ] || [ "$HEIGHT_A" -lt 1 ]; then
  echo "[multinode-smoke] nodeA did not mine a propagation test block" >&2
  exit 1
fi
wait_same_tip "$HEIGHT_A" "$HASH_A"
echo "[multinode-smoke] mined block propagated to nodeB: height=$HEIGHT_A hash=$HASH_A"

run_cli "$NODE_B" "$RPC_B" stop >/dev/null
sleep 2
kill "$PID_B" >/dev/null 2>&1 || true
PID_B=""
"$DAEMON" run "-datadir" "$NODE_B" "-p2pport" "$P2P_B" "-rpcport" "$RPC_B" "-connect" "127.0.0.1:$P2P_A" >/tmp/legacy-nodeB.log 2>&1 &
PID_B=$!
wait_rpc "$NODE_B" "$RPC_B"

reconnected=0
for _ in $(seq 1 30); do
  CONN_B="$(run_cli "$NODE_B" "$RPC_B" getconnectioncount | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1)"
  if [ "${CONN_B:-0}" -gt 0 ]; then
    reconnected=1
    break
  fi
  sleep 1
done
if [ "$reconnected" -ne 1 ]; then
  echo "[multinode-smoke] nodeB did not reconnect to nodeA after restart" >&2
  exit 1
fi

wait_same_tip "$HEIGHT_A" "$HASH_A"
HASH_B2="$(result_string "$NODE_B" "$RPC_B" getbestblockhash)"
if [ -z "$HASH_B2" ] || [ "$HASH_B2" != "$HASH_A" ]; then
  echo "[multinode-smoke] best hash mismatch after reconnect (nodeA=$HASH_A nodeB=$HASH_B2)" >&2
  exit 1
fi
echo "[multinode-smoke] reconnect alignment verified: $HASH_B2"

echo "[multinode-smoke] completed"
