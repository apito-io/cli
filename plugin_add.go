package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
		accountName, _ := cmd.Flags().GetString("account")
		if accountName == "" {
			accountName = selectAccountInteractively()
			if accountName == "" {
				return
			}
		}
		addPlugin(args[0], accountName)
	},
}

func init() {
	pluginAddCmd.Flags().StringP("account", "a", "", "Account to use for installation")
}

var githubURLPattern = regexp.MustCompile(`(?i)^https?://(?:www\.)?github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)

func parseGitHubOwnerRepo(url string) (owner, repo string, ok bool) {
	url = strings.TrimSpace(url)
	if m := githubURLPattern.FindStringSubmatch(url); len(m) == 3 {
		return m[1], strings.TrimSuffix(m[2], ".git"), true
	}
	return "", "", false
}

func addPlugin(pluginNameOrURL, accountName string) {
	print_step("🔌 Add Plugin")
	if !checkServerConfig(accountName) {
		return
	}
	account, err := getAccountConfig(accountName)
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	pluginID := pluginNameOrURL
	if _, repo, ok := parseGitHubOwnerRepo(pluginNameOrURL); ok {
		pluginID = repo
	}

	body, _ := json.Marshal(map[string]string{"id": pluginID})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(account.ServerURL, "/")+"/system/plugin/install", bytes.NewReader(body))
	if err != nil {
		print_error(err.Error())
		return
	}
	req.Header.Set("Authorization", "Bearer "+account.CloudSyncKey)
	req.Header.Set("X-Use-Cookies", "false")
	req.Header.Set("Content-Type", "application/json")

	print_status(fmt.Sprintf("Installing %s via Engine %s", pluginID, account.ServerURL))
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		print_error("Failed to connect to server: " + err.Error())
		return
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
		print_error(fmt.Sprintf("Install failed (%s): %s", parsed.Code, msg))
		if parsed.Error != "" {
			print_error(parsed.Error)
		}
		return
	}
	print_success(parsed.Message)
}
