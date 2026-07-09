package wire

import (
	"bytes"
	"testing"
)

func FuzzReadTx(f *testing.F) {
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 1, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadTx(bytes.NewReader(data))
	})
}

func FuzzReadBlock(f *testing.F) {
	f.Add([]byte{})
	f.Add(bytes.Repeat([]byte{0}, 80))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadBlock(bytes.NewReader(data))
	})
}

func FuzzReadMessage(f *testing.F) {
	magic := [4]byte{0xa4, 0xac, 0xc6, 0x4d}
	var seed bytes.Buffer
	_ = WriteMessage(&seed, magic, CommandPing, []byte("seed"))
	f.Add(seed.Bytes())
	f.Add([]byte{0, 1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReadMessage(bytes.NewReader(data), magic)
	})
}
