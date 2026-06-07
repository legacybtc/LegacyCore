package node

import (
	"testing"

	"legacycoin/legacy-go/internal/config"
)

func TestApplySeedNodePeerDefaults(t *testing.T) {
	pol := applySeedNodePeerDefaults(config.PeerPolicy{})
	if pol.MaxInboundPeers < 512 {
		t.Fatalf("MaxInboundPeers=%d want >=512", pol.MaxInboundPeers)
	}
	if pol.MaxPerIP < 32 || pol.MaxPerSubnet < 128 {
		t.Fatalf("seed peer caps too low: %+v", pol)
	}
	if !pol.SeedPeers || pol.NoSeedNode || !pol.PeerSafety {
		t.Fatalf("seed flags not enforced: %+v", pol)
	}
}
