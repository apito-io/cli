package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var pluginAddCmd = &cobra.Command{
	Use:   "add <plugin-id-or-github-url>",
	Short: "Install a plugin through the Engine signed-registry installer",
	Long:  `Resolves a plugin id and calls Engine POST /system/plugin/install. Engine verifies catalog signature, checksum, and config.yml. Super-admin / plugins.deploy required.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPluginAdd(cmd, args[0]); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var githubURLPattern = regexp.MustCompile(`(?i)^https?://(?:www\.)?github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)

func parseGitHubOwnerRepo(url string) (owner, repo string, ok bool) {
	url = strings.TrimSpace(url)
	if m := githubURLPattern.FindStringSubmatch(url); len(m) == 3 {
		return m[1], strings.TrimSuffix(m[2], ".git"), true
	}
	return "", "", false
}

func installCatalogPlugin(account AccountConfig, pluginNameOrURL string) (string, error) {
	pluginID := pluginNameOrURL
	if _, repo, ok := parseGitHubOwnerRepo(pluginNameOrURL); ok {
		pluginID = repo
	}
	if err := requireHCPluginID(pluginID); err != nil {
		return pluginID, err
	}
	body, _ := json.Marshal(map[string]string{"id": pluginID})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(account.ServerURL, "/")+"/system/plugin/install", bytes.NewReader(body))
	if err != nil {
		return pluginID, err
	}
	req.Header.Set("Authorization", "Bearer "+account.Token())
	req.Header.Set("X-Use-Cookies", "false")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return pluginID, fmt.Errorf("failed to connect to server: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Code    string `json:"code"`
		Error   string `json:"error"`
	}
	_ = json.Unmarshal(raw, &parsed)
	if resp.StatusCode >= 300 || !parsed.Success {
		msg := parsed.Message
		if msg == "" {
			msg = string(raw)
		}
		if parsed.Error != "" {
			msg = msg + ": " + parsed.Error
		}
		return pluginID, fmt.Errorf("install failed (%s): %s", parsed.Code, msg)
	}
	if parsed.Message != "" {
		print_success(parsed.Message)
	}
	return pluginID, nil
}
