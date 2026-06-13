package rpc

import "strings"

const (
	MinerStateStopped                   = "stopped"
	MinerStateStarting                  = "starting"
	MinerStateRunning                   = "running"
	MinerStateSoftRefreshingStillMining = "soft_refreshing_still_mining"
	MinerStatePausedUnsafe              = "paused_unsafe"
	MinerStatePausedHardStaleTemplate   = "paused_hard_stale_template"
	MinerStatePausedRPCTimeout          = "paused_rpc_timeout"
	MinerStatePausedSyncUnsafe          = "paused_sync_unsafe"
	MinerStatePausedPeerUnsafe          = "paused_peer_unsafe"
	MinerStatePausedPayoutInvalid       = "paused_payout_invalid"
	MinerStateWorkerStalled             = "worker_stalled"
	MinerStateError                     = "error"
	minerWorkerProgressGraceSeconds     = 10
)

type MinerRuntimeInput struct {
	SessionActive         bool
	WorkersHashing        bool
	ConfiguredThreads     int
	HashAttempts          uint64
	LastNonce             uint32
	LocalHashPS           float64
	WorkerEpochAgeSeconds float64
	SafetySafe            bool
	SafetyReason          string
	RPCHealth             string
	DataFresh             bool
	SyncState             string
	BlocksBehind          int32
	BlocksBehindAllowed   int32
	GoodPeerCount         int
	MinGoodPeers          int
	DestinationOK         bool
	DestinationError      string
	HasActiveTemplate     bool
	TemplateFresh         bool
	TemplateRefreshDue    bool
	TemplateStaleReason   string
	TemplateRefreshError  string
	LastError             string
	PausedReason          string
	LastStopReason        string
	EverStarted           bool
	StaleRatePauseActive  bool
	RecentReorg           bool
}

type MinerRuntimeState struct {
	State              string
	Reason             string
	ActiveThreads      int
	LiveHashing        bool
	ShouldHaveWorkers  bool
	SupervisorAction   string
	InvariantViolation string
}

func ResolveMinerRuntimeState(input MinerRuntimeInput) MinerRuntimeState {
	if !input.SessionActive {
		stopReason := strings.TrimSpace(input.LastStopReason)
		if stopReason == "" && input.EverStarted {
			stopReason = MinerStopWorkerExitUnexpected
		}
		if normalizeMinerStopReason(stopReason) == MinerStopNodeShutdown && !nodeShutdownReasonConsistent(input) {
			return MinerRuntimeState{State: MinerStateError, Reason: "Mining stop reason node_shutdown is inconsistent while node RPC is still online."}
		}
		if minerStopReasonIsUnexpected(stopReason) {
			return MinerRuntimeState{State: MinerStateError, Reason: "Mining stopped unexpectedly: " + normalizeMinerStopReason(stopReason)}
		}
		return MinerRuntimeState{State: MinerStateStopped, Reason: "miner is stopped"}
	}
	configuredThreads := input.ConfiguredThreads
	if configuredThreads < 0 {
		configuredThreads = 0
	}
	reason := firstMinerBlockerReason(input)
	if reason != "" {
		state := categorizeMinerPauseState(input, reason)
		return MinerRuntimeState{State: state, Reason: reason}
	}
	if configuredThreads <= 0 {
		return MinerRuntimeState{State: MinerStatePausedUnsafe, Reason: "Mining paused: no configured miner threads."}
	}
	if input.WorkersHashing {
		hasProgress := input.HashAttempts > 0 && (input.LastNonce > 0 || input.LocalHashPS > 0)
		if !hasProgress {
			if input.WorkerEpochAgeSeconds > 0 && input.WorkerEpochAgeSeconds < minerWorkerProgressGraceSeconds {
				return MinerRuntimeState{
					State:              MinerStateStarting,
					Reason:             "waiting for first hash progress",
					ActiveThreads:      0,
					LiveHashing:        false,
					ShouldHaveWorkers:  true,
					SupervisorAction:   "waiting_for_hash_progress",
					InvariantViolation: "worker epoch has started but no hash progress has been reported yet",
				}
			}
			return MinerRuntimeState{
				State:              MinerStateWorkerStalled,
				Reason:             "worker_not_hashing: active worker has no hash progress.",
				ActiveThreads:      0,
				LiveHashing:        false,
				ShouldHaveWorkers:  true,
				SupervisorAction:   "restart_workers",
				InvariantViolation: "active worker reported but hash attempts, nonce, and local H/s are still zero",
			}
		}
		state := MinerStateRunning
		reason := ""
		if input.TemplateRefreshDue && input.TemplateFresh {
			state = MinerStateSoftRefreshingStillMining
			reason = "refreshing template in background; current template still valid"
		}
		return MinerRuntimeState{
			State:             state,
			Reason:            reason,
			ActiveThreads:     configuredThreads,
			LiveHashing:       true,
			ShouldHaveWorkers: true,
		}
	}
	return MinerRuntimeState{
		State:              MinerStateRunning,
		Reason:             "",
		ActiveThreads:      configuredThreads,
		LiveHashing:        false,
		ShouldHaveWorkers:  true,
		SupervisorAction:   "resume_workers",
		InvariantViolation: "active session is safe but workers are not reporting live hashing; supervisor should resume workers",
	}
}

func nodeShutdownReasonConsistent(input MinerRuntimeInput) bool {
	rpcHealth := strings.ToLower(strings.TrimSpace(input.RPCHealth))
	if !input.DataFresh {
		return true
	}
	switch rpcHealth {
	case "offline", "timeout", "shutting_down", "shutdown", "stopping", "exiting":
		return true
	default:
		return false
	}
}

func firstMinerBlockerReason(input MinerRuntimeInput) string {
	rpcHealth := strings.ToLower(strings.TrimSpace(input.RPCHealth))
	if rpcHealth == "timeout" || rpcHealth == "offline" {
		return "Mining blocked: RPC is not responding."
	}
	if !input.DataFresh {
		return "Mining blocked: miner data is stale."
	}
	if !input.DestinationOK {
		if strings.TrimSpace(input.DestinationError) != "" {
			return strings.TrimSpace(input.DestinationError)
		}
		return "Mining blocked: reward destination is not safe."
	}
	if input.StaleRatePauseActive {
		return "Mining paused: high stale rate and unstable mining templates."
	}
	if input.RecentReorg {
		return "Mining blocked: recent reorg detected; waiting for chain stability."
	}
	if input.BlocksBehindAllowed < 0 {
		input.BlocksBehindAllowed = defaultMiningBlocksBehindLimit
	}
	if input.BlocksBehind > input.BlocksBehindAllowed {
		return "Mining blocked: node is behind peers by " + int32String(input.BlocksBehind) + " blocks."
	}
	switch strings.ToLower(strings.TrimSpace(input.SyncState)) {
	case "catching_up", "requesting_blocks", "possibly_stalled", "stalled", "unknown", "offline", "no_peers":
		return "Mining blocked: node is not safely synced to the public chain."
	}
	if input.HasActiveTemplate && !input.TemplateFresh {
		reason := strings.TrimSpace(input.TemplateStaleReason)
		if reason == "" {
			reason = "template is stale"
		}
		return "Mining paused: template is stale; waiting for fresh block template. " + reason
	}
	if strings.TrimSpace(input.TemplateRefreshError) != "" && !input.TemplateRefreshDue {
		return strings.TrimSpace(input.TemplateRefreshError)
	}
	if !input.SafetySafe {
		reason := strings.TrimSpace(input.SafetyReason)
		if reason != "" {
			return reason
		}
		return "Mining blocked: safety gate is not satisfied."
	}
	if strings.TrimSpace(input.PausedReason) != "" {
		if !isTransientMinerRecoveryReason(input.PausedReason) {
			return strings.TrimSpace(input.PausedReason)
		}
	}
	lastError := strings.TrimSpace(input.LastError)
	if lastError != "" && !isNormalMinerStopReason(lastError) && !isHistoricalMinerRetryReason(lastError) && !isTransientMinerRecoveryReason(lastError) {
		return lastError
	}
	return ""
}

func categorizeMinerPauseState(input MinerRuntimeInput, reason string) string {
	lower := strings.ToLower(reason)
	switch {
	case strings.Contains(lower, "rpc"):
		return MinerStatePausedRPCTimeout
	case strings.Contains(lower, "destination") || strings.Contains(lower, "payout") || strings.Contains(lower, "address") || strings.Contains(lower, "owned"):
		return MinerStatePausedPayoutInvalid
	case strings.Contains(lower, "template") || strings.Contains(lower, "stale"):
		return MinerStatePausedHardStaleTemplate
	case strings.Contains(lower, "peer") || strings.Contains(lower, "no peers"):
		return MinerStatePausedPeerUnsafe
	case strings.Contains(lower, "sync") || strings.Contains(lower, "behind"):
		return MinerStatePausedSyncUnsafe
	case strings.Contains(lower, "error") || strings.Contains(lower, "failed"):
		return MinerStateError
	default:
		return MinerStatePausedUnsafe
	}
}

func minerStateCountsAsActive(state string) bool {
	switch state {
	case MinerStateRunning, MinerStateSoftRefreshingStillMining:
		return true
	default:
		return false
	}
}

func isTransientMinerRecoveryReason(s string) bool {
	normalized := strings.ToLower(strings.TrimSpace(s))
	return strings.Contains(normalized, MinerStopSupervisorCancelled) ||
		strings.Contains(normalized, "restarting workers") ||
		strings.Contains(normalized, "recovering worker epoch") ||
		strings.Contains(normalized, "soft_reconcile")
}
