package pow

import (
	"encoding/hex"
	"testing"
)

func TestYespowerHashDeterministic(t *testing.T) {
	input := []byte("legacycoin yespower pure go smoke test")
	first := yespowerHash(input, "LegacyCoinPoW")
	second := yespowerHash(input, "LegacyCoinPoW")
	if first != second {
		t.Fatalf("yespower hash is not deterministic: %x != %x", first, second)
	}

	want, err := hex.DecodeString("3ead859dbf546ef85df4500e96c6582eb206ed1ba05fb64feed22b3bd73d04ed")
	if err != nil {
		t.Fatal(err)
	}
	if string(first[:]) != string(want) {
		t.Fatalf("yespower hash mismatch: got %x, want %x", first[:], want)
	}
}
