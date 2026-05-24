# Legacy Coin / LBTC

Legacy Coin is a fair-launch UTXO proof-of-work network focused on local
ownership: run a node, hold keys locally, mine with CPU, and verify the network
from your own computer.

This public `LegacyCore` repository contains:

- `legacycoind` source: Legacy Core full node
- `legacycoin-cli` source: command-line RPC client
- `Legacy Wallet` desktop source: Windows-first desktop wallet built with Wails
- shared `internal/` packages, configs, scripts, and build files

This repository does **not** include the mining pool, public explorer, or
Launchpad service source. Those are separate products/repositories.

## Quick Start

If you are a normal Windows user:

1. Go to Releases:
   https://github.com/legacybtc/LegacyCore/releases

2. Download:
   `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.1.zip`

3. Extract the ZIP.

4. Open the extracted folder.

5. Double-click:
   `START_HERE.bat`

6. Legacy Wallet will start the local Legacy Core node automatically.

7. Wait for:
   - Mainnet status
   - synced/syncing status
   - connected peers

8. Go to `Receive` and create/copy a receive address.

If you are a Linux node operator / miner:

1. Go to Releases:
   https://github.com/legacybtc/LegacyCore/releases

2. Download:
   `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz`

3. Extract and run:

   ```bash
   tar -xzf LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz
   cd LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0
   chmod +x legacycoind legacycoin-cli
   ./legacycoind run -seed-peers
   ```

4. In another terminal:

   ```bash
   ./legacycoin-cli getnetworkinfo
   ./legacycoin-cli getblockcount
   ./legacycoin-cli getpeerinfo
   ```

## What Should I Download?

From this `LegacyCore` repository/release:

| User type | Download | What it does |
|---|---|---|
| Windows normal user | `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.1.zip` | Desktop wallet + local node |
| Linux node operator | `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz` | Full node + CLI |
| Developer | GitHub source or source ZIP | Build Core, CLI, and Wallet from source |

Separate products, not included in this repository:

| User type | Product | Where it belongs |
|---|---|---|
| Miner / pool operator | Legacy Pool | Separate `LegacyPool` repository/package |
| Explorer operator | Legacy Explorer / Launchpad service | Separate explorer/launchpad repository/package |

If you only want to use LBTC, download the Windows desktop wallet.

If you want to run infrastructure, use the Linux node package.

If you are unsure, start with the Windows desktop wallet.

## Windows ZIP: Headless Node and CLI Miner

The Windows wallet ZIP also includes the headless Core daemon and CLI. This works
from the extracted release folder without Go, GCC, MSYS2, npm, Wails, or any
developer tools.

First PowerShell:

```powershell
.\legacycoind.exe params
.\legacycoind.exe run -seed-peers
```

Second PowerShell:

```powershell
.\legacycoin-cli.exe getblockcount
.\legacycoin-cli.exe getsyncstatus
.\legacycoin-cli.exe getpeerinfo
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe getminingaddress
.\legacycoin-cli.exe setminerthreads 4
.\legacycoin-cli.exe startminer
.\legacycoin-cli.exe getminerstatus
.\legacycoin-cli.exe stopminer
.\legacycoin-cli.exe stop
```

Mining from the CLI is solo mining. Hashrate does not guarantee a block. Rewards
only appear if your miner finds a block, and coinbase rewards mature after 100
blocks.

## Windows Desktop Wallet: Download and Run

1. Download:
   `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.1.zip`

2. Right-click ZIP -> Extract All.

3. Open the extracted folder.

4. Double-click:
   `START_HERE.bat`

5. If Windows SmartScreen appears:
   - verify SHA256 first
   - click `More Info`
   - click `Run Anyway` only if checksum matches the official GitHub release

6. Wallet should open and start the local node.

7. Check the bottom/status area:
   - Network: Mainnet
   - Height
   - Peers

8. Go to `Receive` and create/copy a receive address.

9. Back up wallet/private keys before using real funds.

Warning:

- Do not delete wallet files, private keys, or seed phrases.
- If you tested old pre builds, back up wallet first, then remove only old
  runtime data:
  - `blocks`
  - `chainstate`
  - `peers.dat`
  - mempool cache
- Do not delete:
  - wallet backups
  - private keys
  - seed phrases

## Windows Source Build from GitHub

Normal users should use the release ZIP. Source builds are for developers and
require Go, Node.js, and MSYS2 UCRT64 GCC because production yespower uses CGO.

Clone:

```powershell
git clone https://github.com/legacybtc/LegacyCore.git
cd LegacyCore
```

Check your build environment:

```powershell
powershell.exe -ExecutionPolicy Bypass -File scripts\check-windows-build-env.ps1
```

Build:

```powershell
.\build-windows.bat
```

Common build errors:

- `pattern all:frontend/dist: no matching files found`: build the wallet
  frontend first with `npm install` and `npm run build` in
  `cmd\legacywallet\frontend`.
- `cgo: C compiler "gcc" not found`: install MSYS2 UCRT64 GCC with
  `C:\msys64\usr\bin\pacman.exe -S --needed mingw-w64-ucrt-x86_64-gcc`.
- `yespower backend: pure-go-experimental`: CGO/GCC was not used. Public
  production binaries must show `yespower backend: cgo-c-reference`.

Full Windows build details are in `docs/WINDOWS_BUILD.md`.

## Linux Full Node: Download and Run

1. Download:
   `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz`

2. Extract:

   ```bash
   tar -xzf LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz
   cd LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0
   ```

3. Make binaries executable:

   ```bash
   chmod +x legacycoind legacycoin-cli
   ```

4. Verify params:

   ```bash
   ./legacycoind params
   ```

   Expected:

   ```text
   message start: a4 ac c6 4d
   genesis hash: 5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5
   yespower backend: cgo-c-reference
   ```

5. Start node:

   ```bash
   ./legacycoind run -seed-peers
   ```

6. Open second terminal:

   ```bash
   ./legacycoin-cli getnetworkinfo
   ./legacycoin-cli getblockcount
   ./legacycoin-cli getpeerinfo
   ```

7. Force-connect if needed:

   ```bash
   ./legacycoind run -connect legacycoinseed.space:19555
   ./legacycoind run -connect legacycoinseed2.space:19555
   ```

8. RPC should stay local/private. Do not expose port `19556` publicly.

## Verify SHA256 Checksums

Only run binaries if checksums match the official GitHub release.

Windows PowerShell:

```powershell
Get-FileHash .\LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.1.zip -Algorithm SHA256
```

Linux:

```bash
sha256sum LegacyCore-LBTC-mainnet-linux-amd64-v1.0.0.tar.gz
sha256sum -c SHA256SUMS.txt
```

Initial Windows builds may be unsigned and may trigger Microsoft SmartScreen.
Verify SHA256 checksums from the official release before running. If the
checksum matches the official release, Windows users may click `More Info`,
then `Run Anyway`.

Do not bypass warnings for unofficial mirrors, modified files, or mismatched
checksums.

## Build From Source on Windows

Use release binaries if you only want to run the wallet/node. Build from source
if you are reviewing the code, contributing, or creating your own build.

Requirements:

- Git for Windows
- Go 1.22 or newer
- Node.js LTS
- Wails v2
- MSYS2 / MinGW-w64 with `gcc`, `g++`, and `windres` for CGO/C yespower
  production builds

Clone:

```powershell
git clone https://github.com/legacybtc/LegacyCore.git
cd LegacyCore
```

Build wallet frontend first:

```powershell
cd cmd\legacywallet\frontend
npm install
npm run build
cd ..\..\..
```

Build/test:

```powershell
$env:CGO_ENABLED="1"
go test ./...
go vet ./...
go build -trimpath -o legacycoind.exe .\cmd\legacycoind
go build -trimpath -o legacycoin-cli.exe .\cmd\legacycoin-cli
go build -trimpath -o legacy-wallet-internal.exe .\cmd\legacywallet
```

Check params:

```powershell
.\legacycoind.exe params
```

Expected:

```text
yespower backend: cgo-c-reference
```

If it shows:

```text
yespower backend: pure-go-experimental
```

then the build is only for source sanity testing and not for public production
binaries.

Build Wails desktop wallet:

```powershell
cd cmd\legacywallet
wails build -platform windows/amd64 -skipbindings
```

## Build From Source on Linux

Dependencies:

```bash
sudo apt update
sudo apt install -y build-essential gcc g++ make git curl pkg-config ca-certificates nodejs npm
```

Clone:

```bash
git clone https://github.com/legacybtc/LegacyCore.git
cd LegacyCore
```

Build wallet frontend first:

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
cd ../../..
```

Build/test:

```bash
export CGO_ENABLED=1
go test ./...
go vet ./...
go build -trimpath -o legacycoind ./cmd/legacycoind
go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
go build -trimpath -o legacy-wallet-internal ./cmd/legacywallet
```

Check params:

```bash
./legacycoind params
```

Expected:

```text
yespower backend: cgo-c-reference
```

Note: Linux desktop GUI/Wails builds may require GTK/WebKit native dependencies
and are not the primary release path right now.

## Run Commands / Node Status

Common commands:

```bash
./legacycoind params
./legacycoind run -seed-peers
./legacycoin-cli getinfo
./legacycoin-cli getnetworkinfo
./legacycoin-cli getblockcount
./legacycoin-cli getpeerinfo
./legacycoin-cli getselfcheck
./legacycoin-cli getlaunchstatus
```

Useful status fields:

- height / blocks
- peers
- best block hash
- chain ID
- mempool size
- yespower backend

## RPC Cookie/Auth Troubleshooting

For normal same-machine use, Legacy Core uses cookie authentication.

Start node first:

```bash
./legacycoind run -seed-peers
```

When no `rpcuser` / `rpcpassword` is configured, `legacycoind` creates:

Linux:

```text
~/.legacycoin/.cookie
```

Windows:

```text
%APPDATA%\LegacyCoin\.cookie
```

Then CLI should work:

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getblockcount
```

Custom datadir:

```bash
./legacycoind run -datadir=/opt/legacycoin-data -seed-peers
./legacycoin-cli -datadir=/opt/legacycoin-data getnetworkinfo
```

Explicit auth in `legacycoin.conf`:

```ini
rpcuser=legacyrpc
rpcpassword=strong_password
rpcbind=127.0.0.1
rpcport=19556
```

CLI:

```bash
./legacycoin-cli -rpcuser=legacyrpc -rpcpassword=strong_password getnetworkinfo
```

Supported CLI flags:

```text
-datadir=
-rpcuser=
-rpcpassword=
-rpccookiefile=
-rpcconnect=
-rpcport=
-rpcurl=
```

Friendly errors:

- `RPC cookie not found`: start `legacycoind` first or configure
  `rpcuser`/`rpcpassword`.
- `RPC unauthorized`: check `rpcuser`/`rpcpassword` or `.cookie` file.

## Mainnet Parameters

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

## Security Warnings

Legacy Core RPC must remain local/private unless the operator has configured
proper authentication, TLS, and firewall rules. Do not expose wallet RPC methods
to the public internet.

Public binaries that validate blocks, mine blocks, validate pool shares, or
submit blocks must be built with CGO enabled and the bundled C yespower backend.
The pure-Go yespower path is experimental/debug only unless byte-for-byte parity
with the C reference path is proven.

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

User responsibility:

- Verify checksums.
- Keep private keys and wallet backups offline/secure.
- Do not run binaries from unofficial mirrors.
- Do not expose RPC port `19556` publicly.

See `SECURITY.md` for vulnerability reporting and release security checks.

## License

Legacy Core / Legacy Wallet is released under the MIT License. See `LICENSE`.
