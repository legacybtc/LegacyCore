# Security Policy

Legacy Core is early mainnet software. Treat RPC, wallet storage, private keys, seed material, backups, and release binaries as sensitive.

## Supported Versions

| Version | Status |
| --- | --- |
| v1.0.31 | current release |
| v1.0.21 | unsupported |
| v1.0.12 | unsupported |
| v1.0.10 | unsupported |
| older versions | unsupported |

## Critical Warnings

- RPC port `19556` must stay private/firewalled.
- P2P port `19555` may be public.
- Never expose wallet/RPC publicly.
- Back up wallet data before use, mining, imports, or upgrades.
- Never share wallet.dat, private keys, seed material, wallet backups, or RPC cookies.
- Verify SHA256 checksums before running release assets.
- Unsigned Windows builds may trigger SmartScreen.
- Seed operators should firewall RPC even when P2P is public.
- Exchanges should assume hot wallet compromise risk and keep reserves cold.

## RPC Security

Cookie auth and `rpcuser`/`rpcpassword` auth are implemented. Public unauthenticated non-local RPC is refused. Operators should still keep RPC on localhost or a private network.

## Reporting Vulnerabilities

Please report security issues privately to project maintainers before public disclosure.

**Report via GitHub:** [Submit a security advisory](https://github.com/legacybtc/LegacyCore/security/advisories/new)

Include:

- affected version or commit
- operating system
- whether funds, consensus, RPC credentials, wallet keys, or node availability are affected
- reproduction steps
- logs with secrets removed

Do not include private keys, wallet backups, RPC cookies, passwords, or seed material in reports.

## Scope

High-priority examples:

- consensus validation bypass
- wallet key exposure
- RPC authentication bypass
- remote crash or denial of service
- transaction validation flaw
- P2P issue that can force a bad chain state
- release package path/secret leak

Out of scope:

- public P2P port visibility by itself
- SmartScreen warnings for unsigned binaries
- reports requiring leaked user secrets

## Build Verification

Run:

```powershell
.\legacycoind.exe params
.\scripts\verify-mainnet-identity.ps1 -Binary .\legacycoind.exe
```

```bash
./legacycoind params
pwsh ./scripts/verify-mainnet-identity.ps1 -Binary ./legacycoind
```

Expected production yespower backend:

```text
yespower backend: cgo-c-reference
```

## Consensus Safety

v1.0.9 must not change consensus, genesis, chain ID, message start, yespower params, DGW/difficulty rules, ports, address/WIF formats, wallet DB compatibility, reward/supply schedule, halving interval, coinbase maturity, transaction validation consensus rules, P2P identity, or mainnet identity.
