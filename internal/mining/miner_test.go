package mining

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/genesis"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
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

type lowHashTestHasher struct{}

func (lowHashTestHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	var h chainhash.Hash
	h[0] = byte(header.Timestamp)
	if h[0] == 0 {
		h[0] = 1
	}
	return h, nil
}

func TestMultiOutputCoinbaseBlockAccepted(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := lowHashTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, lowHashTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	tip := chain.Tip()
	prev, err := chainhash.FromString(tip.Hash)
	if err != nil {
		t.Fatal(err)
	}
	subsidy := chaincfg.BlockSubsidy(1)
	minerValue := subsidy * 96 / 100
	projectValue := subsidy - minerValue
	coinbase, err := NewCoinbaseTxWithOutputs(1, []CoinbaseOutput{
		{PubKeyHash: bytes.Repeat([]byte{0x11}, 20), Value: minerValue},
		{PubKeyHash: bytes.Repeat([]byte{0x22}, 20), Value: projectValue},
	})
	if err != nil {
		t.Fatal(err)
	}
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Timestamp: tip.Time + 1,
			Bits:      params.PostGenesisBits,
		},
		Transactions: []*wire.MsgTx{coinbase},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}
	block.Header.MerkleRoot = root
	if err := chain.ConnectBlock(block); err != nil {
		t.Fatalf("multi-output coinbase block rejected: %v", err)
	}
	if len(coinbase.TxOut) != 2 {
		t.Fatalf("coinbase outputs=%d want 2", len(coinbase.TxOut))
	}
}

func TestTemplateFreshnessRejectsBehindHeightPrevHashAndAge(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := lowHashTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()
	chain, err := blockchain.New(params, lowHashTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	pubHash := bytes.Repeat([]byte{0x44}, 20)
	template, height, err := NewBlockTemplate(chain, mempool.New(), pubHash)
	if err != nil {
		t.Fatal(err)
	}
	fresh := CheckTemplateFreshness(chain, template, height, time.Now(), DefaultHardTemplateStaleAge)
	if !fresh.ActiveTemplateIsFresh {
		t.Fatalf("fresh template rejected: %+v", fresh)
	}
	behind := CheckTemplateFreshness(chain, template, height-1, time.Now(), DefaultHardTemplateStaleAge)
	if behind.ActiveTemplateIsFresh || behind.ActiveTemplateStaleReason != "template height is not current tip height + 1" {
		t.Fatalf("behind template should be stale, got %+v", behind)
	}
	mismatch := *template
	mismatch.Header.PrevBlock = chainhash.Hash{0x99}
	prevMismatch := CheckTemplateFreshness(chain, &mismatch, height, time.Now(), DefaultHardTemplateStaleAge)
	if prevMismatch.ActiveTemplateIsFresh || prevMismatch.ActiveTemplateStaleReason != "template prev hash does not match current tip" {
		t.Fatalf("prev-hash mismatch should be stale, got %+v", prevMismatch)
	}
	softOld := CheckTemplateFreshness(chain, template, height, time.Now().Add(-DefaultSoftTemplateRefreshAge-time.Second), DefaultHardTemplateStaleAge)
	if !softOld.ActiveTemplateIsFresh || !softOld.ActiveTemplateRefreshDue {
		t.Fatalf("soft-old template should stay fresh and request refresh, got %+v", softOld)
	}
	hardOld := CheckTemplateFreshness(chain, template, height, time.Now().Add(-DefaultHardTemplateStaleAge-time.Second), DefaultHardTemplateStaleAge)
	if hardOld.ActiveTemplateIsFresh || hardOld.ActiveTemplateStaleReason != "template age exceeds hard stale limit" {
		t.Fatalf("hard-old template should be stale, got %+v", hardOld)
	}
}

func TestBenchmarkHashrateReturnsPromptlyWithManyWorkers(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := lowHashTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()
	chain, err := blockchain.New(params, lowHashTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	result, err := BenchmarkHashrate(context.Background(), chain, mempool.New(), lowHashTestHasher{}, bytes.Repeat([]byte{0x33}, 20), 12, 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if result.Threads != 12 {
		t.Fatalf("threads=%d want 12", result.Threads)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("benchmark miner status loop returned too slowly: %s", elapsed)
	}
}
