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
