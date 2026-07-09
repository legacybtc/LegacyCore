package blockchain_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

type fakeHasher struct{}

func (fakeHasher) HashHeader(h wire.BlockHeader) (chainhash.Hash, error) {
	var out chainhash.Hash
	// Keep the hash low so Pow checks pass under easy bits.
	out[0] = 0x00
	out[1] = byte(h.Nonce)
	out[2] = byte(h.Nonce >> 8)
	out[3] = byte(h.Nonce >> 16)
	out[4] = byte(h.Nonce >> 24)
	out[5] = byte(h.Timestamp)
	out[6] = byte(h.Timestamp >> 8)
	out[7] = byte(h.Timestamp >> 16)
	out[8] = byte(h.Timestamp >> 24)
	return out, nil
}

func TestReorgRollbackOnSideBranchFailure(t *testing.T) {
	params := chaincfg.MainNet
	chain, err := blockchain.New(params, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	var zero chainhash.Hash
	genesisLike, err := buildBlock(zero, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, err := fakeHasher{}.HashHeader(genesisLike.Header)
	if err != nil {
		t.Fatal(err)
	}

	main1, err := buildBlock(genesisHash, 1, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(main1); err != nil {
		t.Fatal(err)
	}
	main1Hash, _ := fakeHasher{}.HashHeader(main1.Header)

	main2, err := buildBlock(main1Hash, 2, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(main2); err != nil {
		t.Fatal(err)
	}
	originalTip := chain.Tip()
	if originalTip == nil || originalTip.Height != 2 {
		t.Fatalf("unexpected original tip: %+v", originalTip)
	}

	// Build a longer side branch off genesis.
	side1, err := buildBlock(genesisHash, 1, 11, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(side1); err != nil {
		t.Fatal(err)
	}
	side1Hash, _ := fakeHasher{}.HashHeader(side1.Header)

	side2, err := buildBlock(side1Hash, 2, 12, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(side2); err != nil {
		t.Fatal(err)
	}
	side2Hash, _ := fakeHasher{}.HashHeader(side2.Header)

	// This block makes side branch longer (height 3) but is invalid by design.
	side3Bad, err := buildBlock(side2Hash, 3, 13, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(side3Bad); err == nil {
		t.Fatalf("expected side-branch activation failure")
	}

	finalTip := chain.Tip()
	if finalTip == nil {
		t.Fatalf("missing final tip")
	}
	if finalTip.Hash != originalTip.Hash || finalTip.Height != originalTip.Height {
		t.Fatalf("tip changed after failed reorg: got=%+v want=%+v", finalTip, originalTip)
	}
}

func TestSuccessfulReorgKeepsOldMainBlocksIndexed(t *testing.T) {
	params := chaincfg.MainNet
	chain, err := blockchain.New(params, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	var zero chainhash.Hash
	genesisLike, _ := buildBlock(zero, 0, 21, false)
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)

	main1, _ := buildBlock(genesisHash, 1, 22, false)
	if err := chain.ProcessBlock(main1); err != nil {
		t.Fatal(err)
	}
	main1Hash, _ := fakeHasher{}.HashHeader(main1.Header)
	main2, _ := buildBlock(main1Hash, 2, 23, false)
	if err := chain.ProcessBlock(main2); err != nil {
		t.Fatal(err)
	}
	main2Hash, _ := fakeHasher{}.HashHeader(main2.Header)

	side1, _ := buildBlock(genesisHash, 1, 31, false)
	if err := chain.ProcessBlock(side1); err != nil {
		t.Fatal(err)
	}
	side1Hash, _ := fakeHasher{}.HashHeader(side1.Header)
	side2, _ := buildBlock(side1Hash, 2, 32, false)
	if err := chain.ProcessBlock(side2); err != nil {
		t.Fatal(err)
	}
	side2Hash, _ := fakeHasher{}.HashHeader(side2.Header)
	side3, _ := buildBlock(side2Hash, 3, 33, false)
	if err := chain.ProcessBlock(side3); err != nil {
		t.Fatal(err)
	}
	side3Hash, _ := fakeHasher{}.HashHeader(side3.Header)

	tip := chain.Tip()
	if tip == nil || tip.Hash != side3Hash.String() || tip.Height != 3 {
		t.Fatalf("unexpected tip after reorg: %+v", tip)
	}
	if !chain.HasBlock(main2Hash.String()) {
		t.Fatalf("old main block missing from hash index")
	}
	if _, _, err := chain.BlockByHash(main2Hash.String()); err != nil {
		t.Fatalf("load old main block: %v", err)
	}

	// Extending old main branch should be treated as side-chain progression,
	// not as active-parent progression.
	oldBranchNext, _ := buildBlock(main2Hash, 3, 34, false)
	if err := chain.ProcessBlock(oldBranchNext); err != nil {
		t.Fatalf("old-branch extension rejected: %v", err)
	}
	tip2 := chain.Tip()
	if tip2 == nil || tip2.Hash != side3Hash.String() || tip2.Height != 3 {
		t.Fatalf("active tip changed unexpectedly after old-branch extension: %+v", tip2)
	}
}

func TestRejectBlockAtOrBeforeMedianTimePast(t *testing.T) {
	params := chaincfg.MainNet
	chain, err := blockchain.New(params, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}

	var zero chainhash.Hash
	b0, _ := buildBlock(zero, 0, 101, false)
	if err := chain.ProcessBlock(b0); err != nil {
		t.Fatal(err)
	}
	h0, _ := fakeHasher{}.HashHeader(b0.Header)

	// Build a short chain with increasing timestamps.
	b1, _ := buildBlock(h0, 1, 102, false)
	if err := chain.ProcessBlock(b1); err != nil {
		t.Fatal(err)
	}
	h1, _ := fakeHasher{}.HashHeader(b1.Header)

	b2, _ := buildBlock(h1, 2, 103, false)
	if err := chain.ProcessBlock(b2); err != nil {
		t.Fatal(err)
	}
	h2, _ := fakeHasher{}.HashHeader(b2.Header)

	// Use timestamp equal to current MTP candidate (not strictly greater).
	b3, _ := buildBlock(h2, 3, 104, false)
	b3.Header.Timestamp = b1.Header.Timestamp
	if err := chain.ConnectBlock(b3); err == nil {
		t.Fatalf("expected MTP rejection")
	}
}

func TestRejectBlockTooFarInFuture(t *testing.T) {
	params := chaincfg.MainNet
	chain, err := blockchain.New(params, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zero chainhash.Hash
	b0, _ := buildBlock(zero, 0, 201, false)
	if err := chain.ProcessBlock(b0); err != nil {
		t.Fatal(err)
	}
	h0, _ := fakeHasher{}.HashHeader(b0.Header)

	b1, _ := buildBlock(h0, 1, 202, false)
	b1.Header.Timestamp = math.MaxUint32
	if err := chain.ConnectBlock(b1); err == nil {
		t.Fatalf("expected future-time rejection")
	}
}

func TestOrphanPoolCapped(t *testing.T) {
	params := chaincfg.MainNet
	chain, err := blockchain.New(params, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zero chainhash.Hash
	genesisLike, err := buildBlock(zero, 0, 251, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < blockchain.MaxOrphanBlocks+10; i++ {
		var missingParent chainhash.Hash
		missingParent[0] = 0xaa
		missingParent[1] = byte(i)
		b, err := buildBlock(missingParent, int32(i+1), uint32(300+i), false)
		if err != nil {
			t.Fatal(err)
		}
		if err := chain.ProcessBlock(b); err != nil {
			t.Fatalf("orphan process failed at %d: %v", i, err)
		}
	}
	if got := chain.OrphanCount(); got != blockchain.MaxOrphanBlocks {
		t.Fatalf("orphan count=%d want=%d", got, blockchain.MaxOrphanBlocks)
	}
}

func buildBlock(prev chainhash.Hash, height int32, nonce uint32, corruptMerkle bool) (*wire.MsgBlock, error) {
	coinbase, err := coinbaseTx(height)
	if err != nil {
		return nil, err
	}
	b := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Timestamp: uint32(time.Now().UTC().Unix()-100_000) + nonce,
			Bits:      testBitsForHeight(height),
			Nonce:     nonce,
		},
		Transactions: []*wire.MsgTx{coinbase},
	}
	root, err := b.BuildMerkleRoot()
	if err != nil {
		return nil, err
	}
	b.Header.MerkleRoot = root
	if corruptMerkle {
		b.Header.MerkleRoot[0] ^= 0xff
	}
	return b, nil
}

func testBitsForHeight(height int32) uint32 {
	if height <= 0 {
		return chaincfg.MainNet.GenesisBits
	}
	return chaincfg.MainNet.PostGenesisBits
}

func coinbaseTx(height int32) (*wire.MsgTx, error) {
	pubHash := make([]byte, 20)
	pubHash[0] = 0x42
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		return nil, err
	}
	hb := []byte{byte(height), byte(height >> 8), byte(height >> 16), byte(height >> 24)}
	sigScript := append([]byte{byte(len(hb))}, hb...)
	sigScript = append(sigScript, []byte("/Legacy-GO-ReorgTest/")...)
	return &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32},
			SignatureScript:  sigScript,
			Sequence:         math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{
			Value:    chaincfg.BlockSubsidy(height),
			PkScript: pkScript,
		}},
	}, nil
}

func TestFakeHasherDeterministic(t *testing.T) {
	h := fakeHasher{}
	var prev chainhash.Hash
	b, err := buildBlock(prev, 0, 77, false)
	if err != nil {
		t.Fatal(err)
	}
	a, err := h.HashHeader(b.Header)
	if err != nil {
		t.Fatal(err)
	}
	c, err := h.HashHeader(b.Header)
	if err != nil {
		t.Fatal(err)
	}
	if a != c {
		t.Fatal(fmt.Errorf("non-deterministic hash"))
	}
}
