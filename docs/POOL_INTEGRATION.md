# Legacy Pool Integration (RC2)

This guide is for pool software that talks directly to `legacycoind` RPC.

## 1) Node and RPC safety

- Keep RPC private (`127.0.0.1` or private LAN/VPN only).
- Never expose RPC port `19556` publicly.
- Use cookie auth by default or explicit `rpcuser` + `rpcpassword`.

Example private tunnel:

```bash
ssh -N -L 19556:127.0.0.1:19556 user@NODE_IP
```

## 2) Required pool flow

1. `getblocktemplate`
2. Pool builds custom coinbase / extranonce
3. Pool mines header
4. `submitblock`

## 3) `getblocktemplate` shape

RC2 now returns pool-compatible transaction objects:

```json
{
  "height": 123,
  "version": 1,
  "previousblockhash": "....",
  "bits": "1f0fffff",
  "target": "....",
  "coinbasevalue": 5000000000,
  "transactions": [
    {
      "data": "<raw tx hex>",
      "hash": "<txid>",
      "txid": "<txid>",
      "fee": 372,
      "size": 241
    }
  ],
  "transaction_count": 1,
  "txids": ["..."],
  "mempoolsize": 1,
  "hex": "<full template block hex>"
}
```

Notes:

- `transactions` excludes coinbase.
- `coinbasevalue = subsidy + included fees`.
- `txids`, `mempoolsize`, and `hex` remain for compatibility.

## 4) `submitblock` behavior

- Success: `null`
- Failure: rejection reason string (for example duplicate, stale, invalid PoW, bad previous block).

## 5) Curl examples

```bash
curl --user "$RPCUSER:$RPCPASS" \
  --data '{"jsonrpc":"2.0","id":"pool","method":"getblocktemplate","params":[]}' \
  -H 'content-type: application/json' \
  http://127.0.0.1:19556
```

```bash
curl --user "$RPCUSER:$RPCPASS" \
  --data '{"jsonrpc":"2.0","id":"pool","method":"submitblock","params":["<blockhex>"]}' \
  -H 'content-type: application/json' \
  http://127.0.0.1:19556
```

## 6) Network identity checks

Verify `legacycoind params`:

- message start: `a4 ac c6 4d`
- genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- yespower backend: `cgo-c-reference`

