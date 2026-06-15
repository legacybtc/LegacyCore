# Legacy AI Assistant — Threat Model (v0.1)

## Assets Protected

| Asset | Risk | Mitigation |
|-------|------|------------|
| Seed phrases | Never exposed | Excluded from SanitizedSnapshot schema |
| Private keys | Never exposed | Excluded from schema |
| Wallet password | Never exposed | Excluded from schema |
| RPC credentials | Never exposed | Random session token per start |
| Transaction data | Never exposed | No txids, amounts, or addresses in snapshot |
| Wallet database | Never exposed | AI only sees sanitized JSON |
| Mining reward address | Masked | Payout destination excluded from snapshot |
| Node configuration | Read-only | AI cannot write config files |

## Attack Surface

| Vector | Risk | Mitigation |
|--------|------|------------|
| Sidecar binds to public IP | High | Enforced 127.0.0.1 only |
| Model jailbreak extracts secrets | Medium | Snapshot has no secrets to extract |
| Malicious model file | Medium | SHA256 verification, user-opted download |
| Sidecar process escape | Low | Separate process, no suid, limited scope |
| Sidecar memory leak in wallet | N/A | Separate process — crash does not affect wallet |
| Session token leak | Low | Random per start, never logged, localhost only |
| Conversation history leak | Low | Optional, stored locally, never uploaded |

## Write Operations Prohibited

The AI must never:
- startminer, stopminer
- sendtoaddress, sendrawtransaction
- walletpassphrase, encryptwallet
- importprivkey, dumpprivkey
- sethdseed
- delete wallet files
- modify legacycoin.conf
- expose listen ports publicly

The wallet bridge enforces this by never exposing write RPC methods to the AI.
