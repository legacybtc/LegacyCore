# Mining

## Mainnet Mining Identity

- PoW: yespower, personalization `LegacyCoinPoW`
- Target spacing: 10 minutes
- Coinbase maturity: 100 blocks
- Initial subsidy: 50 LBTC
- Halving interval: 210,000 blocks

## Solo Mining

```bash
./legacycoind run
./legacycoin-cli setupwallet "strong passphrase"
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
./legacycoin-cli stopminer
```

## Safety Checks

```bash
./legacycoin-cli checkstorage
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
```

## Pool RPC

- `getblocktemplate`, `submitblock`, `submitblockdebug`, `validateblockproposal`: available
- Legacy Core does not ship a built-in stratum server
