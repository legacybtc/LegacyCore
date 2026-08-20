package p2p

import "testing"

func TestPeerPrefersLegacyWireHashForLegacyGO010(t *testing.T) {
	legacy := &peer{subver: "/Legacy-GO:0.1.0/"}
	modern := &peer{subver: "/Legacy-GO:1.0.36/"}
	if !peerPrefersLegacyWireHash(legacy) {
		t.Fatal("Legacy-GO 0.1.0 peer must use legacy wire block hashes")
	}
	if peerPrefersLegacyWireHash(modern) {
		t.Fatal("modern peer must remain on canonical wire block hashes")
	}
}
