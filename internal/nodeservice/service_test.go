package nodeservice

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/node"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/wallet"
)

func requireProductionYespower(t *testing.T) {
	t.Helper()
	if pow.BackendName() != "cgo-c-reference" {
		t.Skipf("requires production yespower backend, got %q", pow.BackendName())
	}
}

func TestDefaultDataDirUnchangedWithoutEnv(t *testing.T) {
	t.Setenv("LEGACYCOIN_DATADIR", "")
	got := config.DefaultDataDir()
	if got == "" {
		t.Fatal("default data dir is empty")
	}
	if strings.Contains(got, "LEGACYCOIN_DATADIR") {
		t.Fatalf("default data dir looks env-derived: %q", got)
	}
}

func TestNewWithDataDirUsesCustomPathWithoutEnvMutation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LEGACYCOIN_DATADIR", filepath.Join(t.TempDir(), "sentinel"))
	before := os.Getenv("LEGACYCOIN_DATADIR")
	n, err := node.NewWithDataDir(dir)
	if err != nil {
		t.Fatalf("NewWithDataDir: %v", err)
	}
	if after := os.Getenv("LEGACYCOIN_DATADIR"); after != before {
		t.Fatalf("LEGACYCOIN_DATADIR mutated: before=%q after=%q", before, after)
	}
	if n.RuntimePaths().DataDir != dir {
		t.Fatalf("node data dir = %q, want %q", n.RuntimePaths().DataDir, dir)
	}
	if _, err := os.Stat(filepath.Join(dir, "wallet.json")); err != nil {
		t.Fatalf("wallet not created in custom data dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cookie")); err != nil {
		t.Fatalf("rpc cookie not created in custom data dir: %v", err)
	}
}

func TestPortConflictReturnsCleanError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:19556")
	if err != nil {
		t.Skipf("rpc port already unavailable in test environment: %v", err)
	}
	defer ln.Close()
	s := New(t.TempDir())
	err = s.Start()
	if err == nil {
		s.Stop()
		t.Fatal("expected port conflict error")
	}
	if !strings.Contains(err.Error(), "already using the required port") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSupplyInfoFromHeight(t *testing.T) {
	info := supplyInfoFromHeight(230)
	if got := info["max_supply_base_units"]; got != chaincfg.MaxMoney {
		t.Fatalf("max supply = %v, want %v", got, chaincfg.MaxMoney)
	}
	if got := info["total_issued_base_units"]; got != int64(231)*chaincfg.Subsidy {
		t.Fatalf("total issued = %v, want %v", got, int64(231)*chaincfg.Subsidy)
	}
	if got := info["matured_supply_base_units"]; got != int64(132)*chaincfg.Subsidy {
		t.Fatalf("matured supply = %v, want %v", got, int64(132)*chaincfg.Subsidy)
	}
	if got := info["immature_supply_base_units"]; got != int64(99)*chaincfg.Subsidy {
		t.Fatalf("immature supply = %v, want %v", got, int64(99)*chaincfg.Subsidy)
	}
	if got := info["next_halving_height"]; got != int64(chaincfg.HalvingInterval) {
		t.Fatalf("next halving = %v, want %v", got, chaincfg.HalvingInterval)
	}
	if got := info["blocks_until_halving"]; got != int64(chaincfg.HalvingInterval-230) {
		t.Fatalf("blocks until halving = %v, want %v", got, chaincfg.HalvingInterval-230)
	}
}

func TestWalletTxToMapSelfTransfer(t *testing.T) {
	row := walletTxToMap(walletTxRecord{
		TxID:      strings.Repeat("a", 64),
		Direction: "self_transfer",
		Status:    "pending",
		Amount:    12_990_00000,
		Fee:       1_000,
		Change:    12_990_00000,
		Mempool:   true,
	})
	if got := row["direction"]; got != "self_transfer" {
		t.Fatalf("direction=%v want self_transfer", got)
	}
	if got := row["status_label"]; got != "Pending confirmation" {
		t.Fatalf("status_label=%v", got)
	}
	if got := row["change"]; got != int64(12_990_00000) {
		t.Fatalf("change=%v", got)
	}
}

func TestClassifyWalletSpendSeparatesChangeFromSelfTransfer(t *testing.T) {
	dir, amt := classifyWalletSpend(11_999_00000, 1_00000000)
	if dir != "sent" || amt != 1_00000000 {
		t.Fatalf("external spend with wallet change classified as %s amount=%d", dir, amt)
	}
	dir, amt = classifyWalletSpend(13_00000000, 0)
	if dir != "self_transfer" || amt != 13_00000000 {
		t.Fatalf("wallet-only spend classified as %s amount=%d", dir, amt)
	}
}

func TestSetDefaultMiningAddressWithoutNode(t *testing.T) {
	dir := t.TempDir()
	w, err := wallet.Open(dir)
	if err != nil {
		t.Fatalf("wallet.Open: %v", err)
	}
	addr, err := w.NewAddress()
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	s := New(dir)
	out, err := s.SetDefaultMiningAddress(addr)
	if err != nil {
		t.Fatalf("SetDefaultMiningAddress: %v", err)
	}
	if got := s.defaultMiningAddress(); got != addr {
		t.Fatalf("default mining address=%q want=%q", got, addr)
	}
	if got := out["default_mining_address"]; got != addr {
		t.Fatalf("returned default mining address=%v want=%v", got, addr)
	}
	cfg, err := config.LoadMiningConfig(filepath.Join(dir, config.ConfigFile))
	if err != nil {
		t.Fatalf("LoadMiningConfig: %v", err)
	}
	if cfg.RewardAddress != addr || cfg.PubKeyHash == "" {
		t.Fatalf("mining config not written: %+v", cfg)
	}
}

func TestSetDefaultMiningAddressRejectsUnowned(t *testing.T) {
	dir := t.TempDir()
	if _, err := wallet.Open(dir); err != nil {
		t.Fatalf("wallet.Open: %v", err)
	}
	s := New(dir)
	addr := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, bytes.Repeat([]byte{0x11}, 20))
	if _, err := s.SetDefaultMiningAddress(addr); err == nil {
		t.Fatal("expected unowned mining address error")
	}
}

func TestSetDefaultMiningAddressRejectsInvalid(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.SetDefaultMiningAddress("not-an-address"); err == nil {
		t.Fatal("expected invalid mining address error")
	}
}

func TestEnableAddressAndTxIndexConfigWritesConfig(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	out := s.EnableAddressAndTxIndexConfig()
	if ok, _ := out["ok"].(bool); !ok {
		t.Fatalf("EnableAddressAndTxIndexConfig failed: %#v", out)
	}
	cfg, err := config.LoadIndexConfig(filepath.Join(dir, config.ConfigFile))
	if err != nil {
		t.Fatalf("LoadIndexConfig: %v", err)
	}
	if !cfg.AddressIndex || !cfg.TxIndex {
		t.Fatalf("expected both indexes enabled, got addressindex=%t txindex=%t", cfg.AddressIndex, cfg.TxIndex)
	}
}

func TestSearchExplorerAddressDisabledReturnsActionableState(t *testing.T) {
	s := New(t.TempDir())
	out, err := s.SearchExplorer("lhyb1ZqGjjxY7KDrZPP7HNJHUEad7c6EQT2Bop9")
	if err != nil {
		t.Fatalf("SearchExplorer: %v", err)
	}
	if out["type"] != "address_index_required" {
		t.Fatalf("expected address_index_required, got %#v", out)
	}
	if out["action"] != "enable_indexes_restart_reindex" {
		t.Fatalf("expected enable action, got %#v", out)
	}
}

func TestStopWithReportWhenNodeIsNotRunning(t *testing.T) {
	s := New(t.TempDir())
	out := s.StopWithReport("unit test", 100*time.Millisecond)
	if stopped, _ := out["stopped"].(bool); !stopped {
		t.Fatalf("expected stopped=true, got %#v", out)
	}
	if timedOut, _ := out["timed_out"].(bool); timedOut {
		t.Fatalf("expected timed_out=false, got %#v", out)
	}
}

func TestStatusIncludesRPCPortProbe(t *testing.T) {
	s := New(t.TempDir())
	st := s.Status()
	if st.RPCPortState == "" {
		t.Fatalf("expected rpc port state to be set")
	}
}

func TestStartStopReleasesRPCPort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-node lifecycle test in short mode")
	}
	requireProductionYespower(t)
	ln, err := net.Listen("tcp", "127.0.0.1:19556")
	if err != nil {
		t.Skipf("rpc port not available for integration lifecycle test: %v", err)
	}
	_ = ln.Close()
	s := New(t.TempDir())
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	out := s.StopWithReport("integration lifecycle test", 10*time.Second)
	if stopped, _ := out["stopped"].(bool); !stopped {
		t.Fatalf("expected stopped=true, got %#v", out)
	}
	if err := requiredPortsAvailable(s.DataDir(), false); err != nil {
		t.Fatalf("expected ports to be available after stop, got %v", err)
	}
}

func TestSecondServiceReportsCompatibleRPCConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-node lifecycle test in short mode")
	}
	requireProductionYespower(t)
	ln, err := net.Listen("tcp", "127.0.0.1:19556")
	if err != nil {
		t.Skipf("rpc port not available for conflict test: %v", err)
	}
	_ = ln.Close()
	s1 := New(t.TempDir())
	if err := s1.Start(); err != nil {
		t.Fatalf("s1.Start: %v", err)
	}
	defer s1.Stop()
	time.Sleep(250 * time.Millisecond)
	s2 := New(t.TempDir())
	err = s2.Start()
	if err == nil {
		s2.Stop()
		t.Fatal("expected second service start to fail with rpc conflict")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "rpc 127.0.0.1:19556 is already in use") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopWithReportClosesActivePeerReadLoops(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full-node lifecycle test in short mode")
	}
	requireProductionYespower(t)
	ln, err := net.Listen("tcp", "127.0.0.1:19556")
	if err != nil {
		t.Skipf("rpc port not available for shutdown test: %v", err)
	}
	_ = ln.Close()
	s := New(t.TempDir())
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:19555", 2*time.Second)
	if err != nil {
		s.Stop()
		t.Fatalf("dial p2p: %v", err)
	}
	defer conn.Close()
	time.Sleep(250 * time.Millisecond)
	start := time.Now()
	out := s.StopWithReport("shutdown peer read loop test", 8*time.Second)
	if stopped, _ := out["stopped"].(bool); !stopped {
		t.Fatalf("expected stopped=true, got %#v", out)
	}
	if timedOut, _ := out["timed_out"].(bool); timedOut {
		t.Fatalf("expected timed_out=false, got %#v", out)
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("stop took too long (%s), active peer read loops may still be blocking shutdown", elapsed)
	}
}
