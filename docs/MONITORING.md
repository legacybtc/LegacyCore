# Monitoring

Purpose: operator monitoring checklist for Legacy Core nodes.  
Audience: node operators, seed operators, miners, and integrators.  
Status: active for v1.0.4.  
Safety warning: monitor from private infrastructure; do not expose privileged RPC publicly.

## Core Health Commands

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
./legacycoin-cli getrawmempool
./legacycoin-cli getmempoolinfo
./legacycoin-cli getnetworkhashps
./legacycoin-cli checkstorage
./legacycoin-cli doctor
```

## Pretty Logs / Log Options

Config options:

- `pretty_logs=true|false`
- `log_icons=true|false`
- `peer_ping_interval_seconds=15` (minimum 10; 15-30 recommended)

## Ping/Pong and Peer Liveness

`getpeerinfo` includes fields for:

- `last_ping_time`
- `last_pong_time`
- `ping_latency_ms`
- `missed_pongs`
- `stale`
- `sync_state`
- `reported_height`

Use these to detect stale peers before sync stalls.

## What To Alert On

- RPC unreachable
- no peers for extended periods
- `blocks_behind` rising
- repeated watchdog recovery actions
- stale peer count growing
- storage health failures

## Troubleshooting

- If peers are stale, inspect firewall, connectivity, and addnode configuration.
- If sync is stalled, inspect `getsyncstatus` and `last_sync_error`.
- If storage warnings appear, run `checkstorage` and repair if needed.

## Known Limitations

- Network hash rate remains an estimate, not a direct census of all network hash power.
