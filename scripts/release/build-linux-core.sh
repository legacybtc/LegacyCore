#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-v1.0.21}"
ARCH="${2:-amd64}"

cd "$ROOT_DIR"
echo "[release/linux-core] building Linux Core package version=$VERSION arch=$ARCH"
echo "[release/linux-core] set LINUX_CC/LINUX_CXX if cross-compiling with a dedicated Linux toolchain"
bash "$ROOT_DIR/scripts/package-linux.sh" "$VERSION" "$ARCH"
