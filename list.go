package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().StringP("function", "f", "", "Adds a function for that project")
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects, functions",
	Long:  `List projects, functions in the Apito CLI.`,
	Run: func(cmd *cobra.Command, args []string) {

		actionName := args[0] // take only one and should be one

		projectName, _ := cmd.Flags().GetString("name")
		if projectName == "" {
			fmt.Println("Error: project name is required")
			return
		}

		projectName = strings.TrimSpace(projectName)

		switch actionName {
		case "function":
			listFunctions(projectName)
		default:
			listProjects()
		}
	
	},
}

func init() {
	listCmd.Flags().StringP("project", "p", "", "Project name")
}

func listProjects() {
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

func listFunctions(project string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	functionsDir := filepath.Join(homeDir, ".apito", project, "functions")
	files, err := ioutil.ReadDir(functionsDir)
	if err != nil {
		fmt.Println("Error reading functions directory:", err)
		return
	}

	for _, f := range files {
		if f.IsDir() {
			fmt.Println(f.Name())
		}
	}
}
