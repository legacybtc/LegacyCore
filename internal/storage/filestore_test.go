package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/wire"
)

func TestRecoverConnectJournalRollsBackPartialBlockCommit(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	idx := blockchain.BlockIndex{Height: 1, Hash: "abc", Time: 1, Bits: 1, Nonce: 1}
	added := blockchain.UTXOEntry{Key: "newtx:0", TxID: "newtx", Vout: 0, Value: 50}
	spent := blockchain.UTXOEntry{Key: "oldtx:0", TxID: "oldtx", Vout: 0, Value: 25}
	if err := store.ensureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.blockPath(idx.Hash), []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(idx)
	if err := os.WriteFile(store.hashIndexPath(idx.Hash), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.heightIndexPath(idx.Height), b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.writeUTXO(added); err != nil {
		t.Fatal(err)
	}
	if err := store.writeJournal(storeJournal{Op: journalConnect, Index: idx, Adds: []blockchain.UTXOEntry{added}, SpentEntries: []blockchain.UTXOEntry{spent}}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadTip(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.journalPath()); !os.IsNotExist(err) {
		t.Fatalf("journal still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(store.blockPath(idx.Hash)); !os.IsNotExist(err) {
		t.Fatalf("partial block still exists or stat failed: %v", err)
	}
	if _, err := os.Stat(store.utxoPath(added.Key)); !os.IsNotExist(err) {
		t.Fatalf("added UTXO still exists or stat failed: %v", err)
	}
	if _, err := store.LoadUTXO(spent.Key); err != nil {
		t.Fatalf("spent UTXO not restored: %v", err)
	}
}

func TestRecoverConnectJournalCommittedTipClearsStaleJournal(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	idx := blockchain.BlockIndex{Height: 2, Hash: "committed", Time: 2, Bits: 1, Nonce: 1}
	if err := store.SaveTip(idx); err != nil {
		t.Fatal(err)
	}
	if err := store.writeJournal(storeJournal{Op: journalConnect, Index: idx}); err != nil {
		t.Fatal(err)
	}
	tip, err := store.LoadTip()
	if err != nil {
		t.Fatal(err)
	}
	if tip == nil || tip.Hash != idx.Hash {
		t.Fatalf("unexpected tip: %+v", tip)
	}
	if _, err := os.Stat(store.journalPath()); !os.IsNotExist(err) {
		t.Fatalf("stale journal still exists or stat failed: %v", err)
	}
}

func TestRecoverCorruptJournalFailsClosed(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.journalPath(), []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadTip(); err == nil {
		t.Fatalf("expected corrupt journal recovery error")
	}
}

func TestRecoverDisconnectJournalRestoresPreviousTipAndUTXO(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	prev := blockchain.BlockIndex{Height: 4, Hash: "prev", Time: 4, Bits: 1, Nonce: 1}
	cur := blockchain.BlockIndex{Height: 5, Hash: "cur", Time: 5, Bits: 1, Nonce: 2}
	added := blockchain.UTXOEntry{Key: "added:0", TxID: "added", Vout: 0, Value: 50}
	spent := blockchain.UTXOEntry{Key: "spent:0", TxID: "spent", Vout: 0, Value: 25}
	if err := store.ensureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTip(cur); err != nil {
		t.Fatal(err)
	}
	if err := store.writeUTXO(added); err != nil {
		t.Fatal(err)
	}
	undo := blockchain.UndoData{AddedKeys: []string{added.Key}, Spent: []blockchain.UTXOEntry{spent}}
	if err := store.writeJournal(storeJournal{Op: journalDisconnect, Index: cur, PrevTip: &prev, Undo: &undo}); err != nil {
		t.Fatal(err)
	}
	tip, err := store.LoadTip()
	if err != nil {
		t.Fatal(err)
	}
	if tip == nil || tip.Hash != prev.Hash || tip.Height != prev.Height {
		t.Fatalf("unexpected recovered tip: %+v", tip)
	}
	if _, err := os.Stat(store.utxoPath(added.Key)); !os.IsNotExist(err) {
		t.Fatalf("disconnected added UTXO still exists or stat failed: %v", err)
	}
	if _, err := store.LoadUTXO(spent.Key); err != nil {
		t.Fatalf("spent UTXO not restored: %v", err)
	}
}

func TestLoadIndexByHeightRepairsMissingActiveHeightIndex(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := store.ensureDirs(); err != nil {
		t.Fatal(err)
	}

	genesisBlock := &wire.MsgBlock{Header: wire.BlockHeader{Version: 1, Timestamp: 1, Bits: 1, Nonce: 1}}
	genesisHash, err := genesisBlock.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	genesisIdx := blockchain.BlockIndex{Height: 0, Hash: genesisHash.String(), Time: 1, Bits: 1, Nonce: 1}
	if err := store.SaveBlock(genesisBlock, genesisIdx, nil, nil, nil); err != nil {
		t.Fatal(err)
	}

	block1 := &wire.MsgBlock{Header: wire.BlockHeader{Version: 1, PrevBlock: genesisHash, Timestamp: 2, Bits: 1, Nonce: 2}}
	block1Hash, err := block1.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx1 := blockchain.BlockIndex{Height: 1, Hash: block1Hash.String(), Time: 2, Bits: 1, Nonce: 2}
	if err := store.SaveBlock(block1, idx1, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(store.heightIndexPath(1)); err != nil {
		t.Fatal(err)
	}

	repaired, err := store.LoadIndexByHeight(1)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.Hash != idx1.Hash || repaired.Height != idx1.Height {
		t.Fatalf("unexpected repaired index: %+v", repaired)
	}
	if _, err := os.Stat(store.heightIndexPath(1)); err != nil {
		t.Fatalf("height index was not recreated: %v", err)
	}
}

func TestEmptyDatadirStartupReadsAsNoTip(t *testing.T) {
	store := NewFileStore(t.TempDir())
	tip, err := store.LoadTip()
	if err != nil {
		t.Fatalf("LoadTip on empty datadir: %v", err)
	}
	if tip != nil {
		t.Fatalf("tip=%+v want nil", tip)
	}
	stats, err := store.UTXOStats()
	if err != nil {
		t.Fatalf("UTXOStats on empty datadir: %v", err)
	}
	if stats.Count != 0 || stats.Total != 0 {
		t.Fatalf("stats=%+v want zero", stats)
	}
}

func TestSaveBlockCreatesStorageParents(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "missing", "chain"))
	block := &wire.MsgBlock{Header: wire.BlockHeader{Version: 1, Timestamp: 1, Bits: 1, Nonce: 1}}
	hash, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: 0, Hash: hash.String(), Time: 1, Bits: 1, Nonce: 1}
	if err := store.SaveBlock(block, idx, nil, nil, nil); err != nil {
		t.Fatalf("SaveBlock: %v", err)
	}
	for _, path := range []string{
		store.blocksDir(),
		filepath.Dir(store.hashIndexPath(hash.String())),
		filepath.Dir(store.heightIndexPath(0)),
		store.utxoDir(),
		store.undoDir(),
	} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("expected directory %s, info=%v err=%v", path, info, err)
		}
	}
}

func TestLoadIndexByHeightReportsCorruptIndex(t *testing.T) {
	store := NewFileStore(t.TempDir())
	if err := store.ensureDirs(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.heightIndexPath(7), []byte("{not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadIndexByHeight(7); err == nil {
		t.Fatalf("expected corrupt height index error")
	}
}
