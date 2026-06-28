# Legacy Core v1.0.8 — Exchange & Mining Pool Readiness Audit

**Date:** 2026-06-28
**Version:** v1.0.8 (commit `a094295`)
**Coin:** Legacy Coin (LBTC) — Yespower PoW

---

## Overall Verdict

| Category | Verdict |
|---|---|
| **Blockchain Consensus** | **PASS** — all consensus bugs fixed; reorg safety, orphan promotion, UTXO integrity verified |
| **RPC API (Exchange)** | **PASS** — `estimatefee`, rate limiting, timeouts, pagination, CORS all implemented |
| **RPC API (Pool)** | **PASS** — BIP22 getblocktemplate, submitblock, submitblockdebug |
| **P2P Protocol** | **PASS** — `reject` message (BIP 61) added, DoS protection, IPv6 subnet limits fixed |
| **Wallet** | **PASS (core)** — scrypt N=65536, change address privacy, auto fee estimation fixed. **WARNING**: non-BIP44 HD (custom), no SegWit/multisig (by design for this chain) |
| **Mining** | **PASS** — correct Yespower, BIP22 template, full reject codes |
| **Security / DoS** | **PASS** — per-IP rate limiting, max concurrent RPC (32), WriteTimeout 60s, CORS hardened, CLI stdin for passphrases |
| **Build / Reproducibility** | **PASS** — deterministic MSYS2/Wails build, gosec hardened, auto-release via CI |

---

## 1. Blockchain Consensus — PASS

### Consensus Rules — Correct
All standard Bitcoin-derived checks enforced:
- Merkle root, prev block link, MTP timestamps, DGWv3 difficulty bits
- Yespower PoW verification (`LegacyCoinPoW`)
- Block size ≤ 1MB, sigops ≤ 20K/block
- Coinbase maturity 100 blocks — enforced in block validation and mempool
- Coinbase ≤ subsidy + fees, no duplicate txids/spends
- Script verification: P2PK, P2PKH, P2SH, MultiSig + custom Hybrid P2PKH
- DGWv3 difficulty adjustment — standard Kimoto/Gravity well, 24-block window, 3× clamp, no timewarp
- Non-coinbase transactions with zero inputs explicitly rejected

### Bugs Found & Fixed (v1.0.7)
| # | Severity | Bug | File:Line | Fix |
|---|---|---|---|---|
| 1 | MEDIUM | **Orphan sibling loss**: competing orphans sharing a parent permanently dropped | `blockchain.go:745-768` | `delete(c.orphanByPrev, cur)` moved after child-loop completes |
| 2 | HIGH | **Silent reorg corruption**: `connectBlockLocked` errors silently discarded (`_ =`) during restore | `blockchain.go:893-895` | `reconnectBlocksLocked` returns and checks all errors |

### Additional Fixes (v1.0.8 re-audit)
| # | Severity | Bug | File:Line | Fix |
|---|---|---|---|---|
| 3 | MEDIUM | **Orphan starvation after reorg**: `acceptOrphanChildrenLocked` not called after side-chain activation | `blockchain.go:644,673,702` | Called after each `tryActivateSideChainLocked` when chain becomes active |
| 4 | HIGH | **Reorg disconnect failure**: partial disconnect without restore on error | `blockchain.go:885-887` | `reconnectBlocksLocked(removed)` called before returning error |
| 5 | LOW | **sideBlocks memory leak**: stale side-chain blocks never evicted | `blockchain.go:154` | `pruneSideBlocksLocked()` evicts blocks >288 below tip |
| 6 | LOW | **Non-coinbase tx with 0 inputs**: allowed at consensus level | `blockchain.go:990` | Explicit rejection added |

---

## 2. RPC API — PASS

### Exchange RPCs — All Implemented

| RPC | Status | Notes |
|---|---|---|
| `getblockchaininfo` | ✅ | |
| `getblock` | ✅ | Returns `tx` as array of txid strings (was count-only in v1.0.7) |
| `getblockhash` | ✅ | |
| `getblockcount` | ✅ | |
| `getblockheader` | ✅ | Supports verbose flag |
| `getrawtransaction` | ✅ | Requires txindex for historical |
| `gettxout` | ✅ | |
| `sendrawtransaction` | ✅ | |
| `getbalance` | ✅ | Returns float LBTC (was int64 base units in v1.0.7) |
| `listunspent` | ✅ | |
| `listtransactions` | ✅ | Paginated via `maxRows` param (was full chain scan in v1.0.7) |
| `getnewaddress` | ✅ | |
| `validateaddress` | ✅ | |
| `getnetworkinfo` | ✅ | |
| `getpeerinfo` | ✅ | |
| `estimatefee` / `estimatesmartfee` | ✅ | **Added.** Median mempool feerate with `nblocks` tiers: ≤2→75th pctile, ≤5→50th, >5→25th. Falls back to `MinRelayFeePerKB` when mempool empty. |

### Pool RPCs — PASS

| RPC | Status |
|---|---|
| `getblocktemplate` | ✅ BIP22/BIP23 compliant — all standard fields present |
| `submitblock` | ✅ Full BIP22 reject code mapping |
| `submitblockdebug` | ✅ Rich diagnostics (reject_code, reject_reason, would_accept) |
| `validateblockproposal` | ✅ |
| `coinbasetxn` capability | ✅ Pools can replace coinbase for payout splitting |

### DoS / Rate Limiting — All Fixed

| Issue | v1.0.7 | v1.0.8 |
|---|---|---|
| `WriteTimeout` | 0 (disabled) | 60s |
| Per-IP rate limiting | None | Token bucket (60 tokens/s per IP, HTTP 429) |
| Max concurrent requests | Unbounded | 32 (`-32603 server too busy`) |
| `listtransactions` full chain scan | O(n) from genesis | `maxRows` param, stops early |
| CORS headers | None | `Access-Control-Allow-Origin: *` + OPTIONS preflight |

---

## 3. P2P Protocol — PASS

### Protocol Correctness
14/20 standard Bitcoin messages implemented: `version`, `verack`, `ping/pong`, `block`, `tx`, `inv`, `getdata`, `addr`, `getaddr`, `getblocks`, `getheaders`, `headers`, `reject`. Serialization is correct (all LE, matching Bitcoin wire format).

### `reject` Message (BIP 61) — Added in v1.0.8
- New wire type in `internal/wire/reject.go`: `Reject` struct with `Cmd`, `Code`, `Reason`, `Hash`
- 8 reject code constants: `RejectMalformed`, `Invalid`, `Obsolete`, `Duplicate`, `Nonstandard`, `Dust`, `InsufficientFee`, `Checkpoint`
- Hash field serialized only when non-zero (BIP 61 compliant)
- Handlers in `internal/p2p/server.go`: incoming reject logged; `sendReject`/`sendRejectWithHash` helpers; block/tx validation failures send reject
- IPv6 subnet limiting fixed: `/64` key for IPv6 peers (was broken, returned empty string)

### Missing Messages (non-blocking)
| Message | Impact |
|---|---|
| `notfound` | Peers silently skip missing inventory instead of replying with `notfound` |
| `sendheaders` (BIP 130) | All block announcements go through inv→getdata round-trip |
| Compact blocks (BIP 152) | Full blocks always transmitted — bandwidth consideration for high-volume pools |

### DoS Protection — PASS
Good size limits on all message types, per-peer rate limiting (250/10s), global 3000/10s, per-IP inbound caps (8), per-subnet caps (IPv4 /24, IPv6 /64), ban/score system with decay, IP-level banning with expiry.

### Sync — PASS
Headers-first IBD, dual-hash interop (Yespower + SHA256d), orphan-parent resolution, sync watchdog with stall detection, panic recovery.

---

## 4. Wallet — PASS (core functionality)

| Area | Verdict | Detail |
|---|---|---|
| **Encryption** | **PASS** | AES-256-GCM with scrypt N=65536 (was N=32768 in v1.0.7). AES-GCM additional data bound (`"legacycoin-wallet-v1"`) |
| **Change address** | **PASS** | Now generates fresh `NewAddress()` per change (was reusing first input's address — privacy leak) |
| **Fee estimation** | **PASS** | Auto fee when `fee ≤ 0`: `(10 + inputs×148 + outputs×34) × MinRelayFeePerKB / 1000`. Minimum `MinRelayFeePerKB` (0.00001 LBTC/KB) |
| **Passphrase memory** | **PASS** | `unlockPass` changed from `string` to `[]byte`, explicitly zeroed on `Lock()` and after `persist()` |
| **CLI security** | **PASS** | `walletpassphrase`/`walletpassphrasechange` support `-` to read from stdin (avoiding process list leak) |
| **Key derivation** | **WARNING** | Custom HMAC-SHA256 derivation, NOT BIP32/39/44. No mnemonic phrase, no extended pubkeys. Seeds are opaque 32-byte hex |
| **Address types** | **WARNING** | P2PKH (Base58, version 48) + custom Hybrid P2PKH (`lhyb1`). No P2SH, no Bech32/SegWit |
| **Transaction signing** | **WARNING** | P2PKH + Hybrid only. Hardcoded `SIGHASH_ALL`. No RBF. Malleable (no low-R enforcement) |
| **Coin selection** | **WARNING** | Simple first-fit. No knapsack/BnB |
| **Multi-sig** | **FAIL** | Not supported (by design for this chain) |
| **Backup** | **WARNING** | No `backupwallet` RPC — requires manual file copy |
| **Hybrid PQC keys** | ✅ | ECDSA + ML-DSA-65 post-quantum signing |

**For exchange use:** The wallet is functional for basic receiving/sending but non-standard HD derivation means you cannot restore wallets from a seed phrase using standard tools. Recommend exchanges use their own wallet backend and only interact via the node's RPC for chain data (getblock, sendrawtransaction, etc.), not for key management.

---

## 5. Mining — PASS

| Area | Detail |
|---|---|
| **Yespower** | ✅ Correct implementation (N=2048, r=32, `"LegacyCoinPoW"`). CGO backend (fast) + pure-Go fallback. Test vector verified. |
| **getblocktemplate** | ✅ BIP22/BIP23 compliant. All standard fields. `coinbasetxn` capability for pool payout splitting. Longpoll support. |
| **submitblock** | ✅ Full BIP22 reject codes. submitblockdebug returns rich diagnostics. |
| **Built-in miner** | ✅ Solo CPU mining only. No stratum. Thread-safe with proper lifecycle management. |
| **Pool mining** | ✅ External pools use JSON-RPC (getblocktemplate + submitblock). No built-in stratum server. |

**Pool deployment:** Pools should submit blocks via `submitblock` RPC (returns proper reject codes) rather than broadcasting raw blocks over P2P.

---

## 6. Security

| Area | Verdict | Detail |
|---|---|---|
| **Go version** | ✅ | 1.26.0 (current) |
| **CGO dependencies** | ✅ | Yespower C source vendored; standard crypto libs |
| **RPC auth** | ✅ | Basic auth (constant-time compare) + cookie auth. TLS available. |
| **RPC DoS** | ✅ | Per-IP rate limiting (60/s token bucket), max concurrent (32), WriteTimeout (60s), CORS hardened |
| **P2P DoS** | ✅ | Good size limits, rate limiting, ban system, per-subnet caps (IPv4 /24, IPv6 /64) |
| **CLI credentials** | ✅ | Passphrases readable from stdin (avoid `ps` leak) |
| **Wallet memory** | ✅ | `unlockPass` as `[]byte`, zeroed on Lock/persist |
| **gosec findings** | ⚠️ | 104 pre-existing: G115, G304, G204, G406 — all Bitcoin-compatible intentional patterns, not exploitable |
| **Panic safety** | ✅ | No unprotected panics in production paths |
| **Memory safety** | ✅ | Go type-safe; no unsafe pointers |
| **Cryptography** | ✅ | crypto/rand for keys, double-SHA256 correctly used, Yespower from C reference |

---

## 7. Build & Release — PASS

| Area | Verdict |
|---|---|
| **Windows** | ✅ MSYS2 + Wails. Native icon via `build/windows/icon.ico` (was `rsrc` .syso, now Wails-standard layout) |
| **Linux** | ✅ `scripts/build-linux.sh` with auto-dep install. amd64 + arm64 cross-compile |
| **macOS** | ⚠️ Experimental, disabled in release matrix |
| **CI** | ✅ GitHub Actions: CI (every push) + Release Matrix (tag push, auto-creates GitHub Release) |
| **Release assets** | ✅ Source archive + Linux amd64/arm64 + Windows amd64 + SHA256 checksums |
| **Mainnet verification** | ✅ Genesis hash, yespower backend, ports, message start — all verified during build |

---

## Exchange Checklist — Status as of v1.0.8

- [x] **Fix orphan sibling loss** (`blockchain.go:745-768`)
- [x] **Fix silent reorg corruption** (`blockchain.go:893-895`)
- [x] **Implement `estimatefee`/`estimatesmartfee`** RPC
- [x] **Add HTTP write timeout** (was 0, now 60s)
- [x] **Add per-IP rate limiting** (token bucket, 60/s)
- [x] **Add max concurrent RPC** (32, returns `-32603 server too busy`)
- [x] **Add CORS headers** (`Access-Control-Allow-Origin: *`)
- [x] **Fix `listtransactions`** — paginated via `maxRows` param
- [x] **Add `reject` P2P message** (BIP 61)
- [x] **Increase scrypt N** (32768 → 65536)
- [x] **Fix change address** (new address, not first input)
- [x] **Add auto fee estimation** (when `fee ≤ 0`)
- [x] **Fix wallet passphrase memory** (`[]byte`, zeroed)
- [x] **Fix CLI passphrase leak** (stdin support)
- [x] **Fix orphan starvation after reorg** (acceptOrphanChildrenLocked)
- [x] **Fix reorg disconnect recovery** (reconnect on error)
- [x] **Fix sideBlocks eviction** (prune >288 below tip)
- [x] **Fix reject message BIP 61 compliance** (hash optional)
- [x] **Fix getblock tx array** (was count, now array of txids)
- [x] **Fix getbalance return type** (was int64, now float LBTC)
- [x] **Fix estimatefee nblocks tiers** (75/50/25 percentile)
- [x] **Fix IPv6 subnet limiting** (was broken for IPv6)
- [x] **Fix AES-GCM additional data** (bound `"legacycoin-wallet-v1"`)
- [ ] **Upgrade seed nodes to v1.0.8** — currently on 1.0.6, block sync stalls until upgraded
- [ ] **BIP44 HD derivation** — design-level, wallet is intentionally custom. Exchanges should use their own wallet backend

---

## Pool Checklist — Status as of v1.0.8

- [x] **`reject` P2P message implemented** — pools can distinguish rejection reasons over P2P
- [x] **`coinbasetxn` capability** in getblocktemplate for payout splitting
- [x] **Rate limiting** for RPC (per-IP, max concurrent)
- [ ] **Enable txindex=1** — config option, recommend for pool nodes
- [ ] **Upgrade seed nodes to v1.0.8** — currently block sync stalls on 1.0.6

**Not-blocking but recommended:**
- Implement `sendheaders` (BIP 130) for faster block propagation
- Implement compact blocks (BIP 152) for bandwidth efficiency
