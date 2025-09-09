package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type CLIConfig struct {
	Mode          string `yaml:"mode"`                     // "docker" or "manual"
	ServerURL     string `yaml:"server_url"`               // Apito server URL for plugin management
	CloudSyncKey  string `yaml:"cloud_sync_key"`           // Cloud sync key for authentication
	DefaultPlugin string `yaml:"default_plugin,omitempty"` // Default plugin for operations
	Timeout       int    `yaml:"timeout,omitempty"`        // Request timeout in seconds
}

func configFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".apito"), 0755); err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".apito", "config.yml"), nil
}

func loadCLIConfig() (*CLIConfig, error) {
	path, err := configFilePath()
	if err != nil {
		return nil, err
	}
	cfg := &CLIConfig{
		Timeout: 30, // Default timeout
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	// Set default timeout if not specified
	if cfg.Timeout == 0 {
		cfg.Timeout = 30
	}
	return cfg, nil
}

func saveCLIConfig(cfg *CLIConfig) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// determineRunMode loads mode from config. If empty, prompt the user to
// choose between Docker (recommended) and Manual. Optionally persist choice.
// determineRunMode returns the configured mode or defaults to "docker".
// It does not prompt the user.
func determineRunMode() (string, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	if cfg.Mode == "docker" || cfg.Mode == "manual" {
		return cfg.Mode, nil
	}
	return "docker", nil
}

// selectAndPersistRunMode prompts the user to choose a mode and optionally
// persists it in ~/.apito/config.yml. If Docker is selected, ensures a
// docker-compose.yml is created in ~/.apito.
func selectAndPersistRunMode() (string, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	items := []string{"Docker (recommended, stable)", "Manual (binary & local setup)"}
	selector := promptui.Select{
		Label: "Select run mode",
		Items: items,
		Size:  4,
	}
	idx, _, err := selector.Run()
	if err != nil {
		// default to docker if prompt fails
		idx = 0
	}
	mode := "docker"
	if idx == 1 {
		mode = "manual"
	}

	confirm := promptui.Select{
		Label: fmt.Sprintf("Remember '%s' as default?", mode),
		Items: []string{"Yes", "No"},
	}
	_, remember, err := confirm.Run()
	if err == nil && remember == "Yes" {
		cfg.Mode = mode
		_ = saveCLIConfig(cfg)
		print_success("Saved preference to ~/.apito/config.yml")
	}

	if mode == "docker" {
		if _, err := writeComposeFile(); err == nil {
			print_status("docker-compose.yml prepared in ~/.apito")
		}
	}
	return mode, nil
}

// ===============================================
// Plugin Configuration Management Commands
// ===============================================

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
	Long:  `Configure server URL, cloud sync key, and other CLI settings`,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long:  `Set a configuration value (server_url, cloud_sync_key, timeout, mode)`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		setConfigValue(args[0], args[1])
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get configuration value(s)",
	Long:  `Get a specific configuration value or show all configuration`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			showAllConfig()
		} else {
			getConfigValue(args[0])
		}
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration interactively",
	Long:  `Initialize CLI configuration with interactive prompts`,
	Run: func(cmd *cobra.Command, args []string) {
		initializePluginConfig()
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	Long:  `Reset all configuration to default values`,
	Run: func(cmd *cobra.Command, args []string) {
		resetConfig()
	},
}

func init() {
	// Add config commands
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configResetCmd)

	rootCmd.AddCommand(configCmd)
}

func setConfigValue(key, value string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	switch strings.ToLower(key) {
	case "server_url", "server":
		// Validate URL format
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			print_error("Server URL must start with http:// or https://")
			return
		}
		// Remove trailing slash
		cfg.ServerURL = strings.TrimSuffix(value, "/")

	case "cloud_sync_key", "sync_key", "key":
		if len(value) < 10 {
			print_warning("Cloud sync key seems short, make sure it's correct")
		}
		cfg.CloudSyncKey = value

	case "timeout":
		if timeout := parseIntValue(value); timeout > 0 {
			cfg.Timeout = timeout
		} else {
			print_error("Timeout must be a positive integer")
			return
		}

	case "default_plugin":
		cfg.DefaultPlugin = value

	case "mode":
		if value != "docker" && value != "manual" {
			print_error("Mode must be 'docker' or 'manual'")
			return
		}
		cfg.Mode = value

	default:
		print_error("Unknown configuration key: " + key)
		print_status("Available keys: server_url, cloud_sync_key, timeout, default_plugin, mode")
		return
	}

	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Configuration set: %s = %s", key, maskSensitiveValue(key, value)))
}

func getConfigValue(key string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	switch strings.ToLower(key) {
	case "server_url", "server":
		if cfg.ServerURL == "" {
			print_status("server_url: (not set)")
		} else {
			print_status(fmt.Sprintf("server_url: %s", cfg.ServerURL))
		}

	case "cloud_sync_key", "sync_key", "key":
		if cfg.CloudSyncKey == "" {
			print_status("cloud_sync_key: (not set)")
		} else {
			print_status(fmt.Sprintf("cloud_sync_key: %s", maskSensitiveValue(key, cfg.CloudSyncKey)))
		}

	case "timeout":
		print_status(fmt.Sprintf("timeout: %d seconds", cfg.Timeout))

	case "default_plugin":
		if cfg.DefaultPlugin == "" {
			print_status("default_plugin: (not set)")
		} else {
			print_status(fmt.Sprintf("default_plugin: %s", cfg.DefaultPlugin))
		}

	case "mode":
		if cfg.Mode == "" {
			print_status("mode: (not set, will default to docker)")
		} else {
			print_status(fmt.Sprintf("mode: %s", cfg.Mode))
		}

	default:
		print_error("Unknown configuration key: " + key)
		print_status("Available keys: server_url, cloud_sync_key, timeout, default_plugin, mode")
	}
}

func showAllConfig() {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	print_step("📋 CLI Configuration")

	print_status(fmt.Sprintf("Mode: %s", getValueOrNotSet(cfg.Mode, "docker")))
	print_status(fmt.Sprintf("Server URL: %s", getValueOrNotSet(cfg.ServerURL, "")))
	print_status(fmt.Sprintf("Cloud Sync Key: %s", maskSensitiveValue("key", cfg.CloudSyncKey)))
	print_status(fmt.Sprintf("Timeout: %d seconds", cfg.Timeout))
	print_status(fmt.Sprintf("Default Plugin: %s", getValueOrNotSet(cfg.DefaultPlugin, "")))

	configPath, _ := configFilePath()
	print_status(fmt.Sprintf("Config file: %s", configPath))

	// Check if plugin configuration is complete
	if cfg.ServerURL == "" || cfg.CloudSyncKey == "" {
		print_warning("Plugin configuration is incomplete. Run 'apito config init' to set up")
	} else {
		print_success("Plugin configuration is complete")
	}
}

func initializePluginConfig() {
	print_step("🔧 Initialize Plugin Configuration")

	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Server URL prompt
	serverPrompt := promptui.Prompt{
		Label:    "Apito Server URL",
		Default:  cfg.ServerURL,
		Validate: validateServerURL,
	}

	if serverURL, err := serverPrompt.Run(); err == nil {
		cfg.ServerURL = strings.TrimSuffix(serverURL, "/")
	} else {
		print_error("Configuration cancelled")
		return
	}

	// Cloud sync key prompt
	keyPrompt := promptui.Prompt{
		Label:    "Cloud Sync Key",
		Default:  cfg.CloudSyncKey,
		Validate: validateCloudSyncKey,
		Mask:     '*',
	}

	if cloudSyncKey, err := keyPrompt.Run(); err == nil {
		cfg.CloudSyncKey = cloudSyncKey
	} else {
		print_error("Configuration cancelled")
		return
	}

	// Timeout prompt (optional)
	timeoutPrompt := promptui.Prompt{
		Label:   "Request timeout (seconds)",
		Default: fmt.Sprintf("%d", cfg.Timeout),
		Validate: func(input string) error {
			if timeout := parseIntValue(input); timeout <= 0 {
				return fmt.Errorf("timeout must be a positive integer")
			}
			return nil
		},
	}

	if timeout, err := timeoutPrompt.Run(); err == nil {
		cfg.Timeout = parseIntValue(timeout)
	}

	// Save configuration
	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success("Plugin configuration initialized successfully!")

	// Test connection
	if testPluginConnection(*cfg) {
		print_success("✅ Connection test successful")
	} else {
		print_warning("⚠️  Connection test failed - please verify your settings")
	}
}

func resetConfig() {
	confirmPrompt := promptui.Prompt{
		Label:     "Are you sure you want to reset all configuration? (y/N)",
		IsConfirm: true,
		Default:   "n",
	}

	if _, err := confirmPrompt.Run(); err != nil {
		print_status("Reset cancelled")
		return
	}

	// Delete config file
	configPath, _ := configFilePath()
	if err := os.Remove(configPath); err != nil {
		if !os.IsNotExist(err) {
			print_error("Failed to delete config file: " + err.Error())
			return
		}
	}

	print_success("Configuration reset to defaults")
}

// Helper functions for plugin configuration

func validateServerURL(input string) error {
	if input == "" {
		return fmt.Errorf("server URL is required")
	}
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		return fmt.Errorf("server URL must start with http:// or https://")
	}
	return nil
}

func validateCloudSyncKey(input string) error {
	if input == "" {
		return fmt.Errorf("cloud sync key is required")
	}
	if len(input) < 10 {
		return fmt.Errorf("cloud sync key seems too short")
	}
	return nil
}

func parseIntValue(value string) int {
	if val, err := fmt.Sscanf(value, "%d", new(int)); err == nil && val == 1 {
		var result int
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return 0
}

func maskSensitiveValue(key, value string) string {
	if value == "" {
		return "(not set)"
	}

	if strings.Contains(strings.ToLower(key), "key") || strings.Contains(strings.ToLower(key), "secret") {
		if len(value) <= 8 {
			return strings.Repeat("*", len(value))
		}
		return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
	}

	return value
}

func getValueOrNotSet(value, defaultValue string) string {
	if value == "" {
		if defaultValue != "" {
			return fmt.Sprintf("(not set, defaults to %s)", defaultValue)
		}
		return "(not set)"
	}
	return value
}

func testPluginConnection(cfg CLIConfig) bool {
	if cfg.ServerURL == "" {
		return false
	}

	print_status("Testing connection to " + cfg.ServerURL)

	// Try to connect to the health endpoint
	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	req, err := http.NewRequest("GET", cfg.ServerURL+"/plugin/v2/health", nil)
	if err != nil {
		print_error("Failed to create test request: " + err.Error())
		return false
	}

	// Don't set auth header for health check
	resp, err := client.Do(req)
	if err != nil {
		print_error("Connection failed: " + err.Error())
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200
}
