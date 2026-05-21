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
