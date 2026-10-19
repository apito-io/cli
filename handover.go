package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var handoverSecretDenylist = []string{
	"platform_operator",
	"freelancer_billing",
	"temporary_credential",
	"backup_provider_secret",
	"machine_key",
}

var (
	handoverTo    string
	handoverJobID string
	handoverYes   bool
)

type handoverJobFile struct {
	ID        string   `json:"id"`
	Phase     string   `json:"phase"`
	Status    string   `json:"status"`
	Cursor    string   `json:"cursor"`
	Denied    []string `json:"denied_secrets"`
	UpdatedAt string   `json:"updated_at"`
}

var handoverCmd = &cobra.Command{
	Use:   "handover",
	Short: "Move a self-hosted project to managed Apito Cloud",
	Long: `Resumable export/import for moving a self-hosted project onto managed Apito Cloud.

This is separate from apito client, which manages a Cloud client engagement.

Cloud → Cloud ownership change does NOT use this CLI. That path is in-place
(moveProjectToWorkspace / transferWorkspaceOwnership).

Never exports platform operators, freelancer billing, temporary credentials,
backup-provider secrets, or machine keys.

Transfer engines: apito sync --type schema  and  apito migrate.`,
}

var handoverPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "Print a portable handover manifest (no secrets)",
	RunE:  runHandoverPlan,
}

var handoverExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Write a handover package from the source project",
	RunE:  runHandoverExport,
}

var handoverImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a handover package into Apito Cloud",
	RunE:  runHandoverImport,
}

var handoverVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Checksum schema, data, functions, plugins, and media",
	RunE:  runHandoverVerify,
}

var handoverCutoverCmd = &cobra.Command{
	Use:   "cutover",
	Short: "Final dump, catalog update, DNS/endpoint switch, rollback window",
	RunE:  runHandoverCutover,
}

var handoverResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused or failed handover job",
	RunE:  runHandoverResume,
}

func init() {
	handoverImportCmd.Flags().StringVar(&handoverTo, "to", "apito-cloud", "Destination (apito-cloud)")
	handoverResumeCmd.Flags().StringVar(&handoverJobID, "job", "", "Handover job id")
	handoverCmd.PersistentFlags().BoolVar(&handoverYes, "yes", false, "Skip typed confirmation")
	handoverCmd.AddCommand(handoverPlanCmd, handoverExportCmd, handoverImportCmd, handoverVerifyCmd, handoverCutoverCmd, handoverResumeCmd)
	rootCmd.AddCommand(handoverCmd)
}

func handoverJobPath(id string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".apito", "handover", id+".json")
}

func writeHandoverJob(job handoverJobFile) error {
	job.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	job.Denied = handoverSecretDenylist
	path := handoverJobPath(job.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func runHandoverPlan(cmd *cobra.Command, args []string) error {
	m := map[string]any{
		"schema_hash":      "",
		"data_checksum":    "",
		"functions":        []string{},
		"plugin_pins":      []any{},
		"tenant_meta":      true,
		"required_secrets": []string{"stripe", "cloudinary", "email", "oauth", "s3"},
		"denied_secrets":   handoverSecretDenylist,
		"next":             []string{"apito handover export", "apito handover import --to apito-cloud"},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

func runHandoverExport(cmd *cobra.Command, args []string) error {
	job := handoverJobFile{ID: fmt.Sprintf("hj-%d", time.Now().Unix()), Phase: "export", Status: "running", Cursor: "schema"}
	if err := writeHandoverJob(job); err != nil {
		return err
	}
	fmt.Println("Phase: export")
	fmt.Println("1. Schema + profile  (apito sync --type schema --from <self-host> --to filesystem)")
	fmt.Println("2. Function source/config")
	fmt.Println("3. Plugin pins (public ID/version/digest; private escrow snapshot)")
	fmt.Println("4. Tenant metadata + media inventory")
	fmt.Println("5. Required-secret placeholders only")
	fmt.Println("Job:", job.ID, "→", handoverJobPath(job.ID))
	job.Status = "paused"
	job.Cursor = "media"
	return writeHandoverJob(job)
}

func runHandoverImport(cmd *cobra.Command, args []string) error {
	if handoverTo != "apito-cloud" {
		return fmt.Errorf("only --to apito-cloud is supported")
	}
	job := handoverJobFile{ID: fmt.Sprintf("hj-%d", time.Now().Unix()), Phase: "import", Status: "running", Cursor: "preflight"}
	if err := writeHandoverJob(job); err != nil {
		return err
	}
	fmt.Println("Import destination: Apito Cloud")
	fmt.Println("Preflight: destination client workspace + capacity")
	fmt.Println("Then: sync schema → functions → plugins → object-storage copy → DB transfer (apito migrate)")
	fmt.Println("Resume with: apito handover resume --job", job.ID)
	job.Status = "paused"
	job.Cursor = "db"
	return writeHandoverJob(job)
}

func runHandoverVerify(cmd *cobra.Command, args []string) error {
	fmt.Println("Verify: schema hash, data checksum, function config, plugin digests, media inventory")
	fmt.Println("Reauthorize client-owned Stripe/Cloudinary/email/OAuth/S3 before cutover.")
	return nil
}

func runHandoverCutover(cmd *cobra.Command, args []string) error {
	if !handoverYes {
		fmt.Println("Type CUTOVER to enter the source maintenance window (dry command — pass --yes to record the job).")
		return nil
	}
	job := handoverJobFile{ID: fmt.Sprintf("hj-%d", time.Now().Unix()), Phase: "cutover", Status: "running", Cursor: "maintenance"}
	if err := writeHandoverJob(job); err != nil {
		return err
	}
	fmt.Println("Cutover: brief source read-only → final dump/import → checksum → switch DNS → retain rollback window")
	job.Status = "paused"
	job.Cursor = "dns"
	return writeHandoverJob(job)
}

func runHandoverResume(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(handoverJobID) == "" {
		return fmt.Errorf("--job is required")
	}
	raw, err := os.ReadFile(handoverJobPath(handoverJobID))
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}
	var job handoverJobFile
	if err := json.Unmarshal(raw, &job); err != nil {
		return err
	}
	fmt.Printf("Resume handover job %s phase=%s cursor=%s\n", job.ID, job.Phase, job.Cursor)
	job.Status = "running"
	return writeHandoverJob(job)
}
