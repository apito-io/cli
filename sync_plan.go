package main

import (
	"fmt"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
)

type SyncEndpoints struct {
	FromAccount string
	ToAccount   string
	FromURL     string
	ToURL       string
	FromProject SyncProject
	ToProject   SyncProject
}

func (e SyncEndpoints) fromProfile() string {
	return projectSyncProfileKey(e.FromProject)
}

func (e SyncEndpoints) toProfile() string {
	return projectSyncProfileKey(e.ToProject)
}

func flattenSchemaChanges(diffs []ModelSchemaDiff) []SchemaChange {
	var out []SchemaChange
	for _, md := range diffs {
		out = append(out, md.Changes...)
	}
	return out
}

func formatSchemaChangeDetail(ch SchemaChange) string {
	switch ch.Kind {
	case ChangeAddModel:
		return fmt.Sprintf("Add model %q", ch.Model)
	case ChangeAddField:
		if ch.Field == nil {
			return ch.Summary
		}
		f := ch.Field
		typeLabel := f.FieldType
		if f.FieldSubType != "" {
			typeLabel = f.FieldType + "/" + f.FieldSubType
		}
		indent := strings.Repeat("  ", fieldPathDepth(*f))
		path := fieldSyncKey(*f)
		label := f.Label
		if strings.TrimSpace(label) == "" {
			label = f.Identifier
		}
		base := fmt.Sprintf("%sAdd field %q (%s, %s) on %q [%s]", indent, label, f.Identifier, typeLabel, ch.Model, path)
		if strings.Contains(ch.Summary, "required by") {
			return base + " [required ancestor]"
		}
		return base
	case ChangeUpdateField:
		if ch.Field == nil {
			return ch.Summary
		}
		label := ch.Field.Label
		if strings.TrimSpace(label) == "" {
			label = ch.Field.Identifier
		}
		indent := strings.Repeat("  ", fieldPathDepth(*ch.Field))
		path := fieldSyncKey(*ch.Field)
		return fmt.Sprintf("%sUpdate field %q (%s) on %q [%s]", indent, label, ch.Field.Identifier, ch.Model, path)
	case ChangeDeleteField:
		if ch.Field == nil {
			return ch.Summary
		}
		label := ch.Field.Label
		if label == "" {
			label = ch.Field.Identifier
		}
		indent := strings.Repeat("  ", fieldPathDepth(*ch.Field))
		path := fieldSyncKey(*ch.Field)
		return fmt.Sprintf("%sDelete field %q (%s) on %q [%s]", indent, label, ch.Field.Identifier, ch.Model, path)
	case ChangeAddConnection:
		if ch.Connection == nil {
			return ch.Summary
		}
		c := ch.Connection
		forward := c.Relation
		if forward == "" {
			forward = "has_many"
		}
		reverse := ch.ReverseType
		if reverse == "" {
			reverse = "has_many"
		}
		knownAs := c.KnownAs
		if knownAs == "" {
			knownAs = c.Model
		}
		return fmt.Sprintf("Add relation %q → %q (%s ↔ %s, known_as: %q) on %q", ch.Model, c.Model, forward, reverse, knownAs, ch.Model)
	case ChangeUpdateConnection:
		if ch.Connection == nil {
			return ch.Summary
		}
		c := ch.Connection
		forward := c.Relation
		if forward == "" {
			forward = "has_many"
		}
		reverse := ch.ReverseType
		if reverse == "" {
			reverse = "has_many"
		}
		knownAs := c.KnownAs
		if knownAs == "" {
			knownAs = c.Model
		}
		return fmt.Sprintf("Fix relation direction %q → %q (%s ↔ %s, known_as: %q) on %q", ch.Model, c.Model, forward, reverse, knownAs, ch.Model)
	default:
		return ch.Summary
	}
}

func printSchemaSyncPlan(endpoints SyncEndpoints, diffs []ModelSchemaDiff, destCtx *destSchemaContext) {
	all := flattenSchemaChanges(diffs)
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  SCHEMA SYNC PLAN")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("  FROM  %s\n        %s\n        project: %s (%s) [%s]\n",
		endpoints.FromAccount, endpoints.FromURL,
		endpoints.FromProject.Name, endpoints.FromProject.ID, endpoints.fromProfile())
	fmt.Println()
	fmt.Printf("  TO    %s\n        %s\n        project: %s (%s) [%s]\n",
		endpoints.ToAccount, endpoints.ToURL,
		endpoints.ToProject.Name, endpoints.ToProject.ID, endpoints.toProfile())
	if destCtx != nil && destCtx.hasDraftComparison() {
		fmt.Println()
		print_status("Destination has unpublished draft — comparing against live + draft")
		if destCtx.Status != nil && destCtx.Status.PendingOperations > 0 {
			print_status(fmt.Sprintf("Pending draft operations on destination: %d", destCtx.Status.PendingOperations))
		}
	}
	fmt.Println()
	fmt.Printf("  CHANGES (%d)\n", len(all))
	fmt.Println("  ───────────────────────────────────────────────────────────────")
	for i, ch := range all {
		fmt.Printf("  [%d] %s\n", i+1, formatSchemaChangeDetail(ch))
	}
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
}

func (d *destSchemaContext) hasDraftComparison() bool {
	if d == nil {
		return false
	}
	if d.Status != nil && d.Status.HasDraft {
		return true
	}
	return len(d.Draft) > 0
}

func formatSchemaChangeShort(ch SchemaChange) string {
	switch ch.Kind {
	case ChangeAddModel:
		return fmt.Sprintf("%s — add model", ch.Model)
	case ChangeAddField:
		if ch.Field == nil {
			return ch.Summary
		}
		return fmt.Sprintf("%s — add field %s (%s)", ch.Model, ch.Field.Identifier, ch.Field.FieldType)
	case ChangeUpdateField:
		if ch.Field == nil {
			return ch.Summary
		}
		return fmt.Sprintf("%s — update field %s", ch.Model, ch.Field.Identifier)
	case ChangeDeleteField:
		if ch.Field == nil {
			return ch.Summary
		}
		if ch.Field.ParentField != "" {
			return fmt.Sprintf("%s — delete field %s.%s", ch.Model, ch.Field.ParentField, ch.Field.Identifier)
		}
		return fmt.Sprintf("%s — delete field %s", ch.Model, ch.Field.Identifier)
	case ChangeAddConnection:
		if ch.Connection == nil {
			return ch.Summary
		}
		c := ch.Connection
		knownAs := c.KnownAs
		if knownAs == "" {
			knownAs = c.Model
		}
		return fmt.Sprintf("%s — relation → %s (known_as: %s)", ch.Model, c.Model, knownAs)
	case ChangeUpdateConnection:
		if ch.Connection == nil {
			return ch.Summary
		}
		c := ch.Connection
		knownAs := c.KnownAs
		if knownAs == "" {
			knownAs = c.Model
		}
		return fmt.Sprintf("%s — fix relation direction → %s (known_as: %s)", ch.Model, c.Model, knownAs)
	default:
		return ch.Summary
	}
}

func printApplyConfirmationBlock(endpoints SyncEndpoints, changes []SchemaChange) {
	fmt.Println()
	fmt.Println("  APPLY TO DESTINATION")
	fmt.Println("  ───────────────────────────────────────────────────────────────")
	fmt.Printf("  Account:  %s\n", endpoints.ToAccount)
	fmt.Printf("  URL:      %s\n", endpoints.ToURL)
	fmt.Printf("  Project:  %s (%s)\n", endpoints.ToProject.Name, endpoints.ToProject.ID)
	fmt.Printf("  Mode:     staged as draft on pro (%d change(s))\n", len(changes))
	fmt.Println()
	for i, ch := range changes {
		fmt.Printf("    • [%d] %s\n", i+1, formatSchemaChangeDetail(ch))
	}
	fmt.Println()
}

type schemaDiffScope int

const (
	schemaDiffScopeAdditive schemaDiffScope = iota
	schemaDiffScopeFull
	schemaDiffScopeDeletesOnly
	schemaDiffScopeCancel
)

func selectSchemaDiffScope(additiveCount, deleteCount int) (schemaDiffScope, error) {
	fmt.Println()
	print_status(fmt.Sprintf(
		"Diff scope: %d add/update/relation change(s), %d destination-only field delete(s).",
		additiveCount, deleteCount,
	))
	options := []string{
		"Add/update only (skip deletes)",
		"Full diff (include deletes)",
		"Deletes only",
		"Cancel",
	}
	var mode string
	if err := survey.AskOne(&survey.Select{
		Message: "What should the sync plan include?",
		Options: options,
		Default: options[0],
		Help:    "Deletes remove fields that exist only on the destination. They stage until Console publish.",
	}, &mode); err != nil {
		return schemaDiffScopeCancel, fmt.Errorf("selection cancelled: %w", err)
	}
	switch mode {
	case options[0]:
		return schemaDiffScopeAdditive, nil
	case options[1]:
		return schemaDiffScopeFull, nil
	case options[2]:
		return schemaDiffScopeDeletesOnly, nil
	default:
		return schemaDiffScopeCancel, nil
	}
}

func selectSchemaChanges(allChanges []SchemaChange, autoYes bool) ([]SchemaChange, error) {
	if autoYes {
		return allChanges, nil
	}

	var mode string
	if err := survey.AskOne(&survey.Select{
		Message: "How do you want to proceed?",
		Options: []string{
			"Apply all changes",
			"Pick individual changes",
			"Cancel",
		},
	}, &mode); err != nil {
		return nil, fmt.Errorf("selection cancelled: %w", err)
	}

	switch mode {
	case "Cancel":
		return nil, nil
	case "Apply all changes":
		return allChanges, nil
	case "Pick individual changes":
		return multiselectSchemaChanges(allChanges)
	default:
		return nil, nil
	}
}

func multiselectSchemaChanges(allChanges []SchemaChange) ([]SchemaChange, error) {
	options := make([]string, len(allChanges))
	optionMap := make(map[string]SchemaChange, len(allChanges))
	for i, ch := range allChanges {
		label := fmt.Sprintf("[%d] %s", i+1, formatSchemaChangeShort(ch))
		options[i] = label
		optionMap[label] = ch
	}

	fmt.Println()
	fmt.Println("  Select changes (↑/↓ move, space toggle, enter confirm):")
	for _, opt := range options {
		fmt.Printf("    %s\n", opt)
	}
	fmt.Println()

	var picked []string
	if err := survey.AskOne(&survey.MultiSelect{
		Message:  "Toggle changes to apply",
		Options:  options,
		PageSize: 15,
		Help:     "Use arrow keys and space to select; enter to confirm",
	}, &picked); err != nil {
		return nil, fmt.Errorf("selection cancelled: %w", err)
	}

	selected := make([]SchemaChange, 0, len(picked))
	for _, label := range picked {
		if ch, ok := optionMap[label]; ok {
			selected = append(selected, ch)
		}
	}
	return selected, nil
}

func confirmApplyToDestination(endpoints SyncEndpoints, changes []SchemaChange) (bool, error) {
	printApplyConfirmationBlock(endpoints, changes)
	confirmed := false
	if err := survey.AskOne(&survey.Confirm{
		Message: "Proceed with apply?",
		Default: false,
	}, &confirmed); err != nil {
		return false, err
	}
	return confirmed, nil
}
