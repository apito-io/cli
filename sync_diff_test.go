package main

import "testing"

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
