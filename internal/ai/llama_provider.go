package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type LlamaProvider struct {
	mu         sync.Mutex
	serverURL  string
	apiKey     string
	cmd        *exec.Cmd
	httpClient *http.Client
	config     LlamaConfig
	pid        int
}

type LlamaConfig struct {
	ServerURL        string
	BinaryPath       string
	ModelPath        string
	Host             string
	Port             int
	GPUOffloadLayers int
	ContextSize      int
	Threads          int
	APIKey           string
}

func DefaultLlamaConfig() LlamaConfig {
	return LlamaConfig{
		ServerURL:        "http://127.0.0.1:8080",
		BinaryPath:       "llama-server",
		Host:             "127.0.0.1",
		Port:             8080,
		GPUOffloadLayers: 0,
		ContextSize:      2048,
		Threads:          4,
	}
}

func NewLlamaProvider(cfg LlamaConfig) *LlamaProvider {
	return &LlamaProvider{
		serverURL: cfg.ServerURL,
		apiKey:    cfg.APIKey,
		config:    cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *LlamaProvider) Start(ctx context.Context, aiCfg AIConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.config.BinaryPath != "" && p.config.ModelPath != "" {
		return p.startManaged(ctx)
	}
	return p.checkHealth(ctx)
}

func (p *LlamaProvider) startManaged(ctx context.Context) error {
	args := []string{
		"-m", p.config.ModelPath, "--host", p.config.Host,
		"--port", fmt.Sprintf("%d", p.config.Port),
		"-ngl", fmt.Sprintf("%d", p.config.GPUOffloadLayers),
		"-c", fmt.Sprintf("%d", p.config.ContextSize),
		"-t", fmt.Sprintf("%d", p.config.Threads),
	}
	if p.apiKey != "" {
		args = append(args, "--api-key", p.apiKey)
	}
	p.cmd = exec.CommandContext(ctx, p.config.BinaryPath, args...)
	p.cmd.Stdout = io.Discard
	p.cmd.Stderr = io.Discard
	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}
	p.pid = p.cmd.Process.Pid
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done(): p.killManaged(); return ctx.Err()
		case <-time.After(time.Second):
		}
		if err := p.checkHealth(ctx); err == nil { return nil }
	}
	p.killManaged()
	return fmt.Errorf("llama-server not healthy after 30s")
}

func (p *LlamaProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil && p.cmd.Process != nil {
		req, _ := http.NewRequestWithContext(ctx, "POST", p.serverURL+"/v1/shutdown", nil)
		p.httpClient.Do(req)
		time.Sleep(500 * time.Millisecond)
		if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() {
			p.cmd.Process.Kill()
		}
		p.cmd = nil
		p.pid = 0
	}
	return nil
}

func (p *LlamaProvider) Health(ctx context.Context) (AIHealth, error) {
	if err := p.checkHealth(ctx); err != nil {
		return AIHealth{Status: StatusError, LastError: err.Error(), Backend: "llama.cpp"}, nil
	}
	return AIHealth{Status: StatusReady, ModelLoaded: true, Backend: "llama.cpp", PID: p.pid}, nil
}

func (p *LlamaProvider) checkHealth(ctx context.Context) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", p.serverURL+"/health", nil)
	resp, err := p.httpClient.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return fmt.Errorf("health: %d", resp.StatusCode) }
	return nil
}

func (p *LlamaProvider) ListModels(_ context.Context) ([]AIModel, error) {
	if p.config.ModelPath != "" {
		return []AIModel{{Name: filepath.Base(p.config.ModelPath), Path: p.config.ModelPath}}, nil
	}
	return nil, nil
}

func (p *LlamaProvider) LoadModel(_ context.Context, model string) error {
	p.config.ModelPath = model
	return nil
}

func (p *LlamaProvider) UnloadModel(_ context.Context) error { return p.Stop(context.Background()) }

func (p *LlamaProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	type msg struct{ Role, Content string }
	type body struct {
		Messages    []msg   `json:"messages"`
		Stream      bool    `json:"stream"`
		MaxTokens   int     `json:"max_tokens"`
		Temperature float64 `json:"temperature"`
	}
	prompt := buildSystemPrompt(req)
	payload, _ := json.Marshal(body{
		Messages:    []msg{{Role: "system", Content: prompt}, {Role: "user", Content: req.Message}},
		Stream:      true, MaxTokens: 512, Temperature: 0.1,
	})
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", p.serverURL+"/v1/chat/completions", bytes.NewReader(payload))
	httpReq.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" { httpReq.Header.Set("Authorization", "Bearer "+p.apiKey) }
	resp, err := p.httpClient.Do(httpReq)
	if err != nil { return nil, fmt.Errorf("chat: %w", err) }

	ch := make(chan ChatEvent, 1)
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data: ") { continue }
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" { ch <- ChatEvent{Type: "done"}; return }
			var chunk struct{ Choices []struct{ Delta struct{ Content string `json:"content"` } `json:"delta"` } `json:"choices"` }
			if json.Unmarshal([]byte(data), &chunk) == nil {
				for _, c := range chunk.Choices {
					if c.Delta.Content != "" { ch <- ChatEvent{Type: "token", Content: c.Delta.Content} }
				}
			}
		}
	}()
	return ch, nil
}

func (p *LlamaProvider) killManaged() {
	if p.cmd != nil && p.cmd.Process != nil { p.cmd.Process.Kill() }
}

func (p *LlamaProvider) PID() int { return p.pid }

func buildSystemPrompt(req ChatRequest) string {
	s := req.Snapshot
	return fmt.Sprintf(`You are Legacy AI Companion, a local read-only assistant.
Network: %s | Height: %d | Sync: %s | Peers: %d (%d good, %d agree)
Miner: %s (safe=%v) | Threads: %d/%d | RPC: %s
Wallet: %s LBTC available | Storage: %s
Mode: %s`, s.Network, s.Height, s.SyncState, s.PeerCount, s.GoodPeerCount, s.AgreeingPeers,
		s.MinerState, s.MiningSafe, s.ActiveThreads, s.ConfiguredThreads, s.RPCHealth,
		s.AvailableLBTC, func() string { if s.StorageOK { return "OK" }; return "ERROR" }(), req.Mode)
}
