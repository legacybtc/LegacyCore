# Pool Integration

Purpose: integrate pool software with Legacy Core mining RPCs.  
Audience: mining pool operators and pool developers.  
Status: integration-ready candidate for v1.0.5.
Safety warning: keep RPC private; never expose node wallet RPC to public workers.

## What This Is

Legacy Core provides pool-critical RPC methods:

- `getblocktemplate`
- `submitblock`
- `validateaddress`
- chain/sync/network/mempool RPCs

Legacy Core does **not** include a built-in stratum server.

## Pool Node Start

Run the pool's backing full node with normal mainnet P2P and private RPC:

```bash
./legacycoind run -seed-peers
./legacycoin-cli getblocktemplate
```

Windows:

```powershell
.\legacycoind.exe run -seed-peers
.\legacycoin-cli.exe getblocktemplate
```

Keep RPC on localhost/private networking and point pool middleware at RPC port `19556`; expose pool worker ports from the pool software, not from Legacy Core RPC.

## yespower / Chain Identity

- PoW: yespower
- Personalization: `LegacyCoinPoW`
- P2P/RPC ports: `19555` / `19556`

Verify:

```bash
./legacycoind params
```

## getblocktemplate Flow

1. Call `getblocktemplate`.
2. Build pool coinbase and merkle root.
3. Mine candidate with yespower (`LegacyCoinPoW` personalization).
4. Submit full block using `submitblock`.
5. If rejected, call `submitblockdebug` or `validateblockproposal` with the same block hex.
6. Confirm acceptance with `getblock`/`getblockhash`.

Pool-facing template details:

- `submitold=false`
- `expires=15`
- `longpollid=<tiphash>:<mempoolcount>`
- Tip changes update `previoushash`, `previousblockhash`, `height`, and `longpollid`.

## submitblock Behavior

- Success: `null` result.
- Rejections: BIP-style reject strings such as `bad-prevblk`, `bad-txnmrklroot`, `bad-diffbits`, `high-hash`, `duplicate`, or `inconclusive`.
- `submitblockdebug <block_hex>` submits and returns diagnostics including submitted hash, prevhash, inferred height, daemon tip, `ProcessBlockWithResult`, exact reject reason, and reject category.
- `validateblockproposal <block_hex>` and `testblock <block_hex>` preflight the block without storing it or changing tip/orphan/side-chain state.
- Dry-run proposal validation performs the same active-tip block checks as submission, including prevhash, merkle root, bits, timestamp, proof-of-work, coinbase rules, transaction spends, and total coinbase value.
- Use `submitted_prevhash_equals_tip=false` plus `reject_category=bad-prevblk` or `stale` to identify stale jobs.

Useful diagnostics:

```bash
./legacycoin-cli validateblockproposal <block_hex>
./legacycoin-cli submitblockdebug <block_hex>
```

```powershell
.\legacycoin-cli.exe validateblockproposal <block_hex>
.\legacycoin-cli.exe submitblockdebug <block_hex>
```

## Reward and Maturity

- Subsidy schedule remains chain consensus.
- Coinbase maturity is 100 blocks.
- Pool payout logic must account for maturity.
- Consensus accepts multi-output coinbase transactions when the total output value is no more than subsidy plus included fees.
- Official split policy is wallet/pool policy, not a consensus rule. Pools can construct 96/2/2 or 96/4 style coinbase outputs as long as total value and scripts are valid.

## RPC Private Warning

- Bind RPC to localhost/private interfaces.
- Use cookie auth or strong credentials.
- Apply strict firewall rules.

## Pool Smoke Script

Use:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\pool-rpc-smoke.ps1
```

or:

```bash
bash scripts/pool-rpc-smoke.sh
```

## External Certification Status

Third-party production pool certification is still required before public claims of pool production readiness.

## Known Limitations

- No built-in stratum service.
- Optional indexes (`txindex`, `addressindex`) require explicit enablement and rebuild on existing data.
