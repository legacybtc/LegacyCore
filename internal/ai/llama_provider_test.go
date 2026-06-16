package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLlamaProviderHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(200)
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	p := NewLlamaProvider(LlamaConfig{ServerURL: ts.URL})
	h, err := p.Health(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != StatusReady {
		t.Fatalf("status=%q want ready", h.Status)
	}
}

func TestLlamaProviderChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/chat/completions" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
			w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()

	p := NewLlamaProvider(LlamaConfig{ServerURL: ts.URL})
	ch, err := p.Chat(context.Background(), ChatRequest{
		Message:  "hi",
		Snapshot: SanitizedSnapshot{MinerState: "running", Height: 3500, SyncState: "current"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var response strings.Builder
	for evt := range ch {
		if evt.Type == "error" {
			t.Fatalf("chat error: %s", evt.Error)
		}
		response.WriteString(evt.Content)
	}
	if !strings.Contains(response.String(), "Hello") {
		t.Fatalf("response=%q missing Hello", response.String())
	}
}

func TestLlamaProviderChatCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		<-r.Context().Done()
	}))
	defer ts.Close()

	p := NewLlamaProvider(LlamaConfig{ServerURL: ts.URL})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := p.Chat(ctx, ChatRequest{Message: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range ch {
	}
}

func TestLlamaProviderStopClosesChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer ts.Close()

	p := NewLlamaProvider(LlamaConfig{ServerURL: ts.URL})
	if err := p.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLlamaProviderSystemPromptSafety(t *testing.T) {
	snap := SanitizedSnapshot{
		Network: "LBTC mainnet", Version: "1.0.5", Height: 3500,
		SyncState: "current", PeerCount: 3, GoodPeerCount: 3, AgreeingPeers: 3,
		MinerState: "running", MiningSafe: true, ActiveThreads: 4,
		RPCHealth: "ok", WalletLocked: false,
		AvailableLBTC: "100.0", StorageOK: true,
	}
	prompt := buildSystemPrompt(snap)
	forbidden := []string{"seed", "private key", "password", "credential", "secret"}
	for _, f := range forbidden {
		if strings.Contains(strings.ToLower(prompt), f) {
			t.Fatalf("system prompt contains forbidden word: %q", f)
		}
	}
	if !strings.Contains(prompt, "LBTC mainnet") {
		t.Fatal("system prompt missing network info")
	}
}

func TestLlamaProviderJSONRoundTrip(t *testing.T) {
	snap := SanitizedSnapshot{
		Network: "LBTC mainnet", Version: "1.0.5", Height: 3500,
		SyncState: "current", MinerState: "running", MiningSafe: true,
		RPCHealth: "ok", StorageOK: true, NodeRunning: true,
	}
	b, _ := json.Marshal(snap)
	var restored SanitizedSnapshot
	if err := json.Unmarshal(b, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Height != 3500 {
		t.Fatal("round-trip height mismatch")
	}
}
