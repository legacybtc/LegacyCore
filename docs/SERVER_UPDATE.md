# Legacy Core Server Update Guide

## Operator Workflow (Node 1 first)

```bash
cd ~/LegacyCore
git fetch origin
git checkout main
git pull --ff-only origin main
sudo bash scripts/server/update-node.sh
sudo bash scripts/server/verify-node.sh
```

**Expected duration:** 5-10 minutes
**Required disk space:** ~500 MB (build artifacts)
**Required packages:** git, go (from go.mod), gcc

## What the script does

1. Discovers current legacycoind binary, service, data directory
2. Creates timestamped backup of current binaries and config
3. Records pre-update height and best-block hash
4. Runs `go test -short` and `go vet`
5. Builds new binaries from the checked-out commit
6. Verifies mainnet identity (genesis, chain ID, ports, yespower)
7. Stops the daemon cleanly, confirms process exited
8. Installs new binaries atomically
9. Restarts using the original launch method (systemd or nohup)
10. Waits for RPC, reports pre/post height

## Rollback

```bash
sudo bash scripts/server/rollback-node.sh ~/legacycoin-backup-YYYYMMDD-HHMMSS
```

The update script prints the exact rollback command at completion.

## Verification

```bash
cd ~/LegacyCore
sudo bash scripts/server/verify-node.sh
```

Expected output:
```
LEGACY NODE VERIFICATION: PASS
```

## Two-Node Deployment

1. Update Node 1
2. Run verify-node.sh on Node 1
3. Send results to owner for approval
4. After approval, update Node 2
5. Run verify-node.sh on Node 2
6. Never update both nodes simultaneously

## Backup Locations

- Binaries/config backup: `~/legacycoin-backup-YYYYMMDD-HHMMSS/`
- Update evidence/logs: `/tmp/legacycoin-update-YYYYMMDD-HHMMSS/`
- Build artifacts: `/tmp/legacycoin-build-*/`

## Data Safety

- Blockchain data is NEVER deleted
- Wallet files are NEVER deleted
- Configuration is backed up before modification
- Rollback restores exact previous state

## Evidence to Report

After update, send this to the owner:
```
Hostname: <node hostname>
Deployed commit: <commit SHA>
Update result: SUCCESS
Verification result: PASS
Height: <current>
Best block: <hash>
Peers: <count>
Evidence directory: /tmp/legacycoin-update-YYYYMMDD-HHMMSS/
```
