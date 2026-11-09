package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPluginPicksShowNameAndVersion(t *testing.T) {
	rows := []listedPlugin{
		{Name: "hc-eps-payment-plugin", Version: "0.1.0", Path: "/plugins/hc-eps-payment-plugin"},
		{Name: "hc-cloudinary-plugin", Version: "0.0.3", Path: "/plugins/apito-plugins/hc-cloudinary-plugin"},
		{Name: "hc-cloudinary-plugin", Version: "0.0.5", Path: "/plugins/hc-cloudinary-plugin"},
		{Name: "hc-cloudinary-plugin", Version: "0.0.5", Path: "/plugins/other/hc-cloudinary-plugin"},
	}
	picks := pluginPicks(rows)
	if picks[0].Text != "hc-eps-payment-plugin  0.1.0" {
		t.Fatalf("plain %q", picks[0].Text)
	}
	if picks[1].Text != "hc-cloudinary-plugin  0.0.3" {
		t.Fatalf("different version %q", picks[1].Text)
	}
	if !strings.HasSuffix(picks[2].Text, "  plugins") || !strings.HasSuffix(picks[3].Text, "  other") {
		t.Fatalf("same name and version were not separated: %#v", []string{picks[2].Text, picks[3].Text})
	}
}

func TestPluginListRoots(t *testing.T) {
	roots, err := pluginListRootsFrom("", "/work/plugins")
	if err != nil || len(roots) != 2 || roots[0] != "/work/plugins" {
		t.Fatalf("default roots %+v %v", roots, err)
	}
	home, err := defaultPluginRoot()
	if err != nil || roots[1] != home {
		t.Fatalf("home %s roots %+v", home, roots)
	}
	want, err := filepath.Abs("/tmp/other")
	if err != nil {
		t.Fatal(err)
	}
	roots, err = pluginListRootsFrom("/tmp/other", "/work/plugins")
	if err != nil || len(roots) != 1 || roots[0] != want {
		t.Fatalf("--dir path %+v %v", roots, err)
	}
}

func TestDiscoverLocalPluginsReadsNestedGroups(t *testing.T) {
	root := t.TempDir()
	write := func(dir, id string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "plugin:\n  id: " + id + "\n  version: \"0.1.0\"\n"
		if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "hc-eps-payment-plugin"), "hc-eps-payment-plugin")
	write(filepath.Join(root, "udbhabon-plugins", "hc-rosna-plugin"), "hc-rosna-plugin")
	rows, err := discoverLocalPlugins([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, row := range rows {
		got[row.Name] = true
	}
	if !got["hc-eps-payment-plugin"] || !got["hc-rosna-plugin"] {
		t.Fatalf("rows %+v", rows)
	}
}

func TestBumpSemver(t *testing.T) {
	next, err := bumpSemver("0.1.0", "patch")
	if err != nil || next != "0.1.1" {
		t.Fatalf("patch %s %v", next, err)
	}
	next, err = bumpSemver("0.1.9", "minor")
	if err != nil || next != "0.2.0" {
		t.Fatalf("minor %s %v", next, err)
	}
	next, err = bumpSemver("1.4.2", "major")
	if err != nil || next != "2.0.0" {
		t.Fatalf("major %s %v", next, err)
	}
	if _, err := bumpSemver("1.2", "patch"); err == nil {
		t.Fatal("short version was accepted")
	}
}

func TestRequireNewerVersion(t *testing.T) {
	if err := requireNewerVersion("0.1.0", "0.1.1"); err != nil {
		t.Fatal(err)
	}
	if err := requireNewerVersion("0.1.0", "0.2.0"); err != nil {
		t.Fatal(err)
	}
	if err := requireNewerVersion("0.1.0", "0.1.0"); err == nil {
		t.Fatal("same version was accepted")
	}
	if err := requireNewerVersion("0.1.1", "0.1.0"); err == nil {
		t.Fatal("older version was accepted")
	}
}

func TestCheckPluginReportsMissingReleaseShape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte("plugin:\n  id: demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := checkPlugin(dir)
	if err == nil {
		t.Fatal("incomplete plugin passed check")
	}
	text := err.Error()
	for _, want := range []string{"hc-", "language", "version", "binary_path", "github.com", "handshake", "contributions", "main.go", "build-release.yml", "origin"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestCheckPluginAcceptsReleaseShape(t *testing.T) {
	dir := writeReleasePlugin(t)
	if err := checkPlugin(dir); err != nil {
		t.Fatal(err)
	}
}

func TestWritePluginVersionUpdatesConfigAndInit(t *testing.T) {
	dir := writeReleasePlugin(t)
	if err := writePluginVersion(dir, "0.1.1"); err != nil {
		t.Fatal(err)
	}
	cfg, err := readPluginConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugin.Version != "0.1.1" {
		t.Fatalf("config version %s", cfg.Plugin.Version)
	}
	main, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	id, version := sdkInitIdentity(string(main))
	if id != "hc-demo-plugin" || version != "0.1.1" {
		t.Fatalf("sdk.Init %s %s", id, version)
	}
}

func TestReplaceVersionFieldQuotesBareVersion(t *testing.T) {
	got := replaceVersionField("plugin:\n  version: 0.1.0\n", "0.1.1")
	if !strings.Contains(got, `version: "0.1.1"`) {
		t.Fatalf("got %s", got)
	}
}

func TestDiscoverLocalPluginsReadsConfig(t *testing.T) {
	root := t.TempDir()
	plugin := filepath.Join(root, "hc-demo-plugin")
	if err := os.Mkdir(plugin, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("plugin:\n  id: hc-demo-plugin\n  version: \"0.1.0\"\n")
	if err := os.WriteFile(filepath.Join(plugin, "config.yml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := readPluginConfig(plugin)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Plugin.ID != "hc-demo-plugin" || cfg.Plugin.Version != "0.1.0" {
		t.Fatalf("config %+v", cfg.Plugin)
	}
	table := formatLocalPluginTable([]listedPlugin{{Name: cfg.Plugin.ID, Version: cfg.Plugin.Version, Path: plugin}})
	if !strings.Contains(table, "NAME") || !strings.Contains(table, "hc-demo-plugin") || !strings.Contains(table, "0.1.0") {
		t.Fatalf("table %s", table)
	}
}

func writeReleasePlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"config.yml": `plugin:
  id: "hc-demo-plugin"
  language: "go"
  version: "0.1.0"
  binary_path: "hc-demo-plugin"
  repository_url: "https://github.com/apito-io/hc-demo-plugin"
  handshake_config:
    protocol_version: 1
    magic_cookie_key: "APITO_PLUGIN"
    magic_cookie_value: "apito_plugin_magic_cookie_v1"
  contributions:
    api:
      scope: project
`,
		"main.go":                             "package main\nfunc main() { sdk.Init(\"hc-demo-plugin\", \"0.1.0\", \"\") }\n",
		"go.mod":                              "module github.com/apito-io/hc-demo-plugin\n\ngo 1.25\n",
		"ui.js":                               "window.ApitoPlugin={}\n",
		".github/workflows/build-release.yml": "on:\n  push:\n    tags: [\"v*\"]\njobs:\n  test:\n    steps:\n      - run: go test ./...\n      - run: python3 -c \"config.yml sdk.Init\"\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "ui"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(files["config.yml"]+"  ui_config:\n    entry_path: ui.js\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %s", strings.Join(args, " "), out)
		}
	}
	git("init")
	git("remote", "add", "origin", "https://github.com/apito-io/hc-demo-plugin.git")
	return dir
}
