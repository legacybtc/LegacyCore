# Security Model

Purpose: document operational security expectations for Legacy Core.  
Audience: all operators, wallets, pools, exchanges, and developers.  
Status: active for v1.0.4.  
Safety warning: early mainnet software should be run with strict security controls.

## Network Exposure Rules

- P2P `19555`: may be public.
- RPC `19556`: private only.

Never expose privileged RPC endpoints directly to public internet.

## Wallet Safety

- Never share `wallet.dat`, private keys, seed phrases, or RPC credentials.
- Back up wallet data before upgrades, reindex, migration, or key changes.
- Keep minimal funds in hot wallets.

## Release Trust

- Verify release SHA256 checksums before execution.
- Prefer official source/release channels.
- Treat unsigned binaries (for example SmartScreen prompts) with caution.

## RPC Security

Use one of:

- cookie auth on localhost
- strong `rpcuser`/`rpcpassword` on private networks

Combine auth with firewall controls.

## Operator Warnings

- Exchanges: high hot-wallet risk; enforce strict confirmations and cold-wallet policy.
- Pools: isolate worker infrastructure from wallet RPC.
- Seed nodes: expose P2P only; keep RPC private.

## Known Limitations

- v1.0.4 improves infrastructure hardening, but this remains early mainnet operational software and should be deployed conservatively.
