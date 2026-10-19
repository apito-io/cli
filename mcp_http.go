package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func runMCPStdioBridge(target mcpHTTPTarget, subjectToken string, in io.Reader, out io.Writer, do func(*http.Request) (*http.Response, error)) error {
	if do == nil {
		do = http.DefaultClient.Do
	}
	access, err := exchangeMCPToken(target, subjectToken, do)
	if err != nil {
		return err
	}
	reader := bufio.NewReader(in)
	session := ""
	for {
		body, err := readLSP(reader)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, target.PublicURL, strings.NewReader(string(body)))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+access)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		resp, err := do(req)
		if err != nil {
			return err
		}
		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if id := resp.Header.Get("Mcp-Session-Id"); id != "" {
			session = id
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return fmt.Errorf("MCP endpoint rejected the resource-bound credential (%d)", resp.StatusCode)
		}
		if err := writeLSP(out, respBody); err != nil {
			return err
		}
	}
}

func exchangeMCPToken(target mcpHTTPTarget, subjectToken string, do func(*http.Request) (*http.Response, error)) (string, error) {
	if !strings.HasPrefix(strings.TrimSpace(subjectToken), "apt_") {
		return "", fmt.Errorf("MCP HTTP bridge requires an apt_ grant stored by apito mcp connect")
	}
	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:token-exchange")
	form.Set("subject_token", subjectToken)
	form.Set("subject_token_type", "urn:ietf:params:oauth:token-type:access_token")
	form.Set("resource", target.PublicURL)
	req, err := http.NewRequest(http.MethodPost, target.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MCP token exchange failed (%d)", resp.StatusCode)
	}
	var body struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", err
	}
	if !strings.HasPrefix(body.AccessToken, "mct_") {
		return "", fmt.Errorf("MCP token exchange did not return a resource-bound credential")
	}
	return body.AccessToken, nil
}

func readLSP(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			value := strings.TrimSpace(line[len("content-length:"):])
			var n int
			if _, err := fmt.Sscanf(value, "%d", &n); err == nil {
				length = n
			}
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeLSP(w io.Writer, body []byte) error {
	_, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(body), body)
	return err
}
