package p2p

import (
	"os"
	"strings"
	"testing"
)

func TestSyncRequestsUseCanonicalBlockHashOnly(t *testing.T) {
	b, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "batch = append(batch, wire.InvVect{Type: wire.InvTypeBlock, Hash: w.legacy})") {
		t.Fatal("sync engine must not request legacy SHA256d hash alongside canonical block hash")
	}
}
