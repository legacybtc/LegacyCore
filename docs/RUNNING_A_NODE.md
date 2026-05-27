# Running a Node

Purpose: run and monitor a standalone Legacy Core node.  
Audience: node operators and infrastructure users.  
Status: active for v1.0.4.  
Safety warning: keep RPC (`19556`) private.

## What This Is

Basic operational guide for a headless Legacy Core node.

## Quick Start

```bash
./legacycoind run -seed-peers
```

Check health:

```bash
./legacycoin-cli getblockchaininfo
./legacycoin-cli getnetworkinfo
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
```

## Network Rules

- P2P port `19555` can be public.
- RPC port `19556` should be local/private only.

## Useful Operational Commands

```bash
./legacycoin-cli checkstorage
./legacycoin-cli doctor
./legacycoin-cli getrawmempool
./legacycoin-cli getmempoolinfo
```

## Troubleshooting

- No peers: confirm firewall allows outbound and (if needed) inbound P2P.
- Stuck height: inspect `getsyncstatus` and stale peer fields.
- Storage warning: run `checkstorage`, then `reindex` if advised.

## Known Limitations

- Optional indexes (`txindex`, `addressindex`) are disabled by default.
