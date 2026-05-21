package pow

import (
	"errors"

	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

var ErrYespowerUnavailable = errors.New("yespower hash is not implemented yet")

type Hasher interface {
	HashHeader(header wire.BlockHeader) (chainhash.Hash, error)
}

type YespowerHasher struct {
	Personalization string
}
