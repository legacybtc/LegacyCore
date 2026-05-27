# Storage and Reindex

Purpose: explain storage checks and repair/reindex behavior in v1.0.4.  
Audience: node operators, integrators, and developers.  
Status: active for v1.0.4.  
Safety warning: back up wallet data before repair operations.

## What This Is

Legacy Core storage health and active-chain index rebuild workflow.

## Storage Health Check

```bash
./legacycoin-cli checkstorage
```

Checks include:

- best block readability
- height index readability and tip match
- chainwork readability
- UTXO stats readability

## Reindex / Repair Paths

RPC:

```bash
./legacycoin-cli reindex
./legacycoin-cli checkstorage true
```

Daemon offline command:

```bash
./legacycoind reindex
```

## v1.0.4 Rebuild Scope

When repair/reindex runs, it rebuilds:

- height index
- hash index
- `txindex` if enabled (`txindex=1`)
- `addressindex` if enabled (`addressindex=1`)
- chainwork cache/check visibility for active tip

## Safe Failure Behavior

If required files are unreadable/corrupt, commands return explicit errors instead of silently claiming success.

## Expected Output

Successful checks/rebuilds report:

- `ok: true`
- tip hash/height
- index/chainwork health fields

## Troubleshooting

- If `txindex` lookup fails, verify config and run reindex.
- If address RPC is disabled, enable `addressindex=1` and rebuild.
- If daemon is already running in a conflicting maintenance flow, stop it cleanly before offline repair steps.

## Known Limitations

- Reindex is active-chain focused and not a substitute for external historical analytics indexing.
