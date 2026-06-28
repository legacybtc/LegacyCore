# Mining Pool Operator Guide

This guide explains how to set up and operate a LegacyCoin mining pool.

## Architecture

```
Miners → Stratum (:3333) → legacycoind (RPC :19556) → P2P Network
```

Two approaches:
1. **Built-in Stratum** (new in v1.0.9) — simple embedded server in `legacycoind`
2. **Standalone pool** — use the Stratum protocol with a custom pool backend

## Option 1: Built-in Stratum Server

The simplest option. Start `legacycoind` with Stratum enabled:

```bash
legacycoind -stratum -stratumport=3333
```

Miners connect to `stratum+tcp://your-pool.com:3333`.

### Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-stratum` | `false` | Enable Stratum server |
| `-stratumport` | `3333` | Stratum listening port |
| `-stratumdiff` | `1.0` | Initial share difficulty |

### Worker Authentication

Workers authenticate with `username.workername` as the Stratum user. The pool accepts all connections (no password validation in built-in mode).

### Share Difficulty

The built-in server uses a fixed share difficulty (`-stratumdiff`). For production pools, use a standalone pool server that implements variable difficulty (vardiff).

## Option 2: Standalone Pool Server

For production-scale pools, implement a custom pool using the Stratum protocol.

### Stratum Protocol Overview

Communication is JSON-RPC over TCP, terminated by `\n`.

### Mining.Subscribe

```json
{"id":1,"method":"mining.subscribe","params":[]}
```

Response:

```json
{"id":1,"result":[["mining.notify","0001"],"0001",8],"error":null}
```

### Mining.Authorize

```json
{"id":1,"method":"mining.authorize","params":["worker.1","password"]}
```

Response:

```json
{"id":1,"result":true,"error":null}
```

### Mining.Notify (server → miner)

Broadcast when a new block arrives or a new template is generated:

```json
{
  "id": null,
  "method": "mining.notify",
  "params": [
    "1",           // job_id
    "0000...",     // prevhash (big-endian hex, reversed)
    "abcd...",     // coinbase1 (not yet implemented — use full merkle root)
    "1234...",     // coinbase2
    "0001...",     // merkle_branch (not yet implemented — list of hashes)
    "00000001",    // version (big-endian hex)
    "5f3b8a00",    // nbits (big-endian hex)
    "00000000",    // ntime (big-endian hex, set by miner)
    true           // clean_jobs
  ]
}
```

> **Note**: This implementation sends the full merkle root rather than individual merkle branches. Pool software that needs per-transaction selection should compute its own merkle branches.

### Mining.Submit (miner → server)

```json
{
  "id": 1,
  "method": "mining.submit",
  "params": ["worker.1","1","00000000","5f3b8a00","12345678"]
}
```

Parameters: `[worker_name, job_id, extra_nonce2, ntime, nonce]`

The server validates:
1. Hash meets share target → accepted share
2. Hash meets block target → block found! Submitted to network

## Solo Mining

For solo mining, connect directly:

```bash
legacycoin-cli generate 1
```

Or use CPU mining:

```bash
legacycoind -cpu -cpuminers=4
```

## Recommended Pool Software

For production pools, consider building on:

- **NOMP** (Node Open Mining Protocol) — adapt the Stratum module
- **ckpool** — C-based, highly performant
- Custom implementation using the protocol above

## Payout Strategies

| Strategy | Description |
|----------|-------------|
| PPLNS | Pay Per Last N Shares — most common, anti-cheat |
| PPS | Pay Per Share — pool takes risk |
| PROP | Proportional — per-block payout distribution |
| SOLO | Full block reward to finder |

## Security

- Run the pool backend and `legacycoind` on the same machine
- Bind Stratum to all interfaces (`0.0.0.0`)
- Use a reverse proxy (nginx) for DDoS protection
- Monitor share acceptance rate for worker issues
- Set share difficulty based on miner hashrate

## Troubleshooting

**Miners connect but get "job not found"** — restart the pool server.

**No shares accepted** — verify `-stratumdiff` is appropriate for miner hashrate.

**Blocks found but not submitted** — check RPC connectivity between pool and `legacycoind`.
