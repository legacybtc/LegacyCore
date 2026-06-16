package rpc

import "strings"

const (
	defaultMiningMinGoodPeers           = 3
	defaultMiningMinAgreeingPeers       = 2
	defaultMiningPeerGraceSeconds       = 90
	defaultMiningPeerRecoverySeconds    = 30
	defaultMiningBlocksBehindLimit      = 1
	defaultLocalBlockPropagationSeconds = 120
	minerStaleWarningRate               = 0.10
	minerStaleStrongWarningRate         = 0.30
	minerStalePauseRate                 = 0.50
)

type MiningSafetyInput struct {
	RPCHealth                      string
	StorageOK                      bool
	StorageError                   string
	DestinationOK                  bool
	DestinationError               string
	SafeRequired                   bool
	AllowUnsafe                    bool
	MinGoodPeers                   int
	MinAgreeingPeers               int
	PeerGraceSeconds               int
	PeerRecoverySeconds            int
	BlocksBehindAllowed            int
	LocalHeight                    int32
	BestPeerHeight                 int32
	BlocksBehind                   int32
	PeerCount                      int
	GoodPeerCount                  int
	AgreeingPeerCount              int
	CompatiblePeerCount            int
	Lagging1PeerCount              int
	Lagging2PeerCount              int
	LaggingMorePeerCount           int
	StaleChainDataPeerCount        int
	UnresponsivePeerCount          int
	ConflictingTipPeerCount        int
	StrongerChainworkPeerCount     int
	WrongChainPeerCount            int
	ProtocolErrorPeerCount         int
	StalePeerCount                 int
	SyncState                      string
	RequestInFlight                bool
	NoUsefulChainData              bool
	LastSyncError                  string
	RecentReorg                    bool
	PeerSplit                      bool
	AcceptedBlocks                 int64
	StaleBlocks                    int64
	RejectedBlocks                 int64
	HasActiveTemplate              bool
	ActiveTemplateFresh            bool
	ActiveTemplateRefreshDue       bool
	ActiveTemplateStaleReason      string
	ActiveTemplateRefreshReason    string
	ActiveTemplatePrevHash         string
	CurrentTipHash                 string
	TemplateAgeSeconds             float64
	CurrentTemplateHeight          int32
	CurrentTipHeight               int32
	TemplateSoftRefreshAgeSeconds  float64
	TemplateMaxAgeSeconds          float64
	StaleRatePauseActive           bool
	PeerAgreementGraceActive       bool
	PeerAgreementGraceRemaining    int
	PeerAgreementRecoveryActive    bool
	PeerAgreementRecoveryRemaining int
	PeerSafetyPauseActive          bool
	LocalBlockPropagationGraceActive      bool
	LocalBlockPropagationGraceRemaining   int
	LocalBlockPropagationHeight           int32
	LocalBlockPropagationHash             string
	LastLocalBlockAnnouncementTime        int64
	LocalBlockAnnouncementTargetCount       int
	ExactAgreeingPeerCount                int
}

type MiningSafetyStatus struct {
	Safe                           bool
	State                          string
	Reason                         string
	RPCHealth                      string
	LocalHeight                    int32
	BestPeerHeight                 int32
	BlocksBehind                   int32
	PeerCount                      int
	GoodPeerCount                  int
	AgreeingPeerCount              int
	CompatiblePeerCount            int
	Lagging1PeerCount              int
	Lagging2PeerCount              int
	LaggingMorePeerCount           int
	StaleChainDataPeerCount        int
	UnresponsivePeerCount          int
	ConflictingTipPeerCount        int
	StrongerChainworkPeerCount     int
	WrongChainPeerCount            int
	ProtocolErrorPeerCount         int
	MinAgreeingPeers               int
	PeerGraceSeconds               int
	PeerRecoverySeconds            int
	PeerAgreementGraceActive       bool
	PeerAgreementGraceRemaining    int
	PeerAgreementRecoveryActive    bool
	PeerAgreementRecoveryRemaining int
	PeerSafetyPauseActive          bool
	StalePeerCount                 int
	SyncState                      string
	RequestInFlight                bool
	NoUsefulChainData              bool
	RecentReorg                    bool
	PeerSplit                      bool
	StaleRate                      float64
	StaleRateWarning               string
	UnsafeOverride                 bool
	StorageOK                      bool
	DestinationOK                  bool
	HasActiveTemplate              bool
	ActiveTemplateFresh            bool
	ActiveTemplateRefreshDue       bool
	ActiveTemplateStaleReason      string
	ActiveTemplateRefreshReason    string
	ActiveTemplatePrevHash         string
	CurrentTipHash                 string
	TemplateAgeSeconds             float64
	CurrentTipHeight               int32
	CurrentTemplateHeight          int32
	TemplateSoftRefreshAgeSeconds  float64
	TemplateMaxAgeSeconds          float64
	StaleRatePauseActive           bool
	LocalBlockPropagationGraceActive      bool
	LocalBlockPropagationGraceRemaining   int
	LocalBlockPropagationHeight           int32
	LocalBlockPropagationHash             string
	LastLocalBlockAnnouncementTime        int64
	LocalBlockAnnouncementTargetCount       int
	ExactAgreeingPeerCount                int
}

func (s MiningSafetyStatus) Fields() map[string]any {
	return map[string]any{
		"safe_to_mine":                              s.Safe,
		"mining_safe":                               s.Safe,
		"mining_safety_state":                       s.State,
		"mining_blocked_reason":                     s.Reason,
		"mining_safety_reason":                      s.Reason,
		"rpc_health":                                s.RPCHealth,
		"sync_state":                                s.SyncState,
		"blocks_behind":                             s.BlocksBehind,
		"peer_count":                                s.PeerCount,
		"good_peer_count":                           s.GoodPeerCount,
		"current_agreeing_peer_count":               s.AgreeingPeerCount,
		"compatible_peer_count":                     s.CompatiblePeerCount,
		"lagging_1_block_peer_count":                s.Lagging1PeerCount,
		"lagging_2_blocks_peer_count":               s.Lagging2PeerCount,
		"lagging_more_than_2_peer_count":            s.LaggingMorePeerCount,
		"stale_chain_data_peer_count":               s.StaleChainDataPeerCount,
		"unresponsive_peer_count":                   s.UnresponsivePeerCount,
		"conflicting_tip_peer_count":                s.ConflictingTipPeerCount,
		"stronger_chainwork_peer_count":             s.StrongerChainworkPeerCount,
		"wrong_chain_peer_count":                    s.WrongChainPeerCount,
		"protocol_error_peer_count":                 s.ProtocolErrorPeerCount,
		"min_agreeing_peers":                        s.MinAgreeingPeers,
		"peer_safety_grace_seconds":                 s.PeerGraceSeconds,
		"peer_safety_recovery_seconds":              s.PeerRecoverySeconds,
		"peer_agreement_grace_active":               s.PeerAgreementGraceActive,
		"peer_agreement_grace_remaining_seconds":    s.PeerAgreementGraceRemaining,
		"peer_agreement_recovery_active":            s.PeerAgreementRecoveryActive,
		"peer_agreement_recovery_remaining_seconds": s.PeerAgreementRecoveryRemaining,
		"peer_safety_pause_active":                  s.PeerSafetyPauseActive,
		"stale_peer_count":                          s.StalePeerCount,
		"best_peer_height":                          s.BestPeerHeight,
		"local_height":                              s.LocalHeight,
		"request_in_flight":                         s.RequestInFlight,
		"no_useful_chain_data":                      s.NoUsefulChainData,
		"recent_reorg":                              s.RecentReorg,
		"peer_split":                                s.PeerSplit,
		"stale_rate":                                s.StaleRate,
		"stale_rate_warning":                        s.StaleRateWarning,
		"unsafe_override":                           s.UnsafeOverride,
		"storage_ok":                                s.StorageOK,
		"destination_ok":                            s.DestinationOK,
		"has_active_template":                       s.HasActiveTemplate,
		"active_template_is_fresh":                  s.ActiveTemplateFresh,
		"active_template_refresh_due":               s.ActiveTemplateRefreshDue,
		"active_template_stale_reason":              s.ActiveTemplateStaleReason,
		"active_template_refresh_reason":            s.ActiveTemplateRefreshReason,
		"active_template_prev_hash":                 s.ActiveTemplatePrevHash,
		"current_tip_hash":                          s.CurrentTipHash,
		"last_template_age":                         s.TemplateAgeSeconds,
		"current_template_height":                   s.CurrentTemplateHeight,
		"current_tip_height":                        s.CurrentTipHeight,
		"template_soft_refresh_age_seconds":         s.TemplateSoftRefreshAgeSeconds,
		"template_max_age_seconds":                  s.TemplateMaxAgeSeconds,
		"template_hard_stale_age_seconds":           s.TemplateMaxAgeSeconds,
		"stale_rate_pause_active":                   s.StaleRatePauseActive,
		"local_block_propagation_grace_active":      s.LocalBlockPropagationGraceActive,
		"local_block_propagation_grace_remaining":   s.LocalBlockPropagationGraceRemaining,
		"local_block_propagation_height":            s.LocalBlockPropagationHeight,
		"local_block_propagation_hash":              s.LocalBlockPropagationHash,
		"last_local_block_announcement_time":        s.LastLocalBlockAnnouncementTime,
		"local_block_announcement_target_count":       s.LocalBlockAnnouncementTargetCount,
		"exact_agreeing_peer_count":                 s.ExactAgreeingPeerCount,
	}
}

func CheckSafeToMine(input MiningSafetyInput) MiningSafetyStatus {
	rpcHealth := strings.ToLower(strings.TrimSpace(input.RPCHealth))
	if rpcHealth == "" {
		rpcHealth = "ok"
	}
	syncState := strings.ToLower(strings.TrimSpace(input.SyncState))
	if syncState == "" {
		syncState = "unknown"
	}
	minGoodPeers := input.MinGoodPeers
	if minGoodPeers <= 0 {
		minGoodPeers = defaultMiningMinGoodPeers
	}
	minAgreeingPeers := input.MinAgreeingPeers
	if minAgreeingPeers <= 0 {
		minAgreeingPeers = defaultMiningMinAgreeingPeers
	}
	peerGraceSeconds := input.PeerGraceSeconds
	if peerGraceSeconds <= 0 {
		peerGraceSeconds = defaultMiningPeerGraceSeconds
	}
	peerRecoverySeconds := input.PeerRecoverySeconds
	if peerRecoverySeconds <= 0 {
		peerRecoverySeconds = defaultMiningPeerRecoverySeconds
	}
	blocksBehindAllowed := input.BlocksBehindAllowed
	if blocksBehindAllowed < 0 {
		blocksBehindAllowed = defaultMiningBlocksBehindLimit
	}
	blocksBehind := input.BlocksBehind
	if blocksBehind < 0 {
		blocksBehind = 0
	}
	if input.BestPeerHeight > input.LocalHeight && input.BestPeerHeight-input.LocalHeight > blocksBehind {
		blocksBehind = input.BestPeerHeight - input.LocalHeight
	}
	totalBlocks := input.AcceptedBlocks + input.StaleBlocks + input.RejectedBlocks
	staleRate := float64(0)
	if totalBlocks > 0 {
		staleRate = float64(input.StaleBlocks) / float64(totalBlocks)
	}
	staleWarning := ""
	if totalBlocks > 0 && staleRate >= minerStalePauseRate {
		staleWarning = "Mining paused: high stale rate. Refreshing mining template and waiting for reliable peers."
	} else if totalBlocks > 0 && staleRate >= minerStaleStrongWarningRate {
		staleWarning = "Mining strong warning: high stale rate. Refreshing mining template."
	} else if totalBlocks > 0 && staleRate >= minerStaleWarningRate {
		staleWarning = "Mining warning: high stale rate. Refreshing mining template."
	}
	status := MiningSafetyStatus{
		Safe:                           true,
		State:                          "safe",
		Reason:                         "",
		RPCHealth:                      rpcHealth,
		LocalHeight:                    input.LocalHeight,
		BestPeerHeight:                 input.BestPeerHeight,
		BlocksBehind:                   blocksBehind,
		PeerCount:                      input.PeerCount,
		GoodPeerCount:                  input.GoodPeerCount,
		AgreeingPeerCount:              input.AgreeingPeerCount,
		CompatiblePeerCount:            input.CompatiblePeerCount,
		Lagging1PeerCount:              input.Lagging1PeerCount,
		Lagging2PeerCount:              input.Lagging2PeerCount,
		LaggingMorePeerCount:           input.LaggingMorePeerCount,
		StaleChainDataPeerCount:        input.StaleChainDataPeerCount,
		UnresponsivePeerCount:          input.UnresponsivePeerCount,
		ConflictingTipPeerCount:        input.ConflictingTipPeerCount,
		StrongerChainworkPeerCount:     input.StrongerChainworkPeerCount,
		WrongChainPeerCount:            input.WrongChainPeerCount,
		ProtocolErrorPeerCount:         input.ProtocolErrorPeerCount,
		MinAgreeingPeers:               minAgreeingPeers,
		PeerGraceSeconds:               peerGraceSeconds,
		PeerRecoverySeconds:            peerRecoverySeconds,
		PeerAgreementGraceActive:       input.PeerAgreementGraceActive,
		PeerAgreementGraceRemaining:    input.PeerAgreementGraceRemaining,
		PeerAgreementRecoveryActive:    input.PeerAgreementRecoveryActive,
		PeerAgreementRecoveryRemaining: input.PeerAgreementRecoveryRemaining,
		PeerSafetyPauseActive:          input.PeerSafetyPauseActive,
		StalePeerCount:                 input.StalePeerCount,
		SyncState:                      syncState,
		RequestInFlight:                input.RequestInFlight,
		NoUsefulChainData:              input.NoUsefulChainData,
		RecentReorg:                    input.RecentReorg,
		PeerSplit:                      input.PeerSplit,
		StaleRate:                      staleRate,
		StaleRateWarning:               staleWarning,
		StorageOK:                      input.StorageOK,
		DestinationOK:                  input.DestinationOK,
		HasActiveTemplate:              input.HasActiveTemplate,
		ActiveTemplateFresh:            input.ActiveTemplateFresh,
		ActiveTemplateRefreshDue:       input.ActiveTemplateRefreshDue,
		ActiveTemplateStaleReason:      strings.TrimSpace(input.ActiveTemplateStaleReason),
		ActiveTemplateRefreshReason:    strings.TrimSpace(input.ActiveTemplateRefreshReason),
		ActiveTemplatePrevHash:         strings.TrimSpace(input.ActiveTemplatePrevHash),
		CurrentTipHash:                 strings.TrimSpace(input.CurrentTipHash),
		TemplateAgeSeconds:             input.TemplateAgeSeconds,
		CurrentTemplateHeight:          input.CurrentTemplateHeight,
		CurrentTipHeight:               input.CurrentTipHeight,
		TemplateSoftRefreshAgeSeconds:  input.TemplateSoftRefreshAgeSeconds,
		TemplateMaxAgeSeconds:          input.TemplateMaxAgeSeconds,
		StaleRatePauseActive:           input.StaleRatePauseActive,
	}
	staleTemplateReason := ""
	if input.HasActiveTemplate && !input.ActiveTemplateFresh {
		staleTemplateReason = status.ActiveTemplateStaleReason
	}
	if input.HasActiveTemplate && input.TemplateMaxAgeSeconds > 0 && input.TemplateAgeSeconds > input.TemplateMaxAgeSeconds {
		staleTemplateReason = "template age exceeds hard stale limit"
	}
	if input.HasActiveTemplate && input.CurrentTemplateHeight > 0 && input.CurrentTipHeight >= 0 && input.CurrentTemplateHeight != input.CurrentTipHeight+1 {
		staleTemplateReason = "template height is not current tip height + 1"
	}
	if input.HasActiveTemplate && input.ActiveTemplatePrevHash != "" && input.CurrentTipHash != "" && input.ActiveTemplatePrevHash != input.CurrentTipHash {
		staleTemplateReason = "template prev hash does not match current tip"
	}
	if staleTemplateReason != "" {
		status.ActiveTemplateStaleReason = staleTemplateReason
		status.ActiveTemplateRefreshDue = true
		if status.ActiveTemplateRefreshReason == "" {
			status.ActiveTemplateRefreshReason = staleTemplateRefreshReason(staleTemplateReason)
		}
	} else if input.HasActiveTemplate && input.ActiveTemplateFresh {
		status.ActiveTemplateStaleReason = ""
		if !status.ActiveTemplateRefreshDue {
			status.ActiveTemplateRefreshReason = ""
		} else if staleTemplateHardReason(status.ActiveTemplateRefreshReason) {
			status.ActiveTemplateRefreshReason = "refreshing template in background; current template still valid"
		}
	}
	if !input.SafeRequired && input.AllowUnsafe {
		status.UnsafeOverride = true
		status.State = "unsafe_override"
		status.Reason = "Unsafe mining override enabled by expert config."
		return status
	}
	block := func(reason string) MiningSafetyStatus {
		status.Safe = false
		status.State = "unsafe"
		status.Reason = reason
		return status
	}
	switch rpcHealth {
	case "timeout":
		return block("Mining blocked: RPC is not responding.")
	case "offline":
		return block("Mining blocked: RPC is offline.")
	}
	if !input.StorageOK {
		if strings.TrimSpace(input.StorageError) != "" {
			return block("Mining blocked: storage health failed: " + strings.TrimSpace(input.StorageError))
		}
		return block("Mining blocked: storage health failed.")
	}
	if !input.DestinationOK {
		if strings.TrimSpace(input.DestinationError) != "" {
			return block(strings.TrimSpace(input.DestinationError))
		}
		return block("Mining blocked: reward destination is not safe.")
	}
	if input.PeerCount == 0 || syncState == "no_peers" || syncState == "offline" {
		return block("Mining blocked: node has no peers.")
	}
	if input.StrongerChainworkPeerCount > 0 {
		return block("Mining paused: peer reports a stronger chain candidate.")
	}
	if input.ConflictingTipPeerCount > 0 {
		return block("Mining paused: connected peers report a conflicting tip.")
	}
	if input.WrongChainPeerCount > 0 {
		return block("Mining paused: wrong-chain peer detected.")
	}
	if input.ProtocolErrorPeerCount > 0 {
		return block("Mining paused: peer protocol/header/block error detected.")
	}
	if blocksBehind > int32(blocksBehindAllowed) {
		return block("Mining blocked: node is behind peers by " + int32String(blocksBehind) + " blocks.")
	}
	switch syncState {
	case "catching_up", "requesting_blocks", "possibly_stalled", "stalled":
		return block("Mining blocked: node is not safely synced to the public chain.")
	case "unknown":
		return block("Mining blocked: peer sync state is unknown.")
	}
	if input.RequestInFlight && !miningSyncStateCurrent(syncState, blocksBehind, int32(blocksBehindAllowed)) {
		return block("Mining blocked: sync request is still in progress.")
	}
	agreeingPeerCount := input.AgreeingPeerCount
	if agreeingPeerCount == 0 && input.CompatiblePeerCount == 0 && input.GoodPeerCount > 0 {
		agreeingPeerCount = input.GoodPeerCount
		status.AgreeingPeerCount = agreeingPeerCount
		status.CompatiblePeerCount = input.GoodPeerCount
	}
	status.ExactAgreeingPeerCount = agreeingPeerCount
	status.LocalBlockPropagationGraceActive = input.LocalBlockPropagationGraceActive
	status.LocalBlockPropagationGraceRemaining = input.LocalBlockPropagationGraceRemaining
	status.LocalBlockPropagationHeight = input.LocalBlockPropagationHeight
	status.LocalBlockPropagationHash = input.LocalBlockPropagationHash
	status.LastLocalBlockAnnouncementTime = input.LastLocalBlockAnnouncementTime
	status.LocalBlockAnnouncementTargetCount = input.LocalBlockAnnouncementTargetCount
	if minAgreeingPeers > 0 && agreeingPeerCount < minAgreeingPeers {
		if agreeingPeerCount >= 1 &&
			input.CompatiblePeerCount >= int(minAgreeingPeers) &&
			input.Lagging1PeerCount >= 1 &&
			input.ConflictingTipPeerCount == 0 &&
			input.StrongerChainworkPeerCount == 0 &&
			input.WrongChainPeerCount == 0 &&
			input.ProtocolErrorPeerCount == 0 &&
			input.BlocksBehind <= int32(input.BlocksBehindAllowed) {
			status.State = "degraded"
			status.Reason = "Mining degraded: one or more healthy peers are 1 block behind (normal propagation). Mining continues."
			return status
		}
		if input.LocalBlockPropagationGraceActive {
			graceState := "degraded"
			graceReason := "Locally mined block is propagating to peers; mining continues during a bounded safety grace."
			if input.PeerAgreementGraceActive {
				graceReason = "Locally mined block is propagating to peers; mining continues during a bounded safety grace."
			}
			status.State = graceState
			status.Reason = graceReason
			return status
		}
		if input.PeerAgreementGraceActive {
			status.State = "degraded"
			status.Reason = "Mining degraded: waiting for more current agreeing peers; grace period active."
			return status
		}
		if input.PeerAgreementRecoveryActive {
			return block("Mining paused: waiting for peer agreement recovery hysteresis.")
		}
		return block("Mining paused: fewer than " + int32String(int32(minAgreeingPeers)) + " current agreeing peer(s).")
	}
	if input.LocalBlockPropagationGraceActive {
		input.LocalBlockPropagationGraceActive = false
		status.LocalBlockPropagationGraceActive = false
	}
	if input.NoUsefulChainData {
		return block("Mining blocked: peer data is stale.")
	}
	if input.RecentReorg {
		return block("Mining blocked: recent reorg detected; waiting for chain stability.")
	}
	if input.PeerSplit {
		return block("Mining blocked: connected peers disagree about the public chain tip.")
	}
	if input.HasActiveTemplate && !input.ActiveTemplateFresh {
		reason := strings.TrimSpace(input.ActiveTemplateStaleReason)
		if reason == "" {
			reason = "template is stale"
		}
		return block("Mining paused: template is stale; waiting for fresh block template. " + reason)
	}
	if input.HasActiveTemplate && input.CurrentTemplateHeight > 0 && input.CurrentTipHeight >= 0 && input.CurrentTemplateHeight != input.CurrentTipHeight+1 {
		return block("Mining paused: template height is not current tip height + 1.")
	}
	if input.HasActiveTemplate && input.ActiveTemplatePrevHash != "" && input.CurrentTipHash != "" && input.ActiveTemplatePrevHash != input.CurrentTipHash {
		return block("Mining paused: template prev hash does not match current tip.")
	}
	if input.HasActiveTemplate && input.TemplateMaxAgeSeconds > 0 && input.TemplateAgeSeconds > input.TemplateMaxAgeSeconds {
		return block("Mining paused: template refresh failed; template age exceeds hard stale limit.")
	}
	if input.StaleRatePauseActive || (totalBlocks > 0 && staleRate >= minerStalePauseRate) {
		return block("Mining paused: high stale rate and unstable mining templates.")
	}
	return status
}

func miningSyncStateCurrent(syncState string, blocksBehind, blocksBehindAllowed int32) bool {
	switch syncState {
	case "current", "synced":
		return blocksBehind <= blocksBehindAllowed
	default:
		return false
	}
}

func staleTemplateRefreshReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	normalized := strings.ToLower(trimmed)
	switch {
	case strings.Contains(normalized, "prev hash"):
		return "prev_hash_mismatch: template prev hash does not match current tip"
	case strings.Contains(normalized, "height"):
		return "height_mismatch: template height is not current tip height + 1"
	case strings.Contains(normalized, "age"):
		return "hard_stale_template: template age exceeds hard stale limit"
	case trimmed != "":
		return "template_stale: " + trimmed
	default:
		return "template_stale: refreshing stale mining template"
	}
}

func staleTemplateHardReason(reason string) bool {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	return strings.Contains(normalized, "template_stale") ||
		strings.Contains(normalized, "template unavailable") ||
		strings.Contains(normalized, "prev_hash_mismatch") ||
		strings.Contains(normalized, "height_mismatch") ||
		strings.Contains(normalized, "hard_stale_template")
}

func int32String(n int32) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
