# LegacyCoin JSON-RPC API Reference

## Overview

LegacyCoin exposes a JSON-RPC API on port **19556** (mainnet). Requests use HTTP POST with JSON body.

### Authentication

Two methods:
1. **Cookie auth** (default) — `~/.legacycoin/.cookie` contains `user:pass`
2. **User/password** — set `rpcuser`/`rpcpassword` in `legacycoin.conf`

### Request Format

```json
{"jsonrpc":"1.0","id":1,"method":"methodname","params":[...]}
```

### Response Format

```json
{"result":..., "error":null, "id":1}
```

---

## Blockchain Methods

### `getblockchaininfo`

Returns blockchain state summary.

| Field | Type | Description |
|-------|------|-------------|
| chain | string | Network name (main) |
| blocks | int | Current height |
| bestblockhash | string | Best block hash |
| difficulty | float | Current difficulty |
| mediantime | int | Median time past |
| chainwork | string | Total chainwork (hex) |
| pruned | bool | Always false |
| softforks | object | Softfork status |
| warnings | string | Consensus warnings |

### `getblockcount`

Returns current block height.

### `getblockhash` `<height>`

Returns block hash at given height.

### `getblock` `<hash|height> [verbosity=1]`

Returns block data. verbosity=0: hex, 1: JSON with txids, 2: JSON with full tx data.

### `getblockheader` `<hash>`

Returns block header fields.

### `getbestblockhash`

Returns best block hash.

### `getchaintips`

Returns all chain tips (main chain + forks).

### `getdifficulty`

Returns current difficulty as multiple of minimum.

### `getmempoolinfo`

Returns mempool statistics.

### `getrawmempool` `[verbose=false]`

Returns mempool transactions.

### `gettxoutsetinfo`

Returns UTXO set statistics.

### `gettxout` `<txid> <vout>`

Returns UTXO entry if unspent.

---

## Transaction Methods

### `getrawtransaction` `<txid> [verbose=0]`

Returns raw transaction by txid. Requires `txindex=1`.

### `decoderawtransaction` `<hex>`

Decodes hex transaction to JSON.

### `sendrawtransaction` `<hex>`

Broadcasts transaction to network.

### `createrawtransaction` `<inputs> <outputs> [locktime]`

Creates unsigned transaction.

### `signrawtransactionwithwallet` `<hex>`

Signs transaction with wallet keys.

### `gettransaction` `<txid>`

Returns wallet transaction details.

### `estimatefee` `<blocks>`

Estimates fee per KB for target confirmation.

### `getmempoolentry` `<txid>`

Returns mempool entry for a transaction.

---

## Wallet Methods

### `setupwallet`

Initializes wallet. Returns mnemonic backup phrase on first run.

### `getwalletinfo`

Returns wallet state (balance, transactions, HD seed info).

### `getnewaddress` `[label]`

Generates new receiving address.

### `getrawchangeaddress`

Generates new change address.

### `validateaddress` `<address>`

Returns address validation and info.

### `getbalance` `[minconf=1]`

Returns wallet balance.

### `getreceivedbyaddress` `<address> [minconf=1]`

Returns total received by address.

### `sendtoaddress` `<address> <amount> [comment] [comment-to] [subtractfeefromamount]`

Sends coins to address.

### `sendmany` `<fromaccount> <addresses> [minconf] [comment]`

Sends to multiple addresses.

### `sendfromaddress` `<fromaddress> <toaddress> <amount>`

Sends from specific address.

### `listunspent` `[minconf=1] [maxconf=9999999] [addresses]`

Returns unspent outputs.

### `listtransactions` `[label] [count=10] [skip=0]`

Returns wallet transactions.

### `listsinceblock` `[blockhash] [target_confirmations=1]`

Returns transactions since a block.

### `getaddressbalance` `<addresses>`

Returns confirmed/unconfirmed balance for addresses.

### `getaddressutxos` `<addresses>`

Returns UTXOs for addresses.

### `getaddresstxids` `<addresses>`

Returns transaction IDs involving addresses.

### `getaddressinfo` `<address>`

Returns address metadata.

### `getaddresshistory` `<address>`

Returns full transaction history for address.

### `listaddresses`

Lists all wallet addresses.

### `importprivkey` `<privkey> [label] [rescan=true]`

Imports private key.

### `dumpprivkey` `<address>`

Exports private key.

### `backupwallet` `<destination>`

Backs up wallet.dat.

### `dumpwallet` `<filename>`

Dumps wallet keys.

### `encryptwallet` `<passphrase>`

Encrypts wallet (obsolete — HD wallets are encrypted at rest).

### `walletlock`

Locks wallet.

### `walletpassphrase` `<passphrase> [timeout]`

Unlocks wallet.

### `walletpassphrasechange` `<old> <new>`

Changes wallet passphrase.

### `sethdseed` `[new] [seed]`

Sets HD seed. Without seed, generates new one. Returns mnemonic if available.

### `exportmnemonic`

Returns wallet's BIP39 mnemonic backup phrase.

---

## Mining Methods

### `getblocktemplate` `[capabilities]`

Returns block template for mining.

### `submitblock` `<hex> [params]`

Substitutes a mined block.

### `getmininginfo`

Returns mining statistics.

### `getnetworkhashps` `[blocks=120] [height=-1]`

Estimated network hashrate.

### `generate` `<blocks>`

Mines blocks instantly (regtest).

### `setgenerate` `<generate> [genproclimit]`

Enables/disables CPU mining.

### `getgenerate`

Returns CPU mining status.

### `gethashespersec`

Returns current hashrate.

### `setminingaddress` `<address>`

Sets mining payout address.

### `getminingaddress`

Returns current mining payout address.

### `setminerthreads` `<n>`

Sets CPU miner thread count.

### `startminer`

Starts CPU mining.

### `stopminer`

Stops CPU mining.

### `autotuneminer`

Auto-tunes CPU miner performance.

### `configureminer` `<config>`

Configures miner parameters.

### `getminerstatus`

Returns miner configuration and status.

### `restartminer`

Restarts CPU miner.

### `benchmarkminer`

Runs mining benchmark.

---

## Network Methods

### `getpeerinfo`

Returns connected peer details.

### `getconnectioncount`

Returns peer count.

### `getnetworkinfo`

Returns network protocol info.

### `getnettotals`

Returns network traffic stats.

### `addnode` `<addr> <command>`

Manages peer connections (add/remove/onetry).

### `disconnectnode` `<addr>`

Disconnects a peer.

### `getknownpeers`

Returns known peer addresses.

### `getbootstrapinfo`

Returns bootstrap peer configuration.

### `getforkstatus`

Returns fork monitoring status.

### `getchainstatus`

Returns chain health status.

---

## Utility Methods

### `help` `[method]`

Returns method list or help text.

### `uptime`

Returns node uptime in seconds.

### `verifymessage` `<address> <signature> <message>`

Verifies a signed message.

### `stop`

Shuts down the node.

### `gethealth`

Returns node health status.

### `getselfcheck`

Runs self-diagnostics.

### `getlaunchchecklist`

Returns launch readiness checks.

### `getlaunchstatus`

Returns detailed launch status.

### `getreadiness`

Returns readiness for operation.

### `getnodeconfig`

Returns full node configuration.

### `getchainparams`

Returns chain parameters.

### `getpolicy`

Returns node policy settings.

### `getscriptstatus`

Returns script evaluation status.

### `checkstorage`

Runs storage self-check.

### `getminerstatus`

Returns miner status and configuration.

### `getchaintiming`

Returns chain timing statistics.

### `getdifficultyhistory` `[count=10]`

Returns recent difficulty adjustments.

### `captureresourcediagnostics`

Captures system resource diagnostics.

---

## Error Codes

| Code | Meaning |
|------|---------|
| -1 | General error |
| -2 | Unspecified |
| -3 | Type error |
| -4 | Invalid address or key |
| -5 | Invalid parameter |
| -6 | Invalid transaction |
| -7 | Mempool full |
| -8 | Block rejected |
| -9 | No wallet |
| -10 | Wallet encrypted |
| -11 | Wallet passphrase incorrect |
| -12 | Wallet passphrase too short |
| -13 | Wallet passphrase already set |
| -14 | Wallet already unlocked |
| -15 | Wallet needs passphrase to decrypt seed |
| -25 | Verify message failed |
| -27 | RPC in warmup |
| -28 | Chain is downloading |
| -32600 | Invalid request |
| -32601 | Method not found |
| -32602 | Invalid params |
| -32603 | Internal error |
