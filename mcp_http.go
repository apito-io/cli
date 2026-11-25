package main

import (
	"fmt"
	"net/url"
	"strings"
)

type mcpHTTPTarget struct {
	PublicURL string
	TokenURL  string
}

func resolveMCPHTTPTarget(account AccountConfig) (mcpHTTPTarget, error) {
	server := strings.TrimSuffix(strings.TrimSpace(account.ServerURL), "/")
	kind := inferMCPDeployment(server, account.Kind)
	switch kind {
	case "local":
		return mcpHTTPTarget{
			PublicURL: "http://localhost:" + studioPublishPort + "/mcp",
			TokenURL:  "http://localhost:" + studioPublishPort + "/oauth/token",
		}, nil
	case "cloud":
		return mcpHTTPTarget{
			PublicURL: "https://api.apito.io/mcp",
			TokenURL:  "https://api.apito.io/oauth/token",
		}, nil
	default:
		if server == "" {
			return mcpHTTPTarget{}, fmt.Errorf("account has no server URL")
		}
		parsed, err := url.Parse(server)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return mcpHTTPTarget{}, fmt.Errorf("server URL is invalid")
		}
		origin := parsed.Scheme + "://" + parsed.Host
		return mcpHTTPTarget{PublicURL: origin + "/mcp", TokenURL: origin + "/oauth/token"}, nil
	}
}

func inferMCPDeployment(server, kindHint string) string {
	parsed, err := url.Parse(server)
	host := ""
	if err == nil {
		host = strings.ToLower(parsed.Hostname())
	}
	switch host {
	case "mcp.apito.io", "api.apito.io":
		return "cloud"
	case "localhost", "127.0.0.1", "::1":
		return "local"
	}
	switch strings.ToLower(strings.TrimSpace(kindHint)) {
	case "local", "oss":
		return "local"
	case "cloud":
		return "cloud"
	default:
		return "custom"
	}
}
