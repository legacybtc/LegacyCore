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
	mu        sync.Mutex
	provider  AIProvider
	config    AIConfig
	started   int32
	modelName string

	pid        int32
	startTime  time.Time
	lastError  string
	reqLatency time.Duration
	tokensPS   float64
	logger     *log.Logger
}

func NewLifecycleManager(provider AIProvider, logger *log.Logger) *LifecycleManager {
	return &LifecycleManager{provider: provider, config: DefaultConfig(), logger: logger}
}

func (lm *LifecycleManager) logf(format string, args ...any) {
	if lm.logger != nil {
		lm.logger.Printf(format, args...)
	}
}

func (lm *LifecycleManager) Start(ctx context.Context, cfg AIConfig) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if atomic.LoadInt32(&lm.started) == 1 {
		return fmt.Errorf("AI service is already running")
	}
	startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := lm.provider.Start(startCtx, cfg); err != nil {
		lm.lastError = err.Error()
		return fmt.Errorf("start AI provider: %w", err)
	}
	lm.config = cfg
	lm.startTime = time.Now()
	lm.lastError = ""
	atomic.StoreInt32(&lm.started, 1)
	atomic.StoreInt32(&lm.pid, int32(time.Now().UnixNano()%100000))
	lm.logf("AI service started")
	return nil
}

func (lm *LifecycleManager) Stop(ctx context.Context) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if atomic.LoadInt32(&lm.started) == 0 {
		return nil
	}
	lm.modelName = ""
	stopCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := lm.provider.Stop(stopCtx); err != nil {
		lm.lastError = err.Error()
		return fmt.Errorf("stop AI provider: %w", err)
	}
	atomic.StoreInt32(&lm.started, 0)
	atomic.StoreInt32(&lm.pid, 0)
	lm.logf("AI service stopped")
	return nil
}

func (lm *LifecycleManager) Restart(ctx context.Context, cfg AIConfig) error {
	if err := lm.Stop(ctx); err != nil {
		lm.logf("stop during restart: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	return lm.Start(ctx, cfg)
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
	h.PID = int(atomic.LoadInt32(&lm.pid))
	if !lm.startTime.IsZero() {
		h.Uptime = time.Since(lm.startTime).Round(time.Second).String()
	}
	return h, nil
}

func (lm *LifecycleManager) LoadModel(ctx context.Context, model string) error {
	if atomic.LoadInt32(&lm.started) == 0 {
		return fmt.Errorf("AI service is not running")
	}
	if err := lm.provider.LoadModel(ctx, model); err != nil {
		lm.lastError = err.Error()
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
		return nil, fmt.Errorf("AI service is not running")
	}
	start := time.Now()
	ch, err := lm.provider.Chat(ctx, req)
	if err != nil {
		lm.lastError = err.Error()
		return nil, err
	}
	wrapped := make(chan ChatEvent, 1)
	go func() {
		defer close(wrapped)
		tokenCount := 0
		for evt := range ch {
			if evt.Type == "token" {
				tokenCount++
			}
			if evt.Type == "done" {
				elapsed := time.Since(start).Seconds()
				lm.mu.Lock()
				lm.reqLatency = time.Duration(elapsed * float64(time.Second))
				if elapsed > 0 && tokenCount > 0 {
					lm.tokensPS = float64(tokenCount) / elapsed
				}
				lm.mu.Unlock()
			}
			wrapped <- evt
		}
	}()
	return wrapped, nil
}

func (lm *LifecycleManager) IsRunning() bool { return atomic.LoadInt32(&lm.started) == 1 }

func (lm *LifecycleManager) Diagnostics() map[string]any {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	uptime := ""
	if !lm.startTime.IsZero() {
		uptime = time.Since(lm.startTime).Round(time.Second).String()
	}
	return map[string]any{
		"running":           atomic.LoadInt32(&lm.started) == 1,
		"pid":               atomic.LoadInt32(&lm.pid),
		"uptime":            uptime,
		"model_loaded":      lm.modelName != "",
		"model_name":        lm.modelName,
		"backend":           lm.config.Backend,
		"last_error":        lm.lastError,
		"request_latency_ms": lm.reqLatency.Milliseconds(),
		"tokens_per_sec":    lm.tokensPS,
	}
}
