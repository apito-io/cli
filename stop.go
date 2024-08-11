package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the engine for the specified project",
	Long:  `Stop the engine process based on the PID stored in ~/.apito/<project>/.env file`,
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		if project == "" {
			fmt.Println("Error: --project is required")
			return
		}
		stopEngine(project)
	},
}

func stopEngine(project string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Error finding home directory:", err)
		return
	}
	projectDir := filepath.Join(homeDir, ".apito", project)

	envMap, err := getConfig(projectDir)
	if err != nil {
		fmt.Println("Error reading config file:", err)
		return
	}

	pidStr, ok := envMap["ENGINE_PID"]
	if !ok {
		fmt.Println("No running engine PID found in config file")
		return
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Println("Invalid PID in config file:", err)
		return
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Error finding process:", err)
		return
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Println("Error stopping engine process:", err)
		return
	}

	// Remove the PID from the .env file
	err = updateConfig(projectDir, "ENGINE_PID", "")
	if err != nil {
		fmt.Println("Error updating config file:", err)
		return
	}

	fmt.Println("Engine process stopped")
}
