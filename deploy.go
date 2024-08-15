package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var deployCmd = &cobra.Command{
	Use:       "deploy",
	Short:     "Deploy A Project to Apito Cloud",
	Long:      `Deploy your localhost project to Apito Cloud, AWS, or Google Cloud.`,
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		
		projectName, _ := cmd.Flags().GetString("name")
		if projectName == "" {
			fmt.Println("Error: project name is required")
			return
		}

		if err := deployApito(projectName); err != nil {
			fmt.Println("Error deploying to Docker:", err)
		}
		
	},
}

func deployApito(project string) error {
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
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

	config, err := getConfig(projectDir)
	if err != nil {
		fmt.Println("Error reading config file:", err)
	}

	config["DEPLOY_TOKEN"] = deployToken
	


	return nil
}