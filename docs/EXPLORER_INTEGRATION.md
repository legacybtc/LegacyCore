# Explorer Integration

Purpose: guide explorers/indexers integrating with Legacy Core RPC.  
Audience: explorer builders and indexing teams.  
Status: active for v1.0.4.  
Safety warning: keep RPC private; expose explorer API, not wallet RPC, to users.

## What This Is

A practical RPC integration guide for block, transaction, mempool, and optional index lookups.

## Quick Start

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getblockcount
./legacycoin-cli getblockhash <height>
./legacycoin-cli getblock <hash>
./legacycoin-cli getrawmempool
```

## Index Features in v1.0.4

- `txindex=1`: available as an opt-in foundation for txid historical lookup.
- `addressindex=1`: available as an opt-in foundation for address RPCs.
- `getrawtransaction` works best with `txindex=1`.
- `getaddresstxids`, `getaddressutxos`, `getaddressbalance` require `addressindex=1`.

## Important Warning About Address History

Address index support in v1.0.4 is a **foundation**, not a complete rich historical explorer index model.

Do not claim full historical address analytics unless your explorer adds its own full historical indexing layer.

## Recommended Explorer Flow

1. Read active chain tip (`getblockcount`).
2. Iterate heights: `getblockhash` -> `getblock`.
3. Store block/tx/input/output rows in explorer DB.
4. Track `(height, hash)` for reorg handling.
5. Poll mempool with `getrawmempool` and `getmempoolinfo`.

## Supply and Emission

Base values:

- max supply: 21,000,000 LBTC
- initial reward: 50 LBTC
- halving interval: 210,000 blocks
- coinbase maturity: 100 blocks

Compute issued/matured views in explorer logic, and clearly label immature coinbase balances.

## Troubleshooting

- `getrawtransaction` not found: enable `txindex=1` and rebuild indexes.
- address RPC disabled error: enable `addressindex=1` and rebuild indexes.
- sync lag: inspect `getsyncstatus` and `getpeerinfo`.

## Known Limitations

- Native address index is intentionally conservative in scope.
- External explorer DB/index remains recommended for rich public UX.
