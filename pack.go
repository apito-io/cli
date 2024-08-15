package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var packCmd = &cobra.Command{
	Use:       "deploy",
	Short:     "Deploy the project to a specified provider",
	Long:      `Deploy the project to Docker, zip, AWS, or Google Cloud.`,
	ValidArgs: []string{"apito"},
	Args:      cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		project, _ := cmd.Flags().GetString("project")

		if project == "" {
			fmt.Println("Error: --project is required")
			return
		}

		actionName := args[0]

		switch actionName {
		case "apito":
			if err := deployApito(project); err != nil {
				fmt.Println("Error deploying to Docker:", err)
			}
		case "aws":
			deployAWS(project)
		case "google":
			deployGoogle(project)
		default:
			fmt.Println("Invalid provider. Use 'apito'")
		}
	},
}

func packApito(project string) error {
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

func deployAWS(project string) {
	fmt.Println("Deploying to AWS not implemented yet.")
}

func deployGoogle(project string) {
	fmt.Println("Deploying to Google Cloud not implemented yet.")
}
