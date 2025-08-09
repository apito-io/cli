package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v3"
)

type CLIConfig struct {
	Mode string `yaml:"mode"` // "docker" or "manual"
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
	cfg := &CLIConfig{}
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
