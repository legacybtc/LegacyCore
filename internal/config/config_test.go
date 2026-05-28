package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLoadAddNodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
# sample config
rpcuser=user
addnode=legacycoinseed.space
addnode=192.0.2.10:19555
addnode=legacycoinseed.space # duplicate
addnode=
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	got, err := LoadAddNodes(path)
	if err != nil {
		t.Fatalf("LoadAddNodes: %v", err)
	}
	want := []string{"legacycoinseed.space", "192.0.2.10:19555"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nodes=%v want=%v", got, want)
	}
}

func TestLoadRPCAuth(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
rpcuser=alice
rpcpassword=secret123
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	auth, err := LoadRPCAuth(path)
	if err != nil {
		t.Fatalf("LoadRPCAuth: %v", err)
	}
	if !auth.Enabled || auth.User != "alice" || auth.Password != "secret123" {
		t.Fatalf("auth=%+v", auth)
	}
}

func TestEnsureAndLoadRPCCookieForDataDir(t *testing.T) {
	dir := t.TempDir()
	auth, err := EnsureRPCCookieForDataDir(dir)
	if err != nil {
		t.Fatalf("EnsureRPCCookieForDataDir: %v", err)
	}
	if !auth.Enabled || auth.User != "__cookie__" || auth.Password == "" {
		t.Fatalf("auth=%+v", auth)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cookie")); err != nil {
		t.Fatalf("cookie not created: %v", err)
	}
	loaded, err := LoadRPCCookieForDataDir(dir)
	if err != nil {
		t.Fatalf("LoadRPCCookieForDataDir: %v", err)
	}
	if loaded.User != auth.User || loaded.Password != auth.Password {
		t.Fatalf("loaded=%+v auth=%+v", loaded, auth)
	}
}

func TestDefaultDataDirPlatformName(t *testing.T) {
	old := os.Getenv("LEGACYCOIN_DATADIR")
	t.Cleanup(func() { _ = os.Setenv("LEGACYCOIN_DATADIR", old) })
	_ = os.Unsetenv("LEGACYCOIN_DATADIR")
	got := DefaultDataDir()
	switch runtime.GOOS {
	case "windows":
		if !strings.HasSuffix(got, filepath.Join("LegacyCoin")) {
			t.Fatalf("windows datadir=%q", got)
		}
	case "darwin":
		if !strings.Contains(got, filepath.Join("Application Support", "LegacyCoin")) {
			t.Fatalf("darwin datadir=%q", got)
		}
	default:
		if !strings.HasSuffix(got, ".legacycoin") {
			t.Fatalf("linux datadir=%q", got)
		}
	}
}

func TestLoadRPCBind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
rpcbind=0.0.0.0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	bind, err := LoadRPCBind(path)
	if err != nil {
		t.Fatalf("LoadRPCBind: %v", err)
	}
	if bind.Host != "0.0.0.0" {
		t.Fatalf("host=%q", bind.Host)
	}
}

func TestLoadRPCBindDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
rpcuser=alice
rpcpassword=secret123
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	bind, err := LoadRPCBind(path)
	if err != nil {
		t.Fatalf("LoadRPCBind: %v", err)
	}
	if bind.Host != "127.0.0.1" {
		t.Fatalf("default host=%q", bind.Host)
	}
}

func TestLoadP2PBind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
bind=0.0.0.0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	bind, err := LoadP2PBind(path)
	if err != nil {
		t.Fatalf("LoadP2PBind: %v", err)
	}
	if bind.Host != "0.0.0.0" {
		t.Fatalf("host=%q", bind.Host)
	}
}

func TestLoadInteropReference(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
interop_check=1
interop_genesis_hash=5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5
interop_message_start=a4acc64d
interop_p2p_port=19555
interop_rpc_port=19556
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	ref, err := LoadInteropReference(path)
	if err != nil {
		t.Fatalf("LoadInteropReference: %v", err)
	}
	if !ref.Enabled {
		t.Fatal("interop ref should be enabled")
	}
	if ref.GenesisHash == "" || ref.MessageStart == "" {
		t.Fatalf("interop ref missing values: %+v", ref)
	}
	if ref.P2PPort != 19555 || ref.RPCPort != 19556 {
		t.Fatalf("interop ports unexpected: %+v", ref)
	}
}

func TestLoadPeerPolicyAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacycoin.conf")
	content := `
peer_ban_score_threshold=140
peer_ban_minutes=17
peer_max_inbound=71
peer_rate_limit=333
peer_reconnect_backoff_seconds=21
peer_max_per_ip=4
peer_max_per_subnet=12
peer_global_rate_limit=900
peer_misbehavior_decay_seconds=44
peer_stale_timeout_seconds=500
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	pol, err := LoadPeerPolicy(path)
	if err != nil {
		t.Fatalf("LoadPeerPolicy: %v", err)
	}
	if pol.BanThreshold != 140 {
		t.Fatalf("BanThreshold=%d want 140", pol.BanThreshold)
	}
	if pol.TemporaryBanSeconds != 17*60 {
		t.Fatalf("TemporaryBanSeconds=%d want %d", pol.TemporaryBanSeconds, 17*60)
	}
	if pol.MaxInboundPeers != 71 {
		t.Fatalf("MaxInboundPeers=%d want 71", pol.MaxInboundPeers)
	}
	if pol.PeerRateLimit != 333 {
		t.Fatalf("PeerRateLimit=%d want 333", pol.PeerRateLimit)
	}
	if pol.ReconnectBackoffSeconds != 21 {
		t.Fatalf("ReconnectBackoffSeconds=%d want 21", pol.ReconnectBackoffSeconds)
	}
	if pol.MaxPerIP != 4 {
		t.Fatalf("MaxPerIP=%d want 4", pol.MaxPerIP)
	}
	if pol.MaxPerSubnet != 12 {
		t.Fatalf("MaxPerSubnet=%d want 12", pol.MaxPerSubnet)
	}
	if pol.GlobalRateLimit != 900 {
		t.Fatalf("GlobalRateLimit=%d want 900", pol.GlobalRateLimit)
	}
	if pol.MisbehaviorDecaySeconds != 44 {
		t.Fatalf("MisbehaviorDecaySeconds=%d want 44", pol.MisbehaviorDecaySeconds)
	}
	if pol.StaleTimeoutSeconds != 500 {
		t.Fatalf("StaleTimeoutSeconds=%d want 500", pol.StaleTimeoutSeconds)
	}
}
