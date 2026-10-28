package main

import (
	"strings"
	"testing"
)

func TestFormatStudioStatusHidesPasswordAndShowsLivePorts(t *testing.T) {
	text := formatStudioStatus(studioStatusView{
		ContainerState: "running",
		Image:          "ghcr.io/apito-io/studio:v0.1.12",
		Version:        "v0.1.12",
		CLIVersion:     "v0.5.5",
		Email:          "oss@apito.io",
		ConsoleLive:    true,
		AdminLive:      true,
		EngineLive:     true,
	})
	if studioOverall(studioStatusView{ContainerState: "running", ConsoleLive: true, AdminLive: true, EngineLive: true}) != "running" {
		t.Fatal("healthy studio should be running")
	}
	for _, want := range []string{
		"Overall:   running",
		"Container: apito-studio (running)",
		"Version:   v0.1.12",
		"CLI:       v0.5.5",
		"ports 4055 and 5055",
		"Console: live  http://localhost:4055",
		"Engine:  live  http://localhost:5055",
		"Email:    oss@apito.io",
		"Password: hidden",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "super-secret") {
		t.Fatal("status printed a password")
	}
}

func TestStudioOverallFailsWhenEngineIsDown(t *testing.T) {
	view := studioStatusView{ContainerState: "running", ConsoleLive: true, AdminLive: true, EngineLive: false}
	if studioOverall(view) == "running" {
		t.Fatal("a dead engine must not count as running")
	}
	text := formatStudioStatus(view)
	if !strings.Contains(text, "Overall:   degraded") || !strings.Contains(text, "Engine:  not answering") {
		t.Fatalf("status = %s", text)
	}
	if studioOverall(studioStatusView{ContainerState: "exited"}) != "down" {
		t.Fatal("stopped container should be down")
	}
}
