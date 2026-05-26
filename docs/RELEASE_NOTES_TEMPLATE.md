# Legacy Core Release Notes Template

## Version

`vX.Y.Z`

## Commit

`<commit-hash>`

## Mainnet Identity

- Coin: Legacy Coin / LBTC
- Message start: `a4 ac c6 4d`
- Genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- Genesis time: `1779235200`
- Genesis nonce: `3`
- P2P port: `19555`
- RPC port: `19556`
- yespower personalization: `LegacyCoinPoW`

## Assets

| Asset | SHA256 |
| --- | --- |
| `LegacyCore-...` | `<sha256>` |

## Upgrade Notes

- Back up the wallet directory before upgrading.
- Stop `legacycoind` or Legacy Wallet cleanly before replacing binaries.
- Keep the existing data directory and wallet files unless a release note explicitly says otherwise.

## Known Limitations

- `<document limitations honestly>`

## Safety Warnings

- RPC port `19556` must stay private/firewalled.
- Never expose wallet RPC publicly.
- Verify SHA256 checksums before running downloaded binaries.
- Windows builds may show SmartScreen warnings if unsigned.

## Consensus Statement

No consensus, genesis, chain ID, message start, yespower params, DGW, ports, address/WIF formats, wallet DB, reward/supply, coinbase maturity, transaction validation consensus rules, or P2P identity were changed.
