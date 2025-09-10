package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var selfUpgradeCmd = &cobra.Command{
	Use:   "self-upgrade",
	Short: "Check for updates and upgrade the CLI",
	Long:  `Check for available updates for the Apito CLI and upgrade to the latest version if available.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runSelfUpgrade(); err != nil {
			fmt.Printf("Self-upgrade failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(selfUpgradeCmd)
}

func runSelfUpgrade() error {
	fmt.Println("🔍 Checking for Apito CLI updates...")
	fmt.Println()

	// Get current version
	currentVersion := version
	if currentVersion == "dev" || currentVersion == "" {
		currentVersion = "unknown"
	}

	// Get latest version from GitHub
	latestVersion, err := getLatestVersion("apito-io/cli")
	if err != nil {
		return fmt.Errorf("failed to check for updates: %v", err)
	}

	// Display current and latest versions
	fmt.Printf("Current version: %s\n", currentVersion)
	fmt.Printf("Latest version:  %s\n", latestVersion)
	fmt.Println()

	// Check if update is needed
	if currentVersion == latestVersion {
		fmt.Println("✅ You're already running the latest version!")
		return nil
	}

	if currentVersion != "unknown" && !isVersionNewer(latestVersion, currentVersion) {
		fmt.Println("✅ Your version is newer than the latest release!")
		return nil
	}

	// Ask for confirmation
	upgradeMsg := fmt.Sprintf("Upgrade from %s to %s?", currentVersion, latestVersion)
	if !confirmUpgrade(upgradeMsg) {
		fmt.Println("Upgrade cancelled.")
		return nil
	}

	// Perform the upgrade using install.sh script
	fmt.Println()
	fmt.Println("🚀 Starting self-upgrade...")
	return performSelfUpgrade(latestVersion)
}

func getLatestVersion(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag name found in release")
	}

	return release.TagName, nil
}

func confirmUpgrade(message string) bool {
	prompt := promptui.Select{
		Label: message,
		Items: []string{"Yes", "No"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return false
	}

	return result == "Yes"
}

func isVersionNewer(latest, current string) bool {
	// Simple version comparison - remove 'v' prefix if present
	latestClean := strings.TrimPrefix(latest, "v")
	currentClean := strings.TrimPrefix(current, "v")

	// For now, just do string comparison - in a real implementation
	// you might want to use semver parsing
	return latestClean != currentClean
}

func performSelfUpgrade(latestVersion string) error {
	fmt.Println("📥 Downloading and installing the latest version...")

	// Download the latest release using existing utility function
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Clean version string (remove 'v' prefix if present)
	cleanVersion := strings.TrimPrefix(latestVersion, "v")

	asset := fmt.Sprintf("apito_%s_%s_%s.tar.gz", cleanVersion, osName, arch)
	url := fmt.Sprintf("https://github.com/apito-io/cli/releases/download/%s/%s", latestVersion, asset)

	// Download with progress
	downloadedFile, err := downloadFileWithProgress(url, os.TempDir())
	if err != nil {
		return fmt.Errorf("failed to download: %v", err)
	}
	defer os.Remove(downloadedFile)

	// Extract the archive
	extractDir, err := extractArchiveToTemp(downloadedFile)
	if err != nil {
		return fmt.Errorf("failed to extract: %v", err)
	}
	defer os.RemoveAll(extractDir)

	// Find the apito binary
	newBinaryPath, err := findBinaryInDir(extractDir, "apito")
	if err != nil {
		return fmt.Errorf("failed to find binary in extracted files: %v", err)
	}

	// Get current binary location
	currentBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %v", err)
	}

	// Create backup of current binary
	backupPath := currentBinary + ".backup"
	if err := copyFile(currentBinary, backupPath); err != nil {
		fmt.Printf("Warning: Could not create backup: %v\n", err)
	} else {
		fmt.Printf("📋 Created backup: %s\n", backupPath)
		defer func() {
			// Clean up backup on success
			os.Remove(backupPath)
		}()
	}

	// Replace current binary
	installDir := filepath.Dir(currentBinary)
	if err := installBinary(newBinaryPath, currentBinary, installDir); err != nil {
		// Restore backup if installation fails
		if _, backupErr := os.Stat(backupPath); backupErr == nil {
			copyFile(backupPath, currentBinary)
		}
		return fmt.Errorf("failed to install new binary: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ Self-upgrade completed successfully!")
	fmt.Printf("🎉 Apito CLI has been updated to %s\n", latestVersion)

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return err
	}

	// Copy permissions
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.Chmod(dst, sourceInfo.Mode())
}

// installBinary installs a binary to the target location with proper permissions
func installBinary(srcPath, destPath, installDir string) error {
	// Check if we can write to the install directory
	if isWritable(installDir) {
		// Direct copy
		if err := copyFile(srcPath, destPath); err != nil {
			return err
		}
		return os.Chmod(destPath, 0755)
	}

	// Try with sudo
	if !commandExists("sudo") {
		return fmt.Errorf("cannot write to %s and sudo is not available", installDir)
	}

	fmt.Println("🔐 Requesting sudo permissions to install binary...")

	// Use sudo to copy and set permissions
	copyCmd := exec.Command("sudo", "cp", srcPath, destPath)
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("sudo cp failed: %v", err)
	}

	chmodCmd := exec.Command("sudo", "chmod", "755", destPath)
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("sudo chmod failed: %v", err)
	}

	return nil
}

// isWritable checks if a directory is writable
func isWritable(path string) bool {
	testFile := filepath.Join(path, ".apito-write-test")
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}
