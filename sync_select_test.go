package main

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSelectedProjectAccess_ok(t *testing.T) {
	selected := SyncProject{ID: "kisti_jagt9", Name: "Kisti", ProjectType: "saas"}
	current := &SyncProject{ID: "kisti_jagt9", Name: "Kisti", ProjectType: "saas", PerTenantSeparateDatabase: false}

	got, err := validateSelectedProjectAccess("destination", selected, current, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != selected.ID || got.ProjectType != "saas" {
		t.Fatalf("got %+v", got)
	}
}

func TestValidateSelectedProjectAccess_tokenScopeMismatch(t *testing.T) {
	selected := SyncProject{ID: "kisti_jagt9", Name: "Kisti", ProjectType: "saas"}
	current := &SyncProject{ID: "ecommerce_abcdef", Name: "Ecommerce", ProjectType: "general"}

	_, err := validateSelectedProjectAccess("destination", selected, current, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"does not include project",
		"kisti_jagt9",
		"ecommerce_abcdef",
		"project_ids",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q missing %q", msg, want)
		}
	}
}

func TestValidateSelectedProjectAccess_currentProjectError(t *testing.T) {
	selected := SyncProject{ID: "kisti_jagt9", Name: "Kisti"}

	_, err := validateSelectedProjectAccess("destination", selected, nil, errors.New("graphql errors: forbidden"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot load") {
		t.Fatalf("got %q", err.Error())
	}
}
