package consensus

import "testing"

func BenchmarkWorkForBits(b *testing.B) {
	const bits uint32 = 0x1f0fffff

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = WorkForBits(bits)
	}
}

func BenchmarkCompactToBig(b *testing.B) {
	const bits uint32 = 0x1f0fffff

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CompactToBig(bits)
	}
}
