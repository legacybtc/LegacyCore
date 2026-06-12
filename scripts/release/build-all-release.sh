#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-v1.0.5}"
ARCH="${2:-amd64}"

cd "$ROOT_DIR"
echo "[release/all] building Linux Core package"
bash "$ROOT_DIR/scripts/release/build-linux-core.sh" "$VERSION" "$ARCH"
echo "[release/all] Windows packages must be built on Windows with scripts/release/build-all-release.ps1 after manual GUI smoke passes."
