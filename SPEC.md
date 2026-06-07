# Legacy Coin Protocol Spec v1.0

Purpose: stable protocol anchor for LegacyCore, native miners, stratum pools, explorers, and future implementations.

## Mainnet Identity

- Coin: Legacy Coin / LBTC
- Message start: `a4 ac c6 4d`
- P2P port: `19555`
- RPC port: `19556`
- Chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`
- Genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- yespower personalization: `LegacyCoinPoW`

## References

- P2P discovery, liveness, and sync: `docs/P2P_PROTOCOL.md`
- RPC methods and error behavior: `docs/RPC.md`
- Pool integration and block diagnostics: `docs/POOL_INTEGRATION.md`
- Solo mining, yespower, and coinbase splits: `docs/MINING.md`
- Seed-node operations: `docs/SEED_NODE_OPERATOR.md`

## Compatibility Notes

- Consensus block identity uses the configured yespower header hash.
- P2P block inventory and indexes must use the consensus block hash.
- `getblocktemplate` returns fresh jobs with `submitold=false`, `expires=15`, and a `longpollid` tied to current tip plus mempool count.
- Coinbase transactions may contain multiple outputs if total value is valid.
- Public RPC exposure is refused unless authentication and TLS are configured; seed nodes require local RPC.
