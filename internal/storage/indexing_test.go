package storage

import (
	"encoding/hex"
	"os"
	"testing"

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/wire"
)

func TestTxIndexLookupAfterSaveBlock(t *testing.T) {
	store := NewFileStore(t.TempDir())
	store.SetIndexOptions(true, false)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: ^uint32(0)},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 50, PkScript: []byte{script.OP_1}}},
	}
	block := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 1, Bits: 0x1e7fffff, Nonce: 1},
		Transactions: []*wire.MsgTx{tx},
	}
	bh, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: 0, Hash: bh.String(), Time: 1, Bits: 0x1e7fffff, Nonce: 1, Parent: "", ChainWork: "1"}
	if err := store.SaveBlock(block, idx, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	txHash, err := tx.TxHash()
	if err != nil {
		t.Fatal(err)
	}
	rec, err := store.LookupTxIndex(txHash.String())
	if err != nil {
		t.Fatalf("LookupTxIndex failed: %v", err)
	}
	if rec.TxID != txHash.String() || rec.BlockHash != idx.Hash || rec.BlockHeight != idx.Height || rec.TxPosition != 0 {
		t.Fatalf("unexpected txindex record: %+v", rec)
	}
}

func TestAddressIndexLookupAfterSaveBlock(t *testing.T) {
	store := NewFileStore(t.TempDir())
	store.SetIndexOptions(false, true)

	pubHash := make([]byte, 20)
	for i := range pubHash {
		pubHash[i] = byte(i + 1)
	}
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		t.Fatal(err)
	}
	addr := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, pubHash)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: ^uint32(0)},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 5000000000, PkScript: pkScript}},
	}
	block := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 1, Bits: 0x1e7fffff, Nonce: 2},
		Transactions: []*wire.MsgTx{tx},
	}
	bh, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: 0, Hash: bh.String(), Time: 1, Bits: 0x1e7fffff, Nonce: 2, Parent: "", ChainWork: "1"}
	txHash, err := tx.TxHash()
	if err != nil {
		t.Fatal(err)
	}
	adds := []blockchain.UTXOEntry{{
		Key:      blockchain.OutPointKey(txHash.String(), 0),
		TxID:     txHash.String(),
		Vout:     0,
		Value:    5000000000,
		PkScript: hex.EncodeToString(pkScript),
		Height:   0,
		Coinbase: true,
	}}
	if err := store.SaveBlock(block, idx, adds, nil, nil); err != nil {
		t.Fatal(err)
	}

	txids, err := store.AddressTxIDs(addr)
	if err != nil {
		t.Fatalf("AddressTxIDs failed: %v", err)
	}
	if len(txids) != 1 || txids[0] != txHash.String() {
		t.Fatalf("unexpected address txids: %#v", txids)
	}
	utxos, err := store.AddressUTXOs(addr)
	if err != nil {
		t.Fatalf("AddressUTXOs failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].Value != 5000000000 {
		t.Fatalf("unexpected address utxos: %#v", utxos)
	}
	confirmed, total, err := store.AddressBalance(addr)
	if err != nil {
		t.Fatalf("AddressBalance failed: %v", err)
	}
	if confirmed != 5000000000 || total != 5000000000 {
		t.Fatalf("unexpected address balance confirmed=%d total=%d", confirmed, total)
	}
}

func TestTxIndexSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	store.SetIndexOptions(true, false)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: ^uint32(0)},
			SignatureScript:  []byte{0x01, 0x02},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 42, PkScript: []byte{script.OP_1}}},
	}
	block := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 2, Bits: 0x1e7fffff, Nonce: 10},
		Transactions: []*wire.MsgTx{tx},
	}
	bh, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: 0, Hash: bh.String(), Time: 2, Bits: 0x1e7fffff, Nonce: 10, Parent: "", ChainWork: "1"}
	if err := store.SaveBlock(block, idx, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	txHash, err := tx.TxHash()
	if err != nil {
		t.Fatal(err)
	}

	reopened := NewFileStore(dir)
	reopened.SetIndexOptions(true, false)
	rec, err := reopened.LookupTxIndex(txHash.String())
	if err != nil {
		t.Fatalf("LookupTxIndex after restart failed: %v", err)
	}
	if rec.BlockHash != idx.Hash || rec.BlockHeight != idx.Height {
		t.Fatalf("unexpected txindex record after restart: %+v", rec)
	}
}

func TestRepairIndexesRebuildsTxAndAddressIndexes(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	store.SetIndexOptions(true, true)

	pubHash := make([]byte, 20)
	for i := range pubHash {
		pubHash[i] = byte(0x30 + i)
	}
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		t.Fatal(err)
	}
	addr := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, pubHash)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: ^uint32(0)},
			SignatureScript:  []byte{0x51},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 123_000_000, PkScript: pkScript}},
	}
	block := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 3, Bits: 0x1e7fffff, Nonce: 11},
		Transactions: []*wire.MsgTx{tx},
	}
	bh, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: 0, Hash: bh.String(), Time: 3, Bits: 0x1e7fffff, Nonce: 11, Parent: "", ChainWork: "1"}
	txHash, err := tx.TxHash()
	if err != nil {
		t.Fatal(err)
	}
	adds := []blockchain.UTXOEntry{{
		Key:      blockchain.OutPointKey(txHash.String(), 0),
		TxID:     txHash.String(),
		Vout:     0,
		Value:    123_000_000,
		PkScript: hex.EncodeToString(pkScript),
		Height:   0,
		Coinbase: true,
	}}
	if err := store.SaveBlock(block, idx, adds, nil, nil); err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(store.txIndexPath(txHash.String())); err != nil {
		t.Fatalf("remove txindex path: %v", err)
	}
	if err := os.RemoveAll(store.addressDir(addr)); err != nil {
		t.Fatalf("remove address index dir: %v", err)
	}

	if err := store.RepairIndexes(); err != nil {
		t.Fatalf("RepairIndexes failed: %v", err)
	}

	rec, err := store.LookupTxIndex(txHash.String())
	if err != nil {
		t.Fatalf("LookupTxIndex after RepairIndexes failed: %v", err)
	}
	if rec.TxID != txHash.String() || rec.BlockHash != idx.Hash {
		t.Fatalf("unexpected rebuilt txindex record: %+v", rec)
	}

	utxos, err := store.AddressUTXOs(addr)
	if err != nil {
		t.Fatalf("AddressUTXOs after RepairIndexes failed: %v", err)
	}
	if len(utxos) != 1 || utxos[0].TxID != txHash.String() || utxos[0].Value != 123_000_000 {
		t.Fatalf("unexpected rebuilt address utxos: %#v", utxos)
	}
}

func TestAddressHistoryTracksSpendsAndSurvivesRepair(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	store.SetIndexOptions(false, true)

	pubHashA := bytesRepeat(0x41)
	pubHashB := bytesRepeat(0x51)
	pkScriptA, err := script.PayToPubKeyHashScript(pubHashA)
	if err != nil {
		t.Fatal(err)
	}
	pkScriptB, err := script.PayToPubKeyHashScript(pubHashB)
	if err != nil {
		t.Fatal(err)
	}
	addrA := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, pubHashA)
	addrB := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, pubHashB)

	tx1 := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: ^uint32(0)},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 5_000_000_000, PkScript: pkScriptA}},
	}
	tx1Hash, err := tx1.TxHash()
	if err != nil {
		t.Fatal(err)
	}
	block1 := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 10, Bits: 0x1e7fffff, Nonce: 21},
		Transactions: []*wire.MsgTx{tx1},
	}
	block1Hash, err := block1.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx1 := blockchain.BlockIndex{Height: 0, Hash: block1Hash.String(), Time: 10, Bits: 0x1e7fffff, Nonce: 21, Parent: "", ChainWork: "1"}
	adds1 := []blockchain.UTXOEntry{{
		Key:      blockchain.OutPointKey(tx1Hash.String(), 0),
		TxID:     tx1Hash.String(),
		Vout:     0,
		Value:    5_000_000_000,
		PkScript: hex.EncodeToString(pkScriptA),
		Height:   0,
		Coinbase: true,
	}}
	if err := store.SaveBlock(block1, idx1, adds1, nil, nil); err != nil {
		t.Fatal(err)
	}

	tx2 := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: tx1Hash, Index: 0},
			SignatureScript:  []byte{0x51},
			Sequence:         ^uint32(0),
		}},
		TxOut: []wire.TxOut{{Value: 4_900_000_000, PkScript: pkScriptB}},
	}
	tx2Hash, err := tx2.TxHash()
	if err != nil {
		t.Fatal(err)
	}
	block2 := &wire.MsgBlock{
		Header:       wire.BlockHeader{Version: 1, Timestamp: 20, Bits: 0x1e7fffff, Nonce: 22},
		Transactions: []*wire.MsgTx{tx2},
	}
	block2Hash, err := block2.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx2 := blockchain.BlockIndex{Height: 1, Hash: block2Hash.String(), Time: 20, Bits: 0x1e7fffff, Nonce: 22, Parent: idx1.Hash, ChainWork: "2"}
	adds2 := []blockchain.UTXOEntry{{
		Key:      blockchain.OutPointKey(tx2Hash.String(), 0),
		TxID:     tx2Hash.String(),
		Vout:     0,
		Value:    4_900_000_000,
		PkScript: hex.EncodeToString(pkScriptB),
		Height:   1,
		Coinbase: false,
	}}
	if err := store.SaveBlock(block2, idx2, adds2, []string{adds1[0].Key}, []blockchain.UTXOEntry{adds1[0]}); err != nil {
		t.Fatal(err)
	}

	utxosA, err := store.AddressUTXOs(addrA)
	if err != nil {
		t.Fatalf("AddressUTXOs A: %v", err)
	}
	if len(utxosA) != 0 {
		t.Fatalf("address A should have no spendable utxos after spend: %#v", utxosA)
	}
	txidsA, err := store.AddressTxIDs(addrA)
	if err != nil {
		t.Fatalf("AddressTxIDs A: %v", err)
	}
	if !contains(txidsA, tx1Hash.String()) || !contains(txidsA, tx2Hash.String()) {
		t.Fatalf("address A txids should include receive and spend txids: %#v", txidsA)
	}
	historyA, err := store.AddressHistory(addrA)
	if err != nil {
		t.Fatalf("AddressHistory A: %v", err)
	}
	if len(historyA) != 1 {
		t.Fatalf("address A history len=%d want 1 output record", len(historyA))
	}
	if !historyA[0].Spent || historyA[0].SpendTxID != tx2Hash.String() || historyA[0].SpendHeight != 1 {
		t.Fatalf("address A history spent metadata mismatch: %#v", historyA[0])
	}

	historyB, err := store.AddressHistory(addrB)
	if err != nil {
		t.Fatalf("AddressHistory B: %v", err)
	}
	if len(historyB) != 1 || historyB[0].Spent {
		t.Fatalf("address B history mismatch: %#v", historyB)
	}

	if err := os.RemoveAll(store.addressDir(addrA)); err != nil {
		t.Fatalf("remove address index A: %v", err)
	}
	if err := os.RemoveAll(store.addressDir(addrB)); err != nil {
		t.Fatalf("remove address index B: %v", err)
	}
	if err := store.RepairIndexes(); err != nil {
		t.Fatalf("RepairIndexes: %v", err)
	}
	historyA2, err := store.AddressHistory(addrA)
	if err != nil {
		t.Fatalf("AddressHistory A after repair: %v", err)
	}
	if len(historyA2) != 1 || !historyA2[0].Spent || historyA2[0].SpendTxID != tx2Hash.String() {
		t.Fatalf("address A repaired history mismatch: %#v", historyA2)
	}
}

func bytesRepeat(b byte) []byte {
	out := make([]byte, 20)
	for i := range out {
		out[i] = b
	}
	return out
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
