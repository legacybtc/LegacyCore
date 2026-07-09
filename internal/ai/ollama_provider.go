package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type OllamaProvider struct {
	baseURL    string
	model      string
	started    int32
	modelReady int32
	client     *http.Client
}

func NewOllamaProvider() *OllamaProvider {
	return &OllamaProvider{
		baseURL: "http://127.0.0.1:11434",
		model:   "",
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OllamaProvider) Start(_ context.Context, cfg AIConfig) error {
	if o.IsAvailable() {
		atomic.StoreInt32(&o.started, 1)
		if cfg.ModelName != "" {
			o.model = cfg.ModelName
			atomic.StoreInt32(&o.modelReady, 1)
		}
		return nil
	}
	return fmt.Errorf("ollama not reachable at %s — install from https://ollama.com", o.baseURL)
}

func (o *OllamaProvider) Stop(_ context.Context) error {
	atomic.StoreInt32(&o.started, 0)
	atomic.StoreInt32(&o.modelReady, 0)
	return nil
}

func (o *OllamaProvider) Health(_ context.Context) (AIHealth, error) {
	s := StatusStopped
	if atomic.LoadInt32(&o.started) == 1 {
		s = StatusReady
	}
	return AIHealth{
		Status:      s,
		ModelLoaded: atomic.LoadInt32(&o.modelReady) == 1,
		ModelName:   o.model,
		Backend:     "ollama",
	}, nil
}

func (o *OllamaProvider) IsAvailable() bool {
	return o.ping()
}

func (o *OllamaProvider) ping() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	resp, err := o.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (o *OllamaProvider) ListModels(ctx context.Context) ([]AIModel, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", o.baseURL+"/api/tags", nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ollama returned %d", resp.StatusCode)
	}
	type ollamaModel struct {
		Name   string `json:"name"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"`
	}
	type tagsResp struct {
		Models []ollamaModel `json:"models"`
	}
	var tr tagsResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, err
	}
	out := make([]AIModel, 0, len(tr.Models))
	for _, m := range tr.Models {
		out = append(out, AIModel{
			Name:       m.Name,
			FileSizeMB: int(m.Size / (1024 * 1024)),
			SHA256:     strings.TrimPrefix(m.Digest, "sha256:"),
			License:    "See model source",
		})
	}
	return out, nil
}

func (o *OllamaProvider) LoadModel(ctx context.Context, model string) error {
	o.model = model
	atomic.StoreInt32(&o.modelReady, 1)
	return nil
}

func (o *OllamaProvider) UnloadModel(_ context.Context) error {
	o.model = ""
	atomic.StoreInt32(&o.modelReady, 0)
	return nil
}

func (o *OllamaProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 32)

	model := o.model
	if model == "" {
		models, err := o.ListModels(ctx)
		if err != nil || len(models) == 0 {
			close(ch)
			return ch, fmt.Errorf("no ollama model available — pull a model first: ollama pull llama3.2:1b")
		}
		model = models[0].Name
	}

	systemPrompt := buildOllamaSystemPrompt(req)
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
	}
	if req.Mode == "developer" {
		messages = append(messages, map[string]string{"role": "system", "content": "You are in developer mode. The user can execute allowlisted CLI tools from the LegacyCoin wallet."})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.Message})

	body := map[string]interface{}{
		"model":    model,
		"messages": messages,
		"stream":   true,
		"options":  map[string]interface{}{"temperature": 0.7, "num_predict": 256},
	}
	payload, _ := json.Marshal(body)

	reqCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	go func() {
		defer close(ch)
		defer cancel()

		r, err := http.NewRequestWithContext(reqCtx, "POST", o.baseURL+"/api/chat", bytes.NewReader(payload))
		if err != nil {
			ch <- ChatEvent{Type: "error", Error: err.Error()}
			return
		}
		r.Header.Set("Content-Type", "application/json")
		resp, err := o.client.Do(r)
		if err != nil {
			ch <- ChatEvent{Type: "error", Error: fmt.Sprintf("ollama chat failed: %v", err)}
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			bodyB, _ := io.ReadAll(resp.Body)
			ch <- ChatEvent{Type: "error", Error: fmt.Sprintf("ollama returned %d: %s", resp.StatusCode, string(bodyB))}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		totalTokens := 0
		for scanner.Scan() {
			select {
			case <-reqCtx.Done():
				return
			default:
			}
			line := scanner.Text()
			if line == "" {
				continue
			}
			var evt struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Done bool `json:"done"`
			}
			if err := json.Unmarshal([]byte(line), &evt); err != nil {
				continue
			}
			if evt.Message.Content != "" {
				totalTokens++
				select {
				case ch <- ChatEvent{Type: "token", Content: evt.Message.Content}:
				case <-reqCtx.Done():
					return
				}
			}
			if evt.Done {
				break
			}
		}
		ch <- ChatEvent{Type: "done", Tokens: totalTokens}
	}()
	return ch, nil
}

func buildOllamaSystemPrompt(req ChatRequest) string {
	s := req.Snapshot
	return fmt.Sprintf(
		"You are Legacy AI Companion, a local privacy-first AI assistant inside the Legacy Coin desktop wallet. "+
			"You run entirely on the user's machine. No cloud, no tracking, no secrets shared. Advisory only — you cannot spend coins or control the node.\n\n"+
			"Current wallet state:\n"+
			"- Network: %s v%s\n"+
			"- Height: %d\n"+
			"- Sync: %s (%d blocks behind)\n"+
			"- Peers: %d connected (%d agree on current chain)\n"+
			"- Mining: %s (safe=%v, %d threads at %s)\n"+
			"- Balance: %s LBTC available, %s total\n"+
			"- Immature rewards: %s LBTC\n"+
			"- Storage: %s\n"+
			"- RPC: %s\n"+
			"- Uptime: %s\n\n"+
			"Be concise, helpful, and friendly. If asked about time, explain you're a local AI without internet access.",
		s.Network, s.Version, s.Height, s.SyncState, s.BlocksBehind,
		s.PeerCount, s.AgreeingPeers, s.MinerState, s.MiningSafe,
		s.ActiveThreads, s.LocalHashrate, s.AvailableLBTC, s.TotalLBTC,
		s.ImmatureLBTC, storageStatus(s), s.RPCHealth, s.Uptime,
	)
}

func storageStatus(s SanitizedSnapshot) string {
	if s.StorageOK {
		return "healthy"
	}
	if s.StorageError != "" {
		return s.StorageError
	}
	return "unknown"
}
