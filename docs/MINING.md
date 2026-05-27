# Mining

Purpose: solo mining and mining RPC usage guide.  
Audience: miners and node operators.  
Status: active for v1.0.4.  
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
- `validateaddress`: available

Legacy Core does not ship a built-in stratum server in v1.0.4.

## Troubleshooting

- mining idle: check wallet lock, peers, storage health, sync state
- rejected blocks: verify template freshness and sync health

## Known Limitations

- Built-in miner is local CPU mining, not full pool orchestration.
- External pool deployment requires separate stratum/payout infrastructure.
