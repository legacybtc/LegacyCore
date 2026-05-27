# Legacy Core Release Notes Template

## Version

- Version: `vX.Y.Z`
- Commit: `<git-hash>`
- Date (UTC): `YYYY-MM-DD`

## Mainnet Identity

- Message start: `a4 ac c6 4d`
- Genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- Genesis time: `1779235200`
- Genesis nonce: `3`
- P2P/RPC ports: `19555 / 19556`
- yespower personalization: `LegacyCoinPoW`

## Assets

| Platform | Architecture | Archive | SHA256 |
| --- | --- | --- | --- |
| Windows | amd64 | `LegacyWallet-LBTC-mainnet-windows-amd64-vX.Y.Z.zip` | `<sha256>` |
| Linux | amd64 | `LegacyCore-LBTC-mainnet-linux-amd64-vX.Y.Z.tar.gz` | `<sha256>` |
| Linux | arm64 | `LegacyCore-LBTC-mainnet-linux-arm64-vX.Y.Z.tar.gz` | `<sha256 or N/A>` |
| macOS | amd64 | `LegacyCore-LBTC-mainnet-macos-amd64-vX.Y.Z.tar.gz` | `<sha256 or N/A>` |
| macOS | arm64 | `LegacyCore-LBTC-mainnet-macos-arm64-vX.Y.Z.tar.gz` | `<sha256 or N/A>` |
| Source | clean | `LegacyCore-vX.Y.Z-source-clean.zip` | `<sha256>` |

## Installation / Quick Start

- Windows wallet: extract ZIP and run `START_HERE.bat`.
- Linux/macOS daemon: extract tar, `chmod +x legacycoind legacycoin-cli`, run `./legacycoind run -seed-peers`.

## Upgrade Notes

- `<notable migrations>`
- `<required config changes>`
- `<compatibility notes>`

## Security Warnings

- Keep RPC private/firewalled.
- Verify checksums before execution.
- Back up wallet data before upgrade.

## Known Limitations

- `<list real limitations>`

## No-Consensus-Change Statement

This release does not change consensus, genesis, chain ID, message start, PoW parameters, difficulty rules, ports, address/WIF formats, wallet DB compatibility, reward/supply schedule, coinbase maturity, transaction validation consensus rules, or P2P identity.
