package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects, functions, or models",
	Long:  `List projects, functions, or models in the Apito CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			listProjects()
		} else {
			listFunctions(project)
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
