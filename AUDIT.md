# Legacy Core v1.0.7 — Exchange & Mining Pool Readiness Audit

**Date:** 2026-06-27
**Version:** v1.0.7 (commit `5835ee5`)
**Coin:** Legacy Coin (LBTC) — Yespower PoW

---

## Overall Verdict

| Category | Verdict |
|---|---|
| **Blockchain Consensus** | **PASS** (conditional — fix 2 bugs before exchange listing) |
| **RPC API (Exchange)** | **FAIL** — missing `estimatefee`, no rate limiting, full-chain scans |
| **RPC API (Pool)** | **PASS** — BIP22 getblocktemplate, submitblock, submitblockdebug |
| **P2P Protocol** | **FAIL (pool)** — missing `reject` message; **PASS (exchange)** |
| **Wallet** | **FAIL** — custom (non-BIP44) HD derivation, no SegWit, no multisig, no RBF |
| **Mining** | **PASS** — correct Yespower, BIP22 template, full reject codes |
| **Security / DoS** | **WARNING** — no RPC rate limiting, no cookie auth, basic auth only |
| **Build / Reproducibility** | **PASS** — deterministic MSYS2/Wails build, gosec hardened |

---

## 1. Blockchain Consensus (PASS — fix 2 bugs)

### Consensus Rules — Correct
All standard Bitcoin-derived checks enforced:
- Merkle root, prev block link, MTP timestamps, DGWv3 difficulty bits
- Yespower PoW verification (`LegacyCoinPoW`)
- Block size ≤ 1MB, sigops ≤ 20K/block
- Coinbase maturity 100 blocks — enforced in block validation and mempool
- Coinbase ≤ subsidy + fees, no duplicate txids/spends
- Script verification: P2PK, P2PKH, P2SH, MultiSig + custom Hybrid P2PKH
- DGWv3 difficulty adjustment — standard Kimoto/Gravity well, 24-block window, 3× clamp, no timewarp

### BUGS TO FIX Before Exchange Listing

| # | Severity | Bug | File:Line |
|---|---|---|---|
| 1 | **MEDIUM** | **Orphan sibling loss**: when multiple orphans share the same parent (competing blocks), all but one are permanently dropped — the orphan table entry is deleted before the second child is processed, so it can never be promoted even in a reorg | `internal/blockchain/blockchain.go:745-768` |
| 2 | **HIGH** | **Silent reorg corruption**: if a reorg fails and the code tries to restore the old chain, `connectBlockLocked` errors are silently discarded (`_ =`). Chain state becomes silently corrupted — blocks disconnected but not reconnected | `internal/blockchain/blockchain.go:893-895` |

**Recommended fixes:**
1. Fix orphan sibling loss by not deleting `orphanByPrev[parent]` until all children are attempted
2. Log/return reconnect errors instead of discarding them; panic-safe restart on corruption

---

## 2. RPC API

### Exchange RPCs — FAIL

| RPC | Status |
|---|---|
| `getblockchaininfo` | ✅ |
| `getblock` (by hash) | ✅ (no height lookup or verbose flag) |
| `getblockhash` | ✅ |
| `getblockcount` | ✅ |
| `getrawtransaction` | ✅ (requires txindex for historical) |
| `gettxout` | ✅ |
| `sendrawtransaction` | ✅ |
| `getbalance` | ✅ (returns int64 base units, not LBTC float) |
| `listunspent` | ✅ |
| `listtransactions` | ✅ (but full chain scan O(n) — hangs on mainnet) |
| `getnewaddress` | ✅ |
| `validateaddress` | ✅ |
| `getnetworkinfo` | ✅ |
| `getpeerinfo` | ✅ |
| **`estimatefee` / `estimatesmartfee`** | ❌ **MISSING** — critical for exchanges |

**CRITICAL:** `estimatefee` and `estimatesmartfee` are completely missing. The only fee API is `settxfee` (static manual fee). Exchanges cannot dynamically estimate transaction fees.

### Pool RPCs — PASS

| RPC | Status |
|---|---|
| `getblocktemplate` | ✅ BIP22/BIP23 compliant — all standard fields present |
| `submitblock` | ✅ Full BIP22 reject code mapping |
| `submitblockdebug` | ✅ Rich diagnostics (reject_code, reject_reason, would_accept) |
| `validateblockproposal` | ✅ |
| `coinbasetxn` capability | ✅ Pools can replace coinbase for payout splitting |

### DoS / Rate Limiting — FAIL

| Issue | Detail |
|---|---|
| `WriteTimeout: 0` | HTTP write timeout disabled — slowloris DoS possible |
| No per-IP rate limiting | Any client can flood unlimited requests |
| No max concurrent requests | Unbounded goroutine creation |
| `listtransactions` full chain scan | Iterates every block from tip to genesis — blocks RPC for minutes, OOM on mainnet |
| `getrawtransaction` (no txindex) | Same full chain fallback scan |
| `generate` CPU exhaustion | No rate limit on CPU-heavy mining simulation |
| Missing `reject` message | P2P protocol — peers get disconnected silently with no reason code |

**Exchange deployment MUST:**
1. Add `estimatefee`/`estimatesmartfee`
2. Add HTTP write timeout and per-IP rate limiting
3. Fix `listtransactions` to use txindex or pagination
4. Enable txindex by default for exchange nodes

---

## 3. P2P Protocol — PASS (with caveats)

### Protocol Correctness
13/20 standard Bitcoin messages implemented: `version`, `verack`, `ping/pong`, `block`, `tx`, `inv`, `getdata`, `addr`, `getaddr`, `getblocks`, `getheaders`, `headers`. Serialization is correct (all LE, matching Bitcoin wire format).

### MISSING Messages (pool impact)
| Message | Impact |
|---|---|
| **`reject`** (BIP 61) | **Critical for pools** — when a submitted block is rejected, the miner gets no reason code. Cannot distinguish "bad PoW" from "temporary fork". |
| `notfound` | Peers silently skip missing inventory instead of replying with `notfound` |
| `sendheaders` (BIP 130) | All block announcements go through inv→getdata round-trip (~200ms latency) |
| Compact blocks (BIP 152) | Full blocks always transmitted — bandwidth-heavy for pools |

### DoS Protection — PASS
Good size limits on all message types, per-peer rate limiting (250/10s), global 3000/10s, per-IP inbound caps (8), ban/score system with decay, IP-level banning with expiry.

### Sync — PASS
Headers-first IBD, dual-hash interop (Yespower + SHA256d), orphan-parent resolution, sync watchdog with stall detection, panic recovery. Missing parallel block download (single-peer getdata at a time).

---

## 4. Wallet — FAIL (non-standard HD)

| Area | Verdict | Detail |
|---|---|---|
| **Key derivation** | **FAIL** | Custom HMAC-SHA256 derivation, NOT BIP32/39/44. No mnemonic phrase, no extended pubkeys, no chain code. Seeds are opaque 32-byte hex. |
| **Address types** | **WARNING** | P2PKH (Base58, version 48) + custom Hybrid P2PKH (`lhyb1`). No P2SH, no Bech32/SegWit. |
| **Transaction signing** | **WARNING** | P2PKH + Hybrid only. Hardcoded `SIGHASH_ALL` (no `SIGHASH_NONE/SINGLE/ANYONECANPAY`). No RBF (sequence=`0xffffffff`). Malleable (no low-R enforcement). |
| **Coin selection** | **WARNING** | Simple first-fit. No knapsack/BnB. Change always goes to first input's address (privacy leak/no change address). |
| **Fee estimation** | **FAIL** | Caller-provided only. No automatic fee estimation, no fee-bumping. |
| **Encryption** | **PASS** | AES-256-GCM, scrypt N=32768 (adequate but below Bitcoin's N=262144). |
| **Backup** | **WARNING** | No `backupwallet` RPC — requires manual file copy. Encrypted backup restore explicitly rejected. |
| **Multi-sig** | **FAIL** | Not supported |
| **Hybrid PQC keys** | ✅ | ECDSA + ML-DSA-65 post-quantum signing — innovative but adds complexity |

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

**Pool deployment needs:**
1. Missing `reject` P2P message — pool mining software cannot distinguish rejection reasons over P2P. Workaround: pools submit blocks via `submitblock` RPC (which returns proper reject codes) rather than broadcasting raw blocks over P2P.
2. No compact blocks — bandwidth consideration for high-volume pools.

---

## 6. Security

| Area | Verdict | Detail |
|---|---|---|
| **Go version** | ✅ | 1.26.0 (current) |
| **CGO dependencies** | ✅ | Yespower C source vendored; standard crypto libs |
| **RPC auth** | ⚠️ | Basic auth only (constant-time compare). No cookie auth. Password in cleartext config file. |
| **RPC TLS** | ⚠️ | Available but optional. Not enabled by default. |
| **RPC DoS** | ❌ | No rate limiting, no max peers, WriteTimeout=0 |
| **P2P DoS** | ✅ | Good size limits, rate limiting, ban system |
| **gosec findings** | ⚠️ | 104 pre-existing: G115, G304, G204, G406 — all Bitcoin-compatible intentional patterns, not exploitable |
| **Panic safety** | ✅ | No unprotected panics in production paths |
| **Memory safety** | ✅ | Go type-safe; no unsafe pointers |
| **Cryptography** | ✅ | crypto/rand for keys, double-SHA256 correctly used, Yespower from C reference |

---

## 7. Build & Release

| Area | Verdict |
|---|---|
| **Windows** | ✅ MSYS2 + Wails. Deterministic build via `build-windows.bat` |
| **Linux** | ✅ `scripts/build-linux.sh` with auto-dep install + cross-compile |
| **macOS** | ⚠️ Experimental, disabled in release matrix |
| **CI** | ✅ GitHub Actions: CI (every push) + Release Matrix (tag push) |
| **Mainnet verification** | ✅ Genesis hash, yespower backend, ports, message start — all verified during build |

---

## Exchange Checklist — What MUST be done before listing

- [ ] **Fix orphan sibling loss** (`blockchain.go:745-768`)
- [ ] **Fix silent reorg corruption** (`blockchain.go:893-895`)
- [ ] **Implement `estimatefee`/`estimatesmartfee`** RPC
- [ ] **Add HTTP write timeout** (currently 0)
- [ ] **Add per-IP rate limiting** on RPC
- [ ] **Enable txindex=1** by default for exchange nodes
- [ ] **Fix `listtransactions`** — paginate or use txindex instead of full chain scan
- [ ] **Add RPC cookie auth** or enforce TLS + strong password
- [ ] **Upgrade seed nodes to v1.0.7** — currently on 1.0.2/1.0.6, block sync stalls

**Not-blocking but recommended:**
- Implement `reject` P2P message (BIP 61)
- Increase scrypt N to 65536+ in wallet encryption
- Implement BIP44 HD derivation for standard wallet recovery

---

## Pool Checklist — What MUST be done before listing

- [ ] **Workaround for missing `reject` P2P message**: submit blocks via `submitblock` RPC (which returns proper reject codes) rather than broadcasting raw blocks over P2P
- [ ] **Use `coinbasetxn` capability** in getblocktemplate for payout splitting
- [ ] **Enable txindex=1** for pool node
- [ ] **Upgrade seed nodes to v1.0.7** — currently block sync stalls on 1.0.2
- [ ] **Add rate limiting** before exposing pool RPC to public internet

**Not-blocking but recommended:**
- Implement `sendheaders` (BIP 130) for faster block propagation
- Implement compact blocks (BIP 152) for bandwidth efficiency
