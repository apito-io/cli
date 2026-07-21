package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	headerApitoProjectID = "X-Apito-Project-Id"
	headerApitoTenantID  = "X-Apito-Tenant-ID"
	headerUseCookies     = "X-Use-Cookies"
)

type SyncGraphQLClient struct {
	endpoint  string
	token     string
	projectID string
	tenantID  string
	timeout   time.Duration
}

type SyncProject struct {
	ID                        string `json:"id"`
	Name                      string `json:"name"`
	Description               string `json:"description"`
	ProjectType               string `json:"project_type"`
	TenantModelName           string `json:"tenant_model_name"`
	PerTenantSeparateDatabase bool   `json:"per_tenant_separate_database"`
}

type SyncFieldValidation struct {
	Required             *bool  `json:"required"`
	Unique               *bool  `json:"unique"`
	Hide                 *bool  `json:"hide"`
	FixedListElements    []any  `json:"fixed_list_elements"`
	FixedListElementType string `json:"fixed_list_element_type"`
}

type SyncField struct {
	Identifier   string               `json:"identifier"`
	Label        string               `json:"label"`
	FieldType    string               `json:"field_type"`
	FieldSubType string               `json:"field_sub_type"`
	InputType    string               `json:"input_type"`
	Description  string               `json:"description"`
	Serial       int                  `json:"serial"`
	ParentField  string               `json:"parent_field"`
	SubFieldInfo []SyncField          `json:"sub_field_info"`
	Validation   *SyncFieldValidation `json:"validation"`
}

type SyncConnection struct {
	Model    string `json:"model"`
	Relation string `json:"relation"`
	Type     string `json:"type"`
	KnownAs  string `json:"known_as"`
}

type SyncModel struct {
	Name        string           `json:"name"`
	SinglePage  bool             `json:"single_page"`
	Fields      []SyncField      `json:"fields"`
	Connections []SyncConnection `json:"connections"`
}

type SyncDocument struct {
	ID   string                 `json:"id"`
	Key  string                 `json:"_key"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

type SyncModelDataResult struct {
	Count   int            `json:"count"`
	Results []SyncDocument `json:"results"`
}

type SyncTenant struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

// SyncFunctionRuntimeConfig mirrors ApitoFunctionRuntimeConfig (handler/memory/runtime/time_out).
type SyncFunctionRuntimeConfig struct {
	Handler string `json:"handler,omitempty"`
	Memory  int    `json:"memory,omitempty"`
	Runtime string `json:"runtime,omitempty"`
	TimeOut int    `json:"time_out,omitempty"`
}

// SyncFunctionModelRef is the request/response model reference on a function.
type SyncFunctionModelRef struct {
	Model           string `json:"model,omitempty"`
	IsArray         bool   `json:"is_array,omitempty"`
	OptionalPayload bool   `json:"optional_payload,omitempty"`
}

type SyncFunctionEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SyncFunction is the transferable definition of a Logic function (draft source + metadata).
type SyncFunction struct {
	Name                string                     `json:"name"`
	Description         string                     `json:"description,omitempty"`
	Source              string                     `json:"source,omitempty"`
	TriggerType         string                     `json:"trigger_type,omitempty"`
	Language            string                     `json:"language,omitempty"`
	GraphQLSchemaType   string                     `json:"graphql_schema_type,omitempty"`
	Capabilities        []string                   `json:"capabilities,omitempty"`
	RuntimeConfig       *SyncFunctionRuntimeConfig `json:"runtime_config,omitempty"`
	Request             *SyncFunctionModelRef      `json:"request,omitempty"`
	Response            *SyncFunctionModelRef      `json:"response,omitempty"`
	EnvVars             []SyncFunctionEnvVar       `json:"env_vars,omitempty"`
	FunctionConnected   bool                       `json:"function_connected,omitempty"`
	FunctionProviderID  string                     `json:"function_provider_id,omitempty"`
	ActiveRevisionID    string                     `json:"active_revision_id,omitempty"`
	ActiveRevisionHash  string                     `json:"active_revision_hash,omitempty"`
	RestAPISecretURLKey string                     `json:"rest_api_secret_url_key,omitempty"`
	CreatedAt           string                     `json:"created_at,omitempty"`
	UpdatedAt           string                     `json:"updated_at,omitempty"`
}

// SyncFunctionRevision is a single immutable revision entry.
type SyncFunctionRevision struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Revision     int    `json:"revision"`
	ArtifactKey  string `json:"artifact_key"`
	ArtifactHash string `json:"artifact_hash"`
	CreatedAt    string `json:"created_at"`
}

type graphqlRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphqlResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

func newSyncGraphQLClient(serverURL, token string, timeoutSec int) *SyncGraphQLClient {
	base := strings.TrimSuffix(strings.TrimSpace(serverURL), "/")
	endpoint := base + "/system/graphql"
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &SyncGraphQLClient{
		endpoint: endpoint,
		token:    strings.TrimSpace(token),
		timeout:  time.Duration(timeoutSec) * time.Second,
	}
}

func (c *SyncGraphQLClient) baseURL() string {
	return strings.TrimSuffix(c.endpoint, "/system/graphql")
}

func (c *SyncGraphQLClient) graphqlEndpoint() string {
	return c.endpoint
}

func (c *SyncGraphQLClient) WithProject(projectID string) *SyncGraphQLClient {
	clone := *c
	clone.projectID = strings.TrimSpace(projectID)
	return &clone
}

func (c *SyncGraphQLClient) WithTenant(tenantID string) *SyncGraphQLClient {
	clone := *c
	clone.tenantID = strings.TrimSpace(tenantID)
	return &clone
}

func (c *SyncGraphQLClient) execute(query string, variables map[string]interface{}, dest interface{}) error {
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(req)

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("graphql HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var gqlResp graphqlResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return err
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, 0, len(gqlResp.Errors))
		for _, e := range gqlResp.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("graphql errors: %s", strings.Join(msgs, "; "))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(gqlResp.Data, dest)
}

// setAuthHeaders sends the unified apt_ access token as a standard bearer
// credential. X-Use-Cookies: false tells the engine this is a headless API
// call (no browser session cookies), and X-Apito-Project-Id scopes the
// request to the selected project within the token's project_ids grant.
func (c *SyncGraphQLClient) setAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set(headerUseCookies, "false")
	if c.projectID != "" {
		req.Header.Set(headerApitoProjectID, c.projectID)
	}
	if c.tenantID != "" {
		req.Header.Set(headerApitoTenantID, c.tenantID)
	}
}

func (c *SyncGraphQLClient) ListProjects() ([]SyncProject, error) {
	projects, err := c.listProjectsQuery(true)
	if err != nil {
		// OSS engines may not expose pro-only project fields.
		if fallback, fbErr := c.listProjectsQuery(false); fbErr == nil {
			return fallback, nil
		}
		return nil, err
	}
	return projects, nil
}

func (c *SyncGraphQLClient) listProjectsQuery(extended bool) ([]SyncProject, error) {
	var query string
	if extended {
		query = `
		query ListProjects {
			listProjects {
				id
				name
				description
				project_type
				tenant_model_name
				per_tenant_separate_database
			}
		}
	`
	} else {
		query = `
		query ListProjects {
			listProjects {
				id
				name
				description
				project_type
			}
		}
	`
	}
	var out struct {
		ListProjects []SyncProject `json:"listProjects"`
	}
	if err := c.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return out.ListProjects, nil
}

func (c *SyncGraphQLClient) CurrentProject() (*SyncProject, error) {
	const query = `
		query CurrentProject {
			currentProject {
				id
				name
				description
				project_type
				tenant_model_name
				per_tenant_separate_database
			}
		}
	`
	var out struct {
		CurrentProject SyncProject `json:"currentProject"`
	}
	if err := c.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return &out.CurrentProject, nil
}

func (c *SyncGraphQLClient) ProjectModelsInfo(modelName string) ([]SyncModel, error) {
	const query = `
		query ProjectModelsInfo($model_name: String) {
			projectModelsInfo(model_name: $model_name) {
				name
				single_page
				fields {
					identifier
					label
					field_type
					field_sub_type
					input_type
					description
					serial
					parent_field
					sub_field_info {
						identifier
						label
						field_type
						field_sub_type
						input_type
						serial
						parent_field
						validation {
							required
							unique
							hide
							fixed_list_elements
							fixed_list_element_type
						}
					}
					validation {
						required
						unique
						hide
						fixed_list_elements
						fixed_list_element_type
					}
				}
				connections {
					model
					relation
					type
					known_as
				}
			}
		}
	`
	vars := map[string]interface{}{}
	if modelName != "" {
		vars["model_name"] = modelName
	}
	var out struct {
		ProjectModelsInfo []SyncModel `json:"projectModelsInfo"`
	}
	if err := c.execute(query, vars, &out); err != nil {
		return nil, err
	}
	return out.ProjectModelsInfo, nil
}

func (c *SyncGraphQLClient) AddModel(name string, singleRecord bool) error {
	const mutation = `
		mutation AddModel($name: String!, $single_record: Boolean) {
			addModelToProject(name: $name, single_record: $single_record) {
				name
			}
		}
	`
	var out struct {
		AddModelToProject []SyncModel `json:"addModelToProject"`
	}
	return c.execute(mutation, map[string]interface{}{
		"name":          name,
		"single_record": singleRecord,
	}, &out)
}

func (c *SyncGraphQLClient) UpsertField(modelName string, field SyncField, isUpdate bool) error {
	const mutation = `
		mutation UpsertField(
			$model_name: String!
			$field_label: String!
			$field_type: FIELD_TYPE_ENUM
			$field_sub_type: FIELD_SUB_TYPE_ENUM
			$input_type: INPUT_TYPE_ENUM
			$parent_field: String
			$is_object_field: Boolean
			$is_update: Boolean
			$serial: Int
			$field_description: String
			$validation: module_validation_payload
		) {
			upsertFieldToModel(
				model_name: $model_name
				field_label: $field_label
				field_type: $field_type
				field_sub_type: $field_sub_type
				input_type: $input_type
				parent_field: $parent_field
				is_object_field: $is_object_field
				is_update: $is_update
				serial: $serial
				field_description: $field_description
				validation: $validation
			) {
				identifier
			}
		}
	`
	vars := map[string]interface{}{
		"model_name":  modelName,
		"field_label": field.Label,
		"field_type":  field.FieldType,
		"input_type":  field.InputType,
		"is_update":   isUpdate,
	}
	if field.FieldSubType != "" {
		vars["field_sub_type"] = field.FieldSubType
	}
	if field.ParentField != "" {
		vars["parent_field"] = field.ParentField
	}
	if field.FieldType == "object" || field.FieldType == "repeated" {
		vars["is_object_field"] = true
	}
	if field.Serial > 0 {
		vars["serial"] = field.Serial
	}
	if field.Description != "" {
		vars["field_description"] = field.Description
	}
	if field.Validation != nil {
		vars["validation"] = field.Validation
	}
	var out struct {
		UpsertFieldToModel SyncField `json:"upsertFieldToModel"`
	}
	return c.execute(mutation, vars, &out)
}

func (c *SyncGraphQLClient) UpsertConnection(fromModel, toModel, forwardType, reverseType, knownAs string) error {
	const mutation = `
		mutation UpsertConnection(
			$forward_connection_type: RELATION_TYPE_ENUM!
			$from: String!
			$reverse_connection_type: RELATION_TYPE_ENUM!
			$to: String!
			$known_as: String
		) {
			upsertConnectionToModel(
				forward_connection_type: $forward_connection_type
				from: $from
				reverse_connection_type: $reverse_connection_type
				to: $to
				known_as: $known_as
			) {
				known_as
			}
		}
	`
	vars := map[string]interface{}{
		"forward_connection_type": forwardType,
		"from":                    fromModel,
		"reverse_connection_type": reverseType,
		"to":                      toModel,
	}
	if knownAs != "" {
		vars["known_as"] = knownAs
	}
	var out struct {
		UpsertConnectionToModel []SyncConnection `json:"upsertConnectionToModel"`
	}
	return c.execute(mutation, vars, &out)
}

func (c *SyncGraphQLClient) GetModelData(modelName string, page, limit int) (*SyncModelDataResult, error) {
	const query = `
		query GetModelData($model: String!, $page: Int, $limit: Int, $status: FILTER_STATUS_TYPE_ENUM) {
			getModelData(model: $model, page: $page, limit: $limit, status: $status) {
				count
				results {
					id
					_key
					type
					data
				}
			}
		}
	`
	var out struct {
		GetModelData SyncModelDataResult `json:"getModelData"`
	}
	err := c.execute(query, map[string]interface{}{
		"model":  modelName,
		"page":   page,
		"limit":  limit,
		"status": "all",
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out.GetModelData, nil
}

func (c *SyncGraphQLClient) GetRelatedModelData(modelName, docID string, conn SyncConnection, page, limit int) (*SyncModelDataResult, error) {
	const query = `
		query GetRelatedModelData(
			$model: String!
			$page: Int
			$limit: Int
			$status: FILTER_STATUS_TYPE_ENUM
			$connection: ListAllDataDetailedOfAModelConnectionPayload
		) {
			getModelData(model: $model, page: $page, limit: $limit, status: $status, connection: $connection) {
				count
				results {
					id
					_key
					type
					data
				}
			}
		}
	`
	connection := map[string]interface{}{
		"model":           modelName,
		"_id":             docID,
		"to_model":        conn.Model,
		"relation_type":   conn.Relation,
		"known_as":        conn.KnownAs,
		"connection_type": "outbound",
	}
	var out struct {
		GetModelData SyncModelDataResult `json:"getModelData"`
	}
	err := c.execute(query, map[string]interface{}{
		"model":      modelName,
		"page":       page,
		"limit":      limit,
		"status":     "all",
		"connection": connection,
	}, &out)
	if err != nil {
		return nil, err
	}
	return &out.GetModelData, nil
}

func (c *SyncGraphQLClient) UpsertModelData(modelName, docID string, payload, connect map[string]interface{}) error {
	const mutation = `
		mutation UpsertModelData($model_name: String!, $_id: String, $payload: JSON, $status: String!, $connect: JSON) {
			upsertModelData(model_name: $model_name, _id: $_id, payload: $payload, status: $status, connect: $connect) {
				id
			}
		}
	`
	vars := map[string]interface{}{
		"model_name": modelName,
		"payload":    payload,
		"status":     "published",
	}
	if docID != "" {
		vars["_id"] = docID
	}
	if len(connect) > 0 {
		vars["connect"] = connect
	}
	var out struct {
		UpsertModelData SyncDocument `json:"upsertModelData"`
	}
	return c.execute(mutation, vars, &out)
}

func (c *SyncGraphQLClient) GetTenants() ([]SyncTenant, error) {
	const query = `
		query GetTenants {
			getTenants {
				tenants {
					id
					name
					domain
				}
			}
		}
	`
	var out struct {
		GetTenants struct {
			Tenants []SyncTenant `json:"tenants"`
		} `json:"getTenants"`
	}
	if err := c.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return out.GetTenants.Tenants, nil
}

func (c *SyncGraphQLClient) SchemaVersioningStatus() (*SchemaVersioningStatus, error) {
	const query = `
		query SchemaVersioningStatus {
			schemaVersioningStatus {
				enabled
				active_version
				has_draft
				changeset_id
				changeset_status
				pending_operations
			}
		}
	`
	var out struct {
		SchemaVersioningStatus *SchemaVersioningStatus `json:"schemaVersioningStatus"`
	}
	if err := c.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return out.SchemaVersioningStatus, nil
}

// ProjectFunctionsInfo lists project Logic functions with full transferable fields.
func (c *SyncGraphQLClient) ProjectFunctionsInfo() ([]SyncFunction, error) {
	const query = `
		query ProjectFunctionsInfo {
			projectFunctionsInfo {
				name
				description
				graphql_schema_type
				created_at
				updated_at
				trigger_type
				language
				source
				active_revision_id
				active_revision_hash
				capabilities
				function_connected
				function_provider_id
				request { model optional_payload is_array }
				response { model optional_payload is_array }
				runtime_config { handler memory runtime time_out }
				env_vars { key value }
				rest_api_secret_url_key
			}
		}
	`
	var out struct {
		ProjectFunctionsInfo []SyncFunction `json:"projectFunctionsInfo"`
	}
	if err := c.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return out.ProjectFunctionsInfo, nil
}

// UpsertFunction creates or updates a function's draft source + metadata on the destination.
func (c *SyncGraphQLClient) UpsertFunction(fn SyncFunction, isUpdate bool) (*SyncFunction, error) {
	const mutation = `
		mutation UpsertFunctionToProject(
			$name: String!
			$description: String
			$source: String
			$trigger_type: String
			$language: String
			$graphql_schema_type: String
			$capabilities: [String]
			$runtime_config: Function_Provider_Config_Payload
			$env_vars: [Function_Provider_Env_Vars_Payload]
			$request: String
			$request_payload_is_optional: Boolean
			$response: String
			$response_is_array: Boolean
			$function_connected: Boolean
			$function_provider_id: String
			$update: Boolean
		) {
			upsertFunctionToProject(
				name: $name
				description: $description
				source: $source
				trigger_type: $trigger_type
				language: $language
				graphql_schema_type: $graphql_schema_type
				capabilities: $capabilities
				runtime_config: $runtime_config
				env_vars: $env_vars
				request: $request
				request_payload_is_optional: $request_payload_is_optional
				response: $response
				response_is_array: $response_is_array
				function_connected: $function_connected
				function_provider_id: $function_provider_id
				update: $update
			) {
				name
				source
				active_revision_id
				capabilities
				rest_api_secret_url_key
			}
		}
	`
	vars := map[string]interface{}{
		"name":   fn.Name,
		"update": isUpdate,
	}
	if fn.Description != "" {
		vars["description"] = fn.Description
	}
	if fn.Source != "" {
		vars["source"] = fn.Source
	}
	if fn.TriggerType != "" {
		vars["trigger_type"] = fn.TriggerType
	}
	if fn.Language != "" {
		vars["language"] = fn.Language
	}
	if fn.GraphQLSchemaType != "" {
		vars["graphql_schema_type"] = fn.GraphQLSchemaType
	}
	if len(fn.Capabilities) > 0 {
		vars["capabilities"] = fn.Capabilities
	}
	if fn.RuntimeConfig != nil {
		vars["runtime_config"] = fn.RuntimeConfig
	}
	if len(fn.EnvVars) > 0 {
		vars["env_vars"] = fn.EnvVars
	}
	if fn.Request != nil && fn.Request.Model != "" {
		vars["request"] = fn.Request.Model
		vars["request_payload_is_optional"] = fn.Request.OptionalPayload
	}
	if fn.Response != nil && fn.Response.Model != "" {
		vars["response"] = fn.Response.Model
		vars["response_is_array"] = fn.Response.IsArray
	}
	if fn.FunctionConnected {
		vars["function_connected"] = fn.FunctionConnected
	}
	if fn.FunctionProviderID != "" {
		vars["function_provider_id"] = fn.FunctionProviderID
	}
	var out struct {
		UpsertFunctionToProject SyncFunction `json:"upsertFunctionToProject"`
	}
	if err := c.execute(mutation, vars, &out); err != nil {
		return nil, err
	}
	return &out.UpsertFunctionToProject, nil
}

// DeployFunction publishes the current draft (or given source) as a new active revision.
func (c *SyncGraphQLClient) DeployFunction(name, source string) (*SyncFunction, error) {
	const mutation = `
		mutation DeployFunctionToProject($name: String!, $source: String) {
			deployFunctionToProject(name: $name, source: $source) {
				function {
					name
					active_revision_id
					rest_api_secret_url_key
				}
			}
		}
	`
	vars := map[string]interface{}{"name": name}
	if source != "" {
		vars["source"] = source
	}
	var out struct {
		DeployFunctionToProject struct {
			Function SyncFunction `json:"function"`
		} `json:"deployFunctionToProject"`
	}
	if err := c.execute(mutation, vars, &out); err != nil {
		return nil, err
	}
	return &out.DeployFunctionToProject.Function, nil
}

// ListFunctionRevisions returns revision history for a function.
func (c *SyncGraphQLClient) ListFunctionRevisions(name string, limit int) ([]SyncFunctionRevision, error) {
	const query = `
		query ListFunctionRevisions($name: String!, $limit: Int) {
			listFunctionRevisions(name: $name, limit: $limit) {
				id
				name
				revision
				artifact_key
				artifact_hash
				created_at
			}
		}
	`
	vars := map[string]interface{}{"name": name}
	if limit > 0 {
		vars["limit"] = limit
	}
	var out struct {
		ListFunctionRevisions []SyncFunctionRevision `json:"listFunctionRevisions"`
	}
	if err := c.execute(query, vars, &out); err != nil {
		return nil, err
	}
	return out.ListFunctionRevisions, nil
}

func (c *SyncGraphQLClient) SchemaPreviewModels(source string) ([]SyncModel, error) {
	const query = `
		query SchemaPreview($source: String) {
			schemaPreview(source: $source)
		}
	`
	var out struct {
		SchemaPreview string `json:"schemaPreview"`
	}
	if err := c.execute(query, map[string]interface{}{
		"source": source,
	}, &out); err != nil {
		return nil, err
	}
	return parseModelsFromSchemaJSON(out.SchemaPreview)
}
