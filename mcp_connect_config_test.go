package main

import (
	"strings"
	"testing"
)

func TestMCPConnectChoicesListAccountsThenLoginKinds(t *testing.T) {
	cfg := &CLIConfig{
		DefaultAccount: "local",
		Accounts: map[string]AccountConfig{
			"local": {ServerURL: "http://localhost:5055", Kind: "local"},
			"cloud": {ServerURL: "https://api.apito.io", Kind: "cloud"},
		},
	}
	choices := mcpConnectChoices(cfg)
	if len(choices) != 5 {
		t.Fatalf("choices = %+v", choices)
	}
	if choices[0].Kind != "account" || choices[0].Account != "cloud" || !strings.Contains(choices[0].Label, "https://api.apito.io/mcp") {
		t.Fatalf("first = %+v", choices[0])
	}
	if choices[1].Account != "local" || !strings.Contains(choices[1].Label, "http://localhost:4055/mcp") || !strings.Contains(choices[1].Label, "(default)") {
		t.Fatalf("local = %+v", choices[1])
	}
	if choices[2].Kind != "local" || choices[3].Kind != "cloud" || choices[4].Kind != "custom" {
		t.Fatalf("login kinds = %+v", choices[2:])
	}
}

func TestMCPClientConfigIsTheSameHTTPJSON(t *testing.T) {
	body, err := mcpClientConfigJSON("http://localhost:4055/mcp")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if strings.Contains(text, "apt_") || strings.Contains(text, "command") || !strings.Contains(text, `"type": "http"`) || !strings.Contains(text, "http://localhost:4055/mcp") {
		t.Fatalf("config = %s", text)
	}
	cloud, err := mcpPublicURL(AccountConfig{ServerURL: "https://api.apito.io", Kind: "cloud"})
	if err != nil || cloud != "https://api.apito.io/mcp" {
		t.Fatalf("cloud url = %s %v", cloud, err)
	}
}
