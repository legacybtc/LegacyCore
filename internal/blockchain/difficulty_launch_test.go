package blockchain_test

import (
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/storage"
)

func TestPostGenesisBitsAreHardenedBeforeDGWWindow(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zeroHash chainhash.Hash
	genesisLike, err := buildBlock(zeroHash, 0, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	bits, err := chain.NextRequiredBits()
	if err != nil {
		t.Fatal(err)
	}
	if bits != chaincfg.MainNet.PostGenesisBits {
		t.Fatalf("post-genesis bits=%08x want %08x", bits, chaincfg.MainNet.PostGenesisBits)
	}
	if bits == chaincfg.MainNet.GenesisBits {
		t.Fatalf("post-genesis mining remained at ultra-easy genesis bits %08x", bits)
	}
}

func TestFastEarlyBlocksDoNotUseGenesisBits(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var prev chainhash.Hash
	for height := int32(0); height < 16; height++ {
		b, err := buildBlock(prev, height, uint32(height+1), false)
		if err != nil {
			t.Fatal(err)
		}
		if err := chain.ProcessBlock(b); err != nil {
			t.Fatalf("process height %d: %v", height, err)
		}
		prev, err = fakeHasher{}.HashHeader(b.Header)
		if err != nil {
			t.Fatal(err)
		}
	}
	bits, err := chain.NextRequiredBits()
	if err != nil {
		t.Fatal(err)
	}
	if bits == chaincfg.MainNet.GenesisBits {
		t.Fatalf("height 16 next bits still ultra-easy genesis bits %08x", bits)
	}
}
