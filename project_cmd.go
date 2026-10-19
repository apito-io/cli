package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "List and open projects for the selected account",
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects the account can access",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := resolveCommandContext()
		if err != nil {
			return err
		}
		if err := requireCapability(ctx.Caps, capProjectAccess, "apito project"); err != nil {
			return err
		}
		projects, err := ctx.syncClient().ListProjects()
		if err != nil {
			return err
		}
		return printJSONOrLines(projects, func() {
			for _, p := range projects {
				fmt.Printf("%s\t%s\n", p.ID, p.Name)
			}
		})
	},
}

var projectOpenCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the project in the console URL provided by the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := resolveCommandContext()
		if err != nil {
			return err
		}
		if err := requireCapability(ctx.Caps, capProjectAccess, "apito project"); err != nil {
			return err
		}
		if strings.TrimSpace(ctx.ProjectID) == "" {
			return fmt.Errorf("a project is required. Pass --project or run apito project list")
		}
		console := strings.TrimSpace(ctx.Caps.ConsoleURL)
		if console == "" {
			console = strings.TrimSpace(ctx.Account.ConsoleURL)
		}
		payload := map[string]string{"project_id": ctx.ProjectID, "console_url": console}
		if console == "" {
			fmt.Fprintln(os.Stderr, "This server did not provide a console URL.")
			return printJSONOrLines(payload, func() {
				fmt.Println(ctx.ProjectID)
			})
		}
		fmt.Fprintln(os.Stderr, console)
		if err := loginOpen(console); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser. Open the URL above.\n")
		}
		if flagJSON {
			return printJSONOrLines(payload, nil)
		}
		return nil
	},
}

func printJSONOrLines(v interface{}, lines func()) error {
	if flagJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	if lines != nil {
		lines()
	}
	return nil
}
