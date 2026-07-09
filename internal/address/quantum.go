package address

import (
	"crypto/sha256"
	"errors"
	"strings"
)

const (
	HybridPrefix  = "lhyb1"
	QuantumPrefix = "lpq1"
	VaultPrefix   = "lval1"

	HybridVersion  byte = 0x51
	QuantumVersion byte = 0x52
	VaultVersion   byte = 0x53
	HashSize            = 20
)

var (
	ErrBadQuantumPrefix = errors.New("bad quantum address prefix")
	ErrBadPayloadSize   = errors.New("bad quantum address payload size")
)

func NewHybridAddress(classicalPub, pqPub []byte) string {
	return encodeQuantumAddress(HybridPrefix, HybridVersion, hybridHash(classicalPub, pqPub))
}

func DecodeHybridAddress(addr string) ([]byte, error) {
	version, payload, err := decodeQuantumAddress(addr, HybridPrefix)
	if err != nil {
		return nil, err
	}
	if version != HybridVersion {
		return nil, ErrBadQuantumPrefix
	}
	return payload, nil
}

func hybridHash(classicalPub, pqPub []byte) []byte {
	h := sha256.New()
	_, _ = h.Write([]byte("LegacyCoin hybrid ML-DSA-65 address v1"))
	_, _ = h.Write(classicalPub)
	_, _ = h.Write(pqPub)
	sum := h.Sum(nil)
	return sum[:HashSize]
}

func encodeQuantumAddress(prefix string, version byte, payload []byte) string {
	return prefix + EncodeBase58Check(version, payload)
}

func decodeQuantumAddress(addr string, prefix string) (byte, []byte, error) {
	if !strings.HasPrefix(addr, prefix) {
		return 0, nil, ErrBadQuantumPrefix
	}
	version, payload, err := DecodeBase58Check(strings.TrimPrefix(addr, prefix))
	if err != nil {
		return 0, nil, err
	}
	if len(payload) != HashSize {
		return 0, nil, ErrBadPayloadSize
	}
	return version, payload, nil
}
