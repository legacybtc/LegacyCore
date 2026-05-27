# Multi-node Testing

Purpose: validate peer connect, sync, and reconnect behavior across isolated nodes.  
Audience: developers and operators validating releases.  
Status: active for v1.0.4.  
Safety warning: use isolated test datadirs/ports only.

## Scripts

- `scripts/multinode-smoke.ps1`
- `scripts/multinode-smoke.sh`

## Scenario

1. Start node A with isolated datadir/ports.
2. Start node B with isolated datadir/ports.
3. Connect B to A.
4. Verify both establish peer connectivity.
5. Verify height/hash alignment.
6. Restart follower and verify reconnect/alignment again.

## Expected Output

Script should complete with explicit alignment/reconnect success messages and nonzero peer counts.

## Troubleshooting

- RPC auth/cookie mismatch: ensure CLI and daemon use same datadir.
- no connect: verify `-connect`/`addnode` target and firewall rules.

## Known Limitations

- Smoke harness is intentionally lightweight and not a full adversarial network simulation.
