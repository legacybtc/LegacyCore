# Security Policy

Legacy Core is release-candidate software for the Legacy Coin / LBTC network.
Use careful operational security, verify checksums, back up wallets/private
keys, and do not expose wallet RPC publicly.

## Supported branches

| Branch | Status |
|---|---|
| `main` | Security fixes and release candidates |
| release tags | Security fixes for published releases |

## Reporting vulnerabilities

Please report security issues privately to the project maintainer. Include:

- affected commit or release tag
- clear reproduction steps
- expected vs actual behavior
- whether funds, consensus, RPC credentials, wallet keys, or node availability are affected
- suggested patch, if available

Do not publish a working exploit before maintainers have had a reasonable
opportunity to fix and ship a patch.

## Severity guide

| Severity | Examples |
|---|---|
| Critical | consensus split, remote wallet-key extraction, unauthenticated fund movement, deterministic key compromise |
| High | remote crash/DoS, RPC auth bypass, block validation bypass, mempool exhaustion from unauthenticated peers |
| Medium | local privilege issues, corruptible chainstate recovery failure, sensitive-data logging |
| Low | hardening gaps, docs mistakes, non-sensitive information disclosure |

## Required release security checks

Before public release:

```bash
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 go vet ./...
staticcheck ./...
govulncheck ./...
gosec ./...
```

Production binaries that validate/mine/submit blocks must report:

```text
yespower backend: cgo-c-reference
```

Publish signed source or verifiable source archives and SHA256 checksums for
all binaries.
