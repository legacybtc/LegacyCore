package wire

import (
	"bytes"
	"testing"

	"legacycoin/legacy-go/internal/chainhash"
)

func TestInventoryRoundTrip(t *testing.T) {
	hash := chainhash.DoubleHashB([]byte("legacy block"))
	payload, err := InvPayload([]InvVect{{Type: InvTypeBlock, Hash: hash}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadInvPayload(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Type != InvTypeBlock || got[0].Hash != hash {
		t.Fatalf("inventory mismatch: %+v", got)
	}
}

func TestGetBlocksRoundTrip(t *testing.T) {
	locator := []chainhash.Hash{chainhash.DoubleHashB([]byte("tip"))}
	msg := GetBlocks{Version: 70015, Locator: locator}
	payload, err := msg.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadGetBlocks(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != msg.Version || len(got.Locator) != 1 || got.Locator[0] != locator[0] {
		t.Fatalf("getblocks mismatch: %+v", got)
	}
}

func TestHeadersRoundTrip(t *testing.T) {
	headers := []BlockHeader{{
		Version:   1,
		Timestamp: 1777501200,
		Bits:      0x1e7fffff,
		Nonce:     187769,
	}}
	payload, err := HeadersPayload(headers)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadHeadersPayload(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != headers[0] {
		t.Fatalf("headers mismatch: %+v", got)
	}
}
