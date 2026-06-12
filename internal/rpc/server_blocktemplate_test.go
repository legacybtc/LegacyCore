package rpc

import (
	"encoding/hex"
	"fmt"
	"math"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/genesis"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/mining"
	"legacycoin/legacy-go/internal/storage"
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

func TestNetworkHashEstimateReportsWindowDiagnostics(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := rpcLowHashHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()
	chain, err := blockchain.New(params, rpcLowHashHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if err := connectRPCSyntheticBlock(chain, params); err != nil {
			t.Fatal(err)
		}
	}
	s := &Server{chain: chain}
	estimate := s.estimateNetworkHashPS(100)
	if estimate["genesis_excluded"] != true {
		t.Fatalf("genesis must be excluded: %+v", estimate)
	}
	if estimate["network_hashps_blocks_used"] != int32(4) {
		t.Fatalf("blocks used = %#v want 4 post-startup intervals", estimate["network_hashps_blocks_used"])
	}
	if estimate["network_hashps_window"] != int32(100) {
		t.Fatalf("window = %#v want 100", estimate["network_hashps_window"])
	}
	if estimate["network_hashps_timespan_seconds"] == int64(0) {
		t.Fatalf("timespan should be non-zero: %+v", estimate)
	}
	if estimate["network_hashps_formula"] == "" || estimate["network_hashps_confidence"] == "" || estimate["units"] != "H/s" {
		t.Fatalf("missing hashrate diagnostics: %+v", estimate)
	}
}

type rpcLowHashHasher struct{}

func (rpcLowHashHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	var h chainhash.Hash
	h[0] = byte(header.Timestamp)
	if h[0] == 0 {
		h[0] = 1
	}
	return h, nil
}

func connectRPCSyntheticBlock(chain *blockchain.Chain, params chaincfg.Params) error {
	tip := chain.Tip()
	prev, err := chainhash.FromString(tip.Hash)
	if err != nil {
		return err
	}
	height := tip.Height + 1
	coinbase, err := mining.NewCoinbaseTx(height, []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, chaincfg.BlockSubsidy(height))
	if err != nil {
		return err
	}
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Timestamp: tip.Time + uint32(chaincfg.TargetSpacing.Seconds()),
			Bits:      params.PostGenesisBits,
			Nonce:     uint32(height),
		},
		Transactions: []*wire.MsgTx{coinbase},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return err
	}
	block.Header.MerkleRoot = root
	return chain.ConnectBlock(block)
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
