package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [engine|console] [--db system|project] [--follow] [--tail N]",
	Short: "Show logs for Apito services and databases",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mode, _ := determineRunMode()

		// Get flags
		dbFlag, _ := cmd.Flags().GetString("db")
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")

		if mode == "docker" {
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}

			// Handle database logs
			if dbFlag != "" {
				if err := showDatabaseLogs(dbFlag, follow, tail); err != nil {
					print_error("Failed to show database logs: " + err.Error())
				}
				return
			}

			// Handle main service logs
			service := "engine"
			if len(args) == 1 {
				service = args[0]
			}

			if err := showDockerServiceLogs(service, follow, tail); err != nil {
				print_error("Failed to show " + service + " logs: " + err.Error())
			}
			return
		}

		// Handle non-Docker mode (local services)
		service := "engine"
		if len(args) == 1 {
			service = args[0]
		}

		if err := showLocalServiceLogs(service, follow, tail); err != nil {
			print_error("Failed to show " + service + " logs: " + err.Error())
		}
	},
}

func init() {
	logsCmd.Flags().String("db", "", "Show logs for specific database (system|project)")
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
	logsCmd.Flags().IntP("tail", "n", 100, "Number of lines to show from the end of the logs")
}

// showDatabaseLogs displays logs for a specific database container
func showDatabaseLogs(dbType string, follow bool, tail int) error {
	// Get the database configuration to determine which container to show logs for
	config, err := loadDatabaseConfig(dbType == "system")
	if err != nil {
		return fmt.Errorf("no %s database configuration found: %w", dbType, err)
	}

	// Get container name
	containerName := getContainerName(config.Engine, dbType)

	// Build docker logs command
	args := []string{"logs"}

	if follow {
		args = append(args, "-f")
	}

	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}

	args = append(args, containerName)

	// Execute docker logs command
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	print_status(fmt.Sprintf("Showing logs for %s database (%s):", dbType, containerName))
	return cmd.Run()
}

// showDockerServiceLogs displays logs for Docker services (engine/console)
func showDockerServiceLogs(service string, follow bool, tail int) error {
	if service != "engine" && service != "console" {
		return fmt.Errorf("unknown service: %s. Use 'engine' or 'console'", service)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	composePath := filepath.Join(homeDir, ".apito", "docker-compose.yml")
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return fmt.Errorf("no docker-compose.yml found")
	}

	// Build docker compose logs command
	args := []string{"compose", "-f", composePath, "logs"}

	if follow {
		args = append(args, "-f")
	}

	if tail > 0 {
		args = append(args, "--tail", strconv.Itoa(tail))
	}

	args = append(args, service)

	// Try Docker Compose v2 first
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Fall back to Docker Compose v1
		args = []string{"-f", composePath, "logs"}
		if follow {
			args = append(args, "-f")
		}
		if tail > 0 {
			args = append(args, "--tail", strconv.Itoa(tail))
		}
		args = append(args, service)

		cmd = exec.Command("docker-compose", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	return nil
}

// showLocalServiceLogs displays logs for local services (non-Docker mode)
func showLocalServiceLogs(service string, follow bool, tail int) error {
	if service != "engine" && service != "console" {
		return fmt.Errorf("unknown service: %s. Use 'engine' or 'console'", service)
	}

	// Get log file path
	logPath := getLogPath(service)
	if logPath == "" {
		return fmt.Errorf("no log file found for %s", service)
	}

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file does not exist: %s", logPath)
	}

	// Build tail command
	args := []string{"-n", strconv.Itoa(tail)}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, logPath)

	// Execute tail command
	cmd := exec.Command("tail", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	print_status(fmt.Sprintf("Showing logs for %s service:", service))
	return cmd.Run()
}

// getLogPath returns the log file path for a service
func getLogPath(service string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	logDir := filepath.Join(homeDir, ".apito", "logs")
	return filepath.Join(logDir, service+".log")
}
