package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"legacycoin/legacy-go/internal/config"
)

func TestSeedPeersFlagClearsPreviousNoSeedNode(t *testing.T) {
	dir := t.TempDir()
	oldDataDir := os.Getenv("LEGACYCOIN_DATADIR")
	t.Cleanup(func() {
		if oldDataDir == "" {
			_ = os.Unsetenv("LEGACYCOIN_DATADIR")
			return
		}
		_ = os.Setenv("LEGACYCOIN_DATADIR", oldDataDir)
	})
	if err := os.Setenv("LEGACYCOIN_DATADIR", dir); err != nil {
		t.Fatalf("set datadir: %v", err)
	}

	if err := applyRuntimeNodeFlags([]string{"-connect", "127.0.0.1:19555"}); err != nil {
		t.Fatalf("apply connect flag: %v", err)
	}
	path := filepath.Join(dir, config.ConfigFile)
	pol, err := config.LoadPeerPolicy(path)
	if err != nil {
		t.Fatalf("load policy after connect: %v", err)
	}
	if !pol.NoSeedNode || pol.SeedPeers {
		t.Fatalf("connect should disable seed peers before recovery: %+v", pol)
	}

	if err := applyRuntimeNodeFlags([]string{"-seed-peers"}); err != nil {
		t.Fatalf("apply seed-peers flag: %v", err)
	}
	pol, err = config.LoadPeerPolicy(path)
	if err != nil {
		t.Fatalf("load policy after seed-peers: %v", err)
	}
	if pol.NoSeedNode || !pol.SeedPeers {
		t.Fatalf("seed-peers did not re-enable seed discovery: %+v", pol)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(data), "noseednode=false") {
		t.Fatalf("config did not record noseednode=false:\n%s", string(data))
	}
}
