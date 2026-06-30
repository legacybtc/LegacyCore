package ai

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type LifecycleManager struct {
	mu         sync.Mutex
	provider   AIProvider
	config     AIConfig
	started    int32
	modelName  string
	startTime  time.Time
	lastError  string
	logger     *log.Logger
	toolBroker *ToolBroker
}

func NewLifecycleManager(provider AIProvider, logger *log.Logger) *LifecycleManager {
	return &LifecycleManager{
		provider:   provider,
		config:     DefaultConfig(),
		logger:     logger,
		toolBroker: NewToolBroker(),
	}
}

func (lm *LifecycleManager) SetToolBroker(tb *ToolBroker) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.toolBroker = tb
}

func (lm *LifecycleManager) ExecuteTool(ctx context.Context, cmdLine string) ToolResult {
	lm.mu.Lock()
	tb := lm.toolBroker
	lm.mu.Unlock()
	if tb == nil {
		return ToolResult{Command: cmdLine, Allowed: false, Stderr: "no tool broker"}
	}
	return tb.Execute(ctx, cmdLine)
}

func (lm *LifecycleManager) ListTools() []string {
	lm.mu.Lock()
	tb := lm.toolBroker
	lm.mu.Unlock()
	if tb == nil {
		return nil
	}
	return tb.ListAllowlist()
}

func (lm *LifecycleManager) Start(ctx context.Context, cfg AIConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if atomic.LoadInt32(&lm.started) == 1 {
		return fmt.Errorf("already running")
	}
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := lm.provider.Start(startCtx, cfg); err != nil {
		lm.lastError = err.Error()
		return err
	}
	lm.config = cfg
	lm.startTime = time.Now()
	lm.lastError = ""
	atomic.StoreInt32(&lm.started, 1)
	if lm.logger != nil {
		lm.logger.Println("AI started")
	}
	return nil
}

func (lm *LifecycleManager) Stop(ctx context.Context) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if atomic.LoadInt32(&lm.started) == 0 {
		return nil
	}
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := lm.provider.Stop(stopCtx); err != nil {
		lm.lastError = err.Error()
		return err
	}
	atomic.StoreInt32(&lm.started, 0)
	if lm.logger != nil {
		lm.logger.Println("AI stopped")
	}
	return nil
}

func (lm *LifecycleManager) Health(ctx context.Context) (AIHealth, error) {
	if atomic.LoadInt32(&lm.started) == 0 {
		return AIHealth{Status: StatusStopped, Backend: lm.config.Backend}, nil
	}
	healthCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	h, err := lm.provider.Health(healthCtx)
	if err != nil {
		lm.lastError = err.Error()
		h.Status = StatusError
		h.LastError = err.Error()
	}
	if !lm.startTime.IsZero() {
		h.Uptime = time.Since(lm.startTime).Round(time.Second).String()
	}
	return h, nil
}

func (lm *LifecycleManager) LoadModel(ctx context.Context, model string) error {
	if atomic.LoadInt32(&lm.started) == 0 {
		return fmt.Errorf("not running")
	}
	if err := lm.provider.LoadModel(ctx, model); err != nil {
		return err
	}
	lm.modelName = model
	return nil
}

func (lm *LifecycleManager) UnloadModel(ctx context.Context) error {
	if err := lm.provider.UnloadModel(ctx); err != nil {
		return err
	}
	lm.modelName = ""
	return nil
}

func (lm *LifecycleManager) Chat(ctx context.Context, req ChatRequest) (<-chan ChatEvent, error) {
	if atomic.LoadInt32(&lm.started) == 0 {
		return nil, fmt.Errorf("not running")
	}
	return lm.provider.Chat(ctx, req)
}

func (lm *LifecycleManager) IsRunning() bool { return atomic.LoadInt32(&lm.started) == 1 }

func (lm *LifecycleManager) Config() AIConfig { lm.mu.Lock(); defer lm.mu.Unlock(); return lm.config }
