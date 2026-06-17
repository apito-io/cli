package main

import (
	"fmt"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	syncFromAccount string
	syncToAccount   string
	syncType        string
	syncDryRun      bool
	syncYes         bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync schema or content between Apito accounts/projects",
	Long: `Sync schema (models, fields, relations) or content between two configured accounts.

Uses the system GraphQL API (/system/graphql) with access tokens (cli-/mcp-/sdk-).
Schema changes are staged as drafts on pro engines — publish from Console when ready.`,
	RunE: runSyncCommand,
}

func init() {
	syncCmd.Flags().StringVar(&syncFromAccount, "from", "", "Source account name")
	syncCmd.Flags().StringVar(&syncToAccount, "to", "", "Destination account name")
	syncCmd.Flags().StringVar(&syncType, "type", "", "Sync type: schema or content")
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show planned changes without writing")
	syncCmd.Flags().BoolVar(&syncYes, "yes", false, "Skip confirmation prompts")
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

	fromName, err := resolveSyncAccount("FROM (source)", syncFromAccount)
	if err != nil {
		return err
	}
	toName, err := resolveSyncAccount("TO (destination)", syncToAccount)
	if err != nil {
		return err
	}

	fromCfg, err := getAccountConfig(fromName)
	if err != nil {
		return err
	}
	toCfg, err := getAccountConfig(toName)
	if err != nil {
		return err
	}

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	fromBase := newSyncGraphQLClient(fromCfg.ServerURL, fromCfg.CloudSyncKey, timeout)
	toBase := newSyncGraphQLClient(toCfg.ServerURL, toCfg.CloudSyncKey, timeout)

	print_step(fmt.Sprintf("Source account: %s (%s)", fromName, fromCfg.ServerURL))
	print_step(fmt.Sprintf("Destination account: %s (%s)", toName, toCfg.ServerURL))

	fromProject, err := selectSyncProject(fromBase, "source")
	if err != nil {
		return err
	}
	toProject, err := selectSyncProject(toBase, "destination")
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
	case "content":
		return runContentSync(fromClient, toClient, fromProject, toProject, syncDryRun, syncYes)
	default:
		return fmt.Errorf("invalid --type %q (use schema or content)", syncType)
	}
}

func resolveSyncAccount(label, flagValue string) (string, error) {
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
		print_status(fmt.Sprintf("Using only account for %s: %s", label, names[0]))
		return names[0], nil
	}

	print_step(fmt.Sprintf("Select %s account", label))
	selector := promptui.Select{
		Label: fmt.Sprintf("Account (%s)", label),
		Items: names,
		Size:  len(names),
	}
	_, selected, err := selector.Run()
	if err != nil {
		return "", fmt.Errorf("account selection cancelled")
	}
	return selected, nil
}

func selectSyncProject(baseClient *SyncGraphQLClient, side string) (SyncProject, error) {
	projects, err := baseClient.ListProjects()
	if err != nil {
		return SyncProject{}, fmt.Errorf("list projects (%s): %w", side, err)
	}
	if len(projects) == 0 {
		return SyncProject{}, fmt.Errorf("no projects available on %s account — ensure the access token includes project_ids (Console → Access Token) and restart the engine if you upgraded recently", side)
	}

	options := make([]string, len(projects))
	byOption := make(map[string]SyncProject, len(projects))
	for i, p := range projects {
		pt := p.ProjectType
		if pt == "" {
			pt = "general"
		}
		label := fmt.Sprintf("%s [%s] (%s)", p.Name, pt, p.ID)
		options[i] = label
		byOption[label] = p
	}

	var picked string
	prompt := &survey.Select{
		Message: fmt.Sprintf("Select %s project", side),
		Options: options,
	}
	if err := survey.AskOne(prompt, &picked); err != nil {
		return SyncProject{}, fmt.Errorf("project selection cancelled")
	}

	selected := byOption[picked]
	profile, err := baseClient.WithProject(selected.ID).CurrentProject()
	if err != nil {
		print_warning("Could not load extended project metadata; using listProjects fields.")
		return selected, nil
	}
	return *profile, nil
}

func validateProjectProfiles(from, to SyncProject) error {
	fromKey := projectSyncProfileKey(from)
	toKey := projectSyncProfileKey(to)
	if fromKey != toKey {
		return fmt.Errorf("project type mismatch: source is %q but destination is %q (must match: general, saas-shared, saas-per-tenant)", fromKey, toKey)
	}
	return nil
}

func selectSyncType() (string, error) {
	options := []string{"schema", "content"}
	var picked string
	if err := survey.AskOne(&survey.Select{
		Message: "What do you want to sync?",
		Options: options,
	}, &picked); err != nil {
		return "", err
	}
	return picked, nil
}
