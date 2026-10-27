package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const telemetryURL = "https://telemetry.apito.io/capture"

var telemetryEvents = map[string]struct{}{
	"selfhost_started":    {},
	"cli_command_run":     {},
	"apito_start_success": {},
	"apito_start_failure": {},
	"cli_login_completed": {},
}

var telemetryProps = map[string]struct{}{
	"install_id":  {},
	"version":     {},
	"cli_version": {},
	"os":          {},
	"arch":        {},
	"driver":      {},
	"is_docker":   {},
	"command":     {},
	"stage":       {},
	"error_code":  {},
	"alias":       {},
	"product":     {},
	"surface":     {},
}

type telemetryState struct {
	Enabled   bool   `json:"enabled"`
	Decided   bool   `json:"decided"`
	InstallID string `json:"install_id"`
	LastEvent string `json:"last_event,omitempty"`
	LastBody  string `json:"last_body,omitempty"`
}

func telemetryPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".apito", "telemetry.json")
}

func loadTelemetryState() telemetryState {
	raw, err := os.ReadFile(telemetryPath())
	if err != nil {
		return telemetryState{}
	}
	var state telemetryState
	if json.Unmarshal(raw, &state) != nil {
		return telemetryState{}
	}
	return state
}

func saveTelemetryState(state telemetryState) {
	path := telemetryPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o600)
}

func telemetryDisabledByEnv(env []string) bool {
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		switch key {
		case "DO_NOT_TRACK":
			if value == "1" {
				return true
			}
		case "APITO_TELEMETRY":
			if value == "0" {
				return true
			}
		}
	}
	return false
}

func telemetryCI(env []string) bool {
	keys := map[string]struct{}{
		"CI": {}, "GITHUB_ACTIONS": {}, "GITLAB_CI": {}, "BUILDKITE": {},
		"CIRCLECI": {}, "JENKINS_URL": {}, "TF_BUILD": {},
	}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok || value == "" {
			continue
		}
		if _, found := keys[key]; found {
			return true
		}
	}
	return false
}

func telemetryForcedOn(env []string) bool {
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok && key == "APITO_TELEMETRY" && value == "1" {
			return true
		}
	}
	return false
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func consentAllows(env []string, tty bool, state telemetryState, prompt func() bool) bool {
	if telemetryDisabledByEnv(env) || telemetryCI(env) {
		return false
	}
	if telemetryForcedOn(env) {
		return true
	}
	if state.Decided {
		return state.Enabled
	}
	if !tty || flagJSON {
		return false
	}
	if prompt == nil {
		return false
	}
	return prompt()
}

func askTelemetry() bool {
	if flagJSON {
		return false
	}
	_, _ = os.Stderr.WriteString("Send anonymous CLI usage to Apito? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}

func telemetryAllowed(prompt bool) bool {
	state := loadTelemetryState()
	allowed := consentAllows(os.Environ(), stdinIsTTY(), state, func() bool {
		if !prompt {
			return false
		}
		enabled := askTelemetry()
		state.Decided = true
		state.Enabled = enabled
		if state.InstallID == "" {
			state.InstallID = newInstallID()
		}
		saveTelemetryState(state)
		return enabled
	})
	if allowed && state.InstallID == "" {
		state.InstallID = newInstallID()
		state.Decided = true
		state.Enabled = true
		saveTelemetryState(state)
	}
	return allowed
}

func newInstallID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "cli_local"
	}
	return "cli_" + hex.EncodeToString(buf)
}

func commonTelemetryProps() map[string]string {
	state := loadTelemetryState()
	if state.InstallID == "" {
		state.InstallID = newInstallID()
		saveTelemetryState(state)
	}
	docker := "false"
	if _, err := os.Stat("/.dockerenv"); err == nil {
		docker = "true"
	}
	return map[string]string{
		"install_id":  state.InstallID,
		"version":     version,
		"cli_version": version,
		"os":          runtime.GOOS,
		"arch":        runtime.GOARCH,
		"is_docker":   docker,
		"driver":      "sqlite",
		"product":     "apito",
		"surface":     "cli",
	}
}

func filterTelemetry(props map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range props {
		if _, ok := telemetryProps[key]; !ok {
			continue
		}
		if strings.Contains(value, "@") || strings.Contains(value, "ak_") || strings.Contains(value, "sk_") || strings.Contains(value, "eyJ") || strings.Contains(value, "/") || strings.Contains(value, "\\") || strings.Contains(value, "http") {
			continue
		}
		out[key] = value
	}
	return out
}

func captureTelemetry(event string, extra map[string]string) {
	if _, ok := telemetryEvents[event]; !ok {
		return
	}
	if !telemetryAllowed(false) {
		return
	}
	props := commonTelemetryProps()
	for key, value := range extra {
		props[key] = value
	}
	props = filterTelemetry(props)
	body, err := json.Marshal(map[string]any{
		"event":       event,
		"distinct_id": props["install_id"],
		"properties":  props,
	})
	if err != nil {
		return
	}
	state := loadTelemetryState()
	state.LastEvent = event
	state.LastBody = string(body)
	saveTelemetryState(state)
	go func() {
		req, err := http.NewRequest(http.MethodPost, telemetryURL, bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 1500 * time.Millisecond}
		res, err := client.Do(req)
		if err == nil && res.Body != nil {
			_ = res.Body.Close()
		}
	}()
}

func noteCLICommand(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	name := cmd.Name()
	if name == "" || name == "help" || name == "telemetry" {
		return
	}
	prompt := name == "start" || name == "login"
	if prompt {
		_ = telemetryAllowed(true)
	}
	captureTelemetry("cli_command_run", map[string]string{"command": name})
}

func noteStartResult(err error) {
	if err != nil {
		captureTelemetry("apito_start_failure", map[string]string{
			"stage":      "studio",
			"error_code": startErrorCode(err),
		})
		return
	}
	captureTelemetry("apito_start_success", map[string]string{"stage": "studio"})
	captureTelemetry("selfhost_started", map[string]string{"stage": "studio"})
}

func startErrorCode(err error) string {
	if err == nil {
		return "ok"
	}
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "port"):
		return "port_busy"
	case strings.Contains(text, "docker"):
		return "docker"
	case strings.Contains(text, "not ready"), strings.Contains(text, "nginx"):
		return "not_ready"
	default:
		return "start_failed"
	}
}

func noteLoginCompleted() {
	captureTelemetry("cli_login_completed", nil)
}

var telemetryCmd = &cobra.Command{
	Use:   "telemetry",
	Short: "Anonymous CLI usage preference",
}

var telemetryEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Allow anonymous CLI usage events",
	Run: func(cmd *cobra.Command, args []string) {
		state := loadTelemetryState()
		state.Enabled = true
		state.Decided = true
		if state.InstallID == "" {
			state.InstallID = newInstallID()
		}
		saveTelemetryState(state)
		if !flagJSON {
			print_success("CLI telemetry enabled")
		}
	},
}

var telemetryDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Stop anonymous CLI usage events",
	Run: func(cmd *cobra.Command, args []string) {
		state := loadTelemetryState()
		state.Enabled = false
		state.Decided = true
		saveTelemetryState(state)
		if !flagJSON {
			print_success("CLI telemetry disabled")
		}
	},
}

var telemetryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the telemetry preference and last payload",
	Run: func(cmd *cobra.Command, args []string) {
		state := loadTelemetryState()
		if flagJSON {
			raw, _ := json.Marshal(state)
			fmtPrintln(string(raw))
			return
		}
		status := "off"
		if state.Enabled {
			status = "on"
		}
		if !state.Decided {
			status = "unset"
		}
		println("telemetry: " + status)
		if state.LastEvent != "" {
			println("last event: " + state.LastEvent)
			println(state.LastBody)
		}
	},
}

func fmtPrintln(text string) {
	_, _ = os.Stdout.WriteString(text + "\n")
}

func println(text string) {
	_, _ = os.Stdout.WriteString(text + "\n")
}
