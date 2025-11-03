package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringP("version", "v", "", "Target version (optional)")
}

var updateCmd = &cobra.Command{
	Use:       "update",
	Short:     "Update apito engine, console, or self",
	Long:      `Update apito engine, console, or the CLI itself to the latest or specified version`,
	ValidArgs: []string{"engine", "console", "self"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		targetVersion, _ := cmd.Flags().GetString("version")
		actionName := args[0]

		switch actionName {
		case "engine":
			if err := updateEngineCommand(targetVersion); err != nil {
				print_error("Update engine failed: " + err.Error())
			}
		case "console":
			if err := updateConsoleCommand(targetVersion); err != nil {
				print_error("Update console failed: " + err.Error())
			}
		case "self", "cli":
			if err := runSelfUpgrade(); err != nil {
				print_error("Self update failed: " + err.Error())
			}
		}
	},
}

func latestTag(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TagName == "" {
		return "", errors.New("empty tag")
	}
	return out.TagName, nil
}

func confirmPrompt(msg string) bool {
	p := promptui.Select{Label: msg, Items: []string{"Yes", "No"}}
	_, v, err := p.Run()
	if err != nil {
		return false
	}
	return v == "Yes"
}

// updateEngineCommand updates engine based on current mode (docker or manual)
func updateEngineCommand(targetVersion string) error {
	// Determine current run mode
	mode, err := determineRunMode()
	if err != nil {
		return fmt.Errorf("failed to determine run mode: %w", err)
	}

	print_step("🔄 Updating Engine")
	print_status(fmt.Sprintf("Mode: %s", mode))
	fmt.Println()

	if mode == "docker" {
		return updateEngineDocker(targetVersion)
	}
	return updateEngineManual(targetVersion)
}

// updateConsoleCommand updates console based on current mode (docker or manual)
func updateConsoleCommand(targetVersion string) error {
	// Determine current run mode
	mode, err := determineRunMode()
	if err != nil {
		return fmt.Errorf("failed to determine run mode: %w", err)
	}

	print_step("🔄 Updating Console")
	print_status(fmt.Sprintf("Mode: %s", mode))
	fmt.Println()

	if mode == "docker" {
		return updateConsoleDocker(targetVersion)
	}
	return updateConsoleManual(targetVersion)
}

// updateEngineDocker updates engine Docker image and config
func updateEngineDocker(targetVersion string) error {
	// If no version specified, get latest
	if targetVersion == "" {
		print_status("Fetching latest engine version...")
		latestVersion, err := getLatestEngineVersion()
		if err != nil {
			return fmt.Errorf("failed to fetch latest version: %w", err)
		}
		targetVersion = latestVersion
		print_success(fmt.Sprintf("Latest version: %s", targetVersion))
	}

	// Get current version from config
	cfg, err := loadCLIConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	currentVersion := cfg.EngineVersion
	if currentVersion == "" {
		currentVersion = "unknown"
	}

	// Check if already on target version
	if currentVersion == targetVersion {
		print_success(fmt.Sprintf("Engine already at version %s", targetVersion))
		return nil
	}

	// Confirm update
	print_status(fmt.Sprintf("Current version: %s", currentVersion))
	print_status(fmt.Sprintf("Target version: %s", targetVersion))
	fmt.Println()

	if !confirmPrompt(fmt.Sprintf("Update engine from %s to %s?", currentVersion, targetVersion)) {
		print_status("Update cancelled")
		return nil
	}

	// Pull Docker image
	if err := pullDockerImage("engine", targetVersion); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Update config.yml
	if err := updateComponentVersion("engine", targetVersion); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Regenerate docker-compose.yml
	print_status("Updating docker-compose.yml...")
	if _, err := writeComposeFile(); err != nil {
		return fmt.Errorf("failed to update docker-compose.yml: %w", err)
	}
	print_success("docker-compose.yml updated")

	fmt.Println()
	print_success(fmt.Sprintf("✅ Engine updated to %s", targetVersion))
	print_status("Run 'apito restart' to apply changes")

	return nil
}

// updateConsoleDocker updates console Docker image and config
func updateConsoleDocker(targetVersion string) error {
	// If no version specified, get latest
	if targetVersion == "" {
		print_status("Fetching latest console version...")
		latestVersion, err := getLatestConsoleVersion()
		if err != nil {
			return fmt.Errorf("failed to fetch latest version: %w", err)
		}
		targetVersion = latestVersion
		print_success(fmt.Sprintf("Latest version: %s", targetVersion))
	}

	// Get current version from config
	cfg, err := loadCLIConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	currentVersion := cfg.ConsoleVersion
	if currentVersion == "" {
		currentVersion = "unknown"
	}

	// Check if already on target version
	if currentVersion == targetVersion {
		print_success(fmt.Sprintf("Console already at version %s", targetVersion))
		return nil
	}

	// Confirm update
	print_status(fmt.Sprintf("Current version: %s", currentVersion))
	print_status(fmt.Sprintf("Target version: %s", targetVersion))
	fmt.Println()

	if !confirmPrompt(fmt.Sprintf("Update console from %s to %s?", currentVersion, targetVersion)) {
		print_status("Update cancelled")
		return nil
	}

	// Pull Docker image
	if err := pullDockerImage("console", targetVersion); err != nil {
		return fmt.Errorf("failed to pull image: %w", err)
	}

	// Update config.yml
	if err := updateComponentVersion("console", targetVersion); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	// Regenerate docker-compose.yml
	print_status("Updating docker-compose.yml...")
	if _, err := writeComposeFile(); err != nil {
		return fmt.Errorf("failed to update docker-compose.yml: %w", err)
	}
	print_success("docker-compose.yml updated")

	fmt.Println()
	print_success(fmt.Sprintf("✅ Console updated to %s", targetVersion))
	print_status("Run 'apito restart' to apply changes")

	return nil
}

// updateEngineManual updates engine binary for manual mode
func updateEngineManual(targetVersion string) error {
	if targetVersion == "" {
		tag, err := latestTag("apito-io/engine")
		if err != nil {
			return err
		}
		targetVersion = tag
	}
	if !confirmPrompt("Update engine to " + targetVersion + "?") {
		print_status("Update cancelled")
		return nil
	}
	home, _ := os.UserHomeDir()
	if err := downloadEngine(targetVersion, home); err != nil {
		return err
	}
	print_success(fmt.Sprintf("✅ Engine updated to %s", targetVersion))
	return nil
}

// updateConsoleManual updates console binary for manual mode
func updateConsoleManual(targetVersion string) error {
	if targetVersion == "" {
		tag, err := latestTag("apito-io/console")
		if err != nil {
			return err
		}
		targetVersion = tag
	}
	if !confirmPrompt("Update console to " + targetVersion + "?") {
		print_status("Update cancelled")
		return nil
	}
	home, _ := os.UserHomeDir()
	if err := downloadConsole(targetVersion, home); err != nil {
		return err
	}
	print_success(fmt.Sprintf("✅ Console updated to %s", targetVersion))
	return nil
}
