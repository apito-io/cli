package main

import (
	_ "archive/zip"
	"fmt"
	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/databasedriver/system"
	_ "io"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy A Project to Apito Cloud",
	Long:  `Deploy your localhost project to Apito Cloud, AWS, or Google Cloud.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {

		projectName, _ := cmd.Flags().GetString("name")
		if projectName == "" {
			fmt.Println("Error: project name is required")
			return
		}

		if err := deployApito(projectName); err != nil {
			fmt.Println("Error deploying to Apito Cloud", err)
		}

	},
}

func deployApito(project string) error {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	projectDir := filepath.Join(homeDir, ".apito", project)

	fmt.Println(Blue + fmt.Sprintf(`To Deploy your local project to Apito Cloud, You need a deploy token.`) + Reset)
	fmt.Println(Blue + `Go to https://app.apito.io and login. You will find the deploy token in the Console Space` + Reset)
	// Prompt for project description
	prompt := promptui.Prompt{
		Label: "Enter Apito Deploy Token",
	}

	deployToken, err := prompt.Run()
	if err != nil {
		fmt.Println("Prompt failed:", err)
		return nil
	}

	if err = updateConfig(projectDir, "DEPLOY_TOKEN", deployToken); err != nil {
		return fmt.Errorf("error updating config: %w", err)
	}

	fmt.Println(Green + "Deploying to Apito Cloud..." + Reset)

	config, err := getConfig(projectDir)
	if err != nil {
		fmt.Println("Error reading config file:", err)
	}

	// Connect to Local Project System DB
	db, err := connectToSystemDB(config)
	if err != nil {
		return fmt.Errorf("error connecting to system db: %w", err)
	}

	fmt.Println(db)

	return nil
}

func connectToSystemDB(config map[string]string) (interfaces.SystemDBInterface, error) {
	_cred := protobuff.DriverCredentials{
		Engine:   config["SYSTEM_DB_ENGINE"],
		Host:     config["SYSTEM_DB_HOST"],
		Port:     config["SYSTEM_DB_PORT"],
		User:     config["SYSTEM_DB_USER"],
		Password: config["SYSTEM_DB_PASSWORD"],
		Database: config["SYSTEM_DB_DATABASE"],
	}

	// cfg.SystemDatabaseDBConfig in place of nil
	systemDriver, err := system.GetSystemDriver(&_cred, nil)
	if err != nil {
		panic(err.Error()) // sure do a panic if system db not there
	}
	return systemDriver, nil
}
