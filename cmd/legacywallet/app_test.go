package main

import (
	"path/filepath"
	"testing"
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
