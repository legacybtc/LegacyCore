# Storage and Reindex

Legacy Core stores chain data in the data directory (`~/.legacycoin` on Linux by default).

## Check Storage Health

```bash
legacycoin-cli checkstorage
```

This reports:

- active tip readability
- height index readability/match
- UTXO stats readability

## Repair Height Index

RPC:

```bash
legacycoin-cli reindex
```

or:

```bash
legacycoin-cli checkstorage true
```

Daemon offline command:

```bash
legacycoind reindex
```

Current reindex scope:

- rebuilds active-chain height and hash indexes from stored tip and block linkage
- rebuilds optional txindex/addressindex when enabled
- verifies post-repair storage health

## Safety Notes

- Back up wallet data before repair operations.
- Do not run repair scripts against active production nodes without maintenance windows.
- This operation does not change consensus rules or chain identity.
