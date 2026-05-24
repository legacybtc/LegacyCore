# Legacy Mining Guide (RC2)

## 1) Solo mining basics

- Mining is solo by default.
- Hashrate increases chance, not guarantee, of finding a block.
- Rewards appear only when your node finds a valid block.
- Coinbase rewards mature after 100 blocks.

## 2) Local vs network hashrate

- Local KH/s: your miner performance.
- Network KH/s: estimated from chain difficulty/timing.
- A low direct peer count does not mean no other miners exist.

## 3) Safety states

Miner should pause when node is:

- syncing / behind peers
- stalled
- no peers
- wallet locked (if required)

Miner should resume when node is healthy/current.

## 4) Useful commands

```bash
./legacycoin-cli getminingaddress
./legacycoin-cli setminerthreads 4
./legacycoin-cli startminer
./legacycoin-cli getminerstatus
./legacycoin-cli stopminer
```

## 5) Interpreting rejected/stale blocks

Rejected or stale submissions can happen due to:

- another miner found a block first
- stale template (chain advanced)
- bad previous block reference
- invalid PoW or bits mismatch

Use:

```bash
./legacycoin-cli getminerstatus
./legacycoin-cli getsyncstatus
```

to inspect pause reason, template height, and sync state.

