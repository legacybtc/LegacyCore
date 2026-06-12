package rpc

import (
	"context"
	"errors"
	"testing"
	"time"
)

func safeMinerRuntimeInput() MinerRuntimeInput {
	return MinerRuntimeInput{
		SessionActive:        true,
		WorkersHashing:       true,
		ConfiguredThreads:    1,
		SafetySafe:           true,
		RPCHealth:            "ok",
		DataFresh:            true,
		SyncState:            "current",
		BlocksBehind:         0,
		BlocksBehindAllowed:  1,
		GoodPeerCount:        3,
		MinGoodPeers:         3,
		DestinationOK:        true,
		HasActiveTemplate:    true,
		TemplateFresh:        true,
		TemplateRefreshDue:   false,
		TemplateStaleReason:  "",
		TemplateRefreshError: "",
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

func TestResolveMinerRuntimeStateUnsafePeersPauseAndRecovery(t *testing.T) {
	input := safeMinerRuntimeInput()
	input.GoodPeerCount = 2
	state := ResolveMinerRuntimeState(input)
	if state.State != MinerStatePausedPeerUnsafe {
		t.Fatalf("state = %q, want %q", state.State, MinerStatePausedPeerUnsafe)
	}
	if state.ActiveThreads != 0 || state.Reason == "" {
		t.Fatalf("unsafe peers should pause with reason, got %+v", state)
	}

	input.GoodPeerCount = 3
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
