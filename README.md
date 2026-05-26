# Legacy Core

Legacy Core is the official Go full-node, CLI, miner, and wallet stack for Legacy Coin / LBTC.

Legacy Coin is a UTXO proof-of-work mainnet focused on local verification: run a node, hold keys locally, mine with CPU, and integrate through documented RPC rather than hosted trust.

This repository contains:

- `legacycoind`: full node daemon
- `legacycoin-cli`: JSON-RPC command-line client
- `Legacy Wallet`: Wails desktop wallet source
- shared consensus, wallet, P2P, RPC, storage, mining, and script packages
- release and CI hardening scripts

This repository does not include a mining pool server, public block explorer service, exchange backend, or launchpad service.

## Mainnet Identity

These values are not changed by v1.0.3:

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
| Linux data directory | `~/.legacycoin` |
| DNS seeds | `legacycoinseed.space`, `legacycoinseed2.space` |

Verify a build:

```powershell
.\legacycoind.exe params
```

```bash
./legacycoind params
```

## Downloads

Release assets are published at:

https://github.com/legacybtc/LegacyCore/releases

Expected v1.0.3 asset names:

- Windows wallet package: `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.3.zip`
- Linux core package: `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.3.tar.gz`
- Clean source archive: `LegacyCore-v1.0.3-source-clean.zip`

Always verify SHA256 checksums before running downloaded binaries.

## Build From Source

Required:

- Go matching `go.mod`
- Node.js 20 and npm
- Windows production builds: MSYS2 UCRT64 GCC for CGO yespower
- Linux production builds: GCC

Windows:

```powershell
cd cmd\legacywallet\frontend
npm install
npm run build
cd ..\..\..

go test ./...
go vet ./...
go build -trimpath -o legacycoind.exe ./cmd/legacycoind
go build -trimpath -o legacycoin-cli.exe ./cmd/legacycoin-cli
go build -trimpath -o legacy-wallet-internal.exe ./cmd/legacywallet
```

Linux:

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
cd ../../..

go test ./...
go vet ./...
go build -trimpath -o legacycoind ./cmd/legacycoind
go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
go build -trimpath -o legacy-wallet-internal ./cmd/legacywallet
```

Production yespower builds should report:

```text
yespower backend: cgo-c-reference
```

## Run Node

Windows:

```powershell
.\legacycoind.exe run -seed-peers
```

Linux:

```bash
./legacycoind run -seed-peers
```

## Run CLI

Windows:

```powershell
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe getpeerinfo
```

Linux:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
```

## Run Wallet

Windows release packages include the desktop wallet. Start it from the extracted release folder. The wallet starts and controls a local Legacy Core node.

Linux desktop wallet packaging is not the primary v1.0.3 target; Linux headless daemon and CLI are supported.

## Mining

Solo CPU mining is implemented. Stratum/pool server functionality is not implemented in this repository.

```powershell
.\legacycoin-cli.exe setupwallet "strong passphrase"
.\legacycoin-cli.exe getminingaddress
.\legacycoin-cli.exe setminerthreads 4
.\legacycoin-cli.exe startminer
.\legacycoin-cli.exe getminerstatus
.\legacycoin-cli.exe stopminer
```

```bash
./legacycoin-cli setupwallet "strong passphrase"
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
./legacycoin-cli stopminer
```

See [docs/MINING.md](docs/MINING.md).

## Seed Node Operation

Seed nodes may expose P2P `19555`. RPC `19556` must remain private/firewalled.

See [docs/SEED_NODE_OPERATOR.md](docs/SEED_NODE_OPERATOR.md).

## Integration Guides

- Pool integration: [docs/POOL_INTEGRATION.md](docs/POOL_INTEGRATION.md)
- Exchange integration: [docs/EXCHANGE_INTEGRATION.md](docs/EXCHANGE_INTEGRATION.md)
- Explorer integration: [docs/EXPLORER_INTEGRATION.md](docs/EXPLORER_INTEGRATION.md)
- RPC audit: [docs/RPC.md](docs/RPC.md)
- Confirmations and reorgs: [docs/CONFIRMATIONS_AND_REORGS.md](docs/CONFIRMATIONS_AND_REORGS.md)

## Verify Checksums

Windows:

```powershell
Get-FileHash -Algorithm SHA256 .\LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.3.zip
```

Linux:

```bash
sha256sum LegacyCore-LBTC-mainnet-linux-amd64-v1.0.3.tar.gz
```

Compare against the release `SHA256SUMS` file.

## Security Warnings

- RPC port `19556` must stay private.
- P2P port `19555` may be public.
- Never expose wallet/RPC publicly.
- Back up wallet data before use, mining, imports, or upgrades.
- Never share wallet.dat, private keys, seed material, or RPC cookies.
- Verify SHA256 checksums before running assets.
- Unsigned Windows builds may trigger SmartScreen.
- Legacy Core is early mainnet software; test operational flows with small amounts first.
- Seed operators should firewall RPC.
- Exchanges should treat hot wallets as high risk and maintain cold-wallet procedures.

See [SECURITY.md](SECURITY.md) and [docs/SECURITY_MODEL.md](docs/SECURITY_MODEL.md).

## Known Limitations

- Native address index: planned.
- Native full txindex: planned.
- Native reindex command: planned.
- External pool testing: still required.
- Exchange/explorer production certification: still required.
- Fork choice is audited as height-based rather than explicit cumulative chainwork-based.
- Linux GUI wallet packaging is not the focus of v1.0.3.

## Docs Index

- [docs/RPC.md](docs/RPC.md)
- [docs/MINING.md](docs/MINING.md)
- [docs/POOL_INTEGRATION.md](docs/POOL_INTEGRATION.md)
- [docs/EXCHANGE_INTEGRATION.md](docs/EXCHANGE_INTEGRATION.md)
- [docs/EXPLORER_INTEGRATION.md](docs/EXPLORER_INTEGRATION.md)
- [docs/SEED_NODE_OPERATOR.md](docs/SEED_NODE_OPERATOR.md)
- [docs/MAINNET_LAUNCH.md](docs/MAINNET_LAUNCH.md)
- [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)
- [docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md)
- [docs/STORAGE_AND_REINDEX.md](docs/STORAGE_AND_REINDEX.md)
- [docs/SECURITY_MODEL.md](docs/SECURITY_MODEL.md)
- [docs/CONFIRMATIONS_AND_REORGS.md](docs/CONFIRMATIONS_AND_REORGS.md)
- [docs/WINDOWS_BUILD.md](docs/WINDOWS_BUILD.md)

## Release Status

`v1.0.3-integration-hardening` is a source-code hardening, docs, CI, RPC, P2P, storage, tests, and release-process upgrade. It is not a chain rewrite and does not change mainnet identity.

## GitHub Metadata Suggestion

About:

```text
Legacy Core -- the official Go full-node, CLI, miner, and wallet stack for Legacy Coin / LBTC.
```

Topics:

```text
legacycoin lbtc legacy-core cryptocurrency blockchain full-node proof-of-work cpu-mining yespower mainnet go wallet p2p utxo
```
