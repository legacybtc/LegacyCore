#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-.}"
OUT_FILE="${2:-SHA256SUMS.txt}"

if [[ ! -d "$TARGET_DIR" ]]; then
  echo "[generate-sha256s] target directory not found: $TARGET_DIR" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  HASH_CMD='sha256sum'
elif command -v shasum >/dev/null 2>&1; then
  HASH_CMD='shasum -a 256'
else
  echo "[generate-sha256s] neither sha256sum nor shasum was found" >&2
  exit 1
fi

pushd "$TARGET_DIR" >/dev/null
tmp_file="$(mktemp)"
out_base="$(basename "$OUT_FILE")"

while IFS= read -r file; do
  [[ -z "$file" ]] && continue
  # shellcheck disable=SC2086
  $HASH_CMD "$file" >>"$tmp_file"
done < <(find . -maxdepth 1 -type f -printf '%P\n' | grep -v "^${out_base}$" | LC_ALL=C sort)

mv "$tmp_file" "$OUT_FILE"
chmod 644 "$OUT_FILE"
popd >/dev/null

echo "[generate-sha256s] wrote $TARGET_DIR/$OUT_FILE"
