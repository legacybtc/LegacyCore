# Wallet Backup & Restore

## Backup

```powershell
.\legacycoin-cli.exe backupwallet D:\Backups\wallet-backup.json
```

```bash
./legacycoin-cli backupwallet /var/backups/legacycoin/wallet-backup.json
```

## Restore

1. Stop daemon
2. Restore backup into the data directory
3. Start daemon
4. Verify: `getwalletinfo`, `getbalance`, `listtransactions`, `listunspent`

## Safety

- Keep backups offline and encrypted
- Store multiple copies in separate locations
- Back up before upgrades or reindex
- Never share backups, keys, or passphrases
