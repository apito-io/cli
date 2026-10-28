package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var installMCPRuntime = defaultInstallMCPRuntime

var mcpConnectCmd = &cobra.Command{
	Use:   "connect [account-name]",
	Short: "Approve MCP access and print the client config",
	Long: `Choose an account the same way as apito login.

Local, Cloud, and Custom use the same list. A saved account is listed too.
The choice builds the MCP URL. The command then prints one JSON config.
Cursor, Claude, OpenAI, and VS Code use that same JSON. The old apito-mcp
package is deprecated and is not downloaded.`,
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
	if strings.TrimSpace(loginKind) == "" && name == "" {
		choice, err := promptMCPConnectChoice()
		if err != nil {
			return err
		}
		if choice.Kind == "account" {
			return mcpConnectExisting(choice.Account)
		}
		loginKind = choice.Kind
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
			print_success(fmt.Sprintf("Saved MCP access for account %s (%s)", name, kind))
			return printMCPClientConfig(server, kind)
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

type mcpConnectChoice struct {
	Label   string
	Kind    string
	Account string
}

func mcpConnectChoices(cfg *CLIConfig) []mcpConnectChoice {
	var choices []mcpConnectChoice
	if cfg != nil {
		names := getAccountNames(cfg)
		sort.Strings(names)
		for _, name := range names {
			acc := cfg.Accounts[name]
			url, err := mcpPublicURL(acc)
			if err != nil {
				url = acc.ServerURL
			}
			label := name + " (" + url + ")"
			if name == cfg.DefaultAccount {
				label += " (default)"
			}
			choices = append(choices, mcpConnectChoice{Label: label, Kind: "account", Account: name})
		}
	}
	choices = append(choices,
		mcpConnectChoice{Label: "Local (console " + localConsoleURL + ")", Kind: "local"},
		mcpConnectChoice{Label: "Cloud (" + cloudLoginServer + ")", Kind: "cloud"},
		mcpConnectChoice{Label: "Custom (enter a server URL)", Kind: "custom"},
	)
	return choices
}

func promptMCPConnectChoice() (mcpConnectChoice, error) {
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return mcpConnectChoice{}, fmt.Errorf("choose a connection with --kind local, --kind cloud, or --kind custom")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return mcpConnectChoice{}, err
	}
	choices := mcpConnectChoices(cfg)
	labels := make([]string, len(choices))
	for i, choice := range choices {
		labels[i] = choice.Label
	}
	prompt := promptui.Select{
		Label: "Where should this account connect?",
		Items: labels,
		Size:  len(labels),
	}
	index, _, err := prompt.Run()
	if err != nil {
		return mcpConnectChoice{}, fmt.Errorf("MCP connect cancelled")
	}
	return choices[index], nil
}

func mcpConnectExisting(name string) error {
	acc, err := getAccountConfig(name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(acc.MCPAccessToken) == "" {
		kind := inferMCPDeployment(acc.ServerURL, acc.Kind)
		server := strings.TrimSuffix(strings.TrimSpace(acc.ServerURL), "/")
		if server == "" {
			return fmt.Errorf("account %s has no server URL", name)
		}
		return mcpConnectBrowser(name, server, kind)
	}
	print_success(fmt.Sprintf("Using MCP account %s", name))
	return printMCPClientConfig(acc.ServerURL, acc.Kind)
}

func mcpPublicURL(account AccountConfig) (string, error) {
	target, err := resolveMCPHTTPTarget(account)
	if err != nil {
		return "", err
	}
	return target.PublicURL, nil
}

func mcpClientConfigJSON(rawURL string) ([]byte, error) {
	doc := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"apito": map[string]interface{}{
				"type": "http",
				"url":  rawURL,
			},
		},
	}
	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func printMCPClientConfig(server, kind string) error {
	url, err := mcpPublicURL(AccountConfig{ServerURL: server, Kind: kind})
	if err != nil {
		return err
	}
	body, err := mcpClientConfigJSON(url)
	if err != nil {
		return err
	}
	client := strings.ToLower(strings.TrimSpace(mcpClient))
	if client == "" {
		client = "all"
	}
	fmt.Fprintf(os.Stderr, "MCP URL: %s\n", url)
	switch client {
	case "cursor", "claude", "openai", "vscode", "all":
		fmt.Fprintln(os.Stderr, "Paste this JSON into Cursor, Claude, OpenAI, or VS Code. The config is the same.")
	default:
		return fmt.Errorf("--client must be cursor, claude, openai, vscode, or all")
	}
	fmt.Print(string(body))
	return nil
}

func defaultInstallMCPRuntime(update bool) error {
	return ensureMCPRuntime(update)
}
