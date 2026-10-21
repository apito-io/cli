package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultTSCodegenUIPackage = "@apito-io/js-admin-ui-antd"

// ApitoTypeYAML represents the per-project apito-type.yml configuration file.
type ApitoTypeYAML struct {
	Project   string                         `yaml:"project,omitempty"`
	Language  string                         `yaml:"language,omitempty"`
	Framework string                         `yaml:"framework,omitempty"`
	Out       string                         `yaml:"out,omitempty"`
	UIPackage string                         `yaml:"ui_package,omitempty"`
	Models    map[string]ApitoTypeYAMLModel  `yaml:"models,omitempty"`
}

// ApitoTypeYAMLModel represents per-model overrides in apito-type.yml.
type ApitoTypeYAMLModel struct {
	SupportsConnection *bool             `yaml:"supports_connection,omitempty"`
	FullPayloadUpdate  *bool             `yaml:"full_payload_update,omitempty"`
	Relations          yaml.Node         `yaml:"relations,omitempty"`
	Aliases            map[string]string `yaml:"aliases,omitempty"`
	Fields             []string          `yaml:"fields,omitempty"`
}

// OrderedKV preserves key insertion order for deterministic codegen output.
type OrderedKV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PlannedTSModel is a fully resolved model plan ready for TypeScript/GraphQL emission.
type PlannedTSModel struct {
	Name               string            `json:"name"`
	Fields             []string          `json:"fields"`
	SupportsConnection bool              `json:"supportsConnection"`
	FullPayloadUpdate  bool              `json:"fullPayloadUpdate,omitempty"`
	ConnectionFields   []OrderedKV       `json:"connectionFields,omitempty"`
	AliasFields        map[string]string `json:"aliasFields,omitempty"`
}

// PlannedTSConfig is the complete codegen plan resolved from introspection + apito-type.yml.
type PlannedTSConfig struct {
	Schema    string           `json:"schema"`
	UIPackage string           `json:"uiPackage"`
	Models    []PlannedTSModel `json:"models"`
}

func loadApitoTypeYAML(dir string) (*ApitoTypeYAML, string, error) {
	candidates := []string{
		filepath.Join(dir, "apito-type.yml"),
		filepath.Join(dir, "apito-type.yaml"),
	}
	for _, p := range candidates {
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, "", err
		}
		var cfg ApitoTypeYAML
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return nil, "", fmt.Errorf("parse %s: %w", p, err)
		}
		return &cfg, p, nil
	}
	return nil, "", nil
}

func orderedRelationsFromNode(node *yaml.Node) ([]OrderedKV, error) {
	if node == nil || node.Kind == 0 {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("relations must be a YAML mapping")
	}
	var out []OrderedKV
	for i := 0; i+1 < len(node.Content); i += 2 {
		k := strings.TrimSpace(node.Content[i].Value)
		v := strings.TrimSpace(node.Content[i+1].Value)
		if k != "" && v != "" {
			out = append(out, OrderedKV{Key: k, Value: v})
		}
	}
	return out, nil
}

type introTypeRef struct {
	Kind   string        `json:"kind"`
	Name   string        `json:"name"`
	OfType *introTypeRef `json:"ofType"`
}

type introField struct {
	Name string                  `json:"name"`
	Args []struct{ Name string } `json:"args"`
	Type *introTypeRef           `json:"type"`
}

type introType struct {
	Kind        string                   `json:"kind"`
	Name        string                   `json:"name"`
	Fields      []introField             `json:"fields"`
	InputFields []struct{ Name string }  `json:"inputFields"`
}

type introRoot struct {
	Data *struct {
		Schema *introSchema `json:"__schema"`
	} `json:"data"`
	Schema *introSchema `json:"__schema"`
}

type introSchema struct {
	QueryType *struct {
		Name   string       `json:"name"`
		Fields []introField `json:"fields"`
	} `json:"queryType"`
	Types []introType `json:"types"`
}

func parseIntroSchema(rawBytes []byte) ([]introField, map[string]introType, error) {
	var root introRoot
	if err := json.Unmarshal(rawBytes, &root); err != nil {
		return nil, nil, fmt.Errorf("parse introspection JSON: %w", err)
	}
	schema := root.Schema
	if schema == nil && root.Data != nil {
		schema = root.Data.Schema
	}
	if schema == nil || len(schema.Types) == 0 {
		return nil, nil, fmt.Errorf("published schema introspection did not contain a schema")
	}
	byName := make(map[string]introType, len(schema.Types))
	for _, t := range schema.Types {
		if t.Name != "" {
			byName[t.Name] = t
		}
	}
	if schema.QueryType != nil && len(schema.QueryType.Fields) > 0 {
		return schema.QueryType.Fields, byName, nil
	}
	qName := "Query"
	if schema.QueryType != nil && schema.QueryType.Name != "" {
		qName = schema.QueryType.Name
	}
	qType := byName[qName]
	return qType.Fields, byName, nil
}

func unwrapIntroType(t *introTypeRef) introTypeRef {
	cur := t
	for cur != nil && cur.OfType != nil {
		cur = cur.OfType
	}
	if cur == nil {
		return introTypeRef{}
	}
	return *cur
}

func selectIntroFields(byName map[string]introType, typeName string, depth int, seen map[string]bool) []string {
	if depth > 5 || seen[typeName] {
		return nil
	}
	t, ok := byName[typeName]
	if !ok || len(t.Fields) == 0 {
		return nil
	}
	nextSeen := make(map[string]bool, len(seen)+1)
	for k, v := range seen {
		nextSeen[k] = v
	}
	nextSeen[typeName] = true
	var out []string
	for _, f := range t.Fields {
		if f.Name == "__typename" {
			continue
		}
		named := unwrapIntroType(f.Type)
		switch named.Kind {
		case "SCALAR", "ENUM":
			out = append(out, f.Name)
		case "OBJECT":
			if named.Name != "" {
				nested := selectIntroFields(byName, named.Name, depth+1, nextSeen)
				if len(nested) > 0 {
					out = append(out, fmt.Sprintf("%s { %s }", f.Name, strings.Join(nested, " ")))
				} else {
					out = append(out, f.Name)
				}
			}
		}
	}
	return out
}

var camelBoundaryRe = regexp.MustCompile(`([a-z0-9])([A-Z])`)

func splitCamelSegments(s string) []string {
	spaced := camelBoundaryRe.ReplaceAllString(s, "${1} ${2}")
	parts := strings.Fields(spaced)
	var out []string
	for _, p := range parts {
		clean := strings.ToLower(strings.TrimSpace(p))
		if clean != "" {
			out = append(out, clean)
		}
	}
	return out
}

func apitoSingularResourceName(name string) string {
	t := strings.TrimSpace(name)
	if strings.HasSuffix(t, "ListCount") {
		t = strings.TrimSuffix(t, "ListCount")
	} else if strings.HasSuffix(t, "List") {
		t = strings.TrimSuffix(t, "List")
	}
	t = strings.TrimSpace(t)
	if t == "" {
		return ""
	}
	if strings.Contains(t, "_") {
		parts := strings.Split(t, "_")
		var b strings.Builder
		for i, p := range parts {
			if p == "" {
				continue
			}
			if i == 0 {
				b.WriteString(strings.ToLower(p))
			} else {
				b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
			}
		}
		return b.String()
	}
	segs := splitCamelSegments(t)
	if len(segs) == 0 {
		return strings.ToLower(t)
	}
	var b strings.Builder
	for i, s := range segs {
		if i == 0 {
			b.WriteString(strings.ToLower(s))
		} else {
			b.WriteString(strings.ToUpper(s[:1]) + strings.ToLower(s[1:]))
		}
	}
	return b.String()
}

func apitoMultipleResourceName(name string) string {
	return apitoSingularResourceName(name) + "List"
}

func pascalFromModelID(modelID string) string {
	if modelID == "" {
		return ""
	}
	var parts []string
	if strings.Contains(modelID, "_") {
		parts = strings.Split(modelID, "_")
	} else {
		parts = splitCamelSegments(modelID)
	}
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]) + strings.ToLower(p[1:]))
	}
	return b.String()
}

func apitoSingularGraphQLTypeName(resource string) string {
	return pascalFromModelID(apitoSingularResourceName(resource))
}

func apitoListGraphQLTypeName(resource string) string {
	return apitoSingularGraphQLTypeName(resource) + "List"
}

func apitoStoredSnakeModelID(resource string) string {
	singular := apitoSingularResourceName(resource)
	if strings.Contains(singular, "_") {
		return singular
	}
	return strings.Join(splitCamelSegments(singular), "_")
}

func apitoGraphQLComposedTypeName(modelID, suffix string) string {
	singular := apitoSingularResourceName(modelID)
	var modelSegs []string
	if strings.Contains(singular, "_") {
		for _, p := range strings.Split(singular, "_") {
			if p != "" {
				modelSegs = append(modelSegs, strings.ToLower(p))
			}
		}
	} else {
		modelSegs = splitCamelSegments(singular)
	}
	var extra []string
	for _, chunk := range strings.Split(strings.TrimPrefix(suffix, "_"), "_") {
		if chunk != "" {
			extra = append(extra, splitCamelSegments(chunk)...)
		}
	}
	all := append(modelSegs, extra...)
	var titled []string
	for _, p := range all {
		if p != "" {
			titled = append(titled, strings.ToUpper(p[:1])+strings.ToLower(p[1:]))
		}
	}
	return strings.Join(titled, "_")
}

func apitoWhereRelationFilterConditionType(resource string) string {
	return strings.ToUpper(apitoStoredSnakeModelID(resource) + "_Where_Relation_Filter_Condition")
}

func apitoWhereInputType(resource string) string {
	return strings.ToUpper(apitoListGraphQLTypeName(resource) + "_Input_Where_Payload")
}

func apitoSortInputType(resource string) string {
	return strings.ToUpper(apitoListGraphQLTypeName(resource) + "_Input_Sort_Payload")
}

func apitoListKeyConditionType(resource string) string {
	return strings.ToUpper(apitoListGraphQLTypeName(resource) + "_Key_Condition")
}

func apitoListCountKeyConditionType(resource string) string {
	return strings.ToUpper(apitoGraphQLComposedTypeName(resource, "List_Count") + "_Key_Condition")
}

func apitoListCountWhereInputType(resource string) string {
	return strings.ToUpper(apitoGraphQLComposedTypeName(resource, "List_Count") + "_Input_Where_Payload")
}

func apitoGraphqlConnectionFieldFromMetaKey(key string) string {
	k := strings.TrimSpace(key)
	if k == "" {
		return k
	}
	for _, r := range k {
		if r >= 'A' && r <= 'Z' {
			return k
		}
	}
	if strings.HasSuffix(strings.ToLower(k), "_list") {
		base := k[:len(k)-len("_list")]
		return apitoStoredSnakeModelID(base) + "_list"
	}
	return apitoStoredSnakeModelID(k)
}

func formatApitoConnectionSubselections(connectionFields []OrderedKV, aliasFields map[string]string) string {
	if len(connectionFields) == 0 {
		return ""
	}
	var lines []string
	for _, kv := range connectionFields {
		key := kv.Key
		selection := kv.Value
		rawTarget := ""
		if aliasFields != nil {
			rawTarget = strings.TrimSpace(aliasFields[key])
		}
		targetField := apitoGraphqlConnectionFieldFromMetaKey(key)
		if rawTarget != "" {
			targetField = apitoGraphqlConnectionFieldFromMetaKey(rawTarget)
			if key != targetField {
				lines = append(lines, fmt.Sprintf("%s: %s { %s }", key, targetField, selection))
				continue
			}
		}
		lines = append(lines, fmt.Sprintf("%s { %s }", targetField, selection))
	}
	return strings.Join(lines, "\n")
}

func snakeFromListField(listFieldName string) string {
	camel := strings.TrimSuffix(listFieldName, "List")
	spaced := camelBoundaryRe.ReplaceAllString(camel, "${1}_${2}")
	return strings.ToLower(spaced)
}

func snakeFromRawModel(typeName string) string {
	base := strings.TrimSuffix(typeName, "_Raw_Model")
	parts := strings.Split(base, "_")
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return strings.Join(out, "_")
}

var fullPayloadFieldRe = regexp.MustCompile(`(?i)list|history|config|stages|tests|items|chambers|medicine|complaint`)

func detectFullPayloadUpdate(byName map[string]introType, rawModelName string) bool {
	t, ok := byName[rawModelName]
	if !ok {
		return false
	}
	for _, f := range t.Fields {
		nt := unwrapIntroType(f.Type)
		if nt.Kind == "OBJECT" && (f.Name == "tags" || fullPayloadFieldRe.MatchString(f.Name)) {
			return true
		}
	}
	return false
}

func filterLiveConnections(byName map[string]introType, model string, rels []OrderedKV) []OrderedKV {
	if len(rels) == 0 {
		return nil
	}
	objName := apitoSingularGraphQLTypeName(model)
	t, ok := byName[objName]
	if !ok || len(t.Fields) == 0 {
		return rels
	}
	live := make(map[string]bool, len(t.Fields))
	for _, f := range t.Fields {
		live[f.Name] = true
	}
	var out []OrderedKV
	for _, kv := range rels {
		if live[kv.Key] && strings.TrimSpace(kv.Value) != "" {
			out = append(out, kv)
		}
	}
	return out
}

func planTSConfigFromSchema(rawBytes []byte, schemaPath string, requestedModels []string, includeRelations bool, uiPackage string, typeYAML *ApitoTypeYAML) (*PlannedTSConfig, error) {
	queryFields, byName, err := parseIntroSchema(rawBytes)
	if err != nil {
		return nil, err
	}
	reqSet := make(map[string]bool, len(requestedModels))
	for _, m := range requestedModels {
		if s := strings.TrimSpace(m); s != "" {
			reqSet[s] = true
		}
	}

	seen := make(map[string]bool)
	var models []PlannedTSModel

	applyModelOverrides := func(name string, fields []string, defaultSupportsConn bool, rawName string) PlannedTSModel {
		supportsConn := defaultSupportsConn
		fullPayload := false
		var connFields []OrderedKV
		var aliases map[string]string

		if typeYAML != nil {
			if mCfg, ok := typeYAML.Models[name]; ok {
				if len(mCfg.Fields) > 0 {
					fields = mCfg.Fields
				}
				if mCfg.SupportsConnection != nil {
					supportsConn = *mCfg.SupportsConnection
				}
				if mCfg.FullPayloadUpdate != nil {
					fullPayload = *mCfg.FullPayloadUpdate
				}
				if includeRelations {
					if rels, err := orderedRelationsFromNode(&mCfg.Relations); err == nil && len(rels) > 0 {
						connFields = filterLiveConnections(byName, name, rels)
					}
					if len(mCfg.Aliases) > 0 {
						aliases = mCfg.Aliases
					}
				}
			}
		}
		return PlannedTSModel{
			Name:               name,
			Fields:             fields,
			SupportsConnection: supportsConn,
			FullPayloadUpdate:  fullPayload,
			ConnectionFields:   connFields,
			AliasFields:        aliases,
		}
	}

	for _, qf := range queryFields {
		if !strings.HasSuffix(qf.Name, "List") || strings.HasSuffix(qf.Name, "ListCount") {
			continue
		}
		name := snakeFromListField(qf.Name)
		if name == "" || seen[name] {
			continue
		}
		if len(reqSet) > 0 && !reqSet[name] {
			continue
		}
		rawName := apitoGraphQLComposedTypeName(name, "Raw_Model")
		fields := selectIntroFields(byName, rawName, 0, nil)
		if len(fields) == 0 {
			continue
		}
		seen[name] = true
		hasRelationArg := false
		for _, arg := range qf.Args {
			if arg.Name == "relation" {
				hasRelationArg = true
				break
			}
		}
		models = append(models, applyModelOverrides(name, fields, hasRelationArg, rawName))
	}

	var typeNames []string
	for k := range byName {
		if strings.HasSuffix(k, "_Raw_Model") {
			typeNames = append(typeNames, k)
		}
	}
	sort.Strings(typeNames)
	for _, rawTypeName := range typeNames {
		name := snakeFromRawModel(rawTypeName)
		if name == "" || seen[name] {
			continue
		}
		singular := apitoSingularResourceName(name)
		hasQuery := false
		for _, qf := range queryFields {
			if qf.Name == singular || qf.Name == singular+"List" {
				hasQuery = true
				break
			}
		}
		if !hasQuery {
			continue
		}
		if len(reqSet) > 0 && !reqSet[name] {
			continue
		}
		fields := selectIntroFields(byName, rawTypeName, 0, nil)
		if len(fields) == 0 {
			continue
		}
		seen[name] = true
		models = append(models, applyModelOverrides(name, fields, false, rawTypeName))
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})

	if len(reqSet) > 0 {
		found := make(map[string]bool, len(models))
		for _, m := range models {
			found[m.Name] = true
		}
		var missing []string
		for _, req := range requestedModels {
			req = strings.TrimSpace(req)
			if req != "" && !found[req] {
				missing = append(missing, req)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("published schema has no model: %s", strings.Join(missing, ", "))
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("published schema did not contain any models")
	}

	pkg := strings.TrimSpace(uiPackage)
	if pkg == "" && typeYAML != nil {
		pkg = strings.TrimSpace(typeYAML.UIPackage)
	}
	if pkg == "" {
		pkg = defaultTSCodegenUIPackage
	}

	return &PlannedTSConfig{
		Schema:    schemaPath,
		UIPackage: pkg,
		Models:    models,
	}, nil
}

// generateModelGraphqlFile builds the .graphql operations file for a single model.
func generateModelGraphqlFile(model PlannedTSModel) string {
	var fields []string
	for _, f := range model.Fields {
		if f != "id" {
			fields = append(fields, f)
		}
	}
	if len(fields) == 0 {
		fields = []string{"id"}
	}
	connBlock := formatApitoConnectionSubselections(model.ConnectionFields, model.AliasFields)

	listPascal := apitoListGraphQLTypeName(model.Name)
	singularPascal := apitoSingularGraphQLTypeName(model.Name)
	listField := apitoMultipleResourceName(model.Name)
	countField := listField + "Count"
	singularField := apitoSingularResourceName(model.Name)

	var listQuery string
	if !model.SupportsConnection {
		vars := strings.Join([]string{
			fmt.Sprintf("$where: %s", apitoWhereInputType(model.Name)),
			fmt.Sprintf("$whereCount: %s", apitoListCountWhereInputType(model.Name)),
			fmt.Sprintf("$sort: %s", apitoSortInputType(model.Name)),
			fmt.Sprintf("$_key: %s", apitoListKeyConditionType(model.Name)),
			fmt.Sprintf("$_keyCount: %s", apitoListCountKeyConditionType(model.Name)),
			"$page: Int",
			"$limit: Int",
		}, ",\n    ")
		listQuery = fmt.Sprintf(`query Get%s(
    %s
) {
  %s(where: $where, sort: $sort, _key: $_key, page: $page, limit: $limit) {
    id
    data {
      %s
    }
    meta {
      created_at
      status
      updated_at
    }
  }
  %s(where: $whereCount, _key: $_keyCount, page: $page, limit: $limit) {
    total
  }
}`, listPascal, vars, listField, strings.Join(fields, "\n      "), countField)
	} else {
		vars := strings.Join([]string{
			fmt.Sprintf("$relation: %s", apitoWhereRelationFilterConditionType(model.Name)),
			fmt.Sprintf("$where: %s", apitoWhereInputType(model.Name)),
			fmt.Sprintf("$whereCount: %s", apitoListCountWhereInputType(model.Name)),
			fmt.Sprintf("$sort: %s", apitoSortInputType(model.Name)),
			fmt.Sprintf("$_key: %s", apitoListKeyConditionType(model.Name)),
			fmt.Sprintf("$_keyCount: %s", apitoListCountKeyConditionType(model.Name)),
			"$page: Int",
			"$limit: Int",
		}, "\n    ")
		connLines := "\n"
		if connBlock != "" {
			connLines = "\n    " + strings.ReplaceAll(connBlock, "\n", "\n    ") + "\n"
		}
		listQuery = fmt.Sprintf(`query Get%s(
    %s
) {
  %s(relation: $relation, where: $where, sort: $sort, _key: $_key, page: $page, limit: $limit) {
    id
    data {
      %s
    }
    %s
    meta {
      created_at
      status
      updated_at
    }
  }    %s(relation: $relation, where: $whereCount, _key: $_keyCount, page: $page, limit: $limit) {
      total
    }
}`, listPascal, vars, listField, strings.Join(fields, "\n      "), connLines, countField)
	}

	connLines := "\n"
	if connBlock != "" {
		connLines = "\n    " + strings.ReplaceAll(connBlock, "\n", "\n    ") + "\n"
	}
	getQuery := fmt.Sprintf(`query Get%s($id: String!) {
  %s(_id: $id) {
    id
    data {
      %s
    }
    %s
    meta {
      created_at
      status
      updated_at
    }
  }
}`, singularPascal, singularField, strings.Join(fields, "\n      "), connLines)

	createPayload := apitoGraphQLComposedTypeName(model.Name, "Create_Payload")
	var createMutation string
	if !model.SupportsConnection {
		createMutation = fmt.Sprintf(`mutation Create%s($payload: %s!) {
  create%s(payload: $payload, status: published) {
    id
    data {
      %s
    }
    meta {
      created_at
      status
      updated_at
    }
  }
}`, singularPascal, createPayload, singularPascal, strings.Join(fields, "\n      "))
	} else {
		connectPayload := apitoGraphQLComposedTypeName(model.Name, "Relation_Connect_Payload")
		createMutation = fmt.Sprintf(`mutation Create%s($payload: %s!, $connect: %s) {
  create%s(payload: $payload, connect: $connect, status: published) {
    id
    data {
      %s
    }
    meta {
      created_at
      status
      updated_at
    }
  }
}`, singularPascal, createPayload, connectPayload, singularPascal, strings.Join(fields, "\n      "))
	}

	updatePayload := apitoGraphQLComposedTypeName(model.Name, "Update_Payload")
	var updateMutation string
	if !model.SupportsConnection {
		updateMutation = fmt.Sprintf(`mutation Update%s(
    $id: String!,
    $deltaUpdate: Boolean,
    $payload: %s!
) {
  update%s(_id: $id, deltaUpdate: $deltaUpdate, payload: $payload, status: published) {
    id
    data {
      %s
    }
    meta {
      created_at
      status
      updated_at
    }
  }
}`, singularPascal, updatePayload, singularPascal, strings.Join(fields, "\n      "))
	} else {
		connectPayload := apitoGraphQLComposedTypeName(model.Name, "Relation_Connect_Payload")
		disconnectPayload := apitoGraphQLComposedTypeName(model.Name, "Relation_Disconnect_Payload")
		updateMutation = fmt.Sprintf(`mutation Update%s(
    $id: String!,
    $deltaUpdate: Boolean,
    $payload: %s!,
    $connect: %s,
    $disconnect: %s
) {
  update%s(_id: $id, deltaUpdate: $deltaUpdate, payload: $payload, connect: $connect, disconnect: $disconnect, status: published) {
    id
    data {
      %s
    }
    meta {
      created_at
      status
      updated_at
    }
  }
}`, singularPascal, updatePayload, connectPayload, disconnectPayload, singularPascal, strings.Join(fields, "\n      "))
	}

	deleteMutation := fmt.Sprintf(`mutation Delete%s($ids: [String]!) {
  delete%s(_ids: $ids) {
    response
  }
}`, singularPascal, singularPascal)

	return strings.Join([]string{
		"# AUTO-GENERATED — DO NOT EDIT",
		"# Model: " + model.Name,
		"",
		listQuery,
		"",
		getQuery,
		"",
		createMutation,
		"",
		updateMutation,
		"",
		deleteMutation,
		"",
	}, "\n")
}

func tsPascalCase(snake string) string {
	if strings.Contains(snake, "_") {
		parts := strings.Split(snake, "_")
		var b strings.Builder
		for _, s := range parts {
			if s != "" {
				b.WriteString(strings.ToUpper(s[:1]) + s[1:])
			}
		}
		return b.String()
	}
	if snake == "" {
		return ""
	}
	return strings.ToUpper(snake[:1]) + snake[1:]
}

func tsGqlResourceName(model string) string {
	parts := strings.Split(model, "_")
	var b strings.Builder
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			b.WriteString(part)
		} else {
			b.WriteString(strings.ToUpper(part[:1]) + part[1:])
		}
	}
	return b.String()
}

func tsCamelCase(model string) string {
	return tsGqlResourceName(model)
}

func formatOrderedKVJSON(kvs []OrderedKV) string {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, kv := range kvs {
		kJSON, _ := json.Marshal(kv.Key)
		vJSON, _ := json.Marshal(kv.Value)
		buf.WriteString(fmt.Sprintf("  %s: %s", string(kJSON), string(vJSON)))
		if i+1 < len(kvs) {
			buf.WriteString(",")
		}
		buf.WriteString("\n")
	}
	buf.WriteString("}")
	return buf.String()
}

func writeTSProjectFacades(cfg *PlannedTSConfig, outDir string) error {
	uiPackage := cfg.UIPackage
	if uiPackage == "" {
		uiPackage = defaultTSCodegenUIPackage
	}
	modelsDir := filepath.Join(outDir, "models")
	if err := os.MkdirAll(modelsDir, 0o755); err != nil {
		return err
	}

	for _, model := range cfg.Models {
		pascal := tsPascalCase(model.Name)
		gql := tsGqlResourceName(model.Name)
		camel := tsCamelCase(model.Name)
		modelDir := filepath.Join(modelsDir, model.Name)
		if err := os.MkdirAll(modelDir, 0o755); err != nil {
			return err
		}

		fieldsConst := camel + "Fields"
		supportsConn := model.SupportsConnection
		deltaUpdateLine := ""
		if model.FullPayloadUpdate {
			deltaUpdateLine = "\n    deltaUpdate: false,"
		}

		var relTypeLines, relFieldLines, metaExports string
		if len(model.ConnectionFields) > 0 {
			var rTypes, rFields []string
			for _, kv := range model.ConnectionFields {
				keyPascal := tsPascalCase(kv.Key)
				rTypes = append(rTypes, fmt.Sprintf(`export type %sRelation = NonNullable<NonNullable<Get%sQuery["%s"]>["%s"]>;`, keyPascal, pascal, gql, kv.Key))
				rFields = append(rFields, fmt.Sprintf(`  %s?: %sRelation | %sRelation[];`, kv.Key, keyPascal, keyPascal))
			}
			relTypeLines = strings.Join(rTypes, "\n") + "\n"
			relFieldLines = "\n" + strings.Join(rFields, "\n")
			metaExports = fmt.Sprintf(`
export const %sConnectionFields = %s as const;
export const %sListMeta = {
  fields: %sFields,
  connectionFields: %sConnectionFields,
} as const;
`, camel, formatOrderedKVJSON(model.ConnectionFields), camel, camel, camel)
		}

		var fieldQuoted []string
		for _, f := range model.Fields {
			q, _ := json.Marshal(f)
			fieldQuoted = append(fieldQuoted, fmt.Sprintf("  %s,", string(q)))
		}

		metaContent := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. */
import type { Get%sListQuery, Get%sQuery } from "../../types";

type %sListItem = NonNullable<
  NonNullable<Get%sListQuery["%sList"]>[number]
>;
export type %sMeta = NonNullable<%sListItem["meta"]>;
export type %sData = NonNullable<%sListItem["data"]>;
%sexport type %s = {
  id: string;
  data: %sData;
  meta: %sMeta;%s
};

export const %s = [
%s
] as const;
%s`,
			pascal, pascal,
			pascal, pascal, gql,
			pascal, pascal,
			pascal, pascal,
			relTypeLines, pascal,
			pascal, pascal, relFieldLines,
			fieldsConst, strings.Join(fieldQuoted, "\n"),
			metaExports,
		)
		if !strings.HasSuffix(metaContent, "\n") {
			metaContent += "\n"
		}
		if err := os.WriteFile(filepath.Join(modelDir, "meta.ts"), []byte(metaContent), 0o644); err != nil {
			return err
		}

		indexContent := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. */
"use client";

import { useListPage, useFormPage, useShowPage, useDeleteRecord } from "@apito-io/js-admin-sdk/react";
import type { CrudFilter, CrudSort } from "@apito-io/js-admin-sdk/react";
import {
  useGet%sListQuery,
  useGet%sQuery,
  Create%sDocument,
  Update%sDocument,
  Delete%sDocument,
} from "../../hooks";

import { %s, type %s, type %sData } from "./meta";

export * from "./meta";

const RESOURCE = "%s" as const;
const LIST_KEY = "%sList";
const COUNT_KEY = "%sListCount";
const ONE_KEY = "%s";

export function use%sList(options?: {
  enabled?: boolean;
  initialFilters?: CrudFilter[];
  permanentFilters?: CrudFilter[];
  initialSorters?: CrudSort[];
  initialPagination?: { current: number; pageSize: number };
}) {
  return useListPage<%s>({
    resource: RESOURCE,
    listKey: LIST_KEY,
    countKey: COUNT_KEY,
    supportsConnection: %t,
    enabled: options?.enabled,
    initialFilters: options?.initialFilters,
    permanentFilters: options?.permanentFilters,
    initialSorters: options?.initialSorters,
    initialPagination: options?.initialPagination,
    useListQuery: (_fetcher, variables, opts) =>
      useGet%sListQuery(variables, opts),
  });
}

export function use%s(id: string, options?: { enabled?: boolean }) {
  return useShowPage<%s>({
    id,
    oneKey: ONE_KEY,
    useOneQuery: (_fetcher, vars, opts) =>
      useGet%sQuery(vars, opts),
  });
}

export function use%sForm(id?: string) {
  return useFormPage({
    resource: RESOURCE,
    id,
    oneKey: ONE_KEY,%s
    useOneQuery: (_fetcher, vars, opts) =>
      useGet%sQuery(vars, opts),
    createDocument: Create%sDocument,
    updateDocument: Update%sDocument,
  });
}

export function useDelete%s() {
  return useDeleteRecord({
    deleteDocument: Delete%sDocument,
  });
}

export { %s };
export type { %sData };

export { List%s, Create%s, Edit%s, Show%s, Delete%sButton, Create%sButton } from "./ui";
`,
			pascal, pascal, pascal, pascal, pascal,
			fieldsConst, pascal, pascal,
			model.Name, gql, gql, gql,
			pascal, pascal, supportsConn, pascal,
			pascal, pascal, pascal,
			pascal, deltaUpdateLine, pascal, pascal, pascal,
			pascal, pascal,
			fieldsConst, pascal,
			pascal, pascal, pascal, pascal, pascal, pascal,
		)
		if err := os.WriteFile(filepath.Join(modelDir, "index.ts"), []byte(indexContent), 0o644); err != nil {
			return err
		}

		uiContent := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. Wires %s primitives + model hooks. */
"use client";

import {
  ApitoList,
  ApitoCreate,
  ApitoEdit,
  ApitoShow,
  ApitoDeleteButton,
  ApitoCreateButton,
} from "%s";
import type {
  ApitoListProps,
  ApitoCreateProps,
  ApitoEditProps,
  ApitoShowProps,
  ApitoDeleteButtonProps,
  ApitoCreateButtonProps,
} from "@apito-io/js-admin-sdk/ui";

import {
  use%sList,
  use%sForm,
  use%s,
  useDelete%s,
} from "./index";
import type { %s } from "./meta";

export function List%s(
  props: Omit<ApitoListProps<%s>, keyof ReturnType<typeof use%sList>> &
    Partial<ApitoListProps<%s>>,
) {
  const list = use%sList();
  return (
    <ApitoList<%s>
      resource={RESOURCE}
      createRoute={`+"`/${RESOURCE}/create`"+`}
      editRoute={`+"`/${RESOURCE}/edit/$id`"+`}
      showRoute={`+"`/${RESOURCE}/show/$id`"+`}
      {...list}
      {...props}
      pagination={props.pagination ?? list.pagination}
      onPaginationChange={props.onPaginationChange ?? list.setPagination}
    />
  );
}

export function Create%s(
  props: Omit<ApitoCreateProps, "onFinish" | "initialValues" | "loading" | "saving"> &
    Partial<ApitoCreateProps>,
) {
  const form = use%sForm();
  return (
    <ApitoCreate
      resource={RESOURCE}
      initialValues={form.initialValues}
      loading={form.isLoading}
      saving={form.isSaving}
      onFinish={form.save}
      {...props}
    />
  );
}

export function Edit%s(
  props: { id: string } & Omit<ApitoEditProps, "onFinish" | "initialValues" | "loading" | "saving"> &
    Partial<ApitoEditProps>,
) {
  const form = use%sForm(props.id);
  return (
    <ApitoEdit
      resource={RESOURCE}
      id={props.id}
      initialValues={form.initialValues}
      loading={form.isLoading}
      saving={form.isSaving}
      onFinish={form.save}
      {...props}
    />
  );
}

export function Show%s(
  props: { id: string } & Omit<ApitoShowProps<%s>, "record" | "loading"> &
    Partial<ApitoShowProps<%s>>,
) {
  const show = use%s(props.id);
  return (
    <ApitoShow<%s>
      resource={RESOURCE}
      record={show.record}
      loading={show.isLoading}
      {...props}
    />
  );
}

export function Delete%sButton(
  props: Omit<ApitoDeleteButtonProps, "onConfirm" | "loading"> &
    Partial<ApitoDeleteButtonProps>,
) {
  const del = useDelete%s();
  return (
    <ApitoDeleteButton
      resource={RESOURCE}
      loading={del.isDeleting}
      onConfirm={() => del.remove(props.recordItemId)}
      {...props}
    />
  );
}

export function Create%sButton(
  props: Omit<ApitoCreateButtonProps, "resource" | "createRoute"> &
    Partial<ApitoCreateButtonProps>,
) {
  return (
    <ApitoCreateButton
      resource={RESOURCE}
      createRoute={`+"`/${RESOURCE}/create`"+`}
      {...props}
    />
  );
}

const RESOURCE = "%s";
`,
			uiPackage, uiPackage,
			pascal, pascal, pascal, pascal, pascal,
			pascal, pascal, pascal, pascal, pascal, pascal,
			pascal, pascal,
			pascal, pascal,
			pascal, pascal, pascal, pascal, pascal,
			pascal, pascal,
			pascal,
			model.Name,
		)
		if err := os.WriteFile(filepath.Join(modelDir, "ui.tsx"), []byte(uiContent), 0o644); err != nil {
			return err
		}
	}

	var modelExports []string
	for _, m := range cfg.Models {
		modelExports = append(modelExports, fmt.Sprintf(`export * from "./models/%s";`, m.Name))
	}
	rootIndex := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. */
export * from "./types";
export * from "./hooks";
export * from "./sdk";
export * from "./resource-documents";
export * from "./resource-registry";
%s
`, strings.Join(modelExports, "\n"))
	return os.WriteFile(filepath.Join(outDir, "index.ts"), []byte(rootIndex), 0o644)
}

func writeTSResourceDocuments(cfg *PlannedTSConfig, outDir string) error {
	var docImports, createDocs, updateDocs, deleteDocs, listDocs, oneDocs []string
	var listKeys, countKeys, oneKeys, supportsConns, resNames []string

	for _, m := range cfg.Models {
		pascal := tsPascalCase(m.Name)
		gql := tsGqlResourceName(m.Name)
		docImports = append(docImports, fmt.Sprintf("  Create%sDocument,\n  Update%sDocument,\n  Delete%sDocument,\n  Get%sListDocument,\n  Get%sDocument,", pascal, pascal, pascal, pascal, pascal))
		createDocs = append(createDocs, fmt.Sprintf("  %s: Create%sDocument,", m.Name, pascal))
		updateDocs = append(updateDocs, fmt.Sprintf("  %s: Update%sDocument,", m.Name, pascal))
		deleteDocs = append(deleteDocs, fmt.Sprintf("  %s: Delete%sDocument,", m.Name, pascal))
		listDocs = append(listDocs, fmt.Sprintf("  %s: Get%sListDocument,", m.Name, pascal))
		oneDocs = append(oneDocs, fmt.Sprintf("  %s: Get%sDocument,", m.Name, pascal))
		listKeys = append(listKeys, fmt.Sprintf(`  %s: "%sList",`, m.Name, gql))
		countKeys = append(countKeys, fmt.Sprintf(`  %s: "%sListCount",`, m.Name, gql))
		oneKeys = append(oneKeys, fmt.Sprintf(`  %s: "%s",`, m.Name, gql))
		supportsConns = append(supportsConns, fmt.Sprintf("  %s: %t,", m.Name, m.SupportsConnection))
		resNames = append(resNames, fmt.Sprintf(`"%s"`, m.Name))
	}

	body := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. */
import type {
  ApitoResourceRegistry,
  GraphQLQueryInput,
} from "@apito-io/js-admin-sdk";
import {
%s
} from "./sdk";
export {
%s
} from "./sdk";

export type ApitoResourceName = %s;

export const APITO_CREATE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_UPDATE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_DELETE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_LIST_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_ONE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

const LIST_KEYS: Record<string, string> = {
%s
};

const COUNT_KEYS: Record<string, string> = {
%s
};

const ONE_KEYS: Record<string, string> = {
%s
};

const SUPPORTS_CONNECTION: Record<string, boolean> = {
%s
};

export const apitoServerResourceRegistry: ApitoResourceRegistry = {
  getListDocument: (resource) => APITO_LIST_DOCUMENTS[resource],
  getOneDocument: (resource) => APITO_ONE_DOCUMENTS[resource],
  getCreateDocument: (resource) => APITO_CREATE_DOCUMENTS[resource],
  getUpdateDocument: (resource) => APITO_UPDATE_DOCUMENTS[resource],
  getDeleteDocument: (resource) => APITO_DELETE_DOCUMENTS[resource],
  getListKey: (resource) => LIST_KEYS[resource] ?? `+"`${resource}List`"+`,
  getCountKey: (resource) => COUNT_KEYS[resource] ?? `+"`${resource}ListCount`"+`,
  getOneKey: (resource) => ONE_KEYS[resource] ?? resource,
  supportsConnection: (resource) => SUPPORTS_CONNECTION[resource] !== false,
};
`,
		strings.Join(docImports, "\n"),
		strings.Join(docImports, "\n"),
		strings.Join(resNames, " | "),
		strings.Join(createDocs, "\n"),
		strings.Join(updateDocs, "\n"),
		strings.Join(deleteDocs, "\n"),
		strings.Join(listDocs, "\n"),
		strings.Join(oneDocs, "\n"),
		strings.Join(listKeys, "\n"),
		strings.Join(countKeys, "\n"),
		strings.Join(oneKeys, "\n"),
		strings.Join(supportsConns, "\n"),
	)
	return os.WriteFile(filepath.Join(outDir, "resource-documents.ts"), []byte(body), 0o644)
}

func writeTSResourceRegistry(cfg *PlannedTSConfig, outDir string) error {
	var hookImports, docImports, listHooks, formHooks, deleteHooks []string
	var createDocs, updateDocs, deleteDocs, listKeys, countKeys, oneKeys []string
	var listDocs, oneDocs, supportsConns, resNames []string

	for _, m := range cfg.Models {
		pascal := tsPascalCase(m.Name)
		gql := tsGqlResourceName(m.Name)
		hookImports = append(hookImports, fmt.Sprintf(`import { use%sList, use%sForm, useDelete%s } from "./models/%s";`, pascal, pascal, pascal, m.Name))
		docImports = append(docImports, fmt.Sprintf("  Create%sDocument,\n  Update%sDocument,\n  Delete%sDocument,\n  Get%sListDocument,\n  Get%sDocument,", pascal, pascal, pascal, pascal, pascal))
		listHooks = append(listHooks, fmt.Sprintf("  %s: use%sList,", m.Name, pascal))
		formHooks = append(formHooks, fmt.Sprintf("  %s: use%sForm,", m.Name, pascal))
		deleteHooks = append(deleteHooks, fmt.Sprintf("  %s: useDelete%s,", m.Name, pascal))
		createDocs = append(createDocs, fmt.Sprintf("  %s: Create%sDocument,", m.Name, pascal))
		updateDocs = append(updateDocs, fmt.Sprintf("  %s: Update%sDocument,", m.Name, pascal))
		deleteDocs = append(deleteDocs, fmt.Sprintf("  %s: Delete%sDocument,", m.Name, pascal))
		listKeys = append(listKeys, fmt.Sprintf(`  %s: "%sList",`, m.Name, gql))
		countKeys = append(countKeys, fmt.Sprintf(`  %s: "%sListCount",`, m.Name, gql))
		oneKeys = append(oneKeys, fmt.Sprintf(`  %s: "%s",`, m.Name, gql))
		listDocs = append(listDocs, fmt.Sprintf("  %s: Get%sListDocument,", m.Name, pascal))
		oneDocs = append(oneDocs, fmt.Sprintf("  %s: Get%sDocument,", m.Name, pascal))
		supportsConns = append(supportsConns, fmt.Sprintf("  %s: %t,", m.Name, m.SupportsConnection))
		resNames = append(resNames, fmt.Sprintf(`"%s"`, m.Name))
	}

	body := fmt.Sprintf(`/** AUTO-GENERATED — do not edit. */
"use client";

import type {
  ApitoResourceRegistry,
  CrudFilter,
  GraphQLQueryInput,
} from "@apito-io/js-admin-sdk/react";
%s
import {
%s
} from "./resource-documents";

export type ApitoResourceName = %s;

export const APITO_LIST_HOOKS = {
%s
} as const;

export const APITO_FORM_HOOKS = {
%s
} as const;

export const APITO_DELETE_HOOKS = {
%s
} as const;

export const APITO_CREATE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_UPDATE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

export const APITO_DELETE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

const LIST_KEYS: Record<string, string> = {
%s
};

const COUNT_KEYS: Record<string, string> = {
%s
};

const ONE_KEYS: Record<string, string> = {
%s
};

const LIST_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

const ONE_DOCUMENTS = {
%s
} as unknown as Record<string, GraphQLQueryInput>;

const SUPPORTS_CONNECTION: Record<string, boolean> = {
%s
};

export const apitoResourceRegistry: ApitoResourceRegistry = {
  getListDocument: (resource) => LIST_DOCUMENTS[resource],
  getOneDocument: (resource) => ONE_DOCUMENTS[resource],
  getCreateDocument: (resource) => APITO_CREATE_DOCUMENTS[resource],
  getUpdateDocument: (resource) => APITO_UPDATE_DOCUMENTS[resource],
  getDeleteDocument: (resource) => APITO_DELETE_DOCUMENTS[resource],
  getListKey: (resource) => LIST_KEYS[resource] ?? `+"`${resource}List`"+`,
  getCountKey: (resource) => COUNT_KEYS[resource] ?? `+"`${resource}ListCount`"+`,
  getOneKey: (resource) => ONE_KEYS[resource] ?? resource,
  supportsConnection: (resource) => SUPPORTS_CONNECTION[resource] !== false,
};

export function useApitoResourceList(
  resource: ApitoResourceName,
  options?: {
    enabled?: boolean;
    initialFilters?: CrudFilter[];
    initialPagination?: { current: number; pageSize: number };
    permanentFilters?: CrudFilter[];
  },
) {
  const hook = APITO_LIST_HOOKS[resource];
  return hook(options);
}

export function useApitoResourceForm(resource: ApitoResourceName, id?: string) {
  const hook = APITO_FORM_HOOKS[resource];
  return hook(id);
}

export function useApitoResourceDelete(resource: ApitoResourceName) {
  const hook = APITO_DELETE_HOOKS[resource];
  return hook();
}
`,
		strings.Join(hookImports, "\n"),
		strings.Join(docImports, "\n"),
		strings.Join(resNames, " | "),
		strings.Join(listHooks, "\n"),
		strings.Join(formHooks, "\n"),
		strings.Join(deleteHooks, "\n"),
		strings.Join(createDocs, "\n"),
		strings.Join(updateDocs, "\n"),
		strings.Join(deleteDocs, "\n"),
		strings.Join(listKeys, "\n"),
		strings.Join(countKeys, "\n"),
		strings.Join(oneKeys, "\n"),
		strings.Join(listDocs, "\n"),
		strings.Join(oneDocs, "\n"),
		strings.Join(supportsConns, "\n"),
	)
	return os.WriteFile(filepath.Join(outDir, "resource-registry.ts"), []byte(body), 0o644)
}

func normalizeIntrospectionJSON(rawBytes []byte) ([]byte, error) {
	var intro map[string]interface{}
	if err := json.Unmarshal(rawBytes, &intro); err != nil {
		return nil, err
	}
	data, _ := intro["data"].(map[string]interface{})
	schema, _ := data["__schema"].(map[string]interface{})
	if schema == nil {
		schema, _ = intro["__schema"].(map[string]interface{})
	}
	if schema == nil {
		return rawBytes, nil
	}
	types, ok := schema["types"].([]interface{})
	if !ok {
		return rawBytes, nil
	}
	placeholderField := map[string]interface{}{
		"name": "_codegenPlaceholder",
		"args": []interface{}{},
		"type": map[string]interface{}{"kind": "SCALAR", "name": "String", "ofType": nil},
	}
	for i, rawT := range types {
		t, ok := rawT.(map[string]interface{})
		if !ok {
			continue
		}
		if t["interfaces"] == nil {
			t["interfaces"] = []interface{}{}
		}
		if t["possibleTypes"] == nil {
			t["possibleTypes"] = []interface{}{}
		}
		if _, has := t["enumValues"]; !has {
			t["enumValues"] = nil
		}
		if _, has := t["inputFields"]; !has {
			t["inputFields"] = nil
		}
		if t["kind"] == "OBJECT" {
			fields, _ := t["fields"].([]interface{})
			if len(fields) == 0 {
				fields = []interface{}{placeholderField}
			}
			for j, rawF := range fields {
				if fm, ok := rawF.(map[string]interface{}); ok {
					if fm["args"] == nil {
						fm["args"] = []interface{}{}
					}
					fields[j] = fm
				}
			}
			t["fields"] = fields
		}
		types[i] = t
	}
	schema["types"] = types
	if data == nil {
		intro["data"] = map[string]interface{}{"__schema": schema}
	}
	return json.MarshalIndent(intro, "", "  ")
}

func writeGraphQLCodegenConfig(outDir, schemaDir, opsDir string, skipDocumentsValidation bool, artifacts string) (string, error) {
	cfgPath := filepath.Join(outDir, ".codegen.apito.ts")
	relSchema, err := filepath.Rel(outDir, filepath.Join(schemaDir, "introspection.normalized.json"))
	if err != nil {
		return "", err
	}
	relOps, err := filepath.Rel(outDir, filepath.Join(opsDir, "**/*.graphql"))
	if err != nil {
		return "", err
	}
	hooksSection := ""
	if artifacts != "sdk" {
		hooksSection = `
    'hooks.ts': {
      plugins: ['typescript', 'typescript-operations', 'typescript-react-query'],
      config: {
        fetcher: '@apito-io/js-admin-sdk/react#useApitoGraphQLFetcher',
        reactQueryVersion: 5,
        skipTypename: true,
        avoidOptionals: true,
        exposeFetcher: true,
        exposeQueryKeys: true,
        exposeMutationKeys: true,
      },
    },`
	}
	body := fmt.Sprintf(`const config = {
  schema: '%s',
  documents: '%s',
  ignoreNoDocuments: true,
  config: {
    skipDocumentsValidation: %t,
  },
  generates: {
    'types.ts': {
      plugins: ['typescript', 'typescript-operations'],
      config: { skipTypename: true, avoidOptionals: true },
    },
    'sdk.ts': {
      plugins: ['typescript', 'typescript-operations', 'typescript-generic-sdk'],
      config: { documentMode: 'string', skipTypename: true, avoidOptionals: true },
    },%s
  },
};

export default config;
`, filepath.ToSlash(relSchema), filepath.ToSlash(relOps), skipDocumentsValidation, hooksSection)
	if err := os.WriteFile(cfgPath, []byte(body), 0o644); err != nil {
		return "", err
	}
	return cfgPath, nil
}

func ensureTypedDocumentStringHelper(outDir string) error {
	sdkPath := filepath.Join(outDir, "sdk.ts")
	raw, err := os.ReadFile(sdkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	src := string(raw)
	if !strings.Contains(src, "new TypedDocumentString") || strings.Contains(src, "class TypedDocumentString") {
		return nil
	}
	helper := `/** Runtime wrapper emitted for String-backed GraphQL documents. */
class TypedDocumentString extends String {
  private readonly value: string;

  constructor(value: string) {
    super(value);
    this.value = value;
  }

  override toString(): string {
    return this.value;
  }
}

`
	return os.WriteFile(sdkPath, []byte(helper+src), 0o644)
}

func fixReactQueryTypeImports(outDir string) error {
	hooksPath := filepath.Join(outDir, "hooks.ts")
	raw, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	src := string(raw)
	oldImport := "import { useQuery, useMutation, UseQueryOptions, UseMutationOptions } from '@tanstack/react-query';"
	newImport := "import { useQuery, useMutation } from '@tanstack/react-query';\nimport type { UseQueryOptions, UseMutationOptions } from '@tanstack/react-query';"
	fixed := strings.Replace(src, oldImport, newImport, 1)
	if fixed != src {
		return os.WriteFile(hooksPath, []byte(fixed), 0o644)
	}
	return nil
}

const graphqlCodegenRuntimeVersion = "v1"

func ensureGraphQLCodegenRuntime(workDir string) (binPath string, modulesPath string, err error) {
	// 1. Check if the workspace or monorepo already has @graphql-codegen/cli installed
	candidates := []string{
		filepath.Join(workDir, "node_modules"),
		filepath.Join(workDir, "..", "node_modules"),
		filepath.Join(workDir, "..", "..", "node_modules"),
		filepath.Join(workDir, "..", "..", "..", "apito", "sdk", "js-admin-sdk", "node_modules"),
	}
	for _, nm := range candidates {
		bin := filepath.Join(nm, "@graphql-codegen", "cli", "cjs", "bin.js")
		if _, statErr := os.Stat(bin); statErr == nil {
			absBin, _ := filepath.Abs(bin)
			absNM, _ := filepath.Abs(nm)
			return absBin, absNM, nil
		}
	}

	// 2. Fallback: self-contained CLI cache under ~/.apito/tools/graphql-codegen-<version>
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	cacheDir := filepath.Join(home, ".apito", "tools", "graphql-codegen-"+graphqlCodegenRuntimeVersion)
	nm := filepath.Join(cacheDir, "node_modules")
	bin := filepath.Join(nm, "@graphql-codegen", "cli", "cjs", "bin.js")
	if _, statErr := os.Stat(bin); statErr == nil {
		return bin, nm, nil
	}

	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	pkgJSON := `{
  "name": "apito-cli-graphql-codegen-runtime",
  "private": true,
  "dependencies": {
    "graphql": "16.11.0",
    "@graphql-codegen/cli": "5.0.7",
    "@graphql-codegen/typescript": "4.1.6",
    "@graphql-codegen/typescript-generic-sdk": "4.0.3",
    "@graphql-codegen/typescript-operations": "4.6.1",
    "@graphql-codegen/typescript-react-query": "6.1.1"
  }
}
`
	if err := os.WriteFile(filepath.Join(cacheDir, "package.json"), []byte(pkgJSON), 0o644); err != nil {
		return "", "", err
	}
	npmBin, err := exec.LookPath("npm")
	if err != nil {
		return "", "", fmt.Errorf("npm is required to bootstrap @graphql-codegen plugins for apito types: %w", err)
	}
	installCmd := exec.Command(npmBin, "install", "--no-audit", "--no-fund")
	installCmd.Dir = cacheDir
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to bootstrap @graphql-codegen runtime: %w", err)
	}
	return bin, nm, nil
}

func runNativeTSCodegen(workDir, schemaPath, outDir, modelsCSV, artifacts string, includeRelations bool, cliUIPackage string) error {
	if strings.TrimSpace(workDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		workDir = cwd
	}
	typeYAML, _, err := loadApitoTypeYAML(workDir)
	if err != nil {
		return err
	}
	rawBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	var requestedModels []string
	for _, m := range strings.Split(modelsCSV, ",") {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			requestedModels = append(requestedModels, trimmed)
		}
	}
	config, err := planTSConfigFromSchema(rawBytes, schemaPath, requestedModels, includeRelations, cliUIPackage, typeYAML)
	if err != nil {
		return err
	}
	if len(config.Models) == 0 {
		return fmt.Errorf("published schema did not contain any models")
	}

	workTmp := filepath.Join(outDir, ".codegen-work")
	opsDir := filepath.Join(workTmp, "operations")
	schemaDir := filepath.Join(workTmp, "schema")
	if err := os.MkdirAll(opsDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(workTmp)

	entries, _ := os.ReadDir(opsDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".graphql") {
			_ = os.Remove(filepath.Join(opsDir, e.Name()))
		}
	}

	normBytes, err := normalizeIntrospectionJSON(rawBytes)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "introspection.normalized.json"), normBytes, 0o644); err != nil {
		return err
	}

	for _, model := range config.Models {
		doc := generateModelGraphqlFile(model)
		if err := os.WriteFile(filepath.Join(opsDir, model.Name+".graphql"), []byte(doc), 0o644); err != nil {
			return err
		}
	}

	codegenCfg, err := writeGraphQLCodegenConfig(outDir, schemaDir, opsDir, true, artifacts)
	if err != nil {
		return err
	}
	defer os.Remove(codegenCfg)

	binPath, modulesPath, err := ensureGraphQLCodegenRuntime(workDir)
	if err != nil {
		return err
	}
	nodeBin, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node is required to run GraphQL type/hook codegen: %w", err)
	}
	cmd := exec.Command(nodeBin, binPath, "--config", codegenCfg)
	cmd.Dir = outDir
	env := os.Environ()
	existingNodePath := os.Getenv("NODE_PATH")
	if existingNodePath != "" {
		env = append(env, "NODE_PATH="+modulesPath+string(os.PathListSeparator)+existingNodePath)
	} else {
		env = append(env, "NODE_PATH="+modulesPath)
	}
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("graphql-codegen failed: %w", err)
	}

	if err := ensureTypedDocumentStringHelper(outDir); err != nil {
		return err
	}
	if err := writeTSResourceDocuments(config, outDir); err != nil {
		return err
	}
	if artifacts != "sdk" {
		if err := fixReactQueryTypeImports(outDir); err != nil {
			return err
		}
		if err := writeTSProjectFacades(config, outDir); err != nil {
			return err
		}
		if err := writeTSResourceRegistry(config, outDir); err != nil {
			return err
		}
	}
	return nil
}

