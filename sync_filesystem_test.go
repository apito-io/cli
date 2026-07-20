package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFilesystemSyncSide_ConfiguredLocalIsRemote(t *testing.T) {
	// Mimic ~/.apito with an account named "local" — must NOT be filesystem mode.
	home := t.TempDir()
	t.Setenv("HOME", home)
	apitoDir := filepath.Join(home, ".apito")
	if err := os.MkdirAll(apitoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(apitoDir, "config.yml")
	body := []byte("accounts:\n  local:\n    server_url: http://localhost:5050\n    cloud_sync_key: cli-x\n")
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	if isFilesystemSyncSide("local") {
		t.Fatal("configured account named local must be treated as remote")
	}
	if !isFilesystemSyncSide("filesystem") {
		t.Fatal("filesystem reserved name must be filesystem mode")
	}
}

func TestIsFilesystemSyncSide_LegacyLocalWithoutAccount(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	apitoDir := filepath.Join(home, ".apito")
	if err := os.MkdirAll(apitoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(apitoDir, "config.yml")
	body := []byte("accounts:\n  prod:\n    server_url: https://example.com\n    cloud_sync_key: cli-x\n")
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatal(err)
	}

	if !isFilesystemSyncSide("local") {
		t.Fatal("legacy local with no configured account should be filesystem mode")
	}
}

func TestDefaultFunctionsSyncDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := defaultFunctionsSyncDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".apito", "temp", "functions")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}
