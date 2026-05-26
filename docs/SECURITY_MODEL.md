# Security Model

Legacy Core v1.0.3 is early mainnet software. Treat all wallet and RPC surfaces as sensitive.

## Network Ports

- P2P `19555`: may be public for node connectivity.
- RPC `19556`: must stay private/firewalled.

Never expose wallet/RPC publicly. If RPC must cross a host boundary, use strict firewall rules, authentication, and a private network.

## Wallet Safety

- Back up wallet data before receiving funds, mining, importing keys, or upgrading.
- Never share wallet files, private keys, seed material, or RPC cookies.
- Test restore procedures before holding meaningful balances.
- Keep exchange hot wallet balances small.
- Use cold-wallet procedures for reserves.

## Binary and Source Verification

- Verify SHA256 checksums for downloaded assets.
- Prefer source builds or release assets from the official repository.
- Windows binaries may show SmartScreen warnings if unsigned.
- Production mining/pool builds should report yespower backend `cgo-c-reference`.

## RPC Authentication

Supported:

- Cookie auth: `implemented`.
- `rpcuser`/`rpcpassword`: `implemented`.
- Public unauthenticated RPC: intentionally refused for non-local binds.
- TLS configuration: partially implemented; operators should still keep RPC private.

## P2P Trust Boundaries

P2P peers are untrusted. The node validates message start, chain metadata, block headers, proof of work, block transactions, mempool policy, and peer liveness. Peer diagnostics expose stale metadata, sync errors, and last-seen data for operators.

## Consensus Boundary

v1.0.3 hardening does not change consensus rules. Security work is limited to CI, docs, RPC/P2P diagnostics, storage checks, tests, and release process.

## Operator Warnings

- Seed operators: expose P2P only; keep RPC private.
- Pool operators: keep pool RPC on localhost/private network.
- Exchanges: assume hot wallet risk and use confirmation/reorg policy.
- Wallet users: back up before use and never send secrets to support channels.
