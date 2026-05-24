Legacy Wallet 1.0.1 / Legacy Core 1.0.0

Quick start:
1. Extract this ZIP.
2. Double-click START_HERE.bat.
3. Legacy Wallet opens and starts the local Legacy Core node.
4. Wait for peers and sync status.
5. Use Receive to create/copy a receive address.
6. Use Mining to solo mine from your PC.
7. Back up your wallet before using real funds.

Headless node / CLI miner:
First PowerShell:
  .\legacycoind.exe params
  .\legacycoind.exe run -seed-peers

Second PowerShell:
  .\legacycoin-cli.exe getblockcount
  .\legacycoin-cli.exe getsyncstatus
  .\legacycoin-cli.exe getpeerinfo
  .\legacycoin-cli.exe getnetworkinfo
  .\legacycoin-cli.exe getminingaddress
  .\legacycoin-cli.exe setminerthreads 4
  .\legacycoin-cli.exe startminer
  .\legacycoin-cli.exe getminerstatus
  .\legacycoin-cli.exe stopminer
  .\legacycoin-cli.exe stop

Mining notes:
- This is solo mining unless you configure a pool externally.
- Hashrate does not guarantee a block.
- Mined rewards mature after 100 blocks.
- Open P2P port 19555 if you want to help the network.
- Never expose RPC port 19556 publicly.

Safety:
- Back up wallet files/private keys before using real funds.
- Do not delete wallet files, backups, private keys, or seed phrases.
- This software is early and experimental.
- Windows SmartScreen may appear for unsigned community binaries. Verify SHA256 first.
