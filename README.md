# Legacy Core

[![Go](https://github.com/legacybtc/LegacyCore/actions/workflows/ci.yml/badge.svg)](https://github.com/legacybtc/LegacyCore/actions/workflows/ci.yml)
[![CodeQL](https://github.com/legacybtc/LegacyCore/actions/workflows/codeql.yml/badge.svg)](https://github.com/legacybtc/LegacyCore/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/legacybtc/LegacyCore/badge)](https://securityscorecards.dev/viewer/?uri=github.com/legacybtc/LegacyCore)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/13444/badge)](https://www.bestpractices.dev/projects/13444)

Legacy Core is the official full-node, CLI, miner, and desktop wallet for **Legacy Coin (LBTC)** — a CPU-friendly Yespower Proof-of-Work cryptocurrency.

---

## Quick Start (Pre-built)

Download the latest release from [GitHub Releases](https://github.com/legacybtc/LegacyCore/releases).

**Windows:**
```
Extract the ZIP → double-click START_HERE.bat → wallet opens
```
Or run the node manually:
```
legacycoind.exe
legacycoin-cli getblockchaininfo
```

**Linux:**
```bash
tar -xzf LegacyCore-LBTC-mainnet-linux-amd64-*.tar.gz
chmod +x legacycoind legacycoin-cli
./legacycoind
./legacycoin-cli getblockchaininfo
```

**macOS:**
```bash
tar -xzf LegacyCore-LBTC-mainnet-macos-amd64-*.tar.gz
chmod +x legacycoind legacycoin-cli
./legacycoind
./legacycoin-cli getblockchaininfo
```

> **Verify integrity:** Compare SHA256 checksums against `SHA256SUMS.txt` before running.

---

## Build From Source

```bash
git clone https://github.com/legacybtc/LegacyCore.git
cd LegacyCore
```

### Windows

**Prerequisites:** Go 1.22+, Node.js LTS, MSYS2 UCRT64 with GCC.

1. Install [MSYS2](https://www.msys2.org/), open **MSYS2 UCRT64**, run:
   ```
   pacman -S --needed mingw-w64-ucrt-x86_64-gcc
   ```
2. Open **Command Prompt** in the repo folder, run:
   ```
   build-windows.bat
   ```

Produces: `legacycoind.exe`, `legacycoin-cli.exe`, `cmd\legacywallet\build\bin\LegacyWallet.exe`

To skip the desktop wallet:
```
build-windows.bat -SkipWails
```

### Linux

**Prerequisites:** `gcc`, `golang-go`, `git`

```bash
sudo bash scripts/build-linux.sh
```

Also supports cross-compilation:
```bash
# Build for ARM64
sudo bash scripts/build-linux.sh arm64

# Build with custom output dir
sudo bash scripts/build-linux.sh amd64 ./my-binaries
```

Produces: `legacycoind`, `legacycoin-cli`

### macOS

**Prerequisites:** Xcode Command Line Tools (`clang`), Go 1.22+

```bash
bash scripts/package-macos.sh v1.0.33 amd64
```

For Apple Silicon (ARM64):
```bash
bash scripts/package-macos.sh v1.0.33 arm64
```

Produces: `dist/LegacyCore-LBTC-mainnet-macos-*.tar.gz`

---

## Makefile (all platforms)

```bash
make build          # daemon + CLI + internal wallet
make test           # run Go tests
make vet            # run Go vet
make clean          # remove build artifacts
make linux-arm64    # cross-compile daemon + CLI for ARM64
```

---

## Server Deployment (Linux)

One-command update for production nodes:

```bash
sudo bash scripts/server/update-node.sh
```

Backs up current binaries, builds from source, verifies mainnet identity, stops old daemon, installs new binary, restarts, and verifies RPC is alive. Generates a rollback script and build evidence.

---

## Seed Nodes

Connect to the Legacy Coin network by specifying seed peers on first run:

```bash
legacycoind run -seed-peers
```

The daemon will discover peers from DNS seeds (`legacycoinseed.space`, `legacycoinseed2.space`) and maintain a local peer database. You can also manually add known nodes:

```bash
legacycoin-cli addnode 192.168.1.131:19555 add
```

> **Note:** Legacy Core uses **Yespower Proof-of-Work**. Old SHA256d peers (v1.0.20–v1.0.30) are on a different chain — the daemon will ignore their headers (expected) and sync blocks from them via the INV flow. Sync reaches chain tip reliably.

---

## Mainnet Identity

| Field | Value |
|---|---|
| Coin | Legacy Coin / LBTC |
| Genesis hash | `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5` |
| P2P port | `19555` |
| RPC port | `19556` |
| DNS seeds | `legacycoinseed.space`, `legacycoinseed2.space` |
| yespower | `LegacyCoinPoW` (cgo-c-reference) |
| Data dir | `%APPDATA%\LegacyCoin` (Windows) / `~/.legacycoin` (Linux/macOS) |

Verify any build:
```
legacycoind params
```

---

## Mining

Solo CPU mining from CLI:
```bash
legacycoin-cli setminerthreads 4
legacycoin-cli startminer
legacycoin-cli getminerstatus
```

Or use the **Mining tab** in the desktop wallet.

---

## Legacy AI Companion (Built-in)

The desktop wallet includes a **Legacy AI Workstation** — accessible from the **AI** tab in the sidebar:

- **Chat** — Ask about your node, sync, mining, peers, balance, or storage
- **Image Gen** — Free AI image generation (Pollinations.ai, no API key)
- **Code Agent** — Execute allowlisted CLI tools with `/` commands
- **Research** — DuckDuckGo web search with AI analysis
- **Provider switching** — Built-in AI (offline) / Groq (free cloud) / llama.cpp (local GPU)

No Python, no LangChain, no cloud API required. Privacy-first — wallet secrets are never exposed to AI.

---

## Security

- Keep RPC port (`19556`) private — never expose to the internet
- P2P port (`19555`) may be public
- Never share wallet backups, private keys, seed phrases, or RPC credentials
- Verify SHA256 checksums before running binaries
- Legacy AI is **read-only advisory** — it cannot spend coins, sign transactions, or control the node

---

## Repository Structure

| Directory | Purpose |
|---|---|
| `cmd/legacycoind/` | Full node daemon |
| `cmd/legacycoin-cli/` | RPC command-line client |
| `cmd/legacywallet/` | Wails desktop wallet (Go + React) |
| `internal/ai/` | Legacy AI Companion (providers, tools, web search) |
| `internal/p2p/` | Peer-to-peer networking |
| `internal/rpc/` | JSON-RPC server |
| `internal/mining/` | Solo CPU miner |
| `internal/wallet/` | Wallet operations |
| `internal/blockchain/` | Chain state and validation |
| `scripts/` | Build, deploy, smoke test, and verification scripts |

---

## Docs

- [Mining](docs/MINING.md)
- [Wallet Backup & Restore](docs/WALLET_BACKUP_AND_RESTORE.md)
- [Troubleshooting](docs/TROUBLESHOOTING.md)
