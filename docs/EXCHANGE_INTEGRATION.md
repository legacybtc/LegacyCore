# Legacy Exchange Integration (RC2)

## 1) Operating model

- Run your own `legacycoind`.
- Keep wallet keys and backups under your custody.
- Keep RPC local/private only.

## 2) RPC security baseline

- Bind RPC to loopback/private interface only.
- Use cookie auth or explicit `rpcuser` + `rpcpassword`.
- Do not expose `19556` to the public internet.

## 3) Core deposit/withdraw flow

1. Generate deposit address: `getnewaddress`
2. Track deposits:
   - `getrawmempool`
   - `getblockcount`
   - `getbestblockhash`
   - `getblockhash` + `getblock`
3. Confirm credits by block confirmations.
4. Build withdrawals with wallet RPC (`sendtoaddress`) or raw TX flow (`sendrawtransaction`).

## 4) Confirmation policy

- Choose conservative confirmation thresholds for deposits.
- Coinbase maturity is 100 blocks.
- Handle reorg risk in crediting logic.

## 5) Useful RPCs for exchanges

- `getnetworkinfo`
- `getpeerinfo`
- `getsyncstatus`
- `getmempoolinfo`
- `getrawmempool`
- `getblockcount`
- `getbestblockhash`
- `getblockhash`
- `getblock`
- `gettxout`
- `listunspent`
- `getbalance`
- `sendrawtransaction`

See `docs/RPC_COMPATIBILITY.md` for status details.

## 6) Backups and recovery

- Back up wallet data before production.
- Test restore procedure before go-live.
- Never log or export private keys to shared logs.
