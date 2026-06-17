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
