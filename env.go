package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
)

const (
	BinDir = "~/.apito/bin"
)

// getEnvPath returns the full path to the .env file
func getEnvPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding home directory: %w", err)
	}
	return filepath.Join(homeDir, ".apito", "bin", ConfigFile), nil
}

// ReadEnv reads the entire .env file and returns a map of key-value pairs
func ReadEnv() (map[string]string, error) {
	envPath, err := getEnvPath()
	if err != nil {
		return nil, err
	}

	// Check if .env file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// Return empty map if file doesn't exist
		return make(map[string]string), nil
	}

	// Read the .env file
	envMap, err := godotenv.Read(envPath)
	if err != nil {
		return nil, fmt.Errorf("error reading .env file: %w", err)
	}

	return envMap, nil
}

// WriteEnv writes the entire environment configuration to the .env file
func WriteEnv(envMap map[string]string) error {
	envPath, err := getEnvPath()
	if err != nil {
		return err
	}

	// Ensure the directory exists
	binDir := filepath.Dir(envPath)
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("error creating bin directory: %w", err)
	}

	// Write the environment variables to the .env file
	if err := godotenv.Write(envMap, envPath); err != nil {
		return fmt.Errorf("error writing .env file: %w", err)
	}

	return nil
}

// UpdateEnv updates a single environment variable in the .env file
func UpdateEnv(key, value string) error {
	// Read existing configuration
	envMap, err := ReadEnv()
	if err != nil {
		return err
	}

	// Check if the key already exists
	if existingValue, exists := envMap[key]; exists {
		fmt.Println()
		print_warning("Existing environment variable found:")
		fmt.Printf("  Key: %s\n", key)
		fmt.Printf("  Current Value: %s\n", existingValue)
		fmt.Printf("  New Value: %s\n", value)
		fmt.Println()

		// Ask for confirmation
		prompt := promptui.Select{
			Label: "Do you want to overwrite the existing value?",
			Items: []string{"Yes, overwrite existing value", "No, keep existing value"},
		}

		_, choice, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}

		if choice == "No, keep existing value" {
			print_status("Keeping existing value for " + key)
			return nil
		}

		print_status("Overwriting existing value for " + key)
	}

	// Update the specific key
	envMap[key] = value

	// Write back to file
	return WriteEnv(envMap)
}

// UpdateMultipleEnv updates multiple environment variables in the .env file
func UpdateMultipleEnv(updates map[string]string) error {
	// Read existing configuration
	envMap, err := ReadEnv()
	if err != nil {
		return err
	}

	// Check for existing keys
	existingKeys := []string{}
	for key := range updates {
		if _, exists := envMap[key]; exists {
			existingKeys = append(existingKeys, key)
		}
	}

	// If there are existing keys, show them and ask for confirmation
	if len(existingKeys) > 0 {
		fmt.Println()
		print_warning("Existing environment variables found:")
		for _, key := range existingKeys {
			fmt.Printf("  %s: %s -> %s\n", key, envMap[key], updates[key])
		}
		fmt.Println()

		// Ask for confirmation
		prompt := promptui.Select{
			Label: "Do you want to overwrite the existing values?",
			Items: []string{"Yes, overwrite existing values", "No, keep existing values"},
		}

		_, choice, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("failed to get user confirmation: %w", err)
		}

		if choice == "No, keep existing values" {
			print_status("Keeping existing values")
			return nil
		}

		print_status("Overwriting existing values...")
	}

	// Apply all updates
	for key, value := range updates {
		envMap[key] = value
	}

	// Write back to file
	return WriteEnv(envMap)
}

// GetEnv retrieves a single environment variable value from the .env file
func GetEnv(key string) (string, error) {
	envMap, err := ReadEnv()
	if err != nil {
		return "", err
	}

	value, exists := envMap[key]
	if !exists {
		return "", fmt.Errorf("environment variable %s not found", key)
	}

	return value, nil
}

// GetEnvOrDefault retrieves an environment variable with a default fallback
func GetEnvOrDefault(key, defaultValue string) string {
	value, err := GetEnv(key)
	if err != nil {
		return defaultValue
	}
	return value
}

// DeleteEnv removes an environment variable from the .env file
func DeleteEnv(key string) error {
	// Read existing configuration
	envMap, err := ReadEnv()
	if err != nil {
		return err
	}

	// Remove the key
	delete(envMap, key)

	// Write back to file
	return WriteEnv(envMap)
}

// EnvExists checks if a specific environment variable exists in the .env file
func EnvExists(key string) (bool, error) {
	envMap, err := ReadEnv()
	if err != nil {
		return false, err
	}

	_, exists := envMap[key]
	return exists, nil
}

// ListEnvKeys returns all environment variable keys from the .env file
func ListEnvKeys() ([]string, error) {
	envMap, err := ReadEnv()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}

	return keys, nil
}

// ValidateRequiredEnv checks if all required environment variables are present
func ValidateRequiredEnv(requiredKeys []string) error {
	envMap, err := ReadEnv()
	if err != nil {
		return err
	}

	missingKeys := []string{}
	for _, key := range requiredKeys {
		if value, exists := envMap[key]; !exists || strings.TrimSpace(value) == "" {
			missingKeys = append(missingKeys, key)
		}
	}

	if len(missingKeys) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missingKeys, ", "))
	}

	return nil
}

// BackupEnv creates a backup of the current .env file
func BackupEnv() (string, error) {
	envPath, err := getEnvPath()
	if err != nil {
		return "", err
	}

	// Check if .env file exists
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return "", fmt.Errorf("no .env file to backup")
	}

	// Create backup path with timestamp
	backupDir := filepath.Join(filepath.Dir(envPath), "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("error creating backup directory: %w", err)
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf(".env.backup.%s", time.Now().Format("20060102_150405")))

	// Copy the file
	input, err := os.ReadFile(envPath)
	if err != nil {
		return "", fmt.Errorf("error reading .env file: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return "", fmt.Errorf("error writing backup file: %w", err)
	}

	return backupPath, nil
}
