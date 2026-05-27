# RPC Guide

Purpose: public RPC reference for Legacy Core v1.0.4 integrations.  
Audience: wallet integrators, pools, exchanges, explorers, and operators.  
Status: active for v1.0.4.  
Safety warning: RPC (`19556`) should be private (localhost/private network only).

## What This Is

A practical RPC guide with status, parameters, commands, common errors, and integration notes.

## Authentication

Supported:

- cookie auth (local automation)
- `rpcuser` / `rpcpassword`

Never expose unauthenticated public RPC.

## JSON-RPC Format

```json
{"jsonrpc":"2.0","id":"example","method":"getblockcount","params":[]}
```

## Core Integration Methods

### getblockchaininfo

- Status: implemented
- Parameters: none
- CLI:

```bash
./legacycoin-cli getblockchaininfo
```

- Example response fields: `blocks`, `bestblockhash`, `chainwork`, `fork_choice`, `txindex`, `addressindex`, `storage`
- Errors: storage read issues can appear in nested health fields
- Integration notes: use for sync/health gating and index status checks

### getrawtransaction

- Status: implemented (`txindex=1` recommended for historical lookup)
- Parameters: `<txid> [verbose]`
- CLI:

```bash
./legacycoin-cli getrawtransaction <txid> true
```

- Example response fields: `txid`, `hex`, `vin`, `vout`, `confirmations`, `blockhash`
- Errors: tx not found, invalid txid, txindex disabled limitations
- Integration notes: with `txindex=1`, lookup is on-disk and reliable for historical tx scans

### getaddresstxids

- Status: implemented when `addressindex=1`
- Parameters: `<address>`
- CLI:

```bash
./legacycoin-cli getaddresstxids <address>
```

- Example response: array of txids
- Errors: addressindex disabled, bad address parameter
- Integration notes: foundation-level address index surface

### getaddressutxos

- Status: implemented when `addressindex=1`
- Parameters: `<address>`
- CLI:

```bash
./legacycoin-cli getaddressutxos <address>
```

- Example response fields: `txid`, `vout`, `value`, `height`, `coinbase`
- Errors: addressindex disabled, malformed argument
- Integration notes: useful for lightweight address UTXO snapshots

### getaddressbalance

- Status: implemented when `addressindex=1`
- Parameters: `<address>`
- CLI:

```bash
./legacycoin-cli getaddressbalance <address>
```

- Example response fields: `confirmed`, `total`
- Errors: addressindex disabled, malformed argument
- Integration notes: not a replacement for full external accounting index

### checkstorage

- Status: implemented
- Parameters: `[repair_bool]`
- CLI:

```bash
./legacycoin-cli checkstorage
./legacycoin-cli checkstorage true
```

- Example response fields: `ok`, `tip_height`, `height_index_matches_tip`, `chainwork_readable`
- Errors: storage/index corruption or read failures
- Integration notes: `true` triggers repair/reindex path for active-chain indexes

### reindex

- Status: implemented
- Parameters: none
- CLI:

```bash
./legacycoin-cli reindex
```

- Example response fields: rebuilt index health summary
- Errors: repair failures, read/write failures
- Integration notes: rebuilds height/hash and optional tx/address indexes when enabled

### getpeerinfo

- Status: implemented
- Parameters: none
- CLI:

```bash
./legacycoin-cli getpeerinfo
```

- Example response fields: `addr`, `height`, `reported_height`, `sync_state`, `last_ping_time`, `last_pong_time`, `ping_latency_ms`, `missed_pongs`, `stale`
- Errors: none expected when node is running
- Integration notes: use for stale peer and liveness monitoring

### getblocktemplate / submitblock

- Status: implemented (pool-ready candidate)
- Parameters:
  - `getblocktemplate [request_object]`
  - `submitblock <block_hex>`
- CLI:

```bash
./legacycoin-cli getblocktemplate
./legacycoin-cli submitblock <block_hex>
```

- Example response fields (`getblocktemplate`): `height`, `previousblockhash`, `transactions`, `coinbasevalue`, `bits`, `target`
- Errors (`submitblock`): decode errors, reject codes (e.g. bad prev, bad bits, high hash)
- Integration notes: use yespower with `LegacyCoinPoW` personalization

## Common Error Codes

- `-32602`: invalid params
- `-32601`: method not found
- `-22`: decode / malformed hex
- `-5`: not found or unavailable due to index mode

## Troubleshooting

- RPC refused: daemon not listening or wrong rpcport/datadir.
- RPC unauthorized: cookie mismatch or wrong credentials.
- tx not found: enable `txindex=1` and rebuild if historical lookup is required.
- address RPC disabled: set `addressindex=1` and rebuild.

## Known Limitations

- Address index is a foundation feature in v1.0.4.
- Full rich historical address analytics typically still needs explorer-side indexing.
