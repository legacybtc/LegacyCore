# Changelog

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
