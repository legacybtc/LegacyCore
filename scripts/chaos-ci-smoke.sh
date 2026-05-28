#!/usr/bin/env bash
set -Eeuo pipefail

DAEMON="${1:-./legacycoind}"
CLI="${2:-./legacycoin-cli}"
ROOT="${3:-${TMPDIR:-/tmp}/legacy-chaos-ci-smoke-$$}"

NODE_A="$ROOT/nodeA"
NODE_B="$ROOT/nodeB"

P2P_A="${P2P_A:-29755}"
RPC_A="${RPC_A:-29756}"
P2P_B="${P2P_B:-29757}"
RPC_B="${RPC_B:-29758}"

LOG_A="$ROOT/legacy-chaosA.log"
LOG_B="$ROOT/legacy-chaosB.log"

PID_A=""
PID_B=""

mkdir -p "$NODE_A" "$NODE_B"

run_cli() {
  local datadir="$1"
  local port="$2"
  shift 2
  "$CLI" -datadir "$datadir" -rpcport "$port" "$@"
}

dump_logs() {
  echo "===== node A log =====" >&2
  [ -f "$LOG_A" ] && tail -n 200 "$LOG_A" >&2 || true
  echo "===== node B log =====" >&2
  [ -f "$LOG_B" ] && tail -n 200 "$LOG_B" >&2 || true
}

cleanup() {
  set +e
  if [ -n "${PID_B:-}" ]; then
    run_cli "$NODE_B" "$RPC_B" stop >/dev/null 2>&1 || true
    sleep 1
    kill "$PID_B" >/dev/null 2>&1 || true
    wait "$PID_B" >/dev/null 2>&1 || true
  fi
  if [ -n "${PID_A:-}" ]; then
    run_cli "$NODE_A" "$RPC_A" stop >/dev/null 2>&1 || true
    sleep 1
    kill "$PID_A" >/dev/null 2>&1 || true
    wait "$PID_A" >/dev/null 2>&1 || true
  fi
  rm -rf "$ROOT" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() {
  echo "[chaos-ci-smoke] FAIL: $*" >&2
  dump_logs
  exit 1
}

extract_result_number() {
  sed -n \
    -e 's/.*"result":[[:space:]]*\([0-9][0-9]*\).*/\1/p' \
    -e 's/^[[:space:]]*\([0-9][0-9]*\)[[:space:]]*$/\1/p' | head -n1
}

extract_chain_id() {
  sed -n \
    -e 's/.*"chain_id":[[:space:]]*"\([^"]*\)".*/\1/p' \
    -e 's/^[[:space:]]*chain_id:[[:space:]]*\([^[:space:]]*\).*/\1/p' | head -n1
}

wait_rpc() {
  local datadir="$1"
  local port="$2"
  local label="$3"

  for _ in $(seq 1 90); do
    if run_cli "$datadir" "$port" getblockcount >/dev/null 2>&1; then
      echo "[chaos-ci-smoke] $label RPC ready"
      return 0
    fi
    sleep 1
  done

  fail "$label RPC did not become ready on port $port"
}

chain_id() {
  local datadir="$1"
  local port="$2"
  run_cli "$datadir" "$port" getchainparams 2>/dev/null | extract_chain_id
}

block_count() {
  local datadir="$1"
  local port="$2"
  run_cli "$datadir" "$port" getblockcount 2>/dev/null | extract_result_number
}

echo "[chaos-ci-smoke] root=$ROOT"
echo "[chaos-ci-smoke] starting node A p2p=$P2P_A rpc=$RPC_A"

"$DAEMON" run -datadir "$NODE_A" -p2pport "$P2P_A" -rpcport "$RPC_A" >"$LOG_A" 2>&1 &
PID_A=$!

wait_rpc "$NODE_A" "$RPC_A" "node A"

CHAIN_A="$(chain_id "$NODE_A" "$RPC_A" || true)"
[ -n "$CHAIN_A" ] || fail "node A chain id missing"

HEIGHT_A="$(block_count "$NODE_A" "$RPC_A" || true)"
[ -n "$HEIGHT_A" ] || fail "node A block count missing"

echo "[chaos-ci-smoke] starting node B p2p=$P2P_B rpc=$RPC_B"
"$DAEMON" run -datadir "$NODE_B" -p2pport "$P2P_B" -rpcport "$RPC_B" >"$LOG_B" 2>&1 &
PID_B=$!

wait_rpc "$NODE_B" "$RPC_B" "node B"

CHAIN_B="$(chain_id "$NODE_B" "$RPC_B" || true)"
[ -n "$CHAIN_B" ] || fail "node B chain id missing"
[ "$CHAIN_A" = "$CHAIN_B" ] || fail "chain id mismatch: A=$CHAIN_A B=$CHAIN_B"

HEIGHT_B="$(block_count "$NODE_B" "$RPC_B" || true)"
[ -n "$HEIGHT_B" ] || fail "node B block count missing"

echo "[chaos-ci-smoke] stopping node B for restart test"
run_cli "$NODE_B" "$RPC_B" stop >/dev/null 2>&1 || true
sleep 2
kill "$PID_B" >/dev/null 2>&1 || true
wait "$PID_B" >/dev/null 2>&1 || true
PID_B=""

echo "[chaos-ci-smoke] restarting node B"
"$DAEMON" run -datadir "$NODE_B" -p2pport "$P2P_B" -rpcport "$RPC_B" >"$LOG_B" 2>&1 &
PID_B=$!

wait_rpc "$NODE_B" "$RPC_B" "node B restart"

CHAIN_B2="$(chain_id "$NODE_B" "$RPC_B" || true)"
[ -n "$CHAIN_B2" ] || fail "node B restart chain id missing"
[ "$CHAIN_A" = "$CHAIN_B2" ] || fail "restart chain id mismatch: A=$CHAIN_A B=$CHAIN_B2"

HEIGHT_B2="$(block_count "$NODE_B" "$RPC_B" || true)"
[ -n "$HEIGHT_B2" ] || fail "node B restart block count missing"

echo "[chaos-ci-smoke] PASS"
