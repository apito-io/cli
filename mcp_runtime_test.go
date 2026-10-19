package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPRuntimeInstallVerifiesChecksumAndStaysOffline(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	archiveName := "apito-mcp-v1.8.0.tar.gz"
	body := mcpArchive(t, "server.mjs", "console.error('runtime-a')\n")
	sum := sha256.Sum256(body)
	sumText := hex.EncodeToString(sum[:]) + "  " + archiveName + "\n"
	var archive []byte
	archive = body
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/release.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1.8.0","archive_url":"` + srvURL(r, "/"+archiveName) + `","checksum_url":"` + srvURL(r, "/SHA256SUMS") + `"}`))
		case "/" + archiveName:
			_, _ = w.Write(archive)
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(sumText))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("APITO_MCP_RELEASE_URL", srv.URL+"/release.json")
	if err := ensureMCPRuntime(true); err != nil {
		t.Fatal(err)
	}
	installed, err := installedMCPServer()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(installed)
	if err != nil || !strings.Contains(string(raw), "runtime-a") {
		t.Fatalf("installed %s err=%v", raw, err)
	}
	archive = mcpArchive(t, "server.mjs", "console.error('runtime-b')\n")
	sumText = "0000000000000000000000000000000000000000000000000000000000000000  " + archiveName + "\n"
	if err := ensureMCPRuntime(true); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("bad checksum err = %v", err)
	}
	raw, err = os.ReadFile(installed)
	if err != nil || !strings.Contains(string(raw), "runtime-a") || strings.Contains(string(raw), "runtime-b") {
		t.Fatalf("working runtime was replaced: %s %v", raw, err)
	}
	srv.Close()
	if err := ensureMCPRuntime(true); err != nil {
		t.Fatalf("offline update should keep the verified runtime: %v", err)
	}
	previous := mcpLookPath
	mcpLookPath = func(name string) (string, error) {
		if name == "node" {
			return "/bin/node", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { mcpLookPath = previous })
	bin, args, err := defaultMCPCommand()
	if err != nil {
		t.Fatal(err)
	}
	if bin != "node" || len(args) != 1 || args[0] != installed {
		t.Fatalf("command = %s %v", bin, args)
	}
}

func TestMCPRuntimeRejectsTraversalAndCleartext(t *testing.T) {
	dest := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	payload := []byte("nope")
	if err := tw.WriteHeader(&tar.Header{Name: "../evil.txt", Mode: 0o600, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := extractMCPArchive(bytes.NewReader(buf.Bytes()), dest); err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("traversal err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("archive path escaped the destination")
	}
	if err := validateMCPDownloadURL("http://example.com/apito-mcp.tar.gz"); err == nil {
		t.Fatal("accepted a non-local HTTP download")
	}
	if err := validateMCPDownloadURL("file:///tmp/server.mjs"); err == nil {
		t.Fatal("accepted a file URL")
	}
}

func TestMCPServeUsesSingleProjectDefaultWithoutCLIToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/system/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/system/cli/v1/capabilities":
			_, _ = w.Write([]byte(`{"capabilities":{"mcp":true}}`))
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
		Kind:                "local",
		MCPProjectGrantMode: "selected",
		MCPProjectIDs:       []string{"only"},
		MCPProjectID:        "only",
		MCPTenantGrantMode:  "selected",
		MCPTenantID:         "tenant-1",
		MCPTenantIDs:        map[string][]string{"only": {"tenant-1"}},
	}
	acc.MCPAccessToken = "apt_onlymcp"
	if err := saveCLIConfig(&CLIConfig{DefaultAccount: "local", Accounts: map[string]AccountConfig{"local": acc}}); err != nil {
		t.Fatal(err)
	}
	flagAccount = ""
	flagProject = ""
	flagTenant = ""
	var got string
	mcpCommand = func() (string, []string, error) { return "node", []string{"server.mjs"}, nil }
	mcpRun = func(_ string, args []string, env []string) error {
		got = strings.Join(append(args, env...), "\n")
		return nil
	}
	t.Cleanup(func() {
		flagAccount, flagProject, flagTenant = "", "", ""
		mcpCommand = defaultMCPCommand
		mcpRun = defaultMCPRun
	})
	if err := mcpServeCmd.RunE(mcpServeCmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "APITO_AUTH_TOKEN=apt_onlymcp") || !strings.Contains(got, "APITO_DEFAULT_PROJECT_ID=only") || !strings.Contains(got, "TENANT_ID=tenant-1") {
		t.Fatalf("serve env = %s", got)
	}
	body, err := buildMCPConfig("local", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "apt_") || !strings.Contains(string(body), "--project") || !strings.Contains(string(body), "only") {
		t.Fatalf("config = %s", body)
	}
}

func mcpArchive(t *testing.T, name, contents string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	payload := []byte(contents)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func srvURL(r *http.Request, path string) string {
	return "http://" + r.Host + path
}
