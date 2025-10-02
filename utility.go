package main

import (
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
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

// getConfig reads configuration from a project directory (deprecated, use ReadEnv instead)
func getConfig(projectDir string) (map[string]string, error) {
	// For backward compatibility, if the path contains "bin", use ReadEnv
	if strings.Contains(projectDir, "bin") {
		return ReadEnv()
	}

	// Otherwise, use the old method for project-specific configs
	configFile := filepath.Join(projectDir, ConfigFile)
	envMap, err := godotenv.Read(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	return envMap, nil
}

// saveConfig saves configuration to a project directory (deprecated, use WriteEnv instead)
func saveConfig(projectDir string, config map[string]string) error {
	// For backward compatibility, if the path contains "bin", use WriteEnv
	if strings.Contains(projectDir, "bin") {
		return WriteEnv(config)
	}

	// Otherwise, use the old method for project-specific configs
	configFile := filepath.Join(projectDir, ConfigFile)
	
	// Ensure the directory exists
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	// Read existing config to preserve other variables
	existingConfig, err := godotenv.Read(configFile)
	if err != nil {
		// If file doesn't exist, start with empty config
		existingConfig = make(map[string]string)
	}

	// Merge new config with existing config
	for k, v := range config {
		existingConfig[k] = v
	}

	// Write the merged config to the file
	if err := godotenv.Write(existingConfig, configFile); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// updateConfig updates a single configuration value in a project directory
func updateConfig(projectDir, key, value string) error {
	envMap, err := getConfig(projectDir)
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	envMap[key] = value

	// write back to config file
	if err := saveConfig(projectDir, envMap); err != nil {
		return fmt.Errorf("error saving config file: %w", err)
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
	// Detect extension
	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		zr, err := zip.OpenReader(archivePath)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("open zip: %w", err)
		}
		defer zr.Close()
		for _, f := range zr.File {
			fp := filepath.Join(tmpDir, f.Name)
			if f.FileInfo().IsDir() {
				if err := os.MkdirAll(fp, 0755); err != nil {
					os.RemoveAll(tmpDir)
					return "", err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
				os.RemoveAll(tmpDir)
				return "", err
			}
			rc, err := f.Open()
			if err != nil {
				os.RemoveAll(tmpDir)
				return "", err
			}
			out, err := os.Create(fp)
			if err != nil {
				rc.Close()
				os.RemoveAll(tmpDir)
				return "", err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				rc.Close()
				os.RemoveAll(tmpDir)
				return "", err
			}
			out.Close()
			rc.Close()
		}
	} else if strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") || strings.HasSuffix(strings.ToLower(archivePath), ".tgz") {
		f, err := os.Open(archivePath)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}
		defer gz.Close()
		// Use tar utility if available to keep implementation small
		// Write to tmp file then use system tar
		tmpTar := filepath.Join(tmpDir, "archive.tar")
		tf, err := os.Create(tmpTar)
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}
		if _, err := io.Copy(tf, gz); err != nil {
			tf.Close()
			os.RemoveAll(tmpDir)
			return "", err
		}
		tf.Close()
		// Extract with tar -xf
		if err := execCommand("tar", "-xf", tmpTar, "-C", tmpDir); err != nil {
			os.RemoveAll(tmpDir)
			return "", err
		}
		_ = os.Remove(tmpTar)
	} else {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("unsupported archive format: %s", archivePath)
	}
	return tmpDir, nil
}

func execCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
