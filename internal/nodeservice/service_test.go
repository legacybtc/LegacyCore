package nodeservice

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/node"
)

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
