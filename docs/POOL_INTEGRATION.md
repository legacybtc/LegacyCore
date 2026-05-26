# Pool Integration

Status: `integration-ready candidate, external pool testing still required`.

Legacy Core exposes `getblocktemplate`, `submitblock`, address validation, chain height/hash, peer, mempool, and storage health RPCs. It does not include a stratum server.

## RPC Setup

Keep RPC private. Bind to localhost or a private interface behind firewall rules. Never expose port `19556` to the public internet.

Windows config example:

```text
rpcbind=127.0.0.1
rpcuser=legacyrpc
rpcpassword=change_this_long_random_password
```

Linux config example:

```text
rpcbind=127.0.0.1
rpcuser=legacyrpc
rpcpassword=change_this_long_random_password
```

Cookie auth is supported for local automation. `rpcuser`/`rpcpassword` is better for a pool process running as a separate service account.

## Required Mainnet Values

- P2P: `19555`
- RPC: `19556`
- Message start: `a4 ac c6 4d`
- Genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- yespower personalization: `LegacyCoinPoW`

Verify:

```powershell
.\legacycoind.exe params
```

```bash
./legacycoind params
```

## getblocktemplate Flow

1. Pool calls `getblocktemplate`.
2. Pool builds candidate coinbase paying a pool-controlled Legacy Coin address.
3. Pool builds header and merkle root from template transactions.
4. Workers search nonce/extra-nonce using yespower with personalization `LegacyCoinPoW`.
5. Pool serializes the full block and calls `submitblock`.
6. Pool confirms acceptance by checking height/hash with `getblockhash` and `getblock`.

Example:

```powershell
.\legacycoin-cli.exe getblocktemplate
.\legacycoin-cli.exe submitblock <block_hex>
```

```bash
./legacycoin-cli getblocktemplate
./legacycoin-cli submitblock <block_hex>
```

## Header and Hash Expectations

Legacy Coin block identity uses the chain's yespower header hash. Do not use wire-header SHA256d as the block id for pool accounting, P2P inventory, or submit confirmation.

## Coinbase and Reward Notes

- Coinbase maturity: 100 blocks.
- Pool reward address must be a valid Legacy Coin mainnet address.
- Coinbase value must not exceed subsidy plus fees.
- Pool payout transactions must obey normal transaction and mempool policy.

## Difficulty and Target Notes

Use `bits` and `target` from `getblocktemplate`. DGW/difficulty rules are consensus and must not be changed by pool software.

## Example Pool Operator Checklist

- Can pool call `getblocktemplate`? `yes, implemented`
- Can pool submitblock? `yes, implemented`
- Can pool validate payout address? `yes, validateaddress implemented`
- Can pool read chain height/hash? `yes, getblockcount/getblockhash implemented`
- Can pool detect reorg? `partial, use height/hash scanning and getsyncstatus`
- Can pool confirm submitted block? `yes, getblock/getblockhash`

## Security

- Put pool-to-node RPC on localhost or private network only.
- Use firewall rules so public workers never reach RPC.
- Keep wallet keys outside worker machines.
- Back up pool wallet data before mining payouts.
- Monitor `checkstorage`, `getblockchaininfo`, `getsyncstatus`, and `getpeerinfo`.

## Known Limitations

- External stratum pool testing is still required.
- No built-in stratum server.
- Full txindex/address index is planned, not implemented.
