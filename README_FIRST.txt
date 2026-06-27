Legacy Core / Legacy Wallet - Windows Release

1. Extract the ZIP archive to a normal folder.
2. Double-click START_HERE.bat to launch Legacy Wallet.
3. Keep all files in this folder together. The wallet needs the bundled
   legacycoind.exe, legacycoin-cli.exe, and runtime DLL files.

First launch notes:

- Windows may ask for firewall permission so the node can accept peer
  connections on port 19555.
- The wallet data directory is %APPDATA%\LegacyCoin.
- The RPC port is 19556 and should stay private.

Before running binaries from a downloaded release, compare the files against
SHA256SUMS.txt. Do not use a package if checksums do not match.

Security reminders:

- Back up your wallet before sending or mining funds.
- Never share wallet backups, private keys, seed phrases, or RPC credentials.
- Only download releases from the official LegacyCore GitHub repository.
