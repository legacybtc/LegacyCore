package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/nodeservice"
	"legacycoin/legacy-go/internal/wallet"
)

func TestSaveSettingsKeepsServiceWhenDataDirUnchanged(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	original := app.service
	s := app.settings
	s.DefaultThreads = 3
	if _, err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if app.service != original {
		t.Fatalf("expected existing service to be reused when data dir is unchanged")
	}
}

func TestSaveSettingsRecreatesServiceWhenDataDirChanges(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	app := NewApp()
	original := app.service
	s := app.settings
	s.DataDir = filepath.Join(t.TempDir(), "alt-datadir")
	if _, err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if app.service == original {
		t.Fatalf("expected service instance to change when data dir changes")
	}
}

func TestDefaultMiningThreadsLeavesCPUHeadroom(t *testing.T) {
	got := defaultSettings().DefaultThreads
	if got < 1 {
		t.Fatalf("default threads must be at least 1, got %d", got)
	}
	if runtime.NumCPU() > 2 && got > runtime.NumCPU()-2 {
		t.Fatalf("default threads=%d should leave CPU headroom from %d CPUs", got, runtime.NumCPU())
	}
}

func TestSetDefaultMiningAddressPersistsWalletOwnedConfig(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	dir := t.TempDir()
	w, err := wallet.Open(dir)
	if err != nil {
		t.Fatalf("wallet.Open: %v", err)
	}
	addr, err := w.NewAddress()
	if err != nil {
		t.Fatalf("NewAddress: %v", err)
	}
	app := NewApp()
	app.settings = app.settings.withDefaults()
	app.settings.DataDir = dir
	app.service = nodeservice.New(dir)
	if _, err := app.SetDefaultMiningAddress(addr); err != nil {
		t.Fatalf("SetDefaultMiningAddress: %v", err)
	}
	cfg, err := config.LoadMiningConfig(filepath.Join(dir, config.ConfigFile))
	if err != nil {
		t.Fatalf("LoadMiningConfig: %v", err)
	}
	if cfg.RewardAddress != addr || cfg.PubKeyHash == "" {
		t.Fatalf("mining destination not persisted: %+v", cfg)
	}
	if app.settings.DefaultMiningAddress != addr {
		t.Fatalf("settings default mining address=%q want %q", app.settings.DefaultMiningAddress, addr)
	}
}

func TestEnsureDefaultMiningAddressDoesNotRepairUnownedConfigSilently(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	dir := t.TempDir()
	if _, err := wallet.Open(dir); err != nil {
		t.Fatalf("wallet.Open: %v", err)
	}
	if err := config.AppendConfigLine(filepath.Join(dir, config.ConfigFile), "mining_pubkey_hash", "85f774538db4b5243fe64121bbfe53bc83441e0e"); err != nil {
		t.Fatalf("AppendConfigLine: %v", err)
	}
	app := NewApp()
	app.settings = app.settings.withDefaults()
	app.settings.DataDir = dir
	app.settings.DefaultMiningAddress = ""
	app.service = nodeservice.New(dir)
	app.ensureDefaultMiningAddressFromWallet()
	cfg, err := config.LoadMiningConfig(filepath.Join(dir, config.ConfigFile))
	if err != nil {
		t.Fatalf("LoadMiningConfig: %v", err)
	}
	if cfg.RewardAddress != "" {
		t.Fatalf("expected stale config not to be silently repaired, got %+v", cfg)
	}
}
