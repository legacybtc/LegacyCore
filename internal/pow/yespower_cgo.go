//go:build cgo && !legacycoin_experimental_pure_yespower

package pow

/*
#cgo CFLAGS: -I${SRCDIR}/yespower -O3
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "yespower/yespower.h"
#include "yespower/sha256.c"
#include "yespower/yespower-opt.c"

static int legacy_yespower_hash_with_local(
	yespower_local_t *local,
	const unsigned char* input, size_t inputlen,
	const char* pers, size_t perslen,
	unsigned char* output)
{
	yespower_binary_t dst;
	yespower_params_t params = {YESPOWER_1_0, 2048, 32,
		(const uint8_t*)pers, perslen};
	if (yespower(local, (const uint8_t*)input, inputlen,
		&params, &dst) != 0) {
		memset(output, 0xff, 32);
		return -1;
	}
	memcpy(output, dst.uc, 32);
	return 0;
}

static int legacy_yespower_init_local(yespower_local_t *local) {
	yespower_init_local(local);
	return 0;
}

static int legacy_yespower_free_local(yespower_local_t *local) {
	return yespower_free_local(local);
}

// TLS-based hash (kept for backward compat / non-mining callers)
static int legacy_yespower_hash(
	const unsigned char* input, size_t inputlen,
	const char* pers, size_t perslen,
	unsigned char* output)
{
	yespower_binary_t dst;
	yespower_params_t params = {YESPOWER_1_0, 2048, 32,
		(const uint8_t*)pers, perslen};
	if (yespower_tls((const uint8_t*)input, inputlen,
		&params, &dst) != 0) {
		memset(output, 0xff, 32);
		return -1;
	}
	memcpy(output, dst.uc, 32);
	return 0;
}
*/
import "C"

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

var (
	localInit   atomic.Int64
	localFree   atomic.Int64
	cgoActive   atomic.Int64
	cgoMax      atomic.Int64
)

type yespowerContext struct {
	local  *C.yespower_local_t
	hasher YespowerHasher
	mu     sync.Mutex
	freed  int32
}

// NewContext creates a per-worker context with native scratch memory.
func (h YespowerHasher) NewContext() HasherContext {
	var local C.yespower_local_t
	C.legacy_yespower_init_local(&local)
	localInit.Add(1)
	return &yespowerContext{
		local:  &local,
		hasher: h,
	}
}

func (c *yespowerContext) Close() {
	if c == nil || atomic.LoadInt32(&c.freed) == 1 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if atomic.LoadInt32(&c.freed) == 1 || c.local == nil {
		return
	}
	atomic.StoreInt32(&c.freed, 1)
	C.legacy_yespower_free_local(c.local)
	localFree.Add(1)
	c.local = nil
}

func (h YespowerHasher) hashWithContext(ctx HasherContext, header wire.BlockHeader) (chainhash.Hash, error) {
	c, ok := ctx.(*yespowerContext)
	if !ok || c == nil || atomic.LoadInt32(&c.freed) == 1 {
		return h.HashHeader(header)
	}
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	pers := h.Personalization
	if pers == "" {
		pers = "LegacyCoinPoW"
	}
	cp := C.CString(pers)
	defer C.free(unsafe.Pointer(cp))

	cgoActive.Add(1)
	if cur := cgoActive.Load(); cur > cgoMax.Load() {
		cgoMax.Store(cur)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if atomic.LoadInt32(&c.freed) == 1 {
		cgoActive.Add(-1)
		return h.HashHeader(header)
	}

	var out chainhash.Hash
	inPtr := (*C.uchar)(unsafe.Pointer(&b[0]))
	outPtr := (*C.uchar)(unsafe.Pointer(&out[0]))
	rc := C.legacy_yespower_hash_with_local(
		c.local, inPtr, C.size_t(len(b)),
		cp, C.size_t(len(pers)), outPtr)

	cgoActive.Add(-1)
	if rc != 0 {
		return chainhash.Hash{}, ErrYespowerUnavailable
	}
	return out, nil
}

func (h YespowerHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	pers := h.Personalization
	if pers == "" {
		pers = "LegacyCoinPoW"
	}
	cp := C.CString(pers)
	defer C.free(unsafe.Pointer(cp))

	var out chainhash.Hash
	inPtr := (*C.uchar)(unsafe.Pointer(&b[0]))
	outPtr := (*C.uchar)(unsafe.Pointer(&out[0]))
	rc := C.legacy_yespower_hash(inPtr, C.size_t(len(b)), cp, C.size_t(len(pers)), outPtr)
	if rc != 0 {
		return chainhash.Hash{}, ErrYespowerUnavailable
	}
	return out, nil
}

func YespowerCounters() map[string]int64 {
	return map[string]int64{
		"init":   localInit.Load(),
		"free":   localFree.Load(),
		"active": localInit.Load() - localFree.Load(),
		"cgo":    cgoActive.Load(),
		"cgo_max": cgoMax.Load(),
	}
}

func BackendName() string { return "cgo-c-reference" }
