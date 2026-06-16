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

// LlamaProvider connects to a llama-server instance (llama.cpp OpenAI-compatible API).
type LlamaProvider struct {
	mu         sync.Mutex
	serverURL  string
	cmd        *exec.Cmd
	httpClient *http.Client
	config     LlamaConfig
}

type LlamaConfig struct {
	ServerURL        string // http://127.0.0.1:8080
	BinaryPath       string // path to llama-server executable
	ModelPath        string // path to GGUF model
	Host             string // 127.0.0.1
	Port             int    // 8080
	GPUOffloadLayers int    // --n-gpu-layers
	ContextSize      int    // --ctx-size
	Threads          int    // --threads
}

func DefaultLlamaConfig() LlamaConfig {
	return LlamaConfig{
		ServerURL:        "http://127.0.0.1:8080",
		BinaryPath:       "llama-server",
		ModelPath:        "",
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
		config:    cfg,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *LlamaProvider) Start(ctx context.Context, _ AIConfig) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// If connecting to external server, just check health
	if !p.isManaged() {
		return p.checkHealth(ctx)
	}

	// Start managed llama-server process
	args := []string{
		"-m", p.config.ModelPath,
		"--host", p.config.Host,
		"--port", fmt.Sprintf("%d", p.config.Port),
		"-ngl", fmt.Sprintf("%d", p.config.GPUOffloadLayers),
		"-c", fmt.Sprintf("%d", p.config.ContextSize),
		"-t", fmt.Sprintf("%d", p.config.Threads),
	}
	p.cmd = exec.CommandContext(ctx, p.config.BinaryPath, args...)
	p.cmd.Stdout = io.Discard
	p.cmd.Stderr = io.Discard

	if err := p.cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}

	// Wait for health
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			p.killManaged()
			return ctx.Err()
		case <-time.After(time.Second):
		}
		if err := p.checkHealth(ctx); err == nil {
			return nil
		}
	}
	p.killManaged()
	return fmt.Errorf("llama-server did not become healthy within 30s")
}

func (p *LlamaProvider) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isManaged() && p.cmd != nil && p.cmd.Process != nil {
		// Graceful shutdown via API
		req, _ := http.NewRequestWithContext(ctx, "POST", p.serverURL+"/v1/shutdown", nil)
		p.httpClient.Do(req)
		time.Sleep(500 * time.Millisecond)

		if p.cmd.ProcessState == nil || !p.cmd.ProcessState.Exited() {
			p.cmd.Process.Kill()
		}
		p.cmd = nil
	}
	return nil
}

func (p *LlamaProvider) Health(ctx context.Context) (AIHealth, error) {
	if err := p.checkHealth(ctx); err != nil {
		return AIHealth{Status: StatusError, LastError: err.Error(), Backend: "llama.cpp"}, nil
	}
	return AIHealth{
		Status:      StatusReady,
		ModelLoaded: true,
		Backend:     "llama.cpp",
		RAMMB:       0,
	}, nil
}

func (p *LlamaProvider) ListModels(_ context.Context) ([]AIModel, error) {
	if p.config.ModelPath != "" {
		return []AIModel{{
			Name:  filepath.Base(p.config.ModelPath),
			Path:  p.config.ModelPath,
		}}, nil
	}
	return nil, nil
}

func (p *LlamaProvider) LoadModel(ctx context.Context, model string) error {
	p.config.ModelPath = model
	if p.isManaged() {
		return p.Start(ctx, AIConfig{})
	}
	return nil
}

func (p *LlamaProvider) UnloadModel(ctx context.Context) error {
	return p.Stop(ctx)
}

func (p *LlamaProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Messages    []message `json:"messages"`
		Stream      bool      `json:"stream"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float64   `json:"temperature"`
	}

	sysPrompt := buildSystemPrompt(req.Snapshot)
	body, _ := json.Marshal(request{
		Messages: []message{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: req.Message},
		},
		Stream:      true,
		MaxTokens:   512,
		Temperature: 0.1,
	})

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.serverURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llama-server chat: %w", err)
	}

	ch := make(chan ChatEvent, 1)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				ch <- ChatEvent{Type: "done"}
				return
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &chunk) == nil {
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						ch <- ChatEvent{Type: "token", Content: choice.Delta.Content}
					}
				}
			}
		}
	}()
	return ch, nil
}

func (p *LlamaProvider) isManaged() bool {
	return p.config.ModelPath != ""
}

func (p *LlamaProvider) checkHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.serverURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("health returned %d", resp.StatusCode)
	}
	return nil
}

func (p *LlamaProvider) killManaged() {
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
	}
}

func buildSystemPrompt(s SanitizedSnapshot) string {
	return fmt.Sprintf(`You are Legacy AI, a local read-only assistant for Legacy Wallet.
Never request or reveal private keys, seed phrases, passwords, or RPC credentials.
Use the provided snapshot as the source of truth. Do not invent data.

Current node state:
- Network: %s, Version: %s
- Height: %d, Sync: %s, Blocks behind: %d
- Peers: %d connected (%d good, %d agree)
- Miner: %s (safe=%v, threads=%d/%d, %s KH/s)
- RPC: %s (errors=%d, timeouts=%d)
- Blocks: accepted=%d rejected=%d stale=%d
- Wallet: locked=%v, available=%s LBTC, total=%s LBTC, immature=%s LBTC
- Storage: %s (error=%s)
- Template: fresh=%v age=%s
- Node: running=%v, uptime=%s`,
		s.Network, s.Version,
		s.Height, s.SyncState, s.BlocksBehind,
		s.PeerCount, s.GoodPeerCount, s.AgreeingPeers,
		s.MinerState, s.MiningSafe, s.ActiveThreads, s.ConfiguredThreads, s.LocalHashrate,
		s.RPCHealth, s.RPCErrorCount, s.RPCTimeoutCount,
		s.AcceptedBlocks, s.RejectedBlocks, s.StaleBlocks,
		s.WalletLocked, s.AvailableLBTC, s.TotalLBTC, s.ImmatureLBTC,
		func() string { if s.StorageOK { return "OK" }; return "FAILED" }(), s.StorageError,
		s.TemplateFresh, s.TemplateAge,
		s.NodeRunning, s.Uptime,
	)
}
