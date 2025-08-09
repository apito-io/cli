package main

import (
    "archive/zip"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/spf13/cobra"
)

func init() {
	buildCmd.Flags().StringP("tag", "t", "", "Docker image tag (optional)")
}

var buildCmd = &cobra.Command{
	Use:       "build",
	Short:     "Build project for docker or zip",
	Long:      `Build the entire project for docker or zip`,
	ValidArgs: []string{"docker", "zip"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")
		tag, _ := cmd.Flags().GetString("tag")

		if project == "" {
			fmt.Println("Error: --project is required")
			return
		}

		actionName := args[0]

		switch actionName {
		case "docker":
			if err := deployDocker(project, tag); err != nil {
				fmt.Println("Error deploying to Docker:", err)
			}
		case "zip":
			if err := deployZip(project); err != nil {
				fmt.Println("Error deploying as Zip:", err)
			}
		default:
			fmt.Println("Invalid provider. Use 'docker', 'zip', 'aws', or 'google'.")
		}
	},
}

func deployDocker(project, tag string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	projectDir := filepath.Join(homeDir, ".apito", project)
	if tag == "" {
		tag = fmt.Sprintf("apito.io/project/%s", project)
	}

    imageName := strings.ToLower(project)
    if tag == "" {
        tag = fmt.Sprintf("apito.io/projects/%s", imageName)
    }
    // Build using docker CLI to avoid SDK import/versioning issues
    cmd := exec.Command("docker", "build", "-t", tag, projectDir)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("error building Docker image: %w", err)
    }
    fmt.Println("Docker image built successfully with tag:", tag)
	return nil
}

func deployZip(project string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home directory: %w", err)
	}
	projectDir := filepath.Join(homeDir, ".apito", project)
	zipFile := filepath.Join(homeDir, ".apito", fmt.Sprintf("%s.zip", project))

	zipf, err := os.Create(zipFile)
	if err != nil {
		return fmt.Errorf("error creating zip file: %w", err)
	}
	defer zipf.Close()

	zipWriter := zip.NewWriter(zipf)
	defer zipWriter.Close()

	err = filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(projectDir, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			_, err := zipWriter.Create(relPath + "/")
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		w, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, file)
		return err
	})
	if err != nil {
		return fmt.Errorf("error creating zip archive: %w", err)
	}

	fmt.Println("Project zipped successfully:", zipFile)
	return nil
}
