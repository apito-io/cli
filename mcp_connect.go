package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var installMCPRuntime = defaultInstallMCPRuntime

var mcpConnectCmd = &cobra.Command{
	Use:   "connect [account-name]",
	Short: "Approve MCP access and install the local MCP runtime",
	Long: `Choose where this MCP account connects.

Local asks the engine on this machine for a code, then opens the console MCP page.
Cloud uses Apito Cloud and opens the console URL that server returns.
Custom asks for a server URL and uses the same browser approval.

The MCP token is stored separately from the CLI token. apito mcp serve stays a
foreground process that Cursor or Claude launches.`,
	RunE: runMCPConnect,
}

func runMCPConnect(cmd *cobra.Command, args []string) error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	name := strings.TrimSpace(flagAccount)
	if name == "" && len(args) > 0 {
		name = strings.TrimSpace(args[0])
	}
	kind, err := resolveMCPKind()
	if err != nil {
		return err
	}
	if name == "" {
		name = kind
	}
	switch kind {
	case "local":
		return mcpConnectBrowser(name, localLoginServer, "local")
	case "cloud":
		return mcpConnectBrowser(name, cloudLoginServer, "cloud")
	case "custom":
		server, err := mcpCustomServer()
		if err != nil {
			return err
		}
		return mcpConnectBrowser(name, server, "custom")
	default:
		return fmt.Errorf("--kind must be local, cloud, or custom")
	}
}

func resolveMCPKind() (string, error) {
	if kind := strings.TrimSpace(loginKind); kind != "" {
		return normalizeLoginKind(kind)
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return "", fmt.Errorf("choose a connection with --kind local, --kind cloud, or --kind custom")
	}
	prompt := promptui.Select{
		Label: "Where should MCP connect?",
		Items: []string{
			"Local (console " + localConsoleURL + ")",
			"Cloud (" + cloudLoginServer + ")",
			"Custom (enter a server URL)",
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("MCP connect cancelled")
	}
	switch index {
	case 0:
		return "local", nil
	case 1:
		return "cloud", nil
	default:
		return "custom", nil
	}
}

func mcpCustomServer() (string, error) {
	server := strings.TrimSuffix(strings.TrimSpace(loginServer), "/")
	if server == "" {
		entered, err := loginRead("Server URL: ")
		if err != nil {
			return "", err
		}
		server = strings.TrimSuffix(strings.TrimSpace(entered), "/")
	}
	if !strings.HasPrefix(server, "http://") && !strings.HasPrefix(server, "https://") {
		return "", fmt.Errorf("server URL must start with http:// or https://")
	}
	return server, nil
}

func mcpConnectBrowser(name, server, kind string) error {
	server = strings.TrimSuffix(strings.TrimSpace(server), "/")
	client := newSystemClient(server, "", 30)
	var started struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
	if err := client.postJSON("/system/mcp/v1/login/start", map[string]string{"account_name": name}, &started); err != nil {
		return fmt.Errorf("this server does not support MCP browser approval yet (%s)", redactSecrets(err.Error()))
	}
	page := started.VerificationURI
	if kind == "local" {
		page = localConsolePage("/mcp", started.DeviceCode, started.UserCode)
	}
	if started.DeviceCode == "" || page == "" {
		return fmt.Errorf("MCP approval did not return a device code")
	}
	fmt.Fprintf(os.Stderr, "Open %s\nCode: %s\n", page, started.UserCode)
	if err := loginOpen(page); err != nil {
		fmt.Fprintf(os.Stderr, "Could not open a browser. Open the URL above.\n")
	}
	interval := started.Interval
	if interval < 1 {
		interval = 5
	}
	deadline := time.Now().Add(time.Duration(started.ExpiresIn) * time.Second)
	if started.ExpiresIn <= 0 {
		deadline = time.Now().Add(10 * time.Minute)
	}
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("MCP approval expired. Run apito mcp connect again")
		}
		var polled struct {
			Status           string              `json:"status"`
			AccessToken      string              `json:"access_token"`
			ConsoleURL       string              `json:"console_url"`
			ProjectGrantMode string              `json:"project_grant_mode"`
			ProjectIDs       []string            `json:"project_ids"`
			TenantGrantMode  string              `json:"tenant_grant_mode"`
			TenantIDs        map[string][]string `json:"tenant_ids"`
		}
		if err := client.postJSON("/system/mcp/v1/login/poll", map[string]string{"device_code": started.DeviceCode}, &polled); err != nil {
			return err
		}
		switch polled.Status {
		case "approved":
			if !IsAccessToken(polled.AccessToken) {
				return fmt.Errorf("MCP approval did not return an apt_ access token")
			}
			if err := saveNamedMCPAccount(name, mcpAccountGrant{
				Server:           server,
				Kind:             kind,
				ConsoleURL:       polled.ConsoleURL,
				Token:            polled.AccessToken,
				ProjectGrantMode: polled.ProjectGrantMode,
				ProjectIDs:       polled.ProjectIDs,
				TenantGrantMode:  polled.TenantGrantMode,
				TenantIDs:        polled.TenantIDs,
			}); err != nil {
				return err
			}
			if err := installMCPRuntime(true); err != nil {
				return err
			}
			print_success(fmt.Sprintf("Saved MCP access for account %s (%s)", name, kind))
			return nil
		case "pending":
			time.Sleep(time.Duration(interval) * time.Second)
		case "expired", "denied":
			return fmt.Errorf("MCP approval %s", polled.Status)
		default:
			return fmt.Errorf("unexpected MCP approval status %q", polled.Status)
		}
	}
}

type mcpAccountGrant struct {
	Server           string
	Kind             string
	ConsoleURL       string
	Token            string
	ProjectGrantMode string
	ProjectIDs       []string
	TenantGrantMode  string
	TenantIDs        map[string][]string
}

func saveNamedMCPAccount(name string, grant mcpAccountGrant) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if cfg.Accounts == nil {
		cfg.Accounts = map[string]AccountConfig{}
	}
	acc := cfg.Accounts[name]
	server := strings.TrimSuffix(strings.TrimSpace(grant.Server), "/")
	if existing := strings.TrimSuffix(strings.TrimSpace(acc.ServerURL), "/"); existing != "" && existing != server {
		return fmt.Errorf("account %s already points at %s. Use another account name for %s", name, acc.ServerURL, server)
	}
	acc.ServerURL = server
	if strings.TrimSpace(acc.Kind) == "" {
		acc.Kind = grant.Kind
	}
	if strings.TrimSpace(grant.ConsoleURL) != "" {
		acc.ConsoleURL = strings.TrimSpace(grant.ConsoleURL)
	}
	acc.MCPAccessToken = strings.TrimSpace(grant.Token)
	acc.MCPProjectGrantMode = strings.TrimSpace(grant.ProjectGrantMode)
	acc.MCPProjectIDs = append([]string(nil), grant.ProjectIDs...)
	acc.MCPTenantGrantMode = strings.TrimSpace(grant.TenantGrantMode)
	acc.MCPTenantIDs = grant.TenantIDs
	acc.MCPProjectID = ""
	acc.MCPTenantID = ""
	if acc.MCPProjectGrantMode == "selected" && len(acc.MCPProjectIDs) == 1 {
		acc.MCPProjectID = acc.MCPProjectIDs[0]
	}
	if acc.MCPTenantGrantMode == "selected" {
		var only string
		count := 0
		for _, tenants := range acc.MCPTenantIDs {
			for _, tenant := range tenants {
				tenant = strings.TrimSpace(tenant)
				if tenant == "" {
					continue
				}
				count++
				only = tenant
			}
		}
		if count == 1 {
			acc.MCPTenantID = only
		}
	}
	cfg.Accounts[name] = acc
	if cfg.DefaultAccount == "" {
		cfg.DefaultAccount = name
	}
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	return nil
}

func defaultInstallMCPRuntime(update bool) error {
	return ensureMCPRuntime(update)
}
