package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

type WebSearcher struct {
	client  *http.Client
	timeout time.Duration
}

type SearchResult struct {
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}

func NewWebSearcher() *WebSearcher {
	return &WebSearcher{
		client:  &http.Client{Timeout: 10 * time.Second},
		timeout: 10 * time.Second,
	}
}

func (ws *WebSearcher) Search(ctx context.Context, query string, maxResults int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	ctx, cancel := context.WithTimeout(ctx, ws.timeout)
	defer cancel()

	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "LegacyWallet/1.0.6 (AI Companion; +https://github.com/legacybtc/LegacyCore)")

	resp, err := ws.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search returned %d", resp.StatusCode)
	}

	return parseDuckDuckGoResults(resp.Body, maxResults), nil
}

func (ws *WebSearcher) FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.Snippet, r.URL))
	}
	return sb.String()
}

func parseDuckDuckGoResults(body io.Reader, max int) []SearchResult {
	results := make([]SearchResult, 0, max)
	doc, err := html.Parse(body)
	if err != nil {
		return results
	}

	var current *SearchResult
	var inSnippet bool

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case n.Data == "a" && hasClass(n, "result__a"):
				current = &SearchResult{}
				current.URL = getAttr(n, "href")
				current.Title = getText(n)

			case n.Data == "a" && hasClass(n, "result__snippet"):
				if current != nil {
					current.Snippet = strings.TrimSpace(getText(n))
					results = append(results, *current)
					current = nil
					inSnippet = false
					if len(results) >= max {
						return
					}
				}

			case n.Data == "span" && hasClass(n, "result__snippet"):
				inSnippet = true
			}
		}
		if n.FirstChild != nil {
			walk(n.FirstChild)
		}
		if n.NextSibling != nil && len(results) < max {
			walk(n.NextSibling)
		}
		_ = inSnippet
	}
	if doc != nil {
		walk(doc)
	}
	return results
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			for _, c := range strings.Fields(attr.Val) {
				if strings.HasPrefix(c, class) || c == class {
					return true
				}
			}
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func getText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(getText(c))
	}
	return sb.String()
}

// MemoryStore manages conversation history
type MemoryStore struct {
	messages []ChatMessage
	maxSize  int
	mu       sync.Mutex
}

func NewMemoryStore(maxSize int) *MemoryStore {
	if maxSize <= 0 {
		maxSize = 20
	}
	return &MemoryStore{maxSize: maxSize, messages: make([]ChatMessage, 0, maxSize)}
}

func (ms *MemoryStore) Add(msg ChatMessage) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = append(ms.messages, msg)
	if len(ms.messages) > ms.maxSize {
		ms.messages = ms.messages[len(ms.messages)-ms.maxSize:]
	}
}

func (ms *MemoryStore) Get() []ChatMessage {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	out := make([]ChatMessage, len(ms.messages))
	copy(out, ms.messages)
	return out
}

func (ms *MemoryStore) Clear() {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages = ms.messages[:0]
}

func (ms *MemoryStore) Len() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return len(ms.messages)
}

// GroqProvider connects to Groq API (free tier: https://console.groq.com)
type GroqProvider struct {
	apiKey     string
	model      string
	baseURL    string
	client     *http.Client
	started    int32
}

func NewGroqProvider(apiKey string) *GroqProvider {
	if apiKey == "" {
		return &GroqProvider{baseURL: "https://api.groq.com/openai/v1", client: &http.Client{Timeout: 60 * time.Second}}
	}
	return &GroqProvider{
		apiKey:  apiKey,
		model:   "llama-3.1-8b-instant",
		baseURL: "https://api.groq.com/openai/v1",
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

func (g *GroqProvider) SetAPIKey(key string) { g.apiKey = key }
func (g *GroqProvider) SetModel(model string) { g.model = model }

func (g *GroqProvider) Start(_ context.Context, _ AIConfig) error {
	if g.apiKey == "" {
		return fmt.Errorf("Groq API key required — get one free at https://console.groq.com")
	}
	return nil
}

func (g *GroqProvider) Stop(_ context.Context) error { return nil }

func (g *GroqProvider) Health(_ context.Context) (AIHealth, error) {
	return AIHealth{Status: StatusReady, ModelName: g.model, Backend: "groq", ModelLoaded: g.apiKey != ""}, nil
}

func (g *GroqProvider) ListModels(_ context.Context) ([]AIModel, error) {
	return []AIModel{
		{Name: "llama-3.1-8b-instant", FileSizeMB: 0, License: "Meta Llama 3.1", CompatibleBackend: "groq"},
		{Name: "llama-3.3-70b-versatile", FileSizeMB: 0, License: "Meta Llama 3.3", CompatibleBackend: "groq"},
		{Name: "mixtral-8x7b-32768", FileSizeMB: 0, License: "Apache 2.0", CompatibleBackend: "groq"},
		{Name: "gemma2-9b-it", FileSizeMB: 0, License: "Google", CompatibleBackend: "groq"},
	}, nil
}

func (g *GroqProvider) LoadModel(_ context.Context, model string) error { g.model = model; return nil }
func (g *GroqProvider) UnloadModel(_ context.Context) error            { return nil }

func (g *GroqProvider) Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	ch := make(chan ChatEvent, 64)

	go func() {
		defer close(ch)

		snapshotContext := buildSnapshotContext(req.Snapshot)
		messages := []map[string]string{
			{"role": "system", "content": systemPrompt(req.Mode) + "\n\n" + snapshotContext},
		}
		for _, h := range req.History {
			messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
		}
		messages = append(messages, map[string]string{"role": "user", "content": req.Message})

		body := map[string]interface{}{
			"model":       g.model,
			"messages":    messages,
			"stream":      true,
			"temperature": 0.7,
			"max_tokens":  512,
		}
		payload, _ := json.Marshal(body)

		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		r, err := http.NewRequestWithContext(ctx, "POST", g.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			ch <- ChatEvent{Type: "error", Error: err.Error()}
			return
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+g.apiKey)

		resp, err := g.client.Do(r)
		if err != nil {
			ch <- ChatEvent{Type: "error", Error: fmt.Sprintf("Groq API: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			ch <- ChatEvent{Type: "error", Error: fmt.Sprintf("Groq %d: %s", resp.StatusCode, string(b))}
			return
		}

		decoder := json.NewDecoder(resp.Body)
		totalTokens := 0
		for {
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage struct {
					TotalTokens int `json:"total_tokens"`
				} `json:"usage"`
			}
			if err := decoder.Decode(&chunk); err != nil {
				if err == io.EOF { break }
				ch <- ChatEvent{Type: "error", Error: err.Error()}
				return
			}
			for _, choice := range chunk.Choices {
				if choice.Delta.Content != "" {
					totalTokens++
					ch <- ChatEvent{Type: "token", Content: choice.Delta.Content}
				}
			}
			if chunk.Usage.TotalTokens > 0 {
				totalTokens = chunk.Usage.TotalTokens
			}
		}
		ch <- ChatEvent{Type: "done", Tokens: totalTokens}
	}()
	return ch, nil
}

func buildSnapshotContext(s SanitizedSnapshot) string {
	return fmt.Sprintf(
		"Current wallet state:\n"+
			"- Network: %s v%s, height %d\n"+
			"- Sync: %s (%d blocks behind)\n"+
			"- Peers: %d connected (%d agree)\n"+
			"- Mining: %s (safe=%v, %d threads at %s KH/s)\n"+
			"- Balance: %s LBTC available / %s total\n"+
			"- Immature rewards: %s LBTC\n"+
			"- Storage: %s\n"+
			"- RPC: %s\n"+
			"- Uptime: %s",
		s.Network, s.Version, s.Height,
		s.SyncState, s.BlocksBehind,
		s.PeerCount, s.AgreeingPeers,
		s.MinerState, s.MiningSafe, s.ActiveThreads, s.LocalHashrate,
		s.AvailableLBTC, s.TotalLBTC,
		s.ImmatureLBTC,
		func() string {
			if s.StorageOK { return "healthy" }
			return s.StorageError
		}(),
		s.RPCHealth,
		s.Uptime,
	)
}

func systemPrompt(mode string) string {
	base := "You are Legacy AI Companion, a helpful assistant inside the LegacyCoin desktop wallet. " +
		"You are running on the user's local machine. Be concise, accurate, and friendly. " +
		"You have access to the user's wallet state snapshot. " +
		"Advise on node health, sync, mining, peers, rewards, and storage. " +
		"Never ask for or store private keys, seed phrases, or passwords."

	if mode == "developer" {
		base += " You are in developer mode — the user can execute allowlisted CLI tools and you have web search capability."
	}
	return base
}
