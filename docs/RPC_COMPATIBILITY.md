# RPC Compatibility Matrix (RC2)

Status legend:

- `implemented`: available and usable now
- `partial`: available but output differs from Bitcoin Core
- `missing`: not yet implemented in this repo

| RPC method | Status | Purpose | Bitcoin-compatible? | Notes |
|---|---|---|---|---|
| `getblockcount` | implemented | chain height | mostly | returns integer height |
| `getbestblockhash` | implemented | tip hash | mostly | returns hash string |
| `getblockhash` | implemented | hash by height | mostly | expects block height param |
| `getblock` | implemented | block details/hex | partial | includes local fields and full hex |
| `getblockheader` | implemented | header by hash | partial | supports verbose and raw-header-hex mode |
| `getrawtransaction` | implemented | raw tx by txid | partial | mempool + chain scan; tx index is linear scan, not dedicated DB index |
| `gettransaction` | implemented | wallet tx details | partial | returns tx metadata/hex; wallet debit-credit breakdown is limited |
| `gettxout` | implemented | UTXO lookup | mostly | requires txid + vout |
| `gettxoutsetinfo` | implemented | UTXO stats | partial | local stats structure |
| `sendrawtransaction` | implemented | broadcast raw tx | mostly | returns txid on accept |
| `decoderawtransaction` | implemented | decode raw tx | partial | decodes vin/vout/script fields from raw hex |
| `validateaddress` | implemented | address validation | partial | returns validity, ownership, and hybrid flag |
| `getnewaddress` | implemented | new wallet address | mostly | classic address output |
| `listunspent` | implemented | spendable outputs | partial | wallet-specific shape |
| `getbalance` | implemented | wallet balance | partial | returns LBTC fields in local schema |
| `getwalletinfo` | implemented | wallet/security info | partial | includes encryption/lock state |
| `getnetworkinfo` | implemented | network + identity | partial | includes RC2 identity fields |
| `getpeerinfo` | implemented | peer list/health | partial | returns `{count,outbound,peers}` |
| `getmempoolinfo` | implemented | mempool stats | partial | includes dependency stats |
| `getrawmempool` | implemented | mempool tx ids | mostly | array of txids |
| `getblocktemplate` | implemented | mining template | partial | includes pool-compatible `transactions[]` objects |
| `submitblock` | implemented | block submission | partial | success `null`; reject codes include `duplicate`, `bad-prevblk`, `high-hash`, `bad-diffbits` |
| `stop` | implemented | graceful shutdown | mostly | returns `"stopping"` |
