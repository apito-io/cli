package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	clientEmail    string
	clientSnapshot string
	clientConfirm  string
	backupTarget   string
)

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Cloud client engagement invite and status",
	Long:  `apito client manages a Cloud client engagement. Project migration stays on apito handover.`,
}

var clientInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite a client to a Cloud project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if clientEmail == "" {
			return fmt.Errorf("--email is required")
		}
		if flagDryRun {
			return printJSONOrLines(map[string]string{"dry_run": "true", "email": clientEmail}, nil)
		}
		ctx, err := requireProjectClient(capClientLifecycle, "apito client")
		if err != nil {
			return err
		}
		var out map[string]interface{}
		if err := ctx.Client.postJSON("/system/cli/v1/client/invite", map[string]string{
			"email": clientEmail,
		}, &out); err != nil {
			return err
		}
		return printJSONOrLines(out, func() {
			fmt.Printf("invited %v\n", out["client_email"])
		})
	},
}

var clientStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show client engagement status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := requireProjectClient(capClientLifecycle, "apito client")
		if err != nil {
			return err
		}
		var out struct {
			Engagements []map[string]interface{} `json:"engagements"`
		}
		if err := ctx.Client.getJSON("/system/cli/v1/client/status", &out); err != nil {
			return err
		}
		return printJSONOrLines(out.Engagements, func() {
			for _, row := range out.Engagements {
				fmt.Printf("%v\t%v\t%v\n", row["id"], row["lifecycle"], row["delivery_status"])
			}
		})
	},
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Managed Cloud backups",
	Long:  `Requires a managed Cloud backup capability. Self-hosted engines do not expose this command's API.`,
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backup snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := requireProjectClient(capManagedBackups, "apito backup")
		if err != nil {
			return err
		}
		var out struct {
			Snapshots []map[string]interface{} `json:"snapshots"`
		}
		if err := ctx.Client.getJSON("/system/cli/v1/backups", &out); err != nil {
			return err
		}
		return printJSONOrLines(out.Snapshots, func() {
			for _, row := range out.Snapshots {
				fmt.Printf("%v\t%v\t%v\n", row["id"], row["status"], row["created_at"])
			}
		})
	},
}

var backupRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Start a backup and return the run id",
	RunE: func(cmd *cobra.Command, args []string) error {
		if flagDryRun {
			return printJSONOrLines(map[string]string{"dry_run": "true"}, nil)
		}
		ctx, err := requireProjectClient(capManagedBackups, "apito backup")
		if err != nil {
			return err
		}
		var out map[string]interface{}
		if err := ctx.Client.postJSON("/system/cli/v1/backups/run", map[string]string{}, &out); err != nil {
			return err
		}
		return printJSONOrLines(out, func() {
			fmt.Printf("run %v status %v\n", out["id"], out["status"])
		})
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Preview and apply a managed Cloud restore",
}

var restorePreviewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Preview a restore",
	RunE: func(cmd *cobra.Command, args []string) error {
		if clientSnapshot == "" {
			return fmt.Errorf("--snapshot is required")
		}
		ctx, err := requireProjectClient(capRestoreActions, "apito restore")
		if err != nil {
			return err
		}
		var out map[string]interface{}
		if err := ctx.Client.postJSON("/system/cli/v1/backups/preview", map[string]string{
			"snapshot_id": clientSnapshot,
			"target_type": backupTargetOr(ctx),
			"target_id":   backupTargetID(ctx),
		}, &out); err != nil {
			return err
		}
		return printJSONOrLines(out, func() {
			fmt.Printf("preview %v\n", out["compatibility"])
		})
	},
}

var restoreApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply a restore after preview and typed confirmation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if clientSnapshot == "" || clientConfirm == "" {
			return fmt.Errorf("--snapshot and --confirm are required")
		}
		ctx, err := requireProjectClient(capRestoreActions, "apito restore")
		if err != nil {
			return err
		}
		var preview map[string]interface{}
		targetType := backupTargetOr(ctx)
		targetID := backupTargetID(ctx)
		if err := ctx.Client.postJSON("/system/cli/v1/backups/preview", map[string]string{
			"snapshot_id": clientSnapshot,
			"target_type": targetType,
			"target_id":   targetID,
		}, &preview); err != nil {
			return err
		}
		if !flagYes {
			return printJSONOrLines(preview, func() {
				fmt.Println("Preview only. Re-run with --yes --confirm <target name> to apply.")
			})
		}
		var out map[string]interface{}
		if err := ctx.Client.postJSON("/system/cli/v1/backups/restore", map[string]interface{}{
			"snapshot_id":                 clientSnapshot,
			"target_type":                 targetType,
			"target_id":                   targetID,
			"confirm":                     clientConfirm,
			"confirm_name":                clientConfirm,
			"acknowledge_schema_mismatch": false,
		}, &out); err != nil {
			return err
		}
		return printJSONOrLines(out, func() {
			fmt.Printf("restore %v\n", out["status"])
		})
	},
}

func backupTargetOr(ctx commandContext) string {
	if backupTarget != "" {
		return backupTarget
	}
	if ctx.TenantID != "" {
		return "tenant"
	}
	return "project"
}

func backupTargetID(ctx commandContext) string {
	if ctx.TenantID != "" && backupTargetOr(ctx) == "tenant" {
		return ctx.TenantID
	}
	return ctx.ProjectID
}

func initCloudFlags() {
	clientInviteCmd.Flags().StringVar(&clientEmail, "email", "", "Client email")
	clientInviteCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the action without calling the server")
	backupRunCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the action without calling the server")
	restorePreviewCmd.Flags().StringVar(&clientSnapshot, "snapshot", "", "Snapshot id")
	restoreApplyCmd.Flags().StringVar(&clientSnapshot, "snapshot", "", "Snapshot id")
	restoreApplyCmd.Flags().StringVar(&clientConfirm, "confirm", "", "Type the target name to confirm")
	restoreApplyCmd.Flags().BoolVar(&flagYes, "yes", false, "Apply after preview")
	restorePreviewCmd.Flags().StringVar(&backupTarget, "target", "", "project or tenant")
	restoreApplyCmd.Flags().StringVar(&backupTarget, "target", "", "project or tenant")
}
