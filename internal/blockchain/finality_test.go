package blockchain_test

import (
	"math"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/wire"
)

func TestIsFinalizedTx(t *testing.T) {
	base := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			Sequence: math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{Value: 1}},
	}

	if !blockchain.IsFinalizedTx(base, 100, 2_000_000_000) {
		t.Fatalf("locktime=0 tx should be final")
	}

	byHeight := *base
	byHeight.LockTime = 99
	if !blockchain.IsFinalizedTx(&byHeight, 100, 2_000_000_000) {
		t.Fatalf("height-final tx rejected")
	}

	nonFinalByHeight := *base
	nonFinalByHeight.LockTime = 100
	nonFinalByHeight.TxIn = []wire.TxIn{{Sequence: math.MaxUint32 - 1}}
	if blockchain.IsFinalizedTx(&nonFinalByHeight, 100, 2_000_000_000) {
		t.Fatalf("non-final height tx accepted")
	}

	byTime := *base
	byTime.LockTime = 1_700_000_000
	if !blockchain.IsFinalizedTx(&byTime, 100, 1_800_000_000) {
		t.Fatalf("time-final tx rejected")
	}
}
