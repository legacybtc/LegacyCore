package mining

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/storage"
)

func TestMinerLifecycleResourceStability(t *testing.T) {
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
	cycles := 50
	pubHash := make([]byte, 20)

	var (
		maxGoroutines int
		started       int64
		exited        int64
		startG        int
	)

	startG = runtime.NumGoroutine()
	t.Logf("start: goroutines=%d", startG)

	for cycle := 0; cycle < cycles; cycle++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
		var wg sync.WaitGroup
		for w := 0; w < threads; w++ {
			wg.Add(1)
			started++
			go func() {
				defer wg.Done()
				defer func() { exited++ }()
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				template, _, err := NewBlockTemplate(chain, mempool.New(), pubHash)
				if err != nil {
					return
				}
				hasher := pow.YespowerHasher{Personalization: chaincfg.MainNet.YespowerPers}
				block := *template
				block.Transactions = template.Transactions
				for nonce := uint32(0); ; nonce++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					block.Header.Nonce = nonce
					hasher.HashHeader(block.Header)
					if nonce > 8000 {
						return
					}
				}
			}()
		}
		wg.Wait()
		cancel()

		if g := runtime.NumGoroutine(); g > maxGoroutines {
			maxGoroutines = g
		}
		if cycle == 0 || cycle == cycles/2 || cycle == cycles-1 {
			t.Logf("cycle %3d: goroutines=%d started=%d exited=%d",
				cycle, runtime.NumGoroutine(), started, exited)
		}
	}
	time.Sleep(500 * time.Millisecond)

	endG := runtime.NumGoroutine()
	t.Logf("end: goroutines=%d (start=%d max=%d) started=%d exited=%d",
		endG, startG, maxGoroutines, started, exited)

	if started != exited {
		t.Fatalf("started(%d) != exited(%d) — worker goroutine leak", started, exited)
	}
	if endG > startG+threads+5 {
		t.Fatalf("goroutines grew: start=%d end=%d max=%d", startG, endG, maxGoroutines)
	}
}
