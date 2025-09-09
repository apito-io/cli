package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"os/exec"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [engine|console] [--db system|project]",
	Short: "Show running status for Apito services",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mode, _ := determineRunMode()
		if mode == "docker" {
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}

			// Check main services (engine/console)
			engine, console, err := dockerComposePs()
			if err != nil {
				print_error("Failed to get docker status: " + err.Error())
				return
			}

			// Check database status if db-compose.yml exists
			dbStatus, err := getDatabaseStatus()
			if err == nil && dbStatus != nil {
				showDatabaseStatus(dbStatus)
			}

			if len(args) == 1 {
				if args[0] == "engine" {
					showDockerService("engine", engine)
				} else if args[0] == "console" {
					showDockerService("console", console)
				} else {
					print_error("Unknown service: " + args[0])
				}
				return
			}

			showDockerService("engine", engine)
			fmt.Println()
			showDockerService("console", console)
			return
		}

		if len(args) == 1 {
			showServiceStatus(args[0])
			return
		}
		showServiceStatus("engine")
		fmt.Println()
		showServiceStatus("console")
	},
}

func showServiceStatus(name string) {
	running, pid, logPath := serviceRunning(name)
	if running {
		print_success(fmt.Sprintf("%s is running (pid=%d)", name, pid))
		lines, err := ReadLastLines(logPath, 50)
		if err == nil && len(lines) > 0 {
			print_status(fmt.Sprintf("Last %d log lines for %s:", len(lines), name))
			for _, l := range lines {
				fmt.Println("  " + l)
			}
		} else {
			print_status("No logs available yet for " + name)
		}
	} else {
		print_warning(name + " is not running")
	}
}

func showDockerService(name string, running bool) {
	if running {
		print_success(fmt.Sprintf("%s (docker) is running", name))
	} else {
		print_warning(name + " (docker) is not running")
	}
}

// getDatabaseStatus returns the status of all database containers
func getDatabaseStatus() (map[string]bool, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dbComposePath := filepath.Join(homeDir, ".apito", "db-compose.yml")
	if _, err := os.Stat(dbComposePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no database compose file found")
	}

	// Use Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "-f", dbComposePath, "ps", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		// Fall back to Docker Compose v1
		cmd = exec.Command("docker-compose", "-f", dbComposePath, "ps", "--format", "json")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get database status: %w", err)
		}
	}

	// Parse the JSON output to get container statuses
	// For now, we'll use a simpler approach with docker ps
	cmd = exec.Command("docker", "ps", "--filter", "name=apito-", "--format", "{{.Names}}")
	out, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get container status: %w", err)
	}

	// Parse container names and check if they're running
	containerNames := strings.Split(strings.TrimSpace(string(out)), "\n")
	dbStatus := make(map[string]bool)

	for _, name := range containerNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		// Check if this is a database container
		if strings.Contains(name, "postgres") || strings.Contains(name, "mysql") ||
			strings.Contains(name, "mariadb") || strings.Contains(name, "mongodb") ||
			strings.Contains(name, "redis") || strings.Contains(name, "sqlserver") {
			// Check if container is running
			cmd = exec.Command("docker", "inspect", "--format", "{{.State.Running}}", name)
			if out, err := cmd.Output(); err == nil {
				running := strings.TrimSpace(string(out)) == "true"
				dbStatus[name] = running
			}
		}
	}

	return dbStatus, nil
}

// showDatabaseStatus displays the status of all database containers
func showDatabaseStatus(dbStatus map[string]bool) {
	if len(dbStatus) == 0 {
		return
	}

	fmt.Println()
	print_status("Database Status:")
	for containerName, running := range dbStatus {
		if running {
			print_success(fmt.Sprintf("  %s: Running", containerName))
		} else {
			print_warning(fmt.Sprintf("  %s: Stopped", containerName))
		}
	}
}
