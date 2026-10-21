package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const publishedIntrospectionQuery = `query IntrospectionQuery {
  __schema {
    queryType { name fields { name } }
    mutationType { name }
    subscriptionType { name }
    types { ...FullType }
    directives {
      name
      description
      locations
      args { ...InputValue }
    }
  }
}
fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args { ...InputValue }
    type { ...TypeRef }
    isDeprecated
    deprecationReason
  }
  inputFields { ...InputValue }
  interfaces { ...TypeRef }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes { ...TypeRef }
}
fragment InputValue on __InputValue {
  name
  description
  type { ...TypeRef }
  defaultValue
}
fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType { kind name }
            }
          }
        }
      }
    }
  }
}`

func fetchIntrospection(ctx commandContext) ([]byte, error) {
	if strings.TrimSpace(ctx.ProjectID) == "" || ctx.Client == nil || strings.TrimSpace(ctx.Client.projectID) == "" {
		return nil, fmt.Errorf("project id is required to read the published schema. Pass --project or set it on the account")
	}
	if _, err := projectOnAccount(ctx); err != nil {
		return nil, err
	}
	body, err := json.Marshal(graphqlRequest{Query: publishedIntrospectionQuery})
	if err != nil {
		return nil, err
	}
	if bytes.Contains(body, []byte("generateTenantToken")) {
		return nil, fmt.Errorf("published schema introspection must not mint a tenant token")
	}
	raw, _, err := ctx.Client.do("POST", ctx.Client.baseURL()+"/secured/graphql", body)
	if err != nil {
		return nil, fmt.Errorf("published schema introspection failed: %w", err)
	}
	if bytes.Contains(raw, []byte("apt_")) || bytes.Contains(raw, []byte("ak_")) {
		return nil, fmt.Errorf("introspection response contained a credential and was discarded")
	}
	var parsed struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("published schema introspection returned invalid JSON")
	}
	if len(parsed.Errors) > 0 {
		msgs := make([]string, 0, len(parsed.Errors))
		for _, item := range parsed.Errors {
			msgs = append(msgs, item.Message)
		}
		return nil, fmt.Errorf("published schema introspection failed: %s", redactSecrets(strings.Join(msgs, "; ")))
	}
	if !bytes.Contains(parsed.Data, []byte("__schema")) {
		return nil, fmt.Errorf("published schema introspection returned no schema")
	}
	return raw, nil
}

// projectOnAccount returns the requested project only when ListProjects — the
// same query apito sync and apito project list use — includes its id.
// The list is read with no project or tenant header. Pinning X-Apito-Project-Id
// to an id this account does not have makes the engine reject the call before
// it can say the project is missing.
func projectOnAccount(ctx commandContext) (SyncProject, error) {
	wanted := strings.TrimSpace(ctx.ProjectID)
	if wanted == "" {
		return SyncProject{}, fmt.Errorf("project id is required to read the published schema. Pass --project or set it on the account")
	}
	projects, err := ctx.syncClient().WithProject("").WithTenant("").ListProjects()
	if err != nil {
		return SyncProject{}, fmt.Errorf("cannot list projects on account %s: %w", ctx.Name, err)
	}
	for _, project := range projects {
		if strings.TrimSpace(project.ID) == wanted {
			return project, nil
		}
	}
	return SyncProject{}, fmt.Errorf("account %s does not include project %s", ctx.Name, wanted)
}

func schemaSnapshotDest(out string) string {
	if strings.EqualFold(filepath.Ext(out), ".json") {
		return out
	}
	return filepath.Join(out, "apito_schema.json")
}

func writeSchemaSnapshot(schemaPath, dest string) error {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	pretty, err := sanitizedSchema(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dest, pretty, 0o644)
}

func checkSchemaSnapshot(schemaPath, dest string) error {
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	next, err := sanitizedSchema(raw)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(dest)
	if err != nil {
		return fmt.Errorf("schema snapshot drifted: %s", dest)
	}
	left, err := canonicalJSON(next)
	if err != nil {
		return err
	}
	right, err := canonicalJSON(current)
	if err != nil || !bytes.Equal(left, right) {
		return fmt.Errorf("schema snapshot drifted: %s", dest)
	}
	return nil
}

func sanitizedSchema(raw []byte) ([]byte, error) {
	if bytes.Contains(raw, []byte("apt_")) || bytes.Contains(raw, []byte("ak_")) {
		return nil, fmt.Errorf("refusing to write the schema snapshot because it contains a credential")
	}
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("schema snapshot is not JSON: %w", err)
	}
	encoded, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(encoded, []byte("__schema")) {
		return nil, fmt.Errorf("schema snapshot is missing __schema")
	}
	pretty, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(pretty, '\n'), nil
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	return json.Marshal(parsed)
}
