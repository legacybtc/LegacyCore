package rpc

import (
	"testing"
	"time"

	"legacycoin/legacy-go/internal/p2p"
)

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

func TestCheckSafeToMineAllowsTwoCurrentPeersWithLaggingPeers(t *testing.T) {
	input := safeMiningInput()
	input.LocalHeight = 3310
	input.BestPeerHeight = 3310
	input.BlocksBehind = 0
	input.PeerCount = 4
	input.MinAgreeingPeers = 2
	input.GoodPeerCount = 4
	input.CompatiblePeerCount = 4
	input.AgreeingPeerCount = 2
	input.Lagging1PeerCount = 1
	input.Lagging2PeerCount = 1
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("two current agreeing peers plus 1-2 block lagging peers should be safe: %+v", status)
	}
	if status.AgreeingPeerCount != 2 || status.Lagging1PeerCount != 1 || status.Lagging2PeerCount != 1 {
		t.Fatalf("peer category counts not preserved: %+v", status)
	}
}

func TestCheckSafeToMineGraceAllowsTemporaryPeerDegradation(t *testing.T) {
	input := safeMiningInput()
	input.MinAgreeingPeers = 2
	input.PeerCount = 4
	input.GoodPeerCount = 2
	input.CompatiblePeerCount = 2
	input.AgreeingPeerCount = 1
	input.PeerAgreementGraceActive = true
	input.PeerAgreementGraceRemaining = 42
	status := CheckSafeToMine(input)
	if !status.Safe || status.State != "degraded" {
		t.Fatalf("temporary peer degradation should warn but remain safe during grace: %+v", status)
	}
	if status.Reason == "" || !status.PeerAgreementGraceActive {
		t.Fatalf("expected degraded peer grace diagnostics: %+v", status)
	}
}

func TestCheckSafeToMinePersistentLossOfAgreeingPeersPauses(t *testing.T) {
	input := safeMiningInput()
	input.MinAgreeingPeers = 2
	input.PeerCount = 4
	input.GoodPeerCount = 2
	input.CompatiblePeerCount = 2
	input.AgreeingPeerCount = 1
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("persistent loss of agreeing peers should pause mining: %+v", status)
	}
	if status.Reason != "Mining paused: fewer than 2 current agreeing peer(s)." {
		t.Fatalf("unexpected reason: %q", status.Reason)
	}
}

func TestCheckSafeToMineStrongerChainPausesImmediately(t *testing.T) {
	input := safeMiningInput()
	input.MinAgreeingPeers = 2
	input.PeerCount = 4
	input.AgreeingPeerCount = 2
	input.StrongerChainworkPeerCount = 1
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("stronger chain candidate must pause immediately: %+v", status)
	}
	if status.Reason != "Mining paused: peer reports a stronger chain candidate." {
		t.Fatalf("unexpected reason: %q", status.Reason)
	}
}

func TestMiningPeerAssessmentCategorizesLaggingPeers(t *testing.T) {
	peers := []p2p.PeerInfo{
		{ReportedHeight: 3310, PeerSafetyCategory: "current_agreeing", GoodPeer: true},
		{ReportedHeight: 3310, PeerSafetyCategory: "current_agreeing", GoodPeer: true},
		{ReportedHeight: 3309, PeerSafetyCategory: "lagging_1_block", GoodPeer: true},
		{ReportedHeight: 3308, PeerSafetyCategory: "lagging_2_blocks", GoodPeer: true},
	}
	assessment := assessMiningPeers(peers, 3310)
	if assessment.CurrentAgreeing != 2 || assessment.Lagging1 != 1 || assessment.Lagging2 != 1 || assessment.Compatible != 4 {
		t.Fatalf("assessment = %+v", assessment)
	}
}

func TestPeerAgreementHysteresisPausesAndRecovers(t *testing.T) {
	s := &Server{}
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		PeerGraceSeconds:    60,
		PeerRecoverySeconds: 20,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   1,
	}
	s.minerPeerAgreementLostSince = time.Now().Add(-61 * time.Second)
	s.applyMiningPeerAgreementWindow(&input)
	if !input.PeerSafetyPauseActive {
		t.Fatalf("expected persistent low agreement to activate pause: %+v", input)
	}

	recovering := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		PeerGraceSeconds:    60,
		PeerRecoverySeconds: 20,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   2,
	}
	s.applyMiningPeerAgreementWindow(&recovering)
	if !recovering.PeerAgreementRecoveryActive || !recovering.PeerSafetyPauseActive {
		t.Fatalf("expected recovery hysteresis to keep pause briefly: %+v", recovering)
	}

	recovered := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		PeerGraceSeconds:    60,
		PeerRecoverySeconds: 20,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   2,
	}
	s.minerPeerAgreementRecoveredSince = time.Now().Add(-21 * time.Second)
	s.applyMiningPeerAgreementWindow(&recovered)
	if recovered.PeerSafetyPauseActive || recovered.PeerAgreementRecoveryActive || s.minerPeerAgreementPaused {
		t.Fatalf("expected hysteresis recovery to clear pause: input=%+v paused=%t", recovered, s.minerPeerAgreementPaused)
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
	input.AcceptedBlocks = 1
	input.StaleBlocks = 2
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("high stale rate must block mining: %+v", status)
	}
	if status.StaleRate < 0.5 || status.StaleRateWarning == "" {
		t.Fatalf("expected stale-rate diagnostics, got %+v", status)
	}
}

func TestCheckSafeToMineWarnsAtLowerStaleRates(t *testing.T) {
	input := safeMiningInput()
	input.AcceptedBlocks = 9
	input.StaleBlocks = 1
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("10%% stale rate should warn before hard pause: %+v", status)
	}
	if status.StaleRateWarning == "" {
		t.Fatalf("expected non-empty stale warning: %+v", status)
	}
}

func TestCheckSafeToMineAllowsSoftTemplateRefresh(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.ActiveTemplateRefreshDue = true
	input.ActiveTemplateRefreshReason = "refreshing template in background; current template still valid"
	input.CurrentTipHeight = 101
	input.CurrentTemplateHeight = 102
	input.CurrentTipHash = "tip"
	input.ActiveTemplatePrevHash = "tip"
	input.TemplateAgeSeconds = 2 * 60
	input.TemplateSoftRefreshAgeSeconds = 30
	input.TemplateMaxAgeSeconds = 20 * 60
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("soft-refresh template must not block mining: %+v", status)
	}
	if !status.ActiveTemplateRefreshDue || status.ActiveTemplateRefreshReason == "" {
		t.Fatalf("expected refresh diagnostics to be preserved: %+v", status)
	}
	if status.Reason != "" {
		t.Fatalf("soft refresh should not create blocked reason: %q", status.Reason)
	}
}

func TestCheckSafeToMineClearsUnavailableReasonForFreshTemplate(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.ActiveTemplateRefreshDue = false
	input.ActiveTemplateStaleReason = "template unavailable"
	input.ActiveTemplateRefreshReason = "template_stale: template unavailable"
	input.CurrentTipHeight = 101
	input.CurrentTemplateHeight = 102
	input.CurrentTipHash = "tip"
	input.ActiveTemplatePrevHash = "tip"
	input.TemplateAgeSeconds = 5
	input.TemplateSoftRefreshAgeSeconds = 30
	input.TemplateMaxAgeSeconds = 20 * 60

	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("fresh active template should be safe: %+v", status)
	}
	if status.ActiveTemplateRefreshDue || status.ActiveTemplateStaleReason != "" || status.ActiveTemplateRefreshReason != "" {
		t.Fatalf("fresh active template must clear stale/unavailable state, got %+v", status)
	}

	input.ActiveTemplateRefreshDue = true
	input.ActiveTemplateRefreshReason = "template_stale: template unavailable"
	status = CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("fresh soft-refresh template should be safe: %+v", status)
	}
	if status.ActiveTemplateStaleReason != "" || status.ActiveTemplateRefreshReason != "refreshing template in background; current template still valid" {
		t.Fatalf("fresh soft-refresh template must normalize stale/unavailable wording, got %+v", status)
	}
}

func TestCheckSafeToMineBlocksHardTemplateAge(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.CurrentTipHeight = 101
	input.CurrentTemplateHeight = 102
	input.CurrentTipHash = "tip"
	input.ActiveTemplatePrevHash = "tip"
	input.TemplateAgeSeconds = 21 * 60
	input.TemplateSoftRefreshAgeSeconds = 30
	input.TemplateMaxAgeSeconds = 20 * 60
	status := CheckSafeToMine(input)
	if status.Safe || status.Reason != "Mining paused: template refresh failed; template age exceeds hard stale limit." {
		t.Fatalf("hard-stale template age must block mining, got %+v", status)
	}
}

func TestCheckSafeToMineBlocksStaleActiveTemplate(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = false
	input.ActiveTemplateStaleReason = "template height is not current tip height + 1"
	input.CurrentTipHeight = 3177
	input.CurrentTemplateHeight = 3136
	input.TemplateAgeSeconds = 4.9 * 60 * 60
	input.TemplateMaxAgeSeconds = 120
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("stale active template must block mining: %+v", status)
	}
	if status.CurrentTemplateHeight != 3136 || status.CurrentTipHeight != 3177 {
		t.Fatalf("template diagnostics not preserved: %+v", status)
	}
	if status.Reason == "" || status.StaleRateWarning != "" {
		t.Fatalf("expected template block reason without stale-rate warning: %+v", status)
	}
	if !status.ActiveTemplateRefreshDue || status.ActiveTemplateRefreshReason != "height_mismatch: template height is not current tip height + 1" {
		t.Fatalf("stale active template must force refresh diagnostics: %+v", status)
	}
}

func TestCheckSafeToMineBlocksPrevHashMismatch(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.CurrentTemplateHeight = 102
	input.CurrentTipHeight = 101
	input.ActiveTemplatePrevHash = "old"
	input.CurrentTipHash = "new"
	status := CheckSafeToMine(input)
	if status.Safe || status.Reason != "Mining paused: template prev hash does not match current tip." {
		t.Fatalf("expected prev hash mismatch block, got %+v", status)
	}
	if !status.ActiveTemplateRefreshDue || status.ActiveTemplateRefreshReason != "prev_hash_mismatch: template prev hash does not match current tip" {
		t.Fatalf("prev-hash mismatch must force refresh due, got %+v", status)
	}
}

func TestCheckSafeToMineStaleTemplateNeverReportsRefreshDueNo(t *testing.T) {
	input := safeMiningInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = false
	input.ActiveTemplateStaleReason = "template prev hash does not match current tip"
	input.CurrentTemplateHeight = 3205
	input.CurrentTipHeight = 3206
	input.ActiveTemplatePrevHash = "old-tip"
	input.CurrentTipHash = "new-tip"
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("stale template must block hashing: %+v", status)
	}
	if !status.ActiveTemplateRefreshDue {
		t.Fatalf("stale template must say refresh due yes: %+v", status)
	}
	if status.ActiveTemplateRefreshReason != "prev_hash_mismatch: template prev hash does not match current tip" {
		t.Fatalf("refresh reason = %q", status.ActiveTemplateRefreshReason)
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
