package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	blueprintSchemaFile = "schema.json"
	blueprintConfigFile = "config.yml"
)

// blueprintForbiddenTopLevel keys that must never appear in a portable Blueprint
// schema.json (engine project dumps often embed live secrets / org bindings).
var blueprintForbiddenTopLevel = []string{
	"tokens",
	"api_keys",
	"apiKeys",
	"project_secret_key",
	"projectSecretKey",
	"cloud_sync_key",
	"cloudSyncKey",
	"access_token",
	"accessToken",
	"organization_id",
	"organizationId",
	"database",
	"db",
	"credentials",
	"password",
	"secret",
	"secrets",
}

// blueprintConfig describes the optional portable config.yml beside schema.json.
// Only profile / display metadata — never tokens or credentials.
type blueprintConfig struct {
	Name                       string `yaml:"name" json:"name"`
	Slug                       string `yaml:"slug" json:"slug"`
	Description                string `yaml:"description" json:"description"`
	ProjectType                string `yaml:"project_type" json:"project_type"`
	TenantModel                string `yaml:"tenant_model" json:"tenant_model"`
	TenantModelName            string `yaml:"tenant_model_name" json:"tenant_model_name"`
	PerTenantSeparateDatabase  bool   `yaml:"per_tenant_separate_database" json:"per_tenant_separate_database"`
	PerTenantSeparateDatabaseY *bool  `yaml:"per_tenant_separate_db" json:"per_tenant_separate_db"`
}

func (c blueprintConfig) syncProfileKey() string {
	pt := strings.ToLower(strings.TrimSpace(c.ProjectType))
	if pt == "" || pt == "general" {
		return "general"
	}
	perTenant := c.PerTenantSeparateDatabase
	if c.PerTenantSeparateDatabaseY != nil {
		perTenant = *c.PerTenantSeparateDatabaseY
	}
	if perTenant {
		return "saas-per-tenant"
	}
	return "saas-shared"
}

func (c blueprintConfig) asSyncProject() SyncProject {
	tenant := strings.TrimSpace(c.TenantModelName)
	if tenant == "" {
		tenant = strings.TrimSpace(c.TenantModel)
	}
	perTenant := c.PerTenantSeparateDatabase
	if c.PerTenantSeparateDatabaseY != nil {
		perTenant = *c.PerTenantSeparateDatabaseY
	}
	pt := strings.ToLower(strings.TrimSpace(c.ProjectType))
	if pt == "" {
		pt = "general"
	}
	return SyncProject{
		ID:                        c.Slug,
		Name:                      c.Name,
		ProjectType:               pt,
		TenantModelName:           tenant,
		PerTenantSeparateDatabase: perTenant,
	}
}

func defaultSchemaSyncDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".apito", "temp", "schema"), nil
}

func resolveSchemaSyncDir() (string, error) {
	if d := strings.TrimSpace(syncDir); d != "" {
		return d, nil
	}
	dir, err := defaultSchemaSyncDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create default schema dir %s: %w", dir, err)
	}
	print_status(fmt.Sprintf("Using schema directory: %s (override with --dir)", dir))
	return dir, nil
}

// resolveBlueprintSchemaPath returns the schema.json path for a Blueprint dir
// or a direct path to a schema.json file.
func resolveBlueprintSchemaPath(dirOrFile string) (string, error) {
	info, err := os.Stat(dirOrFile)
	if err != nil {
		return "", fmt.Errorf("schema path %q: %w", dirOrFile, err)
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Base(dirOrFile), blueprintSchemaFile) || strings.HasSuffix(strings.ToLower(dirOrFile), ".json") {
			return dirOrFile, nil
		}
		return "", fmt.Errorf("schema path %q: expected a directory or schema.json file", dirOrFile)
	}
	path := filepath.Join(dirOrFile, blueprintSchemaFile)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("blueprint dir %q: missing %s", dirOrFile, blueprintSchemaFile)
	}
	return path, nil
}

func readBlueprintConfig(dir string) (*blueprintConfig, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	cfgDir := dir
	if !info.IsDir() {
		cfgDir = filepath.Dir(dir)
	}
	path := filepath.Join(cfgDir, blueprintConfigFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %q: %w", path, err)
	}
	if err := rejectBlueprintSecretsYAML(raw, path); err != nil {
		return nil, err
	}
	var cfg blueprintConfig
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	return &cfg, nil
}

func rejectBlueprintSecretsYAML(raw []byte, path string) error {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	return rejectForbiddenKeys(doc, path, "")
}

func rejectBlueprintSecretsJSON(raw []byte, path string) error {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse %q: %w", path, err)
	}
	return rejectForbiddenKeys(doc, path, "")
}

func rejectForbiddenKeys(doc map[string]any, path, prefix string) error {
	for k, v := range doc {
		key := strings.ToLower(strings.TrimSpace(k))
		full := k
		if prefix != "" {
			full = prefix + "." + k
		}
		for _, forbidden := range blueprintForbiddenTopLevel {
			if key == strings.ToLower(forbidden) {
				return fmt.Errorf(
					"%s: portable Blueprints must not include %q (secret/credential field) — strip it and retry",
					path, full,
				)
			}
		}
		// Nested objects under settings/plugins often carry credentials.
		if m, ok := v.(map[string]any); ok {
			if key == "settings" || key == "plugins" || key == "auth" || key == "storage" {
				if err := rejectForbiddenKeys(m, path, full); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// parseSchemaBlueprintJSON accepts engine preview shape `{models:[...]}` or
// project export `{schema:{models:[...]}}` (and optionally top-level models).
func parseSchemaBlueprintJSON(raw []byte) ([]SyncModel, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 || string(raw) == "{}" || string(raw) == "null" {
		return nil, fmt.Errorf("schema JSON is empty")
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("schema JSON: %w", err)
	}

	if modelsRaw, ok := top["models"]; ok {
		var models []SyncModel
		if err := json.Unmarshal(modelsRaw, &models); err != nil {
			return nil, fmt.Errorf("schema.models: %w", err)
		}
		return models, nil
	}

	if schemaRaw, ok := top["schema"]; ok {
		var nested struct {
			Models []SyncModel `json:"models"`
		}
		if err := json.Unmarshal(schemaRaw, &nested); err != nil {
			return nil, fmt.Errorf("schema.schema: %w", err)
		}
		if len(nested.Models) == 0 {
			return nil, fmt.Errorf("schema.schema.models is empty")
		}
		return nested.Models, nil
	}

	return nil, fmt.Errorf("schema JSON must contain \"models\" or \"schema.models\"")
}

// sanitizeBlueprintModels strips engine-owned / non-portable fields before sync.
func sanitizeBlueprintModels(models []SyncModel) []SyncModel {
	out := make([]SyncModel, 0, len(models))
	for _, m := range models {
		name := strings.TrimSpace(m.Name)
		if name == "" {
			continue
		}
		m.Name = name
		m.Fields = sanitizeBlueprintFields(m.Fields)
		m.Connections = sanitizeBlueprintConnections(m.Connections)
		out = append(out, m)
	}
	return out
}

func sanitizeBlueprintFields(fields []SyncField) []SyncField {
	return sanitizeBlueprintFieldsWithParent(fields, "")
}

func sanitizeBlueprintFieldsWithParent(fields []SyncField, parentID string) []SyncField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]SyncField, 0, len(fields))
	for _, f := range fields {
		id := strings.TrimSpace(f.Identifier)
		if id == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(id), "system_") {
			continue
		}
		// Repeated-row engine UUID leaf.
		if id == "_id" {
			continue
		}
		f.Identifier = id
		// Portable Blueprints should not carry renamed/stale parent_field values.
		if parentID == "" {
			f.ParentField = ""
		} else {
			f.ParentField = parentID
		}
		if len(f.SubFieldInfo) > 0 {
			f.SubFieldInfo = sanitizeBlueprintFieldsWithParent(f.SubFieldInfo, id)
		}
		out = append(out, f)
	}
	return out
}

func sanitizeBlueprintConnections(conns []SyncConnection) []SyncConnection {
	if len(conns) == 0 {
		return nil
	}
	out := make([]SyncConnection, 0, len(conns))
	for _, c := range conns {
		if strings.TrimSpace(c.Model) == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// readLocalSchema loads and sanitizes models from a Blueprint directory or schema.json.
func readLocalSchema(dirOrFile string) ([]SyncModel, *blueprintConfig, error) {
	schemaPath, err := resolveBlueprintSchemaPath(dirOrFile)
	if err != nil {
		return nil, nil, err
	}
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read %q: %w", schemaPath, err)
	}
	if err := rejectBlueprintSecretsJSON(raw, schemaPath); err != nil {
		return nil, nil, err
	}
	models, err := parseSchemaBlueprintJSON(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", schemaPath, err)
	}
	models = sanitizeBlueprintModels(models)
	if len(models) == 0 {
		return nil, nil, fmt.Errorf("%s: no models after sanitization", schemaPath)
	}

	cfg, err := readBlueprintConfig(dirOrFile)
	if err != nil {
		return nil, nil, err
	}
	return models, cfg, nil
}

// writeLocalSchema exports models as a portable Blueprint schema.json (+ optional config stub).
func writeLocalSchema(dir string, models []SyncModel, cfg *blueprintConfig) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create %q: %w", dir, err)
	}
	payload := map[string]any{
		"models": sanitizeBlueprintModels(models),
	}
	if cfg != nil {
		if strings.TrimSpace(cfg.Slug) != "" {
			payload["id"] = cfg.Slug
		}
		if strings.TrimSpace(cfg.Name) != "" {
			payload["name"] = cfg.Name
		}
		if strings.TrimSpace(cfg.Description) != "" {
			payload["description"] = cfg.Description
		}
		// Prefer nested shape for round-trip with sample exports.
		payload = map[string]any{
			"id":          payload["id"],
			"name":        payload["name"],
			"description": payload["description"],
			"schema": map[string]any{
				"models": sanitizeBlueprintModels(models),
			},
		}
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, blueprintSchemaFile)
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

// runLocalSchemaSync handles sync where one side is the reserved "filesystem" account.
func runLocalSchemaSync(fromName, toName string, timeout int, dryRun, autoYes, includeDeletes bool, preselectedRemote *SyncProject) error {
	if isFilesystemSyncSide(fromName) && isFilesystemSyncSide(toName) {
		return fmt.Errorf("both sides are %q — at least one side must be a remote account", filesystemAccountName)
	}
	dir, err := resolveSchemaSyncDir()
	if err != nil {
		return err
	}

	remoteSide := "destination"
	remoteName := toName
	if isFilesystemSyncSide(toName) {
		remoteSide = "source"
		remoteName = fromName
	}

	remoteCfg, err := getAccountConfig(remoteName)
	if err != nil {
		return err
	}
	base := newSyncGraphQLClient(remoteCfg.ServerURL, remoteCfg.CloudSyncKey, timeout)
	var project SyncProject
	if preselectedRemote != nil && strings.TrimSpace(preselectedRemote.ID) != "" {
		project = *preselectedRemote
		print_check(fmt.Sprintf("%s project: %s [%s] (%s)", remoteSide, project.Name, projectTypeLabel(project), project.ID))
	} else {
		project, err = selectSyncProject(base, remoteSide)
		if err != nil {
			return err
		}
	}
	remoteClient := base.WithProject(project.ID)

	if isFilesystemSyncSide(fromName) {
		print_status(fmt.Sprintf("Importing schema: filesystem (%s) -> %s / %s (%s)", dir, remoteName, project.Name, project.ID))
		models, bpCfg, err := readLocalSchema(dir)
		if err != nil {
			return err
		}
		if bpCfg != nil {
			srcProfile := bpCfg.syncProfileKey()
			dstProfile := projectSyncProfileKey(project)
			if srcProfile != dstProfile {
				return fmt.Errorf(
					"project type mismatch: blueprint is %q but destination is %q (must match: general, saas-shared, saas-per-tenant)",
					srcProfile, dstProfile,
				)
			}
			print_status(fmt.Sprintf("Blueprint profile: %s (%s)", bpCfg.Name, srcProfile))
		} else {
			print_status("No config.yml beside schema.json — skipping blueprint profile check (destination profile still applies).")
		}

		endpoints := SyncEndpoints{
			FromAccount: filesystemAccountName,
			ToAccount:   remoteName,
			FromURL:     fmt.Sprintf("filesystem:%s", dir),
			ToURL:       remoteCfg.ServerURL,
			FromProject: SyncProject{ID: "filesystem", Name: "filesystem"},
			ToProject:   project,
		}
		if bpCfg != nil {
			endpoints.FromProject = bpCfg.asSyncProject()
		}
		return runSchemaSyncFromModels(models, remoteClient, endpoints, dryRun, autoYes, includeDeletes)
	}

	// project → filesystem (export)
	print_status(fmt.Sprintf("Exporting schema: %s / %s (%s) -> filesystem (%s)", remoteName, project.Name, project.ID, dir))
	models, err := remoteClient.ProjectModelsInfo("")
	if err != nil {
		return fmt.Errorf("source schema: %w", err)
	}
	if issues := validateSyncModels(models); len(issues) > 0 {
		print_error("Source schema validation failed.")
		fmt.Println(formatSchemaValidationReport(issues))
		return fmt.Errorf("source schema has %d validation issue(s); fix schema before export", len(issues))
	}
	cfg := &blueprintConfig{
		Name:                      project.Name,
		Slug:                      project.ID,
		ProjectType:               project.ProjectType,
		TenantModelName:           project.TenantModelName,
		PerTenantSeparateDatabase: project.PerTenantSeparateDatabase,
	}
	if dryRun {
		print_success(fmt.Sprintf("Dry run: would export %d model(s) to %s/%s", len(models), dir, blueprintSchemaFile))
		return nil
	}
	if err := writeLocalSchema(dir, models, cfg); err != nil {
		return err
	}
	print_success(fmt.Sprintf("Exported %d model(s) to %s/%s", len(models), dir, blueprintSchemaFile))
	return nil
}
