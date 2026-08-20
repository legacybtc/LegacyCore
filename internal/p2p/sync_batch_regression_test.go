package p2p

import "testing"

func TestMaxDualHashGetDataBlocksMatchesServeLimit(t *testing.T) {
	if got, want := maxDualHashGetDataBlocks(), maxGetDataItems/2; got != want {
		t.Fatalf("max dual-hash blocks = %d, want %d", got, want)
	}
	if got := maxDualHashGetDataBlocks() * 2; got > maxGetDataItems {
		t.Fatalf("dual-hash batch emits %d inventory entries, exceeds max %d", got, maxGetDataItems)
	}
}

func TestDualHashBatchHasNoOversizedTail(t *testing.T) {
	const wantedBlocks = 2000
	perBatch := maxDualHashGetDataBlocks()
	batches := (wantedBlocks + perBatch - 1) / perBatch
	if batches != 16 {
		t.Fatalf("wanted %d blocks should require 16 batches at %d blocks/batch, got %d", wantedBlocks, perBatch, batches)
	}
	last := wantedBlocks - (batches-1)*perBatch
	if last <= 0 || last > perBatch {
		t.Fatalf("invalid final batch size %d", last)
	}
}
