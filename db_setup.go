package main

import (
	"github.com/manifoldco/promptui"
)

// startDatabaseInteractive prompts user to choose a database and starts it via Docker.
// No-ops if mode is not docker.
func startDatabaseInteractive() {
	mode, _ := determineRunMode()
	if mode != "docker" {
		print_status("Database helper is available in Docker mode only. Skipping.")
		return
	}

	print_status("Database setup (optional)...")
	prompt := promptui.Select{
		Label: "Select a database to run (or Skip)",
		Items: []string{"Postgres", "MySQL", "MariaDB", "SQLServer", "MongoDB", "Skip (I already have a database)"},
	}
	_, choice, err := prompt.Run()
	if err != nil || choice == "Skip (I already have a database)" {
		print_status("Skipping database setup")
		return
	}

	var engine string
	switch choice {
	case "Postgres":
		engine = "postgres"
	case "MySQL":
		engine = "mysql"
	case "MariaDB":
		engine = "mariadb"
	case "SQLServer":
		engine = "sqlserver"
	case "MongoDB":
		engine = "mongodb"
	}

	if engine == "" {
		print_error("Unsupported database selection")
		return
	}

	if err := ensureDockerAvailable(); err != nil {
		print_error("Docker not available: " + err.Error())
		return
	}
	path, err := writeDBComposeFile(engine)
	if err != nil {
		print_error("Failed to prepare DB compose: " + err.Error())
		return
	}
	if err := dockerComposeUpFile(path); err != nil {
		print_error("Failed to start database: " + err.Error())
		return
	}
	print_success("Database started via Docker: " + engine)
}
