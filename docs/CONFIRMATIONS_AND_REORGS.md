# Confirmations and Reorgs

Status: active-chain height/hash lookup and reorg-capable storage are `implemented`; fork choice is currently height-based rather than explicit chainwork-based.

## Confirmations

For a transaction in block height `H` and current active height `T`:

```text
confirmations = T - H + 1
```

For mempool transactions, confirmations are `0`.

## Recommended Policy

- Wallet UI: show pending at 0 confirmations.
- Normal user payments: wait at least 6 confirmations.
- Exchange deposits: 30 confirmations.
- Large exchange deposits: 100 confirmations or manual review.
- Coinbase/mining rewards: 100 confirmations because coinbase maturity is 100 blocks.

These are operational recommendations, not consensus rules.

## Detecting Reorgs

Store active-chain hash for every credited height.

Windows:

```powershell
.\legacycoin-cli.exe getblockcount
.\legacycoin-cli.exe getblockhash 100
```

Linux:

```bash
./legacycoin-cli getblockcount
./legacycoin-cli getblockhash 100
```

If a stored hash for a height changes, roll back credits from that height and rescan from the last common hash.

## Current Fork Choice Audit

The current chain implementation stores side branches and activates a side branch when it becomes longer than the active chain. The code does not yet maintain explicit cumulative chainwork. That is acceptable to document for v1.0.3, but a staged chainwork upgrade should be prioritized for v1.0.4.

Do not silently change fork-choice behavior in a minor integration hardening release.

## Reorg Test Coverage

Current and added tests cover:

- short fork loses
- longer side branch can become active
- invalid fork rejected/rollback paths
- height/hash index behavior after reorg
- UTXO undo restore
- storage rollback failure reporting

Still planned:

- broader mempool reorg reinsertion policy
- wallet coinbase maturity reorg scenarios
