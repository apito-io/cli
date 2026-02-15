package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const (
	pluginRegistryURL = "https://raw.githubusercontent.com/apito-io/plugins/main/plugins.json"
	githubAPIBase     = "https://api.github.com"
	noReleaseMessage  = `No plugin release found for this repository.
This is not a valid plugin repository.
Follow the plugin deployment docs at https://apito.io/docs/plugin-deploy-docs for more information.`
)

// PluginRegistryResponse matches the JSON at plugins.json
type PluginRegistryResponse struct {
	Plugins []PluginRegistryEntry `json:"plugins"`
}

// PluginRegistryEntry represents a single plugin in the registry
type PluginRegistryEntry struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	GitHubURL string `json:"github_url"`
}

// pluginGitHubRelease represents the GitHub /releases/latest API response for plugins
type pluginGitHubRelease struct {
	TagName string                 `json:"tag_name"`
	Assets  []pluginGitHubAsset     `json:"assets"`
}

// pluginGitHubAsset represents a release asset
type pluginGitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var pluginAddCmd = &cobra.Command{
	Use:   "add <plugin-name-or-github-url>",
	Short: "Add a plugin from the official registry or a GitHub repository",
	Long:  `Add a plugin by name (from the official registry) or by GitHub URL. Downloads the latest release and deploys it.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		accountName, _ := cmd.Flags().GetString("account")
		if accountName == "" {
			accountName = selectAccountInteractively()
			if accountName == "" {
				return
			}
		}
		forceReplace, _ := cmd.Flags().GetBool("replace")
		addPlugin(args[0], accountName, forceReplace)
	},
}

func init() {
	pluginAddCmd.Flags().StringP("account", "a", "", "Account to use for deployment")
	pluginAddCmd.Flags().Bool("replace", false, "Delete existing plugin before deployment")
}

func fetchPluginRegistry() (*PluginRegistryResponse, error) {
	resp, err := http.Get(pluginRegistryURL)
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}
	var reg PluginRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return &reg, nil
}

func findPluginInRegistry(reg *PluginRegistryResponse, id string) *PluginRegistryEntry {
	for i := range reg.Plugins {
		if reg.Plugins[i].ID == id {
			return &reg.Plugins[i]
		}
	}
	return nil
}

// githubURLPattern matches https://github.com/owner/repo and variants with .git
var githubURLPattern = regexp.MustCompile(`(?i)^https?://(?:www\.)?github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`)

func parseGitHubOwnerRepo(url string) (owner, repo string, ok bool) {
	url = strings.TrimSpace(url)
	if m := githubURLPattern.FindStringSubmatch(url); len(m) == 3 {
		return m[1], strings.TrimSuffix(m[2], ".git"), true
	}
	return "", "", false
}

func fetchLatestRelease(owner, repo string) (*pluginGitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, owner, repo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	var rel pluginGitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &rel, nil
}

func findPlatformAsset(release *pluginGitHubRelease, pluginName string) *pluginGitHubAsset {
	suffix := "-" + runtime.GOOS + "-" + runtime.GOARCH + ".zip"
	prefix := pluginName + "-"
	for i := range release.Assets {
		a := &release.Assets[i]
		if strings.HasPrefix(a.Name, prefix) && strings.HasSuffix(a.Name, suffix) {
			return a
		}
	}
	return nil
}

func downloadAndExtractPlugin(downloadURL, pluginName string) (pluginDir string, err error) {
	downloadTmpDir, err := os.MkdirTemp("", "apito-plugin-download-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(downloadTmpDir)

	zipPath, err := downloadFileWithProgress(downloadURL, downloadTmpDir)
	if err != nil {
		return "", err
	}

	extractDir, err := extractArchiveToTemp(zipPath)
	if err != nil {
		return "", fmt.Errorf("extract archive: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(extractDir)
		}
	}()

	configPath := filepath.Join(extractDir, "config.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plugin package missing config.yml")
	}

	config, err := readPluginConfig(extractDir)
	if err != nil {
		return "", fmt.Errorf("invalid config: %w", err)
	}

	binaryPath := filepath.Join(extractDir, config.Plugin.BinaryPath)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("plugin package missing binary at %s", config.Plugin.BinaryPath)
	}

	return extractDir, nil
}

func addPlugin(pluginNameOrURL, accountName string, forceReplace bool) {
	print_step("🔌 Add Plugin")

	var owner, repo, pluginID string

	if o, r, ok := parseGitHubOwnerRepo(pluginNameOrURL); ok {
		owner, repo, pluginID = o, r, r
	} else {
		reg, err := fetchPluginRegistry()
		if err != nil {
			print_error("Failed to fetch plugin registry: " + err.Error())
			return
		}
		entry := findPluginInRegistry(reg, pluginNameOrURL)
		if entry == nil {
			print_error("Plugin '" + pluginNameOrURL + "' not found in registry.")
			return
		}
		pluginID = entry.ID
		var o, r string
		o, r, ok = parseGitHubOwnerRepo(entry.GitHubURL)
		if !ok {
			print_error("Invalid github_url in registry for plugin " + pluginID)
			return
		}
		owner, repo = o, r
	}

	print_status(fmt.Sprintf("Fetching latest release for %s/%s", owner, repo))

	release, err := fetchLatestRelease(owner, repo)
	if err != nil {
		print_error("Failed to fetch release: " + err.Error())
		return
	}
	if release == nil {
		print_error(noReleaseMessage)
		return
	}

	asset := findPlatformAsset(release, pluginID)
	if asset == nil {
		print_error(noReleaseMessage)
		return
	}

	print_status(fmt.Sprintf("Downloading %s", asset.Name))

	pluginDir, err := downloadAndExtractPlugin(asset.BrowserDownloadURL, pluginID)
	if err != nil {
		print_error("Failed to download plugin: " + err.Error())
		return
	}
	defer os.RemoveAll(pluginDir)

	deployPlugin(pluginDir, accountName, forceReplace)
}
