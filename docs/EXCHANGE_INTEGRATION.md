# Exchange Integration

Status: `integration-ready candidate, exchange production testing still required`.

This guide covers daemon, RPC, wallet, deposits, withdrawals, confirmations, backups, and upgrade safety for exchange operators.

## Daemon Setup

Windows:

```powershell
.\legacycoind.exe params
.\legacycoind.exe run
.\legacycoin-cli.exe getblockchaininfo
```

Linux:

```bash
./legacycoind params
./legacycoind run
./legacycoin-cli getblockchaininfo
```

Use a dedicated data directory and a dedicated OS service account. Keep RPC private.

## Firewall Rules

- P2P `19555`: may be public for node connectivity.
- RPC `19556`: private only, never public.
- Restrict RPC to localhost or a private exchange application subnet.

## RPC Auth

Cookie auth is supported for same-host automation. For exchange daemons, configure `rpcuser` and `rpcpassword` with a long random secret and strict filesystem permissions.

## Deposit Address Generation

Status: wallet deposit address generation is `implemented`.

```powershell
.\legacycoin-cli.exe getnewaddress
.\legacycoin-cli.exe validateaddress <address>
```

```bash
./legacycoin-cli getnewaddress
./legacycoin-cli validateaddress <address>
```

Back up the wallet after provisioning or key import:

```powershell
.\legacycoin-cli.exe backupwallet D:\LegacyBackups\legacy-wallet-backup.json
```

```bash
./legacycoin-cli backupwallet /var/backups/legacycoin/legacy-wallet-backup.json
```

## Block Scanning

Use active-chain height scanning:

```powershell
.\legacycoin-cli.exe getblockcount
.\legacycoin-cli.exe getblockhash 100
.\legacycoin-cli.exe getblock <hash>
```

```bash
./legacycoin-cli getblockcount
./legacycoin-cli getblockhash 100
./legacycoin-cli getblock <hash>
```

Address index is optional (`addressindex=1`). If disabled, exchanges should maintain their own deposit index by scanning blocks and mempool transactions.

## Transaction Lookup

- `getrawtransaction`: `implemented`; with `txindex=1` it supports on-disk historical lookup, otherwise it falls back to active-chain + mempool scan.
- `gettransaction`: `implemented` for wallet-related transactions.
- `gettxout`: `implemented` for UTXO checks.

## Confirmations

Recommended exchange policy for early mainnet:

- Deposits: 30 confirmations minimum for normal deposits.
- Large deposits: 100 confirmations or manual review.
- Coinbase/mined funds: 100 confirmations because coinbase maturity is 100 blocks.

This recommendation is operational policy, not consensus.

## Withdrawal Flow

1. Validate destination with `validateaddress`.
2. Ensure wallet is backed up and unlocked if encrypted.
3. Send with `sendtoaddress` or `sendmany`.
4. Confirm transaction appears in `getrawmempool`.
5. Track confirmations by scanning blocks.

Examples:

```powershell
.\legacycoin-cli.exe validateaddress <address>
.\legacycoin-cli.exe sendtoaddress <address> 1.25 --yes
```

```bash
./legacycoin-cli validateaddress <address>
./legacycoin-cli sendtoaddress <address> 1.25 --yes
```

## Reorg Policy

Track `(height, hash)` for credited deposits. If `getblockhash <height>` changes, treat affected deposits as unconfirmed and rescan from the last common height.

Fork choice is currently height-based in the source audit; explicit chainwork-based fork choice should be a v1.0.4 priority.

## Hot and Cold Wallet Guidance

- Keep hot wallet balances minimal.
- Move reserves to cold storage procedures controlled outside the online daemon.
- Never place private keys or wallet backups on web servers.
- Test wallet restore before accepting production deposits.

## Upgrade Procedure

1. Stop deposits and withdrawals.
2. Back up wallet and config.
3. Record current `getblockchaininfo`, `getblockhash <tipheight>`, and `checkstorage`.
4. Stop daemon cleanly.
5. Replace binaries.
6. Verify SHA256 checksums and `legacycoind params`.
7. Start daemon and rescan from last recorded safe height.

## Monitoring Commands

```powershell
.\legacycoin-cli.exe getblockchaininfo
.\legacycoin-cli.exe getsyncstatus
.\legacycoin-cli.exe getpeerinfo
.\legacycoin-cli.exe checkstorage
.\legacycoin-cli.exe getwalletsummary
.\legacycoin-cli.exe doctor
```

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
./legacycoin-cli checkstorage
./legacycoin-cli getwalletsummary
./legacycoin-cli doctor
```

## Exchange Checklist

- Can exchange generate deposit address? `yes`
- Can exchange validate address? `yes`
- Can exchange scan blocks? `yes, by height/hash`
- Can exchange detect confirmations? `yes, by scanning active chain`
- Can exchange broadcast withdrawal? `yes`
- Can exchange backup wallet? `yes`
- Can exchange monitor reorg risk? `partial, hash-by-height policy required`
- Can exchange run node as service? `partially implemented; operators should supply service wrapper`
