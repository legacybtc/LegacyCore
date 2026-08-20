# Release hardening notes

The `chatgpt/sync-hardening-2026-08-20` branch is a staging candidate only.

It must not be treated as a production release until:

1. GitHub Actions completes Linux and Windows build/test jobs.
2. P2P and blockchain race tests pass.
3. The candidate survives the existing multinode/chaos smoke tests.
4. A maintainer reviews the diff and confirms no consensus-rule changes.
