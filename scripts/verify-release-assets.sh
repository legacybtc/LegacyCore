#!/usr/bin/env bash
set -euo pipefail

if [[ "$#" -eq 0 ]]; then
  echo "usage: $0 <archive> [archive...]" >&2
  exit 1
fi

SENSITIVE_RE="$(
  printf '%s' \
  'C:''\\Users|C:''/Users|MA''X/|Co''dex|/home/ma''xgor|server''2|root''@|wallet\.dat|\.cookie|config\.local\.json'
)"

check_windows_zip() {
  local archive="$1"
  local listing
  listing="$(unzip -Z -1 "$archive")"

  for required in legacy-wallet.exe legacycoind.exe legacycoin-cli.exe README_FIRST.txt LICENSE NOTICE SHA256SUMS.txt START_HERE.bat; do
    if ! grep -Fxq "$required" <<<"$listing"; then
      echo "[verify-release-assets] missing $required in $archive" >&2
      return 1
    fi
  done
  if grep -E "$SENSITIVE_RE" <<<"$listing" >/dev/null; then
    echo "[verify-release-assets] sensitive path-like entry found in zip listing: $archive" >&2
    return 1
  fi
}

check_unix_tar() {
  local archive="$1"
  local listing
  listing="$(tar -tvf "$archive")"
  local names
  names="$(tar -tf "$archive")"

  for required in legacycoind legacycoin-cli README_FIRST.txt LICENSE NOTICE SHA256SUMS.txt; do
    if ! grep -E "/${required}$" <<<"$names" >/dev/null; then
      echo "[verify-release-assets] missing $required in $archive" >&2
      return 1
    fi
  done
  if ! grep -E '^-rwxr-xr-x .*/legacycoind$' <<<"$listing" >/dev/null; then
    echo "[verify-release-assets] legacycoind is not 755 in $archive" >&2
    return 1
  fi
  if ! grep -E '^-rwxr-xr-x .*/legacycoin-cli$' <<<"$listing" >/dev/null; then
    echo "[verify-release-assets] legacycoin-cli is not 755 in $archive" >&2
    return 1
  fi
  if grep -E "$SENSITIVE_RE" <<<"$listing" >/dev/null; then
    echo "[verify-release-assets] sensitive metadata found in tar listing: $archive" >&2
    return 1
  fi
}

for archive in "$@"; do
  if [[ ! -f "$archive" ]]; then
    echo "[verify-release-assets] archive not found: $archive" >&2
    exit 1
  fi
  case "$archive" in
    *.zip)
      check_windows_zip "$archive"
      ;;
    *.tar.gz|*.tgz)
      check_unix_tar "$archive"
      ;;
    *)
      echo "[verify-release-assets] unsupported archive extension: $archive" >&2
      exit 1
      ;;
  esac
  echo "[verify-release-assets] ok: $archive"
done
