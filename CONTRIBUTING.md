# Contributing to Legacy Core

## Development Environment

- Go 1.22+ (https://go.dev/dl/)
- Node.js LTS + npm
- MSYS2 + UCRT64 GCC (Windows) for CGO cross-compilation
- Wails CLI for desktop wallet builds

## Code Style

- **Go**: Standard `gofmt` formatting. Run `go vet ./...` before committing. Follow idiomatic Go conventions.
- **TypeScript/React**: Standard TypeScript with ES2020 modules. Use functional components and hooks.
- **C/DLLs**: MSYS2 UCRT64 GCC, CGO-compatible.
- **Shell scripts**: Bash with `set -euo pipefail`; PowerShell with `$ErrorActionPreference = "Stop"`.

## Pull Request Process

1. Fork the repository and create a feature branch from `main`.
2. Run tests locally before pushing:
   ```bash
   go test ./...
   cd cmd/legacywallet/frontend && npm run test:dashboard
   ```
3. Keep changes focused — one logical change per PR.
4. All PRs must pass CI (Go tests, frontend tests, dashboard tests).
5. Squash commits on merge.

## Testing

- Go unit tests: `go test -short ./...`
- Dashboard logic: `cd cmd/legacywallet/frontend && npm run test:dashboard`
- Frontend build: `cd cmd/legacywallet/frontend && npm run build`

## Security

- Report vulnerabilities privately via [GitHub Security Advisories](https://github.com/legacybtc/LegacyCore/security/advisories/new).
- Do not commit secrets, RPC credentials, wallet backups, or private keys.

## Versioning

This project follows SemVer. Version constants are in `internal/version/version.go`. When bumping the version, update all references across source code, scripts, and documentation.
