package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

const studioContainerName = "apito-studio"
const studioPublishPort = "4055"

// studioEnginePort is the port inside the container. The public engine refuses 5050.
const studioEnginePort = "5055"

func chooseStudioImage(cfg *CLIConfig) (image, upgradeTag string) {
	currentTag := studioTag(cfg)
	currentImage := studioImageRef(cfg)

	latestTag, err := getLatestStudioVersion()
	if err != nil {
		print_warning("Could not check for Studio image updates: " + err.Error())
		return currentImage, ""
	}

	if !studioUpgradeAvailable(currentTag, latestTag) {
		return currentImage, ""
	}

	created, repoTags := inspectStudioImage(currentImage)
	currentVersion := formatStudioCurrentVersion(currentTag, created, repoTags)
	print_status("Current Studio version: " + currentVersion)
	print_status("Latest Studio version: " + latestTag)
	if !confirmStudioUpgrade(currentVersion, latestTag) {
		print_status("Keeping current Studio image.")
		return currentImage, ""
	}

	return studioImageRefForTag(latestTag), latestTag
}

func inspectStudioImage(image string) (time.Time, []string) {
	cmd := exec.Command("docker", "image", "inspect", "-f", "{{.Created}}\t{{json .RepoTags}}", image)
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}, nil
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	var created time.Time
	if len(parts) > 0 {
		created, _ = time.Parse(time.RFC3339Nano, parts[0])
	}
	var tags []string
	if len(parts) == 2 && parts[1] != "" && parts[1] != "null" {
		_ = json.Unmarshal([]byte(parts[1]), &tags)
	}
	return created, tags
}

func confirmStudioUpgrade(currentVersion, latestTag string) bool {
	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Upgrade Studio image %s → %s? (y/N)", currentVersion, latestTag),
		IsConfirm: true,
		Default:   "n",
	}
	answer, err := prompt.Run()
	if err != nil {
		print_status("Upgrade prompt unavailable; keeping current Studio image.")
		return false
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

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
	currentImage := studioImageRef(cfg)
	image, upgradeTag := chooseStudioImage(cfg)
	print_status("Pulling " + image)
	if err := runDocker("pull", image); err != nil {
		if upgradeTag != "" {
			print_warning(fmt.Sprintf("Could not upgrade Studio to %s: %v", upgradeTag, err))
			if !studioImagePresent(currentImage) {
				return fmt.Errorf("pull %s: %w", image, err)
			}
			image = currentImage
			upgradeTag = ""
			print_warning("Using the current Studio image already on this machine.")
		} else {
			if !studioImagePresent(image) {
				return fmt.Errorf("pull %s: %w", image, err)
			}
			print_warning("Could not refresh " + image + ". Using the image already on this machine.")
		}
	}

	if upgradeTag != "" {
		cfg.StudioVersion = upgradeTag
		if err := saveCLIConfig(cfg); err != nil {
			print_warning("Studio image upgraded, but could not save studio_version: " + err.Error())
		} else {
			print_success("Studio upgraded to " + upgradeTag)
		}
	}
	if err := ensureStudioPortsFree(); err != nil {
		return err
	}
	removeStudioContainer()
	print_status("Starting " + studioContainerName)
	args := []string{
		"run", "-d",
		"--name", studioContainerName,
		"-p", studioPublishPort + ":" + studioPublishPort,
		"-p", studioEnginePort + ":" + studioEnginePort,
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
	if err := waitForStudioReady(); err != nil {
		return fmt.Errorf("studio container started, but it is not ready: %w", err)
	}
	print_success("Studio is running")
	fmt.Println("  Console: http://localhost:" + studioPublishPort)
	fmt.Println("  Admin:   http://localhost:" + studioPublishPort + "/admin")
	fmt.Println("  Engine:  http://localhost:" + studioEnginePort)
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
	nginxPath, err := writeStudioNginx()
	if err != nil {
		return nil, err
	}
	return []string{
		"-v", filepath.Join(home, ".apito", "db") + ":" + studioDataMount,
		"-v", filepath.Join(home, ".apito", "bin", studioEngineEnvName) + ":/opt/apito/.env:ro",
		"-v", filepath.Join(home, ".apito", "bin", studioConsoleEnvName) + ":/opt/apito/.env.console:ro",
		"-v", nginxPath + ":/etc/nginx/http.d/default.conf:ro",
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
