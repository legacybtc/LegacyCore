package blockchain_test

import (
	"strings"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

func TestValidateHeaderSequenceChecksEveryHeaderBits(t *testing.T) {
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
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)

	header1Block, err := buildBlock(genesisHash, 1, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	header1Hash, _ := fakeHasher{}.HashHeader(header1Block.Header)
	header2Block, err := buildBlock(header1Hash, 2, 3, false)
	if err != nil {
		t.Fatal(err)
	}
	header2Block.Header.Bits++

	_, err = chain.ValidateHeaderSequence([]wire.BlockHeader{header1Block.Header, header2Block.Header})
	if err == nil || !strings.Contains(err.Error(), "header 1 has unexpected bits") {
		t.Fatalf("expected second-header bits rejection, got %v", err)
	}
}

func TestValidateHeaderSequenceReturnsCanonicalHashes(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zeroHash chainhash.Hash
	genesisLike, err := buildBlock(zeroHash, 0, 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)

	next, err := buildBlock(genesisHash, 1, 11, false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := fakeHasher{}.HashHeader(next.Header)
	got, err := chain.ValidateHeaderSequence([]wire.BlockHeader{next.Header})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected hashes: got %v want %s", got, want.String())
	}
}

func TestHeadersAfterUnknownLocatorReturnsEmpty(t *testing.T) {
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
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)
	next, err := buildBlock(genesisHash, 1, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(next); err != nil {
		t.Fatal(err)
	}
	var unknown chainhash.Hash
	unknown[0] = 0xaa
	headers, err := chain.HeadersAfter([]chainhash.Hash{unknown}, chainhash.Hash{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 0 {
		t.Fatalf("expected no headers for unknown locator, got %d", len(headers))
	}
}

func TestHeadersAfterGenesisLocatorReturnsPostGenesisHeaders(t *testing.T) {
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
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)
	next, err := buildBlock(genesisHash, 1, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(next); err != nil {
		t.Fatal(err)
	}
	headers, err := chain.HeadersAfter([]chainhash.Hash{genesisHash}, chainhash.Hash{}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 1 {
		t.Fatalf("expected one post-genesis header, got %d", len(headers))
	}
	if headers[0].PrevBlock != genesisHash {
		t.Fatalf("header did not connect to genesis locator")
	}
}

func TestLocatorIncludesActiveAncestors(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zeroHash chainhash.Hash
	genesisLike, err := buildBlock(zeroHash, 0, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)
	prev := genesisHash
	for height := int32(1); height <= 12; height++ {
		block, err := buildBlock(prev, height, uint32(100+height), false)
		if err != nil {
			t.Fatal(err)
		}
		if err := chain.ProcessBlock(block); err != nil {
			t.Fatal(err)
		}
		prev, _ = fakeHasher{}.HashHeader(block.Header)
	}

	locator := chain.Locator()
	if len(locator) < 3 {
		t.Fatalf("locator len=%d want multiple active ancestors", len(locator))
	}
	if locator[0] != prev {
		t.Fatalf("locator tip=%s want %s", locator[0], prev)
	}
	foundGenesis := false
	for _, hash := range locator {
		if hash == genesisHash {
			foundGenesis = true
			break
		}
	}
	if !foundGenesis {
		t.Fatalf("locator did not include genesis ancestor: %v", locator)
	}
}

func TestValidateHeaderSequenceAcceptsActiveAncestor(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zeroHash chainhash.Hash
	genesisLike, err := buildBlock(zeroHash, 0, 200, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, _ := fakeHasher{}.HashHeader(genesisLike.Header)
	main1, err := buildBlock(genesisHash, 1, 201, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(main1); err != nil {
		t.Fatal(err)
	}
	main1Hash, _ := fakeHasher{}.HashHeader(main1.Header)
	main2, err := buildBlock(main1Hash, 2, 202, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(main2); err != nil {
		t.Fatal(err)
	}

	side1, err := buildBlock(genesisHash, 1, 211, false)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := fakeHasher{}.HashHeader(side1.Header)
	got, err := chain.ValidateHeaderSequence([]wire.BlockHeader{side1.Header})
	if err != nil {
		t.Fatalf("ValidateHeaderSequence from active ancestor: %v", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected hashes: got %v want %s", got, want.String())
	}
}

func TestBlockByWireHashAcceptsLegacyHeaderHash(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zeroHash chainhash.Hash
	genesisLike, err := buildBlock(zeroHash, 0, 300, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	canonical, _ := fakeHasher{}.HashHeader(genesisLike.Header)
	legacy, err := chain.LegacyHeaderHash(genesisLike.Header)
	if err != nil {
		t.Fatal(err)
	}
	block, idx, err := chain.BlockByWireHash(legacy.String())
	if err != nil {
		t.Fatalf("BlockByWireHash legacy lookup: %v", err)
	}
	if block == nil || idx == nil || idx.Hash != canonical.String() {
		t.Fatalf("legacy lookup returned block=%v idx=%+v want canonical %s", block != nil, idx, canonical)
	}
}
