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
		t.Fatalf("expected active_mining true in fallback, got %v", status["active_mining"])
	}
	if status["mining_safe"] != false || status["safe_to_mine"] != false {
		t.Fatalf("fallback status must be unsafe/unknown, got mining_safe=%v safe_to_mine=%v", status["mining_safe"], status["safe_to_mine"])
	}
	if status["dashboard_data_fresh"] != false {
		t.Fatalf("fallback status must be marked stale, got %v", status["dashboard_data_fresh"])
	}
	if status["mining_blocked_reason"] != "Mining blocked: RPC is not responding." {
		t.Fatalf("unexpected mining blocked reason: %v", status["mining_blocked_reason"])
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
