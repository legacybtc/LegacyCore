package ai

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// MockProvider returns deterministic responses based on the sanitized snapshot.
// It requires no model download and is safe for CI/testing.
type MockProvider struct {
	started     int32
	modelLoaded int32
	modelName   string
}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Start(_ context.Context, _ AIConfig) error {
	atomic.StoreInt32(&m.started, 1)
	return nil
}

func (m *MockProvider) Stop(_ context.Context) error {
	atomic.StoreInt32(&m.started, 0)
	atomic.StoreInt32(&m.modelLoaded, 0)
	m.modelName = ""
	return nil
}

func (m *MockProvider) Health(_ context.Context) (AIHealth, error) {
	status := StatusStopped
	if atomic.LoadInt32(&m.started) == 1 {
		status = StatusReady
		if atomic.LoadInt32(&m.modelLoaded) == 1 {
			status = StatusReady
		}
	}
	return AIHealth{
		Status:      status,
		ModelLoaded: atomic.LoadInt32(&m.modelLoaded) == 1,
		ModelName:   m.modelName,
		Backend:     "mock",
		RAMMB:       0,
	}, nil
}

func (m *MockProvider) ListModels(_ context.Context) ([]AIModel, error) {
	return []AIModel{
		{Name: "mock-model-1b", Path: "mock://model-1b", FileSizeMB: 100, Quantization: "Q4_K_M", License: "Mock"},
	}, nil
}

func (m *MockProvider) LoadModel(_ context.Context, model string) error {
	atomic.StoreInt32(&m.modelLoaded, 1)
	m.modelName = model
	return nil
}

func (m *MockProvider) UnloadModel(_ context.Context) error {
	atomic.StoreInt32(&m.modelLoaded, 0)
	m.modelName = ""
	return nil
}

func (m *MockProvider) Chat(_ context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 1)
	go func() {
		defer close(ch)
		response := generateMockResponse(req)
		for i, word := range strings.Fields(response) {
			select {
			case ch <- ChatEvent{Type: "token", Content: word + " "}:
				time.Sleep(10 * time.Millisecond)
			case <-time.After(time.Second):
				ch <- ChatEvent{Type: "error", Error: "mock timeout"}
				return
			}
			_ = i
		}
		ch <- ChatEvent{Type: "done", Content: response, Tokens: len(strings.Fields(response))}
	}()
	return ch, nil
}

func generateMockResponse(req ChatRequest) string {
	msg := strings.ToLower(req.Message)
	s := req.Snapshot

	switch {
	case strings.Contains(msg, "miner") && strings.Contains(msg, "pause") || strings.Contains(msg, "miner") && strings.Contains(msg, "stop"):
		if s.MiningSafe && s.MinerState == "running" {
			return "Your miner is running safely with " + fmt.Sprintf("%d", s.ActiveThreads) + " active threads at " + s.LocalHashrate + " KH/s."
		}
		if s.MiningPaused != "" {
			return "Your miner is paused: " + s.MiningPaused + ". It will resume automatically when safe."
		}
		return "Your miner is currently stopped. You can start it from the Mining tab."

	case strings.Contains(msg, "sync") || strings.Contains(msg, "synchronize"):
		if s.SyncState == "current" {
			return fmt.Sprintf("Your node is fully synchronized at height %d with %d peers.", s.Height, s.PeerCount)
		}
		return fmt.Sprintf("Your node is still syncing: state=%s, blocks behind=%d.", s.SyncState, s.BlocksBehind)

	case strings.Contains(msg, "peer") || strings.Contains(msg, "connection"):
		return fmt.Sprintf("You have %d connected peers (%d are good, %d agree on current chain). DNS seeds: %d.", s.PeerCount, s.GoodPeerCount, s.AgreeingPeers, s.DNSSEEDs)

	case strings.Contains(msg, "immature") || strings.Contains(msg, "reward"):
		return fmt.Sprintf("Your wallet has %s LBTC in immature mining rewards. These require 100 confirmations before becoming spendable.", s.ImmatureLBTC)

	case strings.Contains(msg, "rpc") && (strings.Contains(msg, "health") || strings.Contains(msg, "timeout")):
		return fmt.Sprintf("RPC health: %s. Error count: %d, timeout count: %d.", s.RPCHealth, s.RPCErrorCount, s.RPCTimeoutCount)

	case strings.Contains(msg, "storage") || strings.Contains(msg, "disk"):
		if s.StorageOK {
			return "Storage health is OK. No disk issues detected."
		}
		return "Storage health check failed: " + s.StorageError

	case strings.Contains(msg, "balance") || strings.Contains(msg, "lbtc"):
		return fmt.Sprintf("Your wallet has %s LBTC available out of %s LBTC total. Immature rewards: %s LBTC.", s.AvailableLBTC, s.TotalLBTC, s.ImmatureLBTC)

	case strings.Contains(msg, "gpu") || strings.Contains(msg, "hardware"):
		if s.GPUName != "" {
			return fmt.Sprintf("Detected GPU: %s (%d MB VRAM). Backend: %s.", s.GPUName, s.VRAMMB, s.Backend)
		}
		return "No GPU detected. AI inference is using CPU fallback."

	case strings.Contains(msg, "safe") || strings.Contains(msg, "danger") || strings.Contains(msg, "warning"):
		issues := []string{}
		if !s.MiningSafe {
			issues = append(issues, "mining is not safe")
		}
		if s.SyncState != "current" && s.SyncState != "unknown" {
			issues = append(issues, "node is not fully synced")
		}
		if !s.TemplateFresh {
			issues = append(issues, "mining template is stale")
		}
		if !s.StorageOK {
			issues = append(issues, "storage health check failed")
		}
		if s.RPCHealth == "timeout" || s.RPCHealth == "offline" {
			issues = append(issues, "RPC is not healthy")
		}
		if len(issues) == 0 {
			return "All systems are healthy. Mining safety is OK, node is synced, storage is healthy, and RPC is responsive."
		}
		return "Issues detected: " + strings.Join(issues, "; ") + "."

	default:
		return fmt.Sprintf("I am Legacy AI, a read-only local assistant. Your node is at height %d, %s, with %d peers. Mining: %s (%s). Ask me about sync status, peers, mining safety, balances, or storage health.", s.Height, s.SyncState, s.PeerCount, s.MinerState, s.RPCHealth)
	}
}
