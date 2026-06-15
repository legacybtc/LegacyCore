package ai

import "context"

// AIProvider abstracts the local inference runtime.
// The wallet must never couple to a specific model backend.
type AIProvider interface {
	Start(ctx context.Context, config AIConfig) error
	Stop(ctx context.Context) error
	Health(ctx context.Context) (AIHealth, error)
	ListModels(ctx context.Context) ([]AIModel, error)
	LoadModel(ctx context.Context, model string) error
	UnloadModel(ctx context.Context) error
	Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error)
}

// AIStatus represents the lifecycle state of the AI service.
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

// AIConfig holds user-controlled settings for the AI runtime.
type AIConfig struct {
	Port              int    // localhost listen port (default 19570)
	Backend           string // cuda, hip, vulkan, cpu
	ModelPath         string // path to GGUF model file
	ModelName         string // human-readable model name
	GPUOffloadLayers  int    // number of layers to offload to GPU (0 = CPU only)
	CPUThreads        int    // number of CPU helper threads
	ContextSize       int    // max token context window
	MaxResponseTokens int    // max tokens per response
	HistoryEnabled    bool   // persist conversation history
	SessionToken      string // random token for localhost auth
}

// DefaultConfig returns a safe starter config.
func DefaultConfig() AIConfig {
	return AIConfig{
		Port:              19570,
		Backend:           "cpu",
		GPUOffloadLayers:  0,
		CPUThreads:        4,
		ContextSize:       2048,
		MaxResponseTokens: 512,
		HistoryEnabled:    false,
	}
}

// AIHealth reports the current state of the sidecar and model.
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
	LastError   string   `json:"last_error,omitempty"`
}

// AIModel describes a locally available GGUF model file.
type AIModel struct {
	Name         string `json:"name"`
	Path         string `json:"path"`
	FileSizeMB   int    `json:"file_size_mb"`
	Quantization string `json:"quantization"`
	License      string `json:"license"`
	SHA256       string `json:"sha256,omitempty"`
}

// ChatRequest holds a single user message and the current wallet snapshot.
type ChatRequest struct {
	Message  string           `json:"message"`
	Snapshot SanitizedSnapshot `json:"snapshot"`
}

// ChatEvent represents a streaming response token or control event.
type ChatEvent struct {
	Type    string `json:"type"` // "token", "done", "error"
	Content string `json:"content"`
	Tokens  int    `json:"tokens,omitempty"`
	TPS     float64 `json:"tps,omitempty"`
	Error   string `json:"error,omitempty"`
}
