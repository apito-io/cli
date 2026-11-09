package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func resolvePluginAccount(flag string) (string, error) {
	flag = strings.TrimSpace(flag)
	if flag != "" {
		if _, err := getAccountConfig(flag); err != nil {
			return "", err
		}
		return flag, nil
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(cfg.DefaultAccount) != "" {
		if _, err := getAccountConfig(cfg.DefaultAccount); err != nil {
			return "", err
		}
		return cfg.DefaultAccount, nil
	}
	return selectAccountForSync("plugin")
}

func pluginAccountFromCmd(cmd *cobra.Command) (string, AccountConfig, error) {
	flag, _ := cmd.Flags().GetString("account")
	name, err := resolvePluginAccount(flag)
	if err != nil {
		return "", AccountConfig{}, err
	}
	account, err := getAccountConfig(name)
	if err != nil {
		return "", AccountConfig{}, err
	}
	return name, account, nil
}

func pluginAccountAndRoot(cmd *cobra.Command) (string, string, error) {
	name, account, err := pluginAccountFromCmd(cmd)
	if err != nil {
		return "", "", err
	}
	dirFlag, _ := cmd.Flags().GetString("dir")
	root, err := pluginRootForAccount(name, account, dirFlag)
	if err != nil {
		return "", "", err
	}
	return name, root, nil
}

func accountIsLocal(account AccountConfig) bool {
	if strings.EqualFold(strings.TrimSpace(account.Kind), "local") {
		return true
	}
	parsed, err := url.Parse(strings.TrimSpace(account.ServerURL))
	if err != nil {
		return false
	}
	host := strings.TrimSpace(parsed.Hostname())
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func pluginRootForAccount(accountName string, account AccountConfig, dirFlag string) (string, error) {
	if strings.TrimSpace(dirFlag) != "" {
		return filepath.Abs(dirFlag)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	base := filepath.Join(home, ".apito", "plugins")
	if accountIsLocal(account) {
		return base, nil
	}
	if strings.TrimSpace(accountName) == "" {
		return "", fmt.Errorf("account name is required for a non-local plugin path")
	}
	return filepath.Join(base, accountName), nil
}

func ensurePluginCaps(accountName string, account AccountConfig, need []string) (AccountConfig, error) {
	missing, err := missingPluginCaps(account, need)
	if err != nil {
		return account, err
	}
	if len(missing) == 0 {
		return account, nil
	}
	print_warning(fmt.Sprintf(
		"Account %q is missing %s. CLI login does not include plugin install. Choose the CLI plugin preset.",
		accountName, strings.Join(missing, ", "),
	))
	if !syncStdinInteractive() {
		return account, fmt.Errorf("plugin command blocked for %q: missing %s", accountName, strings.Join(missing, ", "))
	}
	prompt := promptui.Select{
		Label: "New access token required",
		Items: []string{"Log in again for " + accountName, "Stop"},
	}
	index, _, err := prompt.Run()
	if err != nil || index != 0 {
		return account, fmt.Errorf("plugin command blocked for %q: missing %s", accountName, strings.Join(missing, ", "))
	}
	kind := strings.TrimSpace(account.Kind)
	if kind == "" {
		kind = "custom"
	}
	if err := loginBrowser(accountName, account.ServerURL, kind); err != nil {
		return account, err
	}
	fresh, err := getAccountConfig(accountName)
	if err != nil {
		return account, err
	}
	missing, err = missingPluginCaps(fresh, need)
	if err != nil {
		return fresh, err
	}
	if len(missing) == 0 {
		print_status("New token includes the plugin capability.")
		return fresh, nil
	}
	return fresh, fmt.Errorf("plugin command blocked for %q: missing %s", accountName, strings.Join(missing, ", "))
}

func missingPluginCaps(account AccountConfig, need []string) ([]string, error) {
	client := newSystemClient(account.ServerURL, account.Token(), 30)
	token, err := readSyncToken(client)
	if err != nil {
		return nil, fmt.Errorf("could not read token permissions: %w", err)
	}
	granted := map[string]struct{}{}
	for _, cap := range token.Capabilities {
		cap = strings.TrimSpace(cap)
		if cap != "" {
			granted[cap] = struct{}{}
		}
	}
	if _, ok := granted["plugins.deploy"]; ok {
		granted["plugins.read"] = struct{}{}
		granted["plugins.write"] = struct{}{}
	}
	if _, ok := granted["plugins.write"]; ok {
		granted["plugins.read"] = struct{}{}
	}
	var missing []string
	for _, cap := range need {
		if _, ok := granted[cap]; !ok {
			missing = append(missing, cap)
		}
	}
	return missing, nil
}

func rewritePluginIdentity(dir, id string) error {
	path := filepath.Join(dir, "config.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cloned plugin has no config.yml: %w", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	plugin, _ := doc["plugin"].(map[string]any)
	if plugin == nil {
		return fmt.Errorf("cloned plugin config has no plugin block")
	}
	old, _ := plugin["id"].(string)
	old = strings.TrimSpace(old)
	if old == "" || old == id {
		return nil
	}
	targets := []string{
		path,
		filepath.Join(dir, "main.go"),
		filepath.Join(dir, "go.mod"),
		filepath.Join(dir, ".github", "workflows", "build-release.yml"),
		filepath.Join(dir, ".gitignore"),
		filepath.Join(dir, "ui.js"),
		filepath.Join(dir, "ui", "package.json"),
		filepath.Join(dir, "ui", "package-lock.json"),
		filepath.Join(dir, "ui", "src", "host.ts"),
		filepath.Join(dir, "ui", "src", "index.tsx"),
	}
	for _, target := range targets {
		body, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		updated := strings.ReplaceAll(string(body), old, id)
		if err := os.WriteFile(target, []byte(updated), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func requireHCPluginID(id string) error {
	if !strings.HasPrefix(strings.TrimSpace(id), "hc-") {
		return fmt.Errorf("plugin id must begin with hc-")
	}
	return nil
}

func localPluginDir(root, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("plugin id is required")
	}
	if err := requireHCPluginID(id); err != nil {
		return "", err
	}
	candidate := filepath.Join(root, id)
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate, nil
	}
	if cfg, err := readPluginConfig(root); err == nil && cfg.Plugin.ID == id {
		return root, nil
	}
	return "", fmt.Errorf("plugin %s is not in %s", id, root)
}
