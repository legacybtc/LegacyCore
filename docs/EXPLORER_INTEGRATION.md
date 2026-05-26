# Explorer Integration

Status: block and transaction primitives are `partially implemented`; full explorer indexing is `planned`.

## Supported Queries

Block by height:

```powershell
.\legacycoin-cli.exe getblockhash 100
.\legacycoin-cli.exe getblock <hash>
```

```bash
./legacycoin-cli getblockhash 100
./legacycoin-cli getblock <hash>
```

Block by hash:

```powershell
.\legacycoin-cli.exe getblock <hash>
```

```bash
./legacycoin-cli getblock <hash>
```

Transaction by txid:

```powershell
.\legacycoin-cli.exe getrawtransaction <txid> true
```

```bash
./legacycoin-cli getrawtransaction <txid> true
```

Mempool:

```powershell
.\legacycoin-cli.exe getrawmempool
.\legacycoin-cli.exe getmempoolinfo
```

```bash
./legacycoin-cli getrawmempool
./legacycoin-cli getmempoolinfo
```

## Supply and Emission

Use the consensus schedule:

- Initial subsidy: 50 LBTC
- Halving interval: 210,000 blocks
- Max supply: 21,000,000 LBTC
- Coinbase maturity: 100 blocks

Explorers should compute issued supply from height and subsidy schedule, then clearly distinguish immature coinbase outputs from spendable supply.

## Network Hash Estimate

`getnetworkhashps` and `getchaintiming` are implemented. They are estimates based on recent block timing and current difficulty, not an oracle.

## Local Explorer Limitations

- Address search: `planned`, not implemented.
- Address balance by address: `planned`, requires address index.
- Full txindex: `planned`.
- Rich token explorer views: `partial`, depends on token RPC coverage and index design.

Do not fake address search by scanning only the wallet or mempool. A public explorer must build its own index from blocks until native address index exists.

## Recommended Explorer Architecture

1. Run a fully synced Legacy Core node.
2. Scan `getblockhash height` then `getblock hash` sequentially.
3. Store block, tx, output, input, and address-derived rows in the explorer database.
4. Track active chain hash at every height.
5. On reorg, roll back to the last common height and rescan.
6. Poll `getrawmempool` for pending transactions.

## Planned Native Indexes

- `txindex`: planned command/config name.
- `addressindex`: planned command/config name.
- Wallet activity history expansion: planned.
- Reindex command: planned as `reindex` or `repairindexes`.
