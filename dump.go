package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	dumpFromAccount  string
	dumpToAccount    string
	dumpTenantDomain string
	dumpDryRun       bool
	dumpYes          bool
	dumpAllowPush    bool
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Replace a destination project or tenant database from another Apito instance",
	Long: `Portable dump/restore between configured accounts.

Selects FROM and TO accounts and projects like apito sync, then replaces the
destination physical database. Destination project (and tenant, for SaaS
per-tenant) must already exist. Schema is a preflight hash — run
apito sync --type schema first if it does not match.

v1 is pull-only by default (destination must be local). Push to a remote
instance requires --allow-push and a second confirmation.`,
	SilenceUsage: true,
	RunE:         runDumpCommand,
}

func init() {
	dumpCmd.Flags().StringVar(&dumpFromAccount, "from", "", "Source account name")
	dumpCmd.Flags().StringVar(&dumpToAccount, "to", "", "Destination account name")
	dumpCmd.Flags().StringVar(&dumpTenantDomain, "tenant-domain", "", "Skip picker; use this domain on both FROM and TO")
	dumpCmd.Flags().BoolVar(&dumpDryRun, "dry-run", false, "Run preflight and print the system-DB write list without transferring")
	dumpCmd.Flags().BoolVar(&dumpYes, "yes", false, "Skip typed confirmation prompts")
	dumpCmd.Flags().BoolVar(&dumpAllowPush, "allow-push", false, "Allow replacing a non-local destination database")
	rootCmd.AddCommand(dumpCmd)
}

func runDumpCommand(cmd *cobra.Command, args []string) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if len(cfg.Accounts) == 0 {
		return fmt.Errorf("no accounts configured — run: apito account create <name>")
	}

	timeoutSec := cfg.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	if timeoutSec < 120 {
		timeoutSec = 120
	}

	fromName, err := resolveSyncAccount("FROM", dumpFromAccount)
	if err != nil {
		return err
	}
	if isFilesystemSyncSide(fromName) {
		return fmt.Errorf("apito dump does not support the filesystem account")
	}
	fromCfg, err := getAccountConfig(fromName)
	if err != nil {
		return err
	}
	fromBase := newSyncGraphQLClient(fromCfg.ServerURL, fromCfg.CloudSyncKey, timeoutSec)
	fromProject, err := selectSyncProject(fromBase, "FROM")
	if err != nil {
		return err
	}

	toName, err := resolveSyncAccount("TO", dumpToAccount)
	if err != nil {
		return err
	}
	if isFilesystemSyncSide(toName) {
		return fmt.Errorf("apito dump does not support the filesystem account")
	}
	toCfg, err := getAccountConfig(toName)
	if err != nil {
		return err
	}
	toBase := newSyncGraphQLClient(toCfg.ServerURL, toCfg.CloudSyncKey, timeoutSec)
	toProject, err := selectSyncProject(toBase, "TO")
	if err != nil {
		return err
	}

	if err := validateProjectProfiles(fromProject, toProject); err != nil {
		return err
	}

	profile := projectSyncProfileKey(fromProject)
	fromClient := fromBase.WithProject(fromProject.ID)
	toClient := toBase.WithProject(toProject.ID)

	var srcTenant, destTenant *SyncTenant
	if profile == "saas-per-tenant" {
		srcTenant, destTenant, err = resolveDumpTenants(fromClient, toClient, fromProject, toProject)
		if err != nil {
			return err
		}
	}

	if isDumpPush(fromCfg.ServerURL, toCfg.ServerURL) && !dumpAllowPush {
		return fmt.Errorf("destination %s is not local — dump pull-only by default. Re-run with --allow-push to replace a remote database", toCfg.ServerURL)
	}

	srcPF, err := fromClient.DumpPreflight(dumpTargetArgs(fromProject.ID, srcTenant))
	if err != nil {
		return fmt.Errorf("source preflight: %w", err)
	}
	dstPF, err := toClient.DumpPreflight(dumpTargetArgs(toProject.ID, destTenant))
	if err != nil {
		return fmt.Errorf("destination preflight: %w", err)
	}

	if err := dumpBlockingPreflight(fromProject, toProject, srcPF, dstPF, destTenant); err != nil {
		return err
	}

	printDumpSystemDBMatrix(toName, toCfg.ServerURL, dstPF, destTenant, profile)

	if dumpDryRun {
		print_success("Dry run complete — no bytes transferred.")
		return nil
	}

	if err := confirmDumpDestructive(toName, toCfg.ServerURL, toProject, destTenant, profile); err != nil {
		return err
	}

	tmp, err := os.CreateTemp("", "apito-dump-*.tar")
	if err != nil {
		return err
	}
	localPath := tmp.Name()
	_ = tmp.Close()
	defer os.Remove(localPath)

	print_step(fmt.Sprintf("Exporting %s from %s…", dumpPhysicalLabel(profile, srcTenant), fromName))
	if err := fromClient.DownloadDump(dumpTargetArgs(fromProject.ID, srcTenant), localPath); err != nil {
		return fmt.Errorf("export: %w", err)
	}
	print_check("Download complete")

	print_step(fmt.Sprintf("Importing into %s on %s…", dumpPhysicalLabel(profile, destTenant), toName))
	if err := toClient.UploadDump(dumpTargetArgs(toProject.ID, destTenant), localPath, toProject.Name); err != nil {
		return fmt.Errorf("import: %w", err)
	}
	print_success(fmt.Sprintf("Dump applied: %q (%s) → %q (%s)", fromProject.Name, fromProject.ID, toProject.Name, toProject.ID))
	return nil
}

func dumpTargetArgs(projectID string, tenant *SyncTenant) dumpTarget {
	t := dumpTarget{TargetType: "project", TargetID: projectID}
	if tenant != nil && strings.TrimSpace(tenant.ID) != "" {
		t.TargetType = "tenant"
		t.TenantID = tenant.ID
	}
	return t
}

func dumpPhysicalLabel(profile string, tenant *SyncTenant) string {
	if profile == "saas-per-tenant" && tenant != nil {
		return fmt.Sprintf("tenant DB %s (%s)", tenant.ID, tenant.Domain)
	}
	return "project DB"
}

func resolveDumpTenants(fromClient, toClient *SyncGraphQLClient, fromProject, toProject SyncProject) (*SyncTenant, *SyncTenant, error) {
	domain := strings.TrimSpace(dumpTenantDomain)
	if domain == "" && dumpYes {
		return nil, nil, fmt.Errorf("saas-per-tenant dump with --yes requires --tenant-domain")
	}
	if domain == "" {
		var err error
		domain, err = pickDumpTenantDomain(fromClient, fromProject.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	if domain == "" {
		return nil, nil, fmt.Errorf("tenant domain required for saas-per-tenant dump")
	}

	src, err := lookupDumpTenant(fromClient, fromProject.ID, domain, "FROM")
	if err != nil {
		return nil, nil, err
	}
	printDumpTenantLine("FROM", src)
	dst, err := lookupDumpTenant(toClient, toProject.ID, domain, "TO")
	if err != nil {
		return nil, nil, err
	}
	printDumpTenantLine("TO", dst)
	printDumpTenantPair(src, dst)
	return src, dst, nil
}

func pickDumpTenantDomain(fromClient *SyncGraphQLClient, fromProjectID string) (string, error) {
	print_step("How do you want to pick the tenant?")
	modeSel := promptui.Select{
		Label: "Tenant picker",
		Items: []string{"Search tenants", "Enter domain"},
	}
	_, mode, err := modeSel.Run()
	if err != nil {
		return "", fmt.Errorf("tenant picker cancelled")
	}
	if mode == "Enter domain" {
		prompt := promptui.Prompt{Label: "Tenant domain (same correlator on FROM and TO)"}
		domain, err := prompt.Run()
		if err != nil {
			return "", fmt.Errorf("tenant domain cancelled")
		}
		return strings.TrimSpace(domain), nil
	}

	qPrompt := promptui.Prompt{Label: "Search FROM tenants (name or domain; empty lists first page)"}
	q, err := qPrompt.Run()
	if err != nil {
		return "", fmt.Errorf("tenant search cancelled")
	}
	rows, err := fromClient.SearchTenants(fromProjectID, strings.TrimSpace(q), 30)
	if err != nil {
		return "", fmt.Errorf("search FROM tenants: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no FROM tenants matched %q", strings.TrimSpace(q))
	}
	picked, err := selectDumpTenantRow(rows)
	if err != nil {
		return "", err
	}
	domain := strings.TrimSpace(picked.Domain)
	if domain == "" {
		return "", fmt.Errorf("FROM tenant %s has no domain — enter a domain instead", picked.ID)
	}
	return domain, nil
}

func selectDumpTenantRow(rows []SyncTenant) (SyncTenant, error) {
	if len(rows) == 1 {
		return rows[0], nil
	}
	labels := make([]string, len(rows))
	by := make(map[string]SyncTenant, len(rows))
	for i, t := range rows {
		label := dumpTenantLabel(t)
		labels[i] = label
		by[label] = t
	}
	sel := promptui.Select{
		Label: "Select FROM tenant (domain is reused on TO)",
		Items: labels,
		Size:  min(len(labels), 12),
	}
	_, picked, err := sel.Run()
	if err != nil {
		return SyncTenant{}, fmt.Errorf("tenant selection cancelled")
	}
	return by[picked], nil
}

func dumpTenantLabel(t SyncTenant) string {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		name = "(unnamed)"
	}
	dom := strings.TrimSpace(t.Domain)
	if dom == "" {
		dom = "(no domain)"
	}
	return fmt.Sprintf("%s  domain=%s  (%s)", name, dom, t.ID)
}

func printDumpTenantLine(side string, t *SyncTenant) {
	if t == nil {
		return
	}
	print_check(fmt.Sprintf("%s tenant: %s  domain=%s  (%s)", side, t.Name, t.Domain, t.ID))
}

func printDumpTenantPair(src, dst *SyncTenant) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  DUMP TENANTS (same domain correlator)")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	if src != nil {
		fmt.Printf("  FROM  %s\n        domain: %s\n        id:     %s\n", src.Name, src.Domain, src.ID)
	}
	if dst != nil {
		fmt.Printf("  TO    %s\n        domain: %s\n        id:     %s\n", dst.Name, dst.Domain, dst.ID)
	}
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

func lookupDumpTenant(client *SyncGraphQLClient, projectID, domain, side string) (*SyncTenant, error) {
	t, err := client.SearchTenantsByDomain(projectID, domain)
	if err == nil && t != nil && strings.TrimSpace(t.ID) != "" {
		return t, nil
	}
	rows, lerr := client.SearchTenants(projectID, domain, 20)
	if lerr == nil {
		for i := range rows {
			if strings.EqualFold(strings.TrimSpace(rows[i].Domain), domain) {
				return &rows[i], nil
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s tenant lookup by domain %q: %w (destination tenant must already exist)", side, domain, err)
	}
	return nil, fmt.Errorf("%s tenant with domain %q not found — dump never provisions a tenant", side, domain)
}

func confirmDumpDestructive(toName, toURL string, toProject SyncProject, destTenant *SyncTenant, profile string) error {
	if dumpYes {
		return nil
	}
	print_step("Type the destination project name to confirm replacement")
	prompt := promptui.Prompt{
		Label: fmt.Sprintf("Type %q", toProject.Name),
	}
	got, err := prompt.Run()
	if err != nil {
		return fmt.Errorf("confirmation cancelled")
	}
	if strings.TrimSpace(got) != toProject.Name {
		return fmt.Errorf("confirmation does not match destination project name")
	}

	if profile == "saas-shared" {
		print_warning("This will replace pro_tenants rows for this project only.")
		p2 := promptui.Prompt{Label: `Type "pro_tenants" to confirm catalog replace`}
		got, err = p2.Run()
		if err != nil {
			return fmt.Errorf("confirmation cancelled")
		}
		if strings.TrimSpace(got) != "pro_tenants" {
			return fmt.Errorf("SaaS catalog confirmation failed")
		}
	}
	if profile == "saas-per-tenant" && destTenant != nil {
		print_warning(fmt.Sprintf("This will update dest tenant row id=%s (metadata only; dest id and credentials stay).", destTenant.ID))
		p2 := promptui.Prompt{Label: fmt.Sprintf("Type dest tenant id %q", destTenant.ID)}
		got, err = p2.Run()
		if err != nil {
			return fmt.Errorf("confirmation cancelled")
		}
		if strings.TrimSpace(got) != destTenant.ID {
			return fmt.Errorf("tenant id confirmation failed")
		}
	}

	if isDumpPush("", toURL) {
		host := dumpHost(toURL)
		print_warning(fmt.Sprintf("PUSH: this replaces the database on %s (%s).", toName, host))
		p3 := promptui.Prompt{Label: fmt.Sprintf("Type the destination host %q", host)}
		got, err = p3.Run()
		if err != nil {
			return fmt.Errorf("confirmation cancelled")
		}
		if strings.TrimSpace(got) != host {
			return fmt.Errorf("push host confirmation failed")
		}
	}
	return nil
}

func dumpHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return strings.TrimSpace(raw)
	}
	return u.Hostname()
}
