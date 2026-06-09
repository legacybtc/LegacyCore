package rpc

import "testing"

func safeMiningInput() MiningSafetyInput {
	return MiningSafetyInput{
		RPCHealth:           "ok",
		StorageOK:           true,
		DestinationOK:       true,
		SafeRequired:        true,
		MinGoodPeers:        3,
		BlocksBehindAllowed: 1,
		LocalHeight:         100,
		BestPeerHeight:      101,
		BlocksBehind:        1,
		PeerCount:           5,
		GoodPeerCount:       5,
		SyncState:           "current",
	}
}

func TestCheckSafeToMineAllowsSyncedWithGoodPeers(t *testing.T) {
	status := CheckSafeToMine(safeMiningInput())
	if !status.Safe {
		t.Fatalf("safe synced node was blocked: %+v", status)
	}
	if status.Reason != "" {
		t.Fatalf("safe status should not have reason: %q", status.Reason)
	}
}

func TestCheckSafeToMineAllowsCurrentWithRecentSyncRequest(t *testing.T) {
	input := safeMiningInput()
	input.LocalHeight = 100
	input.BestPeerHeight = 100
	input.BlocksBehind = 0
	input.GoodPeerCount = 4
	input.RequestInFlight = true
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("current healthy node with old/recent sync request should be safe: %+v", status)
	}
	if status.Reason != "" {
		t.Fatalf("safe status should not have blocked reason: %q", status.Reason)
	}
}

func TestCheckSafeToMineBlocksNoPeers(t *testing.T) {
	input := safeMiningInput()
	input.PeerCount = 0
	input.GoodPeerCount = 0
	input.SyncState = "no_peers"
	status := CheckSafeToMine(input)
	if status.Safe || status.Reason != "Mining blocked: node has no peers." {
		t.Fatalf("expected no-peers block, got %+v", status)
	}
}

func TestCheckSafeToMineBlocksBehindPeersByTwo(t *testing.T) {
	input := safeMiningInput()
	input.LocalHeight = 100
	input.BestPeerHeight = 102
	input.BlocksBehind = 2
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("behind by two must block mining: %+v", status)
	}
	if status.BlocksBehind != 2 {
		t.Fatalf("blocks behind = %d, want 2", status.BlocksBehind)
	}
}

func TestCheckSafeToMineBlocksUnsafeSyncStates(t *testing.T) {
	for _, state := range []string{"catching_up", "requesting_blocks", "possibly_stalled", "stalled"} {
		t.Run(state, func(t *testing.T) {
			input := safeMiningInput()
			input.SyncState = state
			status := CheckSafeToMine(input)
			if status.Safe {
				t.Fatalf("sync state %q must block mining: %+v", state, status)
			}
		})
	}
}

func TestCheckSafeToMineBlocksRPCTimeoutAndOffline(t *testing.T) {
	for _, health := range []string{"timeout", "offline"} {
		t.Run(health, func(t *testing.T) {
			input := safeMiningInput()
			input.RPCHealth = health
			status := CheckSafeToMine(input)
			if status.Safe {
				t.Fatalf("rpc health %q must block mining: %+v", health, status)
			}
			if status.RPCHealth != health {
				t.Fatalf("rpc health = %q, want %q", status.RPCHealth, health)
			}
		})
	}
}

func TestCheckSafeToMineBlocksActiveSyncRequest(t *testing.T) {
	input := safeMiningInput()
	input.SyncState = "requesting_blocks"
	input.LocalHeight = 100
	input.BestPeerHeight = 102
	input.BlocksBehind = 2
	input.RequestInFlight = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("active sync request while behind must block mining: %+v", status)
	}
}

func TestCheckSafeToMineBlocksRecentReorg(t *testing.T) {
	input := safeMiningInput()
	input.RecentReorg = true
	status := CheckSafeToMine(input)
	if status.Safe || status.Reason != "Mining blocked: recent reorg detected; waiting for chain stability." {
		t.Fatalf("expected recent-reorg block, got %+v", status)
	}
}

func TestCheckSafeToMineBlocksHighStaleRate(t *testing.T) {
	input := safeMiningInput()
	input.AcceptedBlocks = 2
	input.StaleBlocks = 3
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("high stale rate must block mining: %+v", status)
	}
	if status.StaleRate < 0.5 || status.StaleRateWarning == "" {
		t.Fatalf("expected stale-rate diagnostics, got %+v", status)
	}
}

func TestCheckSafeToMineExpertOverrideMustBeExplicit(t *testing.T) {
	input := safeMiningInput()
	input.PeerCount = 0
	input.GoodPeerCount = 0
	input.SyncState = "no_peers"
	input.SafeRequired = false
	input.AllowUnsafe = true
	status := CheckSafeToMine(input)
	if !status.Safe || !status.UnsafeOverride {
		t.Fatalf("explicit unsafe override should allow with warning, got %+v", status)
	}
}
