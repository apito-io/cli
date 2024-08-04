package main

import (
	"encoding/json"
	"fmt"
	"github.com/cavaliergopher/grab/v3"
	"github.com/manifoldco/promptui"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Magenta = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

func init() {
	createCmd.Flags().StringP("function", "f", "", "Adds a function for that project")
	createCmd.Flags().StringP("model", "m", "", "Creates a model in the project")
	createCmd.Flags().StringP("name", "n", "", "Name of the function or model or project")
}

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Create a new project, function, or model",
	Long:      `Create a new project, function, or model with the specified parameters.`,
	ValidArgs: []string{"project", "function", "model"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {

		actionName := args[0] // take only one and should be one

		projectName, _ := cmd.Flags().GetString("name")
		if projectName == "" {
			fmt.Println("Error: project name is required")
			return
		}

		switch actionName {
		case "project":
			createProject(projectName)
		case "function":
			functionName, _ := cmd.Flags().GetString(actionName)
			createFunction(projectName, functionName)
		case "model":
			modelName, _ := cmd.Flags().GetString(actionName)
			createModel(projectName, modelName)
		default:
			fmt.Println("Invalid create option. Use 'project', 'function', or 'model'.")
		}
	},
}

func createProject(project string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	projectDir := filepath.Join(homeDir, ".apito", project)

	if _, err = os.Stat(projectDir); err == nil {
		// Create the database file
		fmt.Println(Red + fmt.Sprintf("A project with the name %s already exists in %s\nPlesea Choose a different name", project, projectDir) + Reset)
		return
	}

	if err := os.MkdirAll(projectDir, 0755); err != nil {
		fmt.Println("Error creating project directory:", err)
		return
	}

	// Prompt for project description
	prompt := promptui.Prompt{
		Label: "Project Full Name",
	}
	projectFullName, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	/*	var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingLeft(4).
		Width(22)

	fmt.Println(style.Render(fmt.Sprintf("Project, %s", projectFullName)))*/

	fmt.Println(Blue + fmt.Sprintf(`Project %s needs a System database which will be used to store your login details, project schema information,`, projectFullName) + Reset)
	fmt.Println(Blue + `cloud functions, secret keys and many more system related information. Please Choose a type of system database.` + Reset)
	fmt.Println(`To get started quickly choose 'storageDb' which is a BadgerDB powered database.`)
	fmt.Println(Yellow + `Note : storageDB is not recommended for production use.` + Reset)

	// Prompt for database selection
	dbPrompt := promptui.Select{
		Label: "Select Apito System Database",
		Items: []string{"postgres", "mysql", "storageDb"},
	}
	_, db, err := dbPrompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	// Collect additional database details if necessary
	config := map[string]string{
		"PROJECT_ID":   project,
		"PROJECT_NAME": projectFullName,
		"DB_ENGINE":    db,
	}

	switch db {
	case "storageDb":
		fmt.Println(Green + fmt.Sprintf(`A local database has been created in %s/db`, projectDir) + Reset)
	case "postgres", "mysql":
		prompt := promptui.Prompt{Label: "Database Host"}
		dbHost, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["SYSTEM_DB_HOST"] = dbHost

		prompt = promptui.Prompt{Label: "Database Port"}
		dbPort, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["SYSTEM_DB_PORT"] = dbPort

		prompt = promptui.Prompt{Label: "Database User"}
		dbUser, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["SYSTEM_DB_USER"] = dbUser

		prompt = promptui.Prompt{Label: "Database Password", Mask: '*'}
		dbPass, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["SYSTEM_DB_PASS"] = dbPass

		prompt = promptui.Prompt{Label: "Database Name"}
		dbName, err := prompt.Run()
		if err != nil {
			fmt.Println("Prompt failed:", err)
			return
		}
		config["SYSTEM_DB_NAME"] = dbName
	}

	configFile := filepath.Join(projectDir, ".config")
	if err := saveConfig(configFile, config); err != nil {
		fmt.Println("Error saving config file:", err)
		return
	}

	// Get the latest release tag from GitHub API
	releaseTag, err := getLatestReleaseTag()
	if err != nil {
		fmt.Println("error fetching latest release tag: %w", err)
		return
	}

	// Detect runtime environment and download the appropriate asset
	if err := downloadAndExtractEngine(project, releaseTag, projectDir); err != nil {
		fmt.Println("Error downloading and extracting binary:", err)
		return
	}

	fmt.Println(Green + "Project created successfully!" + Reset)
	fmt.Println(Blue + `To run the project, run the following command` + Reset)
	fmt.Println(Red + fmt.Sprintf(`> apito run -p %s`, project) + Reset)
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

	// start UI loop
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()

Loop:
	for {
		select {
		case <-t.C:
			fmt.Printf("  transferred %v / %v bytes (%.2f%%)\n",
				resp.BytesComplete(),
				resp.Size,
				100*resp.Progress())

		case <-resp.Done:
			// download is complete
			break Loop
		}
	}

	// check for errors
	if err := resp.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
		return err
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
	err = os.Rename(filepath.Join(destDir, binaryName), filepath.Join(destDir, projectName))
	if err != nil {
		return fmt.Errorf("error renaming binary: %w", err)
	}

	fmt.Println("Engine binary extracted to:", filepath.Join(destDir, projectName))
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
