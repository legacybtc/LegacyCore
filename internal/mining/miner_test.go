package mining

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
)

func TestNewBlockTemplate(t *testing.T) {
	if pow.BackendName() != "cgo-c-reference" {
		t.Skipf("skipping RC2 genesis/template integration test with yespower backend %q", pow.BackendName())
	}
	dir := t.TempDir()
	chain, err := blockchain.New(chaincfg.MainNet, pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}, storage.NewFileStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pubHash := script.Hash160(priv.PubKey().SerializeCompressed())
	block, height, err := NewBlockTemplate(chain, mempool.New(), pubHash)
	if err != nil {
		t.Fatal(err)
	}
	if height != 1 {
		t.Fatalf("height=%d", height)
	}
	if len(block.Transactions) != 1 {
		t.Fatalf("tx count=%d", len(block.Transactions))
	}
	if block.Header.Bits != chaincfg.MainNet.PostGenesisBits {
		t.Fatalf("bits=%08x", block.Header.Bits)
	}
}
