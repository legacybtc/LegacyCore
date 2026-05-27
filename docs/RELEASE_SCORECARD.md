# Release Scorecard

Purpose: release readiness checklist before public publication.  
Audience: maintainers and release engineers.  
Status: active for v1.0.4.  
Safety warning: do not publish if critical checks fail.

Use status values:

- `pass`
- `fail`
- `n/a`

| Area | Status | Notes |
| --- | --- | --- |
| Builds (Windows/Linux) | pass/fail | |
| Tests (`go test ./...`) | pass/fail | |
| Vet (`go vet ./...`) | pass/fail | |
| CI workflow green | pass/fail | |
| Source cleanliness scan | pass/fail | |
| Mainnet identity verification | pass/fail | |
| Wallet runtime smoke | pass/fail | |
| Daemon runtime smoke | pass/fail | |
| CLI RPC smoke | pass/fail | |
| Pool smoke script | pass/fail | |
| Exchange smoke script | pass/fail | |
| Explorer smoke script | pass/fail | |
| Multi-node smoke | pass/fail | |
| Storage/reindex checks | pass/fail | |
| Release assets created | pass/fail | |
| SHA256SUMS generated | pass/fail | |
| Sensitive scan pass | pass/fail | |
| Docs updated | pass/fail | |
| Known limitations updated honestly | pass/fail | |

## Scoring Rule

A release is ready only when all critical rows are `pass` or intentionally marked `n/a` with explanation.
