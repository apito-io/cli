package main

import (
	"fmt"
	"testing"
)

func TestMergeEffectiveSchema_DraftModelSuppressesAddModel(t *testing.T) {
	live := []SyncModel{
		{Name: "author", Fields: []SyncField{{Identifier: "name", Label: "Name", FieldType: "text"}}},
		{Name: "book", Fields: []SyncField{{Identifier: "title", Label: "Title", FieldType: "text"}}},
	}
	draft := []SyncModel{
		{Name: "review", Fields: []SyncField{}},
	}

	effective := mergeEffectiveSchema(live, draft)
	diff := computeSchemaDiff([]SyncModel{
		{Name: "review", Fields: []SyncField{
			{Identifier: "name", Label: "name", FieldType: "text"},
			{Identifier: "comment", Label: "Comment", FieldType: "text"},
		}},
	}, effective)

	for _, md := range diff {
		for _, ch := range md.Changes {
			if ch.Kind == ChangeAddModel && ch.Model == "review" {
				t.Fatalf("expected no add_model for review when present in draft")
			}
		}
	}

	foundNameField := false
	for _, md := range diff {
		if md.Model != "review" {
			continue
		}
		for _, ch := range md.Changes {
			if ch.Kind == ChangeAddField && ch.Field != nil && ch.Field.Identifier == "name" {
				foundNameField = true
			}
		}
	}
	if !foundNameField {
		t.Fatal("expected add field name on review")
	}
}

func TestIsIdempotentDraftError(t *testing.T) {
	err := fmt.Errorf(`graphql errors: stage schema mutation: model "review" already exists in draft`)
	if !isIdempotentDraftError(err) {
		t.Fatal("expected idempotent draft error")
	}
}

func TestBuildSyncTasks_Order(t *testing.T) {
	changes := []SchemaChange{
		{ID: "c1", Kind: ChangeAddConnection, Model: "author", Connection: &SyncConnection{Model: "book"}},
		{ID: "f1", Kind: ChangeAddField, Model: "book", Field: &SyncField{Identifier: "dates"}},
		{ID: "m1", Kind: ChangeAddModel, Model: "review"},
	}
	tasks := buildSyncTasks(changes)
	if len(tasks) != 3 {
		t.Fatalf("task count = %d", len(tasks))
	}
	if tasks[0].Change.Kind != ChangeAddModel {
		t.Fatalf("first task kind = %v", tasks[0].Change.Kind)
	}
	if tasks[1].Change.Kind != ChangeAddField {
		t.Fatalf("second task kind = %v", tasks[1].Change.Kind)
	}
	if tasks[2].Change.Kind != ChangeAddConnection {
		t.Fatalf("third task kind = %v", tasks[2].Change.Kind)
	}
}
