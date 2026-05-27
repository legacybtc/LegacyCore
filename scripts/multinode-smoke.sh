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
  "$CLI" "-datadir=$datadir" "-rpcport=$port" "$@"
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

"$DAEMON" run "-datadir=$NODE_A" "-p2pport=$P2P_A" "-rpcport=$RPC_A" -seed-peers >/tmp/legacy-nodeA.log 2>&1 &
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
echo "[multinode-smoke] nodeA ready (rpc=$RPC_A, p2p=$P2P_A)"

"$DAEMON" run "-datadir=$NODE_B" "-p2pport=$P2P_B" "-rpcport=$RPC_B" "-connect=127.0.0.1:$P2P_A" >/tmp/legacy-nodeB.log 2>&1 &
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

HEIGHT_A="$(run_cli "$NODE_A" "$RPC_A" getblockcount | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1)"
HEIGHT_B="$(run_cli "$NODE_B" "$RPC_B" getblockcount | sed -n 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' | head -n1)"
if [ -z "$HEIGHT_A" ] || [ -z "$HEIGHT_B" ] || [ "$HEIGHT_A" -ne "$HEIGHT_B" ]; then
  echo "[multinode-smoke] height mismatch after initial sync (nodeA=$HEIGHT_A nodeB=$HEIGHT_B)" >&2
  exit 1
fi

HASH_A="$(run_cli "$NODE_A" "$RPC_A" getbestblockhash | sed -n 's/.*"result":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
HASH_B="$(run_cli "$NODE_B" "$RPC_B" getbestblockhash | sed -n 's/.*"result":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
if [ -z "$HASH_A" ] || [ -z "$HASH_B" ] || [ "$HASH_A" != "$HASH_B" ]; then
  echo "[multinode-smoke] best hash mismatch after initial sync (nodeA=$HASH_A nodeB=$HASH_B)" >&2
  exit 1
fi
echo "[multinode-smoke] initial sync aligned: height=$HEIGHT_A hash=$HASH_A"

run_cli "$NODE_B" "$RPC_B" stop >/dev/null
sleep 2
kill "$PID_B" >/dev/null 2>&1 || true
"$DAEMON" run "-datadir=$NODE_B" "-p2pport=$P2P_B" "-rpcport=$RPC_B" "-connect=127.0.0.1:$P2P_A" >/tmp/legacy-nodeB.log 2>&1 &
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

HASH_B2="$(run_cli "$NODE_B" "$RPC_B" getbestblockhash | sed -n 's/.*"result":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
if [ -z "$HASH_B2" ] || [ "$HASH_B2" != "$HASH_A" ]; then
  echo "[multinode-smoke] best hash mismatch after reconnect (nodeA=$HASH_A nodeB=$HASH_B2)" >&2
  exit 1
fi
echo "[multinode-smoke] reconnect alignment verified: $HASH_B2"

echo "[multinode-smoke] completed"
