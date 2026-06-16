Legacy Wallet (LBTC mainnet) - v1.0.6

This package contains the Wails-built desktop wallet executable:
  LegacyWallet.exe

Quick start:
1) Start LegacyWallet.exe
2) Create or import a wallet
3) Start the internal node from the Node controls tab if it is not already running
4) Use the sidebar tabs for Overview, Wallet, Send, Receive, Transactions, Mining, Network, Explorer, and RPC Console

Balance visibility (CLI):
  legacycoin-cli.exe getwalletinfo
  legacycoin-cli.exe getbalance
  legacycoin-cli.exe getwalletsummary
  legacycoin-cli.exe listtransactions
  legacycoin-cli.exe listunspent

Mining thread controls (CLI):
  legacycoin-cli.exe setminerthreads 4
  legacycoin-cli.exe getminerstatus

Wallet migration (Windows):
1) Stop legacycoind before copying wallet files.
2) Copy wallet files into:
   %APPDATA%\LegacyCoin
3) Start node, wait for sync, then check balance commands above.

Important:
- Do not share wallet passphrases, seed phrases, private keys, or RPC credentials.
- Keep RPC port 19556 private.
- Keep backups of wallet data before upgrades.

Mainnet identity:
- Coin: Legacy Coin / LBTC
- P2P port: 19555
- RPC port: 19556
- Message start: a4 ac c6 4d
- yespower personalization: LegacyCoinPoW
