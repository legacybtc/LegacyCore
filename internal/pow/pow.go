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

// ContextHasher is an optional extension for hashers that support
// explicit per-worker context for repeated mining with no TLS leak.
type ContextHasher interface {
	Hasher
	NewContext() HasherContext
}

// HasherContext wraps per-worker native state for a ContextHasher.
type HasherContext interface {
	Close()
}

// HashWithContext dispatches to a ContextHasher if the hasher supports it,
// falling back to HashHeader otherwise.
func HashWithContext(hasher Hasher, ctx HasherContext, header wire.BlockHeader) (chainhash.Hash, error) {
	type contextHasherImpl interface {
		hashWithContext(HasherContext, wire.BlockHeader) (chainhash.Hash, error)
	}
	if ch, ok := hasher.(contextHasherImpl); ok {
		return ch.hashWithContext(ctx, header)
	}
	return hasher.HashHeader(header)
}

type YespowerHasher struct {
	Personalization string
}
