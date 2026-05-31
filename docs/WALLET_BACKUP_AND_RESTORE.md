# Wallet Backup and Restore

Purpose: safe wallet backup and restore guidance.  
Audience: wallet users, miners, and operators holding funds.  
Status: active for v1.0.4.  
Safety warning: never share wallet backups, private keys, or passphrases.

## What This Is

This document shows how to back up wallet data and verify restore readiness.

## Quick Backup

Windows:

```powershell
.\legacycoin-cli.exe backupwallet D:\LegacyBackups\legacy-wallet-backup.json
```

Linux:

```bash
./legacycoin-cli backupwallet /var/backups/legacycoin/legacy-wallet-backup.json
```

## Important Warnings

- Keep backups offline and encrypted.
- Store multiple copies in separate secure locations.
- Back up after key creation/import and before upgrades/reindex.

## Restore Notes

1. Stop daemon/wallet.
2. Restore backup into the correct data directory.
3. Start daemon.
4. Run:

```bash
./legacycoin-cli getwalletinfo
./legacycoin-cli getbalance
./legacycoin-cli listaddresses
./legacycoin-cli getwalletsummary
./legacycoin-cli listtransactions
./legacycoin-cli listunspent
```

5. If chain data is missing/corrupt, use reindex flow (does not change consensus).

## Troubleshooting

- If balances look wrong right after restore, verify sync height first.
- If you migrated to another Windows PC, verify wallet files are in:
  `%APPDATA%\LegacyCoin`
- If mining rewards are visible but not spendable, check coinbase maturity (100 confirmations).
- If storage issues appear, run:

```bash
./legacycoin-cli checkstorage
./legacycoin-cli reindex
```

## Known Limitations

- Backup format and operational policy are still early-mainnet and should be tested in staging before production use.
