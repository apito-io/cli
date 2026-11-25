package main

import (
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
