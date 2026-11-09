package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDestinationRebuildReason(t *testing.T) {
	server := ServerPlatformInfo{OS: "linux", Architecture: "amd64"}
	if got := destinationRebuildReason(BinaryInfo{OS: "darwin", Architecture: "arm64"}, server); got == "" {
		t.Fatal("darwin/arm64 binary should need a linux/amd64 rebuild")
	}
	if got := destinationRebuildReason(BinaryInfo{OS: "linux", Architecture: "arm64"}, server); got == "" {
		t.Fatal("linux/arm64 binary should need a linux/amd64 rebuild")
	}
	if got := destinationRebuildReason(BinaryInfo{OS: "linux", Architecture: "amd64"}, server); got != "" {
		t.Fatalf("matching binary should not rebuild, got %q", got)
	}
	if got := destinationRebuildReason(BinaryInfo{Error: "unreadable"}, server); got != "" {
		t.Fatalf("unreadable binary is not a platform choice, got %q", got)
	}
}

func TestDeploymentPackageIncludesUIWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("config.yml", "plugin:\n  id: hc-eps-payment-plugin\n  binary_path: hc-eps-payment-plugin\n  ui_config:\n    entry_path: ui.js\n    official: true\n    signed: true\n")
	write("hc-eps-payment-plugin", "binary")
	write("ui.js", "window.ApitoPlugin={}")

	cfg := &PluginConfig{}
	cfg.Plugin.ID = "hc-eps-payment-plugin"
	cfg.Plugin.BinaryPath = "hc-eps-payment-plugin"
	cfg.Plugin.UIConfig = &struct {
		Enable    bool   `yaml:"enable"`
		DistPath  string `yaml:"dist_path"`
		EntryPath string `yaml:"entry_path"`
	}{EntryPath: "ui.js", Enable: false}

	path, err := createDeploymentPackage(dir, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	names := tarNames(t, path)
	for _, want := range []string{"config.yml", "hc-eps-payment-plugin", "ui.js"} {
		if !names[want] {
			t.Fatalf("archive missing %s, got %v", want, names)
		}
	}
}

func TestDeploymentPackageSkipsMissingUI(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("plugin:\n  id: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo"), []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &PluginConfig{}
	cfg.Plugin.ID = "demo"
	cfg.Plugin.BinaryPath = "demo"

	path, err := createDeploymentPackage(dir, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	names := tarNames(t, path)
	if names["ui.js"] {
		t.Fatal("ui.js packed when the file does not exist")
	}
	if !names["demo"] || !names["config.yml"] {
		t.Fatalf("binary package incomplete: %v", names)
	}
}

func TestDeploymentPackageUsesDistAsUIEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ui", "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("plugin:\n  id: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "demo"), []byte("binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui", "dist", "index.umd.js"), []byte("bundle"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &PluginConfig{}
	cfg.Plugin.ID = "demo"
	cfg.Plugin.BinaryPath = "demo"
	cfg.Plugin.UIConfig = &struct {
		Enable    bool   `yaml:"enable"`
		DistPath  string `yaml:"dist_path"`
		EntryPath string `yaml:"entry_path"`
	}{Enable: true, DistPath: "ui/dist/index.umd.js"}

	path, err := createDeploymentPackage(dir, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	names := tarNames(t, path)
	if !names["ui.js"] {
		t.Fatalf("dist file was not packed as ui.js: %v", names)
	}
}

func TestRealEPSPackageListsBinaryAndUI(t *testing.T) {
	dir := filepath.Join("..", "plugins", "hc-eps-payment-plugin")
	raw, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		t.Skip(err)
	}
	var cfg PluginConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	path, err := createDeploymentPackage(dir, &cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)
	names := tarNames(t, path)
	t.Logf("archive %s", path)
	for name := range names {
		t.Logf("  %s", name)
	}
	if !names["hc-eps-payment-plugin"] || !names["ui.js"] || !names["config.yml"] {
		t.Fatalf("EPS deploy archive = %v", names)
	}
}

func tarNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	out := map[string]bool{}
	for {
		h, err := tr.Next()
		if err != nil {
			break
		}
		out[h.Name] = true
	}
	return out
}
