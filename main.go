package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "apito",
		Short: "Apito CLI",
		Long:  `Apito CLI to manage projects, functions, and more.`,
	}

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(changePassCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
