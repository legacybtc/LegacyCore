# Legacy Coin / LBTC

Legacy Coin is a fair-launch UTXO Proof-of-Work network focused on full-node verification, CPU-accessible mining, and simple desktop wallet ownership.

Legacy Coin is launching in the early Bitcoin spirit: source first, full node first, local keys, CPU-accessible mining, and real users running real software.

---

## Fair Launch Statement

Legacy Coin / LBTC is intended as a public Proof-of-Work network with:

- no premine
- no ICO
- no private sale
- no dev tax
- no founder allocation
- no hidden insider mining window

---

## Quick Start

### Windows normal users

1. Go to GitHub Releases:

   <https://github.com/legacybtc/LegacyCore/releases>

2. Download:

   ```text
   LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.0.zip
   ```

3. Extract the ZIP.

4. Open the extracted folder.

5. Double-click:

   ```text
   START_HERE.bat
   ```

6. Legacy Wallet will start a local Legacy Core node automatically.

7. Wait for:

   - Mainnet status
   - synced / syncing status
   - connected peers

8. Go to **Receive** and create/copy a receive address.

9. Back up your wallet/private keys before using real funds.

### Linux users

The Linux Core/CLI binary package is coming soon.

For now, Linux users should build from source using the instructions below.

The stable Linux path right now is:

```bash
legacycoind + legacycoin-cli
```

Linux desktop GUI/Wails builds are experimental until packaged and tested across Linux distributions.

---

## What Should I Download?

| User type | Recommended path | What it does |
|---|---|---|
| Windows normal user | `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.0.zip` from Releases | Desktop wallet + local node |
| Linux node operator | Build from source for now | Full node + CLI |
| Linux miner | Build from source for now | Node + CLI mining/control path |
| Developer | GitHub source | Build and inspect source |
| Pool operator | Separate pool package/repo | Mining pool server |
| Explorer operator | Separate explorer package/repo | Explorer/index service |
| Launchpad user/operator | Separate Launchpad package/repo | Token/Launchpad app |

If you only want to use LBTC and you are on Windows, start with the Windows desktop wallet.

If you want to run infrastructure, run Legacy Core / CLI.

If you are unsure, start with the Windows desktop wallet.

---

## Windows Desktop Wallet: Download and Run

1. Download the official Windows ZIP from GitHub Releases:

   ```text
   LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.0.zip
   ```

2. Right-click ZIP -> **Extract All**.

3. Open the extracted folder.

4. Double-click:

   ```text
   START_HERE.bat
   ```

5. If Windows SmartScreen appears:

   - verify SHA256 first
   - click **More Info**
   - click **Run Anyway** only if the checksum matches the official GitHub release

6. Wallet should open and start the local node.

7. Check the wallet status area:

   - Network: Mainnet
   - Height
   - Peers
   - Sync status

8. Go to **Receive** and create/copy a receive address.

9. Back up wallet/private keys before using real funds.

Important:

- Do not delete wallet files, private keys, backups, or seed phrases.
- Do not run binaries from unofficial mirrors.
- Do not bypass SmartScreen warnings for modified or mismatched files.
- RPC port `19556` must not be exposed publicly.

---

## Linux Core / CLI: Build From Source and Run

### Dependencies

Debian/Ubuntu:

```bash
sudo apt update
sudo apt install -y build-essential gcc g++ make git curl pkg-config ca-certificates nodejs npm
```

Install Go 1.22+ if it is not already installed.

### Clone

```bash
git clone https://github.com/legacybtc/LegacyCore.git
cd LegacyCore
```

### Build wallet frontend first

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
cd ../../..
```

### Build and test Core/CLI

```bash
export CGO_ENABLED=1
go test ./...
go vet ./...
go build -trimpath -o legacycoind ./cmd/legacycoind
go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
```

### Verify params

```bash
./legacycoind params
```

Expected output must include:

```text
message start: a4 ac c6 4d
genesis hash: 5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5
yespower backend: cgo-c-reference
```

If it shows:

```text
yespower backend: pure-go-experimental
```

then that build is **not** a public production mining/validation build.

### Run node

```bash
./legacycoind run -seed-peers
```

Open a second terminal:

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getblockcount
./legacycoin-cli getpeerinfo
```

If you have 0 peers, try force-connect:

```bash
./legacycoind run -connect legacycoinseed.space:19555
./legacycoind run -connect legacycoinseed2.space:19555
```

### Firewall

If you can accept incoming P2P connections, open P2P port:

```bash
sudo ufw allow 19555/tcp
```

Never expose RPC publicly:

```bash
sudo ufw deny 19556/tcp
```

---

## Linux Desktop GUI Status

The Linux desktop GUI is not the primary release path yet.

Do **not** run the GUI as only `npm run` from the frontend folder and expect it to behave like the full desktop wallet. That only runs/builds the frontend UI. The real desktop wallet requires the Wails/Go backend bridge.

For experimental Linux GUI testing:

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
cd ../../..

export CGO_ENABLED=1
cd cmd/legacywallet
wails dev
```

Or build:

```bash
wails build -platform linux/amd64 -skipbindings
```

Linux Wails builds may require GTK/WebKit native dependencies. Until the Linux GUI is officially packaged and tested, use Core/CLI on Linux.

---

## Verify SHA256 Checksums

Only run release binaries if checksums match the official GitHub Release.

Windows PowerShell:

```powershell
Get-FileHash .\LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.0-rc2.zip -Algorithm SHA256
```

Linux:

```bash
sha256sum <downloaded-file>
sha256sum -c SHA256SUMS.txt
```

Initial Windows builds may be unsigned and may trigger Microsoft SmartScreen. If the checksum matches the official release, Windows users may click:

```text
More Info -> Run Anyway
```

Do not bypass warnings for unofficial mirrors, modified files, or mismatched checksums.

---

## Solo Mining FAQ

### Is mining solo or pool?

Default wallet/node mining is solo mining unless you are using a working pool.

### Why is my balance still 0?

Solo mining has no partial rewards. Your balance stays 0 until your machine actually finds a valid block.

### Does hashrate guarantee a block?

No. Proof-of-Work is probability, not a timer. More hashrate increases your chance, but it does not guarantee a reward in a specific hour or day.

### When can I spend mined coins?

Coinbase rewards mature after 100 blocks.

### Can multiple machines mine to the same address?

Yes. Multiple machines can mine to the same receive address. That increases total work/chance, but it is still solo mining unless you use a pool. Same address means same payout destination; it does not create shared partial rewards.

### What does 0 peers mean?

If your node has 0 peers, it is isolated. Mining with 0 peers will not help the public chain. Connect to peers first.

---

## Run Commands / Node Status

Common commands:

```bash
./legacycoind params
./legacycoind run -seed-peers
./legacycoin-cli getinfo
./legacycoin-cli getnetworkinfo
./legacycoin-cli getblockcount
./legacycoin-cli getbestblockhash
./legacycoin-cli getpeerinfo
./legacycoin-cli getselfcheck
./legacycoin-cli getlaunchstatus
./legacycoin-cli getsyncstatus
```

Useful status fields:

- height / blocks
- peers
- best block hash
- chain ID
- mempool size
- mining status
- yespower backend
- sync status

---

## RPC Cookie/Auth Troubleshooting

For normal same-machine use, Legacy Core uses cookie authentication.

Start node first:

```bash
./legacycoind run -seed-peers
```

When no `rpcuser` / `rpcpassword` is configured, `legacycoind` creates an RPC cookie in the active data directory.

Linux:

```text
~/.legacycoin/.cookie
```

Windows:

```text
%APPDATA%\LegacyCoin\.cookie
```

Custom data directory:

```text
<datadir>/.cookie
```

Then CLI should work from another terminal on the same machine:

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

- `RPC cookie not found`: start `legacycoind` first or configure `rpcuser`/`rpcpassword`.
- `RPC unauthorized`: check `rpcuser`/`rpcpassword` or `.cookie` file.

RPC safety:

- Keep RPC bound to `127.0.0.1`.
- Do not expose RPC port `19556` publicly.
- Public peers and miners should use P2P/pool endpoints, not wallet RPC.

---

## Stuck Sync / Safe Resync

If the wallet shows something like:

```text
Node is behind peers
Sync: 340 / 349
```

first give it time to request blocks. If it stays stuck, perform a safe resync.

### Windows safe resync

1. Close Legacy Wallet / `legacycoind`.

2. Back up your data folder first.

   PowerShell:

   ```powershell
   Copy-Item "$env:APPDATA\LegacyCoin" "$env:USERPROFILE\Desktop\LegacyCoin-backup-before-resync" -Recurse
   ```

3. Open:

   ```powershell
   explorer "$env:APPDATA\LegacyCoin"
   ```

4. Delete only runtime chain/network data if present:

   ```text
   blocks
   index
   undo
   utxo
   chainstate
   .cookie
   peers.dat
   mempool
   mempool.dat
   *.log
   ```

5. Keep wallet/private data:

   ```text
   backups
   wallet
   wallet-transactions
   legacycoin.conf
   wallet.dat
   wallet.db
   wallet.json
   keys
   seed
   ```

6. Restart the wallet and let it resync.

Do not delete wallet files, backups, private keys, or seed phrases.

A future sync fix will improve missing-parent/orphan recovery so users should not normally need to wipe runtime chain data.

---

## Mainnet Parameters

| Field | Value |
|---|---|
| Coin | Legacy Coin |
| Ticker | LBTC |
| Core | Legacy Core 1.0.0 |
| Wallet | Legacy Wallet 1.0.0 |
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
| Difficulty | Dark Gravity Wave v3 |
| Target spacing | 10 minutes |
| Initial subsidy | 50 LBTC |
| Halving interval | 210,000 blocks |
| Max supply target | 21,000,000 LBTC |
| Coinbase maturity | 100 blocks |
| P2PKH version | 48 |
| WIF version | 176 |
| DNS seeds | `legacycoinseed.space`, `legacycoinseed2.space` |

---

## Production PoW Requirement

Public RC2 binaries that validate blocks, mine blocks, validate pool shares, or submit blocks must be built with CGO enabled and the bundled C yespower backend.

Expected verification output:

```text
yespower backend: cgo-c-reference
```

The pure-Go yespower path is experimental/debug only unless byte-for-byte parity with the C reference path is proven.

Do not publish production mining, pool, or submitblock-capable binaries that report the pure-Go backend.

---

## Security Warnings

Legacy Core RPC must remain local/private unless the operator has configured proper authentication, TLS, and firewall rules.

Do not expose wallet RPC methods to the public internet.

If you tested earlier builds, back up wallets, wallet backups, private keys, and seed phrases before changing any data.

User responsibility:

- verify checksums
- keep private keys and wallet backups offline/secure
- do not run binaries from unofficial mirrors
- do not expose RPC port `19556` publicly
- do not mix old pre-RC2 chain data with this release

See `SECURITY.md` for vulnerability reporting and release security checks.

---

## License

Legacy Core / Legacy Wallet is released under the MIT License. See `LICENSE`.
