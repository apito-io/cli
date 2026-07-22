package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// Live probe against local Studio engine. Skips unless APITO_LIVE=1.
// Expects ~/.apito account "local" and project protiva_xtg4d (override via env).
func TestLiveProjectModelsInfo_NestedExamRoutineDetails(t *testing.T) {
	if os.Getenv("APITO_LIVE") != "1" {
		t.Skip("set APITO_LIVE=1 to hit local engine")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".apito", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Accounts map[string]struct {
			ServerURL    string `yaml:"server_url"`
			CloudSyncKey string `yaml:"cloud_sync_key"`
		} `yaml:"accounts"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	accName := os.Getenv("APITO_LIVE_ACCOUNT")
	if accName == "" {
		accName = "local"
	}
	acc, ok := cfg.Accounts[accName]
	if !ok || acc.CloudSyncKey == "" {
		t.Fatalf("account %q missing from ~/.apito/config.yml", accName)
	}
	projectID := os.Getenv("APITO_LIVE_PROJECT")
	if projectID == "" {
		projectID = "protiva_xtg4d"
	}
	serverURL := acc.ServerURL
	if v := os.Getenv("APITO_LIVE_URL"); v != "" {
		serverURL = v
	}
	// Prefer IPv4 loopback — Go's "localhost" may hit ::1 while engine binds oddly under some setups.
	if serverURL == "http://localhost:5050" {
		serverURL = "http://127.0.0.1:5050"
	}

	client := newSyncGraphQLClient(serverURL, acc.CloudSyncKey, 60).WithProject(projectID)
	models, err := client.ProjectModelsInfo("exam")
	if err != nil {
		t.Fatalf("ProjectModelsInfo(exam): %v", err)
	}
	if len(models) == 0 {
		t.Fatal("exam model missing")
	}
	flat := flattenModelFields(models[0])
	got := fieldMap(flat)
	for _, want := range []string{
		"routine",
		"routine.details",
		"routine.details.date_and_time",
		"routine.details.subject_code",
	} {
		if _, ok := got[want]; !ok {
			t.Errorf("missing nested path %q (have %v)", want, keysOf(got))
		}
	}
}
