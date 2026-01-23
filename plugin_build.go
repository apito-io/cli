package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Build-related commands and functionality

var pluginBuildCmd = &cobra.Command{
	Use:   "build [plugin-directory]",
	Short: "Build a plugin based on its configuration",
	Long:  `Build a plugin automatically based on its language configuration in config.yml`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Check for --dir flag first, then positional argument, then default to current dir
		pluginDir, _ := cmd.Flags().GetString("dir")
		if pluginDir == "" && len(args) > 0 {
			pluginDir = args[0]
		}
		if pluginDir == "" {
			pluginDir = "."
		}
		buildPlugin(pluginDir, cmd)
	},
}

// BuildMethod represents different build approaches
type BuildMethod string

const (
	BuildMethodSystem BuildMethod = "system"
	BuildMethodDocker BuildMethod = "docker"
)

// LanguageRuntime contains information about a language runtime
type LanguageRuntime struct {
	Name           string
	SystemCheck    string   // Command to check if runtime exists
	SystemBuild    []string // Commands to build with system runtime
	DockerImage    string   // Docker image for building
	DockerBuild    []string // Commands to build with Docker
	OutputPath     string   // Expected output file path
	RequiredFiles  []string // Files that must exist for this language
	PostBuildSteps []string // Additional steps after build
}

// GoBuildType represents different Go build types
type GoBuildType string

const (
	GoBuildDebug      GoBuildType = "debug"
	GoBuildDevelop    GoBuildType = "develop"
	GoBuildProduction GoBuildType = "production"
)

// Language runtime definitions based on actual Makefile patterns
var languageRuntimes = map[string]LanguageRuntime{
	"go": {
		Name:        "Go",
		SystemCheck: "go version",
		SystemBuild: []string{
			"go mod tidy",
			"go build -o {binary_name} .",
		},
		DockerImage: "golang:1.25.4-alpine",
		DockerBuild: []string{
			"docker run --rm -v {plugin_dir}:/workspace -w /workspace golang:1.25.4-alpine sh -c \"go mod tidy && go build -o {binary_name} .\"",
		},
		OutputPath:     "./{binary_name}",
		RequiredFiles:  []string{"main.go", "go.mod"},
		PostBuildSteps: []string{"chmod +x {binary_name}"},
	},
	"js": {
		Name:        "JavaScript/Node.js",
		SystemCheck: "node --version",
		SystemBuild: []string{
			"npm install",
			"node --check index.js",
		},
		DockerImage: "node:18-alpine",
		DockerBuild: []string{
			"docker run --rm -v {plugin_dir}:/workspace -w /workspace node:18-alpine sh -c \"npm install && node --check index.js\"",
		},
		OutputPath:    "./index.js",
		RequiredFiles: []string{"index.js", "package.json"},
	},
	"javascript": {
		Name:        "JavaScript/Node.js",
		SystemCheck: "node --version",
		SystemBuild: []string{
			"npm install",
			"node --check index.js",
		},
		DockerImage: "node:18-alpine",
		DockerBuild: []string{
			"docker run --rm -v {plugin_dir}:/workspace -w /workspace node:18-alpine sh -c \"npm install && node --check index.js\"",
		},
		OutputPath:    "./index.js",
		RequiredFiles: []string{"index.js", "package.json"},
	},
	"python": {
		Name:        "Python",
		SystemCheck: "python3 --version",
		SystemBuild: []string{
			"pip3 install -r requirements.txt || echo 'No requirements.txt found'",
			"python3 -m py_compile main.py",
		},
		DockerImage: "python:3.11-alpine",
		DockerBuild: []string{
			"docker run --rm -v {plugin_dir}:/workspace -w /workspace python:3.11-alpine sh -c \"pip3 install -r requirements.txt || echo 'No requirements.txt' && python3 -m py_compile main.py\"",
		},
		OutputPath:    "./main.py",
		RequiredFiles: []string{"main.py"},
	},
}

func buildPlugin(pluginDir string, cmd *cobra.Command) {
	print_step(fmt.Sprintf("🔨 Building Plugin in: %s", pluginDir))

	// Read plugin configuration
	config, err := readPluginConfig(pluginDir)
	if err != nil {
		print_error("Failed to read plugin configuration: " + err.Error())
		return
	}

	language := strings.ToLower(config.Plugin.Language)
	if language == "" {
		print_error("Plugin language not specified in config.yml")
		return
	}

	// Get language runtime information
	runtime, exists := languageRuntimes[language]
	if !exists {
		print_error(fmt.Sprintf("Unsupported language: %s", language))
		print_status("Supported languages: go, js, python")
		return
	}

	print_status(fmt.Sprintf("Detected language: %s", runtime.Name))

	// Validate required files
	if !validateRequiredFiles(pluginDir, runtime.RequiredFiles) {
		return
	}

	// Check if system runtime is available
	systemAvailable := checkSystemRuntime(runtime.SystemCheck)

	// Determine build method
	buildMethod := determineBuildMethod(cmd, runtime.Name, systemAvailable)

	// Execute build
	if err := executeBuild(pluginDir, config, runtime, buildMethod, cmd); err != nil {
		print_error("Build failed: " + err.Error())
		return
	}

	// Validate build output
	if err := validateBuildOutput(pluginDir, config, runtime); err != nil {
		print_error("Build validation failed: " + err.Error())
		return
	}

	print_success("✅ Plugin built successfully!")
	print_status(fmt.Sprintf("Binary location: %s", getBinaryPath(pluginDir, config.Plugin.BinaryPath)))

	// Check if UI build is needed
	if shouldBuildUI(config) {
		if err := buildUI(pluginDir, config); err != nil {
			print_warning(fmt.Sprintf("UI build failed: %v", err))
			print_status("You can build the UI manually later")
		}
	}

	// Show next steps
	showNextSteps(config.Plugin.ID)
}

func readPluginConfig(pluginDir string) (*PluginConfig, error) {
	configPath := filepath.Join(pluginDir, "config.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("config.yml not found in %s", pluginDir)
	}

	var config PluginConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid config.yml format: %w", err)
	}

	return &config, nil
}

func validateRequiredFiles(pluginDir string, requiredFiles []string) bool {
	for _, file := range requiredFiles {
		filePath := filepath.Join(pluginDir, file)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			print_error(fmt.Sprintf("Required file not found: %s", file))
			return false
		}
	}
	return true
}

func checkSystemRuntime(checkCommand string) bool {
	if checkCommand == "" {
		return false
	}

	parts := strings.Fields(checkCommand)
	cmd := exec.Command(parts[0], parts[1:]...)

	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

// Validation helper functions for flags

func validateBuildMethod(value string) (BuildMethod, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "system":
		return BuildMethodSystem, nil
	case "docker":
		return BuildMethodDocker, nil
	default:
		return "", fmt.Errorf("invalid build method: %s (must be 'system' or 'docker')", value)
	}
}

func validatePlatformArch(os, arch string) (PlatformTarget, error) {
	normalizedOS := strings.ToLower(strings.TrimSpace(os))
	normalizedArch := strings.ToLower(strings.TrimSpace(arch))

	// If arch is not provided, try to find a default or use host arch
	if normalizedArch == "" {
		normalizedArch = runtime.GOARCH
	}

	// Search for matching platform
	for _, platform := range supportedPlatforms {
		if strings.ToLower(platform.OS) == normalizedOS && strings.ToLower(platform.Arch) == normalizedArch {
			return platform, nil
		}
	}

	// If not found, try to construct a basic platform target
	// This allows for custom combinations that might not be in supportedPlatforms
	if normalizedOS == "linux" || normalizedOS == "darwin" || normalizedOS == "windows" {
		if normalizedArch == "amd64" || normalizedArch == "arm64" {
			platformStr := ""
			if normalizedOS == "linux" {
				platformStr = fmt.Sprintf("linux/%s", normalizedArch)
			}
			// Format OS name for display
			osDisplay := normalizedOS
			if normalizedOS == "darwin" {
				osDisplay = "macOS"
			} else if normalizedOS == "linux" {
				osDisplay = "Linux"
			} else if normalizedOS == "windows" {
				osDisplay = "Windows"
			}
			return PlatformTarget{
				OS:       normalizedOS,
				Arch:     normalizedArch,
				Display:  fmt.Sprintf("%s %s", osDisplay, strings.ToUpper(normalizedArch)),
				Platform: platformStr,
			}, nil
		}
	}

	return PlatformTarget{}, fmt.Errorf("invalid platform/arch combination: %s/%s (supported: linux/darwin/windows with amd64/arm64)", os, arch)
}

func validateGoBuildType(value string) (GoBuildType, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "debug":
		return GoBuildDebug, nil
	case "develop", "development":
		return GoBuildDevelop, nil
	case "production":
		return GoBuildProduction, nil
	default:
		return "", fmt.Errorf("invalid build type: %s (must be 'debug', 'develop', or 'production')", value)
	}
}

func determineBuildMethod(cmd *cobra.Command, languageName string, systemAvailable bool) BuildMethod {
	// Check for --build flag first
	buildFlag, _ := cmd.Flags().GetString("build")
	if buildFlag != "" {
		method, err := validateBuildMethod(buildFlag)
		if err != nil {
			print_error(err.Error())
			print_warning("Falling back to interactive selection")
		} else {
			// Validate that system build is available if requested
			if method == BuildMethodSystem && !systemAvailable {
				print_warning(fmt.Sprintf("%s runtime not found on system, but --build=system was specified", languageName))
				print_warning("Falling back to Docker build")
				return BuildMethodDocker
			}
			return method
		}
	}

	// If no flag or validation failed, use interactive prompt
	if !systemAvailable {
		print_status(fmt.Sprintf("%s runtime not found on system, using Docker", languageName))
		return BuildMethodDocker
	}

	print_status(fmt.Sprintf("%s runtime detected on system", languageName))

	// Ask user for preference
	prompt := promptui.Select{
		Label: fmt.Sprintf("Choose build method for %s", languageName),
		Items: []string{
			fmt.Sprintf("System %s (faster)", languageName),
			"Docker (consistent, isolated)",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		print_warning("Selection failed, defaulting to Docker build")
		return BuildMethodDocker
	}

	if idx == 0 {
		return BuildMethodSystem
	}
	return BuildMethodDocker
}

// PlatformTarget represents a target platform for cross-compilation
type PlatformTarget struct {
	OS       string
	Arch     string
	Display  string
	Platform string // Docker platform string
}

var supportedPlatforms = []PlatformTarget{
	{OS: "linux", Arch: "amd64", Display: "Linux AMD64", Platform: "linux/amd64"},
	{OS: "linux", Arch: "arm64", Display: "Linux ARM64", Platform: "linux/arm64"},
	{OS: "darwin", Arch: "amd64", Display: "macOS AMD64 (Intel)", Platform: "linux/amd64"},         // Use linux for cross-compilation
	{OS: "darwin", Arch: "arm64", Display: "macOS ARM64 (Apple Silicon)", Platform: "linux/arm64"}, // Use linux for cross-compilation
	{OS: "windows", Arch: "amd64", Display: "Windows AMD64", Platform: "linux/amd64"},              // Use linux for cross-compilation
	{OS: runtime.GOOS, Arch: runtime.GOARCH, Display: fmt.Sprintf("Host OS (%s/%s)", runtime.GOOS, runtime.GOARCH), Platform: ""},
}

func selectTargetPlatform(cmd *cobra.Command) PlatformTarget {
	// Check for --platform and --arch flags first
	platformFlag, _ := cmd.Flags().GetString("platform")
	archFlag, _ := cmd.Flags().GetString("arch")

	if platformFlag != "" {
		platform, err := validatePlatformArch(platformFlag, archFlag)
		if err != nil {
			print_error(err.Error())
			print_warning("Falling back to interactive selection")
		} else {
			return platform
		}
	}

	// If no flags or validation failed, use interactive prompt
	var items []string
	for _, platform := range supportedPlatforms {
		items = append(items, platform.Display)
	}

	prompt := promptui.Select{
		Label: "Choose target platform",
		Items: items,
	}

	idx, _, err := prompt.Run()
	if err != nil {
		print_error("Platform selection failed: " + err.Error())
		// Return host platform as fallback
		return PlatformTarget{
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Display:  fmt.Sprintf("Host OS (%s/%s)", runtime.GOOS, runtime.GOARCH),
			Platform: "",
		}
	}

	return supportedPlatforms[idx]
}

func determineGoBuildType(cmd *cobra.Command, systemAvailable bool) GoBuildType {
	// Check for --type flag first
	typeFlag, _ := cmd.Flags().GetString("type")
	if typeFlag != "" {
		buildType, err := validateGoBuildType(typeFlag)
		if err != nil {
			print_error(err.Error())
			print_warning("Falling back to interactive selection")
		} else {
			return buildType
		}
	}

	// If no flag or validation failed, use interactive prompt
	prompt := promptui.Select{
		Label: "Choose Go build type",
		Items: []string{
			"Debug (with debug symbols, gcflags=\"all=-N -l\")",
			"Development (basic build, cross-platform support)",
			"Production (static binary, CGO_ENABLED=0, optimized)",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		print_warning("Selection failed, defaulting to Development build")
		return GoBuildDevelop
	}

	switch idx {
	case 0:
		return GoBuildDebug
	case 1:
		return GoBuildDevelop
	case 2:
		return GoBuildProduction
	default:
		return GoBuildDevelop
	}
}

func executeBuild(pluginDir string, config *PluginConfig, runtime LanguageRuntime, method BuildMethod, cmd *cobra.Command) error {
	language := strings.ToLower(config.Plugin.Language)
	binaryName := config.Plugin.BinaryPath
	if binaryName == "" {
		binaryName = config.Plugin.ID
	}

	// For Go, use special build handling
	if language == "go" {
		buildType := determineGoBuildType(cmd, method == BuildMethodSystem)
		absPluginDir, _ := filepath.Abs(pluginDir)

		if err := executeGoBuild(absPluginDir, binaryName, buildType, method, cmd); err != nil {
			return err
		}

		// Execute post-build steps for Go
		for _, step := range runtime.PostBuildSteps {
			step = strings.ReplaceAll(step, "{binary_name}", binaryName)
			if err := executeCommand(step, pluginDir); err != nil {
				print_warning(fmt.Sprintf("Post-build step failed: %s", step))
			}
		}

		return nil
	}

	// For other languages, use standard commands
	var commands []string
	if method == BuildMethodSystem {
		print_status("Building with system " + runtime.Name)
		commands = runtime.SystemBuild
	} else {
		print_status("Building with Docker")
		if !checkDockerAvailable() {
			return fmt.Errorf("docker is not available, please install Docker or use system build")
		}
		commands = runtime.DockerBuild
	}

	// Replace placeholders in commands
	absPluginDir, _ := filepath.Abs(pluginDir)

	for i, cmd := range commands {
		cmd = strings.ReplaceAll(cmd, "{binary_name}", binaryName)
		cmd = strings.ReplaceAll(cmd, "{plugin_dir}", absPluginDir)
		commands[i] = cmd
	}

	// Execute commands
	for _, cmdStr := range commands {
		print_status("Running: " + cmdStr)

		if err := executeCommand(cmdStr, pluginDir); err != nil {
			return fmt.Errorf("command failed: %s - %w", cmdStr, err)
		}
	}

	// Execute post-build steps
	for _, step := range runtime.PostBuildSteps {
		step = strings.ReplaceAll(step, "{binary_name}", binaryName)
		if err := executeCommand(step, pluginDir); err != nil {
			print_warning(fmt.Sprintf("Post-build step failed: %s", step))
		}
	}

	return nil
}

func executeGoBuild(pluginDir, binaryName string, buildType GoBuildType, method BuildMethod, cmd *cobra.Command) error {
	// Select target platform for cross-compilation
	targetPlatform := selectTargetPlatform(cmd)
	print_status(fmt.Sprintf("🎯 Target Platform: %s", targetPlatform.Display))

	return executeGoBuildWithPlatform(pluginDir, binaryName, buildType, method, targetPlatform)
}

func executeGoBuildWithPlatform(pluginDir, binaryName string, buildType GoBuildType, method BuildMethod, targetPlatform PlatformTarget) error {
	if method == BuildMethodDocker {
		// Handle Docker builds with proper argument construction
		var shellCmd string
		switch buildType {
		case GoBuildDebug:
			shellCmd = fmt.Sprintf("go mod tidy && go build -gcflags='all=-N -l' -o %s .", binaryName)
			print_status("Building Go plugin with debug symbols (Docker)")
		case GoBuildDevelop:
			// Build for the selected target platform
			shellCmd = fmt.Sprintf("go mod tidy && GOOS=%s GOARCH=%s go build -o %s .", targetPlatform.OS, targetPlatform.Arch, binaryName)
			print_status(fmt.Sprintf("Building Go plugin for development (Docker) - target: %s/%s", targetPlatform.OS, targetPlatform.Arch))
		case GoBuildProduction:
			shellCmd = fmt.Sprintf("go mod tidy && CGO_ENABLED=0 go build -ldflags='-s' -a -o %s .", binaryName)
			print_status("Building Go plugin for production (Docker, static binary)")
		}

		// Build Docker command with platform support
		var cmd *exec.Cmd
		if targetPlatform.Platform != "" {
			print_status(fmt.Sprintf("Running: docker run --platform %s --rm -v %s:/workspace -w /workspace golang:1.25.4-alpine sh -c '%s'",
				targetPlatform.Platform, pluginDir, shellCmd))
			cmd = exec.Command("docker", "run", "--platform", targetPlatform.Platform, "--rm",
				"-v", pluginDir+":/workspace",
				"-w", "/workspace",
				"golang:1.25.4-alpine",
				"sh", "-c", shellCmd)
		} else {
			print_status("Running: docker run --rm -v " + pluginDir + ":/workspace -w /workspace golang:1.25.4-alpine sh -c '" + shellCmd + "'")
			cmd = exec.Command("docker", "run", "--rm",
				"-v", pluginDir+":/workspace",
				"-w", "/workspace",
				"golang:1.25.4-alpine",
				"sh", "-c", shellCmd)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Handle system builds with proper argument parsing
	print_status("Running: go mod tidy")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = pluginDir
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	var args []string
	var statusMsg string

	switch buildType {
	case GoBuildDebug:
		args = []string{"build", "-gcflags=all=-N -l", "-o", binaryName, "."}
		statusMsg = "go build -gcflags=all=-N -l -o " + binaryName + " ."
		print_status("Building Go plugin with debug symbols (system)")
	case GoBuildDevelop:
		args = []string{"build", "-o", binaryName, "."}
		statusMsg = "go build -o " + binaryName + " ."
		print_status("Building Go plugin for development (system)")
	case GoBuildProduction:
		args = []string{"build", "-ldflags", "-s", "-a", "-o", binaryName, "."}
		statusMsg = "CGO_ENABLED=0 go build -ldflags -s -a -o " + binaryName + " ."
		print_status("Building Go plugin for production (system, static binary)")
		// Set CGO_ENABLED=0 environment variable
		os.Setenv("CGO_ENABLED", "0")
		defer os.Unsetenv("CGO_ENABLED")
	}

	print_status("Running: " + statusMsg)
	cmd := exec.Command("go", args...)
	cmd.Dir = pluginDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set target platform environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOOS=%s", targetPlatform.OS))
	cmd.Env = append(cmd.Env, fmt.Sprintf("GOARCH=%s", targetPlatform.Arch))

	print_status(fmt.Sprintf("🎯 Building for: GOOS=%s GOARCH=%s", targetPlatform.OS, targetPlatform.Arch))
	return cmd.Run()
}

func getGoBuildCommands(buildType GoBuildType, method BuildMethod) []string {
	// This function is no longer used for Go builds, but kept for compatibility
	// Go builds now use executeGoBuild() directly
	return []string{}
}

func executeCommand(cmdStr, workDir string) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil
	}

	var cmd *exec.Cmd
	if parts[0] == "docker" {
		// For Docker commands, run from current directory but mount the work directory
		cmd = exec.Command(parts[0], parts[1:]...)
	} else {
		// For regular commands, run in the work directory
		cmd = exec.Command(parts[0], parts[1:]...)
		cmd.Dir = workDir
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func validateBuildOutput(pluginDir string, config *PluginConfig, runtime LanguageRuntime) error {
	binaryName := config.Plugin.BinaryPath
	if binaryName == "" {
		binaryName = config.Plugin.ID
	}

	// Check for expected output
	outputPath := strings.ReplaceAll(runtime.OutputPath, "{binary_name}", binaryName)

	// Handle multiple possible paths (for JS)
	if strings.Contains(outputPath, "||") {
		paths := strings.Split(outputPath, "||")
		for _, path := range paths {
			path = strings.TrimSpace(path)
			fullPath := filepath.Join(pluginDir, path)
			if _, err := os.Stat(fullPath); err == nil {
				return nil // Found at least one valid output
			}
		}
		return fmt.Errorf("no build output found in any expected location")
	}

	// Single path check
	fullPath := filepath.Join(pluginDir, outputPath)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("expected output not found: %s", outputPath)
	}

	return nil
}

func checkDockerAvailable() bool {
	cmd := exec.Command("docker", "--version")
	return cmd.Run() == nil
}

func getBinaryPath(pluginDir, binaryName string) string {
	if binaryName == "" {
		return "Not specified"
	}
	return filepath.Join(pluginDir, binaryName)
}

func showNextSteps(pluginID string) {
	print_step("🚀 Next Steps")
	print_status("1. Test your plugin locally (if applicable)")
	print_status("2. Deploy to server: apito plugin deploy")
	print_status(fmt.Sprintf("3. Check status: apito plugin status %s", pluginID))
}

// Build environment information
func showBuildEnvironment() {
	print_step("🔍 Build Environment Information")

	// Check each language runtime
	for lang, runtime := range languageRuntimes {
		available := checkSystemRuntime(runtime.SystemCheck)
		status := "❌ Not Available"
		if available {
			status = "✅ Available"
		}
		print_status(fmt.Sprintf("%s (%s): %s", runtime.Name, lang, status))
	}

	// Check Docker
	dockerAvailable := checkDockerAvailable()
	dockerStatus := "❌ Not Available"
	if dockerAvailable {
		dockerStatus = "✅ Available"
	}
	print_status(fmt.Sprintf("Docker: %s", dockerStatus))

	print_status(fmt.Sprintf("OS: %s", runtime.GOOS))
	print_status(fmt.Sprintf("Architecture: %s", runtime.GOARCH))
}

var pluginEnvCmd = &cobra.Command{
	Use:   "env",
	Short: "Show build environment information",
	Long:  `Show available language runtimes and build tools`,
	Run: func(cmd *cobra.Command, args []string) {
		showBuildEnvironment()
	},
}

func init() {
	// Add --dir flag to build command
	pluginBuildCmd.Flags().StringP("dir", "d", "", "Plugin directory (alternative to positional argument)")

	// Add build method flag
	pluginBuildCmd.Flags().StringP("build", "b", "", "Build method: system or docker (skips interactive prompt)")

	// Add platform flags
	pluginBuildCmd.Flags().StringP("platform", "p", "", "Target OS: linux, darwin, or windows (skips interactive prompt)")
	pluginBuildCmd.Flags().String("arch", "", "Target architecture: amd64 or arm64 (skips interactive prompt)")

	// Add build type flag (for Go plugins)
	pluginBuildCmd.Flags().StringP("type", "t", "", "Go build type: debug, develop, or production (skips interactive prompt)")

	// Add build command to plugin commands
	pluginCmd.AddCommand(pluginBuildCmd)
	pluginCmd.AddCommand(pluginEnvCmd)
}

// UI Build Functions

// shouldBuildUI checks if UI build is needed based on config
func shouldBuildUI(config *PluginConfig) bool {
	if config.Plugin.UIConfig == nil {
		return false
	}
	if !config.Plugin.UIConfig.Enable {
		return false
	}
	if config.Plugin.UIConfig.DistPath == "" {
		return false
	}
	return true
}

// getUIFolder extracts the UI folder path from dist_path
// e.g., "ui/dist/index.umd.js" -> "ui"
func getUIFolder(distPath string) string {
	parts := strings.Split(distPath, string(filepath.Separator))
	if len(parts) > 0 {
		return parts[0]
	}
	return "ui" // default
}

// PackageJSON represents the structure of package.json
type PackageJSON struct {
	Scripts map[string]string `json:"scripts"`
}

// checkUIBuildRequirements checks if UI can be built
func checkUIBuildRequirements(pluginDir string, uiFolder string) (bool, string, error) {
	uiDir := filepath.Join(pluginDir, uiFolder)
	packageJSONPath := filepath.Join(uiDir, "package.json")

	// Check if package.json exists
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return false, "", fmt.Errorf("package.json not found in %s", uiFolder)
	}

	// Read and parse package.json
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkgJSON PackageJSON
	if err := json.Unmarshal(data, &pkgJSON); err != nil {
		return false, "", fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Check if build script exists
	if pkgJSON.Scripts == nil {
		return false, "", fmt.Errorf("no scripts found in package.json")
	}

	buildScript, hasBuild := pkgJSON.Scripts["build"]
	if !hasBuild {
		return false, "", fmt.Errorf("no 'build' script found in package.json")
	}

	return true, buildScript, nil
}

// askUserToBuildUI prompts user if they want to build UI
func askUserToBuildUI() bool {
	prompt := promptui.Select{
		Label: "UI configuration detected. Build UI as well?",
		Items: []string{"Yes", "No"},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false
	}

	return idx == 0
}

// determineUIBuildMethod determines if we should use system (pnpm) or Docker
func determineUIBuildMethod() BuildMethod {
	// Check if pnpm is available
	pnpmAvailable := checkSystemRuntime("pnpm --version")

	if !pnpmAvailable {
		print_status("pnpm not found on system, will use Docker")
		return BuildMethodDocker
	}

	print_status("pnpm detected on system")

	// Ask user for preference
	prompt := promptui.Select{
		Label: "Choose UI build method",
		Items: []string{
			"System pnpm (faster)",
			"Docker (consistent, isolated)",
		},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		print_warning("Selection failed, defaulting to Docker build")
		return BuildMethodDocker
	}

	if idx == 0 {
		return BuildMethodSystem
	}
	return BuildMethodDocker
}

// buildUI handles the UI build process
func buildUI(pluginDir string, config *PluginConfig) error {
	print_step("🎨 Building Plugin UI")

	uiFolder := getUIFolder(config.Plugin.UIConfig.DistPath)
	uiDir := filepath.Join(pluginDir, uiFolder)

	// Check if UI folder exists
	if _, err := os.Stat(uiDir); os.IsNotExist(err) {
		return fmt.Errorf("UI folder '%s' not found", uiFolder)
	}

	// Check build requirements
	canBuild, buildScript, err := checkUIBuildRequirements(pluginDir, uiFolder)
	if !canBuild {
		if err != nil {
			print_warning(fmt.Sprintf("UI build requirements not met: %v", err))
		}
		return err
	}

	print_status(fmt.Sprintf("Found build script: %s", buildScript))

	// Ask user if they want to build UI
	if !askUserToBuildUI() {
		print_status("Skipping UI build")
		return nil
	}

	// Determine build method
	buildMethod := determineUIBuildMethod()

	// Execute UI build
	if err := executeUIBuild(uiDir, buildMethod); err != nil {
		return fmt.Errorf("UI build failed: %w", err)
	}

	// Validate UI build output
	expectedOutput := filepath.Join(pluginDir, config.Plugin.UIConfig.DistPath)
	if _, err := os.Stat(expectedOutput); os.IsNotExist(err) {
		return fmt.Errorf("expected UI output not found at: %s", expectedOutput)
	}

	print_success("✅ UI built successfully!")
	print_status(fmt.Sprintf("UI output location: %s", expectedOutput))

	return nil
}

// executeUIBuild executes the UI build using the selected method
func executeUIBuild(uiDir string, method BuildMethod) error {
	absUIDir, _ := filepath.Abs(uiDir)

	if method == BuildMethodDocker {
		if !checkDockerAvailable() {
			return fmt.Errorf("docker is not available, please install Docker or use system build")
		}

		print_status("Building UI with Docker (node:18-alpine)")
		// Try pnpm first, fallback to npm if pnpm fails
		// Install pnpm globally in container, then use it
		shellCmd := "npm install -g pnpm && pnpm install && pnpm build || (npm install && npm run build)"
		print_status(fmt.Sprintf("Running: docker run --rm -v %s:/workspace -w /workspace node:18-alpine sh -c '%s'", absUIDir, shellCmd))

		cmd := exec.Command("docker", "run", "--rm",
			"-v", absUIDir+":/workspace",
			"-w", "/workspace",
			"node:18-alpine",
			"sh", "-c", shellCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// System build with pnpm
	print_status("Building UI with system pnpm")
	print_status("Running: pnpm install")
	installCmd := exec.Command("pnpm", "install")
	installCmd.Dir = uiDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("pnpm install failed: %w", err)
	}

	print_status("Running: pnpm build")
	buildCmd := exec.Command("pnpm", "build")
	buildCmd.Dir = uiDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("pnpm build failed: %w", err)
	}

	return nil
}
