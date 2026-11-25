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
	mcpClient    string
	mcpStdout    bool
	mcpOut       string
	mcpRawToken  bool
	mcpTransport string
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Connect to the engine MCP endpoint",
}

var mcpConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Print a client config for the engine MCP endpoint (HTTP)",
	RunE: func(cmd *cobra.Command, args []string) error {
		body, err := buildMCPConfig(flagAccount, flagProject, mcpRawToken && flagYes)
		if err != nil {
			return err
		}
		if mcpRawToken && !flagYes {
			return fmt.Errorf("a raw token in MCP config requires --yes after you confirm. Prefer the HTTP config, which keeps the token out of the file")
		}
		if strings.Contains(string(body), "apt_") || strings.Contains(string(body), "ak_") {
			if !(mcpRawToken && flagYes) {
				return fmt.Errorf("refusing to write an MCP config that contains a token")
			}
			fmt.Fprintln(os.Stderr, "warning: this MCP config contains a raw access token. File mode is 0600. Prefer the HTTP config.")
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

// mcpServeCmd is kept only so old client configs that still launch
// `apito mcp serve` get a clear message. The engine serves MCP over HTTP.
var mcpServeCmd = &cobra.Command{
	Use:        "serve",
	Short:      "Removed: connect clients to the engine MCP URL instead",
	Hidden:     true,
	Deprecated: "the engine serves MCP over HTTP; run apito mcp config or apito mcp connect",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("apito mcp serve was removed. The engine serves MCP over HTTP at <server>/mcp. Run `apito mcp connect` (or `apito mcp config`) to write a client config that points at it")
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
}
