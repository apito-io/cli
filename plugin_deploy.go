package main

import (
	"archive/tar"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Plugin deployment commands

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Create, check, build, and publish a plugin",
	Long: `Plugin commands work on a local repository. They do not connect to an account or an engine.

apito plugin                searchable list of name and version, then check, build, or push
apito plugin init [name]    clone a system or payment boilerplate
apito plugin list           same as apito plugin
apito plugin build [dir]    go vet and build
apito plugin check [dir]    required files and the GitHub release workflow
apito plugin push [dir]     bump the version, commit, tag, and push
apito plugin deploy [dir]   same as push`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := ensureCLIConfig(); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPluginList(cmd); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginStatusCmd = &cobra.Command{
	Use:   "status [plugin-id]",
	Short: "Show local files and engine install state",
	Long:  `Compares the account plugin directory with GET /system/plugin on that account. LOCAL is the directory. REMOTE is the engine copy and its version. INSTALLED is whether that engine has the plugin. ACTIVATED is the engine enable flag.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := ""
		if len(args) > 0 {
			id = args[0]
		}
		if err := runPluginStatus(cmd, id); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Clone the Go hello-world system plugin into the account plugin path",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := ""
		if len(args) > 0 {
			name = args[0]
		}
		if err := runPluginCreate(cmd, name); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginDeployCmd = &cobra.Command{
	Use:   "deploy [plugin-id]",
	Short: "Copy a plugin from one account to another",
	Long:  `Reads the source account path and writes the destination account path. A cloud destination installs the signed catalog release. An id that is not in the catalog is not uploaded.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		id := ""
		if len(args) > 0 {
			id = args[0]
		}
		if err := runPluginDeploy(cmd, id); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <plugin-id>",
	Short: "Uninstall a plugin from the account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPluginRemove(cmd, args[0]); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

// PluginConfig represents the plugin configuration structure (matches config.yml)
type PluginConfig struct {
	Plugin struct {
		ID               string `yaml:"id"`
		Language         string `yaml:"language"`
		Title            string `yaml:"title"`
		Description      string `yaml:"description"`
		Type             string `yaml:"type"`
		Version          string `yaml:"version"`
		Author           string `yaml:"author"`
		RepositoryURL    string `yaml:"repository_url"`
		Branch           string `yaml:"branch"`
		BinaryPath       string `yaml:"binary_path"`
		ExportedVariable string `yaml:"exported_variable"`
		Enable           bool   `yaml:"enable"`
		Debug            bool   `yaml:"debug"`
		HandshakeConfig  struct {
			ProtocolVersion  int    `yaml:"protocol_version"`
			MagicCookieKey   string `yaml:"magic_cookie_key"`
			MagicCookieValue string `yaml:"magic_cookie_value"`
		} `yaml:"handshake_config"`
		EnvVars []struct {
			Key   string `yaml:"key"`
			Value string `yaml:"value"`
		} `yaml:"env_vars"`
		UIConfig *struct {
			Enable    bool   `yaml:"enable"`
			DistPath  string `yaml:"dist_path"`
			EntryPath string `yaml:"entry_path"`
		} `yaml:"ui_config,omitempty"`
	} `yaml:"plugin"`
}

// PluginOperationResponse represents the API response structures
type PluginOperationResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	PluginID string `json:"plugin_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

type PluginListResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Plugins []PluginStatusInfo `json:"plugins"`
}

type PluginStatusInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Language    string `json:"language"`
	Type        string `json:"type"`
	Enable      bool   `json:"enable"`
	Debug       bool   `json:"debug"`
	LastUpdated string `json:"last_updated"`
	Error       string `json:"error,omitempty"`
}

func init() {
	pluginCmd.PersistentFlags().StringP("dir", "d", "", "Directory to search, or the parent for init")
	pluginInitCmd.Flags().String("type", "", "Boilerplate: system or payment")
	pluginPushCmd.Flags().StringP("message", "m", "", "Commit message for the version bump")
	pluginPushCmd.Flags().Bool("minor", false, "Bump the minor version instead of the patch")
	pluginPushCmd.Flags().Bool("major", false, "Bump the major version instead of the patch")
	pluginCmd.AddCommand(pluginInitCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginBuildCmd)
	pluginCmd.AddCommand(pluginCheckCmd)
	pluginCmd.AddCommand(pluginPushCmd)
}

func runPluginCreate(cmd *cobra.Command, name string) error {
	accountName, root, err := pluginAccountAndRoot(cmd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		namePrompt := promptui.Prompt{
			Label:    "Plugin name (prefixed with hc-)",
			Validate: validatePluginName,
		}
		name, err = namePrompt.Run()
		if err != nil {
			return fmt.Errorf("plugin name cancelled")
		}
	}
	if err := validatePluginName(name); err != nil {
		return err
	}
	if !strings.HasPrefix(name, "hc-") {
		name = "hc-" + name
	}
	dest := filepath.Join(root, name)
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin directory already exists: %s", dest)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	repoURL := "https://github.com/apito-io/apito-hello-world-go-plugin.git"
	print_step("Create plugin")
	print_status(fmt.Sprintf("Account: %s", accountName))
	print_status("Cloning " + repoURL)
	if err := runGitClone(repoURL, dest); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dest, ".git")); err != nil {
		print_warning("Failed to remove .git: " + err.Error())
	}
	if err := rewritePluginIdentity(dest, name); err != nil {
		return err
	}
	print_success("Plugin scaffold created: " + dest)
	print_status("Next: apito plugin build " + name)
	return nil
}

func listPlugins(accountName string) {
	if !checkServerConfig(accountName) {
		return
	}

	account, err := getAccountConfig(accountName)
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	// Display which account is being used
	print_step(fmt.Sprintf("📋 Listing Plugins from Account: %s", accountName))
	print_status(fmt.Sprintf("Server: %s", account.ServerURL))
	print_status("")

	serverURL := account.ServerURL
	cloudSyncKey := account.Token()

	// Make API request
	req, err := http.NewRequest("GET", serverURL+"/system/plugin", nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("Authorization", "Bearer "+cloudSyncKey)
	req.Header.Set("X-Use-Cookies", "false")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		print_error("Failed to connect to server: " + err.Error())
		return
	}
	defer resp.Body.Close()

	var response PluginListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		print_error("Failed to decode response: " + err.Error())
		return
	}

	if !response.Success {
		print_error("API error: " + response.Message)
		return
	}

	// Display results
	if len(response.Plugins) == 0 {
		print_status(fmt.Sprintf("No plugins found in account '%s'", accountName))
		return
	}

	print_success(fmt.Sprintf("Found %d plugins in account '%s':", len(response.Plugins), accountName))
	for _, plugin := range response.Plugins {
		status := plugin.Status
		if plugin.Error != "" {
			status += " (error: " + plugin.Error + ")"
		}

		enabledStr := "disabled"
		if plugin.Enable {
			enabledStr = "enabled"
		}

		debugStr := ""
		if plugin.Debug {
			debugStr = " [debug]"
		}

		print_status(fmt.Sprintf("  📦 %s v%s (%s) - %s - %s%s",
			plugin.ID, plugin.Version, plugin.Language, status, enabledStr, debugStr))
		if plugin.Title != "" {
			print_status(fmt.Sprintf("     Title: %s", plugin.Title))
		}
		if plugin.LastUpdated != "" {
			print_status(fmt.Sprintf("     Updated: %s", plugin.LastUpdated))
		}
	}
}

func getPluginStatus(pluginID, accountName string) {
	if !checkServerConfig(accountName) {
		return
	}

	print_step(fmt.Sprintf("🔍 Plugin Status: %s", pluginID))

	account, err := getAccountConfig(accountName)
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.Token()

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/system/plugin/%s", serverURL, pluginID), nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("Authorization", "Bearer "+cloudSyncKey)
	req.Header.Set("X-Use-Cookies", "false")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		print_error("Failed to connect to server: " + err.Error())
		return
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		print_error(fmt.Sprintf("Server returned status %d: %s", resp.StatusCode, string(body)))
		return
	}

	// Read response body for better error handling
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		print_error("Failed to read response body: " + err.Error())
		return
	}

	var response struct {
		Success bool             `json:"success"`
		Message string           `json:"message"`
		Plugin  PluginStatusInfo `json:"plugin"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		print_error(fmt.Sprintf("Failed to decode JSON response: %s", err.Error()))
		print_error(fmt.Sprintf("Server response: %s", string(body)))
		return
	}

	if !response.Success {
		print_error("API error: " + response.Message)
		return
	}

	// Display detailed status
	plugin := response.Plugin
	print_success(fmt.Sprintf("Plugin Status: %s", plugin.ID))
	print_status(fmt.Sprintf("  Title: %s", plugin.Title))
	print_status(fmt.Sprintf("  Version: %s", plugin.Version))
	print_status(fmt.Sprintf("  Language: %s", plugin.Language))
	print_status(fmt.Sprintf("  Type: %s", plugin.Type))
	print_status(fmt.Sprintf("  Status: %s", plugin.Status))
	print_status(fmt.Sprintf("  Enabled: %v", plugin.Enable))
	print_status(fmt.Sprintf("  Debug Mode: %v", plugin.Debug))
	if plugin.LastUpdated != "" {
		print_status(fmt.Sprintf("  Last Updated: %s", plugin.LastUpdated))
	}
	if plugin.Error != "" {
		print_error(fmt.Sprintf("  Error: %s", plugin.Error))
	}
}

func restartPlugin(pluginID, accountName string) {
	controlPlugin(pluginID, "restart", accountName)
}

func stopPlugin(pluginID, accountName string) {
	controlPlugin(pluginID, "stop", accountName)
}

func deletePlugin(pluginID, accountName string) {
	if !checkServerConfig(accountName) {
		return
	}

	// Ask for confirmation before deletion
	if !confirmSensitiveOperation("delete", pluginID, accountName) {
		return
	}

	print_step(fmt.Sprintf("🗑️  Deleting Plugin: %s", pluginID))

	statusCode, body, err := performPluginDeleteRequest(pluginID, accountName)
	if err != nil {
		print_error("Failed to delete plugin: " + err.Error())
		return
	}

	if statusCode == http.StatusNotFound {
		print_warning("Plugin not found on server")
		return
	}

	if len(body) == 0 {
		print_error("Server returned empty response during deletion")
		return
	}

	var response PluginOperationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		print_error(fmt.Sprintf("Failed to decode response: %v", err))
		print_error(fmt.Sprintf("Server response: %s", strings.TrimSpace(string(body))))
		return
	}

	if response.Success {
		print_success(response.Message)
	} else {
		print_error("Delete failed: " + response.Message)
		if response.Error != "" {
			print_error("Error details: " + response.Error)
		}
	}
}

// Helper functions

// confirmSensitiveOperation asks for confirmation before performing sensitive operations
func confirmSensitiveOperation(operation, pluginID, accountName string, pluginInfo ...string) bool {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return false
	}

	// Get account info
	var account AccountConfig
	var actualAccountName string
	if accountName != "" {
		if acc, exists := cfg.Accounts[accountName]; exists {
			account = acc
			actualAccountName = accountName
		} else {
			print_error(fmt.Sprintf("Account '%s' does not exist", accountName))
			return false
		}
	} else {
		// Use default account
		if cfg.DefaultAccount != "" {
			if acc, exists := cfg.Accounts[cfg.DefaultAccount]; exists {
				account = acc
				actualAccountName = cfg.DefaultAccount
			}
		}
	}

	// Display operation details
	print_step(fmt.Sprintf("⚠️  Confirmation Required: %s", strings.Title(operation)))
	print_status("")
	print_status("Operation Details:")
	print_status(fmt.Sprintf("  Action: %s", strings.Title(operation)))
	print_status(fmt.Sprintf("  Plugin: %s", pluginID))

	// Show account info
	if actualAccountName != "" {
		print_status(fmt.Sprintf("  Account: %s", actualAccountName))
		print_status(fmt.Sprintf("  Server: %s", account.ServerURL))
	} else {
		print_status("  Account: (default - none set)")
	}

	// Show additional plugin info if provided
	if len(pluginInfo) > 0 {
		for _, info := range pluginInfo {
			print_status(fmt.Sprintf("  %s", info))
		}
	}

	print_status("")
	print_warning("This operation cannot be undone!")

	// Confirmation prompt
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to %s plugin '%s'? (y/N)", operation, pluginID),
		IsConfirm: true,
		Default:   "n",
	}

	if _, err := confirmPrompt.Run(); err != nil {
		print_status("Operation cancelled")
		return false
	}

	return true
}

func controlPlugin(pluginID, action, accountName string) {
	if !checkServerConfig(accountName) {
		return
	}

	// Ask for confirmation before control operations
	if !confirmSensitiveOperation(action, pluginID, accountName) {
		return
	}

	print_step(fmt.Sprintf("🎛️  %s Plugin: %s", strings.Title(action), pluginID))

	account, err := getAccountConfig(accountName)
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.Token()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/system/plugin/%s/%s", serverURL, pluginID, action), nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("Authorization", "Bearer "+cloudSyncKey)
	req.Header.Set("X-Use-Cookies", "false")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		print_error("Failed to connect to server: " + err.Error())
		return
	}
	defer resp.Body.Close()

	var response PluginOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		print_error("Failed to decode response: " + err.Error())
		return
	}

	if response.Success {
		print_success(response.Message)
	} else {
		print_error(fmt.Sprintf("%s failed: %s", strings.Title(action), response.Message))
		if response.Error != "" {
			print_error("Error details: " + response.Error)
		}
	}
}

func performPluginDeleteRequest(pluginID, accountName string) (int, []byte, error) {
	account, err := getAccountConfig(accountName)
	if err != nil {
		return 0, nil, err
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.Token()

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/system/plugin/%s", serverURL, pluginID), nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+cloudSyncKey)
	req.Header.Set("X-Use-Cookies", "false")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}

	return resp.StatusCode, body, nil
}

func extractAPIMessage(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return ""
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err == nil {
		if msg, ok := payload["message"]; ok {
			switch v := msg.(type) {
			case string:
				if v != "" {
					if errMsg, ok := payload["error"].(string); ok && errMsg != "" && errMsg != v {
						return v + ": " + errMsg
					}
					return v
				}
			default:
				if marshaled, err := json.Marshal(v); err == nil {
					text := strings.TrimSpace(string(marshaled))
					if text != "" && text != "null" {
						return text
					}
				}
			}
		}

		if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
			return errMsg
		}
	}

	return trimmed
}

func checkServerConfig(accountName string) bool {
	// Get account configuration
	_, err := getAccountConfig(accountName)
	if err != nil {
		print_error("Account configuration error: " + err.Error())
		return false
	}

	// Account config is already validated in getAccountConfig
	return true
}

func validatePluginName(input string) error {
	if len(input) < 3 {
		return fmt.Errorf("plugin name must be at least 3 characters long")
	}
	if strings.Contains(input, " ") {
		return fmt.Errorf("plugin name cannot contain spaces")
	}
	return nil
}

func runGitClone(repoURL, targetDir string) error {
	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s: %w", repoURL, err)
	}
	return nil
}

// buildPlugin is now implemented in plugin_build.go

// pluginUIFile is the console bundle to ship next to the binary.
// The engine reads entry_path (default ui.js) from the plugin directory.
// A dist_path file is packed under that same name when ui.js is absent.
func pluginUIFile(pluginDir string, config *PluginConfig) (string, string, bool, error) {
	entry := "ui.js"
	var dist string
	if config != nil && config.Plugin.UIConfig != nil {
		if name := filepath.Base(strings.TrimSpace(config.Plugin.UIConfig.EntryPath)); name != "" && name != "." {
			entry = name
		}
		dist = strings.TrimSpace(config.Plugin.UIConfig.DistPath)
	}
	if entry == "" || strings.Contains(entry, "..") {
		return "", "", false, fmt.Errorf("invalid ui entry path")
	}
	direct := filepath.Join(pluginDir, entry)
	if st, err := os.Stat(direct); err == nil && !st.IsDir() {
		return direct, entry, true, nil
	}
	if dist != "" && !strings.Contains(dist, "..") {
		compiled := filepath.Join(pluginDir, dist)
		if st, err := os.Stat(compiled); err == nil && !st.IsDir() {
			return compiled, entry, true, nil
		}
	}
	return "", entry, false, nil
}

func pluginUIConfigured(config *PluginConfig) bool {
	if config == nil || config.Plugin.UIConfig == nil {
		return false
	}
	ui := config.Plugin.UIConfig
	return ui.Enable || strings.TrimSpace(ui.DistPath) != "" || strings.TrimSpace(ui.EntryPath) != ""
}

func createDeploymentPackage(pluginDir string, config *PluginConfig, includeSecrets bool) (string, error) {
	// Validate required files exist before creating package
	configPath := filepath.Join(pluginDir, "config.yml")
	binaryPath := filepath.Join(pluginDir, config.Plugin.BinaryPath)

	// Check if config.yml exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", fmt.Errorf("config.yml not found in plugin directory %s - this is not a valid plugin", pluginDir)
	}

	// Check if binary exists
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("binary file %s not found in plugin directory %s - plugin deployment requires both config.yml and binary", config.Plugin.BinaryPath, pluginDir)
	}

	// Create a tar.gz package with plugin files
	packagePath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-deploy-%d.tar.gz", config.Plugin.ID, time.Now().Unix()))

	file, err := os.Create(packagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create deployment package: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Add config.yml to tar. Secret env values stay blank unless asked.
	configSource := configPath
	if !includeSecrets {
		raw, err := os.ReadFile(configPath)
		if err != nil {
			return "", fmt.Errorf("failed to read config.yml: %w", err)
		}
		redacted, err := redactPluginConfigSecrets(raw)
		if err != nil {
			return "", fmt.Errorf("failed to blank secret env values: %w", err)
		}
		tmp, err := os.CreateTemp("", "apito-plugin-config-*.yml")
		if err != nil {
			return "", err
		}
		if _, err := tmp.Write(redacted); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		if err := tmp.Close(); err != nil {
			os.Remove(tmp.Name())
			return "", err
		}
		defer os.Remove(tmp.Name())
		configSource = tmp.Name()
	}
	if err := addFileToTar(tw, configSource, "config.yml"); err != nil {
		return "", fmt.Errorf("failed to add config.yml to deployment package: %w", err)
	}

	// Add binary to tar
	binaryName := filepath.Base(config.Plugin.BinaryPath)
	if err := addFileToTar(tw, binaryPath, binaryName); err != nil {
		return "", fmt.Errorf("failed to add binary %s to deployment package: %w", binaryName, err)
	}

	uiSrc, uiName, uiFound, err := pluginUIFile(pluginDir, config)
	if err != nil {
		return "", err
	}
	if uiFound {
		if err := addFileToTar(tw, uiSrc, uiName); err != nil {
			return "", fmt.Errorf("failed to add UI file %s to deployment package: %w", uiName, err)
		}
		print_status(fmt.Sprintf("✅ Added UI file: %s", uiName))
	} else if pluginUIConfigured(config) {
		print_warning("⚠️  UI file not found, skipping UI deployment")
	}

	uiConfigJSONPath := filepath.Join(pluginDir, "ui", "config.json")
	if _, err := os.Stat(uiConfigJSONPath); err == nil {
		if err := addFileToTar(tw, uiConfigJSONPath, "ui/config.json"); err != nil {
			return "", fmt.Errorf("failed to add UI config.json to deployment package: %w", err)
		}
		print_status("✅ Added UI config.json")
	}

	return packagePath, nil
}

// Helper function to add a file to the tar archive
func addFileToTar(tw *tar.Writer, filePath, nameInTar string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    nameInTar,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := io.Copy(tw, file); err != nil {
		return err
	}

	return nil
}

// BinaryInfo represents information about a binary file
type BinaryInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Format       string `json:"format"`
	Error        string `json:"error,omitempty"`
}

// ServerPlatformInfo represents server platform information
type ServerPlatformInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
}

// ServerPlatformResponse represents the platform API response
type ServerPlatformResponse struct {
	Success  bool               `json:"success"`
	Message  string             `json:"message"`
	Platform ServerPlatformInfo `json:"platform"`
}

// detectBinaryFormat analyzes a binary file to determine its platform and architecture
func detectBinaryFormat(filePath string) BinaryInfo {
	file, err := os.Open(filePath)
	if err != nil {
		return BinaryInfo{Error: fmt.Sprintf("Failed to open file: %v", err)}
	}
	defer file.Close()

	// Try to parse as ELF (Linux/Unix)
	if elfFile, err := elf.NewFile(file); err == nil {
		defer elfFile.Close()

		var arch string
		switch elfFile.Machine {
		case elf.EM_X86_64:
			arch = "amd64"
		case elf.EM_386:
			arch = "386"
		case elf.EM_AARCH64:
			arch = "arm64"
		case elf.EM_ARM:
			arch = "arm"
		default:
			arch = fmt.Sprintf("unknown(%d)", elfFile.Machine)
		}

		return BinaryInfo{
			OS:           "linux",
			Architecture: arch,
			Format:       "elf",
		}
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Try to parse as Mach-O (macOS)
	if machoFile, err := macho.NewFile(file); err == nil {
		defer machoFile.Close()

		var arch string
		switch machoFile.Cpu {
		case macho.CpuAmd64:
			arch = "amd64"
		case macho.Cpu386:
			arch = "386"
		case macho.CpuArm64:
			arch = "arm64"
		case macho.CpuArm:
			arch = "arm"
		default:
			arch = fmt.Sprintf("unknown(%d)", machoFile.Cpu)
		}

		return BinaryInfo{
			OS:           "darwin",
			Architecture: arch,
			Format:       "macho",
		}
	}

	// Reset file pointer
	file.Seek(0, 0)

	// Try to parse as PE (Windows)
	if peFile, err := pe.NewFile(file); err == nil {
		defer peFile.Close()

		var arch string
		switch peFile.Machine {
		case pe.IMAGE_FILE_MACHINE_AMD64:
			arch = "amd64"
		case pe.IMAGE_FILE_MACHINE_I386:
			arch = "386"
		case pe.IMAGE_FILE_MACHINE_ARM64:
			arch = "arm64"
		case pe.IMAGE_FILE_MACHINE_ARMNT:
			arch = "arm"
		default:
			arch = fmt.Sprintf("unknown(%d)", peFile.Machine)
		}

		return BinaryInfo{
			OS:           "windows",
			Architecture: arch,
			Format:       "pe",
		}
	}

	return BinaryInfo{Error: "Unknown binary format"}
}

// getServerPlatformInfo queries the server for its platform information

// destinationRebuildReason reports why the local binary cannot run on the server.
// An empty string means the binary already matches.
func destinationRebuildReason(binary BinaryInfo, server ServerPlatformInfo) string {
	if binary.Error != "" {
		return ""
	}
	if binary.OS != server.OS {
		return fmt.Sprintf("plugin OS %s does not match server OS %s", binary.OS, server.OS)
	}
	if binary.Architecture != server.Architecture {
		return fmt.Sprintf("plugin architecture %s does not match server architecture %s", binary.Architecture, server.Architecture)
	}
	return ""
}

func describePluginProcess(info *PluginStatusInfo) string {
	if info == nil {
		return "not installed"
	}
	version := strings.TrimSpace(info.Version)
	if version == "" {
		version = "unknown"
	}
	status := strings.TrimSpace(info.Status)
	if status == "" {
		status = "unknown"
	}
	updated := strings.TrimSpace(info.LastUpdated)
	if updated == "" {
		updated = "unknown"
	}
	return fmt.Sprintf("%s version %s, process %s, files updated %s", info.ID, version, status, updated)
}

func pluginReplaceSentence(before, after *PluginStatusInfo) string {
	beforeRunning := before != nil && before.Status == "loaded"
	afterRunning := after != nil && after.Status == "loaded"
	switch {
	case beforeRunning && afterRunning:
		return "Reloaded: the previous process was stopped and the uploaded binary is running."
	case afterRunning:
		return "Loaded: the plugin was not running before, and the uploaded binary is running now."
	case beforeRunning:
		return "Not running: the previous process was stopped and the uploaded binary did not stay up."
	default:
		return "Not running: the uploaded binary is not the active process."
	}
}

func redactPluginConfigSecrets(raw []byte) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	redactPluginSecretNode(&doc)
	return yaml.Marshal(&doc)
}

func redactPluginSecretNode(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			redactPluginSecretNode(child)
		}
	case yaml.MappingNode:
		var keyValue, secretValue *yaml.Node
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			val := node.Content[i+1]
			if key.Kind == yaml.ScalarNode && pluginSecretEnvKey(key.Value) && val.Kind == yaml.ScalarNode {
				val.Value = ""
				val.Tag = "!!str"
			}
			if key.Kind == yaml.ScalarNode && key.Value == "key" && val.Kind == yaml.ScalarNode {
				keyValue = val
			}
			if key.Kind == yaml.ScalarNode && key.Value == "value" && val.Kind == yaml.ScalarNode {
				secretValue = val
			}
			redactPluginSecretNode(val)
		}
		if keyValue != nil && secretValue != nil && pluginSecretEnvKey(keyValue.Value) {
			secretValue.Value = ""
			secretValue.Tag = "!!str"
		}
	}
}

func pluginSecretEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	for _, part := range []string{"PASSWORD", "SECRET", "HASH", "TOKEN", "PRIVATE"} {
		if strings.Contains(upper, part) {
			return true
		}
	}
	return false
}
