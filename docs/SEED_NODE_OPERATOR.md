# Seed Node Operator Guide

Purpose: run a stable public P2P node for network connectivity.  
Audience: seed-node and infrastructure operators.  
Status: active for v1.0.5.
Safety warning: keep RPC private; seed nodes should not expose wallet RPC publicly.

## Mainnet Values

- P2P port: `19555`
- RPC port: `19556`
- message start: `a4 ac c6 4d`
- chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`
- DNS seeds configured: `legacycoinseed.space`, `legacycoinseed2.space`
- Fixed seed fallbacks configured: `77.127.37.157:19555`, `199.19.72.89:19555`, `91.219.63.20:19555`

## Quick Start

```bash
./legacycoind params
./legacycoind run -seednode
```

Windows:

```powershell
.\legacycoind.exe params
.\legacycoind.exe run -seednode
```

## Recommended Config

```text
node_role=seed
seednode=true
bind=0.0.0.0
rpcbind=127.0.0.1
seed_peers=true
peer_safety=true
peer_max_inbound=512
peer_max_per_ip=32
peer_max_per_subnet=128
chain_id_enforce=true
peer_ping_interval_seconds=30
pretty_logs=true
log_icons=true
```

Seed mode behavior:

- runs as a full-node public P2P relay
- refuses non-local RPC binds
- disables miner auto-start and rejects `startminer`
- raises inbound peer caps while keeping per-IP, per-subnet, rate-limit, and ban-score controls
- participates in DNS, addnode, and `addr/getaddr` peer discovery
- keeps discovered peer addresses in memory only; cache entries are not persisted across restarts

## Monitoring Commands

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getbootstrapinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
./legacycoin-cli checkstorage
```

## Firewall Rules

- allow inbound TCP `19555` when running public seed/node
- block public access to `19556`

## Troubleshooting

- low/no peers: verify public reachability and outbound DNS/connectivity
- if a previous `-connect` or `-noseednode` run disabled DNS discovery, run once with `-seed-peers` or set `noseednode=false`
- stale peers: inspect ping/pong and sync state fields

## Known Limitations

- DNS seed infrastructure itself is external to this repository.
