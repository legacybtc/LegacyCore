Legacy Wallet (Legacy Coin / LBTC)
==================================

Legacy Wallet is the official full-node desktop GUI for Legacy Coin mainnet.
It embeds Legacy Core, manages your local wallet, and connects to the LBTC
peer network using CPU-friendly yespower Proof-of-Work (LegacyCoinPoW).

Launch
------
Double-click LegacyWallet.exe (or run from the extracted release folder).
On first run, create or import a wallet, then use the toolbar to start the
internal node if it is not already running.

Node connection
---------------
By default Legacy Wallet starts and stops an internal Legacy Core node in your
data directory. Local RPC listens on 127.0.0.1:19556 for this wallet only.
P2P uses port 19555. Do not expose RPC to the internet.

Data directory
----------------
Default (Windows): %APPDATA%\LegacyCoin
Wallet settings may point to a custom folder (see Settings tab).

Keep RPC private
----------------
- Never forward port 19556 through your router.
- Do not share legacycoin.conf or .cookie files.
- The wallet does not store your RPC password in the web UI source.

Backup
------
Use Wallet Security → backup, or legacycoin-cli backupwallet, before major
changes. Store backups offline. Never share backup files, seeds, or private keys.

Troubleshooting (node offline)
------------------------------
1. Click Start Node on the toolbar.
2. Check that nothing else is using RPC 127.0.0.1:19556.
3. Allow Legacy Wallet through Windows Firewall for P2P (19555).
4. Use Network / Peers → reconnect seeds if peer count stays zero.

Verify mainnet identity
-----------------------
Help → About shows genesis hash, ports, and PoW parameters. You can also run
legacycoind params and scripts\verify-mainnet-identity.ps1 from a full Core
install.

Security warnings
-----------------
- Legacy Wallet never logs passphrases, seeds, WIF, or private keys.
- Verify receive addresses on screen before sharing.
- User-created tokens and third-party services are not endorsed by Legacy Coin.

Version: Legacy Core / Legacy Wallet 1.0.4 (LBTC mainnet)
