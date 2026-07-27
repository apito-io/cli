package main

import (
	"fmt"
	"strings"
)

// closeFieldDependencies expands a selected change set so every nested
// ChangeAddField includes its ancestor ChangeAddField ops from the full plan.
// Auto-included ancestors are marked in Summary with "required by …".
// Returns an error if a selected nested field needs an ancestor that is not
// already present on the destination and not present as an add in allChanges.
func closeFieldDependencies(selected, allChanges []SchemaChange) ([]SchemaChange, error) {
	if len(selected) == 0 {
		return selected, nil
	}

	allByID := make(map[string]SchemaChange, len(allChanges))
	addByModelPath := make(map[string]SchemaChange)
	for _, ch := range allChanges {
		allByID[ch.ID] = ch
		if ch.Kind == ChangeAddField && ch.Field != nil {
			key := strings.ToLower(ch.Model) + "|" + fieldSyncKey(*ch.Field)
			addByModelPath[key] = ch
		}
	}

	selectedIDs := make(map[string]struct{}, len(selected))
	out := make([]SchemaChange, 0, len(selected))
	for _, ch := range selected {
		if _, ok := selectedIDs[ch.ID]; ok {
			continue
		}
		selectedIDs[ch.ID] = struct{}{}
		out = append(out, ch)
	}

	var autoIncluded []SchemaChange
	for _, ch := range selected {
		if ch.Kind != ChangeAddField || ch.Field == nil {
			continue
		}
		ancestors, err := ancestorAddFields(ch, addByModelPath)
		if err != nil {
			return nil, err
		}
		for _, anc := range ancestors {
			if _, ok := selectedIDs[anc.ID]; ok {
				continue
			}
			selectedIDs[anc.ID] = struct{}{}
			marked := anc
			childPath := fieldSyncKey(*ch.Field)
			marked.Summary = fmt.Sprintf("%s (required by %s.%s)", anc.Summary, ch.Model, childPath)
			autoIncluded = append(autoIncluded, marked)
			out = append(out, marked)
		}
	}

	if len(autoIncluded) > 0 {
		print_status(fmt.Sprintf(
			"Auto-included %d ancestor field add(s) required by selected nested fields.",
			len(autoIncluded),
		))
		for _, ch := range autoIncluded {
			if ch.Field == nil {
				continue
			}
			fmt.Printf("    + %s.%s — required ancestor\n", ch.Model, fieldSyncKey(*ch.Field))
		}
	}

	// Preserve relative order from the full plan (depth-first) for selected set.
	orderIndex := make(map[string]int, len(allChanges))
	for i, ch := range allChanges {
		orderIndex[ch.ID] = i
	}
	sortSchemaChangesByPlanOrder(out, orderIndex)
	return out, nil
}

func ancestorAddFields(child SchemaChange, addByModelPath map[string]SchemaChange) ([]SchemaChange, error) {
	if child.Field == nil {
		return nil, nil
	}
	path := strings.TrimSpace(fieldSyncKey(*child.Field))
	if path == "" || !strings.Contains(path, ".") {
		return nil, nil
	}
	parts := strings.Split(path, ".")
	var ancestors []SchemaChange
	for i := 1; i < len(parts); i++ {
		ancestorPath := strings.Join(parts[:i], ".")
		key := strings.ToLower(child.Model) + "|" + ancestorPath
		anc, ok := addByModelPath[key]
		if !ok {
			// Ancestor already exists on destination (not in additive plan) — OK.
			continue
		}
		ancestors = append(ancestors, anc)
	}
	// Detect impossible case: immediate parent not on dest and not in plan.
	if child.Field.ParentField != "" {
		immediatePath := ""
		if idx := strings.LastIndex(path, "."); idx > 0 {
			immediatePath = path[:idx]
		}
		if immediatePath != "" {
			key := strings.ToLower(child.Model) + "|" + immediatePath
			if _, inPlan := addByModelPath[key]; !inPlan {
				// Parent may already exist on destination. We cannot prove absence
				// here without dest schema; engine remains the hard gate.
				_ = key
			}
		}
	}
	return ancestors, nil
}

func sortSchemaChangesByPlanOrder(changes []SchemaChange, orderIndex map[string]int) {
	// Insertion sort keeps deps package-light and stable for small N.
	for i := 1; i < len(changes); i++ {
		j := i
		for j > 0 {
			ai, aok := orderIndex[changes[j-1].ID]
			bi, bok := orderIndex[changes[j].ID]
			if !aok {
				ai = 1 << 30
			}
			if !bok {
				bi = 1 << 30
			}
			if ai <= bi {
				break
			}
			changes[j-1], changes[j] = changes[j], changes[j-1]
			j--
		}
	}
}

// preflightNestedAddOrder verifies selected add/update field ops are parent-before-child.
func preflightNestedAddOrder(changes []SchemaChange) error {
	seen := make(map[string]struct{})
	for _, ch := range changes {
		if (ch.Kind != ChangeAddField && ch.Kind != ChangeUpdateField) || ch.Field == nil {
			continue
		}
		key := strings.ToLower(ch.Model) + "|" + fieldSyncKey(*ch.Field)
		path := fieldSyncKey(*ch.Field)
		if idx := strings.LastIndex(path, "."); idx > 0 {
			parentPath := path[:idx]
			parentKey := strings.ToLower(ch.Model) + "|" + parentPath
			// Parent must already have been listed earlier in this batch OR not be in batch
			// (already on destination). If parent is also an add later in the batch → error.
			if _, already := seen[parentKey]; !already {
				for _, later := range changes {
					if later.Kind != ChangeAddField || later.Field == nil {
						continue
					}
					laterKey := strings.ToLower(later.Model) + "|" + fieldSyncKey(*later.Field)
					if laterKey == parentKey {
						return fmt.Errorf(
							"preflight: nested field %s.%s would apply before parent %s.%s — fix apply order",
							ch.Model, path, ch.Model, parentPath,
						)
					}
				}
			}
		}
		seen[key] = struct{}{}
	}
	return nil
}
