package pqc

import (
	"crypto/sha256"
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/cloudflare/circl/sign/mldsa/mldsa65"

	"legacycoin/legacy-go/internal/address"
)

const (
	HybridAlgorithm    = "hybrid-secp256k1-ecdsa+mldsa65"
	MLDSAContext       = "LegacyCoin/PQC/ML-DSA-65/v1"
	MLDSAPublicKeySize = mldsa65.PublicKeySize
	MLDSASignatureSize = mldsa65.SignatureSize
)

var (
	ErrBadMLDSAPublicKey  = errors.New("bad ML-DSA-65 public key")
	ErrBadMLDSAPrivateKey = errors.New("bad ML-DSA-65 private key")
	ErrBadSecpPublicKey   = errors.New("bad secp256k1 public key")
	ErrBadSecpPrivateKey  = errors.New("bad secp256k1 private key")
	ErrBadECDSASignature  = errors.New("bad secp256k1 signature")
	ErrWrongAddress       = errors.New("hybrid address does not match public keys")
)

type HybridPrivateKey struct {
	Secp *btcec.PrivateKey
	PQ   *mldsa65.PrivateKey
}

type HybridPublicKey struct {
	Secp *btcec.PublicKey
	PQ   *mldsa65.PublicKey
}

type HybridPublicBytes struct {
	SecpCompressed []byte `json:"secp256k1_compressed"`
	MLDSA65        []byte `json:"mldsa65"`
}

type HybridPrivateBytes struct {
	Secp32  []byte `json:"secp256k1"`
	MLDSA65 []byte `json:"mldsa65"`
}

type HybridSignature struct {
	ECDSADER []byte `json:"ecdsa_der"`
	MLDSA65  []byte `json:"mldsa65"`
}

func GenerateHybridKey() (*HybridPrivateKey, error) {
	secp, err := btcec.NewPrivateKey()
	if err != nil {
		return nil, err
	}
	_, pqPriv, err := mldsa65.GenerateKey(nil)
	if err != nil {
		return nil, err
	}
	return &HybridPrivateKey{Secp: secp, PQ: pqPriv}, nil
}

func HybridPrivateKeyFromBytes(raw HybridPrivateBytes) (*HybridPrivateKey, error) {
	if len(raw.Secp32) != btcec.PrivKeyBytesLen {
		return nil, ErrBadSecpPrivateKey
	}
	if len(raw.MLDSA65) != mldsa65.PrivateKeySize {
		return nil, ErrBadMLDSAPrivateKey
	}
	secp, _ := btcec.PrivKeyFromBytes(raw.Secp32)
	var pq mldsa65.PrivateKey
	if err := pq.UnmarshalBinary(raw.MLDSA65); err != nil {
		return nil, err
	}
	return &HybridPrivateKey{Secp: secp, PQ: &pq}, nil
}

func HybridPublicKeyFromBytes(raw HybridPublicBytes) (*HybridPublicKey, error) {
	secp, err := btcec.ParsePubKey(raw.SecpCompressed)
	if err != nil {
		return nil, ErrBadSecpPublicKey
	}
	var pq mldsa65.PublicKey
	if err := pq.UnmarshalBinary(raw.MLDSA65); err != nil {
		return nil, err
	}
	return &HybridPublicKey{Secp: secp, PQ: &pq}, nil
}

func (k *HybridPrivateKey) Public() *HybridPublicKey {
	return &HybridPublicKey{
		Secp: k.Secp.PubKey(),
		PQ:   k.PQ.Public().(*mldsa65.PublicKey),
	}
}

func (k *HybridPrivateKey) Bytes() HybridPrivateBytes {
	return HybridPrivateBytes{
		Secp32:  k.Secp.Serialize(),
		MLDSA65: k.PQ.Bytes(),
	}
}

func (k *HybridPublicKey) Bytes() HybridPublicBytes {
	return HybridPublicBytes{
		SecpCompressed: k.Secp.SerializeCompressed(),
		MLDSA65:        k.PQ.Bytes(),
	}
}

func (k *HybridPublicKey) Address() string {
	raw := k.Bytes()
	return address.NewHybridAddress(raw.SecpCompressed, raw.MLDSA65)
}

func (k *HybridPrivateKey) Sign(message []byte) (HybridSignature, error) {
	digest := hybridDigest(message)
	ecdsaSig := btcecdsa.Sign(k.Secp, digest[:]).Serialize()
	pqSig := make([]byte, mldsa65.SignatureSize)
	if err := mldsa65.SignTo(k.PQ, message, []byte(MLDSAContext), true, pqSig); err != nil {
		return HybridSignature{}, err
	}
	return HybridSignature{ECDSADER: ecdsaSig, MLDSA65: pqSig}, nil
}

func (k *HybridPublicKey) Verify(message []byte, sig HybridSignature) bool {
	if len(sig.MLDSA65) != mldsa65.SignatureSize {
		return false
	}
	ecdsaSig, err := btcecdsa.ParseDERSignature(sig.ECDSADER)
	if err != nil {
		return false
	}
	digest := hybridDigest(message)
	if !ecdsaSig.Verify(digest[:], k.Secp) {
		return false
	}
	return mldsa65.Verify(k.PQ, message, []byte(MLDSAContext), sig.MLDSA65)
}

func (k *HybridPublicKey) VerifyAddress(addr string) error {
	raw := k.Bytes()
	want := address.NewHybridAddress(raw.SecpCompressed, raw.MLDSA65)
	if addr != want {
		return ErrWrongAddress
	}
	return nil
}

func hybridDigest(message []byte) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("LegacyCoin hybrid secp256k1 digest v1"))
	_, _ = h.Write(message)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
