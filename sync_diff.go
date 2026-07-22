package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type SchemaChangeKind string

const (
	ChangeAddModel      SchemaChangeKind = "add_model"
	ChangeAddField      SchemaChangeKind = "add_field"
	ChangeUpdateField   SchemaChangeKind = "update_field"
	ChangeDeleteField   SchemaChangeKind = "delete_field"
	ChangeAddConnection SchemaChangeKind = "add_connection"
)

type SchemaChange struct {
	ID          string
	Kind        SchemaChangeKind
	Model       string
	Field       *SyncField
	Connection  *SyncConnection
	ReverseType string
	Summary     string
}

type ModelSchemaDiff struct {
	Model   string
	Changes []SchemaChange
}

func projectSyncProfileKey(p SyncProject) string {
	pt := strings.ToLower(strings.TrimSpace(p.ProjectType))
	if pt == "" || pt == "general" {
		return "general"
	}
	if p.PerTenantSeparateDatabase {
		return "saas-per-tenant"
	}
	return "saas-shared"
}

func flattenModelFields(model SyncModel) []SyncField {
	out := make([]SyncField, 0, len(model.Fields))
	var walk func(fields []SyncField, parentID, pathPrefix string)
	walk = func(fields []SyncField, parentID, pathPrefix string) {
		for _, f := range fields {
			copy := f
			copy.ParentField = parentID
			path := strings.TrimSpace(f.Identifier)
			if pathPrefix != "" {
				path = pathPrefix + "." + path
			}
			copy.Path = path
			// Children are walked separately; clear nested payload on the flat row
			// so equality checks compare this node only.
			copy.SubFieldInfo = nil
			out = append(out, copy)
			if len(f.SubFieldInfo) > 0 {
				walk(f.SubFieldInfo, f.Identifier, path)
			}
		}
	}
	walk(model.Fields, "", "")
	return out
}

// fieldSyncKey uniquely identifies a field within a model for sync matching.
// Nested/repeated subfields often reuse identifiers (_id, price, quantity);
// keying by identifier alone collapses them and creates false update_field diffs.
// Prefer full Path (routine.details.date_and_time) so two groups that share an
// immediate parent identifier do not collide.
func fieldSyncKey(f SyncField) string {
	if p := strings.ToLower(strings.TrimSpace(f.Path)); p != "" {
		return p
	}
	parent := strings.ToLower(strings.TrimSpace(f.ParentField))
	id := strings.ToLower(strings.TrimSpace(f.Identifier))
	if parent == "" {
		return id
	}
	return parent + "." + id
}

func fieldPathDepth(f SyncField) int {
	key := fieldSyncKey(f)
	if key == "" {
		return 0
	}
	return strings.Count(key, ".")
}

func fieldMap(fields []SyncField) map[string]SyncField {
	m := make(map[string]SyncField, len(fields))
	for _, f := range fields {
		m[fieldSyncKey(f)] = f
	}
	return m
}

func validationBoolEqual(a, b *bool) bool {
	av := a != nil && *a
	bv := b != nil && *b
	return av == bv
}

func validationEqualForSync(a, b *SyncFieldValidation) bool {
	if a == nil && b == nil {
		return true
	}
	var empty SyncFieldValidation
	if a == nil {
		a = &empty
	}
	if b == nil {
		b = &empty
	}
	if !validationBoolEqual(a.Required, b.Required) ||
		!validationBoolEqual(a.Unique, b.Unique) ||
		!validationBoolEqual(a.Hide, b.Hide) ||
		!validationBoolEqual(a.AsTitle, b.AsTitle) ||
		!validationBoolEqual(a.IsMultiChoice, b.IsMultiChoice) ||
		!validationBoolEqual(a.IsEmail, b.IsEmail) ||
		!validationBoolEqual(a.IsGallery, b.IsGallery) ||
		!validationBoolEqual(a.IsURL, b.IsURL) {
		return false
	}
	if a.FixedListElementType != b.FixedListElementType || a.Placeholder != b.Placeholder {
		return false
	}
	la, _ := json.Marshal(a.Locals)
	lb, _ := json.Marshal(b.Locals)
	if string(la) != string(lb) {
		return false
	}
	ab, _ := json.Marshal(a.FixedListElements)
	bb, _ := json.Marshal(b.FixedListElements)
	return string(ab) == string(bb)
}

func validationEqual(a, b *SyncFieldValidation) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

// fieldsMatchForSync compares structural field shape for schema sync.
// Cosmetic metadata (label, serial, input_type) is ignored; nil/false validation booleans are equivalent.
func fieldsMatchForSync(a, b SyncField) bool {
	return strings.EqualFold(a.Identifier, b.Identifier) &&
		a.FieldType == b.FieldType &&
		a.FieldSubType == b.FieldSubType &&
		a.ParentField == b.ParentField &&
		validationEqualForSync(a.Validation, b.Validation)
}

func fieldEqual(a, b SyncField) bool {
	return strings.EqualFold(a.Identifier, b.Identifier) &&
		a.Label == b.Label &&
		a.FieldType == b.FieldType &&
		a.FieldSubType == b.FieldSubType &&
		a.InputType == b.InputType &&
		a.ParentField == b.ParentField &&
		a.Serial == b.Serial &&
		validationEqual(a.Validation, b.Validation)
}

func connectionKey(fromModel string, conn SyncConnection) string {
	knownAs := conn.KnownAs
	if knownAs == "" {
		knownAs = conn.Model
	}
	return fmt.Sprintf("%s->%s:%s", strings.ToLower(fromModel), strings.ToLower(conn.Model), strings.ToLower(knownAs))
}

func isForwardConnection(conn SyncConnection) bool {
	t := strings.ToLower(strings.TrimSpace(conn.Type))
	return t == "" || t == "forward"
}

func findReverseRelationType(models map[string]SyncModel, fromModel, toModel string) string {
	target, ok := models[strings.ToLower(toModel)]
	if !ok {
		return "has_many"
	}
	for _, c := range target.Connections {
		if !isForwardConnection(c) && strings.EqualFold(c.Model, fromModel) {
			if c.Relation != "" {
				return c.Relation
			}
		}
	}
	return "has_many"
}

func computeSchemaDiff(sourceModels, destModels []SyncModel) []ModelSchemaDiff {
	srcMap := make(map[string]SyncModel, len(sourceModels))
	dstMap := make(map[string]SyncModel, len(destModels))
	for _, m := range sourceModels {
		srcMap[strings.ToLower(m.Name)] = m
	}
	for _, m := range destModels {
		dstMap[strings.ToLower(m.Name)] = m
	}

	modelNames := make([]string, 0, len(srcMap))
	for name := range srcMap {
		modelNames = append(modelNames, name)
	}
	sort.Strings(modelNames)

	var diffs []ModelSchemaDiff
	for _, modelKey := range modelNames {
		src := srcMap[modelKey]
		dst, destExists := dstMap[modelKey]
		var changes []SchemaChange

		if !destExists {
			changes = append(changes, SchemaChange{
				ID:      fmt.Sprintf("model:%s", src.Name),
				Kind:    ChangeAddModel,
				Model:   src.Name,
				Summary: fmt.Sprintf("Add model %q", src.Name),
			})
		}

		srcFields := flattenModelFields(src)
		dstFields := fieldMap(flattenModelFields(dst))
		// Depth-first ancestry order: parents before children so nested adds
		// (routine → details → date_and_time) apply cleanly.
		sort.SliceStable(srcFields, func(i, j int) bool {
			di, dj := fieldPathDepth(srcFields[i]), fieldPathDepth(srcFields[j])
			if di != dj {
				return di < dj
			}
			if srcFields[i].Serial != srcFields[j].Serial {
				return srcFields[i].Serial < srcFields[j].Serial
			}
			return fieldSyncKey(srcFields[i]) < fieldSyncKey(srcFields[j])
		})

		for _, sf := range srcFields {
			df, ok := dstFields[fieldSyncKey(sf)]
			fieldCopy := sf
			if !ok {
				changes = append(changes, SchemaChange{
					ID:      fmt.Sprintf("field:%s:%s", src.Name, fieldSyncKey(sf)),
					Kind:    ChangeAddField,
					Model:   src.Name,
					Field:   &fieldCopy,
					Summary: fmt.Sprintf("Add field %q (%s) on %q", sf.Label, sf.Identifier, src.Name),
				})
				continue
			}
			if !fieldsMatchForSync(sf, df) {
				changes = append(changes, SchemaChange{
					ID:      fmt.Sprintf("field-update:%s:%s", src.Name, fieldSyncKey(sf)),
					Kind:    ChangeUpdateField,
					Model:   src.Name,
					Field:   &fieldCopy,
					Summary: fmt.Sprintf("Update field %q on %q", sf.Identifier, src.Name),
				})
			}
		}

		dstConnKeys := make(map[string]struct{})
		if destExists {
			for _, c := range dst.Connections {
				if isForwardConnection(c) {
					dstConnKeys[connectionKey(dst.Name, c)] = struct{}{}
				}
			}
		}
		for _, c := range src.Connections {
			if !isForwardConnection(c) {
				continue
			}
			key := connectionKey(src.Name, c)
			if _, ok := dstConnKeys[key]; ok {
				continue
			}
			connCopy := c
			reverse := findReverseRelationType(srcMap, src.Name, c.Model)
			forward := c.Relation
			if forward == "" {
				forward = "has_many"
			}
			knownAs := c.KnownAs
			if knownAs == "" {
				knownAs = c.Model
			}
			changes = append(changes, SchemaChange{
				ID:          fmt.Sprintf("conn:%s:%s", src.Name, key),
				Kind:        ChangeAddConnection,
				Model:       src.Name,
				Connection:  &connCopy,
				ReverseType: reverse,
				Summary:     fmt.Sprintf("Add relation %q → %q (%s ↔ %s, known_as: %q) on %q", src.Name, c.Model, forward, reverse, knownAs, src.Name),
			})
		}

		if len(changes) > 0 {
			diffs = append(diffs, ModelSchemaDiff{
				Model:   src.Name,
				Changes: changes,
			})
		}
	}
	return diffs
}

// computeSchemaDeleteDiff finds fields present on destination but missing on
// source (destination-only). These are optional destructive removes — not
// included in computeSchemaDiff so additive sync stays safe by default.
func computeSchemaDeleteDiff(sourceModels, destModels []SyncModel) []ModelSchemaDiff {
	srcMap := make(map[string]SyncModel, len(sourceModels))
	for _, m := range sourceModels {
		srcMap[strings.ToLower(m.Name)] = m
	}

	modelNames := make([]string, 0, len(destModels))
	for _, m := range destModels {
		modelNames = append(modelNames, strings.ToLower(m.Name))
	}
	sort.Strings(modelNames)

	seen := make(map[string]struct{}, len(modelNames))
	var diffs []ModelSchemaDiff
	for _, modelKey := range modelNames {
		if _, ok := seen[modelKey]; ok {
			continue
		}
		seen[modelKey] = struct{}{}

		var dst SyncModel
		for _, m := range destModels {
			if strings.EqualFold(m.Name, modelKey) {
				dst = m
				break
			}
		}
		src, srcExists := srcMap[modelKey]
		if !srcExists {
			// Entire model missing on source — model delete is out of scope for now.
			continue
		}

		srcFields := fieldMap(flattenModelFields(src))
		dstFields := flattenModelFields(dst)
		// Delete deepest children first so parent groups are not removed while
		// nested ops still reference them.
		sort.SliceStable(dstFields, func(i, j int) bool {
			di, dj := fieldPathDepth(dstFields[i]), fieldPathDepth(dstFields[j])
			if di != dj {
				return di > dj
			}
			if dstFields[i].Serial != dstFields[j].Serial {
				return dstFields[i].Serial < dstFields[j].Serial
			}
			return fieldSyncKey(dstFields[i]) < fieldSyncKey(dstFields[j])
		})

		var changes []SchemaChange
		for _, df := range dstFields {
			if _, ok := srcFields[fieldSyncKey(df)]; ok {
				continue
			}
			fieldCopy := df
			changes = append(changes, SchemaChange{
				ID:      fmt.Sprintf("field-delete:%s:%s", dst.Name, fieldSyncKey(df)),
				Kind:    ChangeDeleteField,
				Model:   dst.Name,
				Field:   &fieldCopy,
				Summary: fmt.Sprintf("Delete field %q on %q", df.Identifier, dst.Name),
			})
		}
		if len(changes) > 0 {
			diffs = append(diffs, ModelSchemaDiff{Model: dst.Name, Changes: changes})
		}
	}
	return diffs
}

func mergeSchemaDiffs(parts ...[]ModelSchemaDiff) []ModelSchemaDiff {
	byModel := make(map[string]*ModelSchemaDiff)
	order := make([]string, 0)
	for _, part := range parts {
		for _, md := range part {
			key := strings.ToLower(md.Model)
			existing, ok := byModel[key]
			if !ok {
				cp := ModelSchemaDiff{Model: md.Model, Changes: append([]SchemaChange{}, md.Changes...)}
				byModel[key] = &cp
				order = append(order, key)
				continue
			}
			existing.Changes = append(existing.Changes, md.Changes...)
		}
	}
	out := make([]ModelSchemaDiff, 0, len(order))
	for _, key := range order {
		out = append(out, *byModel[key])
	}
	return out
}

func partitionSchemaChanges(changes []SchemaChange) (additive, deletes []SchemaChange) {
	for _, ch := range changes {
		if ch.Kind == ChangeDeleteField {
			deletes = append(deletes, ch)
			continue
		}
		additive = append(additive, ch)
	}
	return additive, deletes
}

func printModelDiffHeader(model string, changes []SchemaChange) {
	fmt.Println()
	print_step(fmt.Sprintf("Model: %s (%d change(s))", model, len(changes)))
	for _, ch := range changes {
		fmt.Printf("  - %s\n", ch.Summary)
	}
}
