#!/usr/bin/env bash
# LegacyCore verify-node.sh
set -e
RPC="http://127.0.0.1:19556/"
FAIL=0

rpc() { curl -s --data-binary "{\"jsonrpc\":\"1.0\",\"id\":\"1\",\"method\":\"$1\",\"params\":$2}" -H 'content-type:application/json' "$RPC" 2>/dev/null; }

check() {
    local label="$1" result="$2" expected="$3"
    if [ "$result" = "$expected" ]; then echo "  [PASS] $label"; else echo "  [FAIL] $label (got=$result want=$expected)"; FAIL=1; fi
}

echo "=== LegacyCore Node Verification ==="

# RPC reachable
if rpc getblockcount '[]' | grep -q '"result"'; then echo "  [PASS] RPC online"; else echo "  [FAIL] RPC offline"; FAIL=1; exit 1; fi

# Identity
GENESIS=$(rpc getblockhash '[0]' | grep -o '"result":"[a-f0-9]*"' | cut -d'"' -f3)
check "Genesis" "$GENESIS" "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"

MS=$(rpc getminerstatus '[]')
WCTX=$(echo "$MS" | grep -o '"yespower_worker_contexts_active":[0-9]*' | grep -o '[0-9]*' || echo "0")
CCTX=$(echo "$MS" | grep -o '"yespower_chain_contexts_active":[0-9]*' | grep -o '[0-9]*' || echo "0")
CGO=$(echo "$MS" | grep -o '"yespower_cgo_calls_active":[0-9]*' | grep -o '[0-9]*' || echo "0")
MA=$(echo "$MS" | grep -o '"active_mining":[a-z]*' | cut -d: -f2 || echo "false")

check "chain_contexts_active=1" "$CCTX" "1"
if [ "$MA" = "false" ]; then
    check "worker_contexts_active=0" "$WCTX" "0"
    check "cgo_calls_active=0" "$CGO" "0"
fi

HLT=$(rpc gethealth '[]' | grep -o '"status":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
STORAGE=$(rpc checkstorage '[]' | grep -o '"ok":[a-z]*' | cut -d: -f2 || echo "false")
check "health" "$HLT" "healthy"
check "storage" "$STORAGE" "true"

H=$(rpc getblockcount '[]' | grep -o '"result":[0-9]*' | grep -o '[0-9]*' || echo "0")
BEST=$(rpc getbestblockhash '[]' | grep -o '"result":"[a-f0-9]*"' | cut -d'"' -f3 || echo "unknown")
PEERS=$(rpc getpeerinfo '[]' | grep -o '"addr"' | wc -l)

echo "  Height: $H"
echo "  Best: $BEST"
echo "  Peers: $PEERS"
echo "  Contexts: worker=$WCTX chain=$CCTX cgo=$CGO"

echo ""
if [ $FAIL -eq 0 ]; then echo "LEGACY NODE VERIFICATION: PASS"; else echo "LEGACY NODE VERIFICATION: FAIL"; exit 1; fi
