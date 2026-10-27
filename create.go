package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a project on a connected account",
	Long: `Create a project on a local Studio account or a cloud account.

One configured account is used automatically. Several accounts open the same
picker as apito sync. When no account is connected, the command asks you to
connect one and then creates the project through the system API.

Plugin scaffolds stay on apito plugin create.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return createProject()
	},
}

func createProject() error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	accountName, err := resolveAccountForCreate()
	if err != nil {
		return err
	}
	account, err := getAccountConfig(accountName)
	if err != nil {
		return err
	}

	namePrompt := promptui.Prompt{Label: "Project name", Validate: func(s string) error {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("name is required")
		}
		return nil
	}}
	name, err := namePrompt.Run()
	if err != nil {
		return fmt.Errorf("project creation cancelled")
	}
	name = strings.TrimSpace(name)

	descPrompt := promptui.Prompt{Label: "Description (optional)"}
	description, err := descPrompt.Run()
	if err != nil {
		return fmt.Errorf("project creation cancelled")
	}

	typeSelect := promptui.Select{
		Label: "Project type",
		Items: []string{"general", "saas"},
	}
	_, projectType, err := typeSelect.Run()
	if err != nil {
		return fmt.Errorf("project creation cancelled")
	}

	projectID := slugProjectID(name)
	payload := map[string]interface{}{
		"name":          name,
		"description":   strings.TrimSpace(description),
		"id":            projectID,
		"project_type":  projectType,
		"database_type": "sqlite",
		"db_config": map[string]string{
			"file": "apito_" + projectID + ".sqlite",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := strings.TrimSuffix(account.ServerURL, "/") + "/system/project/create"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+account.Token())
	req.Header.Set("X-Use-Cookies", "false")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("create failed (%s): %s", resp.Status, strings.TrimSpace(string(respBody)))
	}
	print_success(fmt.Sprintf("Project %s created on account %s (%s)", projectID, accountName, account.ServerURL))
	return nil
}

func resolveAccountForCreate() (string, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	if len(cfg.Accounts) == 0 {
		print_status("Connect an account before creating a project. The account can be local Studio or cloud.")
		prompt := promptui.Prompt{Label: "Account name", Validate: func(s string) error {
			if strings.TrimSpace(s) == "" {
				return fmt.Errorf("name is required")
			}
			return nil
		}}
		name, err := prompt.Run()
		if err != nil {
			return "", fmt.Errorf("account connection cancelled")
		}
		name = strings.TrimSpace(name)
		createAccount(name)
		cfg, err = loadCLIConfig()
		if err != nil {
			return "", err
		}
		if _, ok := cfg.Accounts[name]; !ok {
			return "", fmt.Errorf("account was not connected")
		}
		return name, nil
	}
	return resolveSyncAccount("create", "")
}

func slugProjectID(name string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	base := strings.Trim(b.String(), "_")
	if base == "" {
		base = "project"
	}
	suffix := make([]byte, 2)
	if _, err := rand.Read(suffix); err != nil {
		return base
	}
	return fmt.Sprintf("%s_%x", base, suffix)
}
