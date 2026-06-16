package ai

import (
	"context"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestSanitizedSnapshotNeverContainsSecrets(t *testing.T) {
	raw := map[string]any{
		"mining": map[string]any{
			"mining_reward_address": "LPHZfJgRXqdpJdFMbbJSb8ZR4MWgen6Laq",
			"mining_pubkey_hash":    "85f774538db4b5243fe64121bbfe53bc83441e0e",
		},
		"wallet": map[string]any{
			"seed":                  "never-expose-this",
			"private_key_wif":       "L1secretwifkey",
			"walletpassphrase":      "password123",
			"rpc_user":              "legacyrpc",
			"rpc_password":          "secret",
		},
		"node": map[string]any{
			"rpc_username": "admin",
			"rpc_password": "secret",
		},
	}
	s := BuildSanitizedSnapshot(raw)

	jsonStr := mustJSON(t, s)
	forbidden := []string{
		"LPHZfJgRXqdpJdFMbbJSb8ZR4MWgen6Laq",
		"85f774538db4b5243fe64121bbfe53bc83441e0e",
		"never-expose-this",
		"L1secretwifkey",
		"password123",
		"legacyrpc",
		"admin",
	}
	for _, secret := range forbidden {
		if strings.Contains(strings.ToLower(jsonStr), strings.ToLower(secret)) {
			t.Fatalf("snapshot leaked secret: %q", secret)
		}
	}
}

func TestSanitizedSnapshotOnlyContainsAllowedFields(t *testing.T) {
	raw := map[string]any{
		"mining": map[string]any{
			"miner_state":  "running",
			"mining_safe":  true,
			"local_khps":   "1.234",
			"active_threads": float64(4),
			"configured_threads": float64(4),
		},
		"sync": map[string]any{
			"sync_state":     "current",
			"blocks_behind":  float64(0),
		},
		"blockchain": map[string]any{
			"height":     float64(3500),
			"peer_count": float64(3),
		},
		"node": map[string]any{
			"running": true,
		},
	}
	s := BuildSanitizedSnapshot(raw)

	if s.MinerState != "running" {
		t.Fatalf("miner_state=%q want running", s.MinerState)
	}
	if !s.MiningSafe {
		t.Fatal("mining_safe should be true")
	}
	if s.LocalHashrate != "1.234" {
		t.Fatalf("local_hashrate=%q", s.LocalHashrate)
	}
	if s.Height != 3500 {
		t.Fatalf("height=%d", s.Height)
	}
}

func TestMockProviderStartStop(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()

	if err := m.Start(ctx, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	h, err := m.Health(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != StatusReady {
		t.Fatalf("status=%q want ready", h.Status)
	}

	if err := m.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	h, _ = m.Health(ctx)
	if h.Status != StatusStopped {
		t.Fatalf("status=%q want stopped", h.Status)
	}
}

func TestMockProviderLoadUnloadModel(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()
	_ = m.Start(ctx, DefaultConfig())

	if err := m.LoadModel(ctx, "mock-model-1b"); err != nil {
		t.Fatal(err)
	}
	h, _ := m.Health(ctx)
	if !h.ModelLoaded {
		t.Fatal("model should be loaded")
	}

	if err := m.UnloadModel(ctx); err != nil {
		t.Fatal(err)
	}
	h, _ = m.Health(ctx)
	if h.ModelLoaded {
		t.Fatal("model should be unloaded")
	}
}

func TestMockProviderChat(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()
	_ = m.Start(ctx, DefaultConfig())
	_ = m.LoadModel(ctx, "mock-model-1b")

	snap := SanitizedSnapshot{
		MinerState: "running", MiningSafe: true, ActiveThreads: 4,
		LocalHashrate: "0.871", PeerCount: 3, GoodPeerCount: 3, AgreeingPeers: 3,
		Height: 3500, SyncState: "current",
		AvailableLBTC: "100.0", TotalLBTC: "150.0", ImmatureLBTC: "50.0",
		RPCHealth: "ok", StorageOK: true, TemplateFresh: true,
	}

	tests := []struct {
		msg      string
		contains string
	}{
		{"why is my miner paused?", "miner"},
		{"is my node synchronized?", "synchron"},
		{"explain immature rewards", "immature"},
		{"how many peers?", "3 connect"},
		{"is my RPC healthy?", "ok"},
		{"check my balance", "100.0"},
		{"what GPU do I have?", "No GPU"},
		{"any safety warnings?", "healthy"},
	}
	for _, tc := range tests {
		ch, err := m.Chat(ctx, ChatRequest{Message: tc.msg, Snapshot: snap})
		if err != nil {
			t.Fatalf("chat(%q): %v", tc.msg, err)
		}
		var response strings.Builder
		for evt := range ch {
			if evt.Type == "error" {
				t.Fatalf("chat(%q) error: %s", tc.msg, evt.Error)
			}
			response.WriteString(evt.Content)
		}
		if !strings.Contains(strings.ToLower(response.String()), strings.ToLower(tc.contains)) {
			t.Fatalf("chat(%q) response %q missing %q", tc.msg, response.String(), tc.contains)
		}
	}
}

func TestLifecycleManagerStartStop(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx := context.Background()

	if err := lm.Start(ctx, DefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if !lm.IsRunning() {
		t.Fatal("should be running")
	}

	h, _ := lm.Health(ctx)
	if h.Status != StatusReady {
		t.Fatalf("status=%q", h.Status)
	}

	if err := lm.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if lm.IsRunning() {
		t.Fatal("should be stopped")
	}
}

func TestLifecycleManagerDoubleStartRejected(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx := context.Background()
	_ = lm.Start(ctx, DefaultConfig())

	if err := lm.Start(ctx, DefaultConfig()); err == nil {
		t.Fatal("double start should be rejected")
	}
}

func TestLifecycleManagerRestart(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := lm.Start(ctx, DefaultConfig()); err != nil {
			t.Fatal(err)
		}
		if err := lm.Stop(ctx); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLifecycleManagerChatRequiresRunning(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx := context.Background()

	_, err := lm.Chat(ctx, ChatRequest{Message: "hello", Snapshot: SanitizedSnapshot{}})
	if err == nil {
		t.Fatal("chat should fail when not running")
	}
}

func TestLifecycleManagerNoGoroutineLeak(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx, cancel := context.WithCancel(context.Background())

	_ = lm.Start(ctx, DefaultConfig())
	_ = lm.LoadModel(ctx, "mock-model-1b")

	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		ch, _ := lm.Chat(ctx, ChatRequest{Message: "hello", Snapshot: SanitizedSnapshot{}})
		for range ch {
		}
	}
	time.Sleep(100 * time.Millisecond)
	after := runtime.NumGoroutine()

	if after > before+5 {
		t.Fatalf("goroutine leak: before=%d after=%d", before, after)
	}
	cancel()
}

func TestLifecycleManagerStopClosesChat(t *testing.T) {
	m := NewMockProvider()
	lm := NewLifecycleManager(m, nil)
	ctx := context.Background()

	_ = lm.Start(ctx, DefaultConfig())
	_ = lm.LoadModel(ctx, "mock-model-1b")

	if err := lm.Stop(ctx); err != nil {
		t.Fatal(err)
	}

	_, err := lm.Chat(ctx, ChatRequest{Message: "hello", Snapshot: SanitizedSnapshot{}})
	if err == nil {
		t.Fatal("chat should fail after stop")
	}
}

func mustJSON(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
