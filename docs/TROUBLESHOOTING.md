# Troubleshooting

## RPC Offline

```bash
./legacycoind run
./legacycoin-cli getnetworkinfo
```

## No Peers

Check firewall. Verify seed/addnode config:

```bash
./legacycoin-cli getpeerinfo
./legacycoin-cli getsyncstatus
```

## Stuck at Height 0

No peers or stale peers. Add known nodes and retry.

## Mining Not Starting

Common blockers: wallet locked, no peers, unhealthy storage, missing mining address.

```bash
./legacycoin-cli getminingaddress
./legacycoin-cli getminerstatus
./legacycoin-cli checkstorage
./legacycoin-cli getsyncstatus
```

## Headers Rejected / "not linked at position 1"

```
p2p header batch from ... REJECTED by ValidateHeaderSequence: headers not linked at position 1
```

**This is not a bug.** Old SHA256d peers (v1.0.20–v1.0.30) were mined with SHA256d Proof-of-Work and are on a fundamentally different chain after block 1. Their block headers use different nonces and merkle roots, producing different SHA256d hashes.

The daemon still receives blocks from these peers via the **INV flow** (`requestUnknownBlocks`), which reaches the yespower chain tip. The header rejection is harmless.

To verify:
```bash
./legacycoin-cli getblockcount    # should be > 0 (yespower chain height)
./legacycoin-cli getminerstatus   # miner is active
```

---

## Reindex

```bash
./legacycoin-cli checkstorage
./legacycoin-cli reindex
```

or offline:

```bash
./legacycoind reindex
```

## Port Already In Use

```bash
./legacycoin-cli stop
```
Verify no other instance is running.
