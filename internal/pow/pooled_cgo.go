//go:build cgo && !legacycoin_experimental_pure_yespower

package pow

import (
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

func (p *PooledYespower) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	_ = p.scratch
	return p.hasher.HashHeader(header)
}
