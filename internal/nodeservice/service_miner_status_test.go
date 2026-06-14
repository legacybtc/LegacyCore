package nodeservice

import (
	"context"
	"testing"
)

func TestGetMinerStatusFallsBackWhenRPCOffline(t *testing.T) {
	s := New(t.TempDir())
	s.minerMu.Lock()
	s.minerEnabled = true
	s.minerLoopRunning = true
	s.minerThreads = 12
	s.minerLocalHashPS = 1000
	s.minerSessionHashes = 1234
	s.minerLastNonce = 42
	s.minerMu.Unlock()

	status, err := s.GetMinerStatus()
	if err != nil {
		t.Fatalf("GetMinerStatus returned error: %v", err)
	}
	if status["status_source"] != "local_fallback" {
		t.Fatalf("expected local_fallback status source, got %v", status["status_source"])
	}
	if status["rpc_offline"] != true {
		t.Fatalf("expected rpc_offline true, got %v", status["rpc_offline"])
	}
	if status["active_mining"] != true {
		t.Fatalf("expected active_mining true (matches local state) while RPC is offline but miner is running locally, got %v", status["active_mining"])
	}
	if status["last_known_active_mining"] != true {
		t.Fatalf("expected last_known_active_mining true in fallback, got %v", status["last_known_active_mining"])
	}
	if status["mining_safe"] != false || status["safe_to_mine"] != false {
		t.Fatalf("fallback status must be unsafe/unknown, got mining_safe=%v safe_to_mine=%v", status["mining_safe"], status["safe_to_mine"])
	}
	if status["dashboard_data_fresh"] != false {
		t.Fatalf("fallback status must be marked stale, got %v", status["dashboard_data_fresh"])
	}
	if status["configured_threads"] != 12 || status["configured_threads_last_known"] != 12 {
		t.Fatalf("expected configured threads to remain 12 last-known, got configured=%v last_known=%v", status["configured_threads"], status["configured_threads_last_known"])
	}
	if status["active_threads"] != 0 || status["active_threads_last_known"] != 12 {
		t.Fatalf("expected live active threads 0 and last-known active threads 12, got active=%v last_known=%v", status["active_threads"], status["active_threads_last_known"])
	}
	if status["mining_blocked_reason"] != "Mining blocked: RPC is not responding." {
		t.Fatalf("unexpected mining blocked reason: %v", status["mining_blocked_reason"])
	}
	if status["miner_state"] != "paused_rpc_timeout" || status["miner_state_reason"] != "Mining blocked: RPC is not responding." {
		t.Fatalf("expected paused_rpc_timeout fallback state, got state=%v reason=%v", status["miner_state"], status["miner_state_reason"])
	}
	if status["rpc_health"] != "offline" && status["rpc_health"] != "timeout" {
		t.Fatalf("expected rpc_health offline/timeout, got %v", status["rpc_health"])
	}
}

func TestStopMinerFallsBackWhenRPCOffline(t *testing.T) {
	s := New(t.TempDir())
	_, cancel := context.WithCancel(context.Background())
	s.minerMu.Lock()
	s.minerActive = true
	s.minerEnabled = true
	s.minerLoopRunning = true
	s.minerThreads = 4
	s.minerCancel = cancel
	s.minerMu.Unlock()

	out, err := s.StopMiner()
	if err != nil {
		t.Fatalf("StopMiner returned error: %v", err)
	}
	if out["status_source"] != "local_fallback" {
		t.Fatalf("expected local_fallback status source, got %v", out["status_source"])
	}
	if out["active_mining"] != false {
		t.Fatalf("expected active_mining false, got %v", out["active_mining"])
	}
	if out["last_stop_reason"] != "user_stop" {
		t.Fatalf("expected user_stop reason, got %v", out["last_stop_reason"])
	}
}

func TestForceStopMinerFallsBackWithForceReasonWhenRPCOffline(t *testing.T) {
	s := New(t.TempDir())
	_, cancel := context.WithCancel(context.Background())
	s.minerMu.Lock()
	s.minerActive = true
	s.minerEnabled = true
	s.minerLoopRunning = true
	s.minerThreads = 4
	s.minerCancel = cancel
	s.minerMu.Unlock()

	out, err := s.ForceStopMiner()
	if err != nil {
		t.Fatalf("ForceStopMiner returned error: %v", err)
	}
	if out["active_mining"] != false {
		t.Fatalf("expected active_mining false, got %v", out["active_mining"])
	}
	if out["last_stop_reason"] != "user_force_stop" {
		t.Fatalf("expected user_force_stop reason, got %v", out["last_stop_reason"])
	}
}

func TestRecordMinerStatusSuccessPreservesLastKnownConfig(t *testing.T) {
	s := New(t.TempDir())
	status := map[string]any{
		"active_mining":      true,
		"mining_enabled":     true,
		"configured_threads": float64(12),
		"active_threads":     float64(12),
		"local_hashps":       float64(871),
		"session_hashes":     float64(12345),
		"last_nonce":         float64(99),
	}
	s.recordMinerStatusSuccess(status)
	out := map[string]any{}
	s.addMinerStatusDiagnostics(out, true)
	if out["configured_threads_last_known"] != 12 {
		t.Fatalf("expected configured_threads_last_known 12, got %v", out["configured_threads_last_known"])
	}
	if out["active_threads_last_known"] != 12 {
		t.Fatalf("expected active_threads_last_known 12, got %v", out["active_threads_last_known"])
	}
	if out["status_data_fresh"] != true {
		t.Fatalf("expected status_data_fresh true, got %v", out["status_data_fresh"])
	}
	if out["last_miner_status_success_time"] == int64(0) {
		t.Fatalf("expected last miner status success timestamp")
	}
}

func TestNormalizeMinerStatusTreatsStopAsAction(t *testing.T) {
	status := map[string]any{
		"active_mining":      false,
		"mining_enabled":     false,
		"active_threads":     12,
		"local_hashps":       973.8,
		"local_khps":         0.9738,
		"last_error":         "rpc stopminer",
		"configured_threads": 12,
	}
	normalizeMinerStatusForDashboard(status)
	if status["last_error"] != "" {
		t.Fatalf("expected last_error to be cleared, got %v", status["last_error"])
	}
	if status["last_action"] != "stopped by user/RPC" {
		t.Fatalf("expected normal stop action, got %v", status["last_action"])
	}
	if status["active_threads"] != 0 {
		t.Fatalf("expected active_threads to be zero while stopped, got %v", status["active_threads"])
	}
	if status["last_session_active_threads"] != 12 {
		t.Fatalf("expected last session threads to be preserved, got %v", status["last_session_active_threads"])
	}
	if status["local_khps_live"] != 0.0 {
		t.Fatalf("expected live hashrate to be zero while stopped, got %v", status["local_khps_live"])
	}
	if status["local_khps"] != 0.0 {
		t.Fatalf("expected local_khps to be live-zero while stopped, got %v", status["local_khps"])
	}
}

func TestNormalizeMinerStatusPreservesAuthoritativeRunningState(t *testing.T) {
	status := map[string]any{
		"miner_state":          "running",
		"active_mining":        false,
		"mining_enabled":       true,
		"active_threads":       1,
		"configured_threads":   1,
		"local_hashps":         50.0,
		"local_khps":           0.05,
		"last_error":           "",
		"mining_paused_reason": "",
	}
	normalizeMinerStatusForDashboard(status)
	if status["active_mining"] != true {
		t.Fatalf("authoritative running state should keep dashboard active, got %v", status["active_mining"])
	}
	if status["active_threads"] != 1 {
		t.Fatalf("active threads should not be zeroed for authoritative running state, got %v", status["active_threads"])
	}
}

func TestNormalizeMinerStatusTreatsStoppedRetryAsHistorical(t *testing.T) {
	status := map[string]any{
		"active_mining":  false,
		"mining_enabled": false,
		"active_threads": 4,
		"local_hashps":   100,
		"local_khps":     0.1,
		"last_error":     "stale tip retry",
	}
	normalizeMinerStatusForDashboard(status)
	if status["last_error"] != "" {
		t.Fatalf("expected retry last_error to be cleared while stopped, got %v", status["last_error"])
	}
	if status["last_historical_event"] != "stale tip retry" {
		t.Fatalf("expected historical retry event, got %v", status["last_historical_event"])
	}
	if status["active_threads"] != 0 {
		t.Fatalf("expected active_threads to be zero while stopped, got %v", status["active_threads"])
	}
}
