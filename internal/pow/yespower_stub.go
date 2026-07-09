//go:build !cgo || legacycoin_experimental_pure_yespower

package pow

import (
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

// HashHeader uses the pure-Go yespower implementation only for non-CGO builds
// or explicit experimental/debug builds. Public RC2 mining, pool validation,
// and submitblock-capable binaries must be built with CGO enabled so the C
// yespower reference backend is used instead.
func (h YespowerHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	pers := h.Personalization
	if pers == "" {
		pers = "LegacyCoinPoW"
	}
	return yespowerHash(b, pers), nil
}

func BackendName() string {
	return "pure-go-experimental"
}

func RecordChainContextInit() {}
func RecordChainContextFree() {}
func RecordWorkerContextInit() {}
func RecordWorkerContextFree() {}

func YespowerCounters() map[string]int64 {
	return map[string]int64{
		"worker_contexts_initialized": 0,
		"worker_contexts_freed":       0,
		"worker_contexts_active":      0,
		"chain_contexts_initialized":  0,
		"chain_contexts_freed":        0,
		"chain_contexts_active":       0,
		"total_contexts_active":       0,
		"cgo_calls_active":            0,
		"cgo_calls_max_concurrent":    0,
	}
}
