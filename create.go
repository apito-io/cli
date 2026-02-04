package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func init() {}

var createCmd = &cobra.Command{
	Use:       "create",
	Short:     "Create a new project or plugin",
	Long:      `Create a new project (redirects to console) or scaffold a plugin repository`,
	ValidArgs: []string{"project", "plugin"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "project":
			print_success("Project creation moved to Apito Console.")
			print_status("Open: http://localhost:4000/project/new")
		case "plugin":
			createPlugin()
		default:
			print_error("Invalid create option. Use 'project' or 'plugin'.")
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

func createPlugin() {
	print_step("🔌 Create Plugin Scaffold")
	langs := []string{"golang (default)", "javascript", "python"}
	sel := promptui.Select{Label: "Select language", Items: langs}
	idx, _, err := sel.Run()
	if err != nil {
		return
	}
	repo := "git@github.com:apito-io/apito-hello-world-go-plugin.git"
	if idx == 1 {
		repo = "git@github.com:apito-io/apito-hello-world-js-plugin.git"
	}
	if idx == 2 {
		repo = "git@github.com:apito-io/apito-hello-world-python-plugin.git"
	}

	// choose directory
	dirChoice := promptui.Select{
		Label: "Where to create the plugin?",
		Items: []string{"Current directory", "Different directory"},
	}
	dirIdx, _, err := dirChoice.Run()
	if err != nil {
		return
	}

	var targetDir string
	if dirIdx == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			print_error("Could not get current directory: " + err.Error())
			return
		}
		targetDir = cwd
	} else {
		prompt := promptui.Prompt{
			Label: "Enter directory path (e.g. ~/projects or /path/to/parent)",
		}
		pathInput, err := prompt.Run()
		if err != nil {
			return
		}
		pathInput = strings.TrimSpace(pathInput)
		if pathInput == "" {
			print_error("Directory path cannot be empty.")
			return
		}
		if strings.HasPrefix(pathInput, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				print_error("Could not resolve home directory: " + err.Error())
				return
			}
			pathInput = filepath.Join(home, strings.TrimPrefix(pathInput, "~"))
		}
		absPath, err := filepath.Abs(pathInput)
		if err != nil {
			print_error("Invalid path: " + err.Error())
			return
		}
		if err := os.MkdirAll(absPath, 0755); err != nil {
			print_error("Could not create directory: " + err.Error())
			return
		}
		targetDir = absPath
	}

	// repo name from URL (e.g. apito-hello-world-go-plugin)
	repoName := strings.TrimSuffix(filepath.Base(repo), ".git")
	clonePath := filepath.Join(targetDir, repoName)

	print_status("Cloning " + repo + " into " + clonePath)
	cmd := exec.Command("git", "clone", repo, clonePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		print_error("Clone failed: " + err.Error())
		return
	}
	print_success("Plugin scaffold created at " + clonePath)
}

// removed old project via API flow; console handles project creation now
