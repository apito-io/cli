package main

import (
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [engine|console|all] [--db system|project]",
	Short: "Stop Apito services",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Check if --db flag is set
		dbType, _ := cmd.Flags().GetString("db")

		if dbType != "" {
			if dbType != "system" && dbType != "project" {
				print_error("Invalid database type. Use 'system' or 'project'")
				return
			}
			// Stop only the specified database
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}
			if err := dockerComposeStopDB(dbType); err != nil {
				print_error("Failed to stop " + dbType + " database: " + err.Error())
				return
			}
			print_success(dbType + " database stopped")
			return
		}

		mode, _ := determineRunMode()
		target := "all"
		if len(args) == 1 {
			target = args[0]
		}
		if mode == "docker" {
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}
			_ = dockerComposeDown()
			print_success("Docker services stopped")
			return
		}
		switch target {
		case "engine":
			_ = stopManagedService("engine")
			print_success("Engine stopped")
		case "console":
			_ = stopManagedService("console")
			print_success("Console stopped")
		case "all":
			_ = stopManagedService("console")
			_ = stopManagedService("engine")
			print_success("All services stopped")
		default:
			print_error("Unknown target. Use one of: engine, console, all")
		}
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart [engine|console|all] [--db system|project]",
	Short: "Restart Apito services",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Check if --db flag is set
		dbType, _ := cmd.Flags().GetString("db")

		if dbType != "" {
			if dbType != "system" && dbType != "project" {
				print_error("Invalid database type. Use 'system' or 'project'")
				return
			}
			// Restart only the specified database
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}
			if err := dockerComposeRestartDB(dbType); err != nil {
				print_error("Failed to restart " + dbType + " database: " + err.Error())
				return
			}
			print_success(dbType + " database restarted")
			return
		}

		mode, _ := determineRunMode()
		target := "all"
		if len(args) == 1 {
			target = args[0]
		}
		if mode == "docker" {
			if err := ensureDockerAndComposeAvailable(); err != nil {
				print_error("Docker not available: " + err.Error())
				return
			}
			_ = dockerComposeDown()
			if err := dockerComposeUp(); err != nil {
				print_error("Failed to start docker services: " + err.Error())
				return
			}
			print_success("Docker services restarted")
			return
		}
		switch target {
		case "engine":
			_ = stopManagedService("engine")
			if err := startManagedService("engine"); err != nil {
				print_error("Failed to restart engine: " + err.Error())
				return
			}
			print_success("Engine restarted")
		case "console":
			_ = stopManagedService("console")
			if err := startManagedService("console"); err != nil {
				print_error("Failed to restart console: " + err.Error())
				return
			}
			print_success("Console restarted")
		case "all":
			_ = stopManagedService("console")
			_ = stopManagedService("engine")
			if err := startManagedService("engine"); err != nil {
				print_error("Failed to start engine: " + err.Error())
				return
			}
			if err := startManagedService("console"); err != nil {
				print_error("Failed to start console: " + err.Error())
				return
			}
			print_success("All services restarted")
		default:
			print_error("Unknown target. Use one of: engine, console, all")
		}
	},
}

func init() {
	stopCmd.Flags().String("db", "", "Stop only the specified database (system|project)")
	restartCmd.Flags().String("db", "", "Restart only the specified database (system|project)")
}
