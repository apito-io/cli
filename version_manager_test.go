package main

import (
	"testing"
	"time"
)

func TestLatestSemverTag(t *testing.T) {
	got, err := latestSemverTag([]string{"latest", "local", "v0.1.1", "v0.1.3", "v0.1.2"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "v0.1.3" {
		t.Fatalf("latestSemverTag() = %q, want v0.1.3", got)
	}
}

func TestLatestSemverTagFallsBackToLatest(t *testing.T) {
	got, err := latestSemverTag([]string{"latest", "local"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "latest" {
		t.Fatalf("latestSemverTag() = %q, want latest", got)
	}
}

func TestStudioUpgradeAvailable(t *testing.T) {
	tests := []struct {
		name    string
		current string
		latest  string
		want    bool
	}{
		{name: "newer semver", current: "v0.1.1", latest: "v0.1.2", want: true},
		{name: "same semver", current: "v0.1.2", latest: "v0.1.2", want: false},
		{name: "older latest", current: "v0.1.2", latest: "v0.1.1", want: false},
		{name: "local pin", current: "local", latest: "v0.1.2", want: true},
		{name: "tracks latest", current: "latest", latest: "v0.1.2", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := studioUpgradeAvailable(test.current, test.latest); got != test.want {
				t.Fatalf("studioUpgradeAvailable(%q, %q) = %v, want %v", test.current, test.latest, got, test.want)
			}
		})
	}
}

func TestFormatStudioCurrentVersion(t *testing.T) {
	created := time.Date(2026, 10, 12, 11, 31, 0, 0, time.UTC)
	got := formatStudioCurrentVersion("local", created, []string{"ghcr.io/apito-io/studio:local"})
	want := "local (built " + created.Local().Format("2006-01-02") + ")"
	if got != want {
		t.Fatalf("formatStudioCurrentVersion() = %q, want %q", got, want)
	}

	got = formatStudioCurrentVersion("local", created, []string{
		"ghcr.io/apito-io/studio:local",
		"ghcr.io/apito-io/studio:v0.1.1",
		"ghcr.io/apito-io/studio:latest",
	})
	want = "local (release v0.1.1, built " + created.Local().Format("2006-01-02") + ")"
	if got != want {
		t.Fatalf("formatStudioCurrentVersion() = %q, want %q", got, want)
	}

	if got := formatStudioCurrentVersion("v0.1.1", created, nil); got != "v0.1.1" {
		t.Fatalf("release tag was rewritten: %q", got)
	}
}

func TestStudioTagDefaultsToLatest(t *testing.T) {
	if got := studioTag(&CLIConfig{}); got != "latest" {
		t.Fatalf("studioTag() = %q, want latest", got)
	}
	if got := studioTag(&CLIConfig{StudioVersion: " local "}); got != "local" {
		t.Fatalf("studioTag() did not trim configured tag: %q", got)
	}
}
