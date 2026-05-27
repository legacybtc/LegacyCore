# Windows Service Notes

Legacy Core can run headless on Windows through service wrappers (for example NSSM, WinSW, or Task Scheduler startup tasks).

## Recommended Baseline

- Run `legacycoind.exe run -seed-peers`
- Keep RPC private (`127.0.0.1`) unless explicitly secured
- Use a dedicated data directory and service account where possible

## Operational Checks

```powershell
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe checkstorage
```

## Restart/Upgrade Strategy

1. Stop service cleanly.
2. Back up wallet data.
3. Replace binaries.
4. Start service.
5. Verify with CLI health commands.

Do not expose RPC directly to public internet.
