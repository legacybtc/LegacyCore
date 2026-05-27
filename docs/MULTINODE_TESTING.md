# Multi-node Testing

Use these smoke harnesses for basic multi-node catch-up validation:

- `scripts/multinode-smoke.ps1` (Windows)
- `scripts/multinode-smoke.sh` (Linux/macOS)

## Scenario

1. Start node A with isolated datadir and ports.
2. Start node B with isolated datadir and ports.
3. Connect B to A.
4. Mine block(s) on A.
5. Verify B catches up by height/hash.
6. Stop both nodes cleanly.

## Why This Exists

This harness catches regressions where:

- nodes remain behind despite peer connectivity
- sync watchdog/reconnect behavior regresses
- port/data-directory isolation behavior breaks

## Notes

- These scripts use runtime port override flags for test isolation.
- Do not use test overrides unintentionally in production service units.
