package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const studioContainerName = "apito-studio"
const studioPublishPort = "5055"

// studioEnginePort is the port inside the container. The public engine refuses 5050.
const studioEnginePort = "5051"

func startStudioContainer() error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	if err := ensureDockerAvailable(); err != nil {
		return err
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	image := studioImageRef(cfg)
	print_status("Pulling " + image)
	if err := runDocker("pull", image); err != nil {
		if !studioImagePresent(image) {
			return fmt.Errorf("pull %s: %w", image, err)
		}
		print_warning("Could not refresh " + image + ". Using the image already on this machine.")
	}
	removeStudioContainer()
	print_status("Starting " + studioContainerName)
	args := []string{
		"run", "-d",
		"--name", studioContainerName,
		"-p", studioPublishPort + ":" + studioPublishPort,
	}
	mounts, err := studioVolumeArgs()
	if err != nil {
		return err
	}
	args = append(args, mounts...)
	args = append(args, image)
	if err := runDocker(args...); err != nil {
		return fmt.Errorf("run studio: %w", err)
	}
	print_success("Studio is running")
	print_status("Console: http://localhost:" + studioPublishPort)
	printStudioLogin()
	return nil
}

func stopStudioContainer() error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	if err := ensureDockerAvailable(); err != nil {
		return err
	}
	if err := runDocker("stop", studioContainerName); err != nil {
		return err
	}
	print_success("Studio stopped")
	return nil
}

func restartStudioContainer() error {
	if err := stopStudioContainer(); err != nil {
		print_status("Studio was not running; starting it.")
	}
	return startStudioContainer()
}

func studioStatus() error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	if err := ensureDockerAvailable(); err != nil {
		return err
	}
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}} {{.Config.Image}}", studioContainerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		print_status("Studio container " + studioContainerName + " is not running")
		return nil
	}
	print_status(studioContainerName + ": " + strings.TrimSpace(string(out)))
	print_status("Console: http://localhost:" + studioPublishPort)
	return nil
}

func studioLogs(follow bool, tail int) error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	if err := ensureDockerAvailable(); err != nil {
		return err
	}
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, studioContainerName)
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runDocker(args ...string) error {
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func homeApitoDB() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/apito-db"
	}
	return filepath.Join(home, ".apito", "db")
}

func studioVolumeArgs() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error finding home directory: %w", err)
	}
	return []string{
		"-v", filepath.Join(home, ".apito", "db") + ":" + studioDataMount,
		"-v", filepath.Join(home, ".apito", "bin", studioEngineEnvName) + ":/opt/apito/.env:ro",
		"-v", filepath.Join(home, ".apito", "bin", studioConsoleEnvName) + ":/opt/apito/.env.console:ro",
	}, nil
}

func removeStudioContainer() {
	cmd := exec.Command("docker", "rm", "-f", studioContainerName)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	_ = cmd.Run()
}

func studioImagePresent(image string) bool {
	cmd := exec.Command("docker", "image", "inspect", image)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}
