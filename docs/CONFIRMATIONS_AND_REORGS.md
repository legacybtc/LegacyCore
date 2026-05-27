# Confirmations and Reorgs

Purpose: explain confirmation logic, reorg risk, and operational policy.  
Audience: exchanges, pools, explorers, and node operators.  
Status: active for v1.0.4.  
Safety warning: confirmation policy is operational risk management, not consensus.

## Confirmations

For a transaction in block height `H` and current active height `T`:

```text
confirmations = T - H + 1
```

Mempool transactions have `0` confirmations.

## Coinbase Maturity

Coinbase rewards require **100 confirmations** before they are spendable.

## Reorg Risk

Reorgs can occur on early mainnet and lower-hashrate networks.  
Systems that credit deposits should keep a per-height block hash record and detect hash changes.

## Fork Choice in v1.0.4

Legacy Core v1.0.4 tracks and uses cumulative chainwork for active chain selection.  
Fork choice prefers the valid branch with greatest cumulative chainwork.

This is implementation hardening and does **not** change mainnet consensus identity.

## What Exchanges Should Monitor

Run:

```bash
./legacycoin-cli getblockcount
./legacycoin-cli getblockhash <height>
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
```

Watch for:

- hash changes at already-credited heights
- `blocks_behind` or stale peer/sync warnings
- storage health failures

## Recommended Confirmation Policy

- Small user payments: 6 confirmations.
- Exchange deposits: at least 30 confirmations.
- Large deposits: 100 confirmations or manual review.
- Coinbase-origin funds: 100 confirmations minimum.

## Troubleshooting

- If node appears behind, inspect `getsyncstatus`.
- If hash mismatch appears, roll back credits from affected height and rescan.

## Known Limitations

- Reorg handling policy still depends on operator implementation quality (credit rollback and replay discipline).
