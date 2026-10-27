package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

var (
	loginKind        string
	loginServer      string
	loginOpen        = openBrowser
	loginRead        = readSecretLine
	localLoginServer = "http://localhost:" + studioEnginePort
	localConsoleURL  = "http://localhost:" + studioPublishPort
	cloudLoginServer = "https://api.apito.io"
)

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func readSecretLine(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

var loginCmd = &cobra.Command{
	Use:   "login [account-name]",
	Short: "Connect a local, Cloud, or custom account",
	Long: `Choose where this account connects.

Local asks the engine on this machine for a code, then opens the console.
Cloud uses Apito Cloud and opens the console URL that server returns.
Both store the apt_ token outside config.yml. If the console is not running,
the browser shows that. The engine only issues the code.

Custom asks for a server URL, opens that server's access-token page, and stores
the apt_ token the page approves. It then asks for an account name. Do not
pass the token as a flag.`,
	RunE: runLogin,
}

func initLoginFlags() {
	loginCmd.Flags().StringVar(&loginKind, "kind", "", "local, cloud, or custom")
	loginCmd.Flags().StringVar(&loginServer, "server", "", "Server URL for --kind custom")
}

func runLogin(cmd *cobra.Command, args []string) error {
	if err := ensureCLIConfig(); err != nil {
		return err
	}
	name := strings.TrimSpace(flagAccount)
	if name == "" && len(args) > 0 {
		name = strings.TrimSpace(args[0])
	}
	kind, err := resolveLoginKind()
	if err != nil {
		return err
	}
	if name == "" && kind != "custom" {
		name = kind
	}
	switch kind {
	case "local":
		return loginBrowser(name, localLoginServer, "local")
	case "cloud":
		return loginBrowser(name, cloudLoginServer, "cloud")
	case "custom":
		return loginCustom(name)
	default:
		return fmt.Errorf("--kind must be local, cloud, or custom")
	}
}

func resolveLoginKind() (string, error) {
	if kind := strings.TrimSpace(loginKind); kind != "" {
		return normalizeLoginKind(kind)
	}
	fi, err := os.Stdin.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return "", fmt.Errorf("choose a login with --kind local, --kind cloud, or --kind custom")
	}
	prompt := promptui.Select{
		Label: "Where should this account connect?",
		Items: []string{
			"Local (console " + localConsoleURL + ")",
			"Cloud (" + cloudLoginServer + ")",
			"Custom (enter a server URL)",
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		return "", fmt.Errorf("login cancelled")
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

func normalizeLoginKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "local", "oss":
		return "local", nil
	case "cloud":
		return "cloud", nil
	case "custom", "customer", "other", "enterprise", "self-hosted":
		return "custom", nil
	default:
		return "", fmt.Errorf("--kind must be local, cloud, or custom")
	}
}

func loginCustom(name string) error {
	server, err := customServerURL()
	if err != nil {
		return err
	}
	approved, err := completeBrowserLogin(server, "custom", name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		name, err = promptAccountName()
		if err != nil {
			return err
		}
	}
	return saveNamedAccount(name, server, approved.Token, "custom", approved.ConsoleURL)
}

func customServerURL() (string, error) {
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

func promptAccountName() (string, error) {
	name, err := loginRead("Account name: ")
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, " \t/") {
		return "", fmt.Errorf("enter an account name without spaces, or run apito login <name>")
	}
	return name, nil
}

type browserLoginResult struct {
	Token      string
	ConsoleURL string
}

func loginBrowser(name, server, kind string) error {
	approved, err := completeBrowserLogin(server, kind, name)
	if err != nil {
		return err
	}
	return saveNamedAccount(name, server, approved.Token, kind, approved.ConsoleURL)
}

func completeBrowserLogin(server, kind, accountName string) (browserLoginResult, error) {
	server = strings.TrimSuffix(strings.TrimSpace(server), "/")
	client := newSystemClient(server, "", 30)
	var started struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		Interval        int    `json:"interval"`
		ExpiresIn       int    `json:"expires_in"`
	}
	if err := client.postJSON("/system/cli/v1/login/start", map[string]string{"account_name": accountName}, &started); err != nil {
		return browserLoginResult{}, fmt.Errorf("this server does not support browser login yet (%s)", redactSecrets(err.Error()))
	}
	page := started.VerificationURI
	switch kind {
	case "local":
		page = localConsoleLoginPage(started.DeviceCode, started.UserCode)
	case "custom":
		page = customConsoleLoginPage(server, started.DeviceCode, started.UserCode)
	}
	if started.DeviceCode == "" || page == "" {
		return browserLoginResult{}, fmt.Errorf("browser login did not return a device code")
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
			return browserLoginResult{}, fmt.Errorf("browser login expired. Run apito login again")
		}
		var polled struct {
			Status      string `json:"status"`
			AccessToken string `json:"access_token"`
			ConsoleURL  string `json:"console_url"`
		}
		err := client.postJSON("/system/cli/v1/login/poll", map[string]string{"device_code": started.DeviceCode}, &polled)
		if err != nil {
			return browserLoginResult{}, err
		}
		switch polled.Status {
		case "approved":
			if !IsAccessToken(polled.AccessToken) {
				return browserLoginResult{}, fmt.Errorf("browser login did not return an apt_ access token")
			}
			consoleURL := polled.ConsoleURL
			if consoleURL == "" {
				consoleURL = server
			}
			return browserLoginResult{Token: polled.AccessToken, ConsoleURL: consoleURL}, nil
		case "pending":
			time.Sleep(time.Duration(interval) * time.Second)
		case "expired", "denied":
			return browserLoginResult{}, fmt.Errorf("browser login %s", polled.Status)
		default:
			return browserLoginResult{}, fmt.Errorf("unexpected login status %q", polled.Status)
		}
	}
}

func localConsoleLoginPage(deviceCode, userCode string) string {
	return localConsolePage("/cli", deviceCode, userCode)
}

func customConsoleLoginPage(server, deviceCode, userCode string) string {
	q := url.Values{}
	q.Set("device_code", deviceCode)
	if code := strings.TrimSpace(userCode); code != "" {
		q.Set("user_code", code)
	}
	return strings.TrimRight(server, "/") + "/cli?" + q.Encode()
}

func localConsolePage(path, deviceCode, userCode string) string {
	q := url.Values{}
	q.Set("device_code", deviceCode)
	if code := strings.TrimSpace(userCode); code != "" {
		q.Set("user_code", code)
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(localConsoleURL, "/") + path + "?" + q.Encode()
}

func saveNamedAccount(name, server, token, kind, consoleURL string) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if cfg.Accounts == nil {
		cfg.Accounts = map[string]AccountConfig{}
	}
	acc := cfg.Accounts[name]
	acc.ServerURL = server
	acc.Kind = kind
	if consoleURL != "" {
		acc.ConsoleURL = consoleURL
	}
	acc.SetToken(token)
	cfg.Accounts[name] = acc
	cfg.DefaultAccount = name
	if err := saveCLIConfig(cfg); err != nil {
		return err
	}
	print_success(fmt.Sprintf("Saved account %s (%s)", name, kind))
	noteLoginCompleted()
	return nil
}

func decodeLoginPoll(raw []byte) (status, token string, err error) {
	var polled struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &polled); err != nil {
		return "", "", err
	}
	return polled.Status, polled.AccessToken, nil
}
