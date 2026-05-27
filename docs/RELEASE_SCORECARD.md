# Release Scorecard

Use this checklist before publishing any public release.

| Area | Status | Notes |
| --- | --- | --- |
| Builds (Windows/Linux) | ☐ | |
| Tests (`go test ./...`) | ☐ | |
| Vet (`go vet ./...`) | ☐ | |
| CI workflow green | ☐ | |
| Source cleanliness scan | ☐ | |
| Mainnet identity verification | ☐ | |
| Wallet runtime smoke | ☐ | |
| Daemon runtime smoke | ☐ | |
| CLI RPC smoke | ☐ | |
| JSON-RPC compatibility checks | ☐ | |
| Pool smoke script | ☐ | |
| Exchange smoke script | ☐ | |
| Explorer smoke script | ☐ | |
| Multi-node catch-up smoke | ☐ | |
| Storage / reindex checks | ☐ | |
| Release assets created | ☐ | |
| SHA256SUMS generated | ☐ | |
| Archive verification script pass | ☐ | |
| Sensitive scan pass | ☐ | |
| Docs updated | ☐ | |
| Known limitations updated honestly | ☐ | |

## Scoring

- Ready for public release: all critical areas checked and no red items.
- If any critical item fails, do not publish and document blockers first.
