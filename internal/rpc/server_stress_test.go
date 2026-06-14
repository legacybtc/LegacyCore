package rpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/storage"
)

func TestSustainedRPCResponsiveness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping sustained RPC stress test in short mode")
	}
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		t.Fatal(err)
	}
	s := &Server{chain: chain}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				http.Error(w, fmt.Sprintf("internal panic: %v", v), http.StatusInternalServerError)
			}
		}()
		s.handle(w, r)
	})

	methods := []string{"help"}

	goroutinesBefore := runtime.NumGoroutine()

	totalReqs, failures, avgLat, maxLat, p95Lat, maxActive, _ := sustainedRPCStressTest(
		t, handler, 5, 5*time.Second, methods,
	)

	goroutinesAfter := runtime.NumGoroutine()
	failRate := float64(0)
	if totalReqs > 0 {
		failRate = float64(failures) / float64(totalReqs) * 100
	}

	t.Logf("RPC stress test results (5 concurrent clients, 5s)")
	t.Logf("  total requests:       %d", totalReqs)
	t.Logf("  failures/timeouts:    %d (%.1f%%)", failures, failRate)
	t.Logf("  average latency:      %v", avgLat)
	t.Logf("  maximum latency:      %v", maxLat)
	t.Logf("  p95 latency:          %v", p95Lat)
	t.Logf("  highest active reqs:  %d", maxActive)
	t.Logf("  goroutines (before):  %d", goroutinesBefore)
	t.Logf("  goroutines (after):   %d", goroutinesAfter)

	s.rpcDiagMu.Lock()
	rpcActive := s.rpcActiveRequests
	rpcTotal := s.rpcTotalCalls
	rpcDuration := s.rpcTotalDuration
	rpcTimeout := s.rpcTimeoutCount
	rpcError := s.rpcErrorCount
	s.rpcDiagMu.Unlock()
	t.Logf("  RPC diagnostics: active=%d, total=%d, duration=%.1fs, timeouts=%d, errors=%d",
		rpcActive, rpcTotal, rpcDuration.Seconds(), rpcTimeout, rpcError)

	// Verify goroutines are stable (no leaks)
	if goroutinesAfter > goroutinesBefore+10 {
		t.Fatalf("possible goroutine leak: before=%d after=%d", goroutinesBefore, goroutinesAfter)
	}
	// Accept up to 50% connection-level failures under high concurrency
	if failures > 0 && failRate > 50.0 {
		t.Fatalf("RPC stress test failure rate %.1f%% exceeds 50%% threshold", failRate)
	}
}

func sustainedRPCStressTest(
	t testing.TB,
	handler http.Handler,
	concurrency int,
	duration time.Duration,
	pollMethods []string,
) (totalRequests int64, failures int64, avgLatency time.Duration, maxLatency time.Duration, p95Latency time.Duration, maxActive int64, allLatencies []time.Duration) {
	server := httptest.NewServer(handler)
	defer server.Close()

	var (
		wg             sync.WaitGroup
		totalReqs      int64
		totalFailures  int64
		latencySum     int64
		latencyMax     int64
		activeReqCount int64
		maxActiveReqs  int64
		stopMu         sync.Mutex
		stopped        bool
		latMu          sync.Mutex
		latencies      []time.Duration
	)

	stop := func() bool {
		stopMu.Lock()
		defer stopMu.Unlock()
		return stopped
	}

	worker := func() {
		defer wg.Done()
		client := &http.Client{Timeout: 30 * time.Second}
		var methodIndex int
		for {
			if stop() {
				return
			}
			method := pollMethods[methodIndex%len(pollMethods)]
			methodIndex++
			body, _ := json.Marshal(map[string]any{
				"jsonrpc": "1.0",
				"id":      fmt.Sprintf("stress-%d", methodIndex),
				"method":  method,
				"params":  []any{},
			})
			start := time.Now()
			active := atomic.AddInt64(&activeReqCount, 1)
			for {
				cur := atomic.LoadInt64(&maxActiveReqs)
				if active <= cur {
					break
				}
				if atomic.CompareAndSwapInt64(&maxActiveReqs, cur, active) {
					break
				}
			}
			resp, err := client.Post(server.URL, "application/json", bytes.NewReader(body))
			elapsed := time.Since(start)
			atomic.AddInt64(&totalReqs, 1)
			atomic.AddInt64(&latencySum, int64(elapsed))
			atomic.AddInt64(&activeReqCount, -1)
			latMu.Lock()
			latencies = append(latencies, elapsed)
			latMu.Unlock()
			if err != nil {
				atomic.AddInt64(&totalFailures, 1)
				continue
			}
			_, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				atomic.AddInt64(&totalFailures, 1)
				continue
			}
			for {
				cur := atomic.LoadInt64(&latencyMax)
				if int64(elapsed) <= cur {
					break
				}
				if atomic.CompareAndSwapInt64(&latencyMax, cur, int64(elapsed)) {
					break
				}
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	time.Sleep(duration)
	stopMu.Lock()
	stopped = true
	stopMu.Unlock()
	wg.Wait()

	totalRequests = atomic.LoadInt64(&totalReqs)
	failures = atomic.LoadInt64(&totalFailures)
	if totalRequests > 0 {
		avgLatency = time.Duration(atomic.LoadInt64(&latencySum) / totalRequests)
	}
	maxLatency = time.Duration(atomic.LoadInt64(&latencyMax))
	maxActive = atomic.LoadInt64(&maxActiveReqs)

	// Compute p95
	latMu.Lock()
	if len(latencies) > 0 {
		sorted := make([]time.Duration, len(latencies))
		copy(sorted, latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		idx := len(sorted) * 95 / 100
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		p95Latency = sorted[idx]
		allLatencies = sorted
	}
	latMu.Unlock()

	return
}

func BenchmarkRPCMethodPoll(b *testing.B) {
	chain, err := blockchain.New(chaincfg.MainNet, safetyTestHasher{}, storage.NewFileStore(b.TempDir()))
	if err != nil {
		b.Fatal(err)
	}
	genesis, err := safetyTestGenesisBlock()
	if err != nil {
		b.Fatal(err)
	}
	if err := chain.ProcessBlock(genesis); err != nil {
		b.Fatal(err)
	}
	s := &Server{chain: chain}
	methods := []string{
		"help",
		"getchainparams",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		method := methods[i%len(methods)]
		body, _ := json.Marshal(map[string]any{
			"jsonrpc": "1.0",
			"id":      fmt.Sprintf("bench-%d", i),
			"method":  method,
			"params":  []any{},
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		s.handle(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("status=%d method=%s", rec.Code, method)
		}
	}
}

func benchMinerYield(b *testing.B, threads int, interval uint32) {
	var counter uint64
	done := make(chan struct{})
	var mu sync.Mutex
	var totalIterations int64

	b.SetParallelism(1)
	b.ResetTimer()

	var wg sync.WaitGroup
	for t := 0; t < threads; t++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			localCount := int64(0)
			var yieldCounter uint32
			mask := interval - 1
			for {
				select {
				case <-done:
					mu.Lock()
					totalIterations += localCount
					mu.Unlock()
					return
				default:
				}
				counter++
				localCount++
				yieldCounter++
				if yieldCounter&mask == 0 {
					runtime.Gosched()
				}
			}
		}()
	}

	time.Sleep(time.Duration(b.N) * time.Microsecond)
	close(done)
	wg.Wait()

	b.ReportMetric(float64(totalIterations)/float64(b.N)*1e6, "iterations/sec")
}

func BenchmarkMinerYield1Thread(b *testing.B)   { benchMinerYield(b, 1, 64) }
func BenchmarkMinerYield4Threads(b *testing.B)  { benchMinerYield(b, 4, 64) }
func BenchmarkMinerYield10Threads(b *testing.B) { benchMinerYield(b, 10, 64) }

func BenchmarkMinerYieldOld1Thread(b *testing.B)   { benchMinerYield(b, 1, 256) }
func BenchmarkMinerYieldOld4Threads(b *testing.B)  { benchMinerYield(b, 4, 256) }
func BenchmarkMinerYieldOld10Threads(b *testing.B) { benchMinerYield(b, 10, 256) }
