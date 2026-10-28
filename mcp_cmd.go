package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mcpClient    string
	mcpStdout    bool
	mcpOut       string
	mcpRawToken  bool
	mcpTransport string
	mcpLookPath  = exec.LookPath
	mcpCommand   = defaultMCPCommand
	mcpRun       = defaultMCPRun
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Configure and serve the Apito MCP server",
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

var mcpUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "There is no separate MCP runtime to download",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("apito mcp does not install a separate server. The engine serves MCP over HTTP")
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
		if mcpRuntime() != "http" {
			return fmt.Errorf("apito mcp serve does not start a separate MCP server. The engine serves MCP over HTTP")
		}
		target, err := resolveMCPHTTPTarget(ctx.Account)
		if err != nil {
			return err
		}
		return runMCPStdioBridge(target, ctx.Account.MCPAccessToken, os.Stdin, os.Stdout, nil)
	},
}

func mcpRuntime() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APITO_MCP_RUNTIME"))) {
	case "node":
		return "node"
	case "http":
		return "http"
	}
	switch strings.ToLower(strings.TrimSpace(mcpTransport)) {
	case "stdio", "node":
		return "node"
	default:
		return "http"
	}
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
	if mcpRuntime() == "http" {
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
	args := []string{"mcp", "serve", "--account", account}
	if strings.TrimSpace(project) != "" {
		args = append(args, "--project", project)
	}
	server := map[string]interface{}{
		"command": "apito",
		"args":    args,
	}
	if includeRaw {
		if !hasAccount || strings.TrimSpace(loaded.MCPAccessToken) == "" {
			return nil, fmt.Errorf("account %s has no MCP access token. Run apito mcp connect", account)
		}
		server["env"] = map[string]string{"APITO_AUTH_TOKEN": loaded.MCPAccessToken}
	}
	doc := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"apito": server,
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func defaultMCPCommand() (string, []string, error) {
	if raw := strings.TrimSpace(os.Getenv("APITO_MCP_COMMAND")); raw != "" {
		parts := strings.Fields(raw)
		if len(parts) == 0 {
			return "", nil, fmt.Errorf("APITO_MCP_COMMAND is empty")
		}
		return parts[0], parts[1:], nil
	}
	if _, err := mcpLookPath("node"); err != nil {
		return "", nil, fmt.Errorf("Node.js is required for apito mcp serve. Install Node.js, or set APITO_MCP_COMMAND")
	}
	if dir := strings.TrimSpace(os.Getenv("APITO_MCP_DIR")); dir != "" {
		if _, err := mcpLookPath("npx"); err != nil {
			return "", nil, fmt.Errorf("npx is required to run APITO_MCP_DIR. Install Node.js, or set APITO_MCP_COMMAND")
		}
		return "npx", []string{"tsx", filepath.Join(dir, "src", "index.ts")}, nil
	}
	server, err := installedMCPServer()
	if err != nil {
		return "", nil, err
	}
	if server == "" {
		if err := ensureMCPRuntime(false); err != nil {
			return "", nil, err
		}
		server, err = installedMCPServer()
		if err != nil {
			return "", nil, err
		}
	}
	if server == "" {
		return "", nil, fmt.Errorf("MCP runtime is not installed. Run apito mcp connect or apito mcp update")
	}
	return "node", []string{server}, nil
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

func defaultMCPRun(bin string, args []string, env []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
	mcpConfigCmd.Flags().StringVar(&mcpTransport, "transport", "", "stdio or http. http prints a token-free URL")
	mcpServeCmd.Flags().StringVar(&mcpTransport, "transport", "", "stdio or http. http bridges stdio to the account MCP URL")
}
