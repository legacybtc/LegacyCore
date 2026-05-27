# Monitoring

## Core Health Commands

```bash
legacycoin-cli getblockchaininfo
legacycoin-cli getnetworkinfo
legacycoin-cli getpeerinfo
legacycoin-cli getsyncstatus
legacycoin-cli getrawmempool
legacycoin-cli getmempoolinfo
legacycoin-cli getnetworkhashps
legacycoin-cli checkstorage
legacycoin-cli doctor
```

## What To Watch

- Height progression and best hash movement
- Peer count and peer freshness
- Sync watchdog status and last watchdog action
- Mempool size / pending tx behavior
- RPC reachability
- Storage health (`checkstorage`)

## P2P Ping/Pong Monitoring

`getpeerinfo` includes live ping/pong metadata (latency and freshness fields). Use these fields to detect stale peers and reconnect conditions before sync stalls.

## Alerting Suggestions

- No peers for > 5 minutes
- Height unchanged while peers report ahead
- Repeated watchdog reconnect actions
- Storage health not OK
- RPC endpoint unreachable
