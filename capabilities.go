package main

import (
	"fmt"
	"strings"
)

const (
	capProjectAccess          = "project_access"
	capProjectTokens          = "project_tokens"
	capPublishedIntrospection = "published_introspection"
	capSchemaStaging          = "schema_staging"
	capTenantCatalog          = "tenant_catalog"
	capMCP                    = "mcp"
	capClientLifecycle        = "client_lifecycle"
	capManagedBackups         = "managed_backups"
	capRestoreActions         = "restore_actions"
	capCLILogin               = "cli_login"
)

// ServerCapabilities is discovered from the server. Account kind is only a hint.
type ServerCapabilities struct {
	Connected     bool
	EngineVersion string
	ConsoleURL    string
	Flags         map[string]bool
}

func (s ServerCapabilities) Has(name string) bool {
	return s.Flags != nil && s.Flags[name]
}

func errCloudCapability(feature string) error {
	return fmt.Errorf("%s requires an Apito Cloud account with the required capability. Run apito login", feature)
}

func (c *SystemClient) probeCapabilities() (ServerCapabilities, error) {
	out := ServerCapabilities{Flags: map[string]bool{}}
	if c == nil || strings.TrimSpace(c.baseURL()) == "" {
		return out, fmt.Errorf("server URL is required")
	}
	// /system/health is connectivity only. It is not an edition or version source.
	if _, _, err := c.do("GET", c.baseURL()+"/system/health", nil); err == nil {
		out.Connected = true
	}
	var caps struct {
		Capabilities map[string]bool `json:"capabilities"`
		ConsoleURL   string          `json:"console_url"`
		Version      string          `json:"engine_version"`
	}
	if err := c.getJSON("/system/cli/v1/capabilities", &caps); err == nil {
		out.Connected = true
		for k, v := range caps.Capabilities {
			out.Flags[k] = v
		}
		out.ConsoleURL = strings.TrimSpace(caps.ConsoleURL)
		out.EngineVersion = strings.TrimSpace(caps.Version)
	}
	var status struct {
		SchemaVersioningStatus struct {
			Enabled      bool `json:"enabled"`
			Capabilities struct {
				History bool `json:"history"`
			} `json:"capabilities"`
		} `json:"schemaVersioningStatus"`
	}
	// Catalog and self-introspection confirm the access-token surface. Failures are ignored.
	if strings.TrimSpace(c.token) != "" {
		var me map[string]interface{}
		if err := c.getJSON("/system/access-tokens/me", &me); err == nil {
			out.Connected = true
		}
		var catalog map[string]interface{}
		if err := c.getJSON("/system/access-tokens/catalog", &catalog); err == nil {
			out.Connected = true
		}
	}
	query := `query { schemaVersioningStatus { enabled capabilities { history } } }`
	if err := c.execute(query, nil, &status); err == nil {
		out.Connected = true
		if status.SchemaVersioningStatus.Enabled || status.SchemaVersioningStatus.Capabilities.History {
			out.Flags[capSchemaStaging] = true
		}
	}
	if out.Flags == nil {
		out.Flags = map[string]bool{}
	}
	// Core protocol commands stay available when the server answered at all.
	if out.Connected {
		if _, ok := out.Flags[capProjectAccess]; !ok {
			out.Flags[capProjectAccess] = true
		}
		if _, ok := out.Flags[capProjectTokens]; !ok {
			out.Flags[capProjectTokens] = true
		}
		if _, ok := out.Flags[capPublishedIntrospection]; !ok {
			out.Flags[capPublishedIntrospection] = true
		}
		if _, ok := out.Flags[capMCP]; !ok {
			out.Flags[capMCP] = true
		}
	}
	return out, nil
}

func requireCapability(caps ServerCapabilities, name, feature string) error {
	if caps.Has(name) {
		return nil
	}
	return errCloudCapability(feature)
}
