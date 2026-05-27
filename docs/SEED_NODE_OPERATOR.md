# Seed Node Operator Guide

Purpose: run a stable public P2P node for network connectivity.  
Audience: seed-node and infrastructure operators.  
Status: active for v1.0.4.  
Safety warning: keep RPC private; seed nodes should not expose wallet RPC publicly.

## Mainnet Values

- P2P port: `19555`
- RPC port: `19556`
- message start: `a4 ac c6 4d`
- chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`
- DNS seeds configured: `legacycoinseed.space`, `legacycoinseed2.space`

## Quick Start

```bash
./legacycoind params
./legacycoind run -seed-peers
```

## Recommended Config

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

## Monitoring Commands

```bash
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
./legacycoin-cli checkstorage
```

## Firewall Rules

- allow inbound TCP `19555` when running public seed/node
- block public access to `19556`

## Troubleshooting

- low/no peers: verify public reachability and outbound DNS/connectivity
- stale peers: inspect ping/pong and sync state fields

## Known Limitations

- DNS seed infrastructure itself is external to this repository.
