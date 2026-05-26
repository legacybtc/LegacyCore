# Release Process

Status: v1.0.3 adds release hygiene scripts and CI gates; signing/notarization is `planned`.

Do not zip the whole working directory. Clean source archives must come from Git metadata with `git archive`.

## Required Tools

- Go matching `go.mod`
- Node.js 20 for wallet frontend
- npm
- Git
- Windows: MSYS2 UCRT64 GCC for production yespower CGO builds
- Linux: GCC and standard archive tools

## Validation Commands

Windows:

```powershell
cd cmd\legacywallet\frontend
npm install
npm run build
cd ..\..\..

go test ./...
go vet ./...

go build -trimpath -o legacycoind.exe ./cmd/legacycoind
go build -trimpath -o legacycoin-cli.exe ./cmd/legacycoin-cli
go build -trimpath -o legacy-wallet-internal.exe ./cmd/legacywallet

.\legacycoind.exe params
.\scripts\verify-mainnet-identity.ps1 -Binary .\legacycoind.exe
.\scripts\scan-source-cleanliness.ps1 -Root .
```

Linux:

```bash
cd cmd/legacywallet/frontend
npm install
npm run build
cd ../../..

go test ./...
go vet ./...

go build -trimpath -o legacycoind ./cmd/legacycoind
go build -trimpath -o legacycoin-cli ./cmd/legacycoin-cli
go build -trimpath -o legacy-wallet-internal ./cmd/legacywallet

./legacycoind params
pwsh ./scripts/verify-mainnet-identity.ps1 -Binary ./legacycoind
pwsh ./scripts/scan-source-cleanliness.ps1 -Root .
```

Remove generated binaries and frontend output before committing source.

## Clean Source Archive

```powershell
.\scripts\release-source-archive.ps1 -Version v1.0.3
```

The script uses `git archive` and writes `SHA256SUMS-source.txt`.

Clean source archives must exclude:

- `.git`
- Go caches and temp dirs
- `node_modules`
- `dist`
- release packages
- generated binaries
- wallet files and RPC cookies
- local configs
- logs and temporary files

## Windows Package

```powershell
.\scripts\package-windows.ps1 -Version v1.0.3
```

Expected output:

- `LegacyWallet-LBTC-mainnet-windows-amd64-v1.0.3.zip`
- package-level `SHA256SUMS.txt`

Unsigned Windows binaries may trigger SmartScreen; document this in release notes.

## Linux Package

```bash
bash scripts/package-linux.sh v1.0.3
```

Expected output:

- `LegacyCore-LBTC-mainnet-linux-amd64-v1.0.3.tar.gz`
- package-level `SHA256SUMS.txt`

## Mainnet Identity Gate

Every release must verify:

- message start `a4 ac c6 4d`
- genesis hash `5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5`
- genesis time `1779235200`
- genesis nonce `3`
- yespower personalization `LegacyCoinPoW`
- ports `19555` and `19556`

## Release Notes

Use `docs/RELEASE_NOTES_TEMPLATE.md`. Include:

- version
- commit hash
- assets
- SHA256
- upgrade notes
- known limitations
- RPC safety warning
- wallet backup warning
- no consensus changes statement
