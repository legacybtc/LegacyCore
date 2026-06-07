# P2P Protocol Notes

Purpose: describe network identity, peer liveness, and sync metadata behavior.  
Audience: node operators, seed operators, integrators, and developers.  
Status: active for v1.0.4.  
Safety warning: P2P peers are untrusted; never bypass block/tx validation.

## Network Identity

- Message start: `a4 ac c6 4d`
- P2P port: `19555`
- Chain ID: `legacy-mainnet-1.0.0-rc2-5b4c78e4`

## Ping/Pong Behavior

- Periodic ping is sent to connected peers.
- Peer ping interval is configurable via `peer_ping_interval_seconds`.
- Recommended interval: 15-30 seconds.
- Minimum supported interval: 10 seconds.

## Latency and Stale Detection

Peer metadata tracks:

- last ping/pong timestamps
- ping latency (ms)
- missed pong count
- stale state

Stale peers are monitored and may be reconnected by watchdog logic when sync usefulness degrades.

## Sync State Fields

Operational sync fields exposed through RPC include:

- `reported_height`
- `sync_state`
- `last_sync_request_*`
- `last_sync_error`

Use with `getsyncstatus` for full sync diagnostics.

## Peer Discovery

- DNS seeds are only bootstrap sources.
- Nodes also exchange `getaddr` / `addr` messages after handshake.
- A node that reaches one healthy peer can learn additional public peer addresses from that peer.
- Relayed peer addresses are held in an in-memory known-address manager and retried by the reconnect loop.
- Public relay filters reject unspecified, multicast, link-local, loopback, and private addresses unless the source peer is local/private for local test networks.
- Inbound admission uses `peer_max_inbound`, `peer_max_per_ip`, `peer_max_per_subnet`, and rate limits instead of rejecting every duplicate inbound host.

## Troubleshooting

- If a peer is connected but stale, inspect latency/missed pongs and local firewall/NAT.
- If no peers connect, check seed/addnode configuration and outbound restrictions.
- If DNS seeds are unavailable but one peer is reachable, verify `getbootstrapinfo.known_peer_count` grows after handshake.

## Known Limitations

- Peer diagnostics are operational telemetry and may fluctuate during network churn.
