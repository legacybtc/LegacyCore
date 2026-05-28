package consensus

import (
	"math/big"
	"testing"
)

func FuzzCompactWorkConversions(f *testing.F) {
	f.Add(uint32(0x1e0fffff))
	f.Add(uint32(0x1d00ffff))
	f.Add(uint32(0x207fffff))
	f.Fuzz(func(t *testing.T, bits uint32) {
		target := CompactToBig(bits)
		if target == nil {
			t.Fatalf("nil target")
		}
		if target.Sign() >= 0 {
			round := BigToCompact(target)
			_ = CompactToBig(round)
		}
		_ = WorkForBits(bits)
	})
}

func FuzzBigToCompactRoundTrip(f *testing.F) {
	f.Add([]byte{0x01})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, raw []byte) {
		n := new(big.Int).SetBytes(raw)
		compact := BigToCompact(n)
		_ = CompactToBig(compact)
	})
}
