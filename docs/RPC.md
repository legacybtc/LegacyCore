# Legacy Core RPC Audit

Status terms: `implemented` means callable in v1.0.4 source; `partial` means useful but not a full Bitcoin Core compatible surface; `missing` means not callable; `planned` means documented future work.

Examples assume the daemon is running locally. Windows commands use `.\legacycoin-cli.exe`; Linux commands use `./legacycoin-cli`. JSON-RPC examples use cookie auth or `rpcuser`/`rpcpassword` over private localhost RPC only.

Common JSON-RPC shape:

```json
{"jsonrpc":"2.0","id":"example","method":"getblockcount","params":[]}
```

## Authentication

Cookie auth is implemented. The daemon writes `.cookie` inside the active data directory. `legacycoin-cli` reads it automatically unless `-rpcuser` and `-rpcpassword` are supplied.

Windows:

```powershell
.\legacycoind.exe run
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe -rpcuser=legacyrpc -rpcpassword=change_this getnetworkinfo
```

Linux:

```bash
./legacycoind run
./legacycoin-cli getnetworkinfo
./legacycoin-cli -rpcuser=legacyrpc -rpcpassword=change_this getnetworkinfo
```

Never expose RPC port `19556` publicly.

## Method Audit

### getblocktemplate

- Status: implemented, pool-ready candidate; external pool testing still required.
- Purpose: returns a BIP22/BIP23-style candidate template for CPU/pool mining.
- Parameters: optional request object, including longpoll capability fields.
- CLI: `.\legacycoin-cli.exe getblocktemplate` or `./legacycoin-cli getblocktemplate`
- JSON-RPC: `{"jsonrpc":"2.0","id":"gbt","method":"getblocktemplate","params":[{"capabilities":["longpoll"]}]}`
- Example response: object with `version`, `previousblockhash`, `transactions`, `coinbasevalue`, `bits`, `target`, `height`, `curtime`, `mintime`, `mutable`, `noncerange`, `sigoplimit`, `sizelimit`, `longpollid`, and `rules`.
- Error cases: uninitialized chain, storage failure, invalid request object.
- Integration notes: block hash identity uses Legacy Coin yespower header hash with personalization `LegacyCoinPoW`; do not assume SHA256d block IDs.

### submitblock

- Status: implemented.
- Purpose: submit a serialized block candidate.
- Parameters: one block hex string.
- CLI: `.\legacycoin-cli.exe submitblock <block_hex>` or `./legacycoin-cli submitblock <block_hex>`
- JSON-RPC: `{"jsonrpc":"2.0","id":"submit","method":"submitblock","params":["<block_hex>"]}`
- Example response: `null` when accepted; reject string such as `"bad-diffbits"` or `"bad-prevblk"` when rejected.
- Error cases: missing hex, bad hex, decode failure, invalid proof of work, bad previous block, bad transactions.
- Integration notes: pools should confirm acceptance by checking `getblockhash <height>` or `getblock <hash>` after submission.

### validateaddress

- Status: implemented.
- Purpose: validate Legacy Coin address syntax and wallet ownership hints.
- Parameters: address string.
- CLI: `.\legacycoin-cli.exe validateaddress <address>`
- JSON-RPC: `{"jsonrpc":"2.0","id":"addr","method":"validateaddress","params":["<address>"]}`
- Example response: `{"isvalid":true,"address":"...","ismine":false,"scriptPubKey":"..."}`
- Error cases: missing parameter, malformed base58/checksum/version.
- Integration notes: exchanges and pools should validate payout/deposit addresses before storing them.

### getrawtransaction

- Status: partial.
- Purpose: returns raw transaction hex or a verbose decoded object when the transaction is found.
- Parameters: txid string, optional verbose boolean.
- CLI: `.\legacycoin-cli.exe getrawtransaction <txid> true`
- JSON-RPC: `{"jsonrpc":"2.0","id":"rawtx","method":"getrawtransaction","params":["<txid>",true]}`
- Example response: verbose object with `txid`, `hex`, `vin`, `vout`, optional `blockhash`, `confirmations`, and mempool status.
- Error cases: invalid txid, transaction not found, storage read failure.
- Integration notes: there is no full txindex yet; lookup is best for wallet/mempool/recent chain paths and explorer use remains limited until txindex lands.

### sendrawtransaction

- Status: implemented.
- Purpose: validate and add a raw transaction to the mempool, then relay to peers when P2P is available.
- Parameters: raw transaction hex.
- CLI: `.\legacycoin-cli.exe sendrawtransaction <tx_hex>`
- JSON-RPC: `{"jsonrpc":"2.0","id":"send","method":"sendrawtransaction","params":["<tx_hex>"]}`
- Example response: transaction id string.
- Error cases: bad hex, decode error, duplicate spend, invalid signature, insufficient fee, non-standard output, orphan transaction.
- Integration notes: mempool policy is conservative and RBF is disabled in this release.

### getblock

- Status: implemented.
- Purpose: return block by hash.
- Parameters: block hash string, optional verbosity.
- CLI: `.\legacycoin-cli.exe getblock <hash>`
- JSON-RPC: `{"jsonrpc":"2.0","id":"block","method":"getblock","params":["<hash>"]}`
- Example response: decoded block object with header fields and transactions.
- Error cases: bad hash, block not found, storage read failure.
- Integration notes: use the yespower block hash returned by `getblockhash` or P2P inventory.

### getblockhash

- Status: implemented.
- Purpose: map active-chain height to block hash.
- Parameters: height integer.
- CLI: `.\legacycoin-cli.exe getblockhash 100`
- JSON-RPC: `{"jsonrpc":"2.0","id":"hash","method":"getblockhash","params":[100]}`
- Example response: `"5b4c...154f5"`
- Error cases: missing/invalid height, height unavailable, index corruption.
- Integration notes: explorers and exchanges should use this for active-chain scanning.

### getblockchaininfo

- Status: implemented.
- Purpose: summarize chain, sync, difficulty, supply, and storage state.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getblockchaininfo`
- JSON-RPC: `{"jsonrpc":"2.0","id":"chain","method":"getblockchaininfo","params":[]}`
- Example response: object with `chain`, `blocks`, `bestblockhash`, `difficulty`, `verificationprogress`, and sync/storage fields.
- Error cases: storage health errors can surface in nested fields.
- Integration notes: use alongside `getsyncstatus` to decide whether deposits should be credited.

### getnetworkinfo

- Status: implemented.
- Purpose: summarize P2P/RPC/network identity.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getnetworkinfo`
- JSON-RPC: `{"jsonrpc":"2.0","id":"net","method":"getnetworkinfo","params":[]}`
- Example response: object with version, subversion, protocol version, ports, peers, and network warnings.
- Error cases: none expected for a running daemon.
- Integration notes: confirms P2P port `19555`, RPC port `19556`, and operational peer counts.

### getrawmempool

- Status: implemented.
- Purpose: list mempool transaction ids.
- Parameters: optional verbose flag.
- CLI: `.\legacycoin-cli.exe getrawmempool`
- JSON-RPC: `{"jsonrpc":"2.0","id":"mempool","method":"getrawmempool","params":[]}`
- Example response: `["<txid>"]` or verbose map when supported.
- Error cases: none expected for a running daemon.
- Integration notes: explorers can show pending txids, but address-level mempool search requires an address index.

### getmempoolinfo

- Status: implemented.
- Purpose: return mempool counters and policy limits.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getmempoolinfo`
- JSON-RPC: `{"jsonrpc":"2.0","id":"mempoolinfo","method":"getmempoolinfo","params":[]}`
- Example response: object with `size`, `bytes`, `maxmempool`, `minrelaytxfee`, and orphan/dependency counters.
- Error cases: none expected.
- Integration notes: useful for node health dashboards.

### gettxout

- Status: implemented.
- Purpose: return an unspent output by txid and vout.
- Parameters: txid string, vout integer.
- CLI: `.\legacycoin-cli.exe gettxout <txid> 0`
- JSON-RPC: `{"jsonrpc":"2.0","id":"utxo","method":"gettxout","params":["<txid>",0]}`
- Example response: object with value, script, height, confirmations, coinbase flag, or `null`.
- Error cases: invalid txid/vout, storage read failure.
- Integration notes: good for payout validation and explorer UTXO views.

### gettxoutsetinfo

- Status: implemented.
- Purpose: return UTXO set stats.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe gettxoutsetinfo`
- JSON-RPC: `{"jsonrpc":"2.0","id":"utxos","method":"gettxoutsetinfo","params":[]}`
- Example response: object with UTXO count, total amount, and best block metadata.
- Error cases: storage read failure or corrupt UTXO JSON.
- Integration notes: use for sanity checks, not as a complete supply audit until full index tooling matures.

### getpeerinfo

- Status: implemented.
- Purpose: list connected peer diagnostics.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getpeerinfo`
- JSON-RPC: `{"jsonrpc":"2.0","id":"peers","method":"getpeerinfo","params":[]}`
- Example response: array with `addr`, `synced_blocks`, `last_seen_ago_seconds`, `last_sync_error`, and byte counters.
- Error cases: none expected.
- Integration notes: seed operators should watch stale peers and mismatched chain IDs.

### addnode

- Status: implemented.
- Purpose: request an outbound P2P connection.
- Parameters: address string, with optional port.
- CLI: `.\legacycoin-cli.exe addnode 203.0.113.10:19555`
- JSON-RPC: `{"jsonrpc":"2.0","id":"addnode","method":"addnode","params":["203.0.113.10:19555"]}`
- Example response: object with queued/connected status.
- Error cases: bad address, peer cap reached, dial failure.
- Integration notes: for private exchange/pool topologies, prefer explicit addnodes over public DNS seed dependence.

### disconnectnode

- Status: implemented.
- Purpose: close a matching peer connection.
- Parameters: address or address prefix.
- CLI: `.\legacycoin-cli.exe disconnectnode 203.0.113.10`
- JSON-RPC: `{"jsonrpc":"2.0","id":"disc","method":"disconnectnode","params":["203.0.113.10"]}`
- Example response: `true` when a peer was closed, `false` when not found.
- Error cases: missing address.
- Integration notes: useful for operational recovery from stale or misbehaving peers.

### backupwallet

- Status: implemented.
- Purpose: copy/export wallet data to a requested path.
- Parameters: destination path.
- CLI: `.\legacycoin-cli.exe backupwallet D:\LegacyBackups\wallet-backup.json`
- JSON-RPC: `{"jsonrpc":"2.0","id":"backup","method":"backupwallet","params":["/secure/legacycoin/wallet-backup.json"]}`
- Example response: object with backup path and success flag.
- Error cases: missing path, filesystem permissions, wallet read failure.
- Integration notes: exchanges should back up after wallet creation/import and before upgrades.

### getwalletsummary

- Status: implemented.
- Purpose: concise wallet balance/security/mining-address summary.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getwalletsummary`
- JSON-RPC: `{"jsonrpc":"2.0","id":"wallet","method":"getwalletsummary","params":[]}`
- Example response: object with balances, address counts, encryption state, and mining address.
- Error cases: wallet unavailable or corrupt.
- Integration notes: useful for GUI and hot-wallet monitoring.

### setupwallet

- Status: implemented.
- Purpose: initialize wallet and mining address; can set passphrase where supported.
- Parameters: optional passphrase string.
- CLI: `.\legacycoin-cli.exe setupwallet "strong passphrase"`
- JSON-RPC: `{"jsonrpc":"2.0","id":"setup","method":"setupwallet","params":["strong passphrase"]}`
- Example response: object with receive/mining address and setup status.
- Error cases: wallet locked, bad passphrase input, filesystem error.
- Integration notes: run once during controlled node provisioning, then back up immediately.

### getminingaddress

- Status: implemented.
- Purpose: return or create the wallet-owned default mining reward address.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getminingaddress`
- JSON-RPC: `{"jsonrpc":"2.0","id":"miningaddr","method":"getminingaddress","params":[]}`
- Example response: object with address and pubkey hash.
- Error cases: encrypted locked wallet, wallet storage error.
- Integration notes: pools usually use their own payout address handling; local solo miners use this.

### startminer

- Status: implemented for local CPU miner.
- Purpose: start built-in CPU mining.
- Parameters: optional thread count or object with `threads`, `stop_after_blocks`, `peer_required`.
- CLI: `.\legacycoin-cli.exe startminer`
- JSON-RPC: `{"jsonrpc":"2.0","id":"start","method":"startminer","params":[{"threads":2}]}`
- Example response: object with `active_mining`, `threads`, and reward hash.
- Error cases: no mining address, storage unhealthy, no peers when required, invalid threads.
- Integration notes: not a stratum pool server.

### stopminer

- Status: implemented and idempotent.
- Purpose: stop built-in CPU mining.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe stopminer`
- JSON-RPC: `{"jsonrpc":"2.0","id":"stopminer","method":"stopminer","params":[]}`
- Example response: object with `active_mining:false`, `was_active`, and counters.
- Error cases: none expected.
- Integration notes: safe to call repeatedly.

### restartminer

- Status: implemented.
- Purpose: stop and start built-in miner with current or supplied options.
- Parameters: optional mining options object.
- CLI: `.\legacycoin-cli.exe restartminer`
- JSON-RPC: `{"jsonrpc":"2.0","id":"restart","method":"restartminer","params":[]}`
- Example response: object with stop/start results.
- Error cases: same as `startminer`.
- Integration notes: useful after changing mining address or thread count.

### setminerthreads

- Status: implemented.
- Purpose: persist configured miner thread count.
- Parameters: positive integer.
- CLI: `.\legacycoin-cli.exe setminerthreads 4`
- JSON-RPC: `{"jsonrpc":"2.0","id":"threads","method":"setminerthreads","params":[4]}`
- Example response: `{"configured_threads":4,"note":"restart miner for active thread change to take effect"}`
- Error cases: invalid number, exceeds configured max, config write failure.
- Integration notes: restart miner for live worker count changes.

### getminerstatus

- Status: implemented.
- Purpose: return built-in miner status and safety checks.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe getminerstatus`
- JSON-RPC: `{"jsonrpc":"2.0","id":"miner","method":"getminerstatus","params":[]}`
- Example response: object with `active_mining`, `threads`, `local_hashps`, `last_error`, `accepted_blocks`, storage and wallet state.
- Error cases: config/storage read issues appear in nested fields.
- Integration notes: use for solo-miner monitoring, not external stratum accounting.

### doctor

- Status: implemented.
- Purpose: operator health report covering RPC, ports, storage, wallet, P2P, and mining readiness.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe doctor`
- JSON-RPC: `{"jsonrpc":"2.0","id":"doctor","method":"doctor","params":[]}`
- Example response: object with checks array and warnings.
- Error cases: none expected; failures are reported as check results.
- Integration notes: run before launch, after upgrades, and when wallet/miner status is inconsistent.

### checkstorage

- Status: implemented.
- Purpose: report storage health for best block, height index, and UTXO stats.
- Parameters: optional boolean `repair` (`true` triggers active-chain height-index rebuild).
- CLI: `.\legacycoin-cli.exe checkstorage` or `.\legacycoin-cli.exe checkstorage true`
- JSON-RPC: `{"jsonrpc":"2.0","id":"storage","method":"checkstorage","params":[true]}`
- Example response: `{"ok":true,"tip_height":123,"tip_hash":"...","height_index_matches_tip":true}`
- Error cases: corrupt JSON index, missing best block, unreadable UTXO dir.

### reindex

- Status: implemented.
- Purpose: rebuild active-chain height index from current tip linkage and return post-repair health.
- Parameters: none.
- CLI: `.\legacycoin-cli.exe reindex`
- JSON-RPC: `{"jsonrpc":"2.0","id":"reindex","method":"reindex","params":[]}`
- Integration notes: this is a safe active-chain index repair path; full txindex/addressindex rebuild flows remain planned.

## Compatibility Notes

- Address search by address is not implemented because no address index exists yet.
- Full historical txindex remains staged; explorer integrations should scan blocks by height and maintain their own index for now.
- Fork choice remains height-based for side-chain activation; explicit cumulative-chainwork fork choice is staged consensus-safe infrastructure work.
