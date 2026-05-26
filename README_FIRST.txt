Legacy Core / Legacy Wallet Quick Start

IMPORTANT SECURITY WARNINGS

- RPC port 19556 must stay private/firewalled.
- P2P port 19555 may be public.
- Never expose wallet/RPC publicly.
- Back up wallet data before use, mining, imports, or upgrades.
- Never share wallet.dat, private keys, seed material, or RPC cookies.
- Verify SHA256 checksums before running downloaded assets.
- Unsigned Windows builds may trigger SmartScreen.
- Legacy Core is early mainnet software; test with small amounts first.

WINDOWS WALLET

1. Extract the release ZIP.
2. Run START_HERE.bat from the extracted folder.
3. Wait for the local node to start and connect peers.
4. Create a receive address.
5. Back up wallet data before receiving meaningful funds.

WINDOWS CLI

Open PowerShell in the extracted folder:

  .\legacycoind.exe params
  .\legacycoind.exe run -seed-peers

In another PowerShell:

  .\legacycoin-cli.exe getblockchaininfo
  .\legacycoin-cli.exe getnetworkinfo
  .\legacycoin-cli.exe getpeerinfo
  .\legacycoin-cli.exe checkstorage

LINUX NODE

  chmod +x legacycoind legacycoin-cli
  ./legacycoind params
  ./legacycoind run -seed-peers

In another terminal:

  ./legacycoin-cli getblockchaininfo
  ./legacycoin-cli getnetworkinfo
  ./legacycoin-cli getpeerinfo
  ./legacycoin-cli checkstorage

MAINNET IDENTITY

- Coin: Legacy Coin / LBTC
- Message start: a4 ac c6 4d
- Genesis hash: 5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5
- Genesis time: 1779235200
- Genesis nonce: 3
- P2P port: 19555
- RPC port: 19556
- yespower personalization: LegacyCoinPoW

Read the full documentation in the docs directory before operating a pool,
exchange wallet, explorer, seed node, or production miner.
