package main

import (
	"os"
	"os/exec"

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

	// clone into current directory
	print_status("Cloning " + repo)
	cmd := exec.Command("git", "clone", repo)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// removed old project via API flow; console handles project creation now
