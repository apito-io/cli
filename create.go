package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cavaliergopher/grab/v3"
	"github.com/kyokomi/emoji/v2"
	"github.com/manifoldco/promptui"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

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

		projectName = strings.TrimSpace(projectName)

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

	/*
		var style = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingLeft(4).
		Width(22)

		fmt.Println(style.Render(fmt.Sprintf("Project, %s", projectFullName)))
	*/

	fmt.Println(Blue + fmt.Sprintf(`Project '%s' needs a System database which will be used to store your login details, project schema information,`, projectFullName) + Reset)
	fmt.Println(Blue + `cloud functions, secret keys and many more system related information. Please Choose a type of system database.` + Reset)
	fmt.Println(`To get started quickly choose 'storageDb' which is a BadgerDB powered database.`)
	fmt.Println(Yellow + `Note : storageDB is not recommended for production use.` + Reset)

	// Prompt for database selection
	dbPrompt := promptui.Select{
		Label: emoji.Sprint(":electric_plug: Select Apito System Database"),
		Items: []string{"postgres", "mysql", "storageDb"},
	}
	_, db, err := dbPrompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	if db == "storageDb" {
		db = "badger"
	}

	// Collect additional database details if necessary
	config := map[string]string{
		"ENV":              "local",
		"PROJECT_ID":       project,
		"PROJECT_NAME":     projectFullName,
		"SYSTEM_DB_ENGINE": db,
	}

	switch db {
	case "storageDb":
		fmt.Println(Green + fmt.Sprintf(`A local database has been created in %s/db`, projectDir) + Reset)
	case "postgresql", "mysql", "mariadb":
		dbConfigs := getDBConfig("SYSTEM")
		if dbConfigs == nil {
			fmt.Println("Error getting database configuration")
			return
		}
		for k, v := range dbConfigs {
			config[k] = v
		}
	}

	fmt.Println(Blue + emoji.Sprint("Project Database is the main database of your project") + Reset)
	fmt.Println(Yellow + `Note : firestore/firebase support is still in alpha. Check progess of the driver here: https://github.com/orgs/apito-io/projects/5` + Reset)

	// Prompt for database selection
	dbPrompt = promptui.Select{
		Label: emoji.Sprint(":rocket: Choose Apito Project Database"),
		Items: []string{"postgres", "mysql", "mariadb", "firestore"},
	}
	_, db, err = dbPrompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return
	}

	config["PROJECT_DB_ENGINE"] = db

	switch db {
	case "firestore":
		fmt.Println(Red + `Support for Firestore is still in alpha. Check progess of the driver here: https://github.com/orgs/apito-io/projects/5` + Reset)
	case "postgres", "mysql":
		dbConfigs := getDBConfig("PROJECT")
		if dbConfigs == nil {
			fmt.Println("Error getting database configuration")
			return
		}
		for k, v := range dbConfigs {
			config[k] = v
		}
	}

	if err := saveConfig(projectDir, config); err != nil {
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
	fmt.Println(Green + fmt.Sprintf(`> apito run -p %s`, project) + Reset)
}

func getDBConfig(_prefix string) map[string]string {
	prompt := promptui.Prompt{Label: "Database Host"}
	dbHost, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}

	config := map[string]string{
		_prefix + "_DB_HOST": dbHost,
	}

	prompt = promptui.Prompt{Label: "Database Port"}
	dbPort, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}
	config[_prefix+"_DB_PORT"] = dbPort

	prompt = promptui.Prompt{Label: "Database User"}
	dbUser, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}
	config[_prefix+"_DB_USER"] = dbUser

	prompt = promptui.Prompt{Label: "Database Password", Mask: '*'}
	dbPass, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}
	config[_prefix+"_DB_PASS"] = dbPass

	prompt = promptui.Prompt{Label: "Database Name"}
	dbName, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}
	config[_prefix+"_DB_NAME"] = dbName

	return config
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
