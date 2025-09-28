package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"debug/elf"
	"debug/macho"
	"debug/pe"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// Plugin deployment commands

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugins (create, deploy, update, list, status)",
	Long:  `Plugin management commands for creating, deploying, updating, and monitoring HashiCorp plugins`,
}

var pluginCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new plugin scaffold",
	Long:  `Create a new plugin scaffold from templates`,
	Run: func(cmd *cobra.Command, args []string) {
		createPluginScaffold()
	},
}

var pluginDeployCmd = &cobra.Command{
	Use:   "deploy [plugin-directory]",
	Short: "Deploy a plugin to the Apito server",
	Long:  `Build and deploy a plugin to the configured Apito server`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Check for --dir flag first, then positional argument, then default to current dir
		pluginDir, _ := cmd.Flags().GetString("dir")
		if pluginDir == "" && len(args) > 0 {
			pluginDir = args[0]
		}
		if pluginDir == "" {
			pluginDir = "."
		}
		deployPlugin(pluginDir)
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [plugin-directory]",
	Short: "Update an existing plugin on the server",
	Long:  `Build and update an existing plugin on the configured Apito server`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Check for --dir flag first, then positional argument, then default to current dir
		pluginDir, _ := cmd.Flags().GetString("dir")
		if pluginDir == "" && len(args) > 0 {
			pluginDir = args[0]
		}
		if pluginDir == "" {
			pluginDir = "."
		}
		updatePlugin(pluginDir)
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all plugins on the server",
	Long:  `List all plugins and their status on the configured Apito server`,
	Run: func(cmd *cobra.Command, args []string) {
		listPlugins()
	},
}

var pluginStatusCmd = &cobra.Command{
	Use:   "status [plugin-id]",
	Short: "Get status of a specific plugin",
	Long:  `Get detailed status information for a specific plugin`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		getPluginStatus(args[0])
	},
}

var pluginRestartCmd = &cobra.Command{
	Use:   "restart [plugin-id]",
	Short: "Restart a specific plugin",
	Long:  `Restart a specific plugin on the server`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		restartPlugin(args[0])
	},
}

var pluginStopCmd = &cobra.Command{
	Use:   "stop [plugin-id]",
	Short: "Stop a specific plugin",
	Long:  `Stop a specific plugin on the server`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		stopPlugin(args[0])
	},
}

var pluginDeleteCmd = &cobra.Command{
	Use:   "delete [plugin-id]",
	Short: "Delete a specific plugin",
	Long:  `Delete a specific plugin from the server`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deletePlugin(args[0])
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
	// Add --dir flags to commands that accept plugin directories
	pluginDeployCmd.Flags().StringP("dir", "d", "", "Plugin directory (alternative to positional argument)")
	pluginUpdateCmd.Flags().StringP("dir", "d", "", "Plugin directory (alternative to positional argument)")

	// Add plugin commands to plugin group
	pluginCmd.AddCommand(pluginCreateCmd)
	pluginCmd.AddCommand(pluginDeployCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginStatusCmd)
	pluginCmd.AddCommand(pluginRestartCmd)
	pluginCmd.AddCommand(pluginStopCmd)
	pluginCmd.AddCommand(pluginDeleteCmd)
	// Build commands are added in plugin_build.go
	// pluginCmd is added to rootCmd in main.go
}

func createPluginScaffold() {
	print_step("🔌 Create Plugin Scaffold")

	// Language selection
	languages := []string{"Go (recommended)", "JavaScript", "Python"}
	langPrompt := promptui.Select{
		Label: "Select plugin language",
		Items: languages,
	}
	langIdx, _, err := langPrompt.Run()
	if err != nil {
		print_error("Language selection cancelled")
		return
	}

	var repoURL string
	switch langIdx {
	case 0:
		repoURL = "https://github.com/apito-io/apito-hello-world-go-plugin.git"
	case 1:
		repoURL = "https://github.com/apito-io/apito-hello-world-js-plugin.git"
	case 2:
		repoURL = "https://github.com/apito-io/apito-hello-world-python-plugin.git"
	}

	// Plugin name input
	namePrompt := promptui.Prompt{
		Label:    "Plugin name (will be prefixed with 'hc-')",
		Validate: validatePluginName,
	}
	pluginName, err := namePrompt.Run()
	if err != nil {
		print_error("Plugin name input cancelled")
		return
	}

	// Add hc- prefix if not present
	if !strings.HasPrefix(pluginName, "hc-") {
		pluginName = "hc-" + pluginName
	}

	print_status(fmt.Sprintf("Creating plugin scaffold: %s", pluginName))
	print_status("Cloning template from: " + repoURL)

	// Clone the template repository
	if err := runGitClone(repoURL, pluginName); err != nil {
		print_error("Failed to clone template: " + err.Error())
		return
	}

	// Remove .git directory to start fresh
	gitDir := filepath.Join(pluginName, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		print_warning("Failed to remove .git directory: " + err.Error())
	}

	print_success(fmt.Sprintf("Plugin scaffold created successfully: %s", pluginName))
	print_status("Next steps:")
	print_status("1. cd " + pluginName)
	print_status("2. Customize your plugin code")
	print_status("3. Test locally: make build")
	print_status("4. Deploy: apito plugin deploy")
}

func deployPlugin(pluginDir string) {
	if !checkServerConfig() {
		return
	}

	print_step("🚀 Deploying Plugin")

	// Load plugin configuration
	config, err := readPluginConfig(pluginDir)
	if err != nil {
		print_error("Failed to load plugin configuration: " + err.Error())
		return
	}

	pluginID := config.Plugin.ID
	print_status(fmt.Sprintf("Deploying plugin: %s (version: %s)", pluginID, config.Plugin.Version))

	// Note: Build plugin separately using 'apito plugin build' before deployment
	print_status("Tip: Run 'apito plugin build' first to ensure your plugin is built")

	// Create deployment package
	packagePath, err := createDeploymentPackage(pluginDir, config)
	if err != nil {
		print_error("Failed to create deployment package: " + err.Error())
		return
	}
	defer os.Remove(packagePath) // Clean up

	// Deploy to server (includes platform validation)
	response, err := deployToServer(packagePath, config, false, pluginDir)
	if err != nil {
		print_error("Failed to deploy plugin: " + err.Error())
		return
	}

	if response.Success {
		print_success(response.Message)
		print_status(fmt.Sprintf("Plugin %s is now %s", pluginID, response.Status))
	} else {
		print_error("Deployment failed: " + response.Message)
		if response.Error != "" {
			print_error("Error details: " + response.Error)
		}
	}
}

func updatePlugin(pluginDir string) {
	if !checkServerConfig() {
		return
	}

	print_step("🔄 Updating Plugin")

	// Load plugin configuration
	config, err := readPluginConfig(pluginDir)
	if err != nil {
		print_error("Failed to load plugin configuration: " + err.Error())
		return
	}

	pluginID := config.Plugin.ID
	print_status(fmt.Sprintf("Updating plugin: %s (version: %s)", pluginID, config.Plugin.Version))

	// Note: Build plugin separately using 'apito plugin build' before update
	print_status("Tip: Run 'apito plugin build' first to ensure your plugin is built")

	// Create deployment package
	packagePath, err := createDeploymentPackage(pluginDir, config)
	if err != nil {
		print_error("Failed to create deployment package: " + err.Error())
		return
	}
	defer os.Remove(packagePath)

	// Deploy to server (update mode, includes platform validation)
	response, err := deployToServer(packagePath, config, true, pluginDir)
	if err != nil {
		print_error("Failed to update plugin: " + err.Error())
		return
	}

	if response.Success {
		print_success(response.Message)
		print_status(fmt.Sprintf("Plugin %s is now %s", pluginID, response.Status))
	} else {
		print_error("Update failed: " + response.Message)
		if response.Error != "" {
			print_error("Error details: " + response.Error)
		}
	}
}

func listPlugins() {
	if !checkServerConfig() {
		return
	}

	print_step("📋 Listing Plugins")

	account, err := getAccountConfig("")
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.CloudSyncKey

	// Make API request
	req, err := http.NewRequest("GET", serverURL+"/system/plugin", nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)
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
		print_status("No plugins found")
		return
	}

	print_success(fmt.Sprintf("Found %d plugins:", len(response.Plugins)))
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

func getPluginStatus(pluginID string) {
	if !checkServerConfig() {
		return
	}

	print_step(fmt.Sprintf("🔍 Plugin Status: %s", pluginID))

	account, err := getAccountConfig("")
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.CloudSyncKey

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/system/plugin/%s", serverURL, pluginID), nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

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

func restartPlugin(pluginID string) {
	controlPlugin(pluginID, "restart")
}

func stopPlugin(pluginID string) {
	controlPlugin(pluginID, "stop")
}

func deletePlugin(pluginID string) {
	if !checkServerConfig() {
		return
	}

	// Confirmation prompt
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to delete plugin '%s'? This cannot be undone (y/N)", pluginID),
		IsConfirm: true,
		Default:   "n",
	}

	if _, err := confirmPrompt.Run(); err != nil {
		print_status("Deletion cancelled")
		return
	}

	print_step(fmt.Sprintf("🗑️  Deleting Plugin: %s", pluginID))

	account, err := getAccountConfig("")
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.CloudSyncKey

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/system/plugin/%s", serverURL, pluginID), nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

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
		print_error("Delete failed: " + response.Message)
		if response.Error != "" {
			print_error("Error details: " + response.Error)
		}
	}
}

// Helper functions

func controlPlugin(pluginID, action string) {
	if !checkServerConfig() {
		return
	}

	print_step(fmt.Sprintf("🎛️  %s Plugin: %s", strings.Title(action), pluginID))

	account, err := getAccountConfig("")
	if err != nil {
		print_error("Failed to get account configuration: " + err.Error())
		return
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.CloudSyncKey

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/system/plugin/%s/%s", serverURL, pluginID, action), nil)
	if err != nil {
		print_error("Failed to create request: " + err.Error())
		return
	}

	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

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

func checkServerConfig() bool {
	// Get account configuration
	_, err := getAccountConfig("")
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
	// Implementation would use git clone command
	// For now, return a placeholder
	return fmt.Errorf("git clone not implemented - please clone manually: git clone %s %s", repoURL, targetDir)
}

// buildPlugin is now implemented in plugin_build.go

func createDeploymentPackage(pluginDir string, config *PluginConfig) (string, error) {
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

	// Add config.yml to tar
	if err := addFileToTar(tw, configPath, "config.yml"); err != nil {
		return "", fmt.Errorf("failed to add config.yml to deployment package: %w", err)
	}

	// Add binary to tar
	binaryName := filepath.Base(config.Plugin.BinaryPath)
	if err := addFileToTar(tw, binaryPath, binaryName); err != nil {
		return "", fmt.Errorf("failed to add binary %s to deployment package: %w", binaryName, err)
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
func getServerPlatformInfo(serverURL, cloudSyncKey string) (*ServerPlatformInfo, error) {
	url := fmt.Sprintf("%s/system/plugin/platform", serverURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	var response ServerPlatformResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if !response.Success {
		return nil, fmt.Errorf("platform API error: %s", response.Message)
	}

	return &response.Platform, nil
}

// validatePlatformCompatibility checks if the plugin binary is compatible with the server
func validatePlatformCompatibility(pluginDir string, config *PluginConfig, serverURL, cloudSyncKey string) error {
	print_status("🔍 Checking platform compatibility...")

	// Get server platform information
	serverPlatform, err := getServerPlatformInfo(serverURL, cloudSyncKey)
	if err != nil {
		print_warning("⚠️  Could not get server platform info: " + err.Error())
		print_warning("⚠️  Skipping platform validation - deploy at your own risk!")
		return nil
	}

	print_status(fmt.Sprintf("🖥️  Server Platform: %s/%s (%s)",
		serverPlatform.OS, serverPlatform.Architecture, serverPlatform.Hostname))

	// Detect plugin binary format
	binaryPath := filepath.Join(pluginDir, config.Plugin.BinaryPath)
	binaryInfo := detectBinaryFormat(binaryPath)

	if binaryInfo.Error != "" {
		return fmt.Errorf("❌ Failed to analyze plugin binary: %s", binaryInfo.Error)
	}

	print_status(fmt.Sprintf("🔧 Plugin Binary: %s/%s (%s format)",
		binaryInfo.OS, binaryInfo.Architecture, binaryInfo.Format))

	// Check compatibility
	if binaryInfo.OS != serverPlatform.OS {
		return fmt.Errorf("❌ PLATFORM MISMATCH: Plugin OS (%s) doesn't match server OS (%s)\n"+
			"💡 Build the plugin for %s or deploy to a %s server\n"+
			"💡 Use 'apito plugin build' and select the correct target platform",
			binaryInfo.OS, serverPlatform.OS, serverPlatform.OS, binaryInfo.OS)
	}

	if binaryInfo.Architecture != serverPlatform.Architecture {
		return fmt.Errorf("❌ ARCHITECTURE MISMATCH: Plugin architecture (%s) doesn't match server architecture (%s)\n"+
			"💡 Build the plugin for %s/%s or deploy to a %s server\n"+
			"💡 Use 'apito plugin build' and select the correct target platform",
			binaryInfo.Architecture, serverPlatform.Architecture,
			serverPlatform.OS, serverPlatform.Architecture, binaryInfo.Architecture)
	}

	print_success(fmt.Sprintf("✅ Platform compatibility verified: %s/%s",
		serverPlatform.OS, serverPlatform.Architecture))

	return nil
}

func deployToServer(packagePath string, config *PluginConfig, isUpdate bool, pluginDir string) (*PluginOperationResponse, error) {
	account, err := getAccountConfig("")
	if err != nil {
		return nil, fmt.Errorf("failed to get account configuration: %w", err)
	}

	serverURL := account.ServerURL
	cloudSyncKey := account.CloudSyncKey

	// Validate platform compatibility before deployment
	if err := validatePlatformCompatibility(pluginDir, config, serverURL, cloudSyncKey); err != nil {
		return nil, err
	}

	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add plugin metadata
	writer.WriteField("id", config.Plugin.ID)
	writer.WriteField("language", config.Plugin.Language)
	writer.WriteField("title", config.Plugin.Title)
	writer.WriteField("description", config.Plugin.Description)
	writer.WriteField("type", config.Plugin.Type)
	writer.WriteField("version", config.Plugin.Version)
	writer.WriteField("author", config.Plugin.Author)
	writer.WriteField("repository_url", config.Plugin.RepositoryURL)
	writer.WriteField("branch", config.Plugin.Branch)
	writer.WriteField("binary_path", config.Plugin.BinaryPath)
	writer.WriteField("exported_variable", config.Plugin.ExportedVariable)
	writer.WriteField("enable", fmt.Sprintf("%v", config.Plugin.Enable))
	writer.WriteField("debug", fmt.Sprintf("%v", config.Plugin.Debug))

	// Add plugin package file
	file, err := os.Open(packagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	part, err := writer.CreateFormFile("plugin_files", filepath.Base(packagePath))
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	writer.Close()

	// Create HTTP request
	var method, url string
	if isUpdate {
		method = "PUT"
		url = fmt.Sprintf("%s/system/plugin/%s", serverURL, config.Plugin.ID)
	} else {
		method = "POST"
		url = fmt.Sprintf("%s/system/plugin", serverURL)
	}

	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

	// Send request
	client := &http.Client{Timeout: 5 * time.Minute} // Longer timeout for uploads
	resp, err := client.Do(req)
	if err != nil {
		// Check for connection refused or network errors
		if strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "connect: connection refused") ||
			strings.Contains(err.Error(), "dial tcp") {
			return nil, fmt.Errorf("❌ Apito Engine server is offline or unreachable at %s\n"+
				"💡 Please ensure the Apito Engine server is running:\n"+
				"   • Check if server is started: 'apito status'\n"+
				"   • Start server if needed: 'apito start'\n"+
				"   • Verify server URL in config: %s", serverURL, serverURL)
		}
		return nil, fmt.Errorf("failed to deploy plugin: %w", err)
	}
	defer resp.Body.Close()

	var response PluginOperationResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}
