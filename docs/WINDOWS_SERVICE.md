# Windows Service Notes

Purpose: run Legacy Core daemon in managed Windows service environments.  
Audience: Windows node operators.  
Status: active for v1.0.4.  
Safety warning: RPC port `19556` must remain private.

## What This Is

Operational notes for wrapping `legacycoind.exe` in a service manager (for example NSSM/WinSW/Task Scheduler).

## Baseline Command

```powershell
.\legacycoind.exe run -seed-peers
```

## Health Checks

```powershell
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe checkstorage
```

## Upgrade Procedure

1. Stop service cleanly.
2. Back up wallet/config.
3. Replace binaries.
4. Start service.
5. Verify identity and health commands.

## Known Limitations

- This repository provides guidance and examples, not a single mandatory Windows service wrapper.
