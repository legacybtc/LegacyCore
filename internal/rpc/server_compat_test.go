package rpc

import (
	"encoding/json"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/wire"
)

func TestSubmitBlockRejectCodeMapping(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{blockchain.ErrBadPrevBlock, "bad-prevblk"},
		{blockchain.ErrBadMerkleRoot, "bad-txnmrklroot"},
		{blockchain.ErrBadBits, "bad-diffbits"},
		{blockchain.ErrTimeTooOld, "time-too-old"},
		{blockchain.ErrTimeTooNew, "time-too-new"},
		{consensus.ErrHighHash, "high-hash"},
	}
	for _, tc := range cases {
		if got := submitBlockRejectCode(tc.err); got != tc.want {
			t.Fatalf("reject code mismatch for %v: got %q want %q", tc.err, got, tc.want)
		}
	}
	if got := submitBlockRejectCode(nil); got != "" {
		t.Fatalf("nil error should map to empty code, got %q", got)
	}
}

func TestParseBoolish(t *testing.T) {
	tests := []struct {
		raw  string
		want bool
		ok   bool
	}{
		{`true`, true, true},
		{`false`, false, true},
		{`1`, true, true},
		{`0`, false, true},
		{`"yes"`, true, true},
		{`"off"`, false, true},
		{`"maybe"`, false, false},
	}
	for _, tc := range tests {
		got, ok := parseBoolish(json.RawMessage(tc.raw))
		if got != tc.want || ok != tc.ok {
			t.Fatalf("parseBoolish(%s) = (%v, %v), want (%v, %v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTxVerboseResultHasCoinbaseVin(t *testing.T) {
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Index: ^uint32(0)},
			SignatureScript:  []byte{0x51, 0x01},
		}},
		TxOut: []wire.TxOut{{
			Value:    1_000,
			PkScript: []byte{0x51},
		}},
	}
	h, err := tx.TxHash()
	if err != nil {
		t.Fatalf("tx hash: %v", err)
	}
	out := txVerboseResult(&txLookupResult{
		Tx:            tx,
		TxID:          h.String(),
		Confirmations: 0,
		BlockHeight:   -1,
		InMempool:     true,
	})
	vin, _ := out["vin"].([]map[string]any)
	if len(vin) != 1 {
		t.Fatalf("expected 1 vin, got %d", len(vin))
	}
	if _, ok := vin[0]["coinbase"]; !ok {
		t.Fatalf("expected coinbase vin marker in decoded transaction")
	}
}
