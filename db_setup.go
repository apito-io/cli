package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

// getDefaultConfig returns default configuration for a database engine
func getDefaultConfig(engine string) DatabaseConfig {
	config := DatabaseConfig{
		Engine:       engine,
		Host:         "localhost",
		Database:     "apito",
		Username:     "apito",
		Password:     generateSecurePassword(),
		ExtraOptions: make(map[string]string),
	}

	switch engine {
	case "postgres":
		config.Port = "5432"
	case "mysql", "mariadb":
		config.Port = "3306"
		if engine == "mariadb" {
			config.Port = "3307"
		}
	case "sqlserver":
		config.Port = "1433"
		config.Username = "sa"
		config.Password = "Apito@12345"
	case "mongodb":
		config.Port = "27017"
		config.Username = ""
		config.Password = ""
		config.Database = ""
	}

	return config
}

// generateSecurePassword generates a secure password for database
func generateSecurePassword() string {
	// Simple secure password generation - in production, use crypto/rand
	return "Apito" + fmt.Sprintf("%d", time.Now().Unix()%10000)
}

// promptForCredentials prompts user to choose between default and custom credentials
func promptForCredentials(engine string) (DatabaseConfig, error) {
	config := getDefaultConfig(engine)

	prompt := promptui.Select{
		Label: "Choose credential option",
		Items: []string{
			"Use default credentials (apito user with generated password)",
			"Enter custom credentials",
		},
	}

	_, choice, err := prompt.Run()
	if err != nil {
		return config, err
	}

	if choice == "Use default credentials (apito user with generated password)" {
		return config, nil
	}

	// Custom credentials
	return promptCustomCredentials(engine)
}

// promptCustomCredentials prompts for custom database credentials
func promptCustomCredentials(engine string) (DatabaseConfig, error) {
	config := getDefaultConfig(engine)

	// Host
	hostPrompt := promptui.Prompt{
		Label:   "Database Host",
		Default: config.Host,
	}
	host, err := hostPrompt.Run()
	if err != nil {
		return config, err
	}
	config.Host = host

	// Port
	portPrompt := promptui.Prompt{
		Label:   "Database Port",
		Default: config.Port,
	}
	port, err := portPrompt.Run()
	if err != nil {
		return config, err
	}
	config.Port = port

	// Database name
	if engine != "mongodb" {
		dbPrompt := promptui.Prompt{
			Label:   "Database Name",
			Default: config.Database,
		}
		database, err := dbPrompt.Run()
		if err != nil {
			return config, err
		}
		config.Database = database
	}

	// Username
	if engine != "mongodb" {
		userPrompt := promptui.Prompt{
			Label:   "Username",
			Default: config.Username,
		}
		username, err := userPrompt.Run()
		if err != nil {
			return config, err
		}
		config.Username = username
	}

	// Password
	passPrompt := promptui.Prompt{
		Label: "Password",
		Mask:  '*',
	}
	password, err := passPrompt.Run()
	if err != nil {
		return config, err
	}
	config.Password = password

	// Engine-specific options
	switch engine {
	case "mongodb":
		// MongoDB specific options
		authPrompt := promptui.Select{
			Label: "Authentication Database",
			Items: []string{"admin", "none"},
		}
		_, authChoice, err := authPrompt.Run()
		if err != nil {
			return config, err
		}
		if authChoice == "admin" {
			config.ExtraOptions["authSource"] = "admin"
		}

		// MongoDB connection string format
		if config.Username != "" && config.Password != "" {
			config.ExtraOptions["authMechanism"] = "SCRAM-SHA-1"
		}
	}

	return config, nil
}

// formatConnectionString formats connection information for display
func formatConnectionString(config DatabaseConfig) string {
	var connStr string

	switch config.Engine {
	case "postgres":
		connStr = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s",
			config.Username, config.Password, config.Host, config.Port, config.Database)
	case "mysql", "mariadb":
		connStr = fmt.Sprintf("mysql://%s:%s@%s:%s/%s",
			config.Username, config.Password, config.Host, config.Port, config.Database)
	case "sqlserver":
		connStr = fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			config.Username, config.Password, config.Host, config.Port, config.Database)
	case "mongodb":
		if config.Username != "" && config.Password != "" {
			connStr = fmt.Sprintf("mongodb://%s:%s@%s:%s/%s",
				config.Username, config.Password, config.Host, config.Port, config.Database)
		} else {
			connStr = fmt.Sprintf("mongodb://%s:%s", config.Host, config.Port)
		}
	}

	return connStr
}

// displayConnectionInfo displays formatted connection information
func displayConnectionInfo(config DatabaseConfig) {
	fmt.Println()
	print_success("Database connection information:")
	fmt.Printf("  Engine: %s\n", strings.Title(config.Engine))
	fmt.Printf("  Host: %s\n", config.Host)
	fmt.Printf("  Port: %s\n", config.Port)

	if config.Database != "" {
		fmt.Printf("  Database: %s\n", config.Database)
	}
	if config.Username != "" {
		fmt.Printf("  Username: %s\n", config.Username)
	}
	fmt.Printf("  Password: %s\n", config.Password)

	// Show connection string
	connStr := formatConnectionString(config)
	fmt.Printf("  Connection String: %s\n", connStr)

	// Show environment variables for easy copy-paste
	fmt.Println()
	print_status("Environment variables for your application:")
	switch config.Engine {
	case "postgres":
		fmt.Printf("  DATABASE_URL=%s\n", connStr)
		fmt.Printf("  POSTGRES_HOST=%s\n", config.Host)
		fmt.Printf("  POSTGRES_PORT=%s\n", config.Port)
		fmt.Printf("  POSTGRES_DB=%s\n", config.Database)
		fmt.Printf("  POSTGRES_USER=%s\n", config.Username)
		fmt.Printf("  POSTGRES_PASSWORD=%s\n", config.Password)
	case "mysql", "mariadb":
		fmt.Printf("  DATABASE_URL=%s\n", connStr)
		fmt.Printf("  MYSQL_HOST=%s\n", config.Host)
		fmt.Printf("  MYSQL_PORT=%s\n", config.Port)
		fmt.Printf("  MYSQL_DATABASE=%s\n", config.Database)
		fmt.Printf("  MYSQL_USER=%s\n", config.Username)
		fmt.Printf("  MYSQL_PASSWORD=%s\n", config.Password)
	case "sqlserver":
		fmt.Printf("  DATABASE_URL=%s\n", connStr)
		fmt.Printf("  SQLSERVER_HOST=%s\n", config.Host)
		fmt.Printf("  SQLSERVER_PORT=%s\n", config.Port)
		fmt.Printf("  SQLSERVER_DATABASE=%s\n", config.Database)
		fmt.Printf("  SQLSERVER_USER=%s\n", config.Username)
		fmt.Printf("  SQLSERVER_PASSWORD=%s\n", config.Password)
	case "mongodb":
		fmt.Printf("  MONGODB_URI=%s\n", connStr)
		fmt.Printf("  MONGODB_HOST=%s\n", config.Host)
		fmt.Printf("  MONGODB_PORT=%s\n", config.Port)
		if config.Database != "" {
			fmt.Printf("  MONGODB_DATABASE=%s\n", config.Database)
		}
		if config.Username != "" {
			fmt.Printf("  MONGODB_USER=%s\n", config.Username)
			fmt.Printf("  MONGODB_PASSWORD=%s\n", config.Password)
		}
	}
	fmt.Println()
}

// saveDatabaseConfig saves the database configuration to the .env file
// isSystemDB determines whether this is for system database (init) or project database (start --db)
func saveDatabaseConfig(config DatabaseConfig, isSystemDB bool) error {
	// Read existing config
	existingConfig, err := ReadEnv()
	if err != nil {
		// If .env doesn't exist, create empty config
		existingConfig = make(map[string]string)
	}

	// Check if there's existing database configuration
	var prefix string
	var engineKey string
	if isSystemDB {
		prefix = "SYSTEM_DB"
		engineKey = "APITO_SYSTEM_DB_ENGINE"
	} else {
		prefix = "PROJECT_DB"
		engineKey = "APITO_PROJECT_DB_ENGINE"
	}

	// Check if there's existing configuration for this database type
	existingEngine := existingConfig[engineKey]
	if existingEngine != "" {
		// Show current configuration
		fmt.Println()
		print_warning("Existing " + prefix + " configuration found:")
		fmt.Printf("  Current Engine: %s\n", existingEngine)
		fmt.Printf("  Current Host: %s\n", existingConfig[prefix+"_HOST"])
		fmt.Printf("  Current Port: %s\n", existingConfig[prefix+"_PORT"])
		if existingConfig[prefix+"_NAME"] != "" {
			fmt.Printf("  Current Database: %s\n", existingConfig[prefix+"_NAME"])
		}
		if existingConfig[prefix+"_USER"] != "" {
			fmt.Printf("  Current Username: %s\n", existingConfig[prefix+"_USER"])
		}
		fmt.Printf("  Current Password: %s\n", existingConfig[prefix+"_PASSWORD"])
		fmt.Println()

		// Show new configuration
		print_status("New " + prefix + " configuration:")
		fmt.Printf("  New Engine: %s\n", config.Engine)
		fmt.Printf("  New Host: %s\n", config.Host)
		fmt.Printf("  New Port: %s\n", config.Port)
		if config.Database != "" {
			fmt.Printf("  New Database: %s\n", config.Database)
		}
		if config.Username != "" {
			fmt.Printf("  New Username: %s\n", config.Username)
		}
		fmt.Printf("  New Password: %s\n", config.Password)
		fmt.Println()

		// Ask for confirmation
		prompt := promptui.Select{
			Label: "Do you want to overwrite the existing " + prefix + " configuration?",
			Items: []string{"Yes, overwrite existing configuration", "No, keep existing configuration"},
		}

		_, choice, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}

		if choice == "No, keep existing configuration" {
			print_status("Keeping existing " + prefix + " configuration")
			return nil
		}

		print_status("Overwriting existing " + prefix + " configuration...")
	}

	// Set the database engine
	if isSystemDB {
		existingConfig["APITO_SYSTEM_DB_ENGINE"] = config.Engine
	} else {
		existingConfig["APITO_PROJECT_DB_ENGINE"] = config.Engine
	}

	// Add database configuration with correct prefix
	existingConfig[prefix+"_HOST"] = config.Host
	existingConfig[prefix+"_PORT"] = config.Port
	if config.Database != "" {
		existingConfig[prefix+"_NAME"] = config.Database
	}
	if config.Username != "" {
		existingConfig[prefix+"_USER"] = config.Username
	}
	existingConfig[prefix+"_PASSWORD"] = config.Password

	// Add connection string
	connStr := formatConnectionString(config)
	if isSystemDB {
		existingConfig["SYSTEM_DATABASE_URL"] = connStr
	} else {
		existingConfig["PROJECT_DATABASE_URL"] = connStr
	}

	// Add engine-specific options
	if len(config.ExtraOptions) > 0 {
		for k, v := range config.ExtraOptions {
			existingConfig[prefix+"_"+strings.ToUpper(k)] = v
		}
	}

	// Save to .env file
	return WriteEnv(existingConfig)
}

// loadDatabaseConfig loads the database configuration from the .env file
// isSystemDB determines whether to load system database or project database config
func loadDatabaseConfig(isSystemDB bool) (*DatabaseConfig, error) {
	// Read existing config
	existingConfig, err := ReadEnv()
	if err != nil {
		return nil, errors.New("no database configuration found")
	}

	// Determine which database configuration to load
	var prefix string
	var engineKey string
	if isSystemDB {
		prefix = "SYSTEM_DB"
		engineKey = "APITO_SYSTEM_DB_ENGINE"
	} else {
		prefix = "PROJECT_DB"
		engineKey = "APITO_PROJECT_DB_ENGINE"
	}

	// Get the database engine
	engine, exists := existingConfig[engineKey]
	if !exists || engine == "" {
		return nil, errors.New("no database engine configured")
	}

	// Build config from .env
	config := &DatabaseConfig{
		Engine:       engine,
		Host:         existingConfig[prefix+"_HOST"],
		Port:         existingConfig[prefix+"_PORT"],
		Database:     existingConfig[prefix+"_NAME"],
		Username:     existingConfig[prefix+"_USER"],
		Password:     existingConfig[prefix+"_PASSWORD"],
		ExtraOptions: make(map[string]string),
	}

	// Load extra options
	for k, v := range existingConfig {
		if strings.HasPrefix(k, prefix+"_") && !strings.HasSuffix(k, "_HOST") &&
			!strings.HasSuffix(k, "_PORT") && !strings.HasSuffix(k, "_NAME") &&
			!strings.HasSuffix(k, "_USER") && !strings.HasSuffix(k, "_PASSWORD") {
			optionKey := strings.ToLower(strings.TrimPrefix(k, prefix+"_"))
			config.ExtraOptions[optionKey] = v
		}
	}

	return config, nil
}

// showSavedDatabaseConfig displays the saved database configuration from .env
func showSavedDatabaseConfig() {
	// This function is now deprecated, use showProjectDatabaseConfig instead
	showProjectDatabaseConfig()
}

// showCurrentDatabaseConfig displays the current database configuration from .env
func showCurrentDatabaseConfig() {
	// This function is now deprecated, use showProjectDatabaseConfig instead
	showProjectDatabaseConfig()
}

// showSystemDatabaseConfig displays the system database configuration from .env
func showSystemDatabaseConfig() {
	config, err := loadDatabaseConfig(true) // System database
	if err != nil {
		print_warning("No system database configuration found in .env file")
		return
	}

	fmt.Println()
	print_success("System Database Configuration:")
	fmt.Printf("  Engine: %s\n", strings.Title(config.Engine))
	fmt.Printf("  Host: %s\n", config.Host)
	fmt.Printf("  Port: %s\n", config.Port)
	if config.Database != "" {
		fmt.Printf("  Database: %s\n", config.Database)
	}
	if config.Username != "" {
		fmt.Printf("  Username: %s\n", config.Username)
	}
	fmt.Printf("  Password: %s\n", config.Password)

	if len(config.ExtraOptions) > 0 {
		fmt.Println("  Extra Options:")
		for k, v := range config.ExtraOptions {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	// Show connection string
	connStr := formatConnectionString(*config)
	fmt.Printf("  Connection String: %s\n", connStr)

	// Show environment variables for easy copy-paste
	fmt.Println()
	print_status("Environment variables in .env:")
	fmt.Printf("  APITO_SYSTEM_DB_ENGINE=%s\n", config.Engine)
	fmt.Printf("  SYSTEM_DB_HOST=%s\n", config.Host)
	fmt.Printf("  SYSTEM_DB_PORT=%s\n", config.Port)
	if config.Database != "" {
		fmt.Printf("  SYSTEM_DB_NAME=%s\n", config.Database)
	}
	if config.Username != "" {
		fmt.Printf("  SYSTEM_DB_USER=%s\n", config.Username)
	}
	fmt.Printf("  SYSTEM_DB_PASSWORD=%s\n", config.Password)
	fmt.Printf("  SYSTEM_DATABASE_URL=%s\n", connStr)
	fmt.Println()
}

// showProjectDatabaseConfig displays the project database configuration from .env
func showProjectDatabaseConfig() {
	config, err := loadDatabaseConfig(false) // Project database
	if err != nil {
		print_warning("No project database configuration found in .env file")
		return
	}

	fmt.Println()
	print_success("Project Database Configuration:")
	fmt.Printf("  Engine: %s\n", strings.Title(config.Engine))
	fmt.Printf("  Host: %s\n", config.Host)
	fmt.Printf("  Port: %s\n", config.Port)
	if config.Database != "" {
		fmt.Printf("  Database: %s\n", config.Database)
	}
	if config.Username != "" {
		fmt.Printf("  Username: %s\n", config.Username)
	}
	fmt.Printf("  Password: %s\n", config.Password)

	if len(config.ExtraOptions) > 0 {
		fmt.Println("  Extra Options:")
		for k, v := range config.ExtraOptions {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}

	// Show connection string
	connStr := formatConnectionString(*config)
	fmt.Printf("  Connection String: %s\n", connStr)

	// Show environment variables for easy copy-paste
	fmt.Println()
	print_status("Environment variables in .env:")
	fmt.Printf("  APITO_PROJECT_DB_ENGINE=%s\n", config.Engine)
	fmt.Printf("  PROJECT_DB_HOST=%s\n", config.Host)
	fmt.Printf("  PROJECT_DB_PORT=%s\n", config.Port)
	if config.Database != "" {
		fmt.Printf("  PROJECT_DB_NAME=%s\n", config.Database)
	}
	if config.Username != "" {
		fmt.Printf("  PROJECT_DB_USER=%s\n", config.Username)
	}
	fmt.Printf("  PROJECT_DB_PASSWORD=%s\n", config.Password)
	fmt.Printf("  PROJECT_DATABASE_URL=%s\n", connStr)
	fmt.Println()
}

// showAllDatabaseConfigs displays both system and project database configurations
func showAllDatabaseConfigs() {
	showSystemDatabaseConfig()
	showProjectDatabaseConfig()
}

// startDatabaseInteractive prompts user to choose a database and starts it via Docker.
// No-ops if Docker is not available or running.
func startDatabaseInteractive(dbType string) {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		print_status("Database helper is available in Docker mode only. " + err.Error())
		return
	}

	print_status("Database setup for " + dbType + " database...")
	prompt := promptui.Select{
		Label: "Select a database to run (or Skip)",
		Items: []string{"Postgres", "MySQL", "MariaDB", "SQLServer", "MongoDB", "Skip (I already have a database)"},
	}
	_, choice, err := prompt.Run()
	if err != nil || choice == "Skip (I already have a database)" {
		print_status("Skipping database setup")
		return
	}

	var engine string
	switch choice {
	case "Postgres":
		engine = "postgres"
	case "MySQL":
		engine = "mysql"
	case "MariaDB":
		engine = "mariadb"
	case "SQLServer":
		engine = "sqlserver"
	case "MongoDB":
		engine = "mongodb"
	}

	if engine == "" {
		print_error("Unsupported database selection")
		return
	}

	// Prompt for credentials
	config, err := promptForCredentials(engine)
	if err != nil {
		print_error("Failed to get database credentials: " + err.Error())
		return
	}

	// Create compose file with custom credentials
	path, err := writeDBComposeFileWithConfig(engine, dbType, config)
	if err != nil {
		print_error("Failed to prepare DB compose: " + err.Error())
		return
	}

	// Start the database
	if err := dockerComposeUpFile(path); err != nil {
		print_error("Failed to start database: " + err.Error())
		return
	}

	print_success(dbType + " database started via Docker: " + engine)

	// Save configuration for later reference
	if err := saveDatabaseConfig(config, dbType == "system"); err != nil {
		print_warning("Failed to save database configuration: " + err.Error())
	}

	// Display connection information
	displayConnectionInfo(config)
}
