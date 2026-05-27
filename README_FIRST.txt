Legacy Wallet / Legacy Core Quick Start (v1.0.4)

Windows:
1) Extract this package to a normal folder (not Program Files).
2) Run START_HERE.bat.
3) Wait for internal node startup, then check status in wallet diagnostics.

Headless commands from the same folder:
  legacycoind.exe params
  legacycoind.exe run -seed-peers

Second terminal:
  legacycoin-cli.exe getblockchaininfo
  legacycoin-cli.exe getpeerinfo
  legacycoin-cli.exe getminerstatus

Security:
- P2P port 19555 may be public.
- RPC port 19556 must stay private.
- Do not expose wallet or RPC to public internet.
- Verify SHA256 checksums before running binaries.
