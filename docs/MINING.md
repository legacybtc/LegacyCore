# Mining

Legacy Core includes a local CPU miner and exposes pool-facing block template RPCs. Status: local solo mining is `implemented`; external pool integration is an `integration-ready candidate` and still requires real pool testing.

## Mainnet Mining Identity

- Algorithm: yespower
- Personalization: `LegacyCoinPoW`
- Block target spacing: 10 minutes
- Coinbase maturity: 100 blocks
- Reward schedule: 50 LBTC initial subsidy, halving every 210,000 blocks
- P2P port: `19555`
- RPC port: `19556`

Do not change yespower parameters, DGW/difficulty rules, genesis, ports, address formats, or reward rules for mining integration work.

## Solo Mining Setup

Windows:

```powershell
.\legacycoind.exe run
.\legacycoin-cli.exe setupwallet "strong passphrase"
.\legacycoin-cli.exe getminingaddress
.\legacycoin-cli.exe setminerthreads 4
.\legacycoin-cli.exe startminer
.\legacycoin-cli.exe getminerstatus
.\legacycoin-cli.exe stopminer
```

Linux:

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

Run:

```powershell
.\legacycoind.exe params
.\legacycoin-cli.exe checkstorage
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getsyncstatus
```

Linux:

```bash
./legacycoind params
./legacycoin-cli checkstorage
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
```

The production yespower backend should report `cgo-c-reference`. The pure-Go backend is for development/testing unless parity is explicitly validated for a release.

## Miner RPC Status

- `getminingaddress`: `implemented`
- `startminer`: `implemented`
- `stopminer`: `implemented`
- `restartminer`: `implemented`
- `setminerthreads`: `implemented`
- `getminerstatus`: `implemented`
- `getblocktemplate`: `implemented`
- `submitblock`: `implemented`
- Stratum server: `not implemented`

## Limitations

- Built-in mining is CPU solo mining, not a pool coordinator.
- External stratum pool configuration must be tested by operators before production use.
- Fee/min-relay policy is conservative and may be refined in later releases.
