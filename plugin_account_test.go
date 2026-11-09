package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPluginCommandsDropOldSurface(t *testing.T) {
	got := map[string]bool{}
	for _, cmd := range pluginCmd.Commands() {
		got[cmd.Name()] = true
	}
	for _, name := range []string{"init", "list", "build", "check", "push"} {
		if !got[name] {
			t.Fatalf("missing command %s", name)
		}
	}
	for _, name := range []string{"create", "add", "remove", "status", "upsert", "activate", "deactivate", "update", "restart", "stop", "delete", "env"} {
		if got[name] {
			t.Fatalf("old command still registered: %s", name)
		}
	}
	push, _, err := pluginCmd.Find([]string{"push"})
	if err != nil {
		t.Fatal(err)
	}
	foundDeploy := false
	for _, alias := range push.Aliases {
		if alias == "deploy" {
			foundDeploy = true
		}
	}
	if !foundDeploy {
		t.Fatal("push must keep deploy as an alias")
	}
	if pluginCmd.PersistentFlags().Lookup("account") != nil {
		t.Fatal("plugin commands must not take --account")
	}
}

func TestPluginRootForAccount(t *testing.T) {
	local, err := pluginRootForAccount("dev", AccountConfig{Kind: "local", ServerURL: "http://localhost:5050"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(local) != "plugins" || filepath.Base(filepath.Dir(local)) != ".apito" {
		t.Fatalf("local path = %s", local)
	}
	cloud, err := pluginRootForAccount("prod", AccountConfig{Kind: "cloud", ServerURL: "https://api.apito.io"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(cloud) != "prod" || filepath.Base(filepath.Dir(cloud)) != "plugins" {
		t.Fatalf("cloud path = %s", cloud)
	}
	override, err := pluginRootForAccount("dev", AccountConfig{Kind: "local", ServerURL: "http://127.0.0.1:5050"}, "/tmp/apito-plugins")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(override) != filepath.Clean("/tmp/apito-plugins") {
		t.Fatalf("override = %s", override)
	}
}

func TestParseGitHubOwnerRepo(t *testing.T) {
	owner, repo, ok := parseGitHubOwnerRepo("https://github.com/apito-io/hc-cloudinary-plugin")
	if !ok || owner != "apito-io" || repo != "hc-cloudinary-plugin" {
		t.Fatalf("got %s %s %v", owner, repo, ok)
	}
}

func TestLocalPluginNamesFollowsHCSymlink(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(t.TempDir(), "hc-eps-payment-plugin")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(root, "hc-eps-payment-plugin")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hc-file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	names, err := localPluginNames(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "hc-eps-payment-plugin" {
		t.Fatalf("names = %v", names)
	}
	if _, err := localPluginDir(root, "eps"); err == nil {
		t.Fatal("plugin id without hc- prefix was accepted")
	}
}

func TestOrderedPlatformsPutsHostFirst(t *testing.T) {
	got := orderedPlatforms()
	if len(got) == 0 {
		t.Fatal("no platforms")
	}
	if got[0].OS != runtime.GOOS || got[0].Arch != runtime.GOARCH {
		t.Fatalf("first = %s/%s, want %s/%s", got[0].OS, got[0].Arch, runtime.GOOS, runtime.GOARCH)
	}
	seen := map[string]int{}
	for _, platform := range got {
		seen[platform.OS+"/"+platform.Arch]++
	}
	if seen[runtime.GOOS+"/"+runtime.GOARCH] != 1 {
		t.Fatalf("host platform duplicated: %+v", got)
	}
}

func TestBuildPluginStatusRows(t *testing.T) {
	local := map[string]pluginPresence{
		"hc-eps-payment-plugin": {Version: "0.1.0"},
	}
	remote := []PluginStatusInfo{
		{ID: "hc-eps-payment-plugin", Version: "0.2.0", Enable: true},
		{ID: "hc-cloudinary-plugin", Version: "1.4.0", Enable: false},
		{ID: "not-a-plugin", Version: "9.9.9", Enable: true},
	}
	rows := buildPluginStatusRows(local, remote, true, "")
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Name != "hc-cloudinary-plugin" || rows[0].Local != "no" || rows[0].Remote != "yes 1.4.0" || rows[0].Installed != "yes" || rows[0].Activated != "no" {
		t.Fatalf("cloudinary = %+v", rows[0])
	}
	if rows[1].Name != "hc-eps-payment-plugin" || rows[1].Local != "yes 0.1.0" || rows[1].Remote != "yes 0.2.0" || rows[1].Installed != "yes" || rows[1].Activated != "yes" {
		t.Fatalf("eps = %+v", rows[1])
	}
	onlyLocal := buildPluginStatusRows(local, nil, true, "")
	if onlyLocal[0].Remote != "no" || onlyLocal[0].Installed != "no" || onlyLocal[0].Activated != "no" {
		t.Fatalf("local only = %+v", onlyLocal[0])
	}
	unknown := buildPluginStatusRows(local, nil, false, "hc-eps-payment-plugin")
	if unknown[0].Remote != "?" || unknown[0].Installed != "?" || unknown[0].Activated != "?" {
		t.Fatalf("unknown remote = %+v", unknown[0])
	}
}

func TestDeployConfirmTargetUsesDestinationAccount(t *testing.T) {
	account, info := deployConfirmTarget("udb-local", "udbhabon")
	if account != "udbhabon" {
		t.Fatalf("account = %q", account)
	}
	if strings.Contains(account, "->") {
		t.Fatalf("confirmation account must be one account, got %q", account)
	}
	if len(info) != 2 || info[0] != "From: udb-local" || info[1] != "To: udbhabon" {
		t.Fatalf("info = %#v", info)
	}
}

func TestRedactPluginConfigSecretsBlanksPassword(t *testing.T) {
	raw := []byte("plugin:\n  env_vars:\n    - key: EPS_PASSWORD\n      value: super-secret\n    - key: EPS_STORE\n      value: shop\n")
	out, err := redactPluginConfigSecrets(raw)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if strings.Contains(text, "super-secret") {
		t.Fatal("password value was copied into the package config")
	}
	if !strings.Contains(text, "EPS_STORE") || !strings.Contains(text, "shop") {
		t.Fatalf("non-secret env was dropped: %s", text)
	}
}

func TestPluginReplaceSentence(t *testing.T) {
	running := &PluginStatusInfo{ID: "hc-eps-payment-plugin", Version: "0.1.0", Status: "loaded", LastUpdated: "2026-11-08T13:45:58Z"}
	if got := describePluginProcess(running); got != "hc-eps-payment-plugin version 0.1.0, process loaded, files updated 2026-11-08T13:45:58Z" {
		t.Fatal(got)
	}
	if got := describePluginProcess(nil); got != "not installed" {
		t.Fatal(got)
	}
	if got := pluginReplaceSentence(running, running); !strings.Contains(got, "Reloaded") {
		t.Fatal(got)
	}
	if got := pluginReplaceSentence(nil, running); !strings.Contains(got, "Loaded") {
		t.Fatal(got)
	}
	if got := pluginReplaceSentence(running, nil); !strings.Contains(got, "Not running") {
		t.Fatal(got)
	}
}

func TestCatalogEntryInstallable(t *testing.T) {
	if catalogEntryInstallable(false, "available") {
		t.Fatal("registry off must not install")
	}
	if catalogEntryInstallable(true, "installed-local") {
		t.Fatal("local-only plugin is not a catalog install")
	}
	if !catalogEntryInstallable(true, "available") {
		t.Fatal("available catalog plugin should install")
	}
}
