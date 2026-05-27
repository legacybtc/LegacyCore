# Exchange Integration

Purpose: practical guide for exchange deposits/withdrawals with Legacy Core.  
Audience: exchange backend and operations teams.  
Status: integration-ready candidate for v1.0.4; production validation remains your responsibility.  
Safety warning: RPC (`19556`) must stay private.

## Daemon Setup

Windows:

```powershell
.\legacycoind.exe run
.\legacycoin-cli.exe getblockchaininfo
```

Linux:

```bash
./legacycoind run
./legacycoin-cli getblockchaininfo
```

## RPC Private Warning

- Keep RPC on localhost or private network.
- Use cookie auth or strong `rpcuser`/`rpcpassword`.
- Never expose wallet RPC directly to public internet.

## Deposit Scanning

Use height/hash scanning:

```bash
./legacycoin-cli getblockcount
./legacycoin-cli getblockhash <height>
./legacycoin-cli getblock <hash>
```

Validate addresses before use:

```bash
./legacycoin-cli validateaddress <address>
```

## Confirmation Policy

- Standard deposits: 30 confirmations.
- Large deposits: 100 confirmations/manual review.
- Coinbase-origin funds: 100 confirmations (maturity rule).

## txindex and addressindex Recommendations

- Set `txindex=1` for reliable historical tx lookup.
- Set `addressindex=1` if you want native address RPC helpers.
- After enabling either on existing data, run reindex/repair.

## Withdrawals

Example:

```bash
./legacycoin-cli sendtoaddress <address> 1.25 --yes
./legacycoin-cli sendmany "" "{\"<addr1>\":1.0,\"<addr2>\":2.5}"
```

## Backupwallet

Back up before production use and before upgrades:

```bash
./legacycoin-cli backupwallet /secure/path/legacy-wallet-backup.json
```

## Reorg Monitoring

Track `(height, hash)` over credited range and roll back credits if a stored hash changes.

Also monitor:

```bash
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
./legacycoin-cli checkstorage
```

## Hot Wallet Risk

- Keep minimal hot balance.
- Use cold wallet process for reserves.
- Restrict operator access and credentials.

## Known Limitations

- Integration scripts and RPC are strong, but external production certification remains required.
- Address index is a foundation-level implementation, not a full exchange accounting system.
