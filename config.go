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

// AccountConfig represents configuration for a single account
type AccountConfig struct {
	ServerURL    string `yaml:"server_url"`     // Apito server URL for plugin management
	CloudSyncKey string `yaml:"cloud_sync_key"` // Cloud sync key for authentication
}

type CLIConfig struct {
	Mode           string                   `yaml:"mode"`                     // "docker" or "manual"
	DefaultAccount string                   `yaml:"default_account"`          // Default account name
	DefaultPlugin  string                   `yaml:"default_plugin,omitempty"` // Default plugin for operations
	Timeout        int                      `yaml:"timeout,omitempty"`        // Request timeout in seconds
	Accounts       map[string]AccountConfig `yaml:"accounts"`                 // Account configurations

	// Legacy fields for backward compatibility
	ServerURL    string `yaml:"server_url,omitempty"`     // Legacy: Apito server URL
	CloudSyncKey string `yaml:"cloud_sync_key,omitempty"` // Legacy: Cloud sync key
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
		Timeout:  30, // Default timeout
		Accounts: make(map[string]AccountConfig),
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

	// Initialize accounts map if nil
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]AccountConfig)
	}

	// Migration: Convert legacy flat config to account-based
	if cfg.ServerURL != "" || cfg.CloudSyncKey != "" {
		if len(cfg.Accounts) == 0 {
			// Create default account from legacy config
			cfg.Accounts["default"] = AccountConfig{
				ServerURL:    cfg.ServerURL,
				CloudSyncKey: cfg.CloudSyncKey,
			}
			if cfg.DefaultAccount == "" {
				cfg.DefaultAccount = "default"
			}
			// Clear legacy fields
			cfg.ServerURL = ""
			cfg.CloudSyncKey = ""
			// Save migrated config
			_ = saveCLIConfig(cfg)
		}
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
	items := []string{"Docker (recommended, stable)", "Manual (experimental, local setup)"}
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
	Long:  `Set a configuration value (timeout, mode, default_account) or account-specific values`,
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) >= 4 && strings.ToLower(args[0]) == "account" {
			// Handle: apito config set account <account-name> <url|key> <value>
			setAccountConfigValue(args[1], args[2], args[3])
		} else if len(args) == 3 {
			// Handle: apito config set <account-name> <url|key> <value>
			setAccountConfigValue(args[0], args[1], args[2])
		} else if len(args) == 2 {
			// Handle legacy format: apito config set <key> <value>
			setConfigValue(args[0], args[1])
		} else {
			print_error("Invalid arguments. Use:")
			print_status("  apito config set <key> <value>")
			print_status("  apito config set <account-name> <url|key> <value>")
			print_status("  apito config set account <account-name> <url|key> <value>")
		}
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

// Account management commands
var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage accounts",
	Long:  `Manage multiple Apito accounts for different environments`,
}

var accountCreateCmd = &cobra.Command{
	Use:   "create <account-name>",
	Short: "Create a new account",
	Long:  `Create a new account with server URL and sync key`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		createAccount(args[0])
	},
}

var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all accounts",
	Long:  `List all configured accounts`,
	Run: func(cmd *cobra.Command, args []string) {
		listAccounts()
	},
}

var accountSelectCmd = &cobra.Command{
	Use:   "select <account-name>",
	Short: "Set default account",
	Long:  `Set the default account for plugin operations`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		selectAccount(args[0])
	},
}

var accountDeleteCmd = &cobra.Command{
	Use:   "delete <account-name>",
	Short: "Delete an account",
	Long:  `Delete an account configuration`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		deleteAccount(args[0])
	},
}

var accountTestCmd = &cobra.Command{
	Use:   "test <account-name>",
	Short: "Test account connection",
	Long:  `Test the connection and credentials for a specific account`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		testAccountConnection(args[0])
	},
}

func init() {
	// Add config commands
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configResetCmd)

	// Add account commands
	accountCmd.AddCommand(accountCreateCmd)
	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountSelectCmd)
	accountCmd.AddCommand(accountDeleteCmd)
	accountCmd.AddCommand(accountTestCmd)

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(accountCmd)
}

func setConfigValue(key, value string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Handle account-based configuration
	if strings.ToLower(key) == "account" {
		print_error("Account configuration requires format: apito config set account <account-name> <url|key> <value>")
		print_status("Examples:")
		print_status("  apito config set account production url https://api.apito.io")
		print_status("  apito config set account production key your-sync-key")
		return
	}

	switch strings.ToLower(key) {
	case "server_url", "server":
		// Legacy support - create default account if none exists
		if len(cfg.Accounts) == 0 {
			cfg.Accounts["default"] = AccountConfig{}
			cfg.DefaultAccount = "default"
		}
		// Validate URL format
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			print_error("Server URL must start with http:// or https://")
			return
		}
		// Set for default account
		if account, exists := cfg.Accounts[cfg.DefaultAccount]; exists {
			account.ServerURL = strings.TrimSuffix(value, "/")
			cfg.Accounts[cfg.DefaultAccount] = account
		}

	case "cloud_sync_key", "sync_key", "key":
		// Legacy support - create default account if none exists
		if len(cfg.Accounts) == 0 {
			cfg.Accounts["default"] = AccountConfig{}
			cfg.DefaultAccount = "default"
		}
		if len(value) < 10 {
			print_warning("Cloud sync key seems short, make sure it's correct")
		}
		// Set for default account
		if account, exists := cfg.Accounts[cfg.DefaultAccount]; exists {
			account.CloudSyncKey = value
			cfg.Accounts[cfg.DefaultAccount] = account
		}

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

	case "default_account":
		// Validate account exists
		if _, exists := cfg.Accounts[value]; !exists {
			print_error(fmt.Sprintf("Account '%s' does not exist. Available accounts: %s",
				value, strings.Join(getAccountNames(cfg), ", ")))
			return
		}
		cfg.DefaultAccount = value

	default:
		print_error("Unknown configuration key: " + key)
		print_status("Available keys: timeout, default_plugin, mode, default_account")
		print_status("For account-specific config: apito config set account <account-name> <url|key> <value>")
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
		// Show default account's server URL
		if cfg.DefaultAccount != "" && cfg.Accounts[cfg.DefaultAccount].ServerURL != "" {
			print_status(fmt.Sprintf("server_url: %s (from account '%s')",
				cfg.Accounts[cfg.DefaultAccount].ServerURL, cfg.DefaultAccount))
		} else {
			print_status("server_url: (not set)")
		}

	case "cloud_sync_key", "sync_key", "key":
		// Show default account's sync key
		if cfg.DefaultAccount != "" && cfg.Accounts[cfg.DefaultAccount].CloudSyncKey != "" {
			print_status(fmt.Sprintf("cloud_sync_key: %s (from account '%s')",
				maskSensitiveValue(key, cfg.Accounts[cfg.DefaultAccount].CloudSyncKey), cfg.DefaultAccount))
		} else {
			print_status("cloud_sync_key: (not set)")
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

	case "default_account":
		if cfg.DefaultAccount == "" {
			print_status("default_account: (not set)")
		} else {
			print_status(fmt.Sprintf("default_account: %s", cfg.DefaultAccount))
		}

	case "account":
		// Show all accounts
		if len(cfg.Accounts) == 0 {
			print_status("No accounts configured")
		} else {
			for name, account := range cfg.Accounts {
				defaultMarker := ""
				if name == cfg.DefaultAccount {
					defaultMarker = " (default)"
				}
				print_status(fmt.Sprintf("Account '%s'%s:", name, defaultMarker))
				print_status(fmt.Sprintf("  URL: %s", getValueOrNotSet(account.ServerURL, "")))
				print_status(fmt.Sprintf("  Key: %s", maskSensitiveValue("key", account.CloudSyncKey)))
			}
		}

	default:
		print_error("Unknown configuration key: " + key)
		print_status("Available keys: timeout, default_plugin, mode, default_account, account")
		print_status("For account-specific values, use: apito config get account")
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
	print_status(fmt.Sprintf("Default Account: %s", getValueOrNotSet(cfg.DefaultAccount, "")))
	print_status(fmt.Sprintf("Timeout: %d seconds", cfg.Timeout))
	print_status(fmt.Sprintf("Default Plugin: %s", getValueOrNotSet(cfg.DefaultPlugin, "")))

	configPath, _ := configFilePath()
	print_status(fmt.Sprintf("Config file: %s", configPath))

	// Show accounts
	if len(cfg.Accounts) == 0 {
		print_warning("No accounts configured. Create one with: apito account create <name>")
	} else {
		print_status("")
		print_step("🔑 Accounts")
		for name, account := range cfg.Accounts {
			defaultMarker := ""
			if name == cfg.DefaultAccount {
				defaultMarker = " (default)"
			}
			print_status(fmt.Sprintf("  %s%s", name, defaultMarker))
			print_status(fmt.Sprintf("    URL: %s", getValueOrNotSet(account.ServerURL, "")))
			print_status(fmt.Sprintf("    Key: %s", maskSensitiveValue("key", account.CloudSyncKey)))
		}
	}

	// Check if plugin configuration is complete
	hasValidAccount := false
	for _, account := range cfg.Accounts {
		if account.ServerURL != "" && account.CloudSyncKey != "" {
			hasValidAccount = true
			break
		}
	}

	if !hasValidAccount {
		print_warning("No complete account configuration found. Run 'apito account create <name>' to set up")
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

	// Check if accounts already exist
	if len(cfg.Accounts) > 0 {
		print_status("Accounts already configured:")
		for name, account := range cfg.Accounts {
			defaultMarker := ""
			if name == cfg.DefaultAccount {
				defaultMarker = " (default)"
			}
			print_status(fmt.Sprintf("  %s%s - %s", name, defaultMarker, account.ServerURL))
		}

		// Ask if user wants to create a new account
		createNewPrompt := promptui.Select{
			Label: "Create a new account?",
			Items: []string{"Yes", "No"},
		}
		_, createNew, err := createNewPrompt.Run()
		if err != nil || createNew == "No" {
			print_status("Configuration initialization cancelled")
			return
		}
	}

	// Account name prompt
	namePrompt := promptui.Prompt{
		Label: "Account name",
		Validate: func(input string) error {
			if len(input) < 2 {
				return fmt.Errorf("account name must be at least 2 characters")
			}
			if _, exists := cfg.Accounts[input]; exists {
				return fmt.Errorf("account '%s' already exists", input)
			}
			return nil
		},
	}

	accountName, err := namePrompt.Run()
	if err != nil {
		print_error("Configuration cancelled")
		return
	}

	// Server URL prompt
	serverPrompt := promptui.Prompt{
		Label:    "Apito Server URL",
		Validate: validateServerURL,
	}

	serverURL, err := serverPrompt.Run()
	if err != nil {
		print_error("Configuration cancelled")
		return
	}

	// Cloud sync key prompt
	keyPrompt := promptui.Prompt{
		Label:    "Cloud Sync Key",
		Validate: validateCloudSyncKey,
		Mask:     '*',
	}

	cloudSyncKey, err := keyPrompt.Run()
	if err != nil {
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

	// Create account
	cfg.Accounts[accountName] = AccountConfig{
		ServerURL:    strings.TrimSuffix(serverURL, "/"),
		CloudSyncKey: cloudSyncKey,
	}

	// Set as default if it's the first account
	if len(cfg.Accounts) == 1 {
		cfg.DefaultAccount = accountName
	}

	// Save configuration
	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Account '%s' created successfully!", accountName))
	if cfg.DefaultAccount == accountName {
		print_status("Set as default account")
	}

	// Test connection
	print_status("")
	print_status("Testing account connection...")
	testAccountConnection(accountName)
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

// Account management functions

func setAccountConfigValue(accountName, key, value string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Ensure account exists
	if _, exists := cfg.Accounts[accountName]; !exists {
		cfg.Accounts[accountName] = AccountConfig{}
	}

	account := cfg.Accounts[accountName]

	switch strings.ToLower(key) {
	case "url", "server_url":
		// Validate URL format
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			print_error("Server URL must start with http:// or https://")
			return
		}
		account.ServerURL = strings.TrimSuffix(value, "/")

	case "key", "cloud_sync_key", "sync_key":
		if len(value) < 10 {
			print_warning("Cloud sync key seems short, make sure it's correct")
		}
		account.CloudSyncKey = value

	default:
		print_error("Unknown account configuration key: " + key)
		print_status("Available keys: url, key")
		return
	}

	cfg.Accounts[accountName] = account

	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Account '%s' %s set to %s", accountName, key, maskSensitiveValue(key, value)))
}

func createAccount(accountName string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Check if account already exists
	if _, exists := cfg.Accounts[accountName]; exists {
		print_error(fmt.Sprintf("Account '%s' already exists", accountName))
		return
	}

	// Interactive setup
	print_step(fmt.Sprintf("🔧 Creating Account: %s", accountName))

	// Server URL prompt
	serverPrompt := promptui.Prompt{
		Label:    "Server URL",
		Validate: validateServerURL,
	}

	serverURL, err := serverPrompt.Run()
	if err != nil {
		print_error("Account creation cancelled")
		return
	}

	// Cloud sync key prompt
	keyPrompt := promptui.Prompt{
		Label:    "Cloud Sync Key",
		Validate: validateCloudSyncKey,
		Mask:     '*',
	}

	cloudSyncKey, err := keyPrompt.Run()
	if err != nil {
		print_error("Account creation cancelled")
		return
	}

	// Create account
	cfg.Accounts[accountName] = AccountConfig{
		ServerURL:    strings.TrimSuffix(serverURL, "/"),
		CloudSyncKey: cloudSyncKey,
	}

	// Set as default if it's the first account
	if len(cfg.Accounts) == 1 {
		cfg.DefaultAccount = accountName
	}

	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Account '%s' created successfully!", accountName))
	if cfg.DefaultAccount == accountName {
		print_status("Set as default account")
	}

	// Suggest testing the connection
	print_status("")
	print_status("💡 Test your account connection with:")
	print_status(fmt.Sprintf("   apito account test %s", accountName))
}

func listAccounts() {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	print_step("📋 Account List")

	if len(cfg.Accounts) == 0 {
		print_status("No accounts configured")
		print_status("Create an account with: apito account create <name>")
		return
	}

	for name, account := range cfg.Accounts {
		defaultMarker := ""
		if name == cfg.DefaultAccount {
			defaultMarker = " (default)"
		}

		print_status(fmt.Sprintf("🔑 %s%s", name, defaultMarker))
		print_status(fmt.Sprintf("   URL: %s", getValueOrNotSet(account.ServerURL, "")))
		print_status(fmt.Sprintf("   Key: %s", maskSensitiveValue("key", account.CloudSyncKey)))
		print_status("")
	}
}

func selectAccount(accountName string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Validate account exists
	if _, exists := cfg.Accounts[accountName]; !exists {
		print_error(fmt.Sprintf("Account '%s' does not exist", accountName))
		print_status("Available accounts: " + strings.Join(getAccountNames(cfg), ", "))
		return
	}

	cfg.DefaultAccount = accountName

	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Default account set to: %s", accountName))
}

func deleteAccount(accountName string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Check if account exists
	if _, exists := cfg.Accounts[accountName]; !exists {
		print_error(fmt.Sprintf("Account '%s' does not exist", accountName))
		return
	}

	// Confirmation prompt
	confirmPrompt := promptui.Prompt{
		Label:     fmt.Sprintf("Are you sure you want to delete account '%s'? (y/N)", accountName),
		IsConfirm: true,
		Default:   "n",
	}

	if _, err := confirmPrompt.Run(); err != nil {
		print_status("Deletion cancelled")
		return
	}

	// Delete account
	delete(cfg.Accounts, accountName)

	// Update default account if it was deleted
	if cfg.DefaultAccount == accountName {
		if len(cfg.Accounts) > 0 {
			// Set first available account as default
			for name := range cfg.Accounts {
				cfg.DefaultAccount = name
				break
			}
		} else {
			cfg.DefaultAccount = ""
		}
	}

	if err := saveCLIConfig(cfg); err != nil {
		print_error("Failed to save configuration: " + err.Error())
		return
	}

	print_success(fmt.Sprintf("Account '%s' deleted successfully", accountName))
	if cfg.DefaultAccount != "" {
		print_status(fmt.Sprintf("Default account is now: %s", cfg.DefaultAccount))
	}
}

func getAccountNames(cfg *CLIConfig) []string {
	var names []string
	for name := range cfg.Accounts {
		names = append(names, name)
	}
	return names
}

// Helper function to get account configuration for plugin operations
func getAccountConfig(accountName string) (AccountConfig, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return AccountConfig{}, err
	}

	// Use provided account name or default
	if accountName == "" {
		accountName = cfg.DefaultAccount
	}

	// If still no account, try to select one interactively
	if accountName == "" {
		if len(cfg.Accounts) == 0 {
			return AccountConfig{}, fmt.Errorf("no accounts configured. Create one with: apito account create <name>")
		}

		// Interactive account selection
		accountNames := getAccountNames(cfg)
		selector := promptui.Select{
			Label: "Select account",
			Items: accountNames,
		}
		_, selectedAccount, err := selector.Run()
		if err != nil {
			return AccountConfig{}, fmt.Errorf("account selection cancelled")
		}
		accountName = selectedAccount
	}

	account, exists := cfg.Accounts[accountName]
	if !exists {
		return AccountConfig{}, fmt.Errorf("account '%s' does not exist", accountName)
	}

	if account.ServerURL == "" {
		return AccountConfig{}, fmt.Errorf("account '%s' has no server URL configured", accountName)
	}

	if account.CloudSyncKey == "" {
		return AccountConfig{}, fmt.Errorf("account '%s' has no cloud sync key configured", accountName)
	}

	return account, nil
}

func testAccountConnection(accountName string) {
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	// Check if account exists
	account, exists := cfg.Accounts[accountName]
	if !exists {
		print_error(fmt.Sprintf("Account '%s' does not exist", accountName))
		print_status("Available accounts: " + strings.Join(getAccountNames(cfg), ", "))
		return
	}

	print_step(fmt.Sprintf("🔍 Testing Account Connection: %s", accountName))

	// Validate account configuration
	if account.ServerURL == "" {
		print_error("Account has no server URL configured")
		print_status("Set server URL with: apito config set account " + accountName + " url <url>")
		return
	}

	if account.CloudSyncKey == "" {
		print_error("Account has no cloud sync key configured")
		print_status("Set cloud sync key with: apito config set account " + accountName + " key <key>")
		return
	}

	print_status(fmt.Sprintf("Server URL: %s", account.ServerURL))
	print_status(fmt.Sprintf("Cloud Sync Key: %s", maskSensitiveValue("key", account.CloudSyncKey)))
	print_status("Testing connection...")

	// Test connection
	client := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
	}

	// Test authenticated endpoint with sync key
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/system/health", account.ServerURL), nil)
	if err != nil {
		print_error("Failed to create test request: " + err.Error())
		return
	}

	req.Header.Set("X-Apito-Sync-Key", account.CloudSyncKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		print_error("Connection failed: " + err.Error())
		print_status("Possible issues:")
		print_status("  • Server is offline or unreachable")
		print_status("  • Network connectivity problems")
		print_status("  • Incorrect server URL")
		print_status("  • Firewall blocking the connection")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		print_success("✅ Connection test successful")
		print_status("Server is reachable and sync key is valid")
	} else if resp.StatusCode == 401 {
		print_error("❌ Authentication failed")
		print_status("Sync key is invalid or expired")
		print_status("Update the key with: apito config set account " + accountName + " key <new-key>")
	} else {
		print_warning(fmt.Sprintf("⚠️  Server returned status %d", resp.StatusCode))
		print_status("Server is reachable but may have issues")
	}
}
