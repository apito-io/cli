package main

import "testing"

func TestConsentPrecedence(t *testing.T) {
	off := telemetryState{Decided: true, Enabled: false}
	if consentAllows([]string{"DO_NOT_TRACK=1"}, true, telemetryState{Decided: true, Enabled: true}, func() bool { return true }) {
		t.Fatal("DO_NOT_TRACK must win")
	}
	if consentAllows([]string{"APITO_TELEMETRY=0"}, true, telemetryState{Decided: true, Enabled: true}, func() bool { return true }) {
		t.Fatal("APITO_TELEMETRY=0 must win")
	}
	if !consentAllows([]string{"APITO_TELEMETRY=1"}, true, off, nil) {
		t.Fatal("APITO_TELEMETRY=1 turns telemetry on")
	}
	if consentAllows(nil, true, off, func() bool { return true }) {
		t.Fatal("stored no must stay off")
	}
	if !consentAllows(nil, true, telemetryState{Decided: true, Enabled: true}, nil) {
		t.Fatal("stored yes must stay on")
	}
	prompted := false
	if consentAllows(nil, true, telemetryState{}, func() bool {
		prompted = true
		return false
	}) {
		t.Fatal("default answer is no")
	}
	if !prompted {
		t.Fatal("interactive unset state should prompt")
	}
	if consentAllows(nil, false, telemetryState{}, func() bool { return true }) {
		t.Fatal("non-tty must not prompt")
	}
}

func TestCIDisablesTelemetry(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "CIRCLECI", "JENKINS_URL", "TF_BUILD"} {
		if !telemetryCI([]string{key + "=1"}) {
			t.Fatalf("%s should count as CI", key)
		}
		if consentAllows([]string{key + "=true"}, true, telemetryState{}, func() bool { return true }) {
			t.Fatalf("%s must disable telemetry without a prompt", key)
		}
	}
}

func TestTelemetryFilterDropsSecrets(t *testing.T) {
	got := filterTelemetry(map[string]string{
		"install_id": "cli_abc",
		"command":    "start",
		"email":      "a@b.c",
		"token":      "ak_live",
		"path":       "/tmp/config.yml",
		"url":        "https://example.com",
		"jwt":        "eyJhbGci",
	})
	if got["install_id"] != "cli_abc" || got["command"] != "start" {
		t.Fatalf("props = %#v", got)
	}
	if len(got) != 2 {
		t.Fatalf("unexpected props %#v", got)
	}
}
