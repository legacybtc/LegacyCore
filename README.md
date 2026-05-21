# Legacy Coin / LBTC

Legacy Coin is a fair-launch UTXO proof-of-work network focused on full-node
verification, CPU-accessible mining, and a simple desktop wallet experience.

This repository is the public source for:

- Legacy Core 1.0.0 source
- Legacy Wallet 1.0.0 desktop source

Legacy Pool and Legacy Explorer / Launchpad are maintained separately and are
not included in this repository.

## RC2 Public Mainnet Reset

This source targets the clean public mainnet reset identity. Old pre-reset
wallets, nodes, pools, miners, explorer databases, and local chain data are
obsolete and must not be mixed with this release.

| Field | Value |
|---|---|
| Coin | Legacy Coin |
| Ticker | LBTC |
| Network | Legacy Coin Mainnet |
| Chain ID | `legacy-mainnet-1.0.0-rc2-5b4c78e4` |
| P2P port | `19555` |
| RPC port | `19556` |
| Message start | `a4 ac c6 4d` |
| Genesis timestamp | `onecpuonevote Legacy Coin Public Mainnet 20/May/2026` |
| Genesis time | `1779235200` |
| Genesis bits | `207fffff` |
| Post-genesis bits | `1f0fffff` |
| Genesis nonce | `3` |
| Genesis hash | `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5` |
| PoW | `LegacyCoinPoW, N=2048, r=32` |
| PoW input | Serialized 80-byte Legacy block header |
| Production yespower backend | `cgo-c-reference` |

## Clean Upgrade Warning

If you tested earlier builds, back up wallets, wallet backups, private keys, and
seed phrases before changing any data.

After backup, remove only old runtime chain/network data:

- `blocks`
- `chainstate`
- `peers.dat`
- mempool cache files

Do not delete:

- wallet backups
- private keys
- seed phrases

## Production PoW Requirement

Public RC2 binaries that validate blocks, mine blocks, validate pool shares, or
submit blocks must be built with CGO enabled and the bundled C yespower backend.

Expected verification output:

```text
yespower backend: cgo-c-reference
```

The pure-Go yespower path is experimental/debug only unless byte-for-byte parity
with the C reference path is proven. Do not publish production mining, pool, or
submitblock-capable binaries that report the pure-Go backend.

## Build Checks

Core checks:

```bash
export CGO_ENABLED=1
go test ./...
go vet ./...
go build -trimpath -o legacycoind ./cmd/legacycoind
go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
./legacycoind params
```

The `params` command must show the RC2 identity above and:

```text
yespower backend: cgo-c-reference
```

## Desktop Wallet

Legacy Wallet is a Wails desktop wallet that starts and manages the internal
Legacy Core node for normal users.

Frontend build:

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
```

Windows Wails build:

```powershell
$env:CGO_ENABLED="1"
wails build -platform windows/amd64 -skipbindings
```

Linux Wails builds require native Linux desktop dependencies. Do not ship a
pure-Go fallback as a production wallet build.

## Windows SmartScreen Notice

Initial Windows builds may be unsigned and may trigger Microsoft SmartScreen.
Verify SHA256 checksums from the official release before running. If the
checksum matches the official release, Windows users may click:

```text
More info -> Run anyway
```

Do not bypass warnings for unofficial mirrors, modified files, or mismatched
checksums.

## RPC Safety

Legacy Core RPC must remain local/private unless the operator has configured
proper authentication, TLS, and firewall rules. Do not expose wallet RPC methods
to the public internet.

### Local RPC Authentication

For normal same-machine use, Legacy Core uses cookie authentication.

Start the node:

```bash
./legacycoind run -seed-peers
```

When no `rpcuser` / `rpcpassword` is configured, `legacycoind` automatically
creates an RPC cookie in the active data directory:

- Linux: `~/.legacycoin/.cookie`
- Windows: `%APPDATA%\LegacyCoin\.cookie`
- Custom data dir: `<datadir>/.cookie`

Then the CLI works from another terminal on the same machine:

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getblockcount
```

Custom data directory:

```bash
./legacycoind run -datadir=/opt/legacycoin-data -seed-peers
./legacycoin-cli -datadir=/opt/legacycoin-data getnetworkinfo
```

Explicit RPC username/password flow:

```ini
# legacycoin.conf
rpcuser=legacyrpc
rpcpassword=strong_password
rpcbind=127.0.0.1
rpcport=19556
```

```bash
./legacycoin-cli -rpcuser=legacyrpc -rpcpassword=strong_password getnetworkinfo
```

Supported CLI connection flags:

```text
-datadir=<path>
-rpcuser=<user>
-rpcpassword=<password>
-rpccookiefile=<path>
-rpcconnect=<host>
-rpcport=<port>
-rpcurl=<url>
```

If the CLI reports `RPC cookie not found`, start `legacycoind` first or provide
`-rpcuser` and `-rpcpassword`. If it reports `RPC unauthorized`, check the
cookie file or explicit RPC credentials.

## Helpful Commands

```bash
legacycoind params
legacycoind run -seed-peers
legacycoin-cli getinfo
legacycoin-cli getnetworkinfo
legacycoin-cli getselfcheck
legacycoin-cli getlaunchstatus
```

## Clean Restart Runbook

See:

- `docs/RC2_CLEAN_RESTART_RUNBOOK.md`
- `docs/PUBLIC_MAINNET_IDENTITY_RESET_RC2.md`

## Security

See `SECURITY.md` for vulnerability reporting and release security checks.
