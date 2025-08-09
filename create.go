package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func init() {
	createCmd.Flags().StringP("project", "p", "", "project name")
	createCmd.Flags().StringP("name", "n", "", "name of the project, function, or model")
}

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Create a new project, function, or model",
	Long:      `Create a new project, function, or model with the specified parameters.`,
	ValidArgs: []string{"project", "function", "model"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		actionName := args[0]

		switch actionName {
		case "project":
			createProject(cmd)
		case "function":
			createFunction(cmd)
		case "model":
			createModel(cmd)
		default:
			print_error("Invalid create option. Use 'project', 'function', or 'model'.")
		}
	},
}

type CreateProjectRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	DatabaseType string `json:"database_type"`
}

type CreateProjectResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"data,omitempty"`
}

func createProject(cmd *cobra.Command) {
	print_step("🚀 Creating New Apito Project")
	fmt.Println()

	// Get project name from flags or prompt
	projectName, _ := cmd.Flags().GetString("project")
	if projectName == "" {
		projectName, _ = cmd.Flags().GetString("name")
	}

	if projectName == "" {
		prompt := promptui.Prompt{
			Label: "Project Name",
			Validate: func(input string) error {
				if len(strings.TrimSpace(input)) == 0 {
					return fmt.Errorf("project name cannot be empty")
				}
				return nil
			},
		}
		name, err := prompt.Run()
		if err != nil {
			print_error("Failed to get project name: " + err.Error())
			return
		}
		projectName = strings.TrimSpace(name)
	}

	print_status("Project name: " + projectName)
	fmt.Println()

	// Get project description
	prompt := promptui.Prompt{
		Label: "Project Description",
		Validate: func(input string) error {
			if len(strings.TrimSpace(input)) == 0 {
				return fmt.Errorf("project description cannot be empty")
			}
			return nil
		},
	}
	description, err := prompt.Run()
	if err != nil {
		print_error("Failed to get project description: " + err.Error())
		return
	}
	description = strings.TrimSpace(description)

	print_status("Project description: " + description)
	fmt.Println()

	// Database selection
	databaseType, err := selectDatabase()
	if err != nil {
		print_error("Failed to select database: " + err.Error())
		return
	}

	print_status("Selected database: " + databaseType)
	fmt.Println()

	// Get or create SYNC_TOKEN
	syncToken, err := getOrCreateSyncToken()
	if err != nil {
		print_error("Failed to get sync token: " + err.Error())
		return
	}

	// Create project via HTTP request
	if err := createProjectViaAPI(projectName, description, databaseType, syncToken); err != nil {
		print_error("Failed to create project: " + err.Error())
		return
	}

	print_success("🎉 Project created successfully!")
	print_status("Redirecting to project dashboard...")
	print_status("You can access your project at: http://localhost:4000/projects")
}

func selectDatabase() (string, error) {
	databases := []struct {
		Name string
		Icon string
		Type string
	}{
		{"Embed & SQL", "mdi:database", "embed"},
		{"MySQL", "logos:mysql", "mysql"},
		{"MariaDB", "logos:mariadb", "mariadb"},
		{"PostgreSQL", "logos:postgresql", "postgresql"},
		{"Couchbase", "logos:couchbase", "couchbase"},
		{"Oracle", "logos:oracle", "oracle"},
		{"Firestore", "logos:firebase", "firestore"},
		{"MongoDB", "logos:mongodb", "mongodb"},
		{"DynamoDB", "logos:aws-dynamodb", "dynamodb"},
	}

	var options []string
	for _, db := range databases {
		options = append(options, fmt.Sprintf("%s (%s)", db.Name, db.Icon))
	}

	prompt := promptui.Select{
		Label: "Select Database Type",
		Items: options,
	}

	index, _, err := prompt.Run()
	if err != nil {
		return "", err
	}

	return databases[index].Type, nil
}

func getOrCreateSyncToken() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding home directory: %w", err)
	}

	config, err := getConfig(filepath.Join(homeDir, ".apito", "bin"))
	if err != nil {
		return "", fmt.Errorf("error reading config: %w", err)
	}

	syncToken := config["SYNC_TOKEN"]
	if syncToken != "" {
		print_status("Using existing SYNC_TOKEN")
		return syncToken, nil
	}

	print_warning("SYNC_TOKEN not found in configuration")
	print_status("To create a sync token:")
	print_status("1. Go to http://localhost:4000")
	print_status("2. Navigate to Cloud Sync option")
	print_status("3. Copy the generated token")
	fmt.Println()

	prompt := promptui.Prompt{
		Label: "Paste your SYNC_TOKEN here",
		Validate: func(input string) error {
			if len(strings.TrimSpace(input)) == 0 {
				return fmt.Errorf("sync token cannot be empty")
			}
			return nil
		},
	}

	token, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get sync token: %w", err)
	}

	token = strings.TrimSpace(token)

	// Save the token to config
	config["SYNC_TOKEN"] = token
	if err := saveConfig(filepath.Join(homeDir, ".apito"), config); err != nil {
		return "", fmt.Errorf("error saving sync token: %w", err)
	}

	print_success("SYNC_TOKEN saved to configuration")
	return token, nil
}

func createProjectViaAPI(name, description, databaseType, syncToken string) error {
	requestBody := CreateProjectRequest{
		Name:         name,
		Description:  description,
		DatabaseType: databaseType,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error marshaling request: %w", err)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request
	req, err := http.NewRequest("POST", "http://localhost:5050/system/project/create", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+syncToken)

	print_status("Sending request to create project...")

	// Make request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %w", err)
	}

	// Parse response
	var response CreateProjectResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error parsing response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error (%d): %s", resp.StatusCode, response.Message)
	}

	if !response.Success {
		return fmt.Errorf("project creation failed: %s", response.Message)
	}

	print_success(fmt.Sprintf("Project '%s' created with ID: %s", response.Data.Name, response.Data.ID))
	return nil
}

func createFunction(cmd *cobra.Command) {
	print_error("Function creation not implemented yet")
	print_status("This feature will be available in a future update")
}

func createModel(cmd *cobra.Command) {
	print_error("Model creation not implemented yet")
	print_status("This feature will be available in a future update")
}
