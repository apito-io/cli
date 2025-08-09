package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "apito",
		Short: "Apito CLI",
		Args:  cobra.MinimumNArgs(1),
		Long:  `Apito CLI to manage projects, functions, and more.`,
	}
	var project string
	rootCmd.PersistentFlags().StringVarP(&project, "project", "p", "", "ver")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(changePassCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
