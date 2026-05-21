package wire

import (
	"bytes"
	"testing"

	"legacycoin/legacy-go/internal/chainhash"
)

func TestBlockRoundTrip(t *testing.T) {
	block := &MsgBlock{
		Header: BlockHeader{Version: 1, Timestamp: 123, Bits: 0x1e7fffff, Nonce: 7},
		Transactions: []*MsgTx{{
			Version: 1,
			TxIn: []TxIn{{
				PreviousOutPoint: OutPoint{Hash: chainhash.Hash{}, Index: 0xffffffff},
				SignatureScript:  []byte("coinbase"),
				Sequence:         0xffffffff,
			}},
			TxOut: []TxOut{{Value: 50, PkScript: []byte{0x51}}},
		}},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}
	block.Header.MerkleRoot = root

	var buf bytes.Buffer
	if err := block.Serialize(&buf); err != nil {
		t.Fatal(err)
	}
	got, err := ReadBlock(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if got.Header != block.Header {
		t.Fatalf("header mismatch: got %+v want %+v", got.Header, block.Header)
	}
	if len(got.Transactions) != 1 || len(got.Transactions[0].TxIn) != 1 || len(got.Transactions[0].TxOut) != 1 {
		t.Fatalf("bad tx shape: %+v", got.Transactions)
	}
}
