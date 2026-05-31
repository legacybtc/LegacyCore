# Legacy Core

Legacy Core is the official full-node, CLI, miner, and desktop wallet stack for Legacy Coin (LBTC).

## What Is Legacy Core

This repository provides:

- `legacycoind` (full node daemon)
- `legacycoin-cli` (RPC command-line client)
- Legacy Wallet source (`cmd/legacywallet`)
- Core chain, wallet, mining, P2P, storage, and RPC implementation

This repository does **not** include hosted pool, exchange, or explorer infrastructure services.

## Mainnet Identity

| Field | Value |
| --- | --- |
| Coin | Legacy Coin / LBTC |
| Message start | `a4 ac c6 4d` |
| Genesis hash | `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5` |
| Genesis time | `1779235200` |
| Genesis nonce | `3` |
| P2P port | `19555` |
| RPC port | `19556` |
| yespower personalization | `LegacyCoinPoW` |
| Data dir (Linux default) | `~/.legacycoin` |
| DNS seeds | `legacycoinseed.space`, `legacycoinseed2.space` |

Verify any build:

```powershell
.\legacycoind.exe params
```

```bash
./legacycoind params
```

## Download / Release Note

Release assets are published on GitHub Releases:  
[LegacyCore Releases](https://github.com/legacybtc/LegacyCore/releases)

Current release naming includes:

- `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.4.zip`
- `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.4.tar.gz`

Always verify SHA256 checksums before use.

## Quick Start

Windows:

1. Extract the wallet ZIP.
2. Run `LegacyWallet.exe` (or `START_HERE.bat`).
3. Check status with wallet diagnostics or CLI.

Linux:

```bash
chmod +x legacycoind legacycoin-cli
./legacycoind run -seed-peers
```

Second terminal:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getpeerinfo
```

## Build From Source

Windows:

```powershell
.\scripts\check-windows-build-env.ps1
.\scripts\build-windows.ps1
```

Linux:

```bash
bash scripts/build-linux.sh amd64
```

Frontend build (wallet source):

```powershell
cd cmd\legacywallet\frontend
npm install
npm run build
cd ..\..\..
```

## Run Node

```bash
./legacycoind run -seed-peers
```

## Run CLI

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
```

## Run Wallet

Build/run from source in `cmd/legacywallet` using Wails, or use the Windows wallet release package.

```powershell
cd cmd\legacywallet
wails build
```

## Mining

Solo CPU mining quick start:

```bash
./legacycoin-cli setupwallet "strong passphrase"
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
```

## Docs Index

See [docs/README.md](docs/README.md) for organized documentation by audience.

## Security Warnings

- Keep RPC (`19556`) private.
- P2P (`19555`) may be public.
- Never share wallet backups, private keys, or RPC credentials.
- Verify checksums before running binaries.
- Back up wallet data before upgrades or reindex operations.

## Known Limitations

- `txindex` and `addressindex` are opt-in foundations (`txindex=1`, `addressindex=1`) and require rebuild/reindex when enabled on existing data.
- Address index RPCs are available only when `addressindex=1`.
- External third-party pool certification is still required.
- macOS and Linux ARM64 packaging remains environment-dependent.
