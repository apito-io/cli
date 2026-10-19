package main

import (
	"fmt"
	"strings"
)

var (
	flagAccount string
	flagProject string
	flagTenant  string
	flagJSON    bool
	flagDryRun  bool
	flagYes     bool
)

type commandContext struct {
	Name      string
	Account   AccountConfig
	Client    *SystemClient
	Caps      ServerCapabilities
	ProjectID string
	TenantID  string
	Timeout   int
}

func resolveCommandContext() (commandContext, error) {
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
	if name == "" || cfg.Accounts == nil || cfg.Accounts[name].ServerURL == "" {
		return commandContext{}, fmt.Errorf("no account connected. Run apito login")
	}
	account, err := getAccountConfig(name)
	if err != nil {
		return commandContext{}, err
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 30
	}
	client := newSystemClient(account.ServerURL, account.Token(), timeout)
	caps, err := client.probeCapabilities()
	if err != nil {
		return commandContext{}, err
	}
	if account.ConsoleURL == "" && caps.ConsoleURL != "" {
		account.ConsoleURL = caps.ConsoleURL
	}
	account.Capabilities = caps.Flags
	projectID := strings.TrimSpace(flagProject)
	if projectID == "" {
		projectID = strings.TrimSpace(account.ProjectID)
	}
	tenantID := strings.TrimSpace(flagTenant)
	if tenantID == "" {
		tenantID = strings.TrimSpace(account.TenantID)
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

func (c commandContext) syncClient() *SyncGraphQLClient {
	return &SyncGraphQLClient{SystemClient: *c.Client}
}
