package main

import (
	"fmt"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
)

// functionEndpoint is a source/destination for function sync — either a remote
// project (GraphQL) or a local directory.
type functionEndpoint struct {
	isLocal bool
	dir     string
	client  *SyncGraphQLClient
	label   string
}

func (e functionEndpoint) load() ([]SyncFunction, error) {
	if e.isLocal {
		return readLocalFunctions(e.dir)
	}
	return e.client.ProjectFunctionsInfo()
}

// loadDestForDiff loads existing destination functions for diffing. A missing
// local directory is treated as empty (first export).
func (e functionEndpoint) loadDestForDiff() ([]SyncFunction, error) {
	if e.isLocal {
		fns, err := readLocalFunctions(e.dir)
		if err != nil {
			// A not-yet-created dir just means an empty destination.
			return nil, nil
		}
		return fns, nil
	}
	return e.client.ProjectFunctionsInfo()
}

// runFunctionSync handles project → project function transfer.
func runFunctionSync(fromClient, toClient *SyncGraphQLClient, endpoints SyncEndpoints, deploy, includeSecrets, dryRun, autoYes bool) error {
	from := functionEndpoint{client: fromClient, label: fmt.Sprintf("%s / %s", endpoints.FromAccount, endpoints.FromProject.Name)}
	to := functionEndpoint{client: toClient, label: fmt.Sprintf("%s / %s", endpoints.ToAccount, endpoints.ToProject.Name)}
	return runFunctionSyncEndpoints(from, to, deploy, includeSecrets, dryRun, autoYes)
}

// runFunctionSyncEndpoints is the shared engine for all three directions
// (project→project, project→local, local→project).
func runFunctionSyncEndpoints(from, to functionEndpoint, deploy, includeSecrets, dryRun, autoYes bool) error {
	print_step(fmt.Sprintf("Loading functions from source (%s)...", from.label))
	sourceFns, err := from.load()
	if err != nil {
		return fmt.Errorf("load source functions: %w", err)
	}
	if len(sourceFns) == 0 {
		print_warning("No functions found on source.")
		return nil
	}

	destFns, err := to.loadDestForDiff()
	if err != nil {
		return fmt.Errorf("load destination functions: %w", err)
	}

	changes := actionableFunctionChanges(computeFunctionDiff(sourceFns, destFns))
	if len(changes) == 0 {
		print_success("Functions are already in sync.")
		return nil
	}

	printFunctionSyncPlan(from, to, changes, deploy)

	selected, err := selectFunctionChanges(changes, autoYes)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		print_warning("No functions selected.")
		return nil
	}

	if dryRun {
		print_success(fmt.Sprintf("Dry run: would sync %d function(s); no writes performed.", len(selected)))
		return nil
	}

	if !autoYes {
		confirm := false
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("Sync %d function(s) to %s?", len(selected), to.label),
			Default: false,
		}, &confirm); err != nil {
			return err
		}
		if !confirm {
			print_warning("Function sync cancelled.")
			return nil
		}
	}

	applied := 0
	for _, ch := range selected {
		if to.isLocal {
			if err := writeLocalFunction(to.dir, ch.Source, includeSecrets); err != nil {
				return fmt.Errorf("write function %q: %w", ch.Name, err)
			}
			print_success(fmt.Sprintf("Exported %q → %s", ch.Name, to.dir))
			applied++
			continue
		}

		if ch.Kind == FunctionDeploy {
			dep, err := to.client.DeployFunction(ch.Name, "")
			if err != nil {
				return fmt.Errorf("deploy function %q: %w", ch.Name, err)
			}
			print_success(fmt.Sprintf("Deployed %q → active_revision_id=%s", ch.Name, dep.ActiveRevisionID))
			applied++
			continue
		}

		fn := ch.Source
		if !includeSecrets {
			// Let the destination engine mint its own callable secret.
			fn.RestAPISecretURLKey = ""
		}
		result, err := to.client.UpsertFunction(fn, ch.DestExists)
		if err != nil {
			return fmt.Errorf("upsert function %q: %w", ch.Name, err)
		}
		print_success(fmt.Sprintf("Upserted %q (%s)", ch.Name, ch.Kind))
		applied++

		shouldDeploy := deploy || strings.TrimSpace(ch.Source.ActiveRevisionID) != ""
		if shouldDeploy {
			dep, err := to.client.DeployFunction(ch.Name, "")
			if err != nil {
				return fmt.Errorf("deploy function %q: %w", ch.Name, err)
			}
			print_status(fmt.Sprintf("Deployed %q → active_revision_id=%s", ch.Name, dep.ActiveRevisionID))
		} else if result != nil && result.ActiveRevisionID == "" {
			print_status(fmt.Sprintf("Draft saved for %q (not deployed — pass --deploy to publish a revision)", ch.Name))
		}
	}

	if !to.isLocal {
		printFunctionTenantReminder()
	}
	print_success(fmt.Sprintf("Function sync complete: %d function(s).", applied))
	return nil
}

// runLocalFunctionSync handles a sync where one side is the special "filesystem" account.
func runLocalFunctionSync(fromName, toName string, timeout int, deploy, includeSecrets, dryRun, autoYes bool, preselectedRemote *SyncProject) error {
	if isFilesystemSyncSide(fromName) && isFilesystemSyncSide(toName) {
		return fmt.Errorf("both sides are %q — at least one side must be a remote account", filesystemAccountName)
	}
	dir, err := resolveFunctionsSyncDir()
	if err != nil {
		return err
	}

	remoteSide := "destination"
	remoteName := toName
	if isFilesystemSyncSide(toName) {
		remoteSide = "source"
		remoteName = fromName
	}

	remoteCfg, err := getAccountConfig(remoteName)
	if err != nil {
		return err
	}
	base := newSyncGraphQLClient(remoteCfg.ServerURL, remoteCfg.CloudSyncKey, timeout)
	var project SyncProject
	if preselectedRemote != nil && strings.TrimSpace(preselectedRemote.ID) != "" {
		project = *preselectedRemote
		print_check(fmt.Sprintf("%s project: %s [%s] (%s)", remoteSide, project.Name, projectTypeLabel(project), project.ID))
	} else {
		project, err = selectSyncProject(base, remoteSide)
		if err != nil {
			return err
		}
	}
	remoteClient := base.WithProject(project.ID)
	remoteEndpoint := functionEndpoint{
		client: remoteClient,
		label:  fmt.Sprintf("%s / %s (%s)", remoteName, project.Name, project.ID),
	}
	localEndpoint := functionEndpoint{isLocal: true, dir: dir, label: fmt.Sprintf("filesystem (%s)", dir)}

	if isFilesystemSyncSide(fromName) {
		// filesystem → project (import); --deploy applies to the destination engine.
		print_status(fmt.Sprintf("Importing functions: %s -> %s", localEndpoint.label, remoteEndpoint.label))
		return runFunctionSyncEndpoints(localEndpoint, remoteEndpoint, deploy, includeSecrets, dryRun, autoYes)
	}
	// project → filesystem (export); deploy is not applicable to a directory.
	print_status(fmt.Sprintf("Exporting functions: %s -> %s", remoteEndpoint.label, localEndpoint.label))
	return runFunctionSyncEndpoints(remoteEndpoint, localEndpoint, false, includeSecrets, dryRun, autoYes)
}

func printFunctionSyncPlan(from, to functionEndpoint, changes []FunctionChange, deploy bool) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  FUNCTION SYNC PLAN")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  FROM  %s\n", from.label)
	fmt.Printf("  TO    %s\n", to.label)
	if deploy && !to.isLocal {
		print_status("  --deploy: each upserted function will publish a new active revision")
	}
	print_status("  Published source functions also deploy on destination when live hash drifts")
	fmt.Printf("  CHANGES (%d)\n", len(changes))
	fmt.Println("  ───────────────────────────────────────────────────────────────")
	for i, ch := range changes {
		fmt.Printf("  [%d] %s\n", i+1, ch.Summary)
	}
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

func selectFunctionChanges(changes []FunctionChange, autoYes bool) ([]FunctionChange, error) {
	if autoYes {
		return changes, nil
	}
	options := make([]string, len(changes))
	optionMap := make(map[string]FunctionChange, len(changes))
	for i, ch := range changes {
		label := fmt.Sprintf("[%d] %s", i+1, ch.Summary)
		options[i] = label
		optionMap[label] = ch
	}
	var picked []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message:  "Select functions to sync",
		Options:  options,
		PageSize: 15,
	}, &picked); err != nil {
		return nil, fmt.Errorf("selection cancelled: %w", err)
	}
	selected := make([]FunctionChange, 0, len(picked))
	for _, label := range picked {
		if ch, ok := optionMap[label]; ok {
			selected = append(selected, ch)
		}
	}
	return selected, nil
}

func printFunctionTenantReminder() {
	print_status("Note: function definitions are tenant-agnostic. For SaaS live invoke, use an app-user JWT (tenant from claims). Admin draft test may pass tenant_id.")
}
