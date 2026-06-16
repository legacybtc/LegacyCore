package ai

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestMockProviderLifecycle(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()
	if err := m.Start(ctx, DefaultConfig()); err != nil { t.Fatal(err) }
	h, _ := m.Health(ctx)
	if h.Status != StatusReady { t.Fatalf("status=%q", h.Status) }
	if h.PID == 0 { t.Fatal("PID should be set") }
	if err := m.Stop(ctx); err != nil { t.Fatal(err) }
	h, _ = m.Health(ctx)
	if h.Status != StatusStopped { t.Fatalf("should be stopped") }
}

func TestMockProviderChat(t *testing.T) {
	m := NewMockProvider()
	ctx := context.Background()
	_ = m.Start(ctx, DefaultConfig())
	snap := SanitizedSnapshot{Network: "LBTC mainnet", Height: 3775, SyncState: "current", PeerCount: 3, GoodPeerCount: 3, AgreeingPeers: 3, MinerState: "running", MiningSafe: true, ActiveThreads: 4, ConfiguredThreads: 4, RPCHealth: "ok", StorageOK: true}
	ch, err := m.Chat(ctx, ChatRequest{Message: "why is my miner paused?", Snapshot: snap})
	if err != nil { t.Fatal(err) }
	var resp strings.Builder
	for evt := range ch {
		if evt.Type == "error" { t.Fatal(evt.Error) }
		resp.WriteString(evt.Content)
	}
	if !strings.Contains(resp.String(), "mining") && !strings.Contains(resp.String(), "Mining") { t.Fatalf("no mining in response: %s", resp.String()) }
}

func TestLlamaProviderExternalHealth(t *testing.T) {
	t.Skip("requires running llama-server")
}

func TestSanitizedSnapshotNoSecrets(t *testing.T) {
	raw := map[string]any{"wallet": map[string]any{"seed": "secret", "private_key": "L1key", "password": "pw"}, "mining": map[string]any{"mining_reward_address": "Lxyz"}}
	s := BuildSanitizedSnapshot(raw)
	json := fmt.Sprintf("%+v", s)
	for _, forbidden := range []string{"secret", "L1key", "pw", "Lxyz"} {
		if strings.Contains(json, forbidden) { t.Fatalf("leaked: %s", forbidden) }
	}
}

func TestLifecycleManagerStartStop(t *testing.T) {
	lm := NewLifecycleManager(NewMockProvider(), nil)
	ctx := context.Background()
	if err := lm.Start(ctx, DefaultConfig()); err != nil { t.Fatal(err) }
	if !lm.IsRunning() { t.Fatal("should be running") }
	if err := lm.Stop(ctx); err != nil { t.Fatal(err) }
	if lm.IsRunning() { t.Fatal("should be stopped") }
}

func TestLifecycleManagerDoubleStart(t *testing.T) {
	lm := NewLifecycleManager(NewMockProvider(), nil)
	ctx := context.Background()
	_ = lm.Start(ctx, DefaultConfig())
	if err := lm.Start(ctx, DefaultConfig()); err == nil { t.Fatal("double start should fail") }
}

func TestLifecycleManagerChatWhenStopped(t *testing.T) {
	lm := NewLifecycleManager(NewMockProvider(), nil)
	_, err := lm.Chat(context.Background(), ChatRequest{Message: "hi"})
	if err == nil { t.Fatal("should fail when stopped") }
}

func TestLifecycleManagerRestart(t *testing.T) {
	lm := NewLifecycleManager(NewMockProvider(), nil)
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		if err := lm.Start(ctx, DefaultConfig()); err != nil { t.Fatal(err) }
		if err := lm.Stop(ctx); err != nil { t.Fatal(err) }
	}
}
