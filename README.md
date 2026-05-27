# Legacy Core

Legacy Core is the official Go full-node, CLI, miner, and desktop wallet stack for Legacy Coin / LBTC.

This repository targets infrastructure-grade readiness for:

- wallet users
- node operators
- solo miners
- pool/exchange/explorer integrators
- source builders and contributors

## What Is Legacy Core

Included in this repository:

- `legacycoind` (full node daemon)
- `legacycoin-cli` (JSON-RPC CLI)
- Legacy Wallet source (`cmd/legacywallet`, Wails desktop UI)
- consensus/network/wallet/mining/storage implementation
- release/CI/build/test tooling

Not included in this repository:

- production mining-pool server implementation
- hosted public explorer service deployment
- hosted exchange backend infrastructure

## Mainnet Identity (Must Not Change)

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
| Data dir (Linux) | `~/.legacycoin` |
| DNS seeds | `legacycoinseed.space`, `legacycoinseed2.space` |

Verify a build:

```powershell
.\legacycoind.exe params
```

```bash
./legacycoind params
```

## Release Matrix

| Platform | Architecture | Archive | GUI Wallet | Status |
| --- | --- | --- | --- | --- |
| Windows | x86_64 | `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.4.zip` | Included | Supported |
| Linux | x86_64 | `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.4.tar.gz` | CLI/daemon | Supported |
| Linux | arm64 | `LegacyCore-LBTC-mainnet-linux-arm64-v1.0.4.tar.gz` | CLI/daemon | Experimental |
| macOS | x86_64 | `LegacyCore-LBTC-mainnet-macos-amd64-v1.0.4.tar.gz` | CLI/daemon | Experimental |
| macOS | arm64 | `LegacyCore-LBTC-mainnet-macos-arm64-v1.0.4.tar.gz` | CLI/daemon | Experimental |

Release assets are published at:

- <https://github.com/legacybtc/LegacyCore/releases>

## Download Verification

Windows:

```powershell
Get-FileHash -Algorithm SHA256 .\LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.4.zip
```

Linux/macOS:

```bash
sha256sum LegacyCore-LBTC-mainnet-linux-amd64-v1.0.4.tar.gz
```

Compare with `SHA256SUMS.txt` from the release.

## Quick Start

Windows wallet package:

1. Extract release ZIP.
2. Run `START_HERE.bat`.
3. Verify node status in wallet diagnostics.

Linux headless package:

```bash
chmod +x legacycoind legacycoin-cli
./legacycoind run -seed-peers
```

Second terminal:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getminerstatus
```

## Build From Source

Requirements:

- Go (per `go.mod`)
- Node.js 20 + npm (wallet frontend build)
- CGO-capable C compiler
- Windows production build: MSYS2 UCRT64 GCC (`cgo-c-reference` yespower backend)

Windows:

```powershell
.\scripts\check-windows-build-env.ps1
.\scripts\build-windows.ps1
```

Linux:

```bash
bash scripts/build-linux.sh amd64
```

Cross-platform Make targets:

```bash
make frontend
make test
make vet
make package-linux
make package-windows
```

## Run Node / CLI / Wallet

Node:

```bash
./legacycoind run -seed-peers
```

CLI:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
```

Wallet:

- Desktop package includes `legacy-wallet(.exe)` and internal node lifecycle controls.

## Mining

Solo CPU mining:

```bash
./legacycoin-cli setupwallet "strong passphrase"
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
```

Network hashrate in RPC/UI is an estimate from recent chain data, not total node count.

## Pool / Exchange / Explorer Integration

Smoke scripts:

- `scripts/pool-rpc-smoke.ps1` / `scripts/pool-rpc-smoke.sh`
- `scripts/exchange-rpc-smoke.ps1` / `scripts/exchange-rpc-smoke.sh`
- `scripts/explorer-rpc-smoke.ps1` / `scripts/explorer-rpc-smoke.sh`

Important:

- Address search/index RPCs are planned and are not faked.
- Dedicated txindex/addressindex foundations are still staged work.

## Seed Operator / Monitoring

Use:

```bash
legacycoin-cli getblockchaininfo
legacycoin-cli getnetworkinfo
legacycoin-cli getpeerinfo
legacycoin-cli getsyncstatus
legacycoin-cli checkstorage
legacycoin-cli doctor
```

## Security Warnings

- Keep RPC (`19556`) private/firewalled.
- P2P (`19555`) may be public.
- Do not expose wallet or privileged RPC to public internet.
- Back up wallet before migration/reindex/upgrade.
- Verify release checksums before execution.
- Treat exchange hot wallets as high-risk; use cold-wallet controls.

## Known Limitations

- Active fork-choice remains height-driven; explicit cumulative-chainwork winner selection is staged work.
- Dedicated txindex and addressindex are not yet fully implemented.
- Address search APIs are intentionally not exposed until address index support is real.
- macOS and Linux ARM64 packaging is experimental and environment-dependent.
- External pool certification is pending third-party production validation.

## Docs Index

- [docs/RPC.md](docs/RPC.md)
- [docs/MINING.md](docs/MINING.md)
- [docs/POOL_INTEGRATION.md](docs/POOL_INTEGRATION.md)
- [docs/EXCHANGE_INTEGRATION.md](docs/EXCHANGE_INTEGRATION.md)
- [docs/EXPLORER_INTEGRATION.md](docs/EXPLORER_INTEGRATION.md)
- [docs/SEED_NODE_OPERATOR.md](docs/SEED_NODE_OPERATOR.md)
- [docs/P2P_PROTOCOL.md](docs/P2P_PROTOCOL.md)
- [docs/MONITORING.md](docs/MONITORING.md)
- [docs/STORAGE_AND_REINDEX.md](docs/STORAGE_AND_REINDEX.md)
- [docs/CONFIRMATIONS_AND_REORGS.md](docs/CONFIRMATIONS_AND_REORGS.md)
- [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)
- [docs/MULTINODE_TESTING.md](docs/MULTINODE_TESTING.md)
- [docs/RELEASE_PROCESS.md](docs/RELEASE_PROCESS.md)
- [docs/RELEASE_SCORECARD.md](docs/RELEASE_SCORECARD.md)
- [docs/RELEASE_NOTES_TEMPLATE.md](docs/RELEASE_NOTES_TEMPLATE.md)
- [docs/WINDOWS_SERVICE.md](docs/WINDOWS_SERVICE.md)
- [docs/SECURITY_MODEL.md](docs/SECURITY_MODEL.md)
