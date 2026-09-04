package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSchemaBlueprintJSON_engineAndWrapped(t *testing.T) {
	engine := []byte(`{"models":[{"name":"post","fields":[{"identifier":"title","field_type":"text"}]}]}`)
	models, err := parseSchemaBlueprintJSON(engine)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "post" {
		t.Fatalf("engine shape: %+v", models)
	}

	wrapped := []byte(`{
		"id":"blog_cms",
		"name":"Blog",
		"schema":{"models":[{"name":"post","fields":[{"identifier":"title","field_type":"text"}]}]}
	}`)
	models, err = parseSchemaBlueprintJSON(wrapped)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "post" {
		t.Fatalf("wrapped shape: %+v", models)
	}
}

func TestSanitizeBlueprintModels_stripsSystemAndRowID(t *testing.T) {
	in := []SyncModel{{
		Name: "dashboard",
		Fields: []SyncField{
			{Identifier: "title", FieldType: "text"},
			{Identifier: "system_dashboard_id", FieldType: "text"},
			{Identifier: "items", FieldType: "repeated", SubFieldInfo: []SyncField{
				{Identifier: "_id", FieldType: "text"},
				{Identifier: "label", FieldType: "text"},
			}},
		},
		Connections: []SyncConnection{{Model: "user", Relation: "has_one"}},
	}}
	out := sanitizeBlueprintModels(in)
	if len(out) != 1 {
		t.Fatalf("models: %d", len(out))
	}
	ids := map[string]bool{}
	for _, f := range out[0].Fields {
		ids[f.Identifier] = true
		for _, c := range f.SubFieldInfo {
			ids[f.Identifier+"."+c.Identifier] = true
		}
	}
	if ids["system_dashboard_id"] {
		t.Fatal("system_ field should be stripped")
	}
	if ids["items._id"] {
		t.Fatal("repeated _id should be stripped")
	}
	if !ids["title"] || !ids["items"] || !ids["items.label"] {
		t.Fatalf("expected portable fields kept: %#v", ids)
	}
}

func TestRejectBlueprintSecretsJSON(t *testing.T) {
	raw := []byte(`{"id":"x","name":"X","tokens":[{"token":"apt_leak"}],"schema":{"models":[]}}`)
	err := rejectBlueprintSecretsJSON(raw, "schema.json")
	if err == nil || !strings.Contains(err.Error(), "tokens") {
		t.Fatalf("expected tokens rejection, got %v", err)
	}
}

func TestReadLocalSchema_sampleBlog(t *testing.T) {
	dir := filepath.Join("..", "schema-generation", "quickstart", "sample-blog")
	if _, err := os.Stat(filepath.Join(dir, "schema.json")); err != nil {
		t.Skip("sample-blog fixture not available:", err)
	}
	models, cfg, err := readLocalSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) < 2 {
		t.Fatalf("expected multiple models, got %d", len(models))
	}
	if cfg == nil || cfg.ProjectType == "" {
		t.Fatalf("expected config.yml project_type, got %+v", cfg)
	}
	if cfg.syncProfileKey() != "general" {
		t.Fatalf("blog profile: %s", cfg.syncProfileKey())
	}
	if issues := validateSyncModels(models); len(issues) > 0 {
		t.Fatalf("validation:\n%s", formatSchemaValidationReport(issues))
	}
}

func TestReadLocalSchema_rejectsSecrets(t *testing.T) {
	dir := t.TempDir()
	body := `{"id":"leak","name":"Leak","project_secret_key":"abc","schema":{"models":[{"name":"a","fields":[{"identifier":"t","field_type":"text"}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := readLocalSchema(dir)
	if err == nil || !strings.Contains(err.Error(), "project_secret_key") {
		t.Fatalf("expected secret rejection, got %v", err)
	}
}

func TestWriteAndReadLocalSchema_roundTrip(t *testing.T) {
	dir := t.TempDir()
	models := []SyncModel{{
		Name: "item",
		Fields: []SyncField{
			{Identifier: "title", FieldType: "text", Label: "Title"},
			{Identifier: "system_x", FieldType: "text"},
		},
	}}
	cfg := &blueprintConfig{Name: "Demo", Slug: "demo", ProjectType: "general"}
	if err := writeLocalSchema(dir, models, cfg); err != nil {
		t.Fatal(err)
	}
	got, gotCfg, err := readLocalSchema(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "item" {
		t.Fatalf("got %+v", got)
	}
	for _, f := range got[0].Fields {
		if strings.HasPrefix(f.Identifier, "system_") {
			t.Fatal("system field leaked through round-trip")
		}
	}
	// config.yml not written by writeLocalSchema — cfg from read is nil unless we add one
	_ = gotCfg
}

func TestBlueprintConfig_saasProfiles(t *testing.T) {
	shared := blueprintConfig{ProjectType: "saas"}
	if shared.syncProfileKey() != "saas-shared" {
		t.Fatal(shared.syncProfileKey())
	}
	per := blueprintConfig{ProjectType: "saas", PerTenantSeparateDatabase: true}
	if per.syncProfileKey() != "saas-per-tenant" {
		t.Fatal(per.syncProfileKey())
	}
}

func TestDefaultSchemaSyncDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := defaultSchemaSyncDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".apito", "temp", "schema")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}

func TestParseModelsFromSchemaJSON_stillEnginePreview(t *testing.T) {
	models, err := parseModelsFromSchemaJSON(`{"models":[{"name":"x"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(models) != 1 || models[0].Name != "x" {
		t.Fatalf("%+v", models)
	}
}

func TestReadLocalSchema_examplesBlueprints(t *testing.T) {
	root := filepath.Join("..", "examples", "blueprints")
	for _, slug := range []string{"newspaper-cms", "flutter-finance", "tenant-saas-pos", "mcp-saas"} {
		dir := filepath.Join(root, slug)
		if _, err := os.Stat(filepath.Join(dir, "schema.json")); err != nil {
			t.Skip(err)
		}
		models, cfg, err := readLocalSchema(dir)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if len(models) == 0 {
			t.Fatalf("%s: no models", slug)
		}
		if issues := validateSyncModels(models); len(issues) > 0 {
			t.Fatalf("%s:\n%s", slug, formatSchemaValidationReport(issues))
		}
		_ = cfg
	}
}
