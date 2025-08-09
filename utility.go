package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/joho/godotenv"
	"github.com/mholt/archiver/v3"
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

// Function to print colored output
func print_status(message string) {
	fmt.Println(Blue + "[INFO]" + Reset + " " + message)
}

func print_success(message string) {
	fmt.Println(Green + "[SUCCESS]" + Reset + " " + message)
}

func print_warning(message string) {
	fmt.Println(Yellow + "[WARNING]" + Reset + " " + message)
}

func print_error(message string) {
	fmt.Println(Red + "[ERROR]" + Reset + " " + message)
}

func print_step(message string) {
	fmt.Println(Magenta + "[STEP]" + Reset + " " + message)
}

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

func downloadAndExtractEngine(projectName, releaseTag string, destDir string) error {
	// This function is kept for backward compatibility with update command
	// In the new architecture, projects are created via API calls
	print_warning("Engine download is deprecated. Projects are now created via API.")
	return nil
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

// ensureBaseDirs creates core directories under ~/.apito used by both modes
func ensureBaseDirs() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	binDir := filepath.Join(apitoDir, "bin")
	engineDataDir := filepath.Join(apitoDir, "engine-data")
	logsDir := filepath.Join(apitoDir, "logs")
	runDir := filepath.Join(apitoDir, "run")

	for _, d := range []string{apitoDir, binDir, engineDataDir, logsDir, runDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("error creating directory %s: %w", d, err)
		}
	}
	return nil
}

// downloadFileWithProgress downloads a URL into destDir with progress output and returns the downloaded file path.
func downloadFileWithProgress(url, destDir string) (string, error) {
	resp, err := grab.Get(destDir, url)
	if err != nil {
		return "", fmt.Errorf("error downloading: %w", err)
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
Loop:
	for {
		select {
		case <-ticker.C:
			fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n", resp.BytesComplete(), resp.Size(), 100*resp.Progress())
		case <-resp.Done:
			break Loop
		}
	}
	if err := resp.Err(); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	return resp.Filename, nil
}

// extractArchiveToTemp extracts an archive file into a unique temp directory and returns the directory path.
func extractArchiveToTemp(archivePath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "apito-extract-*")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	if err := archiver.Unarchive(archivePath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("error extracting: %w", err)
	}
	return tmpDir, nil
}

// findBinaryInDir recursively finds a binary by name within root and returns full path.
func findBinaryInDir(root, name string) (string, error) {
	binName := name
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		binName = name + ".exe"
	}
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == binName {
			found = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in %s", binName, root)
	}
	return found, nil
}

// moveAndChmod moves a file to destDir/name and makes it executable. Returns the final path.
func moveAndChmod(srcPath, destDir, name string) (string, error) {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("error creating dir: %w", err)
	}
	destPath := filepath.Join(destDir, name)
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(destPath), ".exe") {
		destPath += ".exe"
	}
	if err := os.Rename(srcPath, destPath); err != nil {
		return "", fmt.Errorf("error moving file: %w", err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(destPath, 0755); err != nil {
			return "", fmt.Errorf("error chmod: %w", err)
		}
	}
	return destPath, nil
}
