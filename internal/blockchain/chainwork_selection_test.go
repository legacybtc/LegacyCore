package blockchain

import (
	"math/big"
	"testing"

	"legacycoin/legacy-go/internal/consensus"
)

func TestSideChainLowerWorkDoesNotActivateEvenIfHigherHeight(t *testing.T) {
	chain := &Chain{
		tip: &BlockIndex{Height: 10, Hash: "main-tip"},
		workByHash: map[string]*big.Int{
			"main-tip": big.NewInt(1000),
		},
		sideBlocks: map[string]*sideBlockNode{
			"side-tip": {
				hash:      "side-tip",
				parent:    "main-tip",
				height:    999,
				chainwork: big.NewInt(900),
			},
		},
	}
	if err := chain.tryActivateSideChainLocked("side-tip"); err != nil {
		t.Fatalf("tryActivateSideChainLocked returned error: %v", err)
	}
	if chain.tip == nil || chain.tip.Hash != "main-tip" {
		t.Fatalf("active tip changed unexpectedly: %+v", chain.tip)
	}
}

func TestSideChainEqualWorkDoesNotActivate(t *testing.T) {
	chain := &Chain{
		tip: &BlockIndex{Height: 20, Hash: "main-tip"},
		workByHash: map[string]*big.Int{
			"main-tip": big.NewInt(2500),
		},
		sideBlocks: map[string]*sideBlockNode{
			"side-tip": {
				hash:      "side-tip",
				parent:    "main-tip",
				height:    25,
				chainwork: big.NewInt(2500),
			},
		},
	}
	if err := chain.tryActivateSideChainLocked("side-tip"); err != nil {
		t.Fatalf("tryActivateSideChainLocked returned error: %v", err)
	}
	if chain.tip == nil || chain.tip.Hash != "main-tip" {
		t.Fatalf("active tip changed unexpectedly: %+v", chain.tip)
	}
}

func TestChildChainworkAddsWorkForBits(t *testing.T) {
	parent := big.NewInt(1000)
	chain := &Chain{
		workByHash: map[string]*big.Int{
			"parent": new(big.Int).Set(parent),
		},
	}
	got, err := chain.computeChildChainworkLocked("parent", 0x1e7fffff)
	if err != nil {
		t.Fatalf("computeChildChainworkLocked failed: %v", err)
	}
	want := new(big.Int).Add(parent, consensus.WorkForBits(0x1e7fffff))
	if got.Cmp(want) != 0 {
		t.Fatalf("chainwork mismatch: got=%s want=%s", got, want)
	}
}
