package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"gopkg.in/yaml.v3"
)

// DockerCompose represents the structure of a docker-compose.yml file
type DockerCompose struct {
	Services map[string]Service `yaml:"services"`
	Volumes  map[string]Volume  `yaml:"volumes"`
}

// Service represents a Docker service configuration
type Service struct {
	Image         string   `yaml:"image"`
	ContainerName string   `yaml:"container_name"`
	Environment   []string `yaml:"environment"`
	Ports         []string `yaml:"ports"`
	Volumes       []string `yaml:"volumes"`
	Restart       string   `yaml:"restart"`
	Command       string   `yaml:"command,omitempty"`
}

// Volume represents a Docker volume configuration
type Volume struct {
	// Empty struct for named volumes
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host         string
	Port         string
	Username     string
	Password     string
	Database     string
	Engine       string
	ExtraOptions map[string]string
}

// getContainerName returns the container name based on engine and type
func getContainerName(engine, dbType string) string {
	return fmt.Sprintf("apito-%s-%s", dbType, engine)
}

// getDatabasePort returns a unique port for the database based on engine and type
func getDatabasePort(engine, dbType string) string {
	// Base ports for different engines
	basePorts := map[string]string{
		"postgres":  "5432",
		"mysql":     "3306",
		"mariadb":   "3307",
		"sqlserver": "1433",
		"mongodb":   "27017",
		"redis":     "6379",
	}

	basePort := basePorts[engine]
	if basePort == "" {
		return "5432" // fallback
	}

	// For system databases, use base port
	// For project databases, use base port + 1000 to avoid conflicts
	if dbType == "system" {
		return basePort
	} else {
		// Parse base port and add 1000
		if port, err := strconv.Atoi(basePort); err == nil {
			return strconv.Itoa(port + 1000)
		}
		return basePort
	}
}

// ensureDockerAvailable checks if Docker is installed and available
func ensureDockerAvailable() error {
	cmd := exec.Command("docker", "--version")
	if err := cmd.Run(); err != nil {
		var installMsg string
		switch runtime.GOOS {
		case "darwin":
			installMsg = "Install Docker Desktop from https://docs.docker.com/desktop/install/mac-install/"
		case "linux":
			installMsg = "Install Docker Engine from https://docs.docker.com/engine/install/"
		case "windows":
			installMsg = "Install Docker Desktop from https://docs.docker.com/desktop/install/windows-install/"
		default:
			installMsg = "Install Docker from https://docs.docker.com/get-docker/"
		}
		return fmt.Errorf("Docker is not installed or not in PATH. %s", installMsg)
	}
	return nil
}

// dockerComposeInstalled checks if Docker Compose is available
func dockerComposeInstalled() error {
	// Try Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "version")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try Docker Compose v1
	cmd = exec.Command("docker-compose", "--version")
	if err := cmd.Run(); err == nil {
		return nil
	}

	return errors.New("Docker Compose is not installed. Install Docker Compose v2 (recommended) or v1")
}

// dockerRunning checks if Docker daemon is running
func dockerRunning() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return errors.New("Docker daemon is not running. Start Docker Desktop or Docker service")
	}
	return nil
}

// ensureDockerAndComposeAvailable checks Docker, Docker Compose, and daemon status
func ensureDockerAndComposeAvailable() error {
	if err := ensureDockerAvailable(); err != nil {
		return err
	}

	if err := dockerComposeInstalled(); err != nil {
		return err
	}

	if err := dockerRunning(); err != nil {
		return err
	}

	return nil
}

func getComposeFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".apito", "docker-compose.yml"), nil
}

// dbComposeFilePath returns the path to the database docker-compose file
func getDBComposeFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".apito", "db-compose.yml"), nil
}

// writeDBComposeFile creates a docker-compose file for the specified database engine with default credentials
func writeDBComposeFile(engine string) (string, error) {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return "", err
	}

	path, err := getDBComposeFilePath()
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	if err := os.MkdirAll(apitoDir, 0755); err != nil {
		return "", err
	}

	// Create compose structure
	compose := &DockerCompose{
		Services: make(map[string]Service),
		Volumes:  make(map[string]Volume),
	}

	// Add service based on engine
	switch engine {
	case "postgres":
		compose.Services["apito-postgres"] = Service{
			Image:         "postgres:16",
			ContainerName: "apito-postgres",
			Environment: []string{
				"POSTGRES_PASSWORD=apito",
				"POSTGRES_USER=apito",
				"POSTGRES_DB=apito",
			},
			Ports:   []string{"5432:5432"},
			Volumes: []string{"apito-postgres_data:/var/lib/postgresql/data"},
			Restart: "unless-stopped",
		}
		compose.Volumes["apito-postgres_data"] = Volume{}

	case "mysql":
		compose.Services["apito-mysql"] = Service{
			Image:         "mysql:8",
			ContainerName: "apito-mysql",
			Environment: []string{
				"MYSQL_ROOT_PASSWORD=apito",
				"MYSQL_DATABASE=apito",
				"MYSQL_USER=apito",
				"MYSQL_PASSWORD=apito",
			},
			Ports:   []string{"3306:3306"},
			Volumes: []string{"apito-mysql_data:/var/lib/mysql"},
			Restart: "unless-stopped",
		}
		compose.Volumes["apito-mysql_data"] = Volume{}

	case "mariadb":
		compose.Services["apito-mariadb"] = Service{
			Image:         "mariadb:11",
			ContainerName: "apito-mariadb",
			Environment: []string{
				"MARIADB_ROOT_PASSWORD=apito",
				"MARIADB_DATABASE=apito",
				"MARIADB_USER=apito",
				"MARIADB_PASSWORD=apito",
			},
			Ports:   []string{"3307:3306"},
			Volumes: []string{"apito-mariadb_data:/var/lib/mysql"},
			Restart: "unless-stopped",
		}
		compose.Volumes["apito-mariadb_data"] = Volume{}

	case "sqlserver":
		compose.Services["apito-sqlserver"] = Service{
			Image:         "mcr.microsoft.com/mssql/server:2022-latest",
			ContainerName: "apito-sqlserver",
			Environment: []string{
				"ACCEPT_EULA=Y",
				"MSSQL_SA_PASSWORD=Apito@12345",
			},
			Ports:   []string{"1433:1433"},
			Volumes: []string{"apito-sqlserver_data:/var/opt/mssql"},
			Restart: "unless-stopped",
		}
		compose.Volumes["apito-sqlserver_data"] = Volume{}

	case "mongodb":
		compose.Services["apito-mongodb"] = Service{
			Image:         "mongo:7",
			ContainerName: "apito-mongodb",
			Ports:         []string{"27017:27017"},
			Volumes:       []string{"apito-mongodb_data:/data/db"},
			Restart:       "unless-stopped",
		}
		compose.Volumes["apito-mongodb_data"] = Volume{}

	default:
		return "", errors.New("unsupported database engine")
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		return "", err
	}

	return path, nil
}

// writeDBComposeFileWithConfig creates a docker-compose file for the specified database engine with custom configuration
func writeDBComposeFileWithConfig(engine, dbType string, config DatabaseConfig) (string, error) {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return "", err
	}

	path, err := getDBComposeFilePath()
	if err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	if err := os.MkdirAll(apitoDir, 0755); err != nil {
		return "", err
	}

	// Read existing compose file if it exists
	compose := &DockerCompose{
		Services: make(map[string]Service),
		Volumes:  make(map[string]Volume),
	}

	if _, err := os.Stat(path); err == nil {
		// File exists, read and parse it
		contentBytes, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(contentBytes, compose); err != nil {
				// If parsing fails, start fresh
				compose = &DockerCompose{
					Services: make(map[string]Service),
					Volumes:  make(map[string]Volume),
				}
			}
		}
	}

	// Get container name
	containerName := getContainerName(engine, dbType)

	// Get unique port for this database instance
	port := getDatabasePort(engine, dbType)

	// Create service based on engine
	var service Service
	switch engine {
	case "postgres":
		service = Service{
			Image:         "postgres:16",
			ContainerName: containerName,
			Environment: []string{
				fmt.Sprintf("POSTGRES_PASSWORD=%s", config.Password),
				fmt.Sprintf("POSTGRES_USER=%s", config.Username),
				fmt.Sprintf("POSTGRES_DB=%s", config.Database),
			},
			Ports:   []string{fmt.Sprintf("%s:%s", port, "5432")},
			Volumes: []string{fmt.Sprintf("%s_data:/var/lib/postgresql/data", containerName)},
			Restart: "unless-stopped",
		}

	case "mysql":
		service = Service{
			Image:         "mysql:8.0",
			ContainerName: containerName,
			Environment: []string{
				fmt.Sprintf("MYSQL_ROOT_PASSWORD=%s", config.Password),
				fmt.Sprintf("MYSQL_DATABASE=%s", config.Database),
				fmt.Sprintf("MYSQL_USER=%s", config.Username),
				fmt.Sprintf("MYSQL_PASSWORD=%s", config.Password),
			},
			Ports:   []string{fmt.Sprintf("%s:%s", port, "3306")},
			Volumes: []string{fmt.Sprintf("%s_data:/var/lib/mysql", containerName)},
			Restart: "unless-stopped",
		}

	case "mariadb":
		service = Service{
			Image:         "mariadb:11",
			ContainerName: containerName,
			Environment: []string{
				fmt.Sprintf("MARIADB_ROOT_PASSWORD=%s", config.Password),
				fmt.Sprintf("MARIADB_DATABASE=%s", config.Database),
				fmt.Sprintf("MARIADB_USER=%s", config.Username),
				fmt.Sprintf("MARIADB_PASSWORD=%s", config.Password),
			},
			Ports:   []string{fmt.Sprintf("%s:%s", port, "3306")},
			Volumes: []string{fmt.Sprintf("%s_data:/var/lib/mysql", containerName)},
			Restart: "unless-stopped",
		}

	case "mongodb":
		service = Service{
			Image:         "mongo:7",
			ContainerName: containerName,
			Environment: []string{
				fmt.Sprintf("MONGO_INITDB_ROOT_USERNAME=%s", config.Username),
				fmt.Sprintf("MONGO_INITDB_ROOT_PASSWORD=%s", config.Password),
				fmt.Sprintf("MONGO_INITDB_DATABASE=%s", config.Database),
			},
			Ports:   []string{fmt.Sprintf("%s:%s", port, "27017")},
			Volumes: []string{fmt.Sprintf("%s_data:/data/db", containerName)},
			Restart: "unless-stopped",
		}

	case "redis":
		service = Service{
			Image:         "redis:7-alpine",
			ContainerName: containerName,
			Command:       fmt.Sprintf("redis-server --requirepass %s", config.Password),
			Ports:         []string{fmt.Sprintf("%s:%s", port, "6379")},
			Volumes:       []string{fmt.Sprintf("%s_data:/data", containerName)},
			Restart:       "unless-stopped",
		}

	default:
		return "", errors.New("unsupported database engine")
	}

	// Add or update service
	compose.Services[containerName] = service

	// Add volume
	volumeName := fmt.Sprintf("%s_data", containerName)
	compose.Volumes[volumeName] = Volume{}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		return "", err
	}

	return path, nil
}

// removeDatabaseFromCompose removes a specific database service from the compose file
func removeDatabaseFromCompose(dbType string) error {
	path, err := getDBComposeFilePath()
	if err != nil {
		return err
	}

	// Check if compose file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to remove
	}

	// Read existing content
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Parse YAML
	compose := &DockerCompose{}
	if err := yaml.Unmarshal(contentBytes, compose); err != nil {
		return fmt.Errorf("failed to parse compose file: %w", err)
	}

	// Get the database configuration to determine which service to remove
	config, err := loadDatabaseConfig(dbType == "system")
	if err != nil {
		return fmt.Errorf("no %s database configuration found: %w", dbType, err)
	}

	// Get container name
	containerName := getContainerName(config.Engine, dbType)
	volumeName := fmt.Sprintf("%s_data", containerName)

	// Remove service and volume
	delete(compose.Services, containerName)
	delete(compose.Volumes, volumeName)

	// If no services left, remove the file
	if len(compose.Services) == 0 {
		return os.Remove(path)
	}

	// Marshal back to YAML
	yamlData, err := yaml.Marshal(compose)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Write the updated content
	return os.WriteFile(path, yamlData, 0644)
}

// dockerComposeUp runs docker compose up with detailed image pulling feedback
func dockerComposeUp() error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	path, err := getComposeFilePath()
	if err != nil {
		return err
	}

	// Check if compose file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		print_warning(fmt.Sprintf("no docker-compose.yml file found. creating it..."))
		// create it
		_, err := writeComposeFile()
		if err != nil {
			return err
		}
	}

	// Load config to show what versions are being used
	cfg, err := loadCLIConfig()
	if err == nil {
		engineVer := cfg.EngineVersion
		if engineVer == "" {
			engineVer = "latest"
		}
		consoleVer := cfg.ConsoleVersion
		if consoleVer == "" {
			consoleVer = "latest"
		}

		print_status(fmt.Sprintf("Engine version: %s", engineVer))
		print_status(fmt.Sprintf("Console version: %s", consoleVer))
		fmt.Println()
	}

	// Pull images explicitly to show progress
	print_status("Pulling Docker images...")
	pullCmd := exec.Command("docker", "compose", "-f", path, "pull")
	pullCmd.Stdout = os.Stdout
	pullCmd.Stderr = os.Stderr

	if err := pullCmd.Run(); err != nil {
		// Try v1 syntax if v2 fails
		pullCmd = exec.Command("docker-compose", "-f", path, "pull")
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			print_warning("Could not pull images: " + err.Error())
			print_status("Attempting to start with cached images...")
		}
	}
	fmt.Println()

	// Start services
	print_status("Starting containers...")

	// Try Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "-f", path, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fall back to Docker Compose v1
	cmd = exec.Command("docker-compose", "-f", path, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// dockerComposeDown runs docker compose down
func dockerComposeDown() error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	path, err := getComposeFilePath()
	if err != nil {
		return err
	}

	// Check if compose file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to stop
	}

	// Try Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "-f", path, "down")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fall back to Docker Compose v1
	cmd = exec.Command("docker-compose", "-f", path, "down")
	return cmd.Run()
}

// dockerComposePs shows docker compose status
func dockerComposePs() (bool, bool, error) {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return false, false, err
	}

	path, err := getComposeFilePath()
	if err != nil {
		return false, false, err
	}

	// Check if compose file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, false, errors.New("no database compose file found")
	}

	// Try Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "-f", path, "ps", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return false, false, err
	}

	// Parse output to check service status
	_ = out // Suppress unused variable warning
	engineRunning := false
	consoleRunning := false

	// For now, return false for both since this is for database compose
	// The main engine/console status is handled elsewhere
	return engineRunning, consoleRunning, nil
}

// writeComposeFile creates the main docker-compose.yml for engine and console
func writeComposeFile() (string, error) {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return "", err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	apitoDir := filepath.Join(homeDir, ".apito")
	if err := os.MkdirAll(apitoDir, 0755); err != nil {
		return "", err
	}

	engineDataDir := filepath.Join(apitoDir, "db")
	if err := os.MkdirAll(engineDataDir, 0777); err != nil {
		return "", err
	}
	_ = os.Chmod(engineDataDir, 0777)

	// Mount .env from ~/.apito/bin/.env into container workdir .env
	// Note: .env file will be created by ensureDefaultEnvironmentConfig() in init.go
	envDir := filepath.Join(apitoDir, "bin")
	_ = os.MkdirAll(envDir, 0755)
	envFile := filepath.Join(envDir, ".env")
	// Don't create empty .env file here - let init.go handle it with proper defaults

	// Load CLI config to get version information
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load CLI config: %w", err)
	}

	// Use version from config or default to latest
	engineVersion := "latest"
	if cfg.EngineVersion != "" {
		engineVersion = cfg.EngineVersion
	}

	consoleVersion := "latest"
	if cfg.ConsoleVersion != "" {
		consoleVersion = cfg.ConsoleVersion
	}

	// Create compose structure
	compose := &DockerCompose{
		Services: make(map[string]Service),
		Volumes:  make(map[string]Volume),
	}

	// Add engine service
	compose.Services["engine"] = Service{
		Image:         fmt.Sprintf("ghcr.io/apito-io/engine:%s", engineVersion),
		ContainerName: "apito-engine",
		Environment:   []string{},
		Ports:         []string{"5050:5050"},
		Volumes: []string{
			fmt.Sprintf("%s:/app/.env", envFile),
			fmt.Sprintf("%s:/app/db", engineDataDir),
		},
		Restart: "unless-stopped",
	}

	// Add console service
	compose.Services["console"] = Service{
		Image:         fmt.Sprintf("ghcr.io/apito-io/console:%s", consoleVersion),
		ContainerName: "apito-console",
		Environment:   []string{},
		Ports:         []string{"4000:8080"},
		Volumes:       []string{},
		Restart:       "unless-stopped",
	}

	// Marshal to YAML
	yamlData, err := yaml.Marshal(compose)
	if err != nil {
		return "", fmt.Errorf("failed to marshal YAML: %w", err)
	}

	// Write to file
	path := filepath.Join(apitoDir, "docker-compose.yml")
	if err := os.WriteFile(path, yamlData, 0644); err != nil {
		return "", err
	}

	return path, nil
}

// dockerComposeUpFile runs docker compose up with a specific file
func dockerComposeUpFile(filePath string) error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	// Try Docker Compose v2 first
	cmd := exec.Command("docker", "compose", "-f", filePath, "up", "-d")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fall back to Docker Compose v1
	cmd = exec.Command("docker-compose", "-f", filePath, "up", "-d")
	return cmd.Run()
}

// dockerComposeStopDB stops a specific database container
func dockerComposeStopDB(dbType string) error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	// Get the database configuration to determine which container to stop
	config, err := loadDatabaseConfig(dbType == "system")
	if err != nil {
		return fmt.Errorf("no %s database configuration found: %w", dbType, err)
	}

	// Get container name
	containerName := getContainerName(config.Engine, dbType)

	// Stop the specific database container
	cmd := exec.Command("docker", "stop", containerName)
	return cmd.Run()
}

// dockerComposeStartDB starts a specific database container
func dockerComposeStartDB(dbType string) error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	// Get the database configuration to determine which container to start
	config, err := loadDatabaseConfig(dbType == "system")
	if err != nil {
		return fmt.Errorf("no %s database configuration found: %w", dbType, err)
	}

	// Get container name
	containerName := getContainerName(config.Engine, dbType)

	// Start the specific database container
	cmd := exec.Command("docker", "start", containerName)
	return cmd.Run()
}

// dockerComposeRestartDB restarts a specific database container
func dockerComposeRestartDB(dbType string) error {
	if err := ensureDockerAndComposeAvailable(); err != nil {
		return err
	}

	// Get the database configuration to determine which container to restart
	config, err := loadDatabaseConfig(dbType == "system")
	if err != nil {
		return fmt.Errorf("no %s database configuration found: %w", dbType, err)
	}

	// Get container name
	containerName := getContainerName(config.Engine, dbType)

	// Restart the specific database container
	cmd := exec.Command("docker", "restart", containerName)
	return cmd.Run()
}
