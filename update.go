package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringP("version", "v", "", "Adds a function for that project")
}

var updateCmd = &cobra.Command{
	Use:       "update",
	Short:     "Update apito engine and console",
	Long:      `Update the apito engine and console to the latest version.`,
	ValidArgs: []string{"engine", "console"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		version, _ := cmd.Flags().GetString("version")

		actionName := args[0]

		switch actionName {
		case "engine":
			replaceEngine(project, version)
		case "console":
			replaceConsole(project, version)
		}
	},
}

func replaceEngine(projectName, version string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	projectDir := filepath.Join(homeDir, ".apito", projectName)

	if version == "" {
		fmt.Println("No version specified, pulling latest version")
		releaseTag, err := getLatestReleaseTag()
		if err != nil {
			fmt.Println("error fetching latest release tag: %w", err)
			return
		}
		version = releaseTag
	}

	// Detect runtime environment and download the appropriate asset
	if err := downloadAndExtractEngine(projectName, version, projectDir); err != nil {
		fmt.Println("Error downloading and extracting binary:", err)
		return
	}
}
func replaceConsole(projectName, version string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	files, err := ioutil.ReadDir(apitoDir)
	if err != nil {
		fmt.Println("Error reading apito directory:", err)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			fmt.Println(f.Name())
		}
	}
}
