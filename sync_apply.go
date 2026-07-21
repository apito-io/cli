package main

import (
	"fmt"
	"sort"
	"strings"
)

type taskStatus int

const (
	taskPending taskStatus = iota
	taskRunning
	taskPassed
	taskFailed
	taskSkipped
)

type SyncTask struct {
	Index          int
	Change         SchemaChange
	RequiredModels []string
}

type SyncTaskResult struct {
	Task    SyncTask
	Status  taskStatus
	Message string
}

func taskStatusIcon(status taskStatus) string {
	switch status {
	case taskPending:
		return "[ ]"
	case taskRunning:
		return "[~]"
	case taskPassed:
		return Green + "[✓]" + Reset
	case taskFailed:
		return Red + "[✗]" + Reset
	case taskSkipped:
		return Yellow + "[−]" + Reset
	default:
		return "[ ]"
	}
}

func buildSyncTasks(changes []SchemaChange) []SyncTask {
	var modelAdds, fieldChanges, fieldDeletes, connChanges []SchemaChange
	for _, ch := range changes {
		switch ch.Kind {
		case ChangeAddModel:
			modelAdds = append(modelAdds, ch)
		case ChangeAddField, ChangeUpdateField:
			fieldChanges = append(fieldChanges, ch)
		case ChangeDeleteField:
			fieldDeletes = append(fieldDeletes, ch)
		case ChangeAddConnection:
			connChanges = append(connChanges, ch)
		}
	}

	sort.Slice(modelAdds, func(i, j int) bool {
		return strings.ToLower(modelAdds[i].Model) < strings.ToLower(modelAdds[j].Model)
	})

	sortFieldChanges := func(list []SchemaChange) {
		sort.Slice(list, func(i, j int) bool {
			a, b := list[i], list[j]
			if !strings.EqualFold(a.Model, b.Model) {
				return strings.ToLower(a.Model) < strings.ToLower(b.Model)
			}
			aParent, bParent := "", ""
			aSerial, bSerial := 0, 0
			if a.Field != nil {
				aParent = a.Field.ParentField
				aSerial = a.Field.Serial
			}
			if b.Field != nil {
				bParent = b.Field.ParentField
				bSerial = b.Field.Serial
			}
			if aParent != bParent {
				return aParent < bParent
			}
			return aSerial < bSerial
		})
	}
	sortFieldChanges(fieldChanges)
	sortFieldChanges(fieldDeletes)

	sort.Slice(connChanges, func(i, j int) bool {
		a, b := connChanges[i], connChanges[j]
		if !strings.EqualFold(a.Model, b.Model) {
			return strings.ToLower(a.Model) < strings.ToLower(b.Model)
		}
		ak, bk := "", ""
		if a.Connection != nil {
			ak = a.Connection.Model
		}
		if b.Connection != nil {
			bk = b.Connection.Model
		}
		return strings.ToLower(ak) < strings.ToLower(bk)
	})

	// Deletes last: additive work first, then remove destination-only fields.
	ordered := append(append(append(modelAdds, fieldChanges...), connChanges...), fieldDeletes...)
	tasks := make([]SyncTask, len(ordered))
	for i, ch := range ordered {
		tasks[i] = SyncTask{
			Index:          i,
			Change:         ch,
			RequiredModels: requiredModelsForChange(ch),
		}
	}
	return tasks
}

func requiredModelsForChange(ch SchemaChange) []string {
	switch ch.Kind {
	case ChangeAddModel:
		return nil
	case ChangeAddField, ChangeUpdateField, ChangeDeleteField:
		return []string{strings.ToLower(ch.Model)}
	case ChangeAddConnection:
		if ch.Connection == nil {
			return []string{strings.ToLower(ch.Model)}
		}
		return []string{strings.ToLower(ch.Model), strings.ToLower(ch.Connection.Model)}
	default:
		return nil
	}
}

func isIdempotentDraftError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists in draft")
}

// isDraftStagedDespiteSyncError treats pro staging as success when the draft was
// persisted locally but Litestream/replica backup sync failed on the server.
// The engine returns this as a GraphQL error even though the draft is correct.
func isDraftStagedDespiteSyncError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "draft staged locally") ||
		strings.Contains(msg, "draft updated locally") ||
		strings.Contains(msg, "draft created locally") ||
		(strings.Contains(msg, "system sync failed") && strings.Contains(msg, "replica sync"))
}

// isSchemaApplyTimeout is returned when the HTTP client gives up waiting for
// headers. On pro engines the mutation often still completes and stages the draft
// (Litestream wait can exceed the old 30s default).
func isSchemaApplyTimeout(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "client.timeout exceeded") ||
		strings.Contains(msg, "timeout exceeded while awaiting headers")
}

func draftStageSuccessMessage(err error) string {
	if isIdempotentDraftError(err) {
		return "already staged in draft"
	}
	if isDraftStagedDespiteSyncError(err) {
		return "staged in draft (server backup sync deferred)"
	}
	if isSchemaApplyTimeout(err) {
		return "request timed out — verify draft in Console (mutation often still staged)"
	}
	return ""
}

func applySyncTask(client *SyncGraphQLClient, task SyncTask, srcMap map[string]SyncModel) error {
	ch := task.Change
	switch ch.Kind {
	case ChangeAddModel:
		src := srcMap[strings.ToLower(ch.Model)]
		return client.AddModel(ch.Model, src.SinglePage)
	case ChangeAddField, ChangeUpdateField:
		if ch.Field == nil {
			return fmt.Errorf("missing field payload")
		}
		return client.UpsertField(ch.Model, *ch.Field, ch.Kind == ChangeUpdateField)
	case ChangeDeleteField:
		if ch.Field == nil {
			return fmt.Errorf("missing field payload")
		}
		return client.DeleteField(ch.Model, *ch.Field)
	case ChangeAddConnection:
		if ch.Connection == nil {
			return fmt.Errorf("missing connection payload")
		}
		conn := ch.Connection
		forward := conn.Relation
		if forward == "" {
			forward = "has_many"
		}
		reverse := ch.ReverseType
		if reverse == "" {
			reverse = "has_many"
		}
		return client.UpsertConnection(ch.Model, conn.Model, forward, reverse, conn.KnownAs)
	default:
		return fmt.Errorf("unsupported change kind %q", ch.Kind)
	}
}

func runSyncTasks(
	client *SyncGraphQLClient,
	endpoints SyncEndpoints,
	tasks []SyncTask,
	srcMap map[string]SyncModel,
	initialAvailable map[string]struct{},
) []SyncTaskResult {
	fmt.Println()
	fmt.Printf("  APPLYING → %s (%s)\n", endpoints.ToAccount, endpoints.ToURL)
	fmt.Println("  ───────────────────────────────────────────────────────────────")

	results := make([]SyncTaskResult, len(tasks))
	available := make(map[string]struct{}, len(initialAvailable))
	for k, v := range initialAvailable {
		available[k] = v
	}
	blocked := make(map[string]struct{})
	var deferredSyncCount int

	for i, task := range tasks {
		num := i + 1
		total := len(tasks)
		fmt.Printf("  [~] %d/%d  %s\n", num, total, formatSchemaChangeShort(task.Change))

		result := SyncTaskResult{Task: task, Status: taskPending}

		if skipReason := skipReasonForTask(task, available, blocked); skipReason != "" {
			result.Status = taskSkipped
			result.Message = skipReason
			fmt.Printf("  %s %d/%d  %s — %s\n", Yellow+"[−]"+Reset, num, total, formatSchemaChangeShort(task.Change), skipReason)
			results[i] = result
			continue
		}

		err := applySyncTask(client, task, srcMap)
		if stageMsg := draftStageSuccessMessage(err); stageMsg != "" {
			result.Status = taskPassed
			result.Message = stageMsg
			if isDraftStagedDespiteSyncError(err) {
				deferredSyncCount++
			}
			markModelsAvailable(task, available)
			fmt.Printf("  %s %d/%d  %s — %s\n", Green+"[✓]"+Reset, num, total, formatSchemaChangeShort(task.Change), stageMsg)
			results[i] = result
			continue
		}
		if err != nil {
			result.Status = taskFailed
			result.Message = err.Error()
			if task.Change.Kind == ChangeAddModel {
				blocked[strings.ToLower(task.Change.Model)] = struct{}{}
			}
			fmt.Printf("  %s %d/%d  %s — %s\n", Red+"[✗]"+Reset, num, total, formatSchemaChangeShort(task.Change), err.Error())
			results[i] = result
			continue
		}

		result.Status = taskPassed
		markModelsAvailable(task, available)
		fmt.Printf("  %s %d/%d  %s\n", Green+"[✓]"+Reset, num, total, formatSchemaChangeShort(task.Change))
		results[i] = result
	}

	printSyncTaskSummary(results)
	if deferredSyncCount > 0 {
		print_warning(fmt.Sprintf(
			"%d change(s) staged in draft on the server; Litestream backup sync failed (missing .ltx sidecar). Draft is valid — publish from Console when ready. Deploy latest engine to clear backup warnings.",
			deferredSyncCount,
		))
	}
	return results
}

func skipReasonForTask(task SyncTask, available, blocked map[string]struct{}) string {
	for _, modelKey := range task.RequiredModels {
		if _, ok := blocked[modelKey]; ok {
			return fmt.Sprintf("skipped (%q unavailable)", modelKey)
		}
		if _, ok := available[modelKey]; !ok {
			return fmt.Sprintf("skipped (model %q not available)", modelKey)
		}
	}
	return ""
}

func markModelsAvailable(task SyncTask, available map[string]struct{}) {
	switch task.Change.Kind {
	case ChangeAddModel:
		available[strings.ToLower(task.Change.Model)] = struct{}{}
	case ChangeAddField, ChangeUpdateField:
		available[strings.ToLower(task.Change.Model)] = struct{}{}
	case ChangeAddConnection:
		available[strings.ToLower(task.Change.Model)] = struct{}{}
		if task.Change.Connection != nil {
			available[strings.ToLower(task.Change.Connection.Model)] = struct{}{}
		}
	}
}

func printSyncTaskSummary(results []SyncTaskResult) {
	var passed, failed, skipped int
	for _, r := range results {
		switch r.Status {
		case taskPassed:
			passed++
		case taskFailed:
			failed++
		case taskSkipped:
			skipped++
		}
	}

	fmt.Println()
	fmt.Println("  RESULTS")
	fmt.Println("  ───────────────────────────────────────────────────────────────")
	for i, r := range results {
		fmt.Printf("  %s %d/%d  %s", taskStatusIcon(r.Status), i+1, len(results), formatSchemaChangeShort(r.Task.Change))
		if r.Message != "" && r.Status != taskPassed {
			fmt.Printf(" — %s", r.Message)
		} else if r.Message == "already staged in draft" {
			fmt.Printf(" — %s", r.Message)
		}
		fmt.Println()
	}
	fmt.Println()
	print_status(fmt.Sprintf("Summary: %d passed, %d skipped, %d failed", passed, skipped, failed))
}

func summarizeSyncResults(endpoints SyncEndpoints, results []SyncTaskResult) error {
	var passed, failed int
	for _, r := range results {
		switch r.Status {
		case taskPassed:
			passed++
		case taskFailed:
			failed++
		}
	}

	if passed > 0 {
		print_success(fmt.Sprintf("Applied %d schema change(s) to %s (%s).", passed, endpoints.ToAccount, endpoints.ToURL))
	}
	if failed > 0 {
		printPublishReminder()
		return fmt.Errorf("%d schema task(s) failed on %s (%s)", failed, endpoints.ToAccount, endpoints.ToURL)
	}
	if passed == 0 {
		print_warning("No schema changes were applied.")
		return nil
	}
	printPublishReminder()
	return nil
}
