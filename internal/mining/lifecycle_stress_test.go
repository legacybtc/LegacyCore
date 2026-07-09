package mining

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/storage"
)

func TestMinerLifecycleExtendedStress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping extended lifecycle stress in short mode")
	}
	if pow.BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend")
	}

	dir := t.TempDir()
	chain, err := blockchain.New(chaincfg.MainNet, pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}, storage.NewFileStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}

	threads := 4
	testDuration := 5 * time.Minute
	cycleDuration := 3 * time.Second
	pubHash := make([]byte, 20)

	var (
		startedCount atomic.Int64
		exitedCount  atomic.Int64
		activeCount  atomic.Int64
		startG       = runtime.NumGoroutine()
		startHeap    uint64
		maxHeap      uint64
		minHeap      uint64 = 1<<64 - 1
		startTS             = time.Now()
		cycleCount   int
		maxRPC       time.Duration
		totalRPC     time.Duration
		rpcCount     atomic.Int64
	)

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	startHeap = memStats.HeapAlloc
	minHeap = startHeap

	maxG := startG
	minG := startG

	t.Logf("START: goroutines=%d heap=%dKB", startG, startHeap/1024)

	record := func(phase string) {
		runtime.ReadMemStats(&memStats)
		g := runtime.NumGoroutine()
		h := memStats.HeapAlloc
		if g > maxG {
			maxG = g
		}
		if g < minG {
			minG = g
		}
		if h > maxHeap {
			maxHeap = h
		}
		if h < minHeap {
			minHeap = h
		}
		if cycleCount > 0 && cycleCount%50 == 0 {
			t.Logf("[%5.0fs] phase=%s goroutines=%d heap=%dKB started=%d exited=%d active=%d",
				time.Since(startTS).Seconds(), phase, g, h/1024,
				startedCount.Load(), exitedCount.Load(), activeCount.Load())
		}
	}

	for time.Since(startTS) < testDuration {
		cycleCount++
		ctx, cancel := context.WithTimeout(context.Background(), cycleDuration)
		var wg sync.WaitGroup
		for w := 0; w < threads; w++ {
			wg.Add(1)
			startedCount.Add(1)
			activeCount.Add(1)
			go func() {
				defer wg.Done()
				defer func() { exitedCount.Add(1); activeCount.Add(-1) }()
				template, _, err := NewBlockTemplate(chain, mempool.New(), pubHash)
				if err != nil {
					return
				}
				hasher := pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}
				block := *template
				block.Transactions = template.Transactions
				var yieldCounter uint32
				for nonce := uint32(0); ; nonce++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					rpcStart := time.Now()
					block.Header.Nonce = nonce
					hasher.HashHeader(block.Header)
					elapsed := time.Since(rpcStart)
					totalRPC += elapsed
					rpcCount.Add(1)
					if elapsed > maxRPC {
						maxRPC = elapsed
					}
					yieldCounter++
					if yieldCounter&0x3f == 0 {
						runtime.Gosched()
					}
					if nonce > 5000 {
						return
					}
				}
			}()
		}
		wg.Wait()
		cancel()
		time.Sleep(100 * time.Millisecond)
		record("cycle")
	}

	time.Sleep(500 * time.Millisecond)
	runtime.ReadMemStats(&memStats)
	endG := runtime.NumGoroutine()
	endHeap := memStats.HeapAlloc
	endActive := activeCount.Load()
	s := startedCount.Load()
	e := exitedCount.Load()
	rc := rpcCount.Load()

	avgRPC := time.Duration(0)
	if rc > 0 {
		avgRPC = time.Duration(totalRPC.Nanoseconds() / rc)
	}

	t.Logf("FINAL: goroutines(start=%d end=%d min=%d max=%d) heap(start=%dKB end=%dKB min=%dKB max=%dKB)",
		startG, endG, minG, maxG, startHeap/1024, endHeap/1024, minHeap/1024, maxHeap/1024)
	t.Logf("WORKERS: started=%d exited=%d active=%d cycles=%d",
		s, e, endActive, cycleCount)
	t.Logf("RPC-HASH: avg=%v max=%v calls=%d", avgRPC, maxRPC, rc)

	if s != e {
		t.Fatalf("WORKER LEAK: started(%d) != exited(%d)", s, e)
	}
	if endActive != 0 {
		t.Fatalf("ACTIVE WORKERS REMAINING: %d (should be 0)", endActive)
	}
	if endG > startG+threads+10 {
		t.Fatalf("GOROUTINE GROWTH: start=%d end=%d", startG, endG)
	}
}
