# Troubleshooting

## RPC Cookie Not Found

Start the daemon first or pass explicit RPC credentials.

Windows:

```powershell
.\legacycoind.exe run
.\legacycoin-cli.exe getnetworkinfo
```

Linux:

```bash
./legacycoind run
./legacycoin-cli getnetworkinfo
```

## RPC Unauthorized

Check `rpcuser` and `rpcpassword`, or ensure the CLI uses the same data directory as the daemon.

```powershell
.\legacycoin-cli.exe -rpcuser=legacyrpc -rpcpassword=change_this getnetworkinfo
```

```bash
./legacycoin-cli -rpcuser=legacyrpc -rpcpassword=change_this getnetworkinfo
```

## RPC Port Conflict

Only one daemon can bind RPC `19556` on the same interface.

```powershell
.\legacycoin-cli.exe stop
```

```bash
./legacycoin-cli stop
```

Then confirm no service manager immediately restarted another copy.

## Node Is Behind

Check sync:

```powershell
.\legacycoin-cli.exe getsyncstatus
.\legacycoin-cli.exe getpeerinfo
```

```bash
./legacycoin-cli getsyncstatus
./legacycoin-cli getpeerinfo
```

Watch `catch_up_pending`, `peer_reported_height`, `last_sync_attempt`, `last_sync_error`, and stale peer counts.

## Storage Error

Run:

```powershell
.\legacycoin-cli.exe checkstorage
```

```bash
./legacycoin-cli checkstorage
```

Repair path:

```powershell
.\legacycoin-cli.exe reindex
```

```bash
./legacycoin-cli reindex
```

Or:

```powershell
.\legacycoin-cli.exe checkstorage true
```

```bash
./legacycoin-cli checkstorage true
```

## Miner Will Not Start

Run:

```powershell
.\legacycoin-cli.exe getminingaddress
.\legacycoin-cli.exe checkstorage
.\legacycoin-cli.exe getminerstatus
```

```bash
./legacycoin-cli getminingaddress
./legacycoin-cli checkstorage
./legacycoin-cli getminerstatus
```

Common causes:

- No mining pubkey hash configured.
- Wallet is encrypted and locked.
- Storage health failed.
- `peer_required` is true and no peers are connected.

## Manual GUI Lifecycle Checklist

Status: manual checklist. Use this before signing a wallet release or accepting a GUI runtime change.

- Start Mining: miner starts, `getminerstatus` reports `active_mining=true`, UI state matches RPC.
- Stop Mining: miner stops, `getminerstatus` reports `active_mining=false`.
- Force Stop Miner: repeated stop is safe and idempotent.
- Restart Internal Node: node stops and starts without losing wallet state.
- Stop Internal Node: RPC port `19556` and P2P port `19555` close after shutdown.
- Open Lifecycle Log: log opens and contains start/stop/miner events.
- Copy Diagnostics Report: report includes node, RPC, wallet, storage, and miner status without secrets.
- Close Application: no orphan node or miner process remains.

## Windows SmartScreen

Unsigned Windows builds may trigger SmartScreen. Verify SHA256 checksums before running any binary.

## Early Mainnet Warning

Legacy Core is early mainnet software. Back up wallets, verify checksums, keep RPC private, and test operations with small amounts first.
