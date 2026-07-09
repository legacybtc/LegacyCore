package wire

import (
	"bytes"
	"testing"
)

func BenchmarkReadTxMalformed(b *testing.B) {
	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 32)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ReadTx(bytes.NewReader(payload))
	}
}

func BenchmarkReadBlockMalformed(b *testing.B) {
	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 128)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ReadBlock(bytes.NewReader(payload))
	}
}

func BenchmarkReadMessageMalformed(b *testing.B) {
	payload := bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 128)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = ReadMessage(bytes.NewReader(payload), [4]byte{})
	}
}
