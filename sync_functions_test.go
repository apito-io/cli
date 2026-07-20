package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleFunction() SyncFunction {
	return SyncFunction{
		Name:                "listFoodNames",
		Description:         "List food names",
		Source:              "export default async () => ({ ok: true })\n",
		TriggerType:         "function",
		Language:            "typescript",
		GraphQLSchemaType:   "String",
		Capabilities:        []string{"data.read"},
		RuntimeConfig:       &SyncFunctionRuntimeConfig{Handler: "default", Runtime: "apito-deno", TimeOut: 30},
		Request:             &SyncFunctionModelRef{Model: "food", OptionalPayload: true},
		Response:            &SyncFunctionModelRef{Model: "food", IsArray: true},
		EnvVars:             []SyncFunctionEnvVar{{Key: "FOO", Value: "bar"}},
		ActiveRevisionID:    "rev_live",
		RestAPISecretURLKey: "supersecret",
	}
}

func TestComputeFunctionDiff_AddUpdateUnchanged(t *testing.T) {
	src := sampleFunction()
	src.ActiveRevisionID = "" // draft-only: revision parity not required

	// Unchanged: identical transferable fields (volatile fields differ, must be ignored).
	unchangedDest := src
	unchangedDest.ActiveRevisionID = "rev_other"
	unchangedDest.RestAPISecretURLKey = "different"

	changes := computeFunctionDiff([]SyncFunction{src}, []SyncFunction{unchangedDest})
	if len(changes) != 1 || changes[0].Kind != FunctionUnchanged {
		t.Fatalf("expected unchanged, got %+v", changes)
	}
	if len(actionableFunctionChanges(changes)) != 0 {
		t.Fatal("unchanged function should not be actionable")
	}

	// Add: destination empty.
	changes = computeFunctionDiff([]SyncFunction{src}, nil)
	if len(changes) != 1 || changes[0].Kind != FunctionAdd || changes[0].DestExists {
		t.Fatalf("expected add, got %+v", changes)
	}

	// Update: source differs (capability drift).
	drifted := src
	drifted.Capabilities = []string{"data.read", "data.write"}
	changes = computeFunctionDiff([]SyncFunction{drifted}, []SyncFunction{src})
	if len(changes) != 1 || changes[0].Kind != FunctionUpdate || !changes[0].DestExists {
		t.Fatalf("expected update, got %+v", changes)
	}
}

func TestComputeFunctionDiff_DeployWhenLiveHashMissing(t *testing.T) {
	src := sampleFunction() // has ActiveRevisionID
	dest := src
	dest.ActiveRevisionID = "rev_other"
	dest.ActiveRevisionHash = ""

	changes := actionableFunctionChanges(computeFunctionDiff([]SyncFunction{src}, []SyncFunction{dest}))
	if len(changes) != 1 || changes[0].Kind != FunctionDeploy {
		t.Fatalf("expected deploy for missing live hash, got %+v", changes)
	}
	if !strings.Contains(changes[0].Summary, "live revision missing") {
		t.Fatalf("summary = %q", changes[0].Summary)
	}
}

func TestComputeFunctionDiff_DeployWhenHashDrifts(t *testing.T) {
	src := sampleFunction()
	dest := src
	dest.ActiveRevisionHash = "deadbeef"

	changes := actionableFunctionChanges(computeFunctionDiff([]SyncFunction{src}, []SyncFunction{dest}))
	if len(changes) != 1 || changes[0].Kind != FunctionDeploy {
		t.Fatalf("expected deploy for hash drift, got %+v", changes)
	}
	if !strings.Contains(changes[0].Summary, "hash drift") {
		t.Fatalf("summary = %q", changes[0].Summary)
	}
}

func TestComputeFunctionDiff_UnchangedWhenLiveHashMatches(t *testing.T) {
	src := sampleFunction()
	dest := src
	dest.ActiveRevisionID = "rev_dest"
	dest.ActiveRevisionHash = functionSourceHash(src.Source)

	changes := computeFunctionDiff([]SyncFunction{src}, []SyncFunction{dest})
	if len(changes) != 1 || changes[0].Kind != FunctionUnchanged {
		t.Fatalf("expected unchanged when live hash matches draft, got %+v", changes)
	}
	if len(actionableFunctionChanges(changes)) != 0 {
		t.Fatal("matching live hash must not be actionable")
	}
}

func TestComputeFunctionDiff_SourceEditIsUpdate(t *testing.T) {
	src := sampleFunction()
	dest := src
	dest.Source = "export default async () => ({ ok: false })\n"

	changes := computeFunctionDiff([]SyncFunction{src}, []SyncFunction{dest})
	if len(changes) != 1 || changes[0].Kind != FunctionUpdate {
		t.Fatalf("expected update for source edit, got %+v", changes)
	}
}

func TestLocalFunctionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fn := sampleFunction()

	// Export without secrets: secret must be stripped, source lands in source.ts.
	if err := writeLocalFunction(dir, fn, false); err != nil {
		t.Fatalf("write: %v", err)
	}

	sourcePath := filepath.Join(dir, fn.Name, "source.ts")
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source.ts: %v", err)
	}
	if string(sourceBytes) != fn.Source {
		t.Fatalf("source.ts mismatch: got %q want %q", string(sourceBytes), fn.Source)
	}

	loaded, err := readLocalFunctions(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 function, got %d", len(loaded))
	}
	got := loaded[0]
	if got.RestAPISecretURLKey != "" {
		t.Fatal("secret must not be written without --include-secrets")
	}
	if got.ActiveRevisionID != "" {
		t.Fatal("volatile active_revision_id must not be persisted")
	}
	// Round-trip preserves the transferable definition (ignoring volatile fields).
	want := fn
	want.RestAPISecretURLKey = ""
	want.ActiveRevisionID = ""
	if !functionEqualForSync(got, want) {
		t.Fatalf("round-trip changed definition:\n got=%+v\nwant=%+v", got, want)
	}
}

func TestLocalFunctionRoundTrip_IncludeSecrets(t *testing.T) {
	dir := t.TempDir()
	fn := sampleFunction()
	if err := writeLocalFunction(dir, fn, true); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := readLocalFunctions(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if loaded[0].RestAPISecretURLKey != "supersecret" {
		t.Fatalf("expected secret preserved with --include-secrets, got %q", loaded[0].RestAPISecretURLKey)
	}
}

func TestReadLocalFunctions_NameMismatch(t *testing.T) {
	dir := t.TempDir()
	fnDir := filepath.Join(dir, "wrongFolder")
	if err := os.MkdirAll(fnDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fnDir, "meta.json"), []byte(`{"name":"actualName"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fnDir, "source.ts"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readLocalFunctions(dir); err == nil {
		t.Fatal("expected error for folder/name mismatch")
	}
}
