package ai

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type MockProvider struct {
	started     int32
	modelLoaded int32
	modelName   string
}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Start(_ context.Context, _ AIConfig) error { atomic.StoreInt32(&m.started, 1); return nil }
func (m *MockProvider) Stop(_ context.Context) error               { atomic.StoreInt32(&m.started, 0); atomic.StoreInt32(&m.modelLoaded, 0); return nil }

func (m *MockProvider) Health(_ context.Context) (AIHealth, error) {
	s := StatusStopped
	if atomic.LoadInt32(&m.started) == 1 { s = StatusReady }
	return AIHealth{Status: s, ModelLoaded: atomic.LoadInt32(&m.modelLoaded) == 1, ModelName: m.modelName, Backend: "mock", PID: int(time.Now().UnixNano() % 100000)}, nil
}

func (m *MockProvider) ListModels(_ context.Context) ([]AIModel, error) {
	return []AIModel{{Name: "mock-model-1b", Path: "mock://model", FileSizeMB: 100, Quantization: "Q4_K_M", License: "Mock Only"}}, nil
}

func (m *MockProvider) LoadModel(_ context.Context, model string) error { atomic.StoreInt32(&m.modelLoaded, 1); m.modelName = model; return nil }
func (m *MockProvider) UnloadModel(_ context.Context) error            { atomic.StoreInt32(&m.modelLoaded, 0); return nil }

func (m *MockProvider) Chat(_ context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 1)
	go func() {
		defer close(ch)
		response := generateMockResponse(req)
		for _, word := range strings.Fields(response) {
			ch <- ChatEvent{Type: "token", Content: word + " "}
			time.Sleep(15 * time.Millisecond)
		}
		ch <- ChatEvent{Type: "done", Tokens: len(strings.Fields(response))}
	}()
	return ch, nil
}

func generateMockResponse(req ChatRequest) string {
	msg := strings.ToLower(req.Message)
	s := req.Snapshot

	if req.Mode == "developer" {
		return generateDeveloperResponse(msg, s)
	}

	switch {
	case strings.Contains(msg, "miner") && (strings.Contains(msg, "pause") || strings.Contains(msg, "stop")):
		if s.MiningSafe && s.MinerState == "running" {
			return fmt.Sprintf("Your miner is running safely with %d active threads at %s KH/s.", s.ActiveThreads, s.LocalHashrate)
		}
		if s.MiningPaused != "" {
			return fmt.Sprintf("Your miner is paused: %s. It will resume when safe.", s.MiningPaused)
		}
		return "Your miner is stopped. Start it from the Mining tab."
	case strings.Contains(msg, "sync"):
		if s.SyncState == "current" {
			return fmt.Sprintf("Your node is fully synced at height %d with %d connected peers.", s.Height, s.PeerCount)
		}
		return fmt.Sprintf("Syncing: state=%s, %d blocks behind.", s.SyncState, s.BlocksBehind)
	case strings.Contains(msg, "peer") || strings.Contains(msg, "degraded"):
		return fmt.Sprintf("You have %d peers (%d agree on current chain). Degraded-safe mining allows 1 exact + 1 lagging-1-block when no danger conditions exist.", s.PeerCount, s.AgreeingPeers)
	case strings.Contains(msg, "reward") || strings.Contains(msg, "immature"):
		return fmt.Sprintf("Your wallet has %s LBTC in immature mining rewards. These need 100 confirmations before spending.", s.ImmatureLBTC)
	case strings.Contains(msg, "rpc") || strings.Contains(msg, "health"):
		return fmt.Sprintf("RPC health: %s. Errors: %d, timeouts: %d.", s.RPCHealth, s.RPCErrorCount, s.RPCTimeoutCount)
	case strings.Contains(msg, "storage"):
		if s.StorageOK { return "Storage is healthy." }
		return fmt.Sprintf("Storage issue: %s", s.StorageError)
	case strings.Contains(msg, "balance"):
		return fmt.Sprintf("Available: %s LBTC. Total: %s LBTC. Immature: %s LBTC.", s.AvailableLBTC, s.TotalLBTC, s.ImmatureLBTC)
	case strings.Contains(msg, "gpu") || strings.Contains(msg, "backend"):
		if s.GPUName != "" {
			return fmt.Sprintf("GPU: %s (%d MB VRAM). Backend: %s.", s.GPUName, s.VRAMMB, s.Backend)
		}
		return "No GPU detected. Using CPU fallback."
	case strings.Contains(msg, "safe") || strings.Contains(msg, "warning"):
		issues := []string{}
		if !s.MiningSafe { issues = append(issues, "mining safety blocked") }
		if s.SyncState != "current" { issues = append(issues, "not fully synced") }
		if !s.StorageOK { issues = append(issues, "storage check failed") }
		if !s.TemplateFresh { issues = append(issues, "template stale") }
		if len(issues) == 0 { return "All systems healthy. No warnings." }
		return "Issues: " + strings.Join(issues, "; ") + "."
	default:
		return fmt.Sprintf("I am Legacy AI Companion. Your node is at height %d, synced=%s, with %d peers. Mining: %s. Ask me about sync, peers, mining, rewards, storage, or safety.", s.Height, s.SyncState, s.PeerCount, s.MinerState)
	}
}

func generateDeveloperResponse(msg string, s SanitizedSnapshot) string {
	switch {
	case strings.Contains(msg, "blockchain"):
		return "Developer mode: Use /legacycoin-cli getblockchaininfo to query chain state directly."
	case strings.Contains(msg, "peer"):
		return "Developer mode: Use /legacycoin-cli getpeerinfo for raw peer data, or /netstat -an | findstr 19555 for connection stats."
	case strings.Contains(msg, "mining") || strings.Contains(msg, "miner"):
		return "Developer mode: Use /legacycoin-cli getmininginfo to inspect mining state."
	case strings.Contains(msg, "mempool"):
		return "Developer mode: Use /legacycoin-cli getmempoolinfo to inspect the mempool."
	case strings.Contains(msg, "process"):
		return "Developer mode: Use /get-process legacywallet or /get-process legacycoind to check running processes."
	case strings.Contains(msg, "help") || strings.Contains(msg, "tool"):
		return "Developer mode tools: /legacycoin-cli getblockchaininfo, /legacycoin-cli getpeerinfo, /legacycoin-cli getmininginfo, /get-process legacywallet, /netstat -an | findstr 19555. Prefix commands with / in chat to execute."
	}
	return fmt.Sprintf("Developer mode active. Node at height %d, %d peers. Use /help for available tools.", s.Height, s.PeerCount)
}
