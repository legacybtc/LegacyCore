# Mainnet Launch Checklist

This document records v1.0.3 integration readiness checks. It does not change mainnet identity.

## Immutable Identity

- Coin: Legacy Coin / LBTC
- Message start: `a4 ac c6 4d`
- Genesis hash: `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- Genesis time: `1779235200`
- Genesis nonce: `3`
- P2P port: `19555`
- RPC port: `19556`
- yespower personalization: `LegacyCoinPoW`
- Data directory on Linux: `~/.legacycoin`
- DNS seeds: `legacycoinseed.space`, `legacycoinseed2.space`

## Preflight

Windows:

```powershell
.\legacycoind.exe params
.\legacycoin-cli.exe doctor
.\legacycoin-cli.exe checkstorage
.\legacycoin-cli.exe getblockchaininfo
```

Linux:

```bash
./legacycoind params
./legacycoin-cli doctor
./legacycoin-cli checkstorage
./legacycoin-cli getblockchaininfo
```

## Launch Roles

- Seed operators: run public P2P, private RPC.
- Miners/pools: verify yespower backend and template/submit flow.
- Exchanges: scan blocks by height/hash and maintain own deposit index.
- Explorers: build external tx/address index until native indexes exist.
- Wallet users: back up before receiving or mining funds.

## Release Gates

- `npm run build` in wallet frontend.
- `go test ./...`
- `go vet ./...`
- Build daemon, CLI, and wallet internal binary.
- Run mainnet identity verification script.
- Run source cleanliness scan.
- Remove generated binaries before committing source.

## Known Launch Limitations

- Native address index: `planned`.
- Native full txindex: `planned`.
- Reindex command: `planned`.
- External pool/exchange/explorer certification: still required.
- Fork choice audit found height-based side-branch activation; chainwork-based fork choice should be staged next.
