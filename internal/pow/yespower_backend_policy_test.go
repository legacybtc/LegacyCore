//go:build !cgo || legacycoin_experimental_pure_yespower

package pow

import (
	"testing"

	"legacycoin/legacy-go/internal/wire"
)

func TestYespowerHasherUsesPureGoBackendOnlyForExperimentalOrNoCGO(t *testing.T) {
	header := wire.BlockHeader{Version: 1, Bits: 0x1f00ffff, Timestamp: 1714435200, Nonce: 42}
	got, err := (YespowerHasher{Personalization: "LegacyCoinPoW"}).HashHeader(header)
	if err != nil {
		t.Fatalf("hash header: %v", err)
	}
	b, err := header.Bytes()
	if err != nil {
		t.Fatalf("header bytes: %v", err)
	}
	want := yespowerHash(b, "LegacyCoinPoW")
	if got != want {
		t.Fatalf("YespowerHasher must use pure-Go consensus backend: got %x want %x", got[:], want[:])
	}
	if BackendName() != "pure-go-experimental" {
		t.Fatalf("unexpected pure-go backend name %q", BackendName())
	}
}
