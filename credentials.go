package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

const (
	credentialService    = "apito-cli"
	mcpCredentialService = "apito-cli-mcp"
)

var credentialMu sync.Mutex

type credentialFile struct {
	Accounts    map[string]string `yaml:"accounts,omitempty"`
	MCPAccounts map[string]string `yaml:"mcp_accounts,omitempty"`
}

func credentialStoreKind() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APITO_CREDENTIAL_STORE"))) {
	case "keychain":
		if runtime.GOOS == "darwin" {
			return "keychain"
		}
	case "file", "":
		return "file"
	default:
		return "file"
	}
	return "file"
}

func credentialFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".apito")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.yml"), nil
}

func putAccountToken(account, token string) error {
	token = strings.TrimSpace(token)
	account = strings.TrimSpace(account)
	if account == "" || token == "" {
		return nil
	}
	if credentialStoreKind() == "keychain" {
		return putKeychainToken(account, token)
	}
	return putFileToken(account, token)
}

func getAccountToken(account string) (string, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", nil
	}
	if credentialStoreKind() == "keychain" {
		return getKeychainToken(account)
	}
	return getFileToken(account)
}

func deleteAccountToken(account string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil
	}
	if err := deleteMCPAccountToken(account); err != nil {
		return err
	}
	if credentialStoreKind() == "keychain" {
		_ = deleteKeychainToken(account)
	}
	return deleteFileToken(account)
}

func putMCPAccountToken(account, token string) error {
	token = strings.TrimSpace(token)
	account = strings.TrimSpace(account)
	if account == "" || token == "" {
		return nil
	}
	if credentialStoreKind() == "keychain" {
		return putKeychainSecret(mcpCredentialService, account, token)
	}
	return putFileSecret(account, token, true)
}

func getMCPAccountToken(account string) (string, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return "", nil
	}
	if credentialStoreKind() == "keychain" {
		return getKeychainSecret(mcpCredentialService, account)
	}
	return getFileSecret(account, true)
}

func deleteMCPAccountToken(account string) error {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil
	}
	if credentialStoreKind() == "keychain" {
		_ = deleteKeychainSecret(mcpCredentialService, account)
	}
	return deleteFileSecret(account, true)
}

func putFileToken(account, token string) error {
	return putFileSecret(account, token, false)
}

func getFileToken(account string) (string, error) {
	return getFileSecret(account, false)
}

func deleteFileToken(account string) error {
	return deleteFileSecret(account, false)
}

func putFileSecret(account, token string, mcp bool) error {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	file, path, err := readCredentialFile()
	if err != nil {
		return err
	}
	if mcp {
		if file.MCPAccounts == nil {
			file.MCPAccounts = map[string]string{}
		}
		file.MCPAccounts[account] = token
	} else {
		if file.Accounts == nil {
			file.Accounts = map[string]string{}
		}
		file.Accounts[account] = token
	}
	return writeCredentialFile(path, file)
}

func getFileSecret(account string, mcp bool) (string, error) {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	file, _, err := readCredentialFile()
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	if mcp {
		return strings.TrimSpace(file.MCPAccounts[account]), nil
	}
	return strings.TrimSpace(file.Accounts[account]), nil
}

func deleteFileSecret(account string, mcp bool) error {
	credentialMu.Lock()
	defer credentialMu.Unlock()
	file, path, err := readCredentialFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if mcp {
		if file.MCPAccounts == nil {
			return nil
		}
		delete(file.MCPAccounts, account)
	} else {
		if file.Accounts == nil {
			return nil
		}
		delete(file.Accounts, account)
	}
	return writeCredentialFile(path, file)
}

func readCredentialFile() (credentialFile, string, error) {
	path, err := credentialFilePath()
	if err != nil {
		return credentialFile{}, "", err
	}
	file := credentialFile{Accounts: map[string]string{}, MCPAccounts: map[string]string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return file, path, nil
		}
		return credentialFile{}, "", err
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return credentialFile{}, "", err
	}
	if file.Accounts == nil {
		file.Accounts = map[string]string{}
	}
	if file.MCPAccounts == nil {
		file.MCPAccounts = map[string]string{}
	}
	return file, path, nil
}

func writeCredentialFile(path string, file credentialFile) error {
	data, err := yaml.Marshal(file)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func putKeychainToken(account, token string) error {
	return putKeychainSecret(credentialService, account, token)
}

func getKeychainToken(account string) (string, error) {
	return getKeychainSecret(credentialService, account)
}

func deleteKeychainToken(account string) error {
	return deleteKeychainSecret(credentialService, account)
}

func putKeychainSecret(service, account, token string) error {
	cmd := exec.Command("security", "add-generic-password", "-U", "-a", account, "-s", service, "-w", token)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain save failed: %s", redactSecrets(string(out)))
	}
	return nil
}

func getKeychainSecret(service, account string) (string, error) {
	cmd := exec.Command("security", "find-generic-password", "-a", account, "-s", service, "-w")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

func deleteKeychainSecret(service, account string) error {
	cmd := exec.Command("security", "delete-generic-password", "-a", account, "-s", service)
	_ = cmd.Run()
	return nil
}

var (
	aptTokenPattern = regexp.MustCompile(`apt_[A-Za-z0-9_\-]+`)
	akTokenPattern  = regexp.MustCompile(`ak_[A-Za-z0-9_\-]+`)
)

func redactSecrets(s string) string {
	s = aptTokenPattern.ReplaceAllString(s, "apt_***")
	s = akTokenPattern.ReplaceAllString(s, "ak_***")
	return s
}

// Token returns the in-memory access token, falling back to a legacy YAML value.
func (a AccountConfig) Token() string {
	if t := strings.TrimSpace(a.AccessToken); t != "" {
		return t
	}
	return strings.TrimSpace(a.CloudSyncKey)
}

// SetToken stores the token in memory and clears the legacy YAML field.
func (a *AccountConfig) SetToken(token string) {
	if a == nil {
		return
	}
	a.AccessToken = strings.TrimSpace(token)
	a.CloudSyncKey = ""
}
