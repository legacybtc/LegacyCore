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
