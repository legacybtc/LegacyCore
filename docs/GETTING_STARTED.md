# Getting Started

Purpose: fast setup for first-time Legacy Core users.  
Audience: wallet users, miners, and new node operators.  
Status: active for v1.0.4.  
Safety warning: do not expose RPC (`19556`) publicly.

## What This Is

A short path to start Legacy Core, verify identity, and check basic sync.

## Quick Start

Windows:

1. Download and extract the official release ZIP.
2. Start `LegacyWallet.exe` (or `START_HERE.bat` if included in your package).
3. Confirm node status with:

```powershell
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getpeerinfo
```

4. Confirm wallet visibility with:

```powershell
.\legacycoin-cli.exe getwalletinfo
.\legacycoin-cli.exe getbalance
.\legacycoin-cli.exe getwalletsummary
.\legacycoin-cli.exe listtransactions
```

Linux:

```bash
chmod +x legacycoind legacycoin-cli
./legacycoind run -seed-peers
./legacycoin-cli getblockchaininfo
./legacycoin-cli getpeerinfo
```

## Verify Mainnet Identity

```bash
./legacycoind params
```

Expect:

- message start `a4 ac c6 4d`
- genesis hash `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- ports `19555` / `19556`

## Troubleshooting

If RPC fails, verify daemon is running and check:

```bash
./legacycoin-cli getsyncstatus
./legacycoin-cli checkstorage
```

If you moved wallet files to another Windows PC and balance is missing:

1. Verify you copied wallet files into `%APPDATA%\LegacyCoin`.
2. Check chain sync height (`getblockchaininfo`).
3. Re-check wallet data (`getwalletinfo`, `getbalance`, `listtransactions`).
4. If required, run `reindex` after backup.

## Known Limitations

- Some advanced explorer-style lookups require `txindex=1` or `addressindex=1`.
- Early mainnet operations should use extra backup and confirmation discipline.
