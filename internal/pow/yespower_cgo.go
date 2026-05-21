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

static int legacy_yespower_hash(const unsigned char* input, size_t inputlen, const char* pers, size_t perslen, unsigned char* output) {
	yespower_binary_t dst;
	yespower_params_t params = {YESPOWER_1_0, 2048, 32, (const uint8_t*)pers, perslen};
	if (yespower_tls((const uint8_t*)input, inputlen, &params, &dst) != 0) {
		memset(output, 0xff, 32);
		return -1;
	}
	memcpy(output, dst.uc, 32);
	return 0;
}
*/
import "C"

import (
	"unsafe"

	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

// HashHeader uses the bundled C yespower 1.0 reference implementation for
// production CGO builds. This is the RC2 public mining path.
func (h YespowerHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	pers := h.Personalization
	if pers == "" {
		pers = "LegacyCoinPoW"
	}
	cPers := C.CString(pers)
	defer C.free(unsafe.Pointer(cPers))

	var out chainhash.Hash
	inPtr := (*C.uchar)(unsafe.Pointer(&b[0]))
	outPtr := (*C.uchar)(unsafe.Pointer(&out[0]))
	rc := C.legacy_yespower_hash(inPtr, C.size_t(len(b)), cPers, C.size_t(len(pers)), outPtr)
	if rc != 0 {
		return chainhash.Hash{}, ErrYespowerUnavailable
	}
	return out, nil
}

func BackendName() string {
	return "cgo-c-reference"
}
