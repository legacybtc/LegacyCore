package rpc

import (
	"math"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/p2p"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

type safetyTestHasher struct{}

func (safetyTestHasher) HashHeader(h wire.BlockHeader) (chainhash.Hash, error) {
	var out chainhash.Hash
	out[0] = 0xaa
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

func safetyTestGenesisBlock() (*wire.MsgBlock, error) {
	height := int32(0)
	pubHash := make([]byte, 20)
	pubHash[0] = 0x42
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		return nil, err
	}
	heightBytes := []byte{byte(height), byte(height >> 8), byte(height >> 16), byte(height >> 24)}
	sigScript := append([]byte{byte(len(heightBytes))}, heightBytes...)
	sigScript = append(sigScript, []byte("/Legacy-GO-Test/")...)
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: chainhash.Hash{},
			Timestamp: uint32(time.Now().UTC().Unix()),
			Bits:      chaincfg.MainNet.GenesisBits,
			Nonce:     0x42,
		},
		Transactions: []*wire.MsgTx{{
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
		}},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return nil, err
	}
	block.Header.MerkleRoot = root
	return block, nil
}

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

func graceInput() MiningSafetyInput {
	return MiningSafetyInput{
		RPCHealth:                       "ok",
		StorageOK:                       true,
		DestinationOK:                   true,
		SafeRequired:                    true,
		MinGoodPeers:                    3,
		MinAgreeingPeers:                2,
		BlocksBehindAllowed:             1,
		LocalHeight:                     100,
		BestPeerHeight:                  101,
		BlocksBehind:                    1,
		PeerCount:                       4,
		GoodPeerCount:                   4,
		CompatiblePeerCount:             4,
		AgreeingPeerCount:               0,
		SyncState:                       "current",
		LocalBlockPropagationGraceActive: true,
		LocalBlockPropagationGraceRemaining: 90,
		LocalBlockPropagationHeight:         100,
		LocalBlockPropagationHash:           "abc123",
		LastLocalBlockAnnouncementTime:     1000,
		LocalBlockAnnouncementTargetCount:   5,
	}
}

func TestCheckSafeToMineLocalGraceAllowsZeroAgreeingWithEnoughTime(t *testing.T) {
	input := graceInput()
	status := CheckSafeToMine(input)
	if !status.Safe || status.State != "degraded" {
		t.Fatalf("local block propagation grace should allow zero agreeing peers: %+v", status)
	}
	if !status.LocalBlockPropagationGraceActive {
		t.Fatalf("local block propagation grace flag must be preserved in output: %+v", status)
	}
	if status.Reason == "" {
		t.Fatalf("degraded state must have a reason: %q", status.Reason)
	}
}

func TestCheckSafeToMineLocalGraceBlockedByConflictingTip(t *testing.T) {
	input := graceInput()
	input.ConflictingTipPeerCount = 1
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("conflicting tip must block even during grace: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("conflicting tip must clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceBlockedByStrongerChainwork(t *testing.T) {
	input := graceInput()
	input.StrongerChainworkPeerCount = 1
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("stronger chainwork must block even during grace: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("stronger chainwork must clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceBlockedByWrongChain(t *testing.T) {
	input := graceInput()
	input.WrongChainPeerCount = 1
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("wrong-chain peer must block even during grace: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("wrong-chain peer must clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceBlockedByProtocolError(t *testing.T) {
	input := graceInput()
	input.ProtocolErrorPeerCount = 1
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("protocol error peer must block even during grace: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("protocol error must clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceBlockedByLocalNodeBehind(t *testing.T) {
	input := graceInput()
	input.BlocksBehind = 2
	input.BestPeerHeight = 102
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("local node behind by 2 must block even during grace: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("local node behind must clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceAllowsDuringCompatiblePeerDrop(t *testing.T) {
	input := graceInput()
	input.CompatiblePeerCount = 1
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("local block grace should allow even with few compatible peers (server-level validates this): %+v", status)
	}
	if !status.LocalBlockPropagationGraceActive {
		t.Fatalf("grace must remain active at CheckSafeToMine level: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceClearsWhenAgreeingPeersReachTarget(t *testing.T) {
	input := graceInput()
	input.AgreeingPeerCount = 2
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("2 agreeing peers should be safe: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("sufficient agreeing peers should clear grace: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceCannotBeActivatedByDefaultInput(t *testing.T) {
	input := safeMiningInput()
	status := CheckSafeToMine(input)
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("default input must not have grace active: %+v", status)
	}
}

func TestCheckSafeToMineLocalGracePassesThroughRemaining(t *testing.T) {
	input := graceInput()
	input.LocalBlockPropagationGraceRemaining = 0
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("CheckSafeToMine keeps grace active as long as flag is set; server clears expired grace before calling: %+v", status)
	}
	if !status.LocalBlockPropagationGraceActive {
		t.Fatalf("grace must remain active at CheckSafeToMine level: %+v", status)
	}
	if status.LocalBlockPropagationGraceRemaining != 0 {
		t.Fatalf("remaining = %d, want 0", status.LocalBlockPropagationGraceRemaining)
	}
}

func TestCheckSafeToMineLocalGraceFieldsPreservedInOutput(t *testing.T) {
	input := graceInput()
	status := CheckSafeToMine(input)
	if status.LocalBlockPropagationHeight != 100 {
		t.Fatalf("propagation height = %d, want 100", status.LocalBlockPropagationHeight)
	}
	if status.LocalBlockPropagationHash != "abc123" {
		t.Fatalf("propagation hash = %q, want abc123", status.LocalBlockPropagationHash)
	}
	if status.LastLocalBlockAnnouncementTime != 1000 {
		t.Fatalf("announcement time = %d, want 1000", status.LastLocalBlockAnnouncementTime)
	}
	if status.LocalBlockAnnouncementTargetCount != 5 {
		t.Fatalf("announcement target count = %d, want 5", status.LocalBlockAnnouncementTargetCount)
	}
}

func TestCheckSafeToMineLocalGraceWithoutHeightHashIsSafe(t *testing.T) {
	input := safeMiningInput()
	input.AgreeingPeerCount = 1
	input.LocalBlockPropagationGraceActive = true
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("grace with flag set should be safe even without height/hash: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceDoesNotFallBackToPeerGrace(t *testing.T) {
	input := graceInput()
	status := CheckSafeToMine(input)
	if status.PeerAgreementGraceActive {
		t.Fatalf("local block grace active should prevent fallback to peer grace: %+v", status)
	}
	if status.PeerSafetyPauseActive {
		t.Fatalf("local block grace active should prevent safety pause: %+v", status)
	}
}

func TestCheckSafeToMineLocalGraceWithRemoteBlockOnly(t *testing.T) {
	input := safeMiningInput()
	input.LocalBlockPropagationHeight = 0
	input.LocalBlockPropagationHash = ""
	input.LocalBlockPropagationGraceActive = false
	input.LocalBlockPropagationGraceRemaining = 0
	status := CheckSafeToMine(input)
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("no local block must not activate grace: %+v", status)
	}
}

func TestCheckSafeToMineStartupCannotActivateGrace(t *testing.T) {
	input := safeMiningInput()
	input.LocalHeight = 0
	input.BestPeerHeight = 50
	input.BlocksBehind = 50
	input.AgreeingPeerCount = 0
	input.LocalBlockPropagationGraceActive = false
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("startup with no local block and far behind must block: %+v", status)
	}
	if status.LocalBlockPropagationGraceActive {
		t.Fatalf("startup must not activate grace: %+v", status)
	}
}

func TestCheckSafeToMineTwoLaggingPeersUnderGrace(t *testing.T) {
	input := graceInput()
	input.AgreeingPeerCount = 0
	input.CompatiblePeerCount = 4
	input.Lagging1PeerCount = 2
	input.Lagging2PeerCount = 2
	status := CheckSafeToMine(input)
	if !status.Safe || status.State != "degraded" {
		t.Fatalf("2 lagging peers should still be degraded/safe under grace: %+v", status)
	}
	if !status.LocalBlockPropagationGraceActive {
		t.Fatalf("lagging peers should not clear grace: %+v", status)
	}
}

func TestServerLocalGraceAnnouncementFieldsSetAfterBlockAcceptance(t *testing.T) {
	// Verify that the announcement fields set by the miner loop after block
	// acceptance are correctly propagated through applyLocalBlockPropagationGraceLocked.
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	tip := chain.Tip()
	now := time.Now()
	s := &Server{chain: chain}
	s.minerMu.Lock()
	s.minerLastLocalBlockAnnouncement = now
	s.minerLocalBlockAnnouncementPeers = 7
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = now
	s.minerLocalBlockGraceHeight = tip.Height
	s.minerLocalBlockGraceHash = tip.Hash
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           7,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 7,
	}
	s.minerMu.Lock()
	result := s.applyLocalBlockPropagationGraceLocked(&input, now, 2)
	s.minerMu.Unlock()
	if !input.LocalBlockPropagationGraceActive {
		t.Fatalf("grace must be active when all conditions pass: %+v", input)
	}
	if input.LocalBlockPropagationHeight != tip.Height {
		t.Fatalf("height = %d, want %d", input.LocalBlockPropagationHeight, tip.Height)
	}
	if input.LocalBlockPropagationHash != tip.Hash {
		t.Fatalf("hash = %q, want %q", input.LocalBlockPropagationHash, tip.Hash)
	}
	if input.LastLocalBlockAnnouncementTime != now.Unix() {
		t.Fatalf("announcement time = %d, want %d", input.LastLocalBlockAnnouncementTime, now.Unix())
	}
	if input.LocalBlockAnnouncementTargetCount != 7 {
		t.Fatalf("announcement target count = %d, want 7", input.LocalBlockAnnouncementTargetCount)
	}
	if result {
		t.Fatalf("grace must not be denied: result=%v", result)
	}
}

func TestCheckSafeToMineLocalGraceAnnouncementFieldIsTargetCount(t *testing.T) {
	// Verify the diagnostic field is documented as target count, not successful receipt.
	input := graceInput()
	input.LocalBlockAnnouncementTargetCount = 5
	status := CheckSafeToMine(input)
	if status.LocalBlockAnnouncementTargetCount != 5 {
		t.Fatalf("target count = %d, want 5", status.LocalBlockAnnouncementTargetCount)
	}
}

func TestCheckSafeToMineLocalGraceReorgDoesNotAffectSafety(t *testing.T) {
	input := graceInput()
	input.CurrentTipHeight = 101
	input.CurrentTipHash = "newtip"
	input.CurrentTemplateHeight = 102
	input.ActiveTemplatePrevHash = "newtip"
	status := CheckSafeToMine(input)
	if !status.Safe {
		t.Fatalf("reorg data in input should not change grace at CheckSafeToMine level: %+v", status)
	}
}

func TestServerLocalGraceClearsWithNilChain(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerLastLocalBlockAnnouncement = time.Now()
	s.minerLocalBlockAnnouncementPeers = 5
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 4,
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("nil chain must clear grace: %+v", input)
	}
}

func TestServerLocalGraceClearsWithExpiredTime(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now().Add(-121 * time.Second)
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 4,
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("expired time must clear grace: %+v", input)
	}
}

func TestServerLocalGraceClearsWithNodeBehind(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		BlocksBehind:        2,
		BestPeerHeight:      103,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 4,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("node behind canaries must clear grace: %+v", input)
	}
}

func TestServerLocalGraceClearsWithInsufficientCompatiblePeers(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 1,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("insufficient compatible peers must clear grace: %+v", input)
	}
}

func TestServerLocalGraceClearsWithConflictingTip(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:               4,
		MinAgreeingPeers:        2,
		BlocksBehindAllowed:     1,
		AgreeingPeerCount:       0,
		CompatiblePeerCount:     4,
		ConflictingTipPeerCount: 1,
		SyncState:               "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("conflicting tip must clear grace: %+v", input)
	}
}

func TestServerLocalGraceClearsWhenAgreeingPeersReached(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   2,
		CompatiblePeerCount: 4,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("sufficient agreeing peers must clear grace: %+v", input)
	}
}

func TestServerLocalGraceExpiredPausesImmediately(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now().Add(-121 * time.Second)
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 1,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("expired local grace must not remain active: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("expired local grace must not fall back to peer agreement grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("expired local grace with no agreeing peers must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("expired local grace must produce state=unsafe, got %q: %+v", status.State, status)
	}
}

func TestServerLocalGraceInsufficientCompatiblePeersPausesImmediately(t *testing.T) {
	s := &Server{}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = 100
	s.minerLocalBlockGraceHash = "abc123"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 1,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("<2 compatible peers must clear grace: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("<2 compatible peers must not start peer grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("<2 compatible peers with no agreeing must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("<2 compatible peers must produce state=unsafe, got %q: %+v", status.State, status)
	}
}

func TestServerLocalGraceReorgPausesImmediately(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	tip := chain.Tip()
	s := &Server{chain: chain}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = tip.Height + 5
	s.minerLocalBlockGraceHash = "different_hash"
	s.minerMu.Unlock()
	input := MiningSafetyInput{
		PeerCount:           4,
		MinAgreeingPeers:    2,
		BlocksBehindAllowed: 1,
		AgreeingPeerCount:   0,
		CompatiblePeerCount: 4,
		SyncState:           "current",
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("reorg/orphan must clear grace: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("reorg/orphan must not start peer grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("reorg/orphan with no agreeing must produce unsafe: %+v", status)
	}
}

func TestServerImmediateRiskStrongerChainworkClearsGrace(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	s := &Server{chain: chain}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = chain.Tip().Height
	s.minerLocalBlockGraceHash = chain.Tip().Hash
	s.minerPeerAgreementLostSince = time.Now().Add(-60 * time.Second)
	s.minerMu.Unlock()

	input := MiningSafetyInput{
		PeerCount:                   4,
		MinAgreeingPeers:            2,
		BlocksBehindAllowed:         1,
		BlocksBehind:                0,
		AgreeingPeerCount:           0,
		CompatiblePeerCount:         4,
		SyncState:                   "current",
		StrongerChainworkPeerCount:  1,
		PeerGraceSeconds:            90,
		PeerRecoverySeconds:         30,
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("stronger chainwork must clear local grace: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("stronger chainwork must not activate peer grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("stronger chainwork must produce unsafe: %+v", status)
	}
	if status.PeerAgreementGraceActive {
		t.Fatalf("stronger chainwork must bypass peer grace: %+v", status)
	}
}

func TestServerImmediateRiskWrongChainClearsGrace(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	s := &Server{chain: chain}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = chain.Tip().Height
	s.minerLocalBlockGraceHash = chain.Tip().Hash
	s.minerMu.Unlock()

	input := MiningSafetyInput{
		PeerCount:            4,
		MinAgreeingPeers:     2,
		BlocksBehindAllowed:  1,
		BlocksBehind:         0,
		AgreeingPeerCount:    0,
		CompatiblePeerCount:  4,
		SyncState:            "current",
		WrongChainPeerCount:  1,
		PeerGraceSeconds:     90,
		PeerRecoverySeconds:  30,
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("wrong chain must clear local grace: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("wrong chain must not activate peer grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("wrong chain must produce unsafe: %+v", status)
	}
	if status.PeerAgreementGraceActive {
		t.Fatalf("wrong chain must bypass peer grace: %+v", status)
	}
}

func TestServerImmediateRiskProtocolErrorClearsGrace(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	s := &Server{chain: chain}
	s.minerMu.Lock()
	s.minerLocalBlockGraceActive = true
	s.minerLocalBlockGraceStartedAt = time.Now()
	s.minerLocalBlockGraceHeight = chain.Tip().Height
	s.minerLocalBlockGraceHash = chain.Tip().Hash
	s.minerMu.Unlock()

	input := MiningSafetyInput{
		PeerCount:              4,
		MinAgreeingPeers:       2,
		BlocksBehindAllowed:    1,
		BlocksBehind:           0,
		AgreeingPeerCount:      0,
		CompatiblePeerCount:    4,
		SyncState:              "current",
		ProtocolErrorPeerCount: 1,
		PeerGraceSeconds:       90,
		PeerRecoverySeconds:    30,
	}
	s.applyMiningPeerAgreementWindow(&input)
	if input.LocalBlockPropagationGraceActive {
		t.Fatalf("protocol error must clear local grace: %+v", input)
	}
	if input.PeerAgreementGraceActive {
		t.Fatalf("protocol error must not activate peer grace: %+v", input)
	}
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("protocol error must produce unsafe: %+v", status)
	}
	if status.PeerAgreementGraceActive {
		t.Fatalf("protocol error must bypass peer grace: %+v", status)
	}
}

func TestCheckSafeToMineStorageFailureBlocksImmediately(t *testing.T) {
	input := graceInput()
	input.StorageOK = false
	input.StorageError = "disk full"
	input.AgreeingPeerCount = 4
	input.LocalBlockPropagationGraceActive = true
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("storage failure must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("storage failure must be unsafe: %+v", status)
	}
	if status.Reason == "" {
		t.Fatal("storage failure must have a reason")
	}
}

func TestCheckSafeToMineInvalidPayoutBlocksImmediately(t *testing.T) {
	input := graceInput()
	input.DestinationOK = false
	input.DestinationError = "reward address is not owned by this wallet"
	input.AgreeingPeerCount = 4
	input.LocalBlockPropagationGraceActive = true
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("invalid payout must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("invalid payout must be unsafe: %+v", status)
	}
	if status.Reason == "" {
		t.Fatal("invalid payout must have a reason")
	}
}

func TestCheckSafeToMineRPCTimeoutBlocksEvenWithGrace(t *testing.T) {
	input := graceInput()
	input.RPCHealth = "timeout"
	input.AgreeingPeerCount = 4
	input.LocalBlockPropagationGraceActive = true
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("RPC timeout must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("RPC timeout must be unsafe even with grace: %+v", status)
	}
}

func TestCheckSafeToMineHardStaleTemplateBlocksEvenWithGrace(t *testing.T) {
	input := graceInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.CurrentTipHeight = 101
	input.CurrentTemplateHeight = 102
	input.CurrentTipHash = "tip"
	input.ActiveTemplatePrevHash = "tip"
	input.TemplateAgeSeconds = 21 * 60
	input.TemplateSoftRefreshAgeSeconds = 30
	input.TemplateMaxAgeSeconds = 20 * 60
	input.AgreeingPeerCount = 4
	input.LocalBlockPropagationGraceActive = true
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("hard stale template must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("hard stale template must be unsafe even with grace: %+v", status)
	}
}

func TestCheckSafeToMinePrevHashMismatchBlocksEvenWithGrace(t *testing.T) {
	input := graceInput()
	input.HasActiveTemplate = true
	input.ActiveTemplateFresh = true
	input.CurrentTemplateHeight = 102
	input.CurrentTipHeight = 101
	input.ActiveTemplatePrevHash = "old"
	input.CurrentTipHash = "new"
	input.AgreeingPeerCount = 4
	input.LocalBlockPropagationGraceActive = true
	input.PeerAgreementGraceActive = true
	status := CheckSafeToMine(input)
	if status.Safe {
		t.Fatalf("prev hash mismatch must produce unsafe: %+v", status)
	}
	if status.State != "unsafe" {
		t.Fatalf("prev hash mismatch must be unsafe even with grace: %+v", status)
	}
}

func TestCheckSafeToMineGraceDoesNotResetOnRepeatedEvaluation(t *testing.T) {
	input := MiningSafetyInput{
		PeerCount:                         4,
		MinAgreeingPeers:                  2,
		BlocksBehindAllowed:               1,
		BlocksBehind:                      1,
		AgreeingPeerCount:                 0,
		CompatiblePeerCount:               2,
		SyncState:                         "current",
		PeerGraceSeconds:                  90,
		PeerRecoverySeconds:               30,
		ConflictingTipPeerCount:           1,
		RPCHealth:                         "ok",
		StorageOK:                         true,
		DestinationOK:                     true,
	}
	for i := 0; i < 5; i++ {
		status := CheckSafeToMine(input)
		if status.Safe {
			t.Fatalf("iteration %d: conflicting tip must be unsafe: %+v", i, status)
		}
		if status.PeerAgreementGraceActive {
			t.Fatalf("iteration %d: conflicting tip must not activate peer grace: %+v", i, status)
		}
		if status.State != "unsafe" {
			t.Fatalf("iteration %d: conflicting tip must remain unsafe: %+v", i, status)
		}
	}
}
