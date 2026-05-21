package consensus

import "testing"

func TestCompactToBig(t *testing.T) {
	target := CompactToBig(0x1e7fffff)
	if target.Sign() <= 0 {
		t.Fatal("target must be positive")
	}
	if target.Cmp(PowLimit) > 0 {
		t.Fatal("canonical compact target exceeds pow limit")
	}
}

func TestBigToCompactRoundTrip(t *testing.T) {
	for _, bits := range []uint32{0x1e7fffff, 0x1d00ffff, 0x1b0404cb} {
		if got := BigToCompact(CompactToBig(bits)); got != bits {
			t.Fatalf("round trip %08x got %08x", bits, got)
		}
	}
}

func TestDarkGravityWaveV3NeedsEnoughHistory(t *testing.T) {
	got := DarkGravityWaveV3(nil, 600, PowLimit, 0x1e7fffff)
	if got != 0x1e7fffff {
		t.Fatalf("bits=%08x", got)
	}
}

func TestDarkGravityWaveV3StableSpacing(t *testing.T) {
	recent := make([]BlockWindowEntry, DGWv3PastBlocks)
	start := uint32(1_000_000)
	for i := range recent {
		recent[i] = BlockWindowEntry{
			Height: int32(DGWv3PastBlocks - 1 - i),
			Time:   start - uint32(i*600),
			Bits:   0x1d00ffff,
		}
	}
	got := DarkGravityWaveV3(recent, 600, PowLimit, 0x1e7fffff)
	if got != 0x1d00f554 {
		t.Fatalf("stable spacing changed bits: got %08x", got)
	}
}

func TestDarkGravityWaveV3ClampsExtremeTimespan(t *testing.T) {
	recent := make([]BlockWindowEntry, DGWv3PastBlocks)
	start := uint32(1_000_000)
	for i := range recent {
		recent[i] = BlockWindowEntry{
			Height: int32(DGWv3PastBlocks - 1 - i),
			Time:   start - uint32(i*10),
			Bits:   0x1d00ffff,
		}
	}
	got := DarkGravityWaveV3(recent, 600, PowLimit, 0x1e7fffff)
	if got != 0x1c555500 {
		t.Fatalf("fast blocks bits=%08x", got)
	}
}
