#!/usr/bin/env bash
set -euo pipefail

DAEMON="${1:-./legacycoind}"
CLI="${2:-./legacycoin-cli}"
ROUNDS="${3:-3}"
ROOT="${4:-/tmp/legacy-chaos-multinode}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CI_SCRIPT="$SCRIPT_DIR/chaos-ci-smoke.sh"

for i in $(seq 1 "$ROUNDS"); do
  echo "[chaos-multinode] round $i/$ROUNDS"
  bash "$CI_SCRIPT" "$DAEMON" "$CLI" "$ROOT/round-$i"
done

echo "[chaos-multinode] PASS rounds=$ROUNDS"
