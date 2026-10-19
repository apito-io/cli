package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchIntrospectionUsesAccountOnSecuredGraphQL(t *testing.T) {
	var sawPath, sawAuth, sawProject, sawTenant string
	var sawBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPath = r.URL.Path
		sawAuth = r.Header.Get("Authorization")
		sawProject = r.Header.Get("X-Apito-Project-Id")
		sawTenant = r.Header.Get("X-Apito-Tenant-ID")
		body, _ := io.ReadAll(r.Body)
		sawBody = body
		_, _ = w.Write([]byte(`{"data":{"__schema":{"queryType":{"name":"Query"},"types":[]}}}`))
	}))
	defer srv.Close()

	ctx := commandContext{
		ProjectID: "suchok_wb8ed",
		TenantID:  "tenant_1",
		Client:    newSystemClient(srv.URL, "apt_testtoken", 5).WithProject("suchok_wb8ed").WithTenant("tenant_1"),
	}
	raw, err := fetchIntrospection(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sawPath != "/secured/graphql" {
		t.Fatalf("path = %s", sawPath)
	}
	if sawAuth != "Bearer apt_testtoken" {
		t.Fatalf("auth = %s", sawAuth)
	}
	if sawProject != "suchok_wb8ed" || sawTenant != "tenant_1" {
		t.Fatalf("project=%s tenant=%s", sawProject, sawTenant)
	}
	if bytes.Contains(sawBody, []byte("generateTenantToken")) || bytes.Contains(sawBody, []byte("ak_")) {
		t.Fatalf("request asked for a tenant or app secret: %s", sawBody)
	}
	if !bytes.Contains(sawBody, []byte("__schema")) || !bytes.Contains(raw, []byte("__schema")) {
		t.Fatal("introspection query or response was incomplete")
	}
}

func TestFetchIntrospectionRequiresProjectAndDropsSecrets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("fetch ran without a project")
	}))
	defer srv.Close()
	if _, err := fetchIntrospection(commandContext{Client: newSystemClient(srv.URL, "apt_testtoken", 5)}); err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("missing project: %v", err)
	}

	secretSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"__schema":{"description":"apt_secret"}}}`))
	}))
	defer secretSrv.Close()
	_, err := fetchIntrospection(commandContext{
		ProjectID: "project_1",
		Client:    newSystemClient(secretSrv.URL, "apt_testtoken", 5).WithProject("project_1"),
	})
	if err == nil || !strings.Contains(err.Error(), "credential") || strings.Contains(err.Error(), "apt_secret") {
		t.Fatalf("secret response: %v", err)
	}
}

func TestTSCodegenArgsLetTheGeneratorDiscoverModels(t *testing.T) {
	previous := genRelations
	genRelations = true
	t.Cleanup(func() { genRelations = previous })
	args := strings.Join(tsCodegenArgs("schema.json", "src/lib/apito-generated", "", "app"), " ")
	if !strings.Contains(args, "--schema schema.json") || strings.Contains(args, "--config") || strings.Contains(args, `"id"`) {
		t.Fatalf("args = %s", args)
	}
	filtered := tsCodegenArgs("schema.json", "out", "order,customer", "sdk")
	if !bytes.Contains([]byte(strings.Join(filtered, "\n")), []byte("--models\norder,customer")) {
		t.Fatalf("filter = %v", filtered)
	}
}

func TestGenDartWritesCredentialFreeSnapshot(t *testing.T) {
	dir := t.TempDir()
	schema := filepath.Join(dir, "schema.json")
	raw := []byte(`{"data":{"__schema":{"queryType":{"name":"Query"}}}}`)
	if err := os.WriteFile(schema, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "apito_schema.json")
	genDir = dir
	genLanguage = "dart"
	genSchema = schema
	genOut = out
	genCheck = false
	runTSCodegen = func(_, _, _, _ string) error {
		t.Fatal("dart snapshot mode invoked the TypeScript generator")
		return nil
	}
	t.Cleanup(func() {
		genDir, genLanguage, genSchema, genOut = "", "", "", ""
		genCheck = false
		runTSCodegen = defaultTSCodegen
	})
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(written, []byte("__schema")) || bytes.Contains(written, []byte("apt_")) {
		t.Fatalf("snapshot = %s", written)
	}

	secret := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(secret, []byte(`{"data":{"__schema":{"token":"ak_secret"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	genSchema = secret
	if err := runGen(nil, nil); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("secret snapshot: %v", err)
	}
}
