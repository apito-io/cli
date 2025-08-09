package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func dockerInstalled() bool {
	if _, err := exec.LookPath("docker"); err == nil {
		return true
	}
	// Mac with Docker Desktop may still have docker cli
	return false
}

func ensureDockerAvailable() error {
	if dockerInstalled() {
		return nil
	}
	// Provide OS-specific guidance
	switch runtime.GOOS {
	case "darwin":
		return errors.New("docker not found. please install Docker Desktop for Mac: https://docs.docker.com/desktop/install/mac-install/")
	case "linux":
		return errors.New("docker not found. please install Docker Engine: https://docs.docker.com/engine/install/")
	case "windows":
		return errors.New("docker not found. please install Docker Desktop for Windows: https://docs.docker.com/desktop/install/windows-install/")
	default:
		return errors.New("docker not found")
	}
}

func composeFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".apito", "docker-compose.yml"), nil
}

func dbComposeFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".apito", "db-compose.yml"), nil
}

func writeDBComposeFile(engine string) (string, error) {
	path, err := dbComposeFilePath()
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
	var content string
	switch engine {
	case "postgres":
		content = "services:\n" +
			"  postgres:\n" +
			"    image: postgres:16\n" +
			"    container_name: apito-postgres\n" +
			"    environment:\n" +
			"      - POSTGRES_PASSWORD=apito\n" +
			"      - POSTGRES_USER=apito\n" +
			"      - POSTGRES_DB=apito\n" +
			"    ports:\n" +
			"      - \"5432:5432\"\n" +
			"    restart: unless-stopped\n"
	case "mysql":
		content = "services:\n" +
			"  mysql:\n" +
			"    image: mysql:8\n" +
			"    container_name: apito-mysql\n" +
			"    environment:\n" +
			"      - MYSQL_ROOT_PASSWORD=apito\n" +
			"      - MYSQL_DATABASE=apito\n" +
			"      - MYSQL_USER=apito\n" +
			"      - MYSQL_PASSWORD=apito\n" +
			"    ports:\n" +
			"      - \"3306:3306\"\n" +
			"    restart: unless-stopped\n"
	case "mariadb":
		content = "services:\n" +
			"  mariadb:\n" +
			"    image: mariadb:11\n" +
			"    container_name: apito-mariadb\n" +
			"    environment:\n" +
			"      - MARIADB_ROOT_PASSWORD=apito\n" +
			"      - MARIADB_DATABASE=apito\n" +
			"      - MARIADB_USER=apito\n" +
			"      - MARIADB_PASSWORD=apito\n" +
			"    ports:\n" +
			"      - \"3307:3306\"\n" +
			"    restart: unless-stopped\n"
	case "sqlserver":
		content = "services:\n" +
			"  sqlserver:\n" +
			"    image: mcr.microsoft.com/mssql/server:2022-latest\n" +
			"    container_name: apito-sqlserver\n" +
			"    environment:\n" +
			"      - ACCEPT_EULA=Y\n" +
			"      - MSSQL_SA_PASSWORD=Apito@12345\n" +
			"    ports:\n" +
			"      - \"1433:1433\"\n" +
			"    restart: unless-stopped\n"
	case "mongodb":
		content = "services:\n" +
			"  mongodb:\n" +
			"    image: mongo:7\n" +
			"    container_name: apito-mongodb\n" +
			"    ports:\n" +
			"      - \"27017:27017\"\n" +
			"    restart: unless-stopped\n"
	default:
		return "", errors.New("unsupported database engine")
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func dockerComposeUpFile(path string) error {
	cmd := exec.Command("docker", "compose", "-f", path, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func writeComposeFile() (string, error) {
	path, err := composeFilePath()
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
	engineDataDir := filepath.Join(apitoDir, "engine-data")
	if err := os.MkdirAll(engineDataDir, 0755); err != nil {
		return "", err
	}

	// Mount .env from ~/.apito/bin/.env into container workdir .env
	envDir := filepath.Join(apitoDir, "bin")
	_ = os.MkdirAll(envDir, 0755)
	envFile := filepath.Join(envDir, ".env")

	content := "" +
		"services:\n" +
		"  engine:\n" +
		"    image: ghcr.io/apito-io/engine:latest\n" +
		"    container_name: apito-engine\n" +
		"    ports:\n" +
		"      - \"5050:5050\"\n" +
		"    volumes:\n" +
		"      - \"" + engineDataDir + ":/go/src/gitlab.com/apito.io/engine/db\"\n" +
		"      - \"" + envFile + ":/go/src/gitlab.com/apito.io/engine/.env\"\n" +
		"    restart: unless-stopped\n" +
		"  console:\n" +
		"    image: ghcr.io/apito-io/console:latest\n" +
		"    container_name: apito-console\n" +
		"    ports:\n" +
		"      - \"4000:8080\"\n" +
		"    restart: unless-stopped\n"

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}
	return path, nil
}

func dockerComposeUp() error {
	path, err := writeComposeFile()
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "compose", "-f", path, "up", "-d")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerComposeDown() error {
	path, err := composeFilePath()
	if err != nil {
		return err
	}
	cmd := exec.Command("docker", "compose", "-f", path, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func dockerComposePs() (bool, bool, error) {
	path, err := composeFilePath()
	if err != nil {
		return false, false, err
	}
	cmd := exec.Command("docker", "compose", "-f", path, "ps", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return false, false, err
	}
	s := string(out)
	engineRunning := strings.Contains(s, "apito-engine")
	consoleRunning := strings.Contains(s, "apito-console")
	return engineRunning, consoleRunning, nil
}
