package rpc

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin/legacy-go/internal/mining"
	"legacycoin/legacy-go/internal/pow"
)

var (
	diagSnapshotActive   atomic.Int64
	diagSnapshotTotal    atomic.Int64
	diagSnapshotMax      atomic.Int64
	diagNodeStatusActive atomic.Int64
	diagNodeStatusTotal  atomic.Int64
	diagNodeStatusMax    atomic.Int64
	diagPeerInfoActive   atomic.Int64
	diagPeerInfoTotal    atomic.Int64
	diagPeerInfoMax      atomic.Int64
	diagPeerDiagActive   atomic.Int64
	diagPeerDiagTotal    atomic.Int64
	diagPeerDiagMax      atomic.Int64
	diagTimeoutActive    atomic.Int64
	diagTimeoutTotal     atomic.Int64
	diagTimeoutMax       atomic.Int64
	diagFanoutActive     atomic.Int64
	diagFanoutTotal      atomic.Int64
	diagFanoutMax        atomic.Int64
	diagChildBorn        atomic.Int64
	diagChildDone        atomic.Int64
	diagChildMax         atomic.Int64

	diagBundleCount   int32
	diagLastCaptureAt int64 // unix millis
	diagWarmupDone    int32
	diagGlobalMu      sync.Mutex
	diagPrevBelow     int32
)

const maxBundles = 10
const cooldownMs = 30000

func EnterDiag(active, total, max *atomic.Int64) {
	cur := active.Add(1)
	total.Add(1)
	if cur > max.Load() {
		max.Store(cur)
	}
	thresholdCheck()
}
func LeaveDiag(active *atomic.Int64) { active.Add(-1) }

func SpawnChildDiag() {
	diagChildBorn.Add(1)
	cur := diagFanoutActive.Add(1)
	if cur > diagFanoutMax.Load() {
		diagFanoutMax.Store(cur)
	}
	if cur > diagChildMax.Load() {
		diagChildMax.Store(cur)
	}
	thresholdCheck()
}
func DoneChildDiag() { diagChildDone.Add(1); diagFanoutActive.Add(-1) }

func DiagCounters() map[string]int64 {
	return map[string]int64{
		"snapshot_active": diagSnapshotActive.Load(), "snapshot_total": diagSnapshotTotal.Load(), "snapshot_max": diagSnapshotMax.Load(),
		"node_status_active": diagNodeStatusActive.Load(), "node_status_total": diagNodeStatusTotal.Load(), "node_status_max": diagNodeStatusMax.Load(),
		"peer_info_active": diagPeerInfoActive.Load(), "peer_info_total": diagPeerInfoTotal.Load(), "peer_info_max": diagPeerInfoMax.Load(),
		"peer_diag_active": diagPeerDiagActive.Load(), "peer_diag_total": diagPeerDiagTotal.Load(), "peer_diag_max": diagPeerDiagMax.Load(),
		"timeout_active": diagTimeoutActive.Load(), "timeout_total": diagTimeoutTotal.Load(), "timeout_max": diagTimeoutMax.Load(),
		"fanout_active": diagFanoutActive.Load(), "fanout_total": diagFanoutTotal.Load(), "fanout_max": diagFanoutMax.Load(),
		"child_workers_born": diagChildBorn.Load(), "child_workers_done": diagChildDone.Load(),
		"child_workers_active": diagChildBorn.Load() - diagChildDone.Load(), "child_workers_max": diagChildMax.Load(),
	}
}

func thresholdCheck() {
	if atomic.LoadInt32(&diagWarmupDone) == 0 {
		return
	}
	g := runtime.NumGoroutine()
	prev := atomic.LoadInt32(&diagPrevBelow)
	var trigger string
	if g >= 2500 && prev < 1500 {
		trigger = "g2500"
	} else if g >= 2000 && prev < 1000 {
		trigger = "g2000"
	} else if g >= 1000 && prev < 500 {
		trigger = "g1000"
	} else if g >= 500 && prev < 300 {
		trigger = "g500"
	}
	if g < 300 {
		atomic.StoreInt32(&diagPrevBelow, int32(g))
	}
	if trigger != "" {
		_ = triggerCapture(trigger, "")
	}
}

func ManualDiagnosticCapture(reason string) map[string]any {
	err := triggerCapture("manual", reason)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}
	}
	return map[string]any{"ok": true, "bundle_count": atomic.LoadInt32(&diagBundleCount)}
}

func triggerCapture(trigger, reason string) error {
	now := time.Now().UnixMilli()
	last := atomic.LoadInt64(&diagLastCaptureAt)
	if now-last < cooldownMs {
		return fmt.Errorf("cooldown: %dms remaining", cooldownMs-(now-last))
	}
	if atomic.LoadInt32(&diagBundleCount) >= maxBundles {
		return fmt.Errorf("max bundles (%d) reached", maxBundles)
	}
	atomic.StoreInt64(&diagLastCaptureAt, now)
	go writeBundle(trigger, reason, now)
	return nil
}

func SetDiagWarmupDone() { atomic.StoreInt32(&diagWarmupDone, 1) }

func writeBundle(trigger, reason string, nowMs int64) {
	diagGlobalMu.Lock()
	defer diagGlobalMu.Unlock()

	ts := time.UnixMilli(nowMs)
	dir := filepath.Join(diagBaseDir(), fmt.Sprintf("%s-%s", ts.Format("20060102-150405"), trigger))
	_ = os.MkdirAll(dir, 0700)

	writePprof(dir, "goroutines-debug2.txt", "goroutine", 2)
	writePprof(dir, "threadcreate-debug2.txt", "threadcreate", 2)
	writePprofBinary(dir, "heap.pprof", "heap")
	writePprofBinary(dir, "allocs.pprof", "allocs")

	ms := new(runtime.MemStats)
	runtime.ReadMemStats(ms)
	b, _ := json.MarshalIndent(memstatsMap(ms), "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "memstats.json"), b, 0644)

	counters := DiagCounters()
	for k, v := range mining.LifecycleCounters() {
		counters["mining_"+k] = v
	}
	for k, v := range pow.YespowerCounters() {
		counters["yespower_"+k] = v
	}
	counters["trigger"] = 0
	b, _ = json.MarshalIndent(counters, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "counters.json"), b, 0644)

	summary := fmt.Sprintf("trigger=%s reason=%s goroutines=%d bundle=%d\n",
		trigger, reason, runtime.NumGoroutine(), atomic.LoadInt32(&diagBundleCount))
	_ = os.WriteFile(filepath.Join(dir, "summary.txt"), []byte(summary), 0644)

	atomic.AddInt32(&diagBundleCount, 1)
}

func writePprof(dir, name, profile string, debug int) {
	var buf strings.Builder
	if p := pprof.Lookup(profile); p != nil {
		p.WriteTo(&buf, debug)
	}
	if buf.Len() > 0 {
		_ = os.WriteFile(filepath.Join(dir, name), []byte(buf.String()), 0644)
	}
}

func writePprofBinary(dir, name, profile string) {
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return
	}
	defer f.Close()
	if p := pprof.Lookup(profile); p != nil {
		p.WriteTo(f, 0)
	}
}

func memstatsMap(ms *runtime.MemStats) map[string]uint64 {
	return map[string]uint64{
		"HeapAlloc": ms.HeapAlloc, "HeapInuse": ms.HeapInuse, "HeapIdle": ms.HeapIdle, "HeapReleased": ms.HeapReleased,
		"StackInuse": ms.StackInuse, "StackSys": ms.StackSys, "MSpanInuse": ms.MSpanInuse, "MCacheInuse": ms.MCacheInuse,
		"BuckHashSys": ms.BuckHashSys, "GCSys": ms.GCSys, "OtherSys": ms.OtherSys, "Sys": ms.Sys,
		"NumGC": uint64(ms.NumGC), "NextGC": ms.NextGC,
	}
}

func diagBaseDir() string {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, "LegacyCoin", "diagnostics")
}
