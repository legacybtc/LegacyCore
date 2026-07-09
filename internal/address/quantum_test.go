package address

import (
	"bytes"
	"strings"
	"testing"
)

func TestHybridAddressRoundTrip(t *testing.T) {
	addr := NewHybridAddress([]byte("secp-pub"), []byte("mldsa-pub"))
	if !strings.HasPrefix(addr, HybridPrefix) {
		t.Fatalf("address prefix: %s", addr)
	}
	hash, err := DecodeHybridAddress(addr)
	if err != nil {
		t.Fatal(err)
	}
	if len(hash) != HashSize {
		t.Fatalf("hash length=%d", len(hash))
	}
	hash2, err := DecodeHybridAddress(addr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(hash, hash2) {
		t.Fatal("address decode is not deterministic")
	}
}

func TestHybridAddressRejectsBadPrefix(t *testing.T) {
	addr := NewHybridAddress([]byte("secp-pub"), []byte("mldsa-pub"))
	if _, err := DecodeHybridAddress("L" + strings.TrimPrefix(addr, HybridPrefix)); err == nil {
		t.Fatal("expected bad prefix")
	}
}
