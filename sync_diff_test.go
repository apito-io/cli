package main

import (
	"strings"
	"testing"
)

func TestFieldsMatchForSync_ValidationNullVsFalse(t *testing.T) {
	falseVal := false
	src := SyncField{
		Identifier: "name",
		Label:      "Name",
		FieldType:  "text",
		InputType:  "string",
		Serial:     1,
		Validation: &SyncFieldValidation{
			Required: &falseVal,
			Unique:   &falseVal,
			Hide:     &falseVal,
		},
	}
	dst := SyncField{
		Identifier: "name",
		Label:      "Name",
		FieldType:  "text",
		InputType:  "string",
		Serial:     1,
		Validation: &SyncFieldValidation{},
	}
	if !fieldsMatchForSync(src, dst) {
		t.Fatal("expected null validation on dest to match explicit false on source")
	}
}

func TestFieldsMatchForSync_EmptyLocalsNilVsSlice(t *testing.T) {
	// Draft schemaPreview often omits locals (nil); live returns []. Must not plan update_field.
	src := SyncField{
		Identifier: "type",
		FieldType:  "list",
		FieldSubType: "dropdown",
		Validation: &SyncFieldValidation{
			Locals:            []string{},
			FixedListElements: []any{"a", "b"},
			FixedListElementType: "string",
		},
	}
	dst := SyncField{
		Identifier: "type",
		FieldType:  "list",
		FieldSubType: "dropdown",
		Validation: &SyncFieldValidation{
			Locals:            nil,
			FixedListElements: []any{"a", "b"},
			FixedListElementType: "string",
		},
	}
	if !fieldsMatchForSync(src, dst) {
		t.Fatal("expected empty locals [] to match nil locals")
	}
}

func TestFieldsMatchForSync_IgnoresLabelSerialInputType(t *testing.T) {
	src := SyncField{Identifier: "name", Label: "Name", FieldType: "text", InputType: "string", Serial: 3}
	dst := SyncField{Identifier: "name", Label: "name", FieldType: "text", InputType: "", Serial: 0}
	if !fieldsMatchForSync(src, dst) {
		t.Fatal("expected cosmetic metadata differences to be ignored")
	}
}

func TestComputeSchemaDiff_NoFalseUpdatesForValidationNoise(t *testing.T) {
	source := []SyncModel{{
		Name: "author",
		Fields: []SyncField{{
			Identifier: "name",
			Label:      "Name",
			FieldType:  "text",
			Serial:     1,
			Validation: func() *SyncFieldValidation {
				f := false
				return &SyncFieldValidation{Required: &f, Unique: &f, Hide: &f}
			}(),
		}},
	}}
	dest := []SyncModel{{
		Name: "author",
		Fields: []SyncField{{
			Identifier: "name",
			Label:      "Name",
			FieldType:  "text",
			Serial:     1,
			Validation: &SyncFieldValidation{},
		}},
	}}

	diff := computeSchemaDiff(source, dest)
	for _, md := range diff {
		for _, ch := range md.Changes {
			if ch.Kind == ChangeUpdateField {
				t.Fatalf("unexpected update_field for validation-only noise: %s", ch.Summary)
			}
		}
	}
}

func TestFieldSyncKey_DisambiguatesNestedIdentifiers(t *testing.T) {
	top := SyncField{Identifier: "price", FieldType: "number"}
	nested := SyncField{Identifier: "price", ParentField: "sizes", FieldType: "number"}
	if fieldSyncKey(top) == fieldSyncKey(nested) {
		t.Fatalf("expected distinct keys for top-level vs nested price, got %q", fieldSyncKey(top))
	}
	if fieldSyncKey(top) != "price" {
		t.Fatalf("top-level key = %q, want price", fieldSyncKey(top))
	}
	if fieldSyncKey(nested) != "sizes.price" {
		t.Fatalf("nested key = %q, want sizes.price", fieldSyncKey(nested))
	}
}

func TestComputeSchemaDiff_NoFalseUpdatesForReusedNestedIdentifiers(t *testing.T) {
	// Mirrors Rosna: root price + sizes.price; stock_add/_minus share _id/quantity.
	mk := func(name string) SyncModel {
		return SyncModel{
			Name: name,
			Fields: []SyncField{
				{Identifier: "price", Label: "Price", FieldType: "number"},
				{
					Identifier: "sizes",
					Label:      "Sizes",
					FieldType:  "repeated",
					SubFieldInfo: []SyncField{
						{Identifier: "_id", Label: "ID", FieldType: "text"},
						{Identifier: "price", Label: "Price", FieldType: "number"},
					},
				},
				{
					Identifier: "stock_add",
					Label:      "Stock Add",
					FieldType:  "repeated",
					SubFieldInfo: []SyncField{
						{Identifier: "_id", Label: "ID", FieldType: "text"},
						{Identifier: "quantity", Label: "Quantity", FieldType: "number"},
					},
				},
				{
					Identifier: "stock_minus",
					Label:      "Stock Minus",
					FieldType:  "repeated",
					SubFieldInfo: []SyncField{
						{Identifier: "_id", Label: "ID", FieldType: "text"},
						{Identifier: "quantity", Label: "Quantity", FieldType: "number"},
					},
				},
			},
		}
	}

	diff := computeSchemaDiff([]SyncModel{mk("food")}, []SyncModel{mk("food")})
	for _, md := range diff {
		for _, ch := range md.Changes {
			if ch.Kind == ChangeUpdateField {
				t.Fatalf("unexpected false update_field from nested identifier collision: %s (%s)", ch.Summary, ch.ID)
			}
		}
	}
}

func TestComputeSchemaDiff_DetectsRealNestedListValidationChange(t *testing.T) {
	source := []SyncModel{{
		Name: "food_order",
		Fields: []SyncField{{
			Identifier: "stages",
			Label:      "Stages",
			FieldType:  "repeated",
			SubFieldInfo: []SyncField{{
				Identifier: "stage",
				Label:      "Stage",
				FieldType:  "list",
				Validation: &SyncFieldValidation{
					FixedListElementType: "string",
					FixedListElements:    []any{"cooking", "delivered"},
				},
			}},
		}},
	}}
	dest := []SyncModel{{
		Name: "food_order",
		Fields: []SyncField{{
			Identifier: "stages",
			Label:      "Stages",
			FieldType:  "repeated",
			SubFieldInfo: []SyncField{{
				Identifier: "stage",
				Label:      "Stage",
				FieldType:  "list",
				Validation: &SyncFieldValidation{
					FixedListElementType: "string",
					FixedListElements:    []any{"sample_collected", "delivered"},
				},
			}},
		}},
	}}

	diff := computeSchemaDiff(source, dest)
	found := false
	for _, md := range diff {
		for _, ch := range md.Changes {
			if ch.Kind == ChangeUpdateField && ch.Field != nil && ch.Field.Identifier == "stage" {
				found = true
				if ch.ID != "field-update:food_order:stages.stage" {
					t.Fatalf("update id = %q, want field-update:food_order:stages.stage", ch.ID)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected real update_field for stages.stage list validation drift")
	}
}

func TestComputeSchemaDeleteDiff_DestinationOnlyFields(t *testing.T) {
	source := []SyncModel{{
		Name: "teacher",
		Fields: []SyncField{
			{Identifier: "name", Label: "name", FieldType: "text"},
			{Identifier: "phone", Label: "phone", FieldType: "text"},
		},
	}}
	dest := []SyncModel{{
		Name: "teacher",
		Fields: []SyncField{
			{Identifier: "name", Label: "name", FieldType: "text"},
			{Identifier: "phone", Label: "phone", FieldType: "text"},
			{Identifier: "username", Label: "username", FieldType: "text"},
			{Identifier: "secret", Label: "secret", FieldType: "text"},
			{Identifier: "token", Label: "token", FieldType: "text"},
			{Identifier: "internal_role", Label: "internal_role", FieldType: "list"},
			{Identifier: "redirect_after_login", Label: "redirect_after_login", FieldType: "list"},
		},
	}}

	// Additive path must stay quiet.
	if add := flattenSchemaChanges(computeSchemaDiff(source, dest)); len(add) != 0 {
		t.Fatalf("additive diff = %d, want 0", len(add))
	}

	del := flattenSchemaChanges(computeSchemaDeleteDiff(source, dest))
	if len(del) != 5 {
		t.Fatalf("delete diff = %d, want 5", len(del))
	}
	got := map[string]bool{}
	for _, ch := range del {
		if ch.Kind != ChangeDeleteField || ch.Field == nil {
			t.Fatalf("unexpected change: %+v", ch)
		}
		got[ch.Field.Identifier] = true
	}
	for _, id := range []string{"username", "secret", "token", "internal_role", "redirect_after_login"} {
		if !got[id] {
			t.Fatalf("missing delete for %s", id)
		}
	}
}

func TestBuildSyncTasks_DeletesAfterAdditive(t *testing.T) {
	changes := []SchemaChange{
		{ID: "d1", Kind: ChangeDeleteField, Model: "teacher", Field: &SyncField{Identifier: "secret"}},
		{ID: "f1", Kind: ChangeAddField, Model: "class", Field: &SyncField{Identifier: "name"}},
		{ID: "m1", Kind: ChangeAddModel, Model: "review"},
	}
	tasks := buildSyncTasks(changes)
	if len(tasks) != 3 {
		t.Fatalf("task count = %d", len(tasks))
	}
	if tasks[0].Change.Kind != ChangeAddModel {
		t.Fatalf("first = %v", tasks[0].Change.Kind)
	}
	if tasks[1].Change.Kind != ChangeAddField {
		t.Fatalf("second = %v", tasks[1].Change.Kind)
	}
	if tasks[2].Change.Kind != ChangeDeleteField {
		t.Fatalf("third = %v", tasks[2].Change.Kind)
	}
}

func TestFlattenModelFields_Depth2RepeatedAndObject(t *testing.T) {
	model := SyncModel{
		Name: "exam",
		Fields: []SyncField{{
			Identifier: "routine",
			FieldType:  "repeated",
			InputType:  "repeated",
			SubFieldInfo: []SyncField{
				{Identifier: "class_code", FieldType: "text", InputType: "string"},
				{
					Identifier: "details",
					FieldType:  "repeated",
					InputType:  "repeated",
					SubFieldInfo: []SyncField{
						{Identifier: "date_and_time", FieldType: "date", InputType: "string"},
						{Identifier: "subject_code", FieldType: "text", InputType: "string"},
						{
							Identifier: "meta",
							FieldType:  "object",
							InputType:  "object",
							SubFieldInfo: []SyncField{
								{Identifier: "room", FieldType: "text", InputType: "string"},
							},
						},
					},
				},
			},
		}},
	}
	flat := flattenModelFields(model)
	got := map[string]SyncField{}
	for _, f := range flat {
		got[fieldSyncKey(f)] = f
	}
	for _, key := range []string{
		"routine",
		"routine.class_code",
		"routine.details",
		"routine.details.date_and_time",
		"routine.details.subject_code",
		"routine.details.meta",
		"routine.details.meta.room",
	} {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing flattened path %q in %#v", key, keysOf(got))
		}
	}
	dt := got["routine.details.date_and_time"]
	if dt.ParentField != "details" {
		t.Fatalf("immediate parent = %q, want details (upsert parent_field)", dt.ParentField)
	}
	if dt.Path != "routine.details.date_and_time" {
		t.Fatalf("path = %q", dt.Path)
	}
}

func TestComputeSchemaDiff_Depth2NestedAddsProtivaExam(t *testing.T) {
	// Local has details children; prod has empty nested repeated — the bug CLI missed.
	source := []SyncModel{{
		Name: "exam",
		Fields: []SyncField{{
			Identifier: "routine",
			Label:      "routine",
			FieldType:  "repeated",
			InputType:  "repeated",
			SubFieldInfo: []SyncField{
				{Identifier: "class_code", Label: "class_code", FieldType: "text", InputType: "string"},
				{
					Identifier: "details",
					Label:      "details",
					FieldType:  "repeated",
					InputType:  "repeated",
					SubFieldInfo: []SyncField{
						{Identifier: "date_and_time", Label: "date_and_time", FieldType: "date", InputType: "string"},
						{Identifier: "subject_code", Label: "subject_code", FieldType: "text", InputType: "string"},
					},
				},
			},
		}},
	}}
	dest := []SyncModel{{
		Name: "exam",
		Fields: []SyncField{{
			Identifier: "routine",
			Label:      "routine",
			FieldType:  "repeated",
			InputType:  "repeated",
			SubFieldInfo: []SyncField{
				{Identifier: "class_code", Label: "class_code", FieldType: "text", InputType: "string"},
				{Identifier: "details", Label: "details", FieldType: "repeated", InputType: "repeated"},
			},
		}},
	}}

	changes := flattenSchemaChanges(computeSchemaDiff(source, dest))
	if len(changes) != 2 {
		t.Fatalf("changes = %d (%+v), want 2 depth-2 adds", len(changes), changeSummaries(changes))
	}
	got := map[string]string{}
	for _, ch := range changes {
		if ch.Kind != ChangeAddField || ch.Field == nil {
			t.Fatalf("unexpected %+v", ch)
		}
		got[fieldSyncKey(*ch.Field)] = ch.Field.ParentField
	}
	if got["routine.details.date_and_time"] != "details" {
		t.Fatalf("date_and_time parent = %q", got["routine.details.date_and_time"])
	}
	if got["routine.details.subject_code"] != "details" {
		t.Fatalf("subject_code parent = %q", got["routine.details.subject_code"])
	}
}

func TestFieldSyncKey_FullPathDisambiguatesSameImmediateParent(t *testing.T) {
	a := SyncField{Identifier: "name", ParentField: "sections", Path: "sections.name"}
	b := SyncField{Identifier: "name", ParentField: "sections", Path: "divisions.sections.name"}
	if fieldSyncKey(a) == fieldSyncKey(b) {
		t.Fatalf("paths must disambiguate reused parent identifier, got %q", fieldSyncKey(a))
	}
}

func TestComputeSchemaDiff_AddOrderParentsBeforeChildren(t *testing.T) {
	source := []SyncModel{{
		Name: "exam",
		Fields: []SyncField{{
			Identifier: "routine",
			FieldType:  "repeated",
			InputType:  "repeated",
			Serial:     1,
			SubFieldInfo: []SyncField{{
				Identifier: "details",
				FieldType:  "repeated",
				InputType:  "repeated",
				Serial:     1,
				SubFieldInfo: []SyncField{
					{Identifier: "date_and_time", FieldType: "date", InputType: "string", Serial: 1},
				},
			}},
		}},
	}}
	dest := []SyncModel{{Name: "exam", Fields: nil}}
	changes := flattenSchemaChanges(computeSchemaDiff(source, dest))
	var paths []string
	for _, ch := range changes {
		if ch.Kind == ChangeAddField && ch.Field != nil {
			paths = append(paths, fieldSyncKey(*ch.Field))
		}
	}
	wantOrder := []string{"routine", "routine.details", "routine.details.date_and_time"}
	if len(paths) != len(wantOrder) {
		t.Fatalf("paths = %v, want %v", paths, wantOrder)
	}
	for i, w := range wantOrder {
		if paths[i] != w {
			t.Fatalf("order[%d] = %q, want %q (full %v)", i, paths[i], w, paths)
		}
	}
}

func TestFieldsMatchForSync_DetectsNestedMultiChoiceChange(t *testing.T) {
	tTrue, tFalse := true, false
	a := SyncField{
		Identifier: "stage",
		FieldType:  "list",
		ParentField: "stages",
		Path:       "stages.stage",
		Validation: &SyncFieldValidation{IsMultiChoice: &tFalse, FixedListElementType: "string"},
	}
	b := SyncField{
		Identifier: "stage",
		FieldType:  "list",
		ParentField: "stages",
		Path:       "stages.stage",
		Validation: &SyncFieldValidation{IsMultiChoice: &tTrue, FixedListElementType: "string"},
	}
	if fieldsMatchForSync(a, b) {
		t.Fatal("is_multi_choice change must not match for sync")
	}
}

func keysOf(m map[string]SyncField) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func changeSummaries(changes []SchemaChange) []string {
	out := make([]string, 0, len(changes))
	for _, ch := range changes {
		out = append(out, ch.Summary)
	}
	return out
}

func TestComputeSchemaDiff_FlippedConnectionPlansFix(t *testing.T) {
	source := []SyncModel{{
		Name: "mark_config",
		Connections: []SyncConnection{
			{Model: "class", Relation: "has_one", Type: "forward"},
		},
	}, {
		Name: "class",
		Connections: []SyncConnection{
			{Model: "mark_config", Relation: "has_many", Type: "backward"},
		},
	}}
	dest := []SyncModel{{
		Name: "mark_config",
		Connections: []SyncConnection{
			{Model: "class", Relation: "has_one", Type: "backward"},
		},
	}, {
		Name: "class",
		Connections: []SyncConnection{
			{Model: "mark_config", Relation: "has_one", Type: "forward"},
		},
	}}
	diffs := computeSchemaDiff(source, dest)
	changes := flattenSchemaChanges(diffs)
	if len(changes) != 1 {
		t.Fatalf("changes = %#v, want 1 fix", changes)
	}
	if changes[0].Kind != ChangeUpdateConnection {
		t.Fatalf("kind = %q, want update_connection", changes[0].Kind)
	}
	if !strings.Contains(changes[0].Summary, "Fix relation direction") {
		t.Fatalf("summary = %q", changes[0].Summary)
	}
}

func TestComputeSchemaDiff_MatchingForwardConnectionNoChange(t *testing.T) {
	source := []SyncModel{{
		Name: "mark_input",
		Connections: []SyncConnection{
			{Model: "class", Relation: "has_one", Type: "forward"},
		},
	}, {
		Name: "class",
		Connections: []SyncConnection{
			{Model: "mark_input", Relation: "has_many", Type: "backward"},
		},
	}}
	dest := []SyncModel{{
		Name: "mark_input",
		Connections: []SyncConnection{
			{Model: "class", Relation: "has_one", Type: "forward"},
		},
	}, {
		Name: "class",
		Connections: []SyncConnection{
			{Model: "mark_input", Relation: "has_many", Type: "backward"},
		},
	}}
	changes := flattenSchemaChanges(computeSchemaDiff(source, dest))
	if len(changes) != 0 {
		t.Fatalf("expected no changes, got %#v", changes)
	}
}

func multilineMessageField(id string) SyncField {
	return SyncField{
		Identifier: id,
		Label:      id,
		FieldType:  "multiline",
		InputType:  "string",
		SubFieldInfo: []SyncField{
			{Identifier: "html", FieldType: "text", InputType: "string"},
			{Identifier: "markdown", FieldType: "text", InputType: "string"},
			{Identifier: "text", FieldType: "text", InputType: "string"},
		},
	}
}

func TestFlattenModelFields_SkipsEngineCompositeChildren(t *testing.T) {
	flat := flattenModelFields(SyncModel{
		Name: "app_release_policy",
		Fields: []SyncField{
			{Identifier: "platform", Label: "platform", FieldType: "text"},
			multilineMessageField("message_en"),
			multilineMessageField("message_bn"),
		},
	})
	got := make(map[string]bool, len(flat))
	for _, f := range flat {
		got[fieldSyncKey(f)] = true
	}
	for _, want := range []string{"platform", "message_en", "message_bn"} {
		if !got[want] {
			t.Fatalf("missing parent %q in %#v", want, got)
		}
	}
	for _, noise := range []string{
		"message_en.html", "message_en.markdown", "message_en.text",
		"message_bn.html", "message_bn.markdown", "message_bn.text",
	} {
		if got[noise] {
			t.Fatalf("engine composite leaf %q must not appear in flatten: %#v", noise, got)
		}
	}
}

func TestComputeSchemaDiff_IgnoresMultilineBuiltinLeaves(t *testing.T) {
	// Source has full multiline sub_field_info; dest only has the parent —
	// common when projectModelsInfo omits built-in leaves on one side.
	source := []SyncModel{{
		Name: "app_release_policy",
		Fields: []SyncField{
			{Identifier: "platform", Label: "platform", FieldType: "text"},
			multilineMessageField("message_en"),
			multilineMessageField("message_bn"),
		},
	}}
	dest := []SyncModel{{
		Name: "app_release_policy",
		Fields: []SyncField{
			{Identifier: "platform", Label: "platform", FieldType: "text"},
			{Identifier: "message_en", Label: "message_en", FieldType: "multiline", InputType: "string"},
			{Identifier: "message_bn", Label: "message_bn", FieldType: "multiline", InputType: "string"},
		},
	}}
	changes := flattenSchemaChanges(computeSchemaDiff(source, dest))
	if len(changes) != 0 {
		t.Fatalf("expected no multiline-leaf noise, got %#v", changes)
	}
	del := flattenSchemaChanges(computeSchemaDeleteDiff(source, dest))
	if len(del) != 0 {
		t.Fatalf("expected no delete noise for composite leaves, got %#v", del)
	}
}
