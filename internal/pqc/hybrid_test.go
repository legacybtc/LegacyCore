package pqc

import "testing"

func TestHybridSignVerify(t *testing.T) {
	key, err := GenerateHybridKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := key.Public()
	msg := []byte("Legacy Coin quantum-safe wallet smoke test")
	sig, err := key.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Verify(msg, sig) {
		t.Fatal("hybrid signature did not verify")
	}
	if pub.Verify([]byte("tampered"), sig) {
		t.Fatal("hybrid signature verified for tampered message")
	}
	if err := pub.VerifyAddress(pub.Address()); err != nil {
		t.Fatal(err)
	}
}

func TestHybridKeyRoundTrip(t *testing.T) {
	key, err := GenerateHybridKey()
	if err != nil {
		t.Fatal(err)
	}
	restored, err := HybridPrivateKeyFromBytes(key.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if restored.Public().Address() != key.Public().Address() {
		t.Fatal("restored key address mismatch")
	}
	pub, err := HybridPublicKeyFromBytes(key.Public().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if pub.Address() != key.Public().Address() {
		t.Fatal("restored public key address mismatch")
	}
}
