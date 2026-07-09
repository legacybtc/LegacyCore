# Changelog

## v1.0.33 (2026-07-09)

### P2P Header Validation Fix
- **prevHash linkage**: `ValidateHeaderSequence` now sets `prevHash` to SHA256d (`LegacyHeaderHash`) instead of yespower canonical hash — matches wire-protocol `PrevBlock` so validator accepts consecutive header batches without rejecting them as non-connected
- **Per-block hash reuse in P2P handler**: `HandleBlock` computes `BlockHash` once and passes `precomputedHash` through `ProcessBlockWithResult`, eliminating redundant yespower hashing during block processing
- **Dual-hash block serving**: `serveInventory` uses `BlockByWireHash` which supports dual-hash lookup (canonical yespower via direct DB load, legacy SHA256d via cache scan), ensuring blocks can be served to peers regardless of which hash they request

### Binaries
- Windows amd64: legacycoind, legacycoin-cli, LegacyWallet
- Linux amd64: legacycoind, legacycoin-cli

---

## v1.0.32 (2026-07-08)

### P2P Sync Stability
- **HashHeader dedup**: `validateActiveBlockLocked` accepts precomputed hash so `connectBlockLocked` skips the second yespower call — halves the dominant per-block CPU cost
- **Async reader goroutine**: dedicated goroutine reads TCP messages into a buffered channel (cap 64) during `handleConn`; server send buffer stays drained during slow block processing, eliminating write-timeout / reconnect cycles

### Binaries
- Windows amd64: legacycoind, legacycoin-cli, LegacyWallet
- Linux amd64: legacycoind, legacycoin-cli

---

## v1.0.31 (2026-07-06)

### P2P Sync Recovery
- **dual-hash getdata**: canonical yespower hash first, SHA256d as fallback for mixed-version peers
- **debug logging**: added `p2p HANDLER` trace logging at handshake, requestHeaders, serveHeaders, and requestSyncFromPeerIfBehind — visible in all log modes
- **getdata robustness**: missing INV-based getdata now re-requests from alternate peers
- **header sync**: locator-based header sync verified working across mixed-version nodes

### Build & Security
- **GitHub Actions**: all action references pinned to commit SHAs (supply chain security)
- **permissions: read-all**: least-privilege token model on all CI workflows
- **CodeQL + Scorecard + Dependabot**: new security workflows integrated
- **.gitignore**: patterns for linux cross-compiled binaries and test data directories

### Binaries
- Windows amd64: legacycoind, legacycoin-cli, LegacyWallet
- Linux amd64/arm64: legacycoind, legacycoin-cli
- macOS amd64/arm64: legacycoind, legacycoin-cli

---

## v1.0.30 (2026-07-03)

### Dual-Hash getdata
- `serveBlockInventory` fixed to use stored hashes instead of recomputing via HashHeader
- Dual-hash getdata: canonical yespower hash first, SHA256d fallback for legacy C-peer compatibility
- Full test suite passes with mixed-version mock peers

---

## v1.0.29 (2026-06-30)

### Dual-Hash Fallback
- Dual-hash getdata with SHA256d fallback for peer compat
- Improved header sync reliability across network protocol versions
