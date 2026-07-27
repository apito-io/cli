package main

import (
	"os"
	"testing"
)

func TestValidateProttoyAstroSchemaJSON(t *testing.T) {
	raw, err := os.ReadFile("../../udbhabon/prottoy/prottoy-astro/schema.json")
	if err != nil {
		t.Skip(err)
	}
	models, err := parseModelsFromSchemaJSON(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if issues := validateSyncModels(models); len(issues) != 0 {
		t.Fatalf("%s", formatSchemaValidationReport(issues))
	}
}
