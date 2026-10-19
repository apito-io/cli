package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolveMCPHTTPTarget(t *testing.T) {
	local, err := resolveMCPHTTPTarget(AccountConfig{ServerURL: "http://localhost:5055", Kind: "cloud"})
	if err != nil || local.PublicURL != "http://localhost:4055/mcp" || !strings.HasSuffix(local.TokenURL, ":4055/oauth/token") {
		t.Fatalf("local %+v %v", local, err)
	}
	cloud, err := resolveMCPHTTPTarget(AccountConfig{ServerURL: "https://api.apito.io", Kind: "local"})
	if err != nil || cloud.PublicURL != "https://api.apito.io/mcp" || cloud.TokenURL != "https://api.apito.io/oauth/token" {
		t.Fatalf("cloud %+v %v", cloud, err)
	}
	custom, err := resolveMCPHTTPTarget(AccountConfig{ServerURL: "https://enterprise.example.com", Kind: "custom"})
	if err != nil || custom.PublicURL != "https://enterprise.example.com/mcp" {
		t.Fatalf("custom %+v %v", custom, err)
	}
}

func TestHTTPConfigIsTokenFree(t *testing.T) {
	t.Setenv("APITO_MCP_RUNTIME", "http")
	body, err := buildMCPConfig("local", "p1", false)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "apt_") || strings.Contains(text, "ak_") || strings.Contains(text, "command") {
		t.Fatalf("config = %s", text)
	}
	if !strings.Contains(text, `"type": "http"`) || !strings.Contains(text, "https://api.apito.io/mcp") {
		t.Fatalf("config = %s", text)
	}
}

func TestStdioBridgeExchangesAndForwards(t *testing.T) {
	var sawSubject, sawBearer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			sawSubject = r.Form.Get("subject_token")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"mct_bridge","token_type":"Bearer"}`))
		case "/mcp":
			sawBearer = r.Header.Get("Authorization")
			w.Header().Set("Mcp-Session-Id", "sess-1")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"apito-mcp"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	in := strings.NewReader(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload))
	var out strings.Builder
	err := runMCPStdioBridge(mcpHTTPTarget{PublicURL: srv.URL + "/mcp", TokenURL: srv.URL + "/oauth/token"}, "apt_grant", in, &out, srv.Client().Do)
	if err != nil {
		t.Fatal(err)
	}
	if sawSubject != "apt_grant" || sawBearer != "Bearer mct_bridge" {
		t.Fatalf("subject=%s bearer=%s", sawSubject, sawBearer)
	}
	if !strings.Contains(out.String(), "apito-mcp") || strings.Contains(out.String(), "apt_grant") {
		t.Fatalf("stdout = %s", out.String())
	}
}

func TestStdioBridgeRejectsMissingGrant(t *testing.T) {
	err := runMCPStdioBridge(mcpHTTPTarget{}, "", strings.NewReader(""), io.Discard, nil)
	if err == nil || !strings.Contains(err.Error(), "apt_") {
		t.Fatalf("err = %v", err)
	}
}
