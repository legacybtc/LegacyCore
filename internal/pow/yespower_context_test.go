//go:build cgo && !legacycoin_experimental_pure_yespower

package pow

import (
	"testing"

	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

func TestYespowerContextHashEquivalent(t *testing.T) {
	if BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend")
	}
	hasher := YespowerHasher{Personalization: "LegacyCoinPoW"}

	headers := []wire.BlockHeader{
		{Version: 1, Bits: 0x1f0fffff, Nonce: 0, Timestamp: 1779235200},
		{Version: 1, Bits: 0x1f0fffff, Nonce: 1, Timestamp: 1779235201},
		{Version: 1, Bits: 0x1f0fffff, Nonce: 42, Timestamp: 1779235300},
		{Version: 2, Bits: 0x207fffff, Nonce: 3, Timestamp: 1779235200},
		{Version: 1, Bits: 0x1e0fffff, Nonce: 9999, Timestamp: 1779240000},
	}

	for i, hdr := range headers {
		tlsHash, err := hasher.HashHeader(hdr)
		if err != nil {
			t.Fatalf("header %d TLS hash: %v", i, err)
		}
		ctx := hasher.NewContext()
		localHash, err := hasher.hashWithContext(ctx, hdr)
		ctx.Close()
		if err != nil {
			t.Fatalf("header %d local hash: %v", i, err)
		}
		if tlsHash != localHash {
			t.Fatalf("header %d mismatch: TLS=%x local=%x", i, tlsHash[:], localHash[:])
		}
	}
}

func TestYespowerContextRepeatedUse(t *testing.T) {
	if BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend")
	}
	hasher := YespowerHasher{Personalization: "LegacyCoinPoW"}
	ctx := hasher.NewContext()
	defer ctx.Close()

	for i := 0; i < 500; i++ {
		hdr := wire.BlockHeader{Version: 1, Bits: 0x1f0fffff, Nonce: uint32(i), Timestamp: 1779235200}
		h1, _ := ctxHash(t, hasher, ctx, hdr)
		h2, _ := hasher.HashHeader(hdr)
		if h1 != h2 {
			t.Fatalf("repeated use iter %d: local=%x TLS=%x", i, h1[:], h2[:])
		}
	}
}

func TestYespowerContextLifecycle(t *testing.T) {
	if BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend")
	}
	hasher := YespowerHasher{Personalization: "LegacyCoinPoW"}
	hdr := wire.BlockHeader{Version: 1, Bits: 0x1f0fffff, Nonce: 0, Timestamp: 1779235200}

	prevInit := localInit.Load()
	prevFree := localFree.Load()
	for i := 0; i < 50; i++ {
		ctx := hasher.NewContext()
		hash, err := hasher.hashWithContext(ctx, hdr)
		ctx.Close()
		if err != nil {
			t.Fatalf("cycle %d: %v", i, err)
		}
		tlsHash, _ := hasher.HashHeader(hdr)
		if hash != tlsHash {
			t.Fatalf("cycle %d mismatch", i)
		}
	}

	deltaInit := localInit.Load() - prevInit
	deltaFree := localFree.Load() - prevFree
	if deltaInit != 50 {
		t.Fatalf("delta init=%d want 50", deltaInit)
	}
	if deltaFree != 50 {
		t.Fatalf("delta free=%d want 50", deltaFree)
	}
}

func TestYespowerContextDoubleClose(t *testing.T) {
	if BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend")
	}
	hasher := YespowerHasher{Personalization: "LegacyCoinPoW"}
	ctx := hasher.NewContext()
	ctx.Close()
	ctx.Close() // must not panic or double-free
}

func TestYespowerContextNilClose(t *testing.T) {
	var ctx *yespowerContext
	ctx.Close() // must not panic
}

func ctxHash(t *testing.T, hasher YespowerHasher, ctx HasherContext, hdr wire.BlockHeader) (chainhash.Hash, error) {
	t.Helper()
	return hasher.hashWithContext(ctx, hdr)
}
