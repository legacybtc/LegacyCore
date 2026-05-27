# Pool Integration

Purpose: integrate pool software with Legacy Core mining RPCs.  
Audience: mining pool operators and pool developers.  
Status: integration-ready candidate for v1.0.4.  
Safety warning: keep RPC private; never expose node wallet RPC to public workers.

## What This Is

Legacy Core provides pool-critical RPC methods:

- `getblocktemplate`
- `submitblock`
- `validateaddress`
- chain/sync/network/mempool RPCs

Legacy Core does **not** include a built-in stratum server.

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
5. Confirm acceptance with `getblock`/`getblockhash`.

## submitblock Behavior

- Success: `null` result.
- Rejections: structured reject strings or decode errors.

## Reward and Maturity

- Subsidy schedule remains chain consensus.
- Coinbase maturity is 100 blocks.
- Pool payout logic must account for maturity.

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
