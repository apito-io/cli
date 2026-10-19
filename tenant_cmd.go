package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	tenantName   string
	tenantDomain string
)

var tenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "List and create tenants when the server exposes the catalog",
	Long: `Tenant list and create use the server's /system/cli/v1/tenants contract.

A self-hosted engine that does not expose that capability asks you to connect
an Apito Cloud account. Hosted quotas, billing, and subscriptions stay on Cloud.`,
}

var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List tenants for the selected project",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := requireProjectClient(capTenantCatalog, "apito tenant")
		if err != nil {
			return err
		}
		var out struct {
			Tenants []map[string]interface{} `json:"tenants"`
		}
		if err := ctx.Client.getJSON("/system/cli/v1/tenants", &out); err != nil {
			return err
		}
		return printJSONOrLines(out.Tenants, func() {
			for _, row := range out.Tenants {
				fmt.Printf("%v\t%v\t%v\n", row["id"], row["name"], row["status"])
			}
		})
	},
}

var tenantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tenant",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(tenantName) == "" {
			return fmt.Errorf("--name is required")
		}
		if flagDryRun {
			return printJSONOrLines(map[string]string{"dry_run": "true", "name": tenantName}, nil)
		}
		ctx, err := requireProjectClient(capTenantCatalog, "apito tenant")
		if err != nil {
			return err
		}
		var out map[string]interface{}
		body := map[string]string{"name": tenantName, "domain": tenantDomain}
		if err := ctx.Client.postJSON("/system/cli/v1/tenants", body, &out); err != nil {
			return err
		}
		return printJSONOrLines(out, func() {
			fmt.Printf("created %v\n", out["id"])
		})
	},
}

func initTenantFlags() {
	tenantCreateCmd.Flags().StringVar(&tenantName, "name", "", "Tenant name")
	tenantCreateCmd.Flags().StringVar(&tenantDomain, "domain", "", "Tenant domain")
	tenantCreateCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the action without calling the server")
	tenantCreateCmd.Flags().BoolVar(&flagYes, "yes", false, "Skip confirmation")
}
