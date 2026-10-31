package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mcpClient      string
	mcpStdout      bool
	mcpOut         string
	mcpRawToken    bool
	mcpTransport   string
	mcpServeBridge = runMCPStdioBridge
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Connect to the engine MCP endpoint",
}

var mcpConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Print a client config that launches apito mcp serve",
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := buildMCPConfig(flagAccount, flagProject, mcpRawToken && flagYes)
		if err != nil {
			return err
		}
		if mcpRawToken && !flagYes {
			return fmt.Errorf("a raw token in MCP config requires --yes after you confirm. Prefer apito mcp serve, which keeps the token out of the file")
		}
		if strings.Contains(string(body), "apt_") || strings.Contains(string(body), "ak_") {
			if !(mcpRawToken && flagYes) {
				return fmt.Errorf("refusing to write an MCP config that contains a token")
			}
			fmt.Fprintln(os.Stderr, "warning: this MCP config contains a raw access token. File mode is 0600. Prefer apito mcp serve.")
		}
		if mcpStdout || mcpOut == "" {
			fmt.Println(string(body))
		}
		if mcpOut != "" {
			if err := os.MkdirAll(filepath.Dir(mcpOut), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(mcpOut, body, 0o600); err != nil {
				return err
			}
		}
		return nil
	},
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Bridge stdio to the engine MCP endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, err := resolveMCPServeContext()
		if err != nil {
			return err
		}
		if err := requireCapability(ctx.Caps, capMCP, "apito mcp"); err != nil {
			return err
		}
		if strings.TrimSpace(ctx.ProjectID) == "" {
			return fmt.Errorf("a project is required. Pass --project")
		}
		target, err := resolveMCPHTTPTarget(ctx.Account)
		if err != nil {
			return err
		}
		return mcpServeBridge(target, ctx.Account.MCPAccessToken, os.Stdin, os.Stdout, nil)
	},
}

func resolveMCPConfigAccount(account string) (string, error) {
	account = strings.TrimSpace(account)
	if account != "" {
		return account, nil
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	if name := strings.TrimSpace(cfg.DefaultAccount); name != "" {
		if _, ok := cfg.Accounts[name]; ok {
			return name, nil
		}
	}
	if len(cfg.Accounts) == 1 {
		for only := range cfg.Accounts {
			return only, nil
		}
	}
	return "", fmt.Errorf("no account connected. Run apito login")
}

func buildMCPConfig(account, project string, includeRaw bool) ([]byte, error) {
	resolved, err := resolveMCPConfigAccount(account)
	if err != nil {
		return nil, err
	}
	account = resolved
	var loaded AccountConfig
	var hasAccount bool
	if strings.TrimSpace(project) == "" || includeRaw {
		cfg, err := loadCLIConfig()
		if includeRaw && err != nil {
			return nil, err
		}
		if err == nil {
			loaded, hasAccount = cfg.Accounts[account]
			if strings.TrimSpace(project) == "" {
				project = loaded.MCPProjectID
			}
		}
	}
	if includeRaw {
		return nil, fmt.Errorf("HTTP MCP config does not embed tokens")
	}
	target := mcpHTTPTarget{PublicURL: "https://api.apito.io/mcp"}
	if hasAccount {
		resolved, err := resolveMCPHTTPTarget(loaded)
		if err != nil {
			return nil, err
		}
		target = resolved
	}
	key := account
	if key == "default" {
		key = "apito"
	}
	doc := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			key: map[string]interface{}{"type": "http", "url": target.PublicURL},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func resolveMCPServeContext() (commandContext, error) {
	if err := ensureCLIConfig(); err != nil {
		return commandContext{}, err
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return commandContext{}, err
	}
	name := strings.TrimSpace(flagAccount)
	if name == "" {
		name = strings.TrimSpace(cfg.DefaultAccount)
	}
	if name == "" {
		if len(cfg.Accounts) == 1 {
			for only := range cfg.Accounts {
				name = only
			}
		}
	}
	account, ok := cfg.Accounts[name]
	if !ok || strings.TrimSpace(account.ServerURL) == "" {
		return commandContext{}, fmt.Errorf("no account connected. Run apito mcp connect")
	}
	token := strings.TrimSpace(account.MCPAccessToken)
	if token == "" {
		return commandContext{}, fmt.Errorf("account %s has no MCP access token. Run apito mcp connect", name)
	}
	if IsRetiredTokenPrefix(token) {
		return commandContext{}, fmt.Errorf("account %s: %s", name, TokenFormatRetiredMessage)
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	client := newSystemClient(account.ServerURL, token, timeout)
	caps, err := client.probeCapabilities()
	if err != nil {
		return commandContext{}, err
	}
	projectID, err := selectMCPProject(account)
	if err != nil {
		return commandContext{}, err
	}
	tenantID, err := selectMCPTenant(account)
	if err != nil {
		return commandContext{}, err
	}
	if projectID != "" {
		client = client.WithProject(projectID)
	}
	if tenantID != "" {
		client = client.WithTenant(tenantID)
	}
	return commandContext{
		Name:      name,
		Account:   account,
		Client:    client,
		Caps:      caps,
		ProjectID: projectID,
		TenantID:  tenantID,
		Timeout:   timeout,
	}, nil
}

func selectMCPProject(account AccountConfig) (string, error) {
	requested := strings.TrimSpace(flagProject)
	mode := strings.TrimSpace(account.MCPProjectGrantMode)
	if requested != "" {
		if mode == "selected" && len(account.MCPProjectIDs) > 0 && !containsString(account.MCPProjectIDs, requested) {
			return "", fmt.Errorf("project %s is outside this MCP grant. Pass one of: %s", requested, strings.Join(account.MCPProjectIDs, ", "))
		}
		return requested, nil
	}
	if id := strings.TrimSpace(account.MCPProjectID); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("a project is required. Pass --project")
}

func selectMCPTenant(account AccountConfig) (string, error) {
	requested := strings.TrimSpace(flagTenant)
	mode := strings.TrimSpace(account.MCPTenantGrantMode)
	if mode == "none" {
		if requested != "" {
			return "", fmt.Errorf("this MCP grant does not include tenant scope")
		}
		return "", nil
	}
	if requested == "" {
		return strings.TrimSpace(account.MCPTenantID), nil
	}
	if mode == "selected" && len(account.MCPTenantIDs) > 0 && !tenantInGrant(account.MCPTenantIDs, requested) {
		return "", fmt.Errorf("tenant %s is outside this MCP grant", requested)
	}
	return requested, nil
}

func mcpAllowedProjects(account AccountConfig, project string) string {
	if account.MCPProjectGrantMode == "selected" && len(account.MCPProjectIDs) > 0 && containsString(account.MCPProjectIDs, project) {
		return strings.Join(account.MCPProjectIDs, ",")
	}
	return project
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func tenantInGrant(grants map[string][]string, want string) bool {
	for _, tenants := range grants {
		if containsString(tenants, want) {
			return true
		}
	}
	return false
}

func initMCPFlags() {
	mcpConnectCmd.Flags().StringVar(&loginKind, "kind", "", "local, cloud, or custom")
	mcpConnectCmd.Flags().StringVar(&loginServer, "server", "", "Server URL for --kind custom")
	mcpConnectCmd.Flags().StringVar(&mcpClient, "client", "", "cursor, claude, openai, vscode, or all")
	mcpConfigCmd.Flags().StringVar(&mcpClient, "client", "cursor", "cursor or claude")
	mcpConfigCmd.Flags().BoolVar(&mcpStdout, "stdout", true, "Print the config")
	mcpConfigCmd.Flags().StringVar(&mcpOut, "out", "", "Also write the config to this file (mode 0600)")
	mcpConfigCmd.Flags().BoolVar(&mcpRawToken, "raw-token", false, "Embed the access token. Requires --yes")
	mcpConfigCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm a raw-token config")
	mcpConfigCmd.Flags().StringVar(&mcpTransport, "transport", "", "Ignored. Config is always the engine HTTP URL")
	mcpServeCmd.Flags().StringVar(&mcpTransport, "transport", "", "Ignored. Serve always bridges stdio to the engine HTTP endpoint")
}
