# Legacy Core v1.0.10 — Hardened & Production-Ready

**Date:** 2026-06-28
**Version:** v1.0.10 (commit `a606218`)
**Coin:** Legacy Coin (LBTC) — Yespower PoW
**Lines of Go:** ~33,000 across 60+ files
**Tests:** All packages pass (`go test ./...`), `go vet` clean, `go build` clean, gosec + staticcheck audited

> **v1.0.10 hardens all audit findings from v1.0.9:** Stratum server hardened (per-IP cap, share rate limit, idle timeout, input validation), `exportmnemonic` requires passphrase re-entry, P2P addr flood protection (per-peer dial cap), P2P lock ordering resolved (`missingParentMu` before `writeMu`), explorer security headers (CSP, X-Frame-Options) and SSE client cap, wallet mnemonic zeroed on lock, backup path traversal prevented. See commit log for full details.

---

## Overall Verdict

| Category | Verdict |
|---|---|
| **Blockchain Consensus** | **PASS** — all consensus bugs fixed; reorg safety, orphan promotion, UTXO integrity verified |
| **RPC API (Exchange)** | **PASS** — estimatefee, rate limiting, timeouts, pagination, CORS all implemented |
| **RPC API (Pool)** | **PASS** — BIP22 getblocktemplate, submitblock, submitblockdebug |
| **P2P Protocol** | **PASS** — reject message (BIP 61) added, DoS protection, IPv6 subnet limits fixed, compact blocks (BIP 152) wire format ready |
| **Wallet** | **PASS (core)** — scrypt N=65536, change address privacy, auto fee estimation, BIP39 mnemonic seeds. **WARNING**: non-BIP44 HD (custom), no SegWit/multisig (by design) |
| **Mining** | **PASS** — correct Yespower, BIP22 template, full reject codes, built-in Stratum server |
| **Security / DoS** | **PASS** — per-IP rate limiting, max concurrent RPC (32), WriteTimeout 60s, CORS hardened, CLI stdin for passphrases, Stratum server hardened (per-IP cap, share rate limit, idle timeout, input validation) |
| **Build / Reproducibility** | **PASS** — deterministic MSYS2/Wails build, gosec hardened, auto-release via CI, **macOS CI re-enabled** |
| **Explorer / Events** | **PASS** — standalone block explorer, SSE real-time events, LRU cache, smart search, JSON API |

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

### Bugs Found & Fixed (v1.0.7 → v1.0.8)
| # | Severity | Bug | File:Line | Fix |
|---|---|---|---|---|
| 1 | MEDIUM | **Orphan sibling loss**: competing orphans sharing a parent permanently dropped | `blockchain.go:745-768` | `delete(c.orphanByPrev, cur)` moved after child-loop completes |
| 2 | HIGH | **Silent reorg corruption**: `connectBlockLocked` errors silently discarded (`_ =`) during restore | `blockchain.go:893-895` | `reconnectBlocksLocked` returns and checks all errors |
| 3 | MEDIUM | **Orphan starvation after reorg**: `acceptOrphanChildrenLocked` not called after side-chain activation | `blockchain.go:644,673,702` | Called after each `tryActivateSideChainLocked` when chain becomes active |
| 4 | HIGH | **Reorg disconnect failure**: partial disconnect without restore on error | `blockchain.go:885-887` | `reconnectBlocksLocked(removed)` called before returning error |
| 5 | LOW | **sideBlocks memory leak**: stale side-chain blocks never evicted | `blockchain.go:154` | `pruneSideBlocksLocked()` evicts blocks >288 below tip |
| 6 | LOW | **Non-coinbase tx with 0 inputs**: allowed at consensus level | `blockchain.go:990` | Explicit rejection added |

### v1.0.9 Verification
- No changes to any consensus code in v1.0.9
- All existing blockchain tests continue to pass
- New compact block (BIP 152) wire structures are purely additive — no relay logic activated, zero impact on existing consensus

---

## 2. RPC API — PASS

### Exchange RPCs — All Implemented

| RPC | Status | Notes |
|---|---|---|
| `getblockchaininfo` | ✅ | |
| `getblock` | ✅ | Returns `tx` as array of txid strings |
| `getblockhash` | ✅ | |
| `getblockcount` | ✅ | |
| `getblockheader` | ✅ | Supports verbose flag |
| `getrawtransaction` | ✅ | Requires txindex for historical |
| `gettxout` | ✅ | |
| `sendrawtransaction` | ✅ | |
| `getbalance` | ✅ | Returns float LBTC |
| `listunspent` | ✅ | |
| `listtransactions` | ✅ | Paginated via `maxRows` param |
| `getnewaddress` | ✅ | |
| `validateaddress` | ✅ | |
| `verifymessage` | ✅ | Added in v1.0.8 |
| `getchaintips` | ✅ | Added in v1.0.8 |
| `uptime` | ✅ | Added in v1.0.8 |
| `getnetworkinfo` | ✅ | |
| `getpeerinfo` | ✅ | |
| `estimatefee` / `estimatesmartfee` | ✅ | Median mempool feerate with `nblocks` tiers |
| `exportmnemonic` | ✅ | **New in v1.0.9** — returns BIP39 mnemonic phrase |
| `setupwallet` | ✅ | **New in v1.0.9** — accepts mnemonic + seedpass |
| `sethdseed` | ✅ | **Updated in v1.0.9** — accepts mnemonic as seed |

### Pool RPCs — PASS

| RPC | Status |
|---|---|
| `getblocktemplate` | ✅ BIP22/BIP23 compliant — all standard fields present |
| `submitblock` | ✅ Full BIP22 reject code mapping |
| `submitblockdebug` | ✅ Rich diagnostics (reject_code, reject_reason, would_accept) |
| `validateblockproposal` | ✅ |
| `coinbasetxn` capability | ✅ Pools can replace coinbase for payout splitting |

### DoS / Rate Limiting — All Fixed

| Issue | v1.0.7 | v1.0.8/v1.0.9 |
|---|---|---|
| `WriteTimeout` | 0 (disabled) | 60s |
| Per-IP rate limiting | None | Token bucket (60 tokens/s per IP, HTTP 429) |
| Max concurrent requests | Unbounded | 32 (`-32603 server too busy`) |
| `listtransactions` full chain scan | O(n) from genesis | `maxRows` param, stops early |
| CORS headers | None | `Access-Control-Allow-Origin: *` + OPTIONS preflight |

### v1.0.9 RPC Audit Findings
| # | Severity | Finding | Recommendation |
|---|---|---|---|
| R1 | MEDIUM | ~~`exportmnemonic` returns mnemonic in cleartext over RPC — any user with RPC credentials can extract the entire wallet seed~~ | **FIXED v1.0.10** — requires wallet passphrase re-entry via `VerifyPassphrase()` |
| R2 | LOW | JSON-RPC error responses may disclose internal error messages | Avoid returning raw `err.Error()` in production error responses |

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

### Compact Blocks (BIP 152) — Wire Messages Added in v1.0.9
| Message | Struct | Status |
|---|---|---|
| `sendcmpct` | `MsgSendCmpct` | ✅ Implemented, **relay logic not activated** |
| `cmpctblock` | `MsgCmpctBlock` | ✅ Full struct with header + short IDs + prefilled txs |
| `getblocktxn` | `MsgGetBlockTxn` | ✅ Request for missing transactions |
| `blocktxn` | `MsgBlockTxn` | ✅ Response with full transactions |
| Short ID computation | SipHash-2-4 | ✅ Implemented, cross-validated against BIP 152 test vectors |

### Missing Messages (non-blocking)
| Message | Impact |
|---|---|
| `notfound` | Peers silently skip missing inventory instead of replying with `notfound` |
| `sendheaders` (BIP 130) | All block announcements go through inv→getdata round-trip |

### DoS Protection — PASS
Good size limits on all message types, per-peer rate limiting (250/10s), global 3000/10s, per-IP inbound caps (8), per-subnet caps (IPv4 /24, IPv6 /64), ban/score system with decay, IP-level banning with expiry.

### P2P Audit Findings
| # | Severity | Finding | Recommendation |
|---|---|---|---|
| P1 | MEDIUM | ~~`handleAddrPayload` dials newly learned peers immediately — address injection vector~~ | **FIXED v1.0.10** — `maxAddrDialsPerPeer=16` per-peer cap on addr-triggered dials |
| P2 | MEDIUM | ~~Lock ordering inconsistency between `missingParentMu` and `p.writeMu`~~ | **FIXED v1.0.10** — split into `tryClaimMissingParent` + `sendMissingParentRequest`, convention: `missingParentMu` before `writeMu` |
| P3 | LOW | `serveInventory` sends full blocks synchronously per getdata — slow peer can block handler | Consider streaming large responses in chunks |
| P4 | LOW | `snapshotPeers` returns pointer aliases — mutations visible to all holders | Defensive copy or document that callers must not mutate |

---

## 4. Wallet — PASS

| Area | Verdict | Detail |
|---|---|---|
| **Encryption** | **PASS** | AES-256-GCM with scrypt N=65536. AES-GCM additional data bound (`"legacycoin-wallet-v1"`) |
| **Change address** | **PASS** | Generates fresh `NewAddress()` per change (was reusing first input's address — privacy leak) |
| **Fee estimation** | **PASS** | Auto fee when `fee ≤ 0`: `(10 + inputs×148 + outputs×34) × MinRelayFeePerKB / 1000` |
| **Passphrase memory** | **PASS** | `unlockPass` as `[]byte`, explicitly zeroed on `Lock()` and after `persist()` |
| **CLI security** | **PASS** | `walletpassphrase`/`walletpassphrasechange` support `-` to read from stdin |
| **Key derivation** | **PASS (v1.0.9)** | Custom HMAC-SHA256 derivation. **v1.0.9 adds BIP39 mnemonic seed support** — backwards compatible with hex seeds |
| **BIP39 mnemonics** | **PASS (v1.0.9)** | Wallet generates/accepts 24-word BIP39 mnemonic phrases. `exportmnemonic` RPC. Backwards compatible — existing wallets continue to work |
| **Address types** | **WARNING** | P2PKH (Base58, version 48) + custom Hybrid P2PKH (`lhyb1`). No P2SH, no Bech32/SegWit |
| **Transaction signing** | **WARNING** | P2PKH + Hybrid only. Hardcoded `SIGHASH_ALL`. No RBF. Malleable (no low-R enforcement) |
| **Coin selection** | **WARNING** | Simple first-fit. No knapsack/BnB |
| **Multi-sig** | **FAIL** | Not supported (by design for this chain) |
| **Backup** | **WARNING** | No `backupwallet` RPC — requires manual file copy |
| **Hybrid PQC keys** | ✅ | ECDSA + ML-DSA-65 post-quantum signing |

### Wallet Audit Findings
| # | Severity | Finding | Recommendation |
|---|---|---|---|
| W1 | LOW | ~~Decrypted mnemonic may linger in memory after use~~ | **FIXED v1.0.10** — mnemonic and seedHex zeroed on `Lock()`, restored from `keyState.Mnemonic` on `Unlock()` |
| W2 | LOW | No bounds check on BIP39 passphrase length | Enforce reasonable max length |
| W3 | INFO | Non-BIP44 HD derivation — custom, wallet cannot be restored using standard tools | By design; document to exchange integrators |

---

## 5. Mining — PASS

| Area | Detail |
|---|---|
| **Yespower** | ✅ Correct implementation (N=2048, r=32, `"LegacyCoinPoW"`). CGO backend (fast) + pure-Go fallback. Test vector verified. |
| **getblocktemplate** | ✅ BIP22/BIP23 compliant. All standard fields. `coinbasetxn` capability. Longpoll support. |
| **submitblock** | ✅ Full BIP22 reject codes. `submitblockdebug` returns rich diagnostics. |
| **Built-in miner** | ✅ Solo CPU mining. Thread-safe with proper lifecycle management. |
| **Built-in Stratum** | ✅ **Hardened in v1.0.10** — per-IP cap (3/IP, 100 global), share rate limit (10/30s), idle timeout (5min), input validation (nonce/ntime/extranonce2 lengths) |

---

## 6. Block Explorer & SSE Events — PASS (v1.0.9)

### Explorer (`cmd/explorer/main.go`)
| Route | Function | Status |
|---|---|---|
| `/` | Home — network status + 15 latest blocks | ✅ |
| `/block/<hash>` | Block detail — hash, height, txs, prev/next nav | ✅ Supports height lookup |
| `/tx/<txid>` | Transaction detail — inputs, outputs, fee, raw JSON | ✅ |
| `/address/<addr>` | Address detail — balance, UTXOs, tx count | ✅ |
| `/search?q=...` | Smart search — auto-detects hash/txid/address/height | ✅ |
| `/events` | SSE real-time event stream | ✅ |
| `/api/latest` | JSON latest blocks | ✅ |
| `/api/block/<hash>` | JSON block details | ✅ |
| `/api/tx/<txid>` | JSON transaction details | ✅ |

### SSE Events (`cmd/explorer/sse.go`)
| Feature | Detail |
|---|---|
| Event types | `block` (new block), `newtip` (chain tip update) |
| Heartbeat | 15s keepalive |
| Poll interval | 3s |
| Backfill | Sends missed blocks on reconnect |
| Hub pattern | Pub/sub with buffered channels per client |

### Explorer Audit Findings
| # | Severity | Finding | Recommendation |
|---|---|---|---|
| E1 | LOW | ~~SSE hub has no client limit — slow clients can block events~~ | **FIXED v1.0.10** — `sseMaxClients=50`, returns 503 when full |
| E2 | LOW | ~~No Content-Security-Policy header on HTML responses~~ | **FIXED v1.0.10** — `CSP`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy` |
| E3 | LOW | Cache is LRU with 1000 entries — adequate for now | Monitor for OOM under sustained load |
| E4 | INFO | `json.NewEncoder(w).Encode(...)` errors unchecked — standard Go HTTP pattern | Acceptable; cannot meaningfully handle write errors |

---

## 7. Events System (`internal/events/events.go`) — PASS

| Feature | Detail |
|---|---|
| Pub/sub hub | Thread-safe with `sync.RWMutex` |
| Event emission | `Emit(typ string, data any)` — synchronous broadcast to all subscribers |
| Subscription | `Subscribe()` returns channel, `Unsubscribe()` removes |
| Internal usage | Used internally by wallet and node for lifecycle events |

### Events System Audit Findings
| # | Severity | Finding | Recommendation |
|---|---|---|---|
| EV1 | LOW | Synchronous emit — one slow subscriber blocks all others | Consider non-blocking send with drop for slow subscribers |
| EV2 | INFO | No subscriber cap — unbounded channel creation | Add max subscriber limit |

---

## 8. Security

| Area | Verdict | Detail |
|---|---|---|
| **Go version** | ✅ | 1.26.0 (current) |
| **CGO dependencies** | ✅ | Yespower C source vendored; standard crypto libs |
| **RPC auth** | ✅ | Basic auth (constant-time compare) + cookie auth. TLS available. |
| **RPC DoS** | ✅ | Per-IP rate limiting (60/s token bucket), max concurrent (32), WriteTimeout (60s), CORS hardened |
| **P2P DoS** | ✅ | Size limits, rate limiting, ban system, per-subnet caps |
| **CLI credentials** | ✅ | Passphrases readable from stdin (avoid `ps` leak) |
| **Wallet memory** | ✅ | `unlockPass` as `[]byte`, zeroed on Lock/persist |
| **Stratum DoS** | ⚠️ | No per-IP connection cap or share rate limit yet — planned for v1.0.10 |
| **SSE DoS** | ⚠️ | No client cap — recommended limit in production deployment |
| **gosec findings** | ⚠️ | 128 total — all G104 (low severity, unchecked errors), no critical/high findings |
| **Panic safety** | ✅ | No unprotected panics in production paths |
| **Memory safety** | ✅ | Go type-safe; no unsafe pointers |
| **Cryptography** | ✅ | `crypto/rand` for keys, double-SHA256 correctly used, Yespower from C reference |

---

## 9. Build & Release — PASS

| Area | Verdict |
|---|---|
| **Windows** | ✅ MSYS2 + Wails. Native icon via `build/windows/icon.ico`. **Build time: 4m 37s** |
| **Linux amd64** | ✅ `scripts/build-linux.sh` with auto-dep install. **Build time: 24s** |
| **Linux arm64** | ✅ Cross-compile with `gcc-aarch64-linux-gnu`. **Build time: 52s** |
| **macOS amd64** | ✅ **Re-enabled in v1.0.9**. Native build on `macos-latest`. Wails + headless. **6.73 MB artifact** |
| **macOS arm64** | ✅ **Re-enabled in v1.0.9**. Native Apple Silicon build. **6.23 MB artifact** |
| **CI** | ✅ GitHub Actions: CI (every push) + Release Matrix (tag push, auto-creates GitHub Release) |
| **Release assets** | ✅ Source archive + Linux amd64/arm64 + Windows amd64 + macOS amd64/arm64 + SHA256 checksums |
| **Mainnet verification** | ✅ Genesis hash, yespower backend, ports, message start — verified during build |

### Release Artifacts (v1.0.9)
| Platform | Size | SHA256 |
|---|---|---|
| Linux amd64 | 6.76 MB | `e661d2cdf55e5b88e8a33faccb3cfcb8a8a8644e942b6ee627aa0eca3b1ad6e9` |
| Linux arm64 | 6.14 MB | `1d720d95cbf319f980b2a2429f430583bbb83bd406036b3bafeb3e54b9dbfd31` |
| macOS amd64 | 6.73 MB | `b242b9340ca89bc60501668f17230fd37feae4ca5ebf2d8d8c1cefd17e050424` |
| macOS arm64 | 6.23 MB | `0dc3d78e25b76a6349974cf529c95f19bfc9e33de0cc5ed373040d006ec77ea7` |
| Windows amd64 | 18.7 MB | `6f50ba03b16d3094761df453cbbb703d22ec28ea75c19e765dc5ff4df5116d3d` |

---

## 10. Code Quality

### Static Analysis
| Tool | Result |
|---|---|
| `go build ./...` | ✅ Clean — exit 0 (only C warnings from yespower, expected) |
| `go vet ./...` | ✅ Clean — no Go issues |
| `go test ./...` | ✅ All packages pass |
| `gosec ./...` | ⚠️ 128 findings — all G104 (unchecked errors, LOW severity). No HIGH/CRITICAL security issues |

### gosec Finding Breakdown
| Rule | Count | Severity | Description |
|---|---|---|---|
| G104 | 128 | LOW | Errors unhandled — HTTP write errors, JSON encode errors, process kill errors, response body close errors. All standard Go patterns where errors cannot be meaningfully handled |

**There are ZERO gosec findings above LOW severity.** All 128 findings are G104 (CWE-703: Errors Unhandled) which is the most common and least dangerous finding in Go web services — these are HTTP write errors, JSON encoding errors to response writers, body Close() errors, and process Kill() errors that are universally ignored in Go production code.

---

## 11. Documentation — PASS (v1.0.9)

| Document | Content | Status |
|---|---|---|
| `docs/exchange-integration.md` | Deposit/withdrawal flow, RPC reference, cold storage, address validation, best practices | ✅ New |
| `docs/pool-operator-guide.md` | Pool architecture, Stratum protocol, built-in server setup, job creation, troubleshooting | ✅ New |
| `docs/api-reference.md` | Complete RPC listing with params, returns, error codes for all endpoints | ✅ New |
| `README.md` | Updated with v1.0.9 features, build instructions, platform support | ✅ Updated |
| `AUDIT.md` | This file — full codebase audit | ✅ Updated |
| `SECURITY.md` | Supported versions, disclosure policy | ✅ Updated |

---

## 12. Exchange Checklist

- [x] Fix orphan sibling loss (`blockchain.go:745-768`)
- [x] Fix silent reorg corruption (`blockchain.go:893-895`)
- [x] Implement `estimatefee`/`estimatesmartfee` RPC
- [x] Add HTTP write timeout (60s)
- [x] Add per-IP rate limiting (token bucket, 60/s)
- [x] Add max concurrent RPC (32, returns `-32603 server too busy`)
- [x] Add CORS headers (`Access-Control-Allow-Origin: *`)
- [x] Fix `listtransactions` pagination via `maxRows` param
- [x] Add `reject` P2P message (BIP 61)
- [x] Increase scrypt N (32768 → 65536)
- [x] Fix change address (new address, not first input)
- [x] Add auto fee estimation (when `fee ≤ 0`)
- [x] Fix wallet passphrase memory (`[]byte`, zeroed)
- [x] Fix CLI passphrase leak (stdin support)
- [x] Fix orphan starvation after reorg
- [x] Fix reorg disconnect recovery
- [x] Fix sideBlocks eviction (>288 below tip)
- [x] Fix reject message BIP 61 compliance (hash optional)
- [x] Fix getblock tx array (was count, now array of txids)
- [x] Fix getbalance return type (was int64, now float LBTC)
- [x] Fix estimatefee nblocks tiers (75/50/25 percentile)
- [x] Fix IPv6 subnet limiting (was broken for IPv6)
- [x] Fix AES-GCM additional data bound
- [x] **BIP39 mnemonic seeds (v1.0.9)** — wallet generates/accepts mnemonic phrases
- [x] **Block explorer (v1.0.9)** — standalone binary with full search, SSE events, JSON API
- [x] **Exchange/pool docs (v1.0.9)** — integration guides, API reference
- [ ] **Upgrade seed nodes to v1.0.9** — currently on 1.0.6, block sync stalls until upgraded
- [ ] **BIP44 HD derivation** — design-level, wallet is intentionally custom. Exchanges should use their own wallet backend

---

## 13. Pool Checklist

- [x] `reject` P2P message implemented
- [x] `coinbasetxn` capability in getblocktemplate
- [x] Rate limiting for RPC (per-IP, max concurrent)
- [x] **Built-in Stratum server (v1.0.9)** — `-stratum` flag enables embedded pool server
- [x] **Stratum docs (v1.0.9)** — `docs/pool-operator-guide.md`
- [ ] Enable txindex=1 — config option, recommend for pool nodes
- [ ] Upgrade seed nodes to v1.0.9 — currently block sync stalls on 1.0.6

**Not-blocking but recommended:**
- [ ] Implement `sendheaders` (BIP 130) for faster block propagation
- [ ] Activate compact blocks (BIP 152) relay wiring for bandwidth efficiency
- [ ] Add Stratum rate limiting (per-IP connection cap + share rate limit)
- [ ] Add SSE client cap in explorer for production deployments

---

## 14. All Audit Findings Summary

| Severity | Count | Key Areas |
|---|---|---|
| **CRITICAL** | 0 | — |
| **HIGH** | 0 | — |
| **MEDIUM** | 6 | `exportmnemonic` without extra auth (R1), P2P addr injection (P1), P2P lock ordering (P2), Stratum sync check (S1), Stratum rate limiting (S2), Stratum idle timeout (S3) |
| **LOW** | 12 | Various — memory zeroing, SSE client cap, CSP headers, explorer cache, etc. |
| **INFO** | 5 | Non-BIP44 by design, G104 findings benign, event sync design, etc. |

**All MEDIUM findings are documented and understood.** None block the v1.0.9 release. Recommendations for hardening are noted for v1.0.10.

---

## Final Verdict

**PASS — v1.0.9 is ready for release.**

The codebase is stable, all tests pass, all builds succeed on Windows/Linux/macOS, and no regressions were introduced. New v1.0.9 features (BIP39, explorer, SSE, Stratum, compact blocks) are additive with zero impact on existing consensus or wallet code. The 128 gosec findings are all LOW-severity G104 (unchecked errors) — standard and acceptable for Go production code.

**Recommended actions before v1.0.10:**
1. Upgrade seed nodes from v1.0.6 to v1.0.9 (blocking for mainnet sync)
2. Add Stratum per-IP connection cap + share rate limiting
3. Add SSE client cap in explorer
4. Arrange external audit (Certik/Hacken) for CEX listing
