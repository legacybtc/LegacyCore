# Explorer Integration

Status: block and transaction primitives are implemented; optional native `txindex` and `addressindex` can be enabled for richer explorer lookup.

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

- Address RPCs require `addressindex=1`; disabled nodes return explicit unavailable errors.
- `getrawtransaction` historical coverage is strongest with `txindex=1`.
- Rich token explorer views remain `partial`, depending on token RPC coverage and index design.

Do not fake address search by scanning only the wallet or mempool. Use `addressindex=1` or maintain an explorer-side index.

## Recommended Explorer Architecture

1. Run a fully synced Legacy Core node.
2. Scan `getblockhash height` then `getblock hash` sequentially.
3. Store block, tx, output, input, and address-derived rows in the explorer database.
4. Track active chain hash at every height.
5. On reorg, roll back to the last common height and rescan.
6. Poll `getrawmempool` for pending transactions.

## Native Indexes

- `txindex=1`: enables txid-to-block lookup.
- `addressindex=1`: enables `getaddresstxids`, `getaddressutxos`, and `getaddressbalance`.
- `reindex`: rebuilds active-chain indexes, including optional tx/address indexes when enabled.
