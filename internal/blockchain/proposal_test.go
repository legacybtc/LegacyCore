package blockchain_test

import (
	"errors"
	"testing"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

type proposalHighHashHasher struct{}

func (proposalHighHashHasher) HashHeader(h wire.BlockHeader) (chainhash.Hash, error) {
	if h.PrevBlock.IsZero() {
		return fakeHasher{}.HashHeader(h)
	}
	var out chainhash.Hash
	for i := range out {
		out[i] = 0xff
	}
	return out, nil
}

func TestValidateBlockProposalDryRunLeavesStateUnchanged(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zero chainhash.Hash
	genesisLike, err := buildBlock(zero, 0, 501, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, err := chain.BlockHash(genesisLike)
	if err != nil {
		t.Fatal(err)
	}
	genesisTip := chain.Tip()

	next, err := buildBlock(genesisHash, 1, 502, false)
	if err != nil {
		t.Fatal(err)
	}
	nextHash, err := chain.BlockHash(next)
	if err != nil {
		t.Fatal(err)
	}
	result, err := chain.ValidateBlockProposal(next)
	if err != nil {
		t.Fatalf("valid proposal rejected: %v", err)
	}
	if result.Status != blockchain.BlockStatusProposal || !result.ExtendsActiveTip {
		t.Fatalf("unexpected active proposal result: %+v", result)
	}
	if chain.HasBlock(nextHash.String()) {
		t.Fatalf("dry-run active proposal was stored")
	}
	if tip := chain.Tip(); tip.Hash != genesisTip.Hash || tip.Height != genesisTip.Height {
		t.Fatalf("tip changed after dry-run active proposal: got=%+v want=%+v", tip, genesisTip)
	}
	if got := chain.OrphanCount(); got != 0 {
		t.Fatalf("orphan count changed after active dry-run: %d", got)
	}

	if err := chain.ProcessBlock(next); err != nil {
		t.Fatal(err)
	}
	activeTip := chain.Tip()
	side, err := buildBlock(genesisHash, 1, 503, false)
	if err != nil {
		t.Fatal(err)
	}
	sideHash, err := chain.BlockHash(side)
	if err != nil {
		t.Fatal(err)
	}
	result, err = chain.ValidateBlockProposal(side)
	if err != nil {
		t.Fatalf("side proposal dry-run failed: %v", err)
	}
	if result.Status != blockchain.BlockStatusSideChain || !result.SideChain {
		t.Fatalf("unexpected side proposal result: %+v", result)
	}
	if chain.HasBlock(sideHash.String()) {
		t.Fatalf("dry-run side-chain proposal was stored")
	}
	if tip := chain.Tip(); tip.Hash != activeTip.Hash || tip.Height != activeTip.Height {
		t.Fatalf("tip changed after dry-run side proposal: got=%+v want=%+v", tip, activeTip)
	}

	var missingParent chainhash.Hash
	missingParent[0] = 0xaa
	orphan, err := buildBlock(missingParent, 2, 504, false)
	if err != nil {
		t.Fatal(err)
	}
	orphanHash, err := chain.BlockHash(orphan)
	if err != nil {
		t.Fatal(err)
	}
	result, err = chain.ValidateBlockProposal(orphan)
	if err != nil {
		t.Fatalf("orphan proposal dry-run failed: %v", err)
	}
	if result.Status != blockchain.BlockStatusOrphan || !result.Orphan {
		t.Fatalf("unexpected orphan proposal result: %+v", result)
	}
	if chain.HasBlock(orphanHash.String()) {
		t.Fatalf("dry-run orphan proposal was stored")
	}
	if got := chain.OrphanCount(); got != 0 {
		t.Fatalf("dry-run orphan was cached: %d", got)
	}
	if tip := chain.Tip(); tip.Hash != activeTip.Hash || tip.Height != activeTip.Height {
		t.Fatalf("tip changed after dry-run orphan proposal: got=%+v want=%+v", tip, activeTip)
	}
}

func TestValidateBlockProposalRejectsInvalidBlocks(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, fakeHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zero chainhash.Hash
	genesisLike, err := buildBlock(zero, 0, 601, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, err := chain.BlockHash(genesisLike)
	if err != nil {
		t.Fatal(err)
	}
	overpay, err := buildBlock(genesisHash, 1, 602, false)
	if err != nil {
		t.Fatal(err)
	}
	overpay.Transactions[0].TxOut[0].Value = chaincfg.BlockSubsidy(1) + 1
	refreshMerkleRoot(t, overpay)
	overpayHash, err := chain.BlockHash(overpay)
	if err != nil {
		t.Fatal(err)
	}
	result, err := chain.ValidateBlockProposal(overpay)
	if !errors.Is(err, blockchain.ErrBadCoinbaseValue) {
		t.Fatalf("expected bad coinbase value, got result=%+v err=%v", result, err)
	}
	if result.Status != blockchain.BlockStatusRejected {
		t.Fatalf("invalid proposal status=%q want %q", result.Status, blockchain.BlockStatusRejected)
	}
	if chain.HasBlock(overpayHash.String()) {
		t.Fatalf("invalid overpay proposal was stored")
	}

	highChain, err := blockchain.New(chaincfg.MainNet, proposalHighHashHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	highGenesis, err := buildBlock(zero, 0, 701, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := highChain.ProcessBlock(highGenesis); err != nil {
		t.Fatal(err)
	}
	highGenesisHash, err := highChain.BlockHash(highGenesis)
	if err != nil {
		t.Fatal(err)
	}
	highBlock, err := buildBlock(highGenesisHash, 1, 702, false)
	if err != nil {
		t.Fatal(err)
	}
	highHash, err := highChain.BlockHash(highBlock)
	if err != nil {
		t.Fatal(err)
	}
	result, err = highChain.ValidateBlockProposal(highBlock)
	if !errors.Is(err, consensus.ErrHighHash) {
		t.Fatalf("expected high hash rejection, got result=%+v err=%v", result, err)
	}
	if result.Status != blockchain.BlockStatusRejected {
		t.Fatalf("high-hash proposal status=%q want %q", result.Status, blockchain.BlockStatusRejected)
	}
	if highChain.HasBlock(highHash.String()) {
		t.Fatalf("high-hash proposal was stored")
	}
}

func refreshMerkleRoot(t *testing.T, block *wire.MsgBlock) {
	t.Helper()
	root, err := block.BuildMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}
	block.Header.MerkleRoot = root
}
