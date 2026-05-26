# Storage and Reindex

Status: file-backed storage, UTXO storage, undo data, journals, storage health checks, and limited height-index repair are `implemented`; full reindex, full txindex, and address index are `planned`.

## Current Storage Layout

Inside the active data directory:

- `blocks/`: serialized block files by block hash.
- `index/hash/`: block index entries by hash.
- `index/height/`: active-chain index entries by height.
- `utxo/`: UTXO JSON entries.
- `undo/`: undo data for disconnect/reorg.
- `chainstate.json`: active tip.
- `chainstate.journal.json`: temporary journal for atomic connect/disconnect recovery.

Linux default data directory: `~/.legacycoin`.

Windows default data directory: the application data `LegacyCoin` directory.

## Atomic Writes

`internal/fsutil.WriteFileAtomic` writes temp file, fsyncs, closes, renames, and best-effort fsyncs the parent directory. Storage tests cover parent directory creation and replacement behavior.

## Health Checks

```powershell
.\legacycoin-cli.exe checkstorage
```

```bash
./legacycoin-cli checkstorage
```

Health reports:

- tip height/hash
- best block readability
- height index readability
- height index matches tip
- UTXO stats readability
- useful error text

## Existing Repair Behavior

If an active-chain height index file is missing, `LoadIndexByHeight` can rebuild active height indexes by walking back from the current tip. This is limited repair, not full reindex.

## Planned Reindex Commands

Planned command names:

- `reindex`: rebuild block/hash/height/UTXO indexes from stored blocks.
- `repairindexes`: repair known-safe index inconsistencies without rewriting blocks.
- `verifychainstate`: stronger UTXO and undo consistency verification.

These are not implemented in v1.0.3.

## Corruption Behavior

- Empty data directory startup: supported.
- Missing storage parent directories: created on write.
- Missing active height index: limited repair.
- Corrupt JSON index: reported as an error.
- Partial connect/disconnect journal: recovered or fails closed.
- Full partial appdata corruption recovery: partial, operator-assisted.

## Address Index Plan

Address index is planned and required for native explorer address search. Until then, explorers and exchanges must scan blocks and maintain their own address database.

## Txindex Plan

Full txindex is planned. Until then, `getrawtransaction` is useful but not a complete historical explorer backend.

## Wallet Activity History Plan

Wallet history exists for wallet-related transactions, but full external address activity history requires address index work.
