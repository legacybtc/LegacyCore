package rpc

import (
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/wire"
)

func TestBlockTemplateTransactionsFromEntries(t *testing.T) {
	coinbase := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32},
			Sequence:         math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{Value: 50_0000_0000}},
	}
	tx1 := simpleTemplateTx(1)
	tx2 := simpleTemplateTx(2)
	h1, _ := tx1.TxHash()
	h2, _ := tx2.TxHash()
	block := &wire.MsgBlock{Transactions: []*wire.MsgTx{coinbase, tx1, tx2}}
	rows := blockTemplateTransactionsFromEntries(block, []mempool.Entry{
		{TxID: h1.String(), Fee: 111, Size: 333},
		{TxID: h2.String(), Fee: 222, Size: 444},
	})
	if len(rows) != 2 {
		t.Fatalf("expected 2 non-coinbase tx rows, got %d", len(rows))
	}
	if rows[0]["txid"] != h1.String() || rows[1]["txid"] != h2.String() {
		t.Fatalf("unexpected txid order: %+v", rows)
	}
	if rows[0]["fee"] != int64(111) || rows[0]["size"] != 333 {
		t.Fatalf("unexpected first tx fee/size: %+v", rows[0])
	}
	if rows[0]["sigops"] != 0 || rows[0]["weight"] != 333*4 {
		t.Fatalf("unexpected first tx sigops/weight: %+v", rows[0])
	}
	if deps, ok := rows[0]["depends"].([]int); !ok || len(deps) != 0 {
		t.Fatalf("unexpected first tx depends: %+v", rows[0]["depends"])
	}
	if rows[1]["fee"] != int64(222) || rows[1]["size"] != 444 {
		t.Fatalf("unexpected second tx fee/size: %+v", rows[1])
	}
	raw1, _ := tx1.Bytes()
	if rows[0]["data"] != hex.EncodeToString(raw1) {
		t.Fatalf("unexpected tx data for row 0")
	}
	if rows[0]["txid"] == blockTxIDs(block)[0] {
		t.Fatalf("coinbase tx must be excluded from getblocktemplate transactions")
	}
}

func TestCompactTargetHex(t *testing.T) {
	bits := uint32(0x1e0ffff0)
	want := fmt.Sprintf("%064x", consensus.CompactToBig(bits))
	got := compactTargetHex(bits)
	if got != want {
		t.Fatalf("target mismatch: got %s want %s", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("target must be 64 hex chars, got %d", len(got))
	}
	if compactTargetHex(0) != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("zero bits should return zero target")
	}
}

func simpleTemplateTx(tag byte) *wire.MsgTx {
	return &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{
				Hash:  chainhash.Hash{tag},
				Index: 0,
			},
			SignatureScript: []byte{0x51, tag},
			Sequence:        math.MaxUint32 - 1,
		}},
		TxOut: []wire.TxOut{{
			Value:    int64(1_000 + int(tag)),
			PkScript: []byte{0x51, tag},
		}},
	}
}
