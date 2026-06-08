Legacy Wallet / Legacy Core Quick Start (v1.0.5)

Windows:
1) Extract this package to a normal folder (not Program Files).
2) Start LegacyWallet.exe (or START_HERE.bat if provided).
3) In a terminal from this folder:
   legacycoind.exe params
   legacycoind.exe run -seed-peers

Second terminal:
  legacycoin-cli.exe getblockchaininfo
  legacycoin-cli.exe getpeerinfo
  legacycoin-cli.exe getwalletinfo
  legacycoin-cli.exe getbalance
  legacycoin-cli.exe getwalletsummary
  legacycoin-cli.exe listtransactions

Miner checks:
  legacycoin-cli.exe setminerthreads 4
  legacycoin-cli.exe getminerstatus

If moving wallet files to another Windows PC:
1) Stop legacycoind first.
2) Copy wallet file(s) into the LegacyCoin data directory.
3) Start node and wait for sync.
4) Re-check with getwalletinfo/getbalance/listtransactions.
5) If balance is still missing, verify correct data directory and chain sync height.

LegacyCoin data directory (Windows):
  %APPDATA%\LegacyCoin

Security:
- P2P port 19555 may be public.
- RPC port 19556 must stay private.
- Do not expose wallet or RPC to public internet.
- Verify SHA256 checksums before running binaries.
