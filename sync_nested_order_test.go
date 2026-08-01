package main

import (
	"strings"
	"testing"
)

func TestBuildSyncTasks_NestedAddsParentsBeforeChildren(t *testing.T) {
	changes := []SchemaChange{
		{
			ID: "lab_order:label_id", Kind: ChangeAddField, Model: "lab_order",
			Field: &SyncField{Identifier: "label_id", ParentField: "test_label_results", Path: "tests.test_label_results.label_id", FieldType: "text", Serial: 2},
		},
		{
			ID: "lab_order:result", Kind: ChangeAddField, Model: "lab_order",
			Field: &SyncField{Identifier: "result", ParentField: "test_label_results", Path: "tests.test_label_results.result", FieldType: "text", Serial: 3},
		},
		{
			ID: "lab_order:test_label_results", Kind: ChangeAddField, Model: "lab_order",
			Field: &SyncField{Identifier: "test_label_results", ParentField: "tests", Path: "tests.test_label_results", FieldType: "repeated", Serial: 9},
		},
		{
			ID: "lab_order:tests", Kind: ChangeAddField, Model: "lab_order",
			Field: &SyncField{Identifier: "tests", Path: "tests", FieldType: "repeated", Serial: 4},
		},
		{
			ID: "ga:anemia", Kind: ChangeAddField, Model: "general_assessment",
			Field: &SyncField{Identifier: "anemia", ParentField: "clinical_signs", Path: "general_examination.clinical_signs.anemia", FieldType: "boolean", Serial: 1},
		},
		{
			ID: "ga:clinical_signs", Kind: ChangeAddField, Model: "general_assessment",
			Field: &SyncField{Identifier: "clinical_signs", ParentField: "general_examination", Path: "general_examination.clinical_signs", FieldType: "object", Serial: 7},
		},
		{
			ID: "ga:eye_examination", Kind: ChangeAddField, Model: "general_assessment",
			Field: &SyncField{Identifier: "eye_examination", ParentField: "general_examination", Path: "general_examination.eye_examination", FieldType: "object", Serial: 5},
		},
		{
			ID: "ga:left_va", Kind: ChangeAddField, Model: "general_assessment",
			Field: &SyncField{Identifier: "left_va", ParentField: "eye_examination", Path: "general_examination.eye_examination.left_va", FieldType: "text", Serial: 1},
		},
		{
			ID: "ga:general_examination", Kind: ChangeAddField, Model: "general_assessment",
			Field: &SyncField{Identifier: "general_examination", Path: "general_examination", FieldType: "object", Serial: 1},
		},
	}

	tasks := buildSyncTasks(changes)
	var labPaths, gaPaths []string
	for _, task := range tasks {
		if task.Change.Field == nil {
			continue
		}
		key := fieldSyncKey(*task.Change.Field)
		switch task.Change.Model {
		case "lab_order":
			labPaths = append(labPaths, key)
		case "general_assessment":
			gaPaths = append(gaPaths, key)
		}
	}

	wantLab := []string{"tests", "tests.test_label_results", "tests.test_label_results.label_id", "tests.test_label_results.result"}
	if len(labPaths) != len(wantLab) {
		t.Fatalf("lab paths = %v, want %v", labPaths, wantLab)
	}
	for i, w := range wantLab {
		if labPaths[i] != w {
			t.Fatalf("lab order[%d] = %q, want %q (full %v)", i, labPaths[i], w, labPaths)
		}
	}

	// general_examination first, then depth-1 containers, then leaves.
	if gaPaths[0] != "general_examination" {
		t.Fatalf("ga first = %q, want general_examination (%v)", gaPaths[0], gaPaths)
	}
	depth1 := map[string]bool{}
	for _, p := range gaPaths[1:] {
		d := strings.Count(p, ".")
		if d == 1 {
			depth1[p] = true
		}
		if d == 2 {
			parent := p[:strings.LastIndex(p, ".")]
			if !depth1[parent] && parent != "general_examination.clinical_signs" && parent != "general_examination.eye_examination" {
				// parent must appear earlier
				found := false
				for _, earlier := range gaPaths {
					if earlier == parent {
						found = true
						break
					}
					if earlier == p {
						break
					}
				}
				if !found {
					t.Fatalf("leaf %q before parent %q in %v", p, parent, gaPaths)
				}
			}
		}
	}
	for i, p := range gaPaths {
		for j := 0; j < i; j++ {
			if strings.HasPrefix(gaPaths[j], p+".") {
				t.Fatalf("child %q appears before parent %q", gaPaths[j], p)
			}
		}
	}
}

func TestBuildSyncTasks_NestedDeletesDeepestFirst(t *testing.T) {
	changes := []SchemaChange{
		{
			ID: "d-root", Kind: ChangeDeleteField, Model: "exam",
			Field: &SyncField{Identifier: "routine", Path: "routine", FieldType: "repeated"},
		},
		{
			ID: "d-leaf", Kind: ChangeDeleteField, Model: "exam",
			Field: &SyncField{Identifier: "date_and_time", ParentField: "details", Path: "routine.details.date_and_time", FieldType: "date"},
		},
		{
			ID: "d-mid", Kind: ChangeDeleteField, Model: "exam",
			Field: &SyncField{Identifier: "details", ParentField: "routine", Path: "routine.details", FieldType: "repeated"},
		},
	}
	tasks := buildSyncTasks(changes)
	got := make([]string, 0, len(tasks))
	for _, task := range tasks {
		got = append(got, fieldSyncKey(*task.Change.Field))
	}
	want := []string{"routine.details.date_and_time", "routine.details", "routine"}
	for i, w := range want {
		if got[i] != w {
			t.Fatalf("delete order[%d] = %q, want %q (full %v)", i, got[i], w, got)
		}
	}
}

func TestCloseFieldDependencies_AutoIncludesAncestors(t *testing.T) {
	all := []SchemaChange{
		{ID: "a-tests", Kind: ChangeAddField, Model: "lab_order", Field: &SyncField{Identifier: "tests", Path: "tests", FieldType: "repeated"}, Summary: "Add tests"},
		{ID: "a-tlr", Kind: ChangeAddField, Model: "lab_order", Field: &SyncField{Identifier: "test_label_results", ParentField: "tests", Path: "tests.test_label_results", FieldType: "repeated"}, Summary: "Add tlr"},
		{ID: "a-label", Kind: ChangeAddField, Model: "lab_order", Field: &SyncField{Identifier: "label_id", ParentField: "test_label_results", Path: "tests.test_label_results.label_id", FieldType: "text"}, Summary: "Add label_id"},
	}
	selected := []SchemaChange{all[2]}
	closed, err := closeFieldDependencies(selected, all)
	if err != nil {
		t.Fatal(err)
	}
	if len(closed) != 3 {
		t.Fatalf("closed len = %d (%+v), want 3", len(closed), closed)
	}
	got := map[string]bool{}
	for _, ch := range closed {
		got[ch.ID] = true
		if ch.ID != "a-label" && !strings.Contains(ch.Summary, "required by") {
			t.Fatalf("ancestor %s missing required-by mark: %q", ch.ID, ch.Summary)
		}
	}
	for _, id := range []string{"a-tests", "a-tlr", "a-label"} {
		if !got[id] {
			t.Fatalf("missing %s", id)
		}
	}
	if err := preflightNestedAddOrder(closed); err != nil {
		t.Fatal(err)
	}
	tasks := buildSyncTasks(closed)
	if err := preflightNestedAddOrder(selectedFromTasks(tasks)); err != nil {
		t.Fatal(err)
	}
}

func TestPreflightNestedAddOrder_DetectsChildBeforeParent(t *testing.T) {
	bad := []SchemaChange{
		{ID: "child", Kind: ChangeAddField, Model: "m", Field: &SyncField{Identifier: "a", ParentField: "p", Path: "p.a"}},
		{ID: "parent", Kind: ChangeAddField, Model: "m", Field: &SyncField{Identifier: "p", Path: "p"}},
	}
	if err := preflightNestedAddOrder(bad); err == nil {
		t.Fatal("expected preflight error for child-before-parent")
	}
}

func TestValidateSyncModels_StaleParentAndListChildren(t *testing.T) {
	models := []SyncModel{{
		Name: "lab_order",
		Fields: []SyncField{{
			Identifier: "tests",
			FieldType:  "repeated",
			SubFieldInfo: []SyncField{
				{Identifier: "test_id", FieldType: "text", ParentField: "product_wise_quantity"},
				{
					Identifier: "koilonychias",
					FieldType:  "list",
					SubFieldInfo: []SyncField{
						{Identifier: "leukonychia", FieldType: "list"},
					},
				},
			},
		}},
	}}
	issues := validateSyncModels(models)
	if len(issues) < 2 {
		t.Fatalf("issues = %+v, want at least stale parent + list children", issues)
	}
	joined := formatSchemaValidationReport(issues)
	if !strings.Contains(joined, "stale parent_field") {
		t.Fatalf("missing stale parent: %s", joined)
	}
	if !strings.Contains(joined, "must not contain sub_field_info") {
		t.Fatalf("missing list children: %s", joined)
	}
}

func TestValidateSyncModels_CleanPasses(t *testing.T) {
	models := []SyncModel{{
		Name: "lab_order",
		Fields: []SyncField{{
			Identifier: "tests",
			FieldType:  "repeated",
			SubFieldInfo: []SyncField{
				{Identifier: "test_id", FieldType: "text", ParentField: "tests"},
				{
					Identifier: "test_label_results",
					FieldType:  "repeated",
					ParentField: "tests",
					SubFieldInfo: []SyncField{
						{Identifier: "label_id", FieldType: "text", ParentField: "test_label_results"},
					},
				},
			},
		}},
	}}
	if issues := validateSyncModels(models); len(issues) != 0 {
		t.Fatalf("unexpected issues: %s", formatSchemaValidationReport(issues))
	}
}

func TestValidateSyncModels_SystemCompositesAllowed(t *testing.T) {
	models := []SyncModel{{
		Name: "tenant",
		Fields: []SyncField{{
			Identifier: "bio",
			FieldType:  "multiline",
			SubFieldInfo: []SyncField{
				{Identifier: "html", FieldType: "text", ParentField: "bio"},
				{Identifier: "markdown", FieldType: "text", ParentField: "bio"},
				{Identifier: "text", FieldType: "text", ParentField: "bio"},
			},
		}},
	}}
	if issues := validateSyncModels(models); len(issues) != 0 {
		t.Fatalf("unexpected issues: %s", formatSchemaValidationReport(issues))
	}
}
