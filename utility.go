package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

const ConfigFile = ".env"

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Magenta = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

func ArrayContains(arr []string, str string) bool {
	for _, k := range arr {
		if k == str {
			return true
		}
	}
	return false
}

func getLatestReleaseTag() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/apito-io/engine/releases/latest")
	if err != nil {
		return "", fmt.Errorf("error fetching latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest release: status code %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	return result.TagName, nil
}

func getConfig(projectDir string) (map[string]string, error) {
	configFile := filepath.Join(projectDir, ConfigFile)
	envMap, err := godotenv.Read(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return envMap, nil
}

func updateConfig(projectDir, key, value string) error {
	envMap, err := getConfig(projectDir)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	envMap[key] = value

	// write goenv back to config file

	if err := saveConfig(projectDir, envMap); err != nil {
		return fmt.Errorf("error saving config file: %w", err)
	}

	return nil
}

func saveConfig(projectDir string, config map[string]string) error {
	configFile := filepath.Join(projectDir, ConfigFile)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		_, err := os.Create(configFile)
		if err != nil {
			return fmt.Errorf("error creating config file: %w", err)
		}
	}

	f, err := os.Open(configFile)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer f.Close()

	// write the config to the file
	if err := godotenv.Write(config, configFile); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}
