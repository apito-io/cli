package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var startWithDB bool

var startCmd = &cobra.Command{
	Use:   "start [--db system|project]",
	Short: "Start the Apito engine and console",
	Long:  `Start the Apito engine and console with automatic setup and downloads. Optionally start a system or project database in Docker mode.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if --db flag is set
		dbType, _ := cmd.Flags().GetString("db")

		// If --db specified, run database helper first
		if dbType != "" {
			if dbType != "system" && dbType != "project" {
				print_error("Invalid database type. Use 'system' or 'project'")
				return
			}
			startDatabaseInteractive(dbType)
		} else {
			startApito()
		}
	},
}

func init() {
	startCmd.Flags().String("db", "", "Start a database via Docker before services (system|project)")
}

func startApito() {
	print_step("🚀 Starting Apito Engine and Console")
	fmt.Println()

	// Ensure required directories/files exist
	if err := ensureBaseDirs(); err != nil {
		print_error("Failed to prepare directories: " + err.Error())
		return
	}

	// Determine run mode (docker/manual)
	mode, err := determineRunMode()
	if err != nil {
		print_error("Failed to determine run mode: " + err.Error())
		return
	}

	print_status("Run mode: " + mode)

	switch mode {
	case "manual":
		// Step 1: Check port availability (interactive) only in manual mode
		print_status("Step 1: Checking port availability...")
		// Ensure apito is initialized
		homeDir, err := os.UserHomeDir()
		if err != nil {
			print_error("Unable to determine home directory: " + err.Error())
			return
		}
		apitoDir := filepath.Join(homeDir, ".apito")
		if _, statErr := os.Stat(apitoDir); os.IsNotExist(statErr) {
			print_error("Apito is not initialized. Please run: apito init")
			return
		}

		// Validate critical assets in both modes
		binDir := filepath.Join(apitoDir, "bin")
		envFile := filepath.Join(binDir, ".env")
		dbDir := filepath.Join(apitoDir, "db")
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			print_error("Missing ~/.apito/bin/.env. Please run: apito init")
			return
		}
		if _, err := os.Stat(dbDir); os.IsNotExist(err) {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				print_error("Failed to create db directory: " + err.Error())
				return
			}
		}

		if err := checkPortAvailabilityInteractive(); err != nil {
			print_warning("Port availability check encountered issues: " + err.Error())
		}
		print_success("Port availability check completed")

		if mode == "manual" {
			// Step 2: Check and download engine
			print_status("Step 2: Checking engine binary...")
			if err := ensureEngineBinary(); err != nil {
				print_error("Failed to ensure engine binary: " + err.Error())
				return
			}
			print_success("Engine binary ready")
			fmt.Println()
		}

		// Step 3: Check if engine is already running
		print_status("Step 3: Checking if engine is already running...")
		if running, _, _ := serviceRunning("engine"); running {
			print_warning("Engine is already running")
			print_status("Skipping engine startup")
		} else {
			print_status("Engine is not running, will start it")
		}
		fmt.Println()

		// Step 4: Check and download console
		print_status("Step 4: Checking console files...")
		if err := ensureConsoleFiles(); err != nil {
			print_error("Failed to ensure console files: " + err.Error())
			return
		}
		print_success("Console files ready")
		fmt.Println()

		// Step 5: Check and install Caddy
		print_status("Step 5: Checking Caddy installation...")
		if err := ensureCaddyInstalled(); err != nil {
			print_error("Failed to ensure Caddy installation: " + err.Error())
			return
		}
		print_success("Caddy ready")
		fmt.Println()

		// Step 6: Start engine (if not already running)
		if running, _, _ := serviceRunning("engine"); !running {
			print_status("Step 6: Starting engine...")
			if err := startManagedService("engine"); err != nil {
				print_error("Failed to start engine: " + err.Error())
				return
			}
			print_success("Engine started successfully")
			fmt.Println()
		}

		// Step 7: Start console with Caddy
		print_status("Step 7: Starting console with Caddy...")
		if err := setupCaddyfile(); err != nil {
			print_error("Failed to prepare console config: " + err.Error())
			return
		}
		if running, _, _ := serviceRunning("console"); !running {
			if err := startManagedService("console"); err != nil {
				print_error("Failed to start console: " + err.Error())
				return
			}
		}
		print_success("Console started successfully")
		fmt.Println()

	case "docker":
		print_status("Docker mode: skipping local port pre-check (Docker will map ports)")

		// Check for component updates before starting
		print_status("Checking for component updates...")
		updates, err := checkForComponentUpdates()
		if err != nil {
			print_warning("Could not check for updates: " + err.Error())
		} else if len(updates) > 0 {
			// Prompt user for updates
			componentsToUpdate := promptForComponentUpdates(updates)
			if len(componentsToUpdate) > 0 {
				for _, component := range componentsToUpdate {
					update := updates[component]

					// Pull the new image
					if err := pullDockerImage(component, update.LatestVersion); err != nil {
						print_error(fmt.Sprintf("Failed to pull %s: %v", component, err))
						continue
					}

					// Update config.yml
					if err := updateComponentVersion(component, update.LatestVersion); err != nil {
						print_error(fmt.Sprintf("Failed to update config for %s: %v", component, err))
						continue
					}

					print_success(fmt.Sprintf("Updated %s to %s", component, update.LatestVersion))
				}

				// Regenerate docker-compose.yml with new versions
				print_status("Updating docker-compose.yml...")
				if _, err := writeComposeFile(); err != nil {
					print_error("Failed to update docker-compose.yml: " + err.Error())
				} else {
					print_success("docker-compose.yml updated")
				}
			} else {
				print_status("Skipping updates, using current versions")
			}
		} else {
			print_success("All components are up to date")
		}
		fmt.Println()

		// Docker mode - start services
		print_step("🐳 Starting Docker Services")
		if err := ensureEnvFileReady(); err != nil {
			print_error("Failed to ensure .env file: " + err.Error())
			return
		}
		if err := ensureDockerAndComposeAvailable(); err != nil {
			print_error("Docker not available: " + err.Error())
			return
		}
		if err := dockerComposeUp(); err != nil {
			print_error("Failed to start Docker services: " + err.Error())
			return
		}
		fmt.Println()
		print_success("Docker services started (engine:5050, console:4000)")

	default:
		print_error("Invalid run mode: " + mode)
		return
	}
	fmt.Println()

	print_success("🎉 Apito is now running!")
	print_status("Engine: http://localhost:5050")
	print_status("Console: http://localhost:4000")
	print_status("Services run in the background.")
	print_status("First time? Run 'apito logs engine' for login credentials.")
	print_status("To stop services, run 'apito stop'")
	fmt.Println()
}

func ensureEngineBinary() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	enginePath := filepath.Join(homeDir, ".apito", "bin", "engine")

	// Check if engine binary exists
	if _, err := os.Stat(enginePath); err == nil {
		print_status("Engine binary already exists")
		return nil
	}

	print_status("Engine binary not found, downloading...")

	// Get latest release tag
	releaseTag, err := getLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("error fetching latest release tag: %w", err)
	}

	// Download engine based on system architecture
	if err := downloadEngine(releaseTag, homeDir); err != nil {
		return fmt.Errorf("error downloading engine: %w", err)
	}

	return nil
}

func downloadEngine(releaseTag, homeDir string) error {
	baseURL := fmt.Sprintf("https://github.com/apito-io/engine/releases/download/%s/", releaseTag)
	var assetName string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			assetName = "engine-linux-arm64.zip"
		} else {
			assetName = "engine-linux-amd64.zip"
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			assetName = "engine-darwin-arm64.zip"
		} else {
			assetName = "engine-darwin-amd64.zip"
		}
	case "windows":
		if runtime.GOARCH == "arm64" {
			assetName = "engine-windows-arm64.zip"
		} else {
			assetName = "engine-windows-amd64.zip"
		}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	assetURL := baseURL + assetName
	print_status("Downloading engine from: " + assetURL)

	// Download to temp directory
	resp, err := grab.Get(os.TempDir(), assetURL)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}

	// Show download progress
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n",
				resp.BytesComplete(),
				resp.Size(),
				100*resp.Progress())
		case <-resp.Done:
			break Loop
		}
	}

	if err := resp.Err(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	print_status("Downloaded file saved to: " + resp.Filename)

	// Extract into a unique temp directory
	tmpDir, err := os.MkdirTemp("", "apito-engine-*")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := extractArchiveToTemp(resp.Filename); err != nil {
		return fmt.Errorf("error extracting file: %w", err)
	}

	// Locate engine binary
	engineBinary := "engine"
	if runtime.GOOS == "windows" {
		engineBinary += ".exe"
	}
	sourcePath, err := findBinaryInDir(tmpDir, engineBinary)
	if err != nil {
		return fmt.Errorf("unable to locate engine binary: %w", err)
	}

	// Move to ~/.apito/bin/engine
	destDir := filepath.Join(homeDir, ".apito", "bin")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating bin directory: %w", err)
	}
	destPath := filepath.Join(destDir, engineBinary)
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("error moving engine binary: %w", err)
	}
	if err := os.Chmod(destPath, 0755); err != nil {
		return fmt.Errorf("error making binary executable: %w", err)
	}

	print_status("Engine binary extracted to: " + destPath)
	return nil
}

func isEngineRunning() bool {
	// Check ENGINE_PID in config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	config, err := getConfig(filepath.Join(homeDir, ".apito"))
	if err != nil {
		return false
	}

	pidStr, ok := config["ENGINE_PID"]
	if !ok {
		return false
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Try to send signal 0 to check if process is running
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
}

func ensureConsoleFiles() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	consoleDir := filepath.Join(homeDir, ".apito", "console")

	// Check if console directory exists
	if _, err := os.Stat(consoleDir); err == nil {
		print_status("Console files already exist")
		return nil
	}

	print_status("Console files not found, downloading...")

	// Get latest console release
	consoleReleaseTag, err := getLatestConsoleReleaseTag()
	if err != nil {
		return fmt.Errorf("error fetching latest console release: %w", err)
	}

	// Download console
	if err := downloadConsole(consoleReleaseTag, homeDir); err != nil {
		return fmt.Errorf("error downloading console: %w", err)
	}

	return nil
}

func getLatestConsoleReleaseTag() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/apito-io/console/releases/latest")
	if err != nil {
		return "", fmt.Errorf("error fetching latest console release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest console release: status code %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding console response: %w", err)
	}

	return result.TagName, nil
}

func downloadConsole(releaseTag, homeDir string) error {
	baseURL := fmt.Sprintf("https://github.com/apito-io/console/releases/download/%s/", releaseTag)
	assetURL := baseURL + fmt.Sprintf("console-%s.zip", releaseTag)

	print_status("Downloading console from: " + assetURL)

	// Download the file
	resp, err := grab.Get(homeDir, assetURL)
	if err != nil {
		return fmt.Errorf("error downloading console: %w", err)
	}

	// Show download progress
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n",
				resp.BytesComplete(),
				resp.Size(),
				100*resp.Progress())

		case <-resp.Done:
			break Loop
		}
	}

	// Check for errors
	if err := resp.Err(); err != nil {
		return fmt.Errorf("console download failed: %w", err)
	}

	print_status("Console downloaded to: " + resp.Filename)

	// Extract to console directory
	consoleDir := filepath.Join(homeDir, ".apito", "console")
	if _, err := extractArchiveToTemp(resp.Filename); err != nil {
		return fmt.Errorf("error extracting console: %w", err)
	}

	print_status("Console extracted to: " + consoleDir)
	return nil
}

func ensureCaddyInstalled() error {
	// Check if Caddy is already installed
	if foundPath, err := exec.LookPath("caddy"); err == nil {
		print_status("Caddy is already installed")
		// Persist discovered path for future use
		if homeDir, herr := os.UserHomeDir(); herr == nil {
			_ = updateEnvConfig(filepath.Join(homeDir, ".apito", "bin"), "CADDY_PATH", foundPath)
		}
		return nil
	}

	print_status("Caddy not found, downloading and installing...")

	// Get latest Caddy release
	caddyReleaseTag, err := getLatestCaddyReleaseTag()
	if err != nil {
		return fmt.Errorf("error fetching latest Caddy release: %w", err)
	}

	// Download and install Caddy
	installedPath, err := downloadAndInstallCaddy(caddyReleaseTag)
	if err != nil {
		return fmt.Errorf("error installing Caddy: %w", err)
	}

	// Save installed path to config for later use
	if homeDir, herr := os.UserHomeDir(); herr == nil {
		_ = updateEnvConfig(filepath.Join(homeDir, ".apito", "bin"), "CADDY_PATH", installedPath)
	}

	return nil
}

func getLatestCaddyReleaseTag() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/caddyserver/caddy/releases/latest")
	if err != nil {
		return "", fmt.Errorf("error fetching latest Caddy release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest Caddy release: status code %d", resp.StatusCode)
	}

	var result struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding Caddy response: %w", err)
	}

	return result.TagName, nil
}

func downloadAndInstallCaddy(releaseTag string) (string, error) {
	// Determine Caddy asset name based on system
	var assetName string
	switch runtime.GOOS {
	case "linux":
		if runtime.GOARCH == "arm64" {
			assetName = "caddy_" + strings.TrimPrefix(releaseTag, "v") + "_linux_arm64.tar.gz"
		} else {
			assetName = "caddy_" + strings.TrimPrefix(releaseTag, "v") + "_linux_amd64.tar.gz"
		}
	case "darwin":
		if runtime.GOARCH == "arm64" {
			assetName = "caddy_" + strings.TrimPrefix(releaseTag, "v") + "_mac_arm64.tar.gz"
		} else {
			assetName = "caddy_" + strings.TrimPrefix(releaseTag, "v") + "_mac_amd64.tar.gz"
		}
	case "windows":
		assetName = "caddy_" + strings.TrimPrefix(releaseTag, "v") + "_windows_amd64.zip"
	default:
		return "", fmt.Errorf("unsupported OS for Caddy: %s", runtime.GOOS)
	}

	baseURL := fmt.Sprintf("https://github.com/caddyserver/caddy/releases/download/%s/", releaseTag)
	assetURL := baseURL + assetName

	print_status("Downloading Caddy from: " + assetURL)

	// Download Caddy
	resp, err := grab.Get(os.TempDir(), assetURL)
	if err != nil {
		return "", fmt.Errorf("error downloading Caddy: %w", err)
	}

	// Extract Caddy into a unique temp directory to avoid name collisions
	tmpDir, err := os.MkdirTemp("", "apito-caddy-*")
	if err != nil {
		return "", fmt.Errorf("error creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if _, err := extractArchiveToTemp(resp.Filename); err != nil {
		return "", fmt.Errorf("error extracting Caddy: %w", err)
	}

	// Locate extracted caddy binary
	caddyBinary := "caddy"
	if runtime.GOOS == "windows" {
		caddyBinary += ".exe"
	}
	sourcePath, err := findBinaryInDir(tmpDir, caddyBinary)
	if err != nil {
		return "", fmt.Errorf("unable to locate caddy binary after extraction: %w", err)
	}

	// Move Caddy to a location in PATH
	destPath := filepath.Join("/usr/local/bin", caddyBinary)

	// Try to move to /usr/local/bin (requires sudo on some systems)
	if err := os.Rename(sourcePath, destPath); err != nil {
		// Fallback to ~/.apito/bin
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("error finding home directory: %w", err)
		}

		apitoBinDir := filepath.Join(homeDir, ".apito", "bin")
		if err := os.MkdirAll(apitoBinDir, 0755); err != nil {
			return "", fmt.Errorf("error creating apito bin directory: %w", err)
		}

		destPath = filepath.Join(apitoBinDir, caddyBinary)
		if err := os.Rename(sourcePath, destPath); err != nil {
			return "", fmt.Errorf("error moving Caddy binary: %w", err)
		}

		print_warning("Caddy installed to " + destPath)
		print_warning("Please add " + apitoBinDir + " to your PATH")
	} else {
		print_status("Caddy installed to " + destPath)
	}

	// Make it executable
	if err := os.Chmod(destPath, 0755); err != nil {
		return "", fmt.Errorf("error making Caddy executable: %w", err)
	}

	return destPath, nil
}

func startEngine() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	enginePath := filepath.Join(homeDir, ".apito", "bin", "engine")

	// Start engine in background
	cmd := exec.Command(enginePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting engine: %w", err)
	}

	// Legacy: previously saved PID to config. Now managed via PID files; keep no-op for compatibility.

	print_status("Engine started with PID: " + strconv.Itoa(cmd.Process.Pid))
	return nil
}

// setupCaddyfile creates or updates the Caddyfile for serving the console
func setupCaddyfile() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}

	consoleDir := filepath.Join(homeDir, ".apito", "console")

	// Create Caddyfile
	caddyfilePath := filepath.Join(homeDir, ".apito", "Caddyfile")
	caddyfileContent := fmt.Sprintf(`:4000 {
	root * %s
	file_server
	encode gzip
}`, consoleDir)

	if err := os.WriteFile(caddyfilePath, []byte(caddyfileContent), 0644); err != nil {
		return fmt.Errorf("error creating Caddyfile: %w", err)
	}

	return nil
}

// resolveCaddyPath returns the absolute path to the caddy binary if known,
// otherwise falls back to the name "caddy" to let the OS PATH resolution handle it.
func resolveCaddyPath() string {
	// Prefer configured path
	if homeDir, err := os.UserHomeDir(); err == nil {
		if cfg, err := getConfig(filepath.Join(homeDir, ".apito", "bin")); err == nil {
			if p, ok := cfg["CADDY_PATH"]; ok && p != "" {
				return p
			}
		}
	}

	// Next, system PATH
	if lp, err := exec.LookPath("caddy"); err == nil {
		return lp
	}

	// Common install locations
	if homeDir, err := os.UserHomeDir(); err == nil {
		// Prefer managed location inside ~/.apito/bin
		candidate := filepath.Join(homeDir, ".apito", "bin", "caddy")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if _, err := os.Stat("/usr/local/bin/caddy"); err == nil {
		return "/usr/local/bin/caddy"
	}

	// Fallback: rely on PATH (will error when executed if not found)
	return "caddy"
}

// findFileRecursive moved to utility: findBinaryInDir

// checkPortAvailabilityInteractive checks ports 5050 and 4000 and, if occupied,
// asks the user whether to terminate the process using the port.
func checkPortAvailabilityInteractive() error {
	ports := []int{5050, 4000}

	for _, port := range ports {
		address := ":" + strconv.Itoa(port)
		listener, err := net.Listen("tcp", address)
		if err != nil {
			print_warning(fmt.Sprintf("Port %d is already in use", port))

			// Prompt user to free the port
			prompt := promptui.Select{
				Label: fmt.Sprintf("Port %d is in use. Free it now?", port),
				Items: []string{"Yes, kill the process", "No, skip"},
			}
			_, choice, perr := prompt.Run()
			if perr != nil {
				// Non-interactive or prompt failed; skip gracefully
				print_status(fmt.Sprintf("Skipping freeing port %d (prompt unavailable)", port))
				continue
			}

			if strings.HasPrefix(strings.ToLower(choice), "yes") {
				if err := killProcessOnPort(port); err != nil {
					print_error(fmt.Sprintf("Failed to free port %d: %v", port, err))
				} else {
					print_success(fmt.Sprintf("Freed port %d", port))
				}
			} else {
				print_status(fmt.Sprintf("Skipped freeing port %d", port))
			}
		} else {
			listener.Close()
			print_status(fmt.Sprintf("Port %d is available", port))
		}
	}

	return nil
}

// killProcessOnPort finds and terminates processes listening on the given TCP port.
func killProcessOnPort(port int) error {
	pids, err := pidsForPort(port)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return fmt.Errorf("no process found on port %d", port)
	}

	// Try graceful termination first
	for _, pid := range pids {
		_ = exec.Command("kill", "-TERM", pid).Run()
	}

	// Small delay to allow shutdown
	time.Sleep(500 * time.Millisecond)

	// Verify and force kill if still present
	remaining, _ := pidsForPort(port)
	for _, pid := range remaining {
		_ = exec.Command("kill", "-9", pid).Run()
	}

	return nil
}

// pidsForPort returns process IDs listening on the given TCP port using common system tools.
func pidsForPort(port int) ([]string, error) {
	// Prefer lsof
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf("tcp:%d", port))
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return lines, nil
	}

	// Try alternate lsof args (LISTEN only)
	cmd = exec.Command("lsof", "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-t")
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		return lines, nil
	}

	// Linux fallback: fuser
	cmd = exec.Command("fuser", fmt.Sprintf("%d/tcp", port))
	out, err = cmd.Output()
	if err == nil && len(out) > 0 {
		// fuser output is space separated PIDs
		fields := strings.Fields(strings.TrimSpace(string(out)))
		return fields, nil
	}

	return []string{}, nil
}

func stopAllServices() {
	// Determine the run mode and stop services accordingly
	cfg, err := loadCLIConfig()
	if err != nil {
		print_error("Failed to load configuration: " + err.Error())
		return
	}

	mode := cfg.Mode
	if mode == "" {
		mode = "manual"
	}

	switch mode {
	case "docker":
		// Stop Docker containers
		print_status("Stopping Docker containers...")
		if err := dockerComposeDown(); err != nil {
			print_error("Failed to stop Docker services: " + err.Error())
			return
		}
		print_success("All Docker services stopped")

	case "manual":
		// Stop managed processes
		_ = stopManagedService("console")
		_ = stopManagedService("engine")
		print_success("All services stopped")

	default:
		print_error("Unknown run mode: " + mode)
	}
}
