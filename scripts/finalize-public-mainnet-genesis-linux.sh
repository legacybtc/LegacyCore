#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THREADS="${1:-$(nproc)}"
GENESIS_TIMESTAMP="${LEGACY_GENESIS_TIMESTAMP:-onecpuonevote Legacy Coin Public Mainnet 20/May/2026}"
GENESIS_TIME="${LEGACY_GENESIS_TIME:-1779235200}"
GENESIS_BITS="${LEGACY_GENESIS_BITS:-207fffff}"
POST_GENESIS_BITS="${LEGACY_POST_GENESIS_BITS:-1f0fffff}"
OUT="$ROOT/dist/rc2-public-genesis"
FINAL_CHAIN_ID="legacy-mainnet-1.0.0-rc2-5b4c78e4"
FINAL_GENESIS_HASH="5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5"

export CGO_ENABLED=1

mkdir -p "$OUT"

if grep -q "$FINAL_CHAIN_ID" "$ROOT/internal/chaincfg/params.go" && grep -q "$FINAL_GENESIS_HASH" "$ROOT/internal/chaincfg/params.go"; then
	cat > "$OUT/PUBLIC_MAINNET_IDENTITY.txt" <<EOF
chain_id=legacy-mainnet-1.0.0-rc2-5b4c78e4
message_start=a4 ac c6 4d
message_start_hex=a4acc64d
genesis_timestamp=onecpuonevote Legacy Coin Public Mainnet 20/May/2026
genesis_time=1779235200
genesis_bits=207fffff
post_genesis_bits=1f0fffff
genesis_nonce=3
genesis_hash=5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5
EOF
	echo "Public mainnet identity is already finalized:"
	cat "$OUT/PUBLIC_MAINNET_IDENTITY.txt"
	exit 0
fi

python3 - "$ROOT" "$GENESIS_TIMESTAMP" "$GENESIS_TIME" "$GENESIS_BITS" "$POST_GENESIS_BITS" <<'PY'
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
timestamp, genesis_time, genesis_bits, post_bits = sys.argv[2:6]
params = root / "internal" / "chaincfg" / "params.go"
text = params.read_text()
text = re.sub(r'ChainID:\s+"[^"]+"', 'ChainID:          "legacy-mainnet-rc2-pending"', text, count=1)
text = re.sub(r'GenesisTimestamp:\s+"[^"]+"', f'GenesisTimestamp: "{timestamp}"', text, count=1)
text = re.sub(r'GenesisTime:\s+uint32\(time\.Date\([^)]+\)\.Unix\(\)\)', f'GenesisTime:      {genesis_time}', text, count=1)
text = re.sub(r'GenesisBits:\s+0x[0-9a-fA-F]+', f'GenesisBits:      0x{genesis_bits}', text, count=1)
text = re.sub(r'PostGenesisBits:\s+0x[0-9a-fA-F]+', f'PostGenesisBits:  0x{post_bits}', text, count=1)
text = re.sub(r'GenesisNonce:\s+\d+', 'GenesisNonce:     0', text, count=1)
text = re.sub(r'GenesisHash:\s+"[^"]*"', 'GenesisHash:      ""', text, count=1)
params.write_text(text)
PY

echo "== Public genesis template =="
go run ./cmd/legacycoind params | tee "$OUT/params-before-genesis.txt"
grep -q "yespower backend: cgo-c-reference" "$OUT/params-before-genesis.txt"

echo "== Mining public mainnet genesis with $THREADS threads =="
go run ./cmd/legacycoind genesis "$THREADS" | tee "$OUT/genesis-mining.txt"

python3 - "$ROOT" "$OUT/genesis-mining.txt" "$GENESIS_TIMESTAMP" "$GENESIS_TIME" "$GENESIS_BITS" "$POST_GENESIS_BITS" <<'PY'
import hashlib
import pathlib
import re
import sys

root = pathlib.Path(sys.argv[1])
mining_log = pathlib.Path(sys.argv[2])
timestamp, genesis_time, genesis_bits, post_bits = sys.argv[3:7]
# These old_* values are replacement anchors for migrating pre-RC2 source
# snapshots only. They are not active RC2 network parameters. The script exits
# earlier when the finalized RC2 chain ID and genesis hash are already locked.
old_chain = "legacy-mainnet-v5.12-4fdb7844"
old_hash = "4fdb78446d4ac600d06d8e41e0f282a83aed1c8454a9d8c9807656a60fa02d17"
old_time = "1777501200"
old_bits = "207fffff"
old_post = "1f0fffff"
old_nonce = "0"
old_msg_plain = "e2 36 24 18"
old_msg_hex = "e2362418"

log = mining_log.read_text()
m = re.search(r"mined nonce=(\d+) time=(\d+) hash=([0-9a-fA-F]{64})", log)
if not m:
    raise SystemExit("could not parse mined genesis line")
nonce, mined_time, genesis_hash = m.group(1), m.group(2), m.group(3).lower()
chain_id = f"legacy-mainnet-1.0.0-rc2-{genesis_hash[:8]}"
magic = hashlib.sha256(("Legacy Coin LBTC public mainnet " + genesis_hash).encode()).digest()[:4]
magic_hex = magic.hex()
magic_plain = " ".join(f"{b:02x}" for b in magic)
magic_go = "[4]byte{" + ", ".join(f"0x{b:02x}" for b in magic) + "}"

params = root / "internal" / "chaincfg" / "params.go"
text = params.read_text()
text = re.sub(r'ChainID:\s+"[^"]+"', f'ChainID:          "{chain_id}"', text, count=1)
text = re.sub(r'MessageStart:\s+\[4\]byte\{[^}]+\}', f'MessageStart:     {magic_go}', text, count=1)
text = re.sub(r'GenesisTimestamp:\s+"[^"]+"', f'GenesisTimestamp: "{timestamp}"', text, count=1)
text = re.sub(r'GenesisTime:\s+(?:uint32\(time\.Date\([^)]+\)\.Unix\(\)\)|\d+)', f'GenesisTime:      {mined_time}', text, count=1)
text = re.sub(r'GenesisBits:\s+0x[0-9a-fA-F]+', f'GenesisBits:      0x{genesis_bits}', text, count=1)
text = re.sub(r'PostGenesisBits:\s+0x[0-9a-fA-F]+', f'PostGenesisBits:  0x{post_bits}', text, count=1)
text = re.sub(r'GenesisNonce:\s+\d+', f'GenesisNonce:     {nonce}', text, count=1)
text = re.sub(r'GenesisHash:\s+"[^"]*"', f'GenesisHash:      "{genesis_hash}"', text, count=1)
params.write_text(text)

targets = [
    "README.md",
    "README_RC2_BUILD_LINUX.md",
    "docs",
    "configs",
    "cmd/legacysite",
    "cmd/legacywallet",
    "internal/config",
]
for rel in targets:
    path = root / rel
    files = [path] if path.is_file() else [p for p in path.rglob("*") if p.is_file() and "node_modules" not in p.parts]
    for f in files:
        try:
            s = f.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        ns = s.replace(old_chain, chain_id)
        ns = ns.replace(old_hash, genesis_hash)
        ns = ns.replace(old_time, mined_time)
        ns = ns.replace(old_bits, genesis_bits)
        ns = ns.replace(old_post, post_bits)
        ns = ns.replace(old_msg_plain, magic_plain)
        ns = ns.replace(old_msg_hex, magic_hex)
        ns = ns.replace(f"Genesis nonce: `{old_nonce}`", f"Genesis nonce: `{nonce}`")
        ns = ns.replace(f"GenesisNonce:     {old_nonce}", f"GenesisNonce:     {nonce}")
        if ns != s:
            f.write_text(ns, encoding="utf-8")

summary = root / "dist" / "rc2-public-genesis" / "PUBLIC_MAINNET_IDENTITY.txt"
summary.write_text(
    "\n".join([
        f"chain_id={chain_id}",
        f"message_start={magic_plain}",
        f"message_start_hex={magic_hex}",
        f"genesis_timestamp={timestamp}",
        f"genesis_time={mined_time}",
        f"genesis_bits={genesis_bits}",
        f"post_genesis_bits={post_bits}",
        f"genesis_nonce={nonce}",
        f"genesis_hash={genesis_hash}",
    ]) + "\n",
    encoding="utf-8",
)
print(summary.read_text())
PY

echo "== Final public identity params =="
go run ./cmd/legacycoind params | tee "$OUT/params-after-genesis.txt"
grep -q "yespower backend: cgo-c-reference" "$OUT/params-after-genesis.txt"
grep -q "genesis hash: [0-9a-f]" "$OUT/params-after-genesis.txt"

echo "Public mainnet identity finalized. Review $OUT/PUBLIC_MAINNET_IDENTITY.txt, then build with scripts/build-rc2-cgo-linux-amd64.sh."
