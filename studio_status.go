package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type studioStatusView struct {
	ContainerState string
	Image          string
	Version        string
	CLIVersion     string
	Email          string
	ConsoleLive    bool
	AdminLive      bool
	EngineLive     bool
}

func studioOverall(view studioStatusView) string {
	if view.ContainerState != "running" {
		return "down"
	}
	if view.ConsoleLive && view.AdminLive && view.EngineLive {
		return "running"
	}
	return "degraded"
}

func formatStudioStatus(view studioStatusView) string {
	console := endpointWord(view.ConsoleLive)
	admin := endpointWord(view.AdminLive)
	engine := endpointWord(view.EngineLive)
	email := strings.TrimSpace(view.Email)
	if email == "" {
		email = "not set"
	}
	version := strings.TrimSpace(view.Version)
	if version == "" {
		version = "unknown"
	}
	image := strings.TrimSpace(view.Image)
	if image == "" {
		image = "none"
	}
	cli := strings.TrimSpace(view.CLIVersion)
	if cli == "" {
		cli = "dev"
	}
	state := view.ContainerState
	if state == "" {
		state = "missing"
	}
	return fmt.Sprintf(`Studio
  Overall:   %s
  Container: %s (%s)
  Image:     %s
  Version:   %s
  CLI:       %s

Running
  Studio:  %s  ports %s and %s
  Console: %s  http://localhost:%s
  Admin:   %s  http://localhost:%s/admin
  Engine:  %s  http://localhost:%s

Login
  Email:    %s
  Password: hidden
`, studioOverall(view), studioContainerName, state, image, version, cli,
		state, studioPublishPort, studioEnginePort,
		console, studioPublishPort,
		admin, studioPublishPort,
		engine, studioEnginePort,
		email)
}

func endpointWord(live bool) string {
	if live {
		return "live"
	}
	return "not answering"
}

func studioImageTag(image string) string {
	image = strings.TrimSpace(image)
	if i := strings.LastIndex(image, ":"); i >= 0 && i < len(image)-1 {
		return image[i+1:]
	}
	return ""
}

func studioAdminEmail() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	envMap, err := readEnvFile(filepath.Join(home, ".apito", "bin", studioEngineEnvName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(envMap["BOOTSTRAP_ADMIN_EMAIL"])
}

func collectStudioStatus() studioStatusView {
	view := studioStatusView{
		ContainerState: "missing",
		CLIVersion:     version,
		Email:          studioAdminEmail(),
	}
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}|{{.Config.Image}}", studioContainerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return view
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "|", 2)
	if parts[0] != "" {
		view.ContainerState = parts[0]
	}
	if len(parts) == 2 {
		view.Image = parts[1]
		view.Version = studioImageTag(parts[1])
	}
	if view.ContainerState != "running" {
		return view
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	console, consoleErr := probeURL(client, "http://127.0.0.1:"+studioPublishPort+"/")
	admin, adminErr := probeURL(client, "http://127.0.0.1:"+studioPublishPort+"/admin")
	engine, engineErr := probeURL(client, "http://127.0.0.1:"+studioEnginePort+"/")
	view.ConsoleLive = consoleErr == nil && acceptableConsoleStatus(console.Status)
	view.AdminLive = adminErr == nil && acceptableAdminStatus(admin.Status)
	view.EngineLive = engineErr == nil && engine.Status > 0 && engine.Status < 500
	return view
}
