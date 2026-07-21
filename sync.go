package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	syncFromAccount   string
	syncToAccount     string
	syncType          string
	syncDryRun        bool
	syncYes           bool
	syncDir           string
	syncDeploy        bool
	syncIncludeSecret bool
)

// filesystemAccountName is the reserved account name that maps to a local
// directory (via --dir, defaulting to ~/.apito/temp/functions) instead of a
// remote GraphQL endpoint. Only valid for --type functions.
//
// Note: a configured account literally named "local" (common for localhost
// engines) is a normal remote account — it must NOT collide with this reserved
// name. Prefer "filesystem" for on-disk import/export.
const filesystemAccountName = "filesystem"

// legacyFilesystemAccountName was the old reserved token; kept only when no
// configured account uses that name (see isFilesystemSyncSide).
const legacyFilesystemAccountName = "local"

func isFilesystemSyncSide(name string) bool {
	n := strings.TrimSpace(name)
	if strings.EqualFold(n, filesystemAccountName) {
		return true
	}
	// "local" is filesystem only when it is NOT a configured CLI account.
	if strings.EqualFold(n, legacyFilesystemAccountName) {
		if _, err := getAccountConfig(n); err == nil {
			return false
		}
		return true
	}
	return false
}

// defaultFunctionsSyncDir returns ~/.apito/temp/functions.
func defaultFunctionsSyncDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".apito", "temp", "functions"), nil
}

func resolveFunctionsSyncDir() (string, error) {
	if d := strings.TrimSpace(syncDir); d != "" {
		return d, nil
	}
	dir, err := defaultFunctionsSyncDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create default functions dir %s: %w", dir, err)
	}
	print_status(fmt.Sprintf("Using functions directory: %s (override with --dir)", dir))
	return dir, nil
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync schema or content between Apito accounts/projects",
	Long: `Sync schema (models, fields, relations) or content between two configured accounts.

Uses the system GraphQL API (/system/graphql) with unified apt_ access tokens
(Authorization: Bearer). Legacy cli-/mcp-/sdk- prefixed keys are retired.
Schema changes are staged as drafts on pro engines — publish from Console when ready.`,
	RunE: runSyncCommand,
}

func init() {
	syncCmd.Flags().StringVar(&syncFromAccount, "from", "", "Source account name (or \"filesystem\" for on-disk functions)")
	syncCmd.Flags().StringVar(&syncToAccount, "to", "", "Destination account name (or \"filesystem\" for on-disk functions)")
	syncCmd.Flags().StringVar(&syncType, "type", "", "Sync type: schema, functions or content")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show planned changes without writing")
	syncCmd.Flags().BoolVar(&syncYes, "yes", false, "Skip confirmation prompts")
	syncCmd.Flags().StringVar(&syncDir, "dir", "", "Local functions directory (default: ~/.apito/temp/functions when using filesystem)")
	syncCmd.Flags().BoolVar(&syncDeploy, "deploy", false, "After upserting functions, deploy a new active revision on the destination")
	syncCmd.Flags().BoolVar(&syncIncludeSecret, "include-secrets", false, "Copy rest_api_secret_url_key instead of regenerating on the destination")
	rootCmd.AddCommand(syncCmd)
}

func runSyncCommand(cmd *cobra.Command, args []string) error {
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

	// Prompt order: FROM account → FROM project → TO account → TO project
	// (pick each side's project immediately after its account).
	fromName, err := resolveSyncAccount("FROM", syncFromAccount)
	if err != nil {
		return err
	}

	// Filesystem side has no project; resolve TO next, then remote project.
	if isFilesystemSyncSide(fromName) {
		toName, err := resolveSyncAccount("TO", syncToAccount)
		if err != nil {
			return err
		}
		kind := strings.ToLower(strings.TrimSpace(syncType))
		if kind != "" && kind != "functions" {
			return fmt.Errorf("account %q is only supported for --type functions", filesystemAccountName)
		}
		return runLocalFunctionSync(fromName, toName, timeoutSec, syncDeploy, syncIncludeSecret, syncDryRun, syncYes, nil)
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

	toName, err := resolveSyncAccount("TO", syncToAccount)
	if err != nil {
		return err
	}

	if isFilesystemSyncSide(toName) {
		kind := strings.ToLower(strings.TrimSpace(syncType))
		if kind != "" && kind != "functions" {
			return fmt.Errorf("account %q is only supported for --type functions", filesystemAccountName)
		}
		return runLocalFunctionSync(fromName, toName, timeoutSec, syncDeploy, syncIncludeSecret, syncDryRun, syncYes, &fromProject)
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

	fromClient := fromBase.WithProject(fromProject.ID)
	toClient := toBase.WithProject(toProject.ID)

	kind := strings.ToLower(strings.TrimSpace(syncType))
	if kind == "" {
		kind, err = selectSyncType()
		if err != nil {
			return err
		}
	}

	print_status(fmt.Sprintf("Syncing %s: %q (%s) -> %q (%s)", kind, fromProject.Name, fromProject.ID, toProject.Name, toProject.ID))

	endpoints := SyncEndpoints{
		FromAccount: fromName,
		ToAccount:   toName,
		FromURL:     fromCfg.ServerURL,
		ToURL:       toCfg.ServerURL,
		FromProject: fromProject,
		ToProject:   toProject,
	}

	switch kind {
	case "schema":
		return runSchemaSync(fromClient, toClient, endpoints, syncDryRun, syncYes)
	case "functions":
		return runFunctionSync(fromClient, toClient, endpoints, syncDeploy, syncIncludeSecret, syncDryRun, syncYes)
	case "content":
		return runContentSync(fromClient, toClient, fromProject, toProject, syncDryRun, syncYes)
	default:
		return fmt.Errorf("invalid --type %q (use schema, functions or content)", syncType)
	}
}

func resolveSyncAccount(label, flagValue string) (string, error) {
	if isFilesystemSyncSide(flagValue) {
		if strings.EqualFold(strings.TrimSpace(flagValue), filesystemAccountName) {
			return filesystemAccountName, nil
		}
		// Legacy "local" with no configured account → filesystem.
		return filesystemAccountName, nil
	}
	if flagValue != "" {
		if _, err := getAccountConfig(flagValue); err != nil {
			return "", err
		}
		return flagValue, nil
	}
	return selectAccountForSync(label)
}

func selectAccountForSync(label string) (string, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	names := getAccountNames(cfg)
	if len(names) == 0 {
		return "", fmt.Errorf("no accounts configured")
	}
	if len(names) == 1 {
		print_check(fmt.Sprintf("%s account: %s", label, names[0]))
		return names[0], nil
	}

	print_step(fmt.Sprintf("Select %s account", label))
	selector := promptui.Select{
		Label: fmt.Sprintf("%s account", label),
		Items: names,
		Size:  len(names),
	}
	_, selected, err := selector.Run()
	if err != nil {
		return "", fmt.Errorf("account selection cancelled")
	}
	return selected, nil
}

func projectTypeLabel(p SyncProject) string {
	if p.ProjectType == "" {
		return "general"
	}
	return p.ProjectType
}

func selectSyncProject(baseClient *SyncGraphQLClient, side string) (SyncProject, error) {
	projects, err := baseClient.ListProjects()
	if err != nil {
		return SyncProject{}, fmt.Errorf("list projects (%s): %w", side, err)
	}
	if len(projects) == 0 {
		return SyncProject{}, fmt.Errorf("no projects available on %s account — ensure the access token includes project_ids (Console → Access Token) and restart the engine if you upgraded recently", side)
	}

	if len(projects) == 1 {
		p := projects[0]
		print_check(fmt.Sprintf("%s project: %s [%s] (%s)", side, p.Name, projectTypeLabel(p), p.ID))
		profile, err := baseClient.WithProject(p.ID).CurrentProject()
		return validateSelectedProjectAccess(side, p, profile, err)
	}

	options := make([]string, len(projects))
	byOption := make(map[string]SyncProject, len(projects))
	for i, p := range projects {
		label := fmt.Sprintf("%s [%s] (%s)", p.Name, projectTypeLabel(p), p.ID)
		options[i] = label
		byOption[label] = p
	}

	print_step(fmt.Sprintf("Select %s project", side))
	selector := promptui.Select{
		Label: fmt.Sprintf("%s project", side),
		Items: options,
		Size:  min(len(options), 12),
	}
	_, picked, err := selector.Run()
	if err != nil {
		return SyncProject{}, fmt.Errorf("project selection cancelled")
	}

	selected := byOption[picked]
	profile, err := baseClient.WithProject(selected.ID).CurrentProject()
	return validateSelectedProjectAccess(side, selected, profile, err)
}

func selectSyncType() (string, error) {
	options := []string{"schema", "functions", "content"}
	print_step("What do you want to sync?")
	selector := promptui.Select{
		Label: "Sync type",
		Items: options,
		Size:  len(options),
	}
	_, picked, err := selector.Run()
	if err != nil {
		return "", err
	}
	return picked, nil
}

// validateSelectedProjectAccess ensures the access token can operate on the picked project.
// listProjects returns every project the user belongs to; sync tokens only honor
// X-Apito-Project-Id when that id is in the token's project_ids claim.
func validateSelectedProjectAccess(side string, selected SyncProject, current *SyncProject, currentErr error) (SyncProject, error) {
	if currentErr != nil {
		return SyncProject{}, fmt.Errorf(
			"cannot load %s project %q (%s) with this access token: %w\n\n"+
				"Regenerate the token in Console → Access Token, include this project in project_ids, "+
				"update the account key (apito config set account <name> key <token>), and retry",
			side, selected.Name, selected.ID, currentErr,
		)
	}
	if current == nil || strings.TrimSpace(current.ID) == "" {
		return SyncProject{}, fmt.Errorf(
			"cannot resolve %s project %q (%s): currentProject returned empty data\n\n"+
				"Regenerate the access token with this project in project_ids and update the account key",
			side, selected.Name, selected.ID,
		)
	}
	if strings.TrimSpace(current.ID) != strings.TrimSpace(selected.ID) {
		return SyncProject{}, fmt.Errorf(
			"access token for %s account does not include project %q (%s)\n\n"+
				"The token resolved currentProject to %q (%s) instead. "+
				"listProjects lists every project you belong to, but sync only works for projects "+
				"listed in the token's project_ids.\n\n"+
				"Regenerate the token in Console → Access Token, add %s to project_ids, "+
				"update the account key, and retry",
			side,
			selected.Name, selected.ID,
			current.Name, current.ID,
			selected.ID,
		)
	}
	return *current, nil
}

func validateProjectProfiles(from, to SyncProject) error {
	fromKey := projectSyncProfileKey(from)
	toKey := projectSyncProfileKey(to)
	if fromKey != toKey {
		return fmt.Errorf("project type mismatch: source is %q but destination is %q (must match: general, saas-shared, saas-per-tenant)", fromKey, toKey)
	}
	return nil
}
