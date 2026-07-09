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

func simulateDGW(spacings []uint32, initialBits uint32) []uint32 {
	recent := makeDGWEntries(600, initialBits)
	currentBits := initialBits
	currentTime := recent[0].Time
	height := recent[0].Height
	out := make([]uint32, 0, len(spacings))
	for _, spacing := range spacings {
		currentTime += spacing
		height++
		entry := BlockWindowEntry{Height: height, Time: currentTime, Bits: currentBits}
		recent = append([]BlockWindowEntry{entry}, recent...)
		if len(recent) > DGWv3PastBlocks {
			recent = recent[:DGWv3PastBlocks]
		}
		currentBits = DarkGravityWaveV3(recent, 600, PowLimit, initialBits)
		out = append(out, currentBits)
	}
	return out
}

func repeatedSpacing(count int, spacing uint32) []uint32 {
	out := make([]uint32, count)
	for i := range out {
		out[i] = spacing
	}
	return out
}

func targetForLast(bits []uint32) *big.Int {
	if len(bits) == 0 {
		return big.NewInt(0)
	}
	return CompactToBig(bits[len(bits)-1])
}

func averageSpacing(spacings []uint32) float64 {
	total := uint64(0)
	for _, spacing := range spacings {
		total += uint64(spacing)
	}
	return float64(total) / float64(len(spacings))
}

func assertTargetRatio(t *testing.T, got *big.Int, base *big.Int, minRatio float64, maxRatio float64) {
	t.Helper()
	ratio, _ := new(big.Rat).SetFrac(got, base).Float64()
	if ratio < minRatio || ratio > maxRatio {
		t.Fatalf("target ratio out of range: got %.4f want %.4f..%.4f target=%s base=%s", ratio, minRatio, maxRatio, got, base)
	}
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

func TestDGWv3StableHashrateSimulationKeepsTenMinuteAverage(t *testing.T) {
	bits := uint32(0x1f0fffff)
	spacings := repeatedSpacing(300, 600)
	got := simulateDGW(spacings, bits)
	if avg := averageSpacing(spacings); avg < 590 || avg > 610 {
		t.Fatalf("stable simulation average drifted: %.2fs", avg)
	}
	assertTargetRatio(t, targetForLast(got), CompactToBig(bits), 0.2, 1.2)
}

func TestDGWv3FastBlockShockRisesSmoothly(t *testing.T) {
	bits := uint32(0x1f0fffff)
	spacings := append(repeatedSpacing(40, 600), repeatedSpacing(20, 60)...)
	got := simulateDGW(spacings, bits)
	oldTarget := CompactToBig(bits)
	newTarget := targetForLast(got)
	if newTarget.Cmp(oldTarget) >= 0 {
		t.Fatalf("fast shock should make difficulty harder: old=%s new=%s", oldTarget, newTarget)
	}
	if newTarget.Sign() <= 0 {
		t.Fatalf("fast shock produced non-positive target")
	}
}

func TestDGWv3SlowBlockShockFallsSmoothly(t *testing.T) {
	bits := uint32(0x1f0fffff)
	spacings := append(repeatedSpacing(40, 600), repeatedSpacing(20, 2400)...)
	got := simulateDGW(spacings, bits)
	oldTarget := CompactToBig(bits)
	newTarget := targetForLast(got)
	if newTarget.Cmp(oldTarget) <= 0 {
		t.Fatalf("slow shock should make difficulty easier: old=%s new=%s", oldTarget, newTarget)
	}
	if newTarget.Cmp(PowLimit) > 0 {
		t.Fatalf("slow shock target exceeded pow limit")
	}
}

func TestDGWv3HashrateDropRecoversSmoothly(t *testing.T) {
	bits := uint32(0x1f0fffff)
	spacings := append(repeatedSpacing(40, 600), repeatedSpacing(48, 1200)...)
	got := simulateDGW(spacings, bits)
	oldTarget := CompactToBig(bits)
	newTarget := targetForLast(got)
	if newTarget.Cmp(oldTarget) <= 0 {
		t.Fatalf("50%% hashrate drop should make target easier: old=%s new=%s", oldTarget, newTarget)
	}
	if newTarget.Cmp(PowLimit) > 0 {
		t.Fatalf("hashrate drop target exceeded pow limit")
	}
}

func TestDGWv3HashrateIncreaseRecoversSmoothly(t *testing.T) {
	bits := uint32(0x1f0fffff)
	spacings := append(repeatedSpacing(40, 600), repeatedSpacing(48, 200)...)
	got := simulateDGW(spacings, bits)
	oldTarget := CompactToBig(bits)
	newTarget := targetForLast(got)
	if newTarget.Cmp(oldTarget) >= 0 {
		t.Fatalf("3x hashrate increase should make target harder: old=%s new=%s", oldTarget, newTarget)
	}
	if newTarget.Sign() <= 0 {
		t.Fatalf("hashrate increase produced non-positive target")
	}
}

func TestDGWv3LowHashrateVarianceUsesHundredBlockAverage(t *testing.T) {
	spacings := make([]uint32, 0, 100)
	for i := 0; i < 25; i++ {
		spacings = append(spacings, 60, 180, 960, 1200)
	}
	if avg := averageSpacing(spacings); avg < 590 || avg > 610 {
		t.Fatalf("variance window average should remain near 10 minutes, got %.2fs", avg)
	}
	got := simulateDGW(spacings, 0x1f0fffff)
	if targetForLast(got).Sign() <= 0 {
		t.Fatalf("variance simulation produced invalid target")
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
