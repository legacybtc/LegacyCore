#!/usr/bin/env bash
# LegacyCore discover-node.sh — server discovery
set -e
echo "=== LegacyCore Node Discovery ==="
echo "PID: $(pgrep -x legacycoind 2>/dev/null || echo 'stopped')"
echo "Binary: $(which legacycoind 2>/dev/null || echo 'not found')"
echo "Data: ${LEGACYCOIN_DATADIR:-${HOME}/LegacyCoin}"
UNIT=$(systemctl is-active legacycoind 2>/dev/null || echo "inactive")
echo "Systemd: $UNIT"
RPC="http://127.0.0.1:19556/"
H=$(curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockcount","params":[]}' -H 'content-type:application/json' "$RPC" 2>/dev/null | grep -o '"result":[0-9]*' | grep -o '[0-9]*' || echo "unreachable")
echo "Height: $H"
