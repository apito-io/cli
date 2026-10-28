package main

import (
	"strings"
	"testing"
)

func TestMissingSyncCapsSchemaLoginToken(t *testing.T) {
	missing := missingSyncCaps([]string{"projects.read", "schema.read", "projects.write"}, "schema", false)
	if len(missing) != 1 || missing[0] != "schema.write" {
		t.Fatalf("missing = %#v", missing)
	}
	msg := syncTokenMessage("local", "cli_login", "CLI login", missing, "schema")
	for _, want := range []string{"schema.write", "CLI sync", "Custom", "apito login", "local"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q missing %q", msg, want)
		}
	}
}

func TestMissingSyncCapsSchemaSyncToken(t *testing.T) {
	missing := missingSyncCaps([]string{"projects.read", "schema.read", "schema.write", "sync.write"}, "schema", false)
	if len(missing) != 0 {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestMissingSyncCapsFunctionDeploy(t *testing.T) {
	missing := missingSyncCaps([]string{"functions.write"}, "functions", true)
	if len(missing) != 1 || missing[0] != "functions.deploy" {
		t.Fatalf("missing = %#v", missing)
	}
}

func TestSyncWriteCapsContent(t *testing.T) {
	got := syncWriteCaps("content", false)
	if len(got) != 1 || got[0] != "data.write" {
		t.Fatalf("caps = %#v", got)
	}
}

