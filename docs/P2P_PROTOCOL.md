# P2P Protocol Notes

Legacy Core uses Bitcoin-style wire framing with Legacy Coin mainnet identity.

## Identity

- Message start: `a4 ac c6 4d`
- Default P2P port: `19555`
- Chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`

## Heartbeats and Liveness

- Periodic ping/pong exchange is enabled for peer liveness.
- Configurable ping interval via `peer_ping_interval_seconds` (minimum 10).
- `getpeerinfo` includes ping/pong latency and freshness fields.

## Sync Watchdog

`getsyncstatus` exposes watchdog and sync internals:

- local and best peer heights
- blocks behind
- last header/block/message/sync request ages
- stale peer counts
- watchdog actions and reconnect counts

## Logging Modes

- ASCII-safe server logs are always available.
- Pretty local logs can be enabled through log configuration.
- Avoid enabling noisy debug traces on constrained production hosts unless needed.
