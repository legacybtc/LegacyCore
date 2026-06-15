package rpc

import (
	"fmt"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	diagSnapshotActive    atomic.Int64
	diagSnapshotTotal     atomic.Int64
	diagSnapshotMax       atomic.Int64
	diagNodeStatusActive  atomic.Int64
	diagNodeStatusTotal   atomic.Int64
	diagNodeStatusMax     atomic.Int64
	diagPeerInfoActive    atomic.Int64
	diagPeerInfoTotal     atomic.Int64
	diagPeerInfoMax       atomic.Int64
	diagPeerDiagActive    atomic.Int64
	diagPeerDiagTotal     atomic.Int64
	diagPeerDiagMax       atomic.Int64
	diagTimeoutActive     atomic.Int64
	diagTimeoutTotal      atomic.Int64
	diagTimeoutMax        atomic.Int64
	diagFanoutActive      atomic.Int64
	diagFanoutTotal       atomic.Int64
	diagFanoutMax         atomic.Int64
	diagChildWorkersBorn  atomic.Int64
	diagChildWorkersDone  atomic.Int64
	diagChildWorkersMax   atomic.Int64
)

var (
	captured500  int32
	captured1000 int32
	captured2000 int32
	captured2500 int32
)

var diagMu sync.Mutex

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
	diagChildWorkersBorn.Add(1)
	cur := diagFanoutActive.Add(1)
	if cur > diagFanoutMax.Load() {
		diagFanoutMax.Store(cur)
	}
	thresholdCheck()
}

func DoneChildDiag() {
	diagChildWorkersDone.Add(1)
	diagFanoutActive.Add(-1)
}

func DiagCounters() map[string]int64 {
	return map[string]int64{
		"snapshot_active":     diagSnapshotActive.Load(),
		"snapshot_total":      diagSnapshotTotal.Load(),
		"snapshot_max":        diagSnapshotMax.Load(),
		"node_status_active":  diagNodeStatusActive.Load(),
		"node_status_total":   diagNodeStatusTotal.Load(),
		"node_status_max":     diagNodeStatusMax.Load(),
		"peer_info_active":    diagPeerInfoActive.Load(),
		"peer_info_total":     diagPeerInfoTotal.Load(),
		"peer_info_max":       diagPeerInfoMax.Load(),
		"peer_diag_active":    diagPeerDiagActive.Load(),
		"peer_diag_total":     diagPeerDiagTotal.Load(),
		"peer_diag_max":       diagPeerDiagMax.Load(),
		"timeout_active":      diagTimeoutActive.Load(),
		"timeout_total":       diagTimeoutTotal.Load(),
		"timeout_max":         diagTimeoutMax.Load(),
		"fanout_active":       diagFanoutActive.Load(),
		"fanout_total":        diagFanoutTotal.Load(),
		"fanout_max":          diagFanoutMax.Load(),
		"child_workers_born":  diagChildWorkersBorn.Load(),
		"child_workers_done":  diagChildWorkersDone.Load(),
		"child_workers_active": diagChildWorkersBorn.Load() - diagChildWorkersDone.Load(),
		"child_workers_max":   diagChildWorkersMax.Load(),
	}
}

func thresholdCheck() {
	g := runtime.NumGoroutine()
	trigger := false
	if g >= 500 && atomic.CompareAndSwapInt32(&captured500, 0, 1) {
		trigger = true
	} else if g >= 1000 && atomic.CompareAndSwapInt32(&captured1000, 0, 1) {
		trigger = true
	} else if g >= 2000 && atomic.CompareAndSwapInt32(&captured2000, 0, 1) {
		trigger = true
	} else if g >= 2500 && atomic.CompareAndSwapInt32(&captured2500, 0, 1) {
		trigger = true
	}
	if trigger {
		go captureProfiles(g)
	}
}

func captureProfiles(g int) {
	diagMu.Lock()
	defer diagMu.Unlock()

	var gr strings.Builder
	pprof.Lookup("goroutine").WriteTo(&gr, 2)
	grSummary := summarizeStacks(gr.String())

	var tc strings.Builder
	pprof.Lookup("threadcreate").WriteTo(&tc, 2)

	var heap strings.Builder
	pprof.Lookup("heap").WriteTo(&heap, 1)

	fmt.Printf("DIAG_STACK_CAPTURE goroutines=%d\n%s\nthreadcreate=%s\nheap_bytes=%d\n",
		g, grSummary, tc.String()[:min(500, tc.Len())], heap.Len())
}

func summarizeStacks(raw string) string {
	lines := strings.Split(raw, "\n")
	groups := make(map[string]int)
	var currentStack []string
	inStack := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inStack && len(currentStack) > 0 {
				key := currentStack[0]
				groups[key] = groups[key] + 1
				currentStack = nil
			}
			inStack = false
			continue
		}
		if strings.HasPrefix(trimmed, "goroutine ") {
			inStack = true
			currentStack = nil
			continue
		}
		if inStack && strings.Contains(trimmed, ".") {
			currentStack = append(currentStack, trimmed)
		}
	}
	var out strings.Builder
	for stack, count := range groups {
		if count > 5 {
			fmt.Fprintf(&out, "  [%d] %s\n", count, stack)
		}
	}
	if out.Len() == 0 {
		out.WriteString("  (no dominant stacks)\n")
	}
	return out.String()
}

func heapSummary(h *strings.Builder) string {
	return fmt.Sprintf("heap_profile_bytes=%d", h.Len())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
