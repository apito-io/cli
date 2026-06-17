package main

import (
	"fmt"
	"strings"
)

func runSchemaSync(fromClient, toClient *SyncGraphQLClient, endpoints SyncEndpoints, dryRun, autoYes bool) error {
	print_step("Loading schema from source and destination...")
	sourceModels, err := fromClient.ProjectModelsInfo("")
	if err != nil {
		return fmt.Errorf("source schema (%s): %w", endpoints.FromURL, err)
	}

	destCtx, err := loadDestinationSchema(toClient)
	if err != nil {
		return fmt.Errorf("destination schema (%s): %w", endpoints.ToURL, err)
	}

	diffs := computeSchemaDiff(sourceModels, destCtx.Effective)
	if len(diffs) == 0 {
		print_success("Schemas are already in sync (add/update scope).")
		return nil
	}

	allChanges := flattenSchemaChanges(diffs)
	printSchemaSyncPlan(endpoints, diffs, &destCtx)
	print_status(fmt.Sprintf("Found %d change(s) across %d model(s).", len(allChanges), len(diffs)))

	selected, err := selectSchemaChanges(allChanges, autoYes)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		print_warning("No schema changes selected.")
		return nil
	}

	if dryRun {
		printApplyConfirmationBlock(endpoints, selected)
		print_success(fmt.Sprintf("Dry run: would apply %d schema change(s).", len(selected)))
		return nil
	}

	if !autoYes {
		confirmed, err := confirmApplyToDestination(endpoints, selected)
		if err != nil {
			return err
		}
		if !confirmed {
			print_warning("Schema sync cancelled.")
			return nil
		}
	}

	srcMap := make(map[string]SyncModel)
	for _, m := range sourceModels {
		srcMap[strings.ToLower(m.Name)] = m
	}

	tasks := buildSyncTasks(selected)
	results := runSyncTasks(toClient, endpoints, tasks, srcMap, effectiveModelNames(destCtx.Effective))
	return summarizeSyncResults(endpoints, results)
}

func printPublishReminder() {
	fmt.Println()
	print_warning("Schema changes are staged as a draft on pro engines.")
	print_status("Open Apito Console → Project Settings → Schema Changes → review the timeline → Publish manually.")
	print_status("Public API and physical tables update only after publish.")
}
