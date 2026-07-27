package main

import (
	"fmt"
	"strings"
)

func runSchemaSync(fromClient, toClient *SyncGraphQLClient, endpoints SyncEndpoints, dryRun, autoYes, includeDeletes bool) error {
	print_step("Loading schema from source and destination...")
	sourceModels, err := fromClient.ProjectModelsInfo("")
	if err != nil {
		return fmt.Errorf("source schema (%s): %w", endpoints.FromURL, err)
	}

	if issues := validateSyncModels(sourceModels); len(issues) > 0 {
		print_error("Source schema validation failed.")
		fmt.Println(formatSchemaValidationReport(issues))
		return fmt.Errorf("source schema has %d validation issue(s); fix schema before sync", len(issues))
	}
	print_status("Source schema validation passed.")

	destCtx, err := loadDestinationSchema(toClient)
	if err != nil {
		return fmt.Errorf("destination schema (%s): %w", endpoints.ToURL, err)
	}

	additiveDiffs := computeSchemaDiff(sourceModels, destCtx.Effective)
	deleteDiffs := computeSchemaDeleteDiff(sourceModels, destCtx.Effective)
	additiveChanges := flattenSchemaChanges(additiveDiffs)
	deleteChanges := flattenSchemaChanges(deleteDiffs)

	if len(additiveChanges) == 0 && len(deleteChanges) == 0 {
		print_success("Schemas are already in sync.")
		return nil
	}

	wantDeletes := includeDeletes
	if len(deleteChanges) > 0 && !includeDeletes {
		if autoYes {
			// --yes never applies destructive deletes unless --include-deletes is set.
			print_warning(fmt.Sprintf(
				"Found %d destination-only field(s) that would be deleted; skipped (pass --include-deletes to include).",
				len(deleteChanges),
			))
			wantDeletes = false
		} else {
			choice, err := selectSchemaDiffScope(len(additiveChanges), len(deleteChanges))
			if err != nil {
				return err
			}
			switch choice {
			case schemaDiffScopeCancel:
				print_warning("Schema sync cancelled.")
				return nil
			case schemaDiffScopeFull:
				wantDeletes = true
			case schemaDiffScopeDeletesOnly:
				wantDeletes = true
				additiveDiffs = nil
				additiveChanges = nil
			case schemaDiffScopeAdditive:
				wantDeletes = false
			}
		}
	}

	var diffs []ModelSchemaDiff
	if wantDeletes {
		diffs = mergeSchemaDiffs(additiveDiffs, deleteDiffs)
	} else {
		diffs = additiveDiffs
		if len(deleteChanges) > 0 {
			print_status(fmt.Sprintf(
				"Skipped %d destination-only field delete(s). Choose full diff next time or pass --include-deletes.",
				len(deleteChanges),
			))
		}
	}

	allChanges := flattenSchemaChanges(diffs)
	if len(allChanges) == 0 {
		print_success("Nothing to apply with the selected scope.")
		return nil
	}

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

	selected, err = closeFieldDependencies(selected, allChanges)
	if err != nil {
		return err
	}
	if err := preflightNestedAddOrder(selected); err != nil {
		return err
	}

	if _, deletes := partitionSchemaChanges(selected); len(deletes) > 0 {
		print_warning(fmt.Sprintf(
			"%d selected change(s) delete fields on the destination (staged until Console publish).",
			len(deletes),
		))
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
	if err := preflightNestedAddOrder(selectedFromTasks(tasks)); err != nil {
		return err
	}
	results := runSyncTasks(toClient, endpoints, tasks, srcMap, effectiveModelNames(destCtx.Effective))
	return summarizeSyncResults(endpoints, results)
}

func selectedFromTasks(tasks []SyncTask) []SchemaChange {
	out := make([]SchemaChange, len(tasks))
	for i, t := range tasks {
		out[i] = t.Change
	}
	return out
}

func printPublishReminder() {
	fmt.Println()
	print_warning("Schema changes are staged as a draft — NOT published yet.")
	print_status("Review: Apito Console → Project Settings → Schema Changes")
	print_status("Confirm pending ops match the apply summary, then Publish manually.")
	print_status("Public API / tables update only after publish. CLI never auto-publishes.")
	print_status("After publish: re-run apito sync (dry) to confirm zero remaining drift.")
}
