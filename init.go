package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Apito CLI system configuration",
	Long:  `Initialize and validate Apito CLI system configuration, check ports, and set up required environment variables.`,
	Run: func(cmd *cobra.Command, args []string) {
		initializeSystem()
	},
}

func initializeSystem() {
	print_step("🚀 Initializing Apito CLI System")
	fmt.Println()

	// Prepare core directories
	if err := ensureBaseDirs(); err != nil {
		print_error("Failed to prepare directories: " + err.Error())
		return
	}

	// Choose run mode (Docker vs Manual) and persist if confirmed
	print_status("Step 0: Select run mode (Docker recommended)...")
	mode, err := selectAndPersistRunMode()
	if err != nil {
		print_error("Failed to set run mode: " + err.Error())
		return
	}
	print_success("Run mode: " + mode)
	fmt.Println()

	// Step 1: Check and create ~/.apito directory
	print_status("Step 1: Checking Apito directory...")
	if err := ensureApitoDirectory(); err != nil {
		print_error("Failed to create Apito directory: " + err.Error())
		return
	}
	print_success("Apito directory ready")
	fmt.Println()

	// Step 2: Check and create .config file
	print_status("Step 2: Checking system configuration...")
	if err := ensureSystemConfig(); err != nil {
		print_error("Failed to create system configuration: " + err.Error())
		return
	}
	print_success("System configuration ready")
	fmt.Println()

	// Step 2.5: Optional database setup (Docker mode only)
	if mode == "docker" {
		print_status("Step 2.5: Database setup (optional)...")
		print_status("Database setup will be handled by 'apito start --db system' or 'apito start --db project'")
		print_status("You can set up databases when starting services")
	} else {
		print_status("Database setup will be handled by 'apito start --db system' or 'apito start --db project'")
	}
	fmt.Println()

	// Step 3: Validate system database configuration
	print_status("Step 3: Validating system database configuration...")
	if err := validateSystemDatabase(); err != nil {
		print_error("System database validation failed: " + err.Error())
		return
	}
	print_success("System database configuration validated")
	fmt.Println()

	// Step 4: Validate environment configuration
	print_status("Step 4: Validating environment configuration...")
	if err := validateEnvironmentConfig(); err != nil {
		print_error("Environment configuration validation failed: " + err.Error())
		return
	}
	print_success("Environment configuration validated")
	fmt.Println()

	// Step 5: Check port availability
	print_status("Step 5: Checking port availability...")
	if err := checkPortAvailability(); err != nil {
		print_warning("Port availability check failed: " + err.Error())
	} else {
		print_success("Port availability check passed")
	}
	fmt.Println()

	print_success("🎉 Apito CLI system initialization completed successfully!")
	print_status("You can now start apito studio using : apito start")
}

func ensureApitoDirectory() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	apitoDir := filepath.Join(homeDir, ".apito")
	if _, err := os.Stat(apitoDir); os.IsNotExist(err) {
		print_status("Creating Apito directory: " + apitoDir)
		if err := os.MkdirAll(apitoDir, 0755); err != nil {
			return fmt.Errorf("error creating Apito directory: %w", err)
		}
	} else {
		print_status("Apito directory already exists: " + apitoDir)
	}

	return nil
}

func ensureSystemConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	apitoBinDir := filepath.Join(homeDir, ".apito", "bin")
	if err := os.MkdirAll(apitoBinDir, 0755); err != nil {
		return fmt.Errorf("error creating bin directory: %w", err)
	}
	configFile := filepath.Join(apitoBinDir, ".env")

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		print_status("Creating system configuration file...")

		// Create default configuration
		defaultConfig := map[string]string{
			"ENVIRONMENT":             "local",
			"COOKIE_DOMAIN":           "localhost",
			"CORS_ORIGIN":             "http://localhost:4000",
			"PLUGIN_PATH":             "plugins",
			"PUBLIC_KEY_PATH":         "keys/public.key",
			"PRIVATE_KEY_PATH":        "keys/private.key",
			"BRANKA_KEY":              "",
			"APITO_SYSTEM_DB_ENGINE":  "coreDB",
			"SYSTEM_DB_HOST":          "",
			"SYSTEM_DB_PORT":          "",
			"SYSTEM_DB_USER":          "",
			"SYSTEM_DB_PASSWORD":      "",
			"SYSTEM_DB_NAME":          "~/.apito/engine-data/apito_system.db",
			"SERVE_PORT":              "5050",
			"CACHE_DRIVER":            "memory",
			"CACHE_TTL":               "600",
			"KV_ENGINE":               "coreDB",
			"AUTH_SERVICE_PROVIDER":   "local",
			"TOKEN_TTL":               "60",
		}

		if err := saveConfig(apitoBinDir, defaultConfig); err != nil {
			return fmt.Errorf("error creating system config: %w", err)
		}
		print_success("System configuration file created")
	} else {
		print_status("System configuration file already exists")
	}

	return nil
}

func validateSystemDatabase() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	config, err := getConfig(filepath.Join(homeDir, ".apito", "bin"))
	if err != nil {
		return fmt.Errorf("error reading system config: %w", err)
	}

	dbEngine := config["APITO_SYSTEM_DB_ENGINE"]
	if dbEngine == "" {
		dbEngine = "embed"
	}

	print_status("System database engine: " + dbEngine)

	// If using external database, validate configuration
	if dbEngine != "embed" {
		requiredFields := []string{"SYSTEM_DB_HOST", "SYSTEM_DB_PORT", "SYSTEM_DB_USER", "SYSTEM_DB_PASSWORD", "SYSTEM_DB_NAME"}
		missingFields := []string{}

		for _, field := range requiredFields {
			if config[field] == "" {
				missingFields = append(missingFields, field)
			}
		}

		if len(missingFields) > 0 {
			print_warning("Missing system database configuration fields: " + strings.Join(missingFields, ", "))
			print_status("Please configure the following database settings:")

			if err := promptForDatabaseConfig(config, "SYSTEM"); err != nil {
				return fmt.Errorf("error configuring system database: %w", err)
			}
		}
	}

	return nil
}

func validateEnvironmentConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	config, err := getConfig(filepath.Join(homeDir, ".apito", "bin"))
	if err != nil {
		return fmt.Errorf("error reading system config: %w", err)
	}

	// Check mandatory environment variables
	mandatoryFields := []string{"ENVIRONMENT", "CORS_ORIGIN", "COOKIE_DOMAIN"}
	missingFields := []string{}

	for _, field := range mandatoryFields {
		if config[field] == "" {
			missingFields = append(missingFields, field)
		}
	}

	// Handle BRANKA_KEY separately - generate if missing
	if config["BRANKA_KEY"] == "" {
		print_status("Generating BRANKA_KEY...")
		config["BRANKA_KEY"] = generateBrankaKey()
		print_success("BRANKA_KEY generated successfully")

		// Save the generated key
		if err := saveConfig(filepath.Join(homeDir, ".apito"), config); err != nil {
			return fmt.Errorf("error saving generated BRANKA_KEY: %w", err)
		}
	}

	if len(missingFields) > 0 {
		print_warning("Missing mandatory environment configuration: " + strings.Join(missingFields, ", "))
		print_status("Please configure the following environment settings:")

		if err := promptForEnvironmentConfig(config); err != nil {
			return fmt.Errorf("error configuring environment: %w", err)
		}
	}

	return nil
}

func promptForDatabaseConfig(config map[string]string, prefix string) error {
	print_status("Configuring " + prefix + " database settings...")

	// Database host
	prompt := promptui.Prompt{
		Label:   prefix + " Database Host",
		Default: config[prefix+"_DB_HOST"],
	}
	dbHost, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config[prefix+"_DB_HOST"] = dbHost

	// Database port
	prompt = promptui.Prompt{
		Label:   prefix + " Database Port",
		Default: config[prefix+"_DB_PORT"],
	}
	dbPort, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config[prefix+"_DB_PORT"] = dbPort

	// Database user
	prompt = promptui.Prompt{
		Label:   prefix + " Database User",
		Default: config[prefix+"_DB_USER"],
	}
	dbUser, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config[prefix+"_DB_USER"] = dbUser

	// Database password
	prompt = promptui.Prompt{
		Label:   prefix + " Database Password",
		Mask:    '*',
		Default: config[prefix+"_DB_PASSWORD"],
	}
	dbPassword, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config[prefix+"_DB_PASSWORD"] = dbPassword

	// Database name
	prompt = promptui.Prompt{
		Label:   prefix + " Database Name",
		Default: config[prefix+"_DB_NAME"],
	}
	dbName, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config[prefix+"_DB_NAME"] = dbName

	// Save configuration
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	if err := saveConfig(filepath.Join(homeDir, ".apito"), config); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	print_success(prefix + " database configuration saved")
	return nil
}

func promptForEnvironmentConfig(config map[string]string) error {
	print_status("Configuring environment settings...")

	// Environment
	envOptions := []string{"local", "development", "staging", "production"}
	currentEnv := config["ENVIRONMENT"]
	if currentEnv == "" {
		currentEnv = "local"
	}

	prompt := promptui.Select{
		Label: "Environment",
		Items: envOptions,
	}
	_, env, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config["ENVIRONMENT"] = env

	// CORS Origin
	promptInput := promptui.Prompt{
		Label:   "CORS Origin (e.g., http://localhost:3000, https://yourdomain.com)",
		Default: config["CORS_ORIGIN"],
	}
	corsOrigin, err := promptInput.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config["CORS_ORIGIN"] = corsOrigin

	// Cookie Domain
	promptInput = promptui.Prompt{
		Label:   "Cookie Domain (e.g., localhost, yourdomain.com)",
		Default: config["COOKIE_DOMAIN"],
	}
	cookieDomain, err := promptInput.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	config["COOKIE_DOMAIN"] = cookieDomain

	// Note: BRANKA_KEY is auto-generated if not provided

	// Save configuration
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	if err := saveConfig(filepath.Join(homeDir, ".apito"), config); err != nil {
		return fmt.Errorf("error saving configuration: %w", err)
	}

	print_success("Environment configuration saved")
	return nil
}

func generateBrankaKey() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"
	const keyLength = 32

	result := make([]byte, keyLength)
	for i := range result {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// Fallback to a simple random selection if crypto/rand fails
			result[i] = charset[i%len(charset)]
		} else {
			result[i] = charset[randomIndex.Int64()]
		}
	}
	return string(result)
}

func checkPortAvailability() error {
	ports := []int{5050, 4000}

	for _, port := range ports {
		address := ":" + strconv.Itoa(port)
		listener, err := net.Listen("tcp", address)
		if err != nil {
			print_warning(fmt.Sprintf("Port %d is already in use", port))
			print_status(fmt.Sprintf("Please ensure port %d is available for Apito to run properly", port))
		} else {
			listener.Close()
			print_status(fmt.Sprintf("Port %d is available", port))
		}
	}

	return nil
}
