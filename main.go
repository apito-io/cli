package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

// Define rootCmd globally so other files can access it
var rootCmd = &cobra.Command{
	Use:           "apito",
	Short:         "Apito CLI",
	Long:          `Apito CLI for local engines and Apito Cloud. One binary. Cloud-only commands stay visible and fail closed until the account has the capability.`,
	SilenceErrors: true,
}

func main() {
	// Set version and enable version flag
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Version}}\n")

	rootCmd.PersistentFlags().StringVar(&flagAccount, "account", "", "Named account from apito login")
	rootCmd.PersistentFlags().StringVar(&flagProject, "project", "", "Project id")
	rootCmd.PersistentFlags().StringVar(&flagTenant, "tenant", "", "Tenant id")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Print machine-readable JSON")
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		noteCLICommand(cmd)
	}

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(pluginCmd)
	rootCmd.AddCommand(adminCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
