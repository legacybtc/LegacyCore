# LegacyCore Sync Hardening Candidate — 2026-08-20

This branch is a non-consensus P2P synchronization hardening candidate.

## Current fixes

- Keep dual-hash getdata batches within the peer's `maxServeInvItems` limit.
- Remove the redundant handshake-time `getblocks` request; header-driven block retrieval is the primary sync path.
- Add regression coverage for the dual-hash batching invariant.
- Run full Go tests, P2P/blockchain race tests, `go vet`, daemon/CLI/wallet builds, and Linux packaging in CI.

## Safety

Consensus rules are intentionally unchanged in this candidate. `main` remains untouched until CI and manual review pass.
