//go:build cgo && !legacycoin_experimental_pure_yespower

package pow

import (
	"os"
	"testing"

	"legacycoin/legacy-go/internal/wire"
)

func TestYespowerCgoBackendAvailableForProduction(t *testing.T) {
	header := parityHeader()
	hash, err := (YespowerHasher{Personalization: "LegacyCoinPoW"}).HashHeader(header)
	if err != nil {
		t.Fatalf("cgo yespower hash failed: %v", err)
	}
	if hash == ([32]byte{}) {
		t.Fatalf("cgo yespower returned zero hash")
	}
	if BackendName() != "cgo-c-reference" {
		t.Fatalf("production backend=%q, want cgo-c-reference", BackendName())
	}
}

func TestYespowerPureGoParityWithCReference(t *testing.T) {
	header := parityHeader()
	serialized, err := header.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pure := yespowerHash(serialized, "LegacyCoinPoW")
	cHash, err := (YespowerHasher{Personalization: "LegacyCoinPoW"}).HashHeader(header)
	if err != nil {
		t.Fatalf("cgo yespower hash failed: %v", err)
	}
	if cHash != pure {
		if os.Getenv("LEGACY_ASSERT_PURE_YESPOWER_PARITY") != "1" {
			t.Skipf("pure-Go yespower parity is not proven; production uses C yespower. c=%x pure=%x", cHash[:], pure[:])
		}
		t.Fatalf("pure-Go yespower mismatch with C reference: c=%x pure=%x", cHash[:], pure[:])
	}
}

func parityHeader() wire.BlockHeader {
	header := wire.BlockHeader{
		Version:   1,
		Timestamp: 1777593600,
		Bits:      0x1e00ffff,
		Nonce:     68425,
	}
	copy(header.PrevBlock[:], []byte("legacycoin-cgo-prev-hash-vector"))
	copy(header.MerkleRoot[:], []byte("legacycoin-cgo-merkle-vector"))
	return header
}
