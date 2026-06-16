package ai

import "context"

// AIProvider abstracts the local inference runtime.
type AIProvider interface {
	Start(ctx context.Context, config AIConfig) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) (AIHealth, error)
	ListModels(ctx context.Context) ([]AIModel, error)
	LoadModel(ctx context.Context, model string) error
	UnloadModel(ctx context.Context) error
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
}

type AIStatus string

const (
	StatusDisabled   AIStatus = "disabled"
	StatusStopped    AIStatus = "stopped"
	StatusStarting   AIStatus = "starting"
	StatusLoading    AIStatus = "loading_model"
	StatusReady      AIStatus = "ready"
	StatusGenerating AIStatus = "generating"
	StatusError      AIStatus = "error"
)

type AIConfig struct {
	Port               int    `json:"port"`
	Backend            string `json:"backend"`
	ModelPath          string `json:"model_path"`
	ModelName          string `json:"model_name"`
	GPUOffloadLayers   int    `json:"gpu_offload_layers"`
	CPUThreads         int    `json:"cpu_threads"`
	ContextSize        int    `json:"context_size"`
	MaxResponseTokens  int    `json:"max_response_tokens"`
	HistoryEnabled     bool   `json:"history_enabled"`
	SessionToken       string `json:"session_token"`
	Provider           string `json:"provider"` // mock, llama-server, ollama
	LlamaBinaryPath    string `json:"llama_binary_path,omitempty"`
	LlamaServerURL     string `json:"llama_server_url,omitempty"`
}

func DefaultConfig() AIConfig {
	return AIConfig{
		Port:              19570,
		Backend:           "cpu",
		GPUOffloadLayers:  0,
		CPUThreads:        4,
		ContextSize:       2048,
		MaxResponseTokens: 512,
		HistoryEnabled:    false,
		Provider:          "mock",
	}
}

type AIHealth struct {
	Status      AIStatus `json:"status"`
	PID         int      `json:"pid,omitempty"`
	Uptime      string   `json:"uptime,omitempty"`
	ModelLoaded bool     `json:"model_loaded"`
	ModelName   string   `json:"model_name,omitempty"`
	Backend     string   `json:"backend"`
	GPUName     string   `json:"gpu_name,omitempty"`
	VRAMMB      int      `json:"vram_mb,omitempty"`
	RAMMB       int      `json:"ram_mb"`
	TokensPS    float64  `json:"tokens_per_sec,omitempty"`
	LastLatency string   `json:"last_latency,omitempty"`
	LastError   string   `json:"last_error,omitempty"`
}

type AIModel struct {
	Name            string `json:"name"`
	Path            string `json:"path"`
	FileSizeMB      int    `json:"file_size_mb"`
	Quantization    string `json:"quantization"`
	License         string `json:"license"`
	SHA256          string `json:"sha256,omitempty"`
	ContextSize     int    `json:"context_size,omitempty"`
	EstimatedRAMMB  int    `json:"estimated_ram_mb,omitempty"`
	EstimatedVRAMMB int    `json:"estimated_vram_mb,omitempty"`
	CompatibleBackend string `json:"compatible_backend,omitempty"`
}

type ChatRequest struct {
	Message  string            `json:"message"`
	Snapshot SanitizedSnapshot `json:"snapshot"`
	Mode     string            `json:"mode,omitempty"` // "advisor" or "developer"
	History  []ChatMessage     `json:"history,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatEvent struct {
	Type    string  `json:"type"`
	Content string  `json:"content"`
	Tokens  int     `json:"tokens,omitempty"`
	TPS     float64 `json:"tps,omitempty"`
	Error   string  `json:"error,omitempty"`
}
