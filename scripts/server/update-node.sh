#!/usr/bin/env bash
# LegacyCore update-node.sh — one-command server deployment
# Run as root from the LegacyCore repository root.
# Usage: sudo bash scripts/server/update-node.sh
set -Eeuo pipefail
trap 'echo "UPDATE FAILED at line $LINENO" >&2; exit 1' ERR

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$REPO_ROOT"

COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -Iseconds)
BACKUP_DIR="${HOME}/legacycoin-backup-$(date +%Y%m%d-%H%M%S)"
EVIDENCE_DIR="/tmp/legacycoin-update-$(date +%Y%m%d-%H%M%S)"
TMP_BUILD="/tmp/legacycoin-build-$$"
VERSION="1.0.8"

mkdir -p "$BACKUP_DIR" "$EVIDENCE_DIR" "$TMP_BUILD"

log() { echo "[$(date +%H:%M:%S)] $*" | tee -a "$EVIDENCE_DIR/update.log"; }

# --- PRECONDITIONS ---
log "=== LegacyCore Update $VERSION (commit ${COMMIT:0:7}) ==="
log "System: $(uname -m)"
[ "$(uname -m)" = "x86_64" ] || { log "ERROR: x86_64 required"; exit 1; }
command -v gcc >/dev/null || { log "ERROR: gcc required"; exit 1; }
command -v go >/dev/null || { log "ERROR: Go required"; exit 1; }
[ "$(git status --porcelain)" = "" ] || { log "ERROR: working tree must be clean"; exit 1; }
[ "${CGO_ENABLED:-1}" = "1" ] || export CGO_ENABLED=1

# --- DISCOVERY ---
log "Discovering current node..."
CUR_BIN=$(which legacycoind 2>/dev/null || find /usr/local/bin /usr/bin /opt -name legacycoind -type f 2>/dev/null | head -1)
[ -n "$CUR_BIN" ] || { log "ERROR: legacycoind not found. Install first, then run update."; exit 1; }
CUR_BIN_DIR=$(dirname "$CUR_BIN")
CUR_CLI="${CUR_BIN_DIR}/legacycoin-cli"
CUR_PID=$(pgrep -x legacycoind 2>/dev/null || true)
DATA_DIR="${LEGACYCOIN_DATADIR:-${HOME}/LegacyCoin}"
[ -d "$DATA_DIR" ] || DATA_DIR="${HOME}/.legacycoin"

UNIT=""
if systemctl is-active --quiet legacycoind 2>/dev/null; then UNIT="legacycoind"; fi
log "Binary: $CUR_BIN (PID: ${CUR_PID:-stopped})"
log "Service: ${UNIT:-manual/nohup}"
log "Data dir: $DATA_DIR"

# --- BACKUP ---
log "Creating backup in $BACKUP_DIR"
cp "$CUR_BIN" "$BACKUP_DIR/"
[ -f "$CUR_CLI" ] && cp "$CUR_CLI" "$BACKUP_DIR/" || true
[ -f "$DATA_DIR/legacycoin.conf" ] && cp "$DATA_DIR/legacycoin.conf" "$BACKUP_DIR/" || true

# Pre-update state
RPC="http://127.0.0.1:19556/"
PRE_H=$(curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockcount","params":[]}' -H 'content-type:application/json' "$RPC" 2>/dev/null | grep -o '"result":[0-9]*' | grep -o '[0-9]*' || echo "0")
PRE_BEST=$(curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getbestblockhash","params":[]}' -H 'content-type:application/json' "$RPC" 2>/dev/null | grep -o '"result":"[a-f0-9]*"' | cut -d'"' -f3 || echo "unknown")

cat > "$EVIDENCE_DIR/pre-update.json" << EOF
{"commit":"$COMMIT","version":"$VERSION","pre_height":$PRE_H,"pre_best_hash":"$PRE_BEST","binary":"$CUR_BIN"}
EOF

# --- BUILD ---
log "Building from commit $COMMIT..."
export CGO_ENABLED=1 CC=gcc

LDFLAGS="-X legacycoin/legacy-go/internal/version.CoreVersion=$VERSION"
LDFLAGS="$LDFLAGS -X legacycoin/legacy-go/internal/version.CoreCommit=$COMMIT"
LDFLAGS="$LDFLAGS -X legacycoin/legacy-go/internal/version.BuildTime=$BUILD_TIME"

go test -short -count=1 ./... 2>&1 | tee "$EVIDENCE_DIR/go-test.log"
go vet ./... 2>&1 | tee "$EVIDENCE_DIR/go-vet.log"

go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_BUILD/legacycoind" ./cmd/legacycoind/ 2>&1 | tee "$EVIDENCE_DIR/build.log"
go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_BUILD/legacycoin-cli" ./cmd/legacycoin-cli/

# --- IDENTITY ---
log "Verifying mainnet identity..."
"$TMP_BUILD/legacycoind" params > "$TMP_BUILD/params.txt"
check_param() { grep -q "$2" "$TMP_BUILD/params.txt" || { log "IDENTITY FAILED: $1"; exit 1; }; }
check_param "genesis" "genesis hash: 5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"
check_param "yespower" "yespower backend: cgo-c-reference"
check_param "p2p_port" "p2p port: 19555"
check_param "rpc_port" "rpc port: 19556"
check_param "msg_start" "message start: a4 ac c6 4d"
log "Identity verified"

# --- STOP ---
log "Stopping legacycoind..."
if [ -n "$UNIT" ]; then
    systemctl stop "$UNIT"
else
    [ -n "$CUR_PID" ] && kill "$CUR_PID" 2>/dev/null || true
fi
for i in $(seq 1 15); do
    if ! pgrep -x legacycoind >/dev/null 2>&1; then break; fi
    sleep 1
done
if pgrep -x legacycoind >/dev/null 2>&1; then
    log "ERROR: legacycoind still running after stop"
    exit 1
fi
log "Daemon stopped"

# --- INSTALL ---
log "Installing new binaries..."
cp "$TMP_BUILD/legacycoind" "$CUR_BIN"
cp "$TMP_BUILD/legacycoin-cli" "$CUR_BIN_DIR/"
chmod +x "$CUR_BIN" "$CUR_CLI"

# --- START ---
log "Starting legacycoind..."
if [ -n "$UNIT" ]; then
    systemctl start "$UNIT"
else
    nohup "$CUR_BIN" run > /tmp/legacycoind.log 2>&1 &
fi

# --- VERIFY ---
log "Waiting for RPC (up to 30s)..."
for i in $(seq 1 15); do
    if curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockcount","params":[]}' -H 'content-type:application/json' "$RPC" > /dev/null 2>&1; then
        log "RPC ready after $((i*2))s"
        break
    fi
    sleep 2
done

POST_H=$(curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getblockcount","params":[]}' -H 'content-type:application/json' "$RPC" 2>/dev/null | grep -o '"result":[0-9]*' | grep -o '[0-9]*' || echo "0")
POST_BEST=$(curl -s --data-binary '{"jsonrpc":"1.0","id":"1","method":"getbestblockhash","params":[]}' -H 'content-type:application/json' "$RPC" 2>/dev/null | grep -o '"result":"[a-f0-9]*"' | cut -d'"' -f3 || echo "unknown")

log "Pre-update:  height=$PRE_H hash=$PRE_BEST"
log "Post-update: height=$POST_H hash=$POST_BEST"

# --- ROLLBACK COMMAND ---
cat > "$BACKUP_DIR/rollback.sh" << RBEOF
#!/bin/bash
set -e
systemctl stop legacycoind 2>/dev/null || pkill legacycoind || true
sleep 2
cp "$BACKUP_DIR/legacycoind" "$CUR_BIN"
[ -f "$BACKUP_DIR/legacycoin-cli" ] && cp "$BACKUP_DIR/legacycoin-cli" "$CUR_CLI" || true
[ -f "$BACKUP_DIR/legacycoin.conf" ] && cp "$BACKUP_DIR/legacycoin.conf" "$DATA_DIR/" || true
chmod +x "$CUR_BIN" "$CUR_CLI"
systemctl start legacycoind 2>/dev/null || nohup "$CUR_BIN" run > /tmp/legacycoind.log 2>&1 &
echo "Rollback complete from $BACKUP_DIR"
RBEOF
chmod +x "$BACKUP_DIR/rollback.sh"

# --- EVIDENCE ---
cat > "$EVIDENCE_DIR/build-info.json" << EOF
{"commit":"$COMMIT","version":"$VERSION","build_time":"$BUILD_TIME","go":"$(go version)","os":"linux","arch":"amd64"}
EOF
sha256sum "$CUR_BIN" > "$EVIDENCE_DIR/new-binary-sha256.txt"

log "=== UPDATE COMPLETE ==="
log "Evidence: $EVIDENCE_DIR"
log "Backup + rollback: $BACKUP_DIR/rollback.sh"
echo ""
echo "LEGACY NODE UPDATE: SUCCESS"
echo "Commit: $COMMIT"
echo "Pre height:  $PRE_H"
echo "Post height: $POST_H"
