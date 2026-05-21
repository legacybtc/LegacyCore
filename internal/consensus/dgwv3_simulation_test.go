package consensus

import (
	"math/big"
	"testing"
)

func makeDGWEntries(spacing uint32, bits uint32) []BlockWindowEntry {
	recent := make([]BlockWindowEntry, DGWv3PastBlocks)
	base := uint32(2_000_000)
	for i := 0; i < DGWv3PastBlocks; i++ {
		recent[i] = BlockWindowEntry{
			Height: int32(DGWv3PastBlocks - 1 - i),
			Time:   base - uint32(i)*spacing,
			Bits:   bits,
		}
	}
	return recent
}

func TestDGWv3TwentyFastBlocksIncreaseDifficultyAggressively(t *testing.T) {
	bits := uint32(0x1f0fffff)
	got := DarkGravityWaveV3(makeDGWEntries(60, bits), 600, PowLimit, bits)
	oldTarget := CompactToBig(bits)
	newTarget := CompactToBig(got)
	if newTarget.Cmp(oldTarget) >= 0 {
		t.Fatalf("fast blocks must make target harder/lower: old=%s new=%s bits=%08x", oldTarget, newTarget, got)
	}
	minExpected := new(big.Int).Div(oldTarget, big.NewInt(3))
	if newTarget.Cmp(minExpected) < 0 {
		t.Fatalf("fast-block retarget exceeded clamp: got target %s below 1/3 old target %s", newTarget, minExpected)
	}
}

func TestDGWv3TwentySlowBlocksDecreaseDifficultySafely(t *testing.T) {
	bits := uint32(0x1f0fffff)
	got := DarkGravityWaveV3(makeDGWEntries(2400, bits), 600, PowLimit, bits)
	oldTarget := CompactToBig(bits)
	newTarget := CompactToBig(got)
	if newTarget.Cmp(oldTarget) <= 0 {
		t.Fatalf("slow blocks must make target easier/higher: old=%s new=%s bits=%08x", oldTarget, newTarget, got)
	}
	maxExpected := new(big.Int).Mul(oldTarget, big.NewInt(3))
	if newTarget.Cmp(maxExpected) > 0 {
		t.Fatalf("slow-block retarget exceeded clamp: got target %s above 3x old target %s", newTarget, maxExpected)
	}
	if newTarget.Cmp(PowLimit) > 0 {
		t.Fatalf("target must not exceed pow limit")
	}
}

func TestCompactBitsRoundTripAndDirection(t *testing.T) {
	samples := []uint32{0x1f0fffff, 0x1e7fffff, 0x207fffff, 0x1d00ffff}
	for _, bits := range samples {
		target := CompactToBig(bits)
		if target.Sign() <= 0 {
			t.Fatalf("sample bits decoded to non-positive target: %08x", bits)
		}
		round := BigToCompact(target)
		if CompactToBig(round).Cmp(target) != 0 {
			t.Fatalf("compact roundtrip changed target: bits=%08x round=%08x", bits, round)
		}
	}

	harder := CompactToBig(0x1e0fffff)
	easier := CompactToBig(0x1f0fffff)
	if harder.Cmp(easier) >= 0 {
		t.Fatalf("difficulty direction sanity failed: lower target should be harder")
	}
}
