package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureStudioFilesLinksEnvVolumeAndKeys(t *testing.T) {
	home := t.TempDir()
	legacyDir := filepath.Join(home, ".apito", "bin")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyKey := "abcdefghijklmnopqrstuvwxyz012345"
	legacy := map[string]string{
		"BRANKA_KEY":       legacyKey,
		"SERVE_PORT":       "5050",
		"CORS_ORIGIN":      "http://localhost:4000",
		"PRIVATE_KEY_PATH": "keys/private.key",
	}
	if err := writeEnvFile(filepath.Join(legacyDir, ".env"), legacy, 0600); err != nil {
		t.Fatal(err)
	}

	created, err := ensureStudioFilesIn(home)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected first run to create studio files")
	}

	engine, err := readEnvFile(filepath.Join(legacyDir, ".env.engine"))
	if err != nil {
		t.Fatal(err)
	}
	if engine["BRANKA_KEY"] != legacyKey {
		t.Fatalf("BRANKA_KEY was replaced")
	}
	if engine["SERVE_PORT"] != "5055" {
		t.Fatalf("SERVE_PORT = %q", engine["SERVE_PORT"])
	}
	if engine["DEFAULT_DATABASE_DIR"] != "/data" || engine["SQLITE_DATA_DIR"] != "/data" {
		t.Fatalf("database dir = %q %q", engine["DEFAULT_DATABASE_DIR"], engine["SQLITE_DATA_DIR"])
	}
	if strings.Contains(engine["DEFAULT_DATABASE_DIR"], ".apito") {
		t.Fatal("engine env must use the container data path")
	}
	if engine["PRIVATE_KEY_PATH"] != "/data/keys/private.key" {
		t.Fatalf("private key path = %q", engine["PRIVATE_KEY_PATH"])
	}
	if engine["SYSTEM_DB_ENGINE"] != "sqlite" {
		t.Fatalf("SYSTEM_DB_ENGINE = %q", engine["SYSTEM_DB_ENGINE"])
	}
	if engine["BOOTSTRAP_ADMIN_EMAIL"] != "oss@apito.io" {
		t.Fatalf("BOOTSTRAP_ADMIN_EMAIL = %q", engine["BOOTSTRAP_ADMIN_EMAIL"])
	}
	if len(engine["BOOTSTRAP_ADMIN_PASSWORD"]) != 20 {
		t.Fatalf("BOOTSTRAP_ADMIN_PASSWORD length = %d", len(engine["BOOTSTRAP_ADMIN_PASSWORD"]))
	}
	if engine["CORS_ORIGIN"] != "http://localhost:4055" {
		t.Fatalf("CORS_ORIGIN = %q", engine["CORS_ORIGIN"])
	}
	if strings.TrimSpace(engine["APITO_ADMIN_RESET_SECRET"]) == "" {
		t.Fatal("admin reset secret was not generated")
	}

	mirror, err := readEnvFile(filepath.Join(legacyDir, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if mirror["BRANKA_KEY"] != legacyKey || mirror["SERVE_PORT"] != "5055" {
		t.Fatal("legacy .env was not kept in sync with .env.engine")
	}

	console, err := readEnvFile(filepath.Join(legacyDir, ".env.console"))
	if err != nil {
		t.Fatal(err)
	}
	if console["VITE_REST_API"] != "http://localhost:4055" {
		t.Fatalf("console rest api = %q", console["VITE_REST_API"])
	}
	if console["VITE_GRAPH_API"] != "http://localhost:4055/system/graphql" {
		t.Fatalf("console graph api = %q", console["VITE_GRAPH_API"])
	}
	if console["VITE_PUBLIC_GRAPH_API"] != "http://localhost:4055/secured/graphql" {
		t.Fatalf("console public graph api = %q", console["VITE_PUBLIC_GRAPH_API"])
	}

	priv := filepath.Join(home, ".apito", "db", "keys", "private.key")
	pub := filepath.Join(home, ".apito", "db", "keys", "public.key")
	if !fileExists(priv) || !fileExists(pub) {
		t.Fatal("signing keys were not written under the database directory")
	}
	for _, dir := range []string{"sqlite/system", "sqlite/projects", "sqlite/tenants", "logs", "keys"} {
		info, err := os.Stat(filepath.Join(home, ".apito", "db", dir))
		if err != nil || !info.IsDir() {
			t.Fatalf("database directory %s was not created", dir)
		}
	}

	again, err := ensureStudioFilesIn(home)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("second run created files again")
	}
	reread, err := readEnvFile(filepath.Join(legacyDir, ".env.engine"))
	if err != nil {
		t.Fatal(err)
	}
	if reread["BRANKA_KEY"] != legacyKey {
		t.Fatal("second run rotated BRANKA_KEY")
	}
	if reread["APITO_ADMIN_RESET_SECRET"] != engine["APITO_ADMIN_RESET_SECRET"] {
		t.Fatal("second run rotated the admin reset secret")
	}
	if reread["BOOTSTRAP_ADMIN_PASSWORD"] != engine["BOOTSTRAP_ADMIN_PASSWORD"] {
		t.Fatal("second run rotated the Studio admin password")
	}
}

func TestEnsureStudioFilesGeneratesBrankaWhenMissing(t *testing.T) {
	home := t.TempDir()
	if _, err := ensureStudioFilesIn(home); err != nil {
		t.Fatal(err)
	}
	engine, err := readEnvFile(filepath.Join(home, ".apito", "bin", ".env.engine"))
	if err != nil {
		t.Fatal(err)
	}
	if len(engine["BRANKA_KEY"]) != 32 {
		t.Fatalf("BRANKA_KEY length = %d", len(engine["BRANKA_KEY"]))
	}
}
