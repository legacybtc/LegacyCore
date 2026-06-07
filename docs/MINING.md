# Mining

Purpose: solo mining and mining RPC usage guide.  
Audience: miners and node operators.  
Status: active for v1.0.5.
Safety warning: back up wallet before mining with real funds.

## Mainnet Mining Identity

- PoW: yespower
- Personalization: `LegacyCoinPoW`
- Target spacing: 10 minutes
- Coinbase maturity: 100 blocks
- Initial subsidy: 50 LBTC
- Halving interval: 210,000 blocks

## Solo Mining Quick Start

```bash
./legacycoind run
./legacycoin-cli setupwallet "strong passphrase"
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
```

Stop:

```bash
./legacycoin-cli stopminer
```

## Safety Checks

```bash
./legacycoind params
./legacycoin-cli checkstorage
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
```

## Pool-Relevant RPC

- `getblocktemplate`: available
- `submitblock`: available
- `submitblockdebug`: available for detailed rejection diagnostics
- `validateblockproposal` / `testblock`: available for non-mutating candidate preflight
- `validateaddress`: available

Examples:

```bash
./legacycoin-cli getblocktemplate
./legacycoin-cli validateblockproposal <block_hex>
./legacycoin-cli submitblockdebug <block_hex>
```

```powershell
.\legacycoin-cli.exe getblocktemplate
.\legacycoin-cli.exe validateblockproposal <block_hex>
.\legacycoin-cli.exe submitblockdebug <block_hex>
```

Legacy Core does not ship a built-in stratum server in v1.0.5.

## Coinbase Split Policy

- Consensus allows one or more coinbase outputs.
- Built-in solo mining still creates a single-output coinbase by default.
- `mining.NewCoinbaseTxWithOutputs` is available for official bridge/pool tooling that needs explicit reward splits.
- Official operating policy can use 96% miner / 4% project for solo bridge blocks or 96% miner / 2% pool infrastructure / 2% project for pool blocks.
- A block is valid only if total coinbase output value is within subsidy plus fees.

## Troubleshooting

- mining idle: check wallet lock, peers, storage health, sync state
- rejected blocks: verify template freshness and sync health

## Known Limitations

- Built-in miner is local CPU mining, not full pool orchestration.
- External pool deployment requires separate stratum/payout infrastructure.
