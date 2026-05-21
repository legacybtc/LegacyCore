package genesis

import (
	"testing"

	"legacycoin/legacy-go/internal/chaincfg"
)

func TestNewBlock(t *testing.T) {
	block, err := NewBlock(chaincfg.MainNet)
	if err != nil {
		t.Fatal(err)
	}
	if len(block.Transactions) != 1 {
		t.Fatalf("tx count=%d", len(block.Transactions))
	}
	if block.Header.Bits != 0x207fffff {
		t.Fatalf("bits=%08x", block.Header.Bits)
	}
	if block.Header.Timestamp != 1779235200 {
		t.Fatalf("time=%d", block.Header.Timestamp)
	}
	if block.Header.Nonce != 3 {
		t.Fatalf("nonce=%d", block.Header.Nonce)
	}
	if block.Header.MerkleRoot.IsZero() {
		t.Fatal("empty merkle root")
	}
}
