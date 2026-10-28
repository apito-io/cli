package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialFileOmitsTokenAndIsPrivate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	acc := AccountConfig{ServerURL: "http://engine.local"}
	acc.SetToken("apt_secretvalue")
	cfg := &CLIConfig{Accounts: map[string]AccountConfig{"local": acc}, DefaultAccount: "local"}
	if err := saveCLIConfig(cfg); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".apito", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "apt_secretvalue") {
		t.Fatalf("config retained the token:\n%s", raw)
	}
	info, err := os.Stat(filepath.Join(dir, ".apito", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %o", info.Mode().Perm())
	}
	loaded, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accounts["local"].Token() != "apt_secretvalue" {
		t.Fatalf("token = %q", loaded.Accounts["local"].Token())
	}
}

func TestRedactSecrets(t *testing.T) {
	got := redactSecrets("failed apt_abc123 and ak_secret")
	if strings.Contains(got, "apt_abc123") || strings.Contains(got, "ak_secret") {
		t.Fatalf("redact = %s", got)
	}
}

func TestCustomLoginOpensServerPageThenAsksForName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/cli/v1/login/start":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD","verification_uri":"http://other.example/cli?device_code=dev","interval":1,"expires_in":60}`))
		case "/system/cli/v1/login/poll":
			_, _ = w.Write([]byte(`{"status":"approved","access_token":"apt_customtoken","console_url":"http://localhost:4000"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	loginKind = "custom"
	loginServer = srv.URL
	flagAccount = ""
	var prompts []string
	loginRead = func(prompt string) (string, error) {
		prompts = append(prompts, prompt)
		if strings.Contains(prompt, "Account name") {
			return "lab", nil
		}
		return "", fmt.Errorf("custom login asked for %s", prompt)
	}
	var opened string
	loginOpen = func(page string) error {
		opened = page
		return nil
	}
	t.Cleanup(func() {
		loginKind = ""
		loginServer = ""
		flagAccount = ""
		loginRead = readSecretLine
		loginOpen = openBrowser
	})
	if err := runLogin(nil, nil); err != nil {
		t.Fatal(err)
	}
	if opened != srv.URL+"/cli?device_code=dev&user_code=ABCD" {
		t.Fatalf("opened %s", opened)
	}
	if len(prompts) != 1 || !strings.Contains(prompts[0], "Account name") {
		t.Fatalf("prompts = %v", prompts)
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts["lab"].Token() != "apt_customtoken" || cfg.Accounts["lab"].Kind != "custom" {
		t.Fatalf("account = %+v token=%s", cfg.Accounts["lab"], cfg.Accounts["lab"].Token())
	}
	if cfg.Accounts["lab"].ServerURL != srv.URL || cfg.DefaultAccount != "lab" {
		t.Fatalf("server=%s default=%s", cfg.Accounts["lab"].ServerURL, cfg.DefaultAccount)
	}
}

func TestLocalLoginUsesKnownEngine(t *testing.T) {
	var seenHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/cli/v1/login/start":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD","verification_uri":"http://verify","interval":1,"expires_in":60}`))
		case "/system/cli/v1/login/poll":
			_, _ = w.Write([]byte(`{"status":"approved","access_token":"apt_localtoken","console_url":"http://localhost:5055"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	previous := localLoginServer
	localLoginServer = srv.URL
	loginKind = "local"
	loginServer = "http://should-not-be-used"
	asked := false
	loginRead = func(prompt string) (string, error) {
		asked = true
		return "", fmt.Errorf("local login asked for %s", prompt)
	}
	var opened string
	loginOpen = func(page string) error {
		opened = page
		return nil
	}
	t.Cleanup(func() {
		localLoginServer = previous
		loginKind, loginServer = "", ""
		loginRead = readSecretLine
		loginOpen = openBrowser
	})
	if err := runLogin(nil, nil); err != nil {
		t.Fatal(err)
	}
	if asked {
		t.Fatal("local login asked for input")
	}
	if !strings.Contains(opened, "http://localhost:4055/cli?") || !strings.Contains(opened, "device_code=dev") || !strings.Contains(opened, "user_code=ABCD") {
		t.Fatalf("opened %s", opened)
	}
	if !strings.Contains(seenHost, strings.TrimPrefix(srv.URL, "http://")) && seenHost == "" {
		t.Fatal("login did not call the known local server")
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts["local"].ServerURL != srv.URL || cfg.Accounts["local"].Token() != "apt_localtoken" {
		t.Fatalf("account = %+v token=%s", cfg.Accounts["local"], cfg.Accounts["local"].Token())
	}
}

func TestLoginAndMCPConnectKeepSeparateTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/mcp/v1/login/start":
			_, _ = w.Write([]byte(`{"device_code":"mcpdev","user_code":"MCP1","verification_uri":"http://verify/mcp","interval":1,"expires_in":60}`))
		case "/system/mcp/v1/login/poll":
			_, _ = w.Write([]byte(`{"status":"approved","access_token":"apt_mcptoken","console_url":"http://localhost:4055","project_grant_mode":"selected","project_ids":["p1"],"tenant_grant_mode":"selected","tenant_ids":{"p1":["tenant-1"]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	if err := saveNamedAccount("lab", srv.URL, "apt_clitoken", "local", ""); err != nil {
		t.Fatal(err)
	}
	previous := localLoginServer
	localLoginServer = srv.URL
	loginKind = "local"
	var opened string
	loginOpen = func(page string) error {
		opened = page
		return nil
	}
	installMCPRuntime = func(bool) error { return nil }
	t.Cleanup(func() {
		localLoginServer = previous
		loginKind = ""
		loginOpen = openBrowser
		installMCPRuntime = defaultInstallMCPRuntime
	})
	if err := runMCPConnect(nil, []string{"lab"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(opened, "http://localhost:4055/mcp?") || !strings.Contains(opened, "device_code=mcpdev") || !strings.Contains(opened, "user_code=MCP1") {
		t.Fatalf("opened %s", opened)
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	acc := cfg.Accounts["lab"]
	if acc.Token() != "apt_clitoken" {
		t.Fatalf("cli token = %q", acc.Token())
	}
	if acc.MCPAccessToken != "apt_mcptoken" {
		t.Fatalf("mcp token = %q", acc.MCPAccessToken)
	}
	if acc.MCPProjectID != "p1" || acc.MCPTenantID != "tenant-1" {
		t.Fatalf("scope project=%s tenant=%s", acc.MCPProjectID, acc.MCPTenantID)
	}
	configRaw, err := os.ReadFile(filepath.Join(dir, ".apito", "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(configRaw), "apt_clitoken") || strings.Contains(string(configRaw), "apt_mcptoken") {
		t.Fatalf("config retained a token:\n%s", configRaw)
	}
	credRaw, err := os.ReadFile(filepath.Join(dir, ".apito", "credentials.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(credRaw), "apt_clitoken") || !strings.Contains(string(credRaw), "apt_mcptoken") || !strings.Contains(string(credRaw), "mcp_accounts") {
		t.Fatalf("credentials = %s", credRaw)
	}
	body, err := buildMCPConfig("lab", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "apt_") {
		t.Fatalf("config leaked a token: %s", body)
	}
	if err := deleteAccountToken("lab"); err != nil {
		t.Fatal(err)
	}
	if tok, err := getAccountToken("lab"); err != nil || tok != "" {
		t.Fatalf("cli token after delete = %q %v", tok, err)
	}
	if tok, err := getMCPAccountToken("lab"); err != nil || tok != "" {
		t.Fatalf("mcp token after delete = %q %v", tok, err)
	}
}

func TestMCPConnectCustomDoesNotAskForToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/mcp/v1/login/start":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"CODE","verification_uri":"http://console.example/mcp?device_code=dev&user_code=CODE","interval":1,"expires_in":30}`))
		case "/system/mcp/v1/login/poll":
			_, _ = w.Write([]byte(`{"status":"approved","access_token":"apt_custommcp","project_grant_mode":"all_admin_projects","tenant_grant_mode":"none"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	loginKind = "custom"
	loginServer = srv.URL
	var opened string
	loginOpen = func(page string) error {
		opened = page
		return nil
	}
	loginRead = func(prompt string) (string, error) {
		return "", fmt.Errorf("custom MCP asked for %s", prompt)
	}
	installMCPRuntime = func(bool) error { return nil }
	t.Cleanup(func() {
		loginKind, loginServer = "", ""
		loginOpen = openBrowser
		loginRead = readSecretLine
		installMCPRuntime = defaultInstallMCPRuntime
	})
	if err := runMCPConnect(nil, []string{"enterprise"}); err != nil {
		t.Fatal(err)
	}
	if opened != "http://console.example/mcp?device_code=dev&user_code=CODE" {
		t.Fatalf("opened %s", opened)
	}
	cfg, err := loadCLIConfig()
	if err != nil {
		t.Fatal(err)
	}
	acc := cfg.Accounts["enterprise"]
	if acc.Token() != "" || acc.MCPAccessToken != "apt_custommcp" {
		t.Fatalf("cli=%q mcp=%q", acc.Token(), acc.MCPAccessToken)
	}
	if acc.MCPProjectID != "" {
		t.Fatalf("all-admin grant stored a default project %s", acc.MCPProjectID)
	}
}

func TestTenantCommandFailsClosedWithoutCapability(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/system/health":
			w.WriteHeader(http.StatusOK)
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"project_access":true,"tenant_catalog":false,"client_lifecycle":false,"managed_backups":false,"restore_actions":false}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	acc := AccountConfig{ServerURL: srv.URL, Kind: "local"}
	acc.SetToken("apt_labtoken")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = "lab"
	flagProject = "p1"
	t.Cleanup(func() { flagAccount = ""; flagProject = "" })
	_, err := requireProjectClient(capTenantCatalog, "apito tenant")
	if err == nil || !strings.Contains(err.Error(), "requires an Apito Cloud account with the required capability. Run apito login") {
		t.Fatalf("err = %v", err)
	}
}

func TestProjectListAndKeyMetadata(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/system/health":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"project_access":true,"project_tokens":true},"console_url":"http://console.local"}`))
		case strings.HasSuffix(r.URL.Path, "/system/graphql"):
			var body struct {
				Query string `json:"query"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if strings.Contains(body.Query, "listProjects") {
				_, _ = w.Write([]byte(`{"data":{"listProjects":[{"id":"p1","name":"One"}]}}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":{"currentProject":{"tokens":[{"token_id":"tok1","name":"ci","role":"admin","token_prefix":"ak_abcd","token":"ak_fullsecret"}]}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	acc := AccountConfig{ServerURL: srv.URL}
	acc.SetToken("apt_labtoken")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = "lab"
	flagProject = "p1"
	t.Cleanup(func() { flagAccount = ""; flagProject = "" })
	ctx, err := resolveCommandContext()
	if err != nil {
		t.Fatal(err)
	}
	projects, err := ctx.syncClient().ListProjects()
	if err != nil || len(projects) != 1 || projects[0].ID != "p1" {
		t.Fatalf("projects=%v err=%v", projects, err)
	}
	keys, err := listProjectKeys()
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(keys)
	if strings.Contains(string(raw), "ak_fullsecret") {
		t.Fatalf("key list included the secret: %s", raw)
	}
}

func TestMCPConfigHasNoToken(t *testing.T) {
	body, err := buildMCPConfig("lab", "p1", false)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "apt_") || strings.Contains(text, "ak_") {
		t.Fatalf("config leaked a token: %s", text)
	}
	if !strings.Contains(text, `"mcp","serve"`) && !strings.Contains(text, "mcp") {
		t.Fatalf("config = %s", text)
	}
}

func TestGenRefusesSecretsCollisionsAndCleansOwnedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"dependencies":{"react":"19.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(`{"data":{"__schema":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "generated")
	genDir = dir
	genLanguage = "typescript"
	genFramework = "none"
	genSchema = schema
	genOut = out
	genDraft = false
	genForce = false
	genClean = false
	flagDryRun = false
	runTSCodegen = func(schemaPath, stage, models, artifacts string) error {
		if artifacts != "sdk" {
			t.Fatalf("artifacts = %s", artifacts)
		}
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\napt_secret\n"), 0o644)
	}
	t.Cleanup(func() {
		genDir, genLanguage, genFramework, genSchema, genOut = "", "", "", "", ""
		genClean, genForce = false, false
		runTSCodegen = defaultTSCodegen
		promoteGenDir = defaultPromoteGenDir
	})
	if err := runGen(nil, nil); err == nil || !strings.Contains(err.Error(), "credential") {
		t.Fatalf("secret err = %v", err)
	}

	runTSCodegen = func(schemaPath, stage, models, artifacts string) error {
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\n"), 0o644)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "types.ts"), []byte("hand written\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runGen(nil, nil); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("collision err = %v", err)
	}
	if err := os.Remove(filepath.Join(out, "types.ts")); err != nil {
		t.Fatal(err)
	}
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	runTSCodegen = func(schemaPath, stage, models, artifacts string) error {
		if err := os.WriteFile(filepath.Join(stage, "sdk.ts"), []byte("export {}\n"), 0o644); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\n"), 0o644)
	}
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	genClean = true
	runTSCodegen = func(schemaPath, stage, models, artifacts string) error {
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\n"), 0o644)
	}
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "sdk.ts")); !os.IsNotExist(err) {
		t.Fatalf("clean left sdk.ts: %v", err)
	}
	manifest, err := os.ReadFile(filepath.Join(out, ".apito-gen-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "apt_") || strings.Contains(string(manifest), "ak_") {
		t.Fatalf("manifest leaked a credential: %s", manifest)
	}
}

func TestGenDraftRequiresStagingCapability(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/system/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/system/cli/v1/capabilities" {
			_, _ = w.Write([]byte(`{"capabilities":{"published_introspection":true,"schema_staging":false}}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	acc := AccountConfig{ServerURL: srv.URL}
	acc.SetToken("apt_labtoken")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	genDir = dir
	genLanguage = "go"
	genDraft = true
	flagAccount = "lab"
	t.Cleanup(func() {
		genDir, genLanguage = "", ""
		genDraft = false
		flagAccount = ""
	})
	err := runGen(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "draft schema") {
		t.Fatalf("err = %v", err)
	}
}

func TestPromoteFailureLeavesDestinationUntouched(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(`{"data":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "generated")
	genDir = dir
	genLanguage = "typescript"
	genFramework = "none"
	genSchema = schema
	genOut = out
	runTSCodegen = func(schemaPath, stage, models, artifacts string) error {
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\n"), 0o644)
	}
	promoteGenDir = func(stage, dest string, files []string) error {
		return os.ErrClosed
	}
	t.Cleanup(func() {
		genDir, genLanguage, genFramework, genSchema, genOut = "", "", "", "", ""
		runTSCodegen = defaultTSCodegen
		promoteGenDir = defaultPromoteGenDir
	})
	if err := runGen(nil, nil); err == nil {
		t.Fatal("expected promote failure")
	}
	if _, err := os.Stat(filepath.Join(out, "types.ts")); !os.IsNotExist(err) {
		t.Fatalf("destination was written: %v", err)
	}
}

func TestNoAccountSaysLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	flagAccount = ""
	_, err := resolveCommandContext()
	if err == nil || !strings.Contains(err.Error(), "Run apito login") {
		t.Fatalf("err = %v", err)
	}
}

func TestCredentialStoreFileIsDefault(t *testing.T) {
	t.Setenv("APITO_CREDENTIAL_STORE", "")
	if credentialStoreKind() != "file" {
		t.Fatalf("store = %s", credentialStoreKind())
	}
	t.Setenv("APITO_CREDENTIAL_STORE", "vault")
	if credentialStoreKind() != "file" {
		t.Fatalf("unknown store = %s", credentialStoreKind())
	}
}

func TestBrowserLoginExpiredAndDenied(t *testing.T) {
	status := "expired"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/cli/v1/login/start":
			_, _ = w.Write([]byte(`{"device_code":"dev","user_code":"ABCD","verification_uri":"http://verify","interval":1,"expires_in":60}`))
		case "/system/cli/v1/login/poll":
			fmt.Fprintf(w, `{"status":%q}`, status)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	loginKind = "cloud"
	previousCloud := cloudLoginServer
	cloudLoginServer = srv.URL
	flagAccount = "cloud"
	loginOpen = func(string) error { return nil }
	t.Cleanup(func() {
		loginKind, loginServer, flagAccount = "", "", ""
		cloudLoginServer = previousCloud
		loginOpen = openBrowser
	})
	err := runLogin(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "browser login expired") {
		t.Fatalf("expired err = %v", err)
	}
	status = "denied"
	err = runLogin(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "browser login denied") {
		t.Fatalf("denied err = %v", err)
	}
}

func TestSelfHostedCloudCommandsFailClosed(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"project_access":true,"project_tokens":true,"published_introspection":true,"mcp":true,"tenant_catalog":false,"client_lifecycle":false,"managed_backups":false,"restore_actions":false},"console_url":"http://console.local"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	acc := AccountConfig{ServerURL: srv.URL, Kind: "local"}
	acc.SetToken("apt_selfhost")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = "lab"
	flagProject = "project-1"
	clientEmail = "a@example.com"
	clientSnapshot = "snap-1"
	t.Cleanup(func() {
		flagAccount, flagProject = "", ""
		clientEmail, clientSnapshot = "", ""
	})
	cases := []struct {
		name string
		run  func() error
	}{
		{"tenant", func() error { return tenantListCmd.RunE(tenantListCmd, nil) }},
		{"client", func() error { return clientInviteCmd.RunE(clientInviteCmd, nil) }},
		{"backup", func() error { return backupListCmd.RunE(backupListCmd, nil) }},
		{"restore", func() error { return restorePreviewCmd.RunE(restorePreviewCmd, nil) }},
	}
	for _, tc := range cases {
		err := tc.run()
		if err == nil || !strings.Contains(err.Error(), "requires an Apito Cloud account with the required capability. Run apito login") {
			t.Fatalf("%s err = %v", tc.name, err)
		}
	}
	for _, path := range paths {
		if strings.Contains(path, "/tenants") || strings.Contains(path, "/client/") || strings.Contains(path, "/backups") {
			t.Fatalf("fail-closed command called %s", path)
		}
	}
}

func TestCloudBackupListAndRestoreDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"managed_backups":true,"restore_actions":true,"project_access":true}}`))
		case "/system/cli/v1/backups":
			_, _ = w.Write([]byte(`{"snapshots":[{"id":"snap-1","status":"ready","created_at":"2026-10-12T00:00:00Z","target_type":"project"}]}`))
		case "/system/cli/v1/backups/preview":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"restore is not enabled"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	acc := AccountConfig{ServerURL: srv.URL, Kind: "cloud"}
	acc.SetToken("apt_cloud")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "cloud", Accounts: map[string]AccountConfig{"cloud": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = "cloud"
	flagProject = "project-1"
	clientSnapshot = "snap-1"
	t.Cleanup(func() {
		flagAccount, flagProject, clientSnapshot = "", "", ""
	})
	if err := backupListCmd.RunE(backupListCmd, nil); err != nil {
		t.Fatal(err)
	}
	err := restorePreviewCmd.RunE(restorePreviewCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "restore is not enabled") {
		t.Fatalf("restore err = %v", err)
	}
}

func TestProjectKeyWritesEnvFileOnce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"project_tokens":true,"project_access":true}}`))
		case "/system/graphql":
			_, _ = w.Write([]byte(`{"data":{"generateProjectToken":{"token":"ak_once"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	acc := AccountConfig{ServerURL: srv.URL}
	acc.SetToken("apt_lab")
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(home, "app.env")
	flagAccount = "lab"
	flagProject = "project-1"
	keyName = "ci"
	keyRole = "admin"
	keyDuration = "1h"
	keyEnvFile = envFile
	flagYes = false
	t.Cleanup(func() {
		flagAccount, flagProject, keyName, keyRole, keyDuration, keyEnvFile = "", "", "", "", "", ""
		flagYes = false
	})
	if err := keyProjectCreateCmd.RunE(keyProjectCreateCmd, nil); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("without yes: %v", err)
	}
	flagYes = true
	if err := keyProjectCreateCmd.RunE(keyProjectCreateCmd, nil); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "APITO_PROJECT_KEY=ak_once\n" {
		t.Fatalf("env file = %q", raw)
	}
	info, err := os.Stat(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
}

func TestGenArtifactModeAndGoModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pnpm-lock.yaml"), []byte("lockfileVersion: 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/app\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	schema := filepath.Join(dir, "schema.json")
	if err := os.WriteFile(schema, []byte(`{"data":{"__schema":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if detectPackageManager(dir) != "pnpm" {
		t.Fatalf("manager = %s", detectPackageManager(dir))
	}
	var gotArtifacts, gotModule string
	genDir = dir
	genLanguage = "typescript"
	genFramework = "none"
	genSchema = schema
	genOut = filepath.Join(dir, "generated")
	runTSCodegen = func(_, stage, _, artifacts string) error {
		gotArtifacts = artifacts
		return os.WriteFile(filepath.Join(stage, "types.ts"), []byte("export type ID = string\n"), 0o644)
	}
	t.Cleanup(func() {
		genDir, genLanguage, genFramework, genSchema, genOut = "", "", "", "", ""
		runTSCodegen = defaultTSCodegen
		runGoCodegen = defaultGoCodegen
	})
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotArtifacts != "sdk" {
		t.Fatalf("artifacts = %s", gotArtifacts)
	}
	genLanguage = "go"
	genFramework = ""
	genOut = filepath.Join(dir, "go-generated")
	runGoCodegen = func(_, stage, _, module string, _ bool) error {
		gotModule = module
		return os.WriteFile(filepath.Join(stage, "doc.go"), []byte("package generated\n"), 0o644)
	}
	if err := runGen(nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotModule != "example.com/app" {
		t.Fatalf("module = %s", gotModule)
	}
}

func TestMCPServeUsesCapabilityEdition(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("APITO_MCP_COMMAND", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"mcp":true,"tenant_catalog":true,"project_access":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("APITO_CREDENTIAL_STORE", "file")
	acc := AccountConfig{
		ServerURL:           srv.URL,
		MCPProjectGrantMode: "selected",
		MCPProjectIDs:       []string{"project-1", "project-2"},
		MCPTenantGrantMode:  "none",
	}
	acc.SetToken("apt_cli_not_for_mcp")
	acc.MCPAccessToken = "apt_mcp"
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "lab", Accounts: map[string]AccountConfig{"lab": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = "lab"
	flagProject = "project-1"
	mcpTransport = "node"
	called := false
	mcpRun = func(string, []string, []string) error {
		called = true
		return nil
	}
	t.Cleanup(func() {
		flagAccount, flagProject = "", ""
		mcpTransport = ""
		mcpCommand = defaultMCPCommand
		mcpRun = defaultMCPRun
	})
	err := mcpServeCmd.RunE(mcpServeCmd, nil)
	if err == nil || !strings.Contains(err.Error(), "does not start a separate MCP server") || called {
		t.Fatalf("serve err=%v called=%v", err, called)
	}
	flagProject = ""
	if err := mcpServeCmd.RunE(mcpServeCmd, nil); err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("missing project: %v", err)
	}
	flagProject = "other"
	if err := mcpServeCmd.RunE(mcpServeCmd, nil); err == nil || !strings.Contains(err.Error(), "outside this MCP grant") {
		t.Fatalf("outside grant: %v", err)
	}
}
