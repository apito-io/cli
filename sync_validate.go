package main

import (
	"fmt"
	"strings"
)

type schemaValidationIssue struct {
	Model   string
	Path    string
	Message string
}

func (i schemaValidationIssue) String() string {
	if i.Path == "" {
		return fmt.Sprintf("%s: %s", i.Model, i.Message)
	}
	return fmt.Sprintf("%s.%s: %s", i.Model, i.Path, i.Message)
}

// validateSyncModels checks nested field shapes before diff/apply.
// Scalar/list/date/number/boolean must not carry sub_field_info.
// multiline/media/geo are engine system composites and may ship fixed children.
// Tree-derived parent must match explicit parent_field when set.
// Duplicate full paths are errors.
func validateSyncModels(models []SyncModel) []schemaValidationIssue {
	var issues []schemaValidationIssue
	for _, model := range models {
		seenPaths := make(map[string]struct{})
		var walk func(fields []SyncField, parentID, pathPrefix string)
		walk = func(fields []SyncField, parentID, pathPrefix string) {
			for _, f := range fields {
				id := strings.TrimSpace(f.Identifier)
				if id == "" {
					issues = append(issues, schemaValidationIssue{
						Model:   model.Name,
						Path:    pathPrefix,
						Message: "field missing identifier",
					})
					continue
				}
				path := id
				if pathPrefix != "" {
					path = pathPrefix + "." + id
				}
				if _, dup := seenPaths[path]; dup {
					issues = append(issues, schemaValidationIssue{
						Model:   model.Name,
						Path:    path,
						Message: "duplicate full field path",
					})
				}
				seenPaths[path] = struct{}{}

				metaParent := strings.TrimSpace(f.ParentField)
				if metaParent != "" && parentID != "" && metaParent != parentID {
					issues = append(issues, schemaValidationIssue{
						Model: model.Name,
						Path:  path,
						Message: fmt.Sprintf(
							"stale parent_field %q (tree parent is %q)",
							metaParent, parentID,
						),
					})
				}

				ft := strings.ToLower(strings.TrimSpace(f.FieldType))
				hasChildren := len(f.SubFieldInfo) > 0
				switch ft {
				case "object", "repeated":
					// nested containers OK
				case "multiline", "media", "geo":
					// engine system composites ship fixed sub_field_info
				default:
					if hasChildren {
						issues = append(issues, schemaValidationIssue{
							Model: model.Name,
							Path:  path,
							Message: fmt.Sprintf(
								"field_type %q must not contain sub_field_info (use object/repeated)",
								f.FieldType,
							),
						})
					}
				}

				if hasChildren {
					walk(f.SubFieldInfo, id, path)
				}
			}
		}
		walk(model.Fields, "", "")
	}
	return issues
}

func formatSchemaValidationReport(issues []schemaValidationIssue) string {
	if len(issues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("schema validation failed (%d issue(s)):\n", len(issues)))
	for _, issue := range issues {
		b.WriteString("  - ")
		b.WriteString(issue.String())
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
