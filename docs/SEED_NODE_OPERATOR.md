# Seed Node Operator Guide

Status: public P2P seed operation is `implemented`; DNS seed infrastructure is external.

## Mainnet Identity

- P2P port: `19555`
- RPC port: `19556`
- DNS seeds in source: `legacycoinseed.space`, `legacycoinseed2.space`
- Message start: `a4 ac c6 4d`
- Chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`

## Running a Seed Node

Windows:

```powershell
.\legacycoind.exe params
.\legacycoind.exe run -seed-peers
```

Linux:

```bash
./legacycoind params
./legacycoind run -seed-peers
```

Use a stable host, static IP if possible, reliable disk, and clock synchronization.

## Firewall

- Allow inbound TCP `19555`.
- Block public access to TCP `19556`.
- Do not expose wallet RPC publicly.

## Useful Commands

```powershell
.\legacycoin-cli.exe getnetworkinfo
.\legacycoin-cli.exe getpeerinfo
.\legacycoin-cli.exe getsyncstatus
.\legacycoin-cli.exe checkstorage
```

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
./legacycoin-cli checkstorage
```

## Configuration Example

```text
bind=0.0.0.0
rpcbind=127.0.0.1
seed_peers=true
peer_safety=true
chain_id_enforce=true
peer_ping_interval_seconds=30
pretty_logs=true
log_icons=true
```

## Operations Checklist

- Verify `legacycoind params`.
- Confirm P2P port is reachable from outside.
- Confirm RPC is not reachable from outside.
- Watch stale peer metadata in `getpeerinfo`.
- Watch `getsyncstatus` for `sync_state`, `catch_up_pending`, `last_sync_error`.
- Back up wallet only if this node also holds keys; seed-only nodes should avoid hot funds.
