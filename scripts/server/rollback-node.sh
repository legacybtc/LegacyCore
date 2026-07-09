#!/usr/bin/env bash
# LegacyCore rollback-node.sh
set -e
BACKUP_DIR="${1:-}"
if [ -z "$BACKUP_DIR" ] || [ ! -d "$BACKUP_DIR" ]; then
    echo "Usage: sudo bash scripts/server/rollback-node.sh <backup-dir>"
    ls -d "${HOME}/legacycoin-backup-"* 2>/dev/null | sort -r | head -5
    exit 1
fi
BIN=$(which legacycoind 2>/dev/null || find /usr/local/bin /usr/bin -name legacycoind -type f | head -1)
[ -n "$BIN" ] || { echo "legacycoind not found"; exit 1; }
BIN_DIR=$(dirname "$BIN")
DATA_DIR="${LEGACYCOIN_DATADIR:-${HOME}/LegacyCoin}"
[ -d "$DATA_DIR" ] || DATA_DIR="${HOME}/.legacycoin"

echo "Rolling back from: $BACKUP_DIR"

systemctl stop legacycoind 2>/dev/null || pkill legacycoind 2>/dev/null || true
sleep 3

[ -f "$BACKUP_DIR/legacycoind" ] && cp "$BACKUP_DIR/legacycoind" "$BIN" && chmod +x "$BIN"
[ -f "$BACKUP_DIR/legacycoin-cli" ] && cp "$BACKUP_DIR/legacycoin-cli" "$BIN_DIR/" && chmod +x "$BIN_DIR/legacycoin-cli"
[ -f "$BACKUP_DIR/legacycoin.conf" ] && cp "$BACKUP_DIR/legacycoin.conf" "$DATA_DIR/"

systemctl start legacycoind 2>/dev/null || nohup "$BIN" run > /tmp/legacycoind.log 2>&1 &
echo "Rollback complete. Data directory was NOT modified."
