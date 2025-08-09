package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [engine|console]",
	Short: "Show running status for Apito services",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mode, _ := determineRunMode()
		if mode == "docker" {
			engine, console, err := dockerComposePs()
			if err != nil {
				print_error("Failed to get docker status: " + err.Error())
				return
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
