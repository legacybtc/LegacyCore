package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

const (
	MinerStopUserStop             = "user_stop"
	MinerStopUserForceStop        = "user_force_stop"
	MinerStopRPCStopMiner         = "rpc_stopminer"
	MinerStopNodeShutdown         = "node_shutdown"
	MinerStopSupervisorShutdown   = "supervisor_shutdown"
	MinerStopSupervisorCancelled  = "supervisor_context_cancelled"
	MinerStopUnsafeSync           = "unsafe_sync"
	MinerStopUnsafePeers          = "unsafe_peers"
	MinerStopRPCTimeout           = "rpc_timeout"
	MinerStopHardStaleTemplate    = "hard_stale_template"
	MinerStopPayoutInvalid        = "payout_invalid"
	MinerStopHighStaleRate        = "high_stale_rate"
	MinerStopInternalError        = "internal_error"
	MinerStopWorkerExitUnexpected = "worker_exit_unexpected"
	MinerStopNoConfiguredThreads  = "no_configured_threads"
)

func normalizeMinerStopReason(reason string) string {
	normalized := strings.ToLower(strings.TrimSpace(reason))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case MinerStopUserStop, "stopped_by_user", "user", "stopminer", "smoke_test_complete":
		return MinerStopUserStop
	case MinerStopUserForceStop, "force_stop", "force":
		return MinerStopUserForceStop
	case MinerStopRPCStopMiner, "rpc_stop":
		return MinerStopRPCStopMiner
	case "rpc_restartminer", "restartminer", "restart":
		return MinerStopSupervisorShutdown
	case MinerStopNodeShutdown, "shutdown":
		return MinerStopNodeShutdown
	case MinerStopSupervisorShutdown, "block_limit_reached", "requested_block_limit":
		return MinerStopSupervisorShutdown
	case MinerStopSupervisorCancelled:
		return MinerStopSupervisorCancelled
	case MinerStopUnsafeSync:
		return MinerStopUnsafeSync
	case MinerStopUnsafePeers, "no_peers", "peer_required_but_no_peers":
		return MinerStopUnsafePeers
	case MinerStopRPCTimeout:
		return MinerStopRPCTimeout
	case MinerStopHardStaleTemplate, "stale_template":
		return MinerStopHardStaleTemplate
	case MinerStopPayoutInvalid:
		return MinerStopPayoutInvalid
	case MinerStopHighStaleRate:
		return MinerStopHighStaleRate
	case MinerStopInternalError, "fatal_storage_error", "storage_error":
		return MinerStopInternalError
	case MinerStopWorkerExitUnexpected, "miner_loop_exited", "worker_exit":
		return MinerStopWorkerExitUnexpected
	case MinerStopNoConfiguredThreads:
		return MinerStopNoConfiguredThreads
	default:
		if normalized == "" {
			return ""
		}
		return MinerStopInternalError
	}
}

func parseMinerStopReason(params json.RawMessage, fallback string) string {
	var args []json.RawMessage
	if len(params) > 0 {
		_ = json.Unmarshal(params, &args)
	}
	if len(args) > 0 {
		var reason string
		if json.Unmarshal(args[0], &reason) == nil && strings.TrimSpace(reason) != "" {
			return normalizeMinerStopReason(reason)
		}
		var obj map[string]json.RawMessage
		if json.Unmarshal(args[0], &obj) == nil {
			for _, key := range []string{"reason", "stop_reason", "last_stop_reason"} {
				if raw, ok := obj[key]; ok && json.Unmarshal(raw, &reason) == nil && strings.TrimSpace(reason) != "" {
					return normalizeMinerStopReason(reason)
				}
			}
		}
	}
	return normalizeMinerStopReason(fallback)
}

func minerStopReasonIsUnexpected(reason string) bool {
	switch normalizeMinerStopReason(reason) {
	case MinerStopWorkerExitUnexpected, MinerStopInternalError, MinerStopSupervisorCancelled:
		return true
	default:
		return false
	}
}

func classifyMinerContextCancellation(err error, outer context.Context) (string, bool) {
	if outer != nil && outer.Err() != nil {
		return MinerStopNodeShutdown, true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return MinerStopSupervisorCancelled, false
	}
	return "", false
}
