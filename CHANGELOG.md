# Changelog

## v1.0.36 (2026-08-20)

### Release hardening and synchronization reliability
- Promoted the clean production line to **Legacy Core 1.0.36** and **Legacy Wallet 1.0.36**.
- Added/retained P2P synchronization regression coverage for request batching and wire-message limits.
- Removed obsolete development-branch triggers from the production CI workflow; CI now targets `main` for normal pushes and validation.
- Updated GitHub Actions tooling used by the production/release pipeline to current validated major releases.
- Kept consensus parameters and mainnet identity unchanged; this release is a maintenance, reliability, and release-engineering update rather than a consensus fork.

### Release components
- Windows amd64: `legacycoind.exe`, `legacycoin-cli.exe`, Legacy Wallet
- Linux amd64/arm64: `legacycoind`, `legacycoin-cli`
- macOS amd64/arm64: `legacycoind`, `legacycoin-cli`
- Clean source archive and SHA256 checksums

### Verification gate
The v1.0.36 release is intended to ship only from the corrected `main` commit after the repository CI/release matrix reports successful Linux/Windows testing, source-cleanliness validation, mainnet identity checks, package generation, and artifact verification.

---

## v1.0.35 (2026-07-13)

### Gosec Deep Audit — 78 HIGH Findings Resolved

All 78 `gosec -severity high` findings are resolved (39 `#nosec` suppressions + 5 content fixes).
Run: `gosec -severity high -exclude=G115 ./...` → zero findings.

#### Content Fixes
- **G118 context leak (HIGH)** — `server.go:572` uses `context.WithoutCancel(ctx)` instead of `context.Background()` for server shutdown
- **G306 file permissions (MEDIUM)** — `diag.go` 4 `os.WriteFile` calls changed from `0644` to `0600`

#### Centralized G115 Bounds Checks (overflow)
- `asInt32()` / `intFromAny()` added in `nodeservice/service.go` — safe integer conversion with bounds preservation
- `int32FromMap()` added in `mining_safety_server.go` — typed map-safe integer extraction
- `acceptedBlockHeights()` in `server.go` — int→int32 bounds-safe conversion for block height filter
- 62 remaining G115 sites excluded via `-exclude=G115` flag (protocol-mandated: Bitcoin block timestamps as int32, compact targets, SHA256d nonces, script pushdata bytes, yespower params, genesis constants — all inherently bounded by protocol rules)
- `.gosec` config file documents exclusion rationale

#### #nosec Suppressions (39 sites, by-design documented)
- **G703 path traversal (7)** — config-derived file paths: service.go:3777/3995, config.go:717, app.go:885/920, server.go:2197/2215
- **G704 SSRF (2)** — CLI RPC connection URLs: main.go:61/69
- **G101 hardcoded credential (1)** — `MinerStopWorkerExitUnexpected` error constant: miner_stop.go:24
- **G406/G507 RIPEMD160 (4)** — Bitcoin P2PKH HASH160 protocol requirement: script.go:15/93, server.go:30/2722
- **G203 XSS (4)** — block explorer intentionally renders formatted HTML: explorer/main.go:214/474/581/653
- **G204 subprocess (8)** — file open/exec paths: service.go:3953-3962, exec_windows.go:13, tool_broker.go:77, llama_provider.go:81
- **G304 file inclusion (13)** — config-derived read paths: wallet.go:114/494, filestore.go:1072, server.go:2193/2211, diag.go:183, service.go:3973, atomicfile.go:54, config.go:104/178, app.go:853/995, explorer/main.go:782

### Binaries
- Windows amd64: legacycoind, legacycoin-cli, LegacyWallet
- Linux amd64/arm64: legacycoind, legacycoin-cli (native CGo yespower, musl-linked)
- macOS amd64/arm64: legacycoind, legacycoin-cli

### Known Limitations (unchanged from v1.0.34)
- **Headers from SHA256d peers are incompatible**: old peers (v1.0.20, v1.0.30) are on a SHA256d-mined chain diverging at block 1. Header-based sync fails with these peers. **Sync relies on the INV flow** (`requestUnknownBlocks`) which is reliable.
- **Height comparison with SHA256d peers is misleading**: peer heights (e.g. 7080) cannot be compared to yespower chain heights (e.g. 1025).

---

## v1.0.34 (2026-07-13)

### Security Audit — 23 Findings Fixed
- **CRITICAL: P2P panic crash** — `handleConn` now recovers from peer-handler panics instead of crashing the node
- **CRITICAL: Mutex leak** — `ReindexActiveChain` uses `defer Unlock` so early returns can't leave the mutex locked
- **CRITICAL: Use-after-close** — `Close()` nils `hasherCtx` so subsequent `HashHeader` calls get a clear error
- **CRITICAL: UTXO sum overflow** — `selectUTXOs` checks `math.MaxInt64` before adding each UTXO value
- **CRITICAL: Key bytes not zeroed** — private key and seed bytes are zeroed after use at all 4 sites (DumpPrivKey, signTxInputs, selectUTXOs, deriveNextPrivateKey)
- **CRITICAL: PQC fee underestimate (22×)** — fee estimation counts hybrid inputs at 5400 bytes vs 148 for ECDSA
- **HIGH: Data races (connectOnly, config fields)** — `configMu` + `connectOnlyMu` protect all runtime-config reads/writes
- **HIGH: CORS wildcard default** — empty origin now means no CORS header instead of `Access-Control-Allow-Origin: *`
- **HIGH: Mining safety bypass** — `generate` RPC now calls `checkSafeToMine` respecting `mining_safe_required` config
- **HIGH: RPC RBAC** — 56 public/read-only methods are accessible without auth; all others (wallet, mining, node control) require authentication even on localhost
- **HIGH: Orphan eviction non-determinism** — FIFO eviction via `orphanOrder` slice prevents attacker-controlled eviction
- **HIGH: Mempool tx rate limit** — max 1000 tx/sec prevents submission DoS
- **HIGH: Coinbase maturity bypass** — 0-confirm coinbase outputs are now rejected (was `> 0`, now `>= 0`)
- **HIGH: Change address premature persist** — change address is no longer persisted to disk until after the tx broadcasts successfully

### Medium / Low
- **scrypt N upgraded** — new wallets use N=1048576 (2^20); existing wallets still decrypt via fallback to old N=65536
- **Info leak fixed** — outpoint keys (txid:vout) removed from blockchain error messages returned to RPC callers
- **GPU cache staleness** — `sync.Once` replaced with 5-minute TTL so hot-plugged GPUs are discovered
- **Reorg rollback safety** — partial connect/disconnect failures during reorg are rolled back with full error reporting
- **Unbounded orphan memory** — bounded by `maxOrph` (100); `orphRef` cleaned up when last dependent orphan is evicted

### P2P Sync Flow — Single Getdata & Header Linkage Fix
- **Single combined getdata**: `requestHeaderBlocks` sends one combined message per batch instead of two separate getdata calls — fixes double `markBlocksRequested` / `addBlocksRequested` count that could stall block-body processing
- **Deduplicated LegacyHeaderHash**: `ValidateHeaderSequence` computes `LegacyHeaderHash` once per header, reuses it for both `prevHash` linkage and cache-warming; falls back to canonical hash on error
- **Cache warming during header validation**: `ValidateHeaderSequence` warms `legacyByHash` cache so `BlockByWireHash` lookups (from SHA256d peers) succeed without a full DB scan
- **Header linkage debug logging**: when a header batch is rejected at position N, logs header.PrevBlock, computed prevHash, first_prev, our_tip, and batch_len

### Known Limitations
- **Headers from SHA256d peers are incompatible**: old peers (v1.0.20, v1.0.30) are on a SHA256d-mined chain diverging at block 1. Header-based sync fails with these peers. **Sync relies on the INV flow** (`requestUnknownBlocks`) which is reliable.
- **Height comparison with SHA256d peers is misleading**: peer heights (e.g. 7080) cannot be compared to yespower chain heights (e.g. 1025).

### Binaries
- Windows amd64: legacycoind, legacycoin-cli, LegacyWallet
- Linux amd64/arm64: legacycoind, legacycoin-cli (native CGo yespower, musl-linked)
- macOS amd64/arm64: legacycoind, legacycoin-cli
