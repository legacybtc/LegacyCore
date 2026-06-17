# Legacy Core

Legacy Core is the official full-node, CLI, miner, and desktop wallet for **Legacy Coin (LBTC)** — a CPU-friendly Yespower Proof-of-Work cryptocurrency.

---

## Quick Start (Pre-built)

Download the latest release from [GitHub Releases](https://github.com/legacybtc/LegacyCore/releases).

**Windows:** Extract the ZIP, double-click `LegacyWallet.exe`.

**Linux:** Extract the tarball, then:
```bash
chmod +x legacycoind legacycoin-cli
./legacycoind
```

---

## Build From Source (One Command)

### Windows

Requirements: **MSYS2 UCRT64** with GCC (one-time, 5 minutes).

1. Download and install MSYS2 from https://www.msys2.org/
2. Open **MSYS2 UCRT64** from the Start Menu, run:
   ```
   pacman -S --needed mingw-w64-ucrt-x86_64-gcc
   ```
3. Download the source ZIP from GitHub, extract it
4. Open Command Prompt in the extracted folder, run:
   ```
   build-windows
   ```

Produces: `legacycoind.exe`, `legacycoin-cli.exe`, `LegacyWallet.exe`

### Linux

Requirements: **gcc, golang-go, git**

```bash
apt update && apt install -y gcc golang-go git
```

Download the source ZIP, extract, then:
```bash
bash build-linux.sh
```

Produces: `legacycoind`, `legacycoin-cli`

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
| Data dir | `%APPDATA%\LegacyCoin` (Windows) / `~/.legacycoin` (Linux) |

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

No Python, no LangChain, no cloud API required. Everything runs in the wallet. Privacy-first — wallet secrets are never exposed to AI.

---

## Server Deployment (Linux)

```bash
sudo bash scripts/server/update-node.sh
```

Backs up the current node, builds from source, verifies identity, stops old daemon, installs new binary, restarts, and verifies RPC is alive.

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

## Docs Index

See [docs/README.md](docs/README.md) for organized documentation by audience.
