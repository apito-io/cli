package main

import (
	"encoding/json"
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"github.com/mholt/archiver/v3"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new project, function, or model",
	Long:  `Create a new project, function, or model with the specified parameters.`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			fmt.Println("Error: --project is required")
			return
		}

		if len(args) == 0 {
			fmt.Println("Error: Please specify what to create (project, function, or model)")
			return
		}

		switch args[0] {
		case "project":
			createProject(project)
		case "function":
			functionName, _ := cmd.Flags().GetString("name")
			createFunction(project, functionName)
		case "model":
			modelName, _ := cmd.Flags().GetString("name")
			createModel(project, modelName)
		default:
			fmt.Println("Invalid create option. Use 'project', 'function', or 'model'.")
		}
	},
}

func init() {
	createCmd.Flags().StringP("project", "p", "", "Project name")
	createCmd.Flags().StringP("name", "n", "", "Name for function or model")
}

func createProject(project string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	projectDir := filepath.Join(homeDir, ".apito", project)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Println("Error creating project directory:", err)
		return
	}

	// Prompt for project description
	prompt := promptui.Prompt{
		Label: "Project Description",
	}
	description, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	// Prompt for database selection
	dbPrompt := promptui.Select{
		Label: "Select Database",
		Items: []string{"memorydb", "postgres", "mysql"},
	}
	_, db, err := dbPrompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	// Collect additional database details if necessary
	config := map[string]string{
		"PROJECT_NAME": project,
		"PROJECT_DESC": description,
		"DB_ENGINE":    db,
	}

	if db == "postgres" || db == "mysql" {
		prompt := promptui.Prompt{Label: "Database Host"}
		dbHost, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["DB_HOST"] = dbHost

		prompt = promptui.Prompt{Label: "Database Port"}
		dbPort, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["DB_PORT"] = dbPort

		prompt = promptui.Prompt{Label: "Database User"}
		dbUser, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["DB_USER"] = dbUser

		prompt = promptui.Prompt{Label: "Database Password", Mask: '*'}
		dbPass, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["DB_PASS"] = dbPass

		prompt = promptui.Prompt{Label: "Database Name"}
		dbName, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["DB_NAME"] = dbName
	}

	configFile := filepath.Join(projectDir, ".config")
	if err := saveConfig(configFile, config); err != nil {
		fmt.Println("Error saving config file:", err)
		return
	}

	// Detect runtime environment and download the appropriate asset
	if err := downloadAndExtractEngine(projectDir); err != nil {
		fmt.Println("Error downloading and extracting binary:", err)
		return
	}

	fmt.Println("Project created:", project)
	fmt.Println("Description:", description)
	fmt.Println("Database:", db)
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

func downloadAndExtractEngine(destDir string) error {

	// Get the latest release tag from GitHub API
	releaseTag, err := getLatestReleaseTag()
	if err != nil {
		return fmt.Errorf("error fetching latest release tag: %w", err)
	}

	baseURL := fmt.Sprintf("https://github.com/apito-io/engine/releases/download/%s/", releaseTag)
	var assetURL string

	switch runtime.GOOS {
	case "linux":
		assetURL = baseURL + "engine-linux-amd64.zip"
	case "darwin":
		assetURL = baseURL + "engine-darwin-amd64.zip"
	case "windows":
		assetURL = baseURL + "engine-windows-amd64.zip"
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	fmt.Println("Downloading engine from:", assetURL)

	// Download the file
	resp, err := grab.Get(destDir, assetURL)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}

	fmt.Println("Downloaded file saved to:", resp.Filename)

	// Unzip the file
	err = archiver.Unarchive(resp.Filename, destDir)
	if err != nil {
		return fmt.Errorf("error extracting file: %w", err)
	}

	// Rename the binary to "engine"
	binaryName := "engine"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	err = os.Rename(filepath.Join(destDir, binaryName), filepath.Join(destDir, "engine"))
	if err != nil {
		return fmt.Errorf("error renaming binary: %w", err)
	}

	fmt.Println("Engine binary extracted to:", filepath.Join(destDir, "engine"))
	return nil
}

func saveConfig(configFile string, config map[string]string) error {
	f, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("error creating config file: %w", err)
	}
	defer f.Close()

	for key, value := range config {
		_, err := f.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		if err != nil {
			return fmt.Errorf("error writing to config file: %w", err)
		}
	}

	return nil
}

func createFunction(project, functionName string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	functionDir := filepath.Join(homeDir, ".apito", project, "functions", functionName)
	if err := os.MkdirAll(functionDir, 0755); err != nil {
		fmt.Println("Error creating function directory:", err)
		return
	}
	fmt.Println("Function created:", functionName)
}

func createModel(project, modelName string) {
	fmt.Println("Creating model:", modelName)
	// Here you can add your GraphQL request logic
}
