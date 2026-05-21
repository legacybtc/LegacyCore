package blockchain_test

import (
	"fmt"
	"strings"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

type failStore struct {
	base             *storage.FileStore
	failDisconnectOn string
	failSaveOn       string
}

func (s *failStore) LoadTip() (*blockchain.BlockIndex, error) { return s.base.LoadTip() }
func (s *failStore) SaveTip(tip blockchain.BlockIndex) error  { return s.base.SaveTip(tip) }
func (s *failStore) SaveBlock(block *wire.MsgBlock, idx blockchain.BlockIndex, adds []blockchain.UTXOEntry, spends []string, spent []blockchain.UTXOEntry) error {
	if s.failSaveOn != "" && idx.Hash == s.failSaveOn {
		return fmt.Errorf("injected save failure on %s", idx.Hash)
	}
	return s.base.SaveBlock(block, idx, adds, spends, spent)
}
func (s *failStore) DisconnectBlock(hash string, prevTip *blockchain.BlockIndex, undo blockchain.UndoData) error {
	if s.failDisconnectOn != "" && hash == s.failDisconnectOn {
		return fmt.Errorf("injected disconnect failure on %s", hash)
	}
	return s.base.DisconnectBlock(hash, prevTip, undo)
}
func (s *failStore) LoadBlock(hash string) (*wire.MsgBlock, *blockchain.BlockIndex, error) {
	return s.base.LoadBlock(hash)
}
func (s *failStore) LoadIndexByHeight(height int32) (*blockchain.BlockIndex, error) {
	return s.base.LoadIndexByHeight(height)
}
func (s *failStore) LoadUTXO(key string) (*blockchain.UTXOEntry, error) { return s.base.LoadUTXO(key) }
func (s *failStore) LoadUndo(hash string) (*blockchain.UndoData, error) { return s.base.LoadUndo(hash) }
func (s *failStore) ListUTXO() ([]blockchain.UTXOEntry, error)          { return s.base.ListUTXO() }
func (s *failStore) UTXOStats() (blockchain.UTXOStats, error)           { return s.base.UTXOStats() }

func TestReorgReportsRollbackFailure(t *testing.T) {
	params := chaincfg.MainNet
	fs := &failStore{base: storage.NewFileStore(t.TempDir())}
	chain, err := blockchain.New(params, fakeHasher{}, fs)
	if err != nil {
		t.Fatal(err)
	}

	var zero chainhash.Hash
	genesisLike, err := buildBlock(zero, 0, 401, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)

	main1, _ := buildBlock(genesisHash, 1, 402, false)
	if err := chain.ProcessBlock(main1); err != nil {
		t.Fatal(err)
	}
	main1Hash, _ := fakeHasher{}.HashHeader(main1.Header)
	main2, _ := buildBlock(main1Hash, 2, 403, false)
	if err := chain.ProcessBlock(main2); err != nil {
		t.Fatal(err)
	}

	side1, _ := buildBlock(genesisHash, 1, 411, false)
	if err := chain.ProcessBlock(side1); err != nil {
		t.Fatal(err)
	}
	side1Hash, _ := fakeHasher{}.HashHeader(side1.Header)
	side2, _ := buildBlock(side1Hash, 2, 412, false)
	if err := chain.ProcessBlock(side2); err != nil {
		t.Fatal(err)
	}
	side2Hash, _ := fakeHasher{}.HashHeader(side2.Header)

	// side3 invalid -> connect failure after one side block has already connected.
	side3Bad, _ := buildBlock(side2Hash, 3, 413, true)
	// Inject failure when rollback tries to disconnect side2.
	fs.failDisconnectOn = side2Hash.String()
	err = chain.ProcessBlock(side3Bad)
	if err == nil {
		t.Fatalf("expected reorg failure due to rollback disconnect failure")
	}
	if got := err.Error(); got == "" || (got != "" && !containsAll(got, []string{"rollback failed", "injected disconnect failure"})) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReorgReportsRestoreFailure(t *testing.T) {
	params := chaincfg.MainNet
	fs := &failStore{base: storage.NewFileStore(t.TempDir())}
	chain, err := blockchain.New(params, fakeHasher{}, fs)
	if err != nil {
		t.Fatal(err)
	}

	var zero chainhash.Hash
	genesisLike, _ := buildBlock(zero, 0, 501, false)
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)

	main1, _ := buildBlock(genesisHash, 1, 502, false)
	if err := chain.ProcessBlock(main1); err != nil {
		t.Fatal(err)
	}
	main1Hash, _ := fakeHasher{}.HashHeader(main1.Header)
	main2, _ := buildBlock(main1Hash, 2, 503, false)
	if err := chain.ProcessBlock(main2); err != nil {
		t.Fatal(err)
	}
	main2Hash, _ := fakeHasher{}.HashHeader(main2.Header)

	side1, _ := buildBlock(genesisHash, 1, 511, false)
	if err := chain.ProcessBlock(side1); err != nil {
		t.Fatal(err)
	}
	side1Hash, _ := fakeHasher{}.HashHeader(side1.Header)
	side2, _ := buildBlock(side1Hash, 2, 512, false)
	if err := chain.ProcessBlock(side2); err != nil {
		t.Fatal(err)
	}
	side2Hash, _ := fakeHasher{}.HashHeader(side2.Header)

	// side3 invalid forces side activation rollback and main restore path.
	side3Bad, _ := buildBlock(side2Hash, 3, 513, true)
	// Inject restore failure while reconnecting old main block.
	fs.failSaveOn = main2Hash.String()
	err = chain.ProcessBlock(side3Bad)
	if err == nil {
		t.Fatalf("expected reorg failure due to restore save failure")
	}
	if got := err.Error(); !containsAll(got, []string{"restore failed", "injected save failure"}) {
		t.Fatalf("unexpected restore failure error: %v", err)
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !strings.Contains(s, p) {
			return false
		}
	}
	return true
}
