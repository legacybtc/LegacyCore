package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func safeMinerRuntimeInput() MinerRuntimeInput {
	return MinerRuntimeInput{
		SessionActive:         true,
		WorkersHashing:        true,
		ConfiguredThreads:     1,
		HashAttempts:          100,
		LastNonce:             99,
		LocalHashPS:           10,
		WorkerEpochAgeSeconds: 30,
		SafetySafe:            true,
		RPCHealth:             "ok",
		DataFresh:             true,
		SyncState:             "current",
		BlocksBehind:          0,
		BlocksBehindAllowed:   1,
		GoodPeerCount:         3,
		MinGoodPeers:          3,
		DestinationOK:         true,
		HasActiveTemplate:     true,
		TemplateFresh:         true,
		TemplateRefreshDue:    false,
		TemplateStaleReason:   "",
		TemplateRefreshError:  "",
	}
}

func TestResolveMinerRuntimeStateRunningWhenSafeAndFresh(t *testing.T) {
	state := ResolveMinerRuntimeState(safeMinerRuntimeInput())
	if state.State != MinerStateRunning {
		t.Fatalf("state = %q, want %q", state.State, MinerStateRunning)
	}
	if state.ActiveThreads != 1 || !state.LiveHashing {
		t.Fatalf("expected one live worker, got %+v", state)
	}
	if state.Reason != "" {
		t.Fatalf("safe running miner should not have pause reason: %q", state.Reason)
	}
}

func TestResolveMinerRuntimeStateResumesWorkersWhenSafeWithoutBlocker(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.WorkersHashing = false
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateRunning {
		t.Fatalf("state = %q, want %q", state.State, MinerStateRunning)
	}
	if state.ActiveThreads != 1 || !state.ShouldHaveWorkers {
		t.Fatalf("expected configured workers to be resumed, got %+v", state)
	}
	if state.SupervisorAction != "resume_workers" {
		t.Fatalf("supervisor action = %q, want resume_workers", state.SupervisorAction)
	}
	if state.Reason != "" {
		t.Fatalf("safe resume path should not invent blocker reason: %q", state.Reason)
	}
	if state.InvariantViolation == "" {
		t.Fatalf("expected invariant diagnostic for safe session with no live workers")
	}
}

func TestResolveMinerRuntimeStateSoftRefreshStillMining(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.TemplateRefreshDue = true
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateSoftRefreshingStillMining {
		t.Fatalf("state = %q, want %q", state.State, MinerStateSoftRefreshingStillMining)
	}
	if state.ActiveThreads != 1 || !state.LiveHashing {
		t.Fatalf("soft refresh should keep workers live, got %+v", state)
	}
	if state.Reason == "" {
		t.Fatalf("expected non-blocking refresh reason")
	}
}

func TestResolveMinerRuntimeStateActiveWorkerRequiresHashProgress(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.WorkersHashing = true
	input.HashAttempts = 0
	input.LastNonce = 0
	input.LocalHashPS = 0
	input.WorkerEpochAgeSeconds = 30
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateWorkerStalled {
		t.Fatalf("active worker with no hash progress must not render healthy, got %+v", state)
	}
	if !strings.Contains(state.Reason, "worker_not_hashing") {
		t.Fatalf("expected worker_not_hashing reason, got %+v", state)
	}
	if state.ActiveThreads != 0 || state.LiveHashing {
		t.Fatalf("zero-progress worker must not count as active live hashing, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateStartupGraceDoesNotFakeActiveThreads(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.HashAttempts = 0
	input.LastNonce = 0
	input.LocalHashPS = 0
	input.WorkerEpochAgeSeconds = 2
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateStarting {
		t.Fatalf("startup grace should be starting, got %+v", state)
	}
	if state.ActiveThreads != 0 || state.LiveHashing {
		t.Fatalf("startup grace without progress must not count active threads, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateHardStalePausesWorkers(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.TemplateFresh = false
	input.TemplateStaleReason = "template prev hash does not match current tip"
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStatePausedHardStaleTemplate {
		t.Fatalf("state = %q, want %q", state.State, MinerStatePausedHardStaleTemplate)
	}
	if state.ActiveThreads != 0 || state.LiveHashing {
		t.Fatalf("hard stale template must stop live workers, got %+v", state)
	}
	if state.Reason == "" {
		t.Fatalf("expected hard-stale blocker reason")
	}
}

func TestResolveMinerRuntimeStateTemplateRefreshFailureIsActionable(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.WorkersHashing = false
	input.TemplateFresh = false
	input.TemplateRefreshDue = true
	input.TemplateStaleReason = "template prev hash does not match current tip"
	input.TemplateRefreshError = "template_refresh_failed: recovery timeout waiting for fresh block template"
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStatePausedHardStaleTemplate {
		t.Fatalf("template refresh failure should remain a template recovery state, got %+v", state)
	}
	if strings.Contains(strings.ToLower(state.Reason), "internal") {
		t.Fatalf("template recovery failure must not become generic internal_error: %+v", state)
	}
}

func TestResolveMinerRuntimeStateUnsafePeersPauseAndRecovery(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.SafetySafe = false
	input.SafetyReason = "Mining paused: fewer than 2 current agreeing peer(s)."
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStatePausedPeerUnsafe {
		t.Fatalf("state = %q, want %q", state.State, MinerStatePausedPeerUnsafe)
	}
	if state.ActiveThreads != 0 || state.Reason == "" {
		t.Fatalf("unsafe peers should pause with reason, got %+v", state)
	}

	input.SafetySafe = true
	input.SafetyReason = ""
	state = ResolveMinerRuntimeState(input)
	if state.State != MinerStateRunning || state.ActiveThreads != 1 {
		t.Fatalf("peer recovery should resume running state, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateRPCTimeoutPauses(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.RPCHealth = "timeout"
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStatePausedRPCTimeout {
		t.Fatalf("state = %q, want %q", state.State, MinerStatePausedRPCTimeout)
	}
	if state.ActiveThreads != 0 || state.LiveHashing {
		t.Fatalf("rpc timeout must not expose live hashing, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateTransientSupervisorCancelResumesWorkers(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.WorkersHashing = false
	input.LastError = MinerStopSupervisorCancelled + ": mining worker epoch cancelled; restarting workers."
	input.PausedReason = "Mining worker epoch cancelled unexpectedly; restarting workers."
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateRunning {
		t.Fatalf("transient supervisor cancellation should reconcile to running, got %+v", state)
	}
	if state.ActiveThreads != 1 || !state.ShouldHaveWorkers || state.SupervisorAction != "resume_workers" {
		t.Fatalf("expected worker resume action, got %+v", state)
	}
	if state.Reason != "" {
		t.Fatalf("transient recovery should not be a stable blocker: %q", state.Reason)
	}
}

func TestResolveMinerRuntimeStateUnexpectedExitIsErrorNotCleanStopped(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.SessionActive = false
	input.WorkersHashing = false
	input.EverStarted = true
	input.LastStopReason = MinerStopWorkerExitUnexpected
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateError {
		t.Fatalf("state = %q, want %q", state.State, MinerStateError)
	}
	if state.Reason == "" {
		t.Fatalf("unexpected worker exit must have visible reason")
	}
}

func TestResolveMinerRuntimeStateStoppedAfterStartRequiresReason(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.SessionActive = false
	input.WorkersHashing = false
	input.EverStarted = true
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateError {
		t.Fatalf("stopped after start with no reason must be error, got %+v", state)
	}

	input.LastStopReason = MinerStopUserStop
	state = ResolveMinerRuntimeState(input)
	if state.State != MinerStateStopped {
		t.Fatalf("user stop should render stopped, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateRejectsFalseNodeShutdownWhileRPCOnline(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.SessionActive = false
	input.WorkersHashing = false
	input.EverStarted = true
	input.LastStopReason = MinerStopNodeShutdown
	input.RPCHealth = "ok"
	input.DataFresh = true
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateError {
		t.Fatalf("false node_shutdown while RPC online should be error, got %+v", state)
	}
	if state.Reason == "" || state.Reason == "miner is stopped" {
		t.Fatalf("false node_shutdown must have visible invariant reason, got %+v", state)
	}
}

func TestResolveMinerRuntimeStateAllowsTrueNodeShutdownWhenOffline(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.SessionActive = false
	input.WorkersHashing = false
	input.EverStarted = true
	input.LastStopReason = MinerStopNodeShutdown
	input.RPCHealth = "offline"
	input.DataFresh = false
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStateStopped {
		t.Fatalf("true node_shutdown should render stopped, got %+v", state)
	}
}

func TestClassifyMinerContextCancellationDoesNotUseNodeShutdownForInnerCancel(t *testing.T) {
	reason, shouldExit := classifyMinerContextCancellation(context.Canceled, context.Background())
	if reason != MinerStopSupervisorCancelled || shouldExit {
		t.Fatalf("inner miner cancellation classified as reason=%q shouldExit=%t", reason, shouldExit)
	}
}

func TestClassifyMinerContextCancellationUsesNodeShutdownForOuterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reason, shouldExit := classifyMinerContextCancellation(errors.New("worker failed"), ctx)
	if reason != MinerStopNodeShutdown || !shouldExit {
		t.Fatalf("outer shutdown classified as reason=%q shouldExit=%t", reason, shouldExit)
	}
}

func TestStopMinerRecordsMachineStopReason(t *testing.T) {
	s := &Server{configPath: t.TempDir() + "/legacycoin.conf"}
	s.minerActive = true
	s.minerStartedAt = time.Now()
	out := s.stopMiner(MinerStopUserStop)
	if out["last_stop_reason"] != MinerStopUserStop {
		t.Fatalf("last_stop_reason = %v, want %s", out["last_stop_reason"], MinerStopUserStop)
	}
	if s.minerLastStopReason != MinerStopUserStop {
		t.Fatalf("server last stop reason = %q", s.minerLastStopReason)
	}
}

func TestParseMinerStopReasonAllowsForceStop(t *testing.T) {
	reason := parseMinerStopReason([]byte(`[{"reason":"user_force_stop"}]`), MinerStopRPCStopMiner)
	if reason != MinerStopUserForceStop {
		t.Fatalf("reason = %q, want %q", reason, MinerStopUserForceStop)
	}
}

func TestParseMinerStopReasonTreatsSmokeCleanupAsUserStop(t *testing.T) {
	reason := parseMinerStopReason([]byte(`[{"reason":"smoke_test_complete"}]`), MinerStopRPCStopMiner)
	if reason != MinerStopUserStop {
		t.Fatalf("reason = %q, want %q", reason, MinerStopUserStop)
	}
}

func TestStaleTemplateSafetyBlockTriggersRefreshPath(t *testing.T) {
	status := MiningSafetyStatus{
		Safe:                        false,
		HasActiveTemplate:           true,
		ActiveTemplateFresh:         false,
		ActiveTemplateStaleReason:   "template prev hash does not match current tip",
		ActiveTemplatePrevHash:      "old-tip",
		CurrentTipHash:              "new-tip",
		CurrentTemplateHeight:       3205,
		CurrentTipHeight:            3206,
		ActiveTemplateRefreshDue:    true,
		ActiveTemplateRefreshReason: "prev_hash_mismatch: template prev hash does not match current tip",
	}
	if !staleTemplateSafetyBlock(status) {
		t.Fatalf("stale template safety block should enter refresh path")
	}
	status = MiningSafetyStatus{Safe: false, HasActiveTemplate: false, ActiveTemplateFresh: false}
	if staleTemplateSafetyBlock(status) {
		t.Fatalf("non-template safety block must not bypass normal pause path")
	}
}

func TestMarkStaleTemplateRefreshLockedSetsDueAndSkip(t *testing.T) {
	s := &Server{}
	s.minerHashing = true
	s.minerLocalHashPS = 42
	s.markStaleTemplateRefreshLocked("template prev hash does not match current tip", true)
	if s.minerHashing || s.minerLocalHashPS != 0 {
		t.Fatalf("stale template refresh must stop old-template hashing")
	}
	if !s.minerLastTemplateRefreshDue {
		t.Fatalf("stale template must mark refresh due")
	}
	if s.minerLastTemplateRefreshReason != "prev_hash_mismatch: template prev hash does not match current tip" {
		t.Fatalf("refresh reason = %q", s.minerLastTemplateRefreshReason)
	}
	if s.minerStaleTemplateSkips != 1 || s.minerStaleTemplateRefreshAttempts != 1 {
		t.Fatalf("expected skip and refresh attempt counts, got skips=%d attempts=%d", s.minerStaleTemplateSkips, s.minerStaleTemplateRefreshAttempts)
	}
	if s.minerLastTemplateRefreshAttempt.IsZero() {
		t.Fatalf("expected refresh attempt timestamp")
	}
	if !s.minerTemplateRecoveryPending || s.minerTemplateRecoveryStartedAt.IsZero() {
		t.Fatalf("expected template recovery pending with start time")
	}
}

func TestMarkStaleTemplateRefreshLockedNamesRecoveryTimeout(t *testing.T) {
	s := &Server{minerTemplateRecoveryStartedAt: time.Now().Add(-miningTemplateRecoveryTimeout() - time.Second)}
	s.markStaleTemplateRefreshLocked("template prev hash does not match current tip", true)
	if s.minerLastTemplateRefreshReason != "template_refresh_failed" {
		t.Fatalf("refresh reason = %q, want template_refresh_failed", s.minerLastTemplateRefreshReason)
	}
	if !strings.Contains(s.minerLastTemplateRefreshError, "recovery timeout") {
		t.Fatalf("expected recovery timeout error, got %q", s.minerLastTemplateRefreshError)
	}
}

func TestClearValidTemplateStateDropsStaleUnavailableReason(t *testing.T) {
	templateAt := time.Now()
	s := &Server{
		minerLastTemplateTime:          templateAt,
		minerLastTemplateHeight:        3229,
		minerLastTemplatePrevHash:      "tip",
		minerLastTemplateFresh:         false,
		minerLastTemplateStaleReason:   "template unavailable",
		minerLastTemplateRefreshDue:    true,
		minerLastTemplateRefreshReason: "template_stale: template unavailable",
		minerLastTemplateRefreshError:  "previous getblocktemplate timeout",
		minerTemplateRecoveryPending:   true,
		minerTemplateRecoveryStartedAt: templateAt.Add(-5 * time.Second),
	}
	s.clearValidTemplateStateIfCurrent(3229, "tip", templateAt)
	if !s.minerLastTemplateFresh {
		t.Fatalf("valid current template should be fresh")
	}
	if s.minerLastTemplateRefreshDue || s.minerLastTemplateStaleReason != "" || s.minerLastTemplateRefreshReason != "" || s.minerLastTemplateRefreshError != "" {
		t.Fatalf("valid current template must clear stale/unavailable state: due=%t stale=%q reason=%q error=%q", s.minerLastTemplateRefreshDue, s.minerLastTemplateStaleReason, s.minerLastTemplateRefreshReason, s.minerLastTemplateRefreshError)
	}
	if s.minerTemplateRecoveryPending || !s.minerTemplateRecoveryStartedAt.IsZero() {
		t.Fatalf("valid current template must clear recovery state")
	}
}

func TestAcceptedBlockMarksTemplateRefreshDue(t *testing.T) {
	s := &Server{}
	s.minerLastTemplateFresh = true
	s.minerLastTemplateRefreshDue = false
	s.markAcceptedBlockTemplateRefreshLocked()
	if s.minerLastTemplateFresh {
		t.Fatalf("accepted block should invalidate previous template freshness")
	}
	if !s.minerLastTemplateRefreshDue {
		t.Fatalf("accepted block should mark template refresh due")
	}
	if s.minerLastTemplateRefreshReason != "accepted_block_refresh: accepted block connected; refreshing template for new tip" {
		t.Fatalf("refresh reason = %q", s.minerLastTemplateRefreshReason)
	}
	if s.minerStaleTemplateRefreshAttempts != 1 {
		t.Fatalf("accepted block should count a refresh attempt, got %d", s.minerStaleTemplateRefreshAttempts)
	}
	if !s.minerTemplateRecoveryPending || s.minerTemplateRecoveryStartedAt.IsZero() {
		t.Fatalf("accepted block should start template recovery")
	}
}
