package p2p

import "testing"

// The P2P protocol currently uses two block hashes per requested block
// (canonical Yespower plus the legacy SHA256d compatibility hash). Keep the
// regression invariant tied directly to the wire limits used by the server.
func TestDualHashGetDataCapacityMatchesServeLimit(t *testing.T) {
	const perBatch = maxGetDataItems / 2
	if perBatch <= 0 {
		t.Fatal("dual-hash batch capacity must be positive")
	}
	if got := perBatch * 2; got > maxGetDataItems {
		t.Fatalf("dual-hash batch emits %d inventory entries, exceeds max %d", got, maxGetDataItems)
	}
	if got, want := maxGetDataItems, maxServeInvItems; got != want {
		t.Fatalf("request and serve inventory limits diverged: request=%d serve=%d", got, want)
	}
}

func TestDualHashBatchHasNoOversizedTail(t *testing.T) {
	const wantedBlocks = 2000
	const perBatch = maxGetDataItems / 2
	batches := (wantedBlocks + perBatch - 1) / perBatch
	if batches != 16 {
		t.Fatalf("wanted %d blocks should require 16 batches at %d blocks/batch, got %d", wantedBlocks, perBatch, batches)
	}
	last := wantedBlocks - (batches-1)*perBatch
	if last <= 0 || last > perBatch {
		t.Fatalf("invalid final batch size %d", last)
	}
}
