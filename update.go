package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

func init() {
	updateCmd.Flags().StringP("version", "v", "", "Target version (optional)")
}

var updateCmd = &cobra.Command{
	Use:       "update",
	Short:     "Update apito engine, console, or self",
	Long:      `Update apito engine, console, or the CLI itself to the latest or specified version`,
	ValidArgs: []string{"engine", "console", "self"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		version, _ := cmd.Flags().GetString("version")
		actionName := args[0]

		switch actionName {
		case "engine":
			if err := updateEngine(version); err != nil {
				fmt.Println("Update engine failed:", err)
			}
		case "console":
			if err := updateConsole(version); err != nil {
				fmt.Println("Update console failed:", err)
			}
		case "self":
			if err := updateSelf(version); err != nil {
				fmt.Println("Self update failed:", err)
			}
		}
	},
}

func latestTag(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.TagName == "" {
		return "", errors.New("empty tag")
	}
	return out.TagName, nil
}

func confirmPrompt(msg string) bool {
	p := promptui.Select{Label: msg, Items: []string{"Yes", "No"}}
	_, v, err := p.Run()
	if err != nil {
		return false
	}
	return v == "Yes"
}

func updateSelf(ver string) error {
	// Determine latest if not provided
	if ver == "" {
		tag, err := latestTag("apito-io/cli")
		if err != nil {
			return err
		}
		ver = tag
	}
	// Compare with current version
	if version == ver {
		fmt.Println("CLI already up-to-date:", ver)
		return nil
	}
	if !confirmPrompt(fmt.Sprintf("Update CLI from %s to %s?", version, ver)) {
		return nil
	}

	// Download platform artifact
	osName := runtime.GOOS
	arch := runtime.GOARCH
	asset := fmt.Sprintf("apito_%s_%s_%s.tar.gz", ver, osName, arch)
	url := fmt.Sprintf("https://github.com/apito-io/cli/releases/download/%s/%s", ver, asset)
	tmp, err := downloadFileWithProgress(url, os.TempDir())
	if err != nil {
		return err
	}
	dir, err := extractArchiveToTemp(tmp)
	if err != nil {
		return err
	}
	// Move apito binary to install location in PATH
	// Heuristic: prefer /usr/local/bin, else ~/.local/bin
	dest := "/usr/local/bin"
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		dest = filepath.Join(os.Getenv("HOME"), ".local", "bin")
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}
	src, err := findBinaryInDir(dir, "apito")
	if err != nil {
		return err
	}
	final := filepath.Join(dest, "apito")
	if err := os.Rename(src, final); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(final, 0755)
	}
	fmt.Println("CLI updated:", ver, "->", final)
	return nil
}

func updateEngine(ver string) error {
	if ver == "" {
		tag, err := latestTag("apito-io/engine")
		if err != nil {
			return err
		}
		ver = tag
	}
	if !confirmPrompt("Update engine to " + ver + "?") {
		return nil
	}
	home, _ := os.UserHomeDir()
	return downloadEngine(ver, home)
}

func updateConsole(ver string) error {
	if ver == "" {
		tag, err := latestTag("apito-io/console")
		if err != nil {
			return err
		}
		ver = tag
	}
	if !confirmPrompt("Update console to " + ver + "?") {
		return nil
	}
	home, _ := os.UserHomeDir()
	return downloadConsole(ver, home)
}
