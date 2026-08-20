#!/usr/bin/env python3
from pathlib import Path

p = Path("internal/p2p/server.go")
s = p.read_text(encoding="utf-8")

old = '''\t\t\tif err := s.requestBlocks(p); err != nil {\n\t\t\t\ts.log.Printf("p2p request blocks from %s: %v", conn.RemoteAddr(), err)\n\t\t\t}\n'''
if old not in s:
    raise SystemExit("handshake requestBlocks block not found")
s = s.replace(old, "", 1)

old = '''\t\tfor batchStart < len(wanted) && len(batch) < maxGetDataItems-1 {'''
new = '''\t\t// Each requested block currently has a canonical Yespower hash and a\n\t\t// legacy SHA256d compatibility hash. Keep the total INV entries within\n\t\t// the same maxGetDataItems limit enforced by serveInventory. The old\n\t\t// maxGetDataItems-1 condition could create 510 INV entries for a 256\n\t\t// item serving limit, silently dropping the tail of every large batch.\n\t\tfor batchStart < len(wanted) && len(batch) <= maxGetDataItems-2 && len(batch)/2 < maxGetDataItems/2 {'''
if old not in s:
    raise SystemExit("unsafe dual-hash batch condition not found")
s = s.replace(old, new, 1)

# Add a small pure helper so the batching invariant is directly testable.
anchor = '''func limitInv(inv []wire.InvVect, max int) []wire.InvVect {'''
helper = '''// maxDualHashGetDataBlocks is the maximum number of blocks that can be\n// represented in one dual-hash getdata message without exceeding\n// maxGetDataItems inventory entries.\nfunc maxDualHashGetDataBlocks() int {\n\treturn maxGetDataItems / 2\n}\n\n'''
if anchor not in s:
    raise SystemExit("limitInv anchor not found")
s = s.replace(anchor, helper + anchor, 1)

# Use the helper in the batching condition.
s = s.replace('len(batch)/2 < maxGetDataItems/2 {', 'len(batch)/2 < maxDualHashGetDataBlocks() {', 1)

p.write_text(s, encoding="utf-8")
