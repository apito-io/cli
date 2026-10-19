package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SystemClient is the shared HTTP/GraphQL client for every CLI command.
// It speaks the public engine protocol and does not embed private host code.
type SystemClient struct {
	endpoint  string
	token     string
	projectID string
	tenantID  string
	timeout   time.Duration
}

func newSystemClient(serverURL, token string, timeoutSec int) *SystemClient {
	base := strings.TrimSuffix(strings.TrimSpace(serverURL), "/")
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &SystemClient{
		endpoint: base + "/system/graphql",
		token:    strings.TrimSpace(token),
		timeout:  time.Duration(timeoutSec) * time.Second,
	}
}

func (c *SystemClient) baseURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimSuffix(c.endpoint, "/system/graphql")
}

func (c *SystemClient) graphqlEndpoint() string {
	if c == nil {
		return ""
	}
	return c.endpoint
}

func (c *SystemClient) WithProject(projectID string) *SystemClient {
	if c == nil {
		return nil
	}
	clone := *c
	clone.projectID = strings.TrimSpace(projectID)
	return &clone
}

func (c *SystemClient) WithTenant(tenantID string) *SystemClient {
	if c == nil {
		return nil
	}
	clone := *c
	clone.tenantID = strings.TrimSpace(tenantID)
	return &clone
}

func (c *SystemClient) execute(query string, variables map[string]interface{}, dest interface{}) error {
	body, err := json.Marshal(graphqlRequest{Query: query, Variables: variables})
	if err != nil {
		return err
	}
	raw, _, err := c.do(http.MethodPost, c.endpoint, body)
	if err != nil {
		return err
	}
	var gqlResp graphqlResponse
	if err := json.Unmarshal(raw, &gqlResp); err != nil {
		return err
	}
	if len(gqlResp.Errors) > 0 {
		msgs := make([]string, 0, len(gqlResp.Errors))
		for _, e := range gqlResp.Errors {
			msgs = append(msgs, e.Message)
		}
		return fmt.Errorf("graphql errors: %s", redactSecrets(strings.Join(msgs, "; ")))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(gqlResp.Data, dest)
}

func (c *SystemClient) do(method, url string, body []byte) ([]byte, int, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		return nil, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.setAuthHeaders(req)
	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return raw, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, redactSecrets(string(raw)))
	}
	return raw, resp.StatusCode, nil
}

func (c *SystemClient) getJSON(path string, dest interface{}) error {
	raw, _, err := c.do(http.MethodGet, c.baseURL()+path, nil)
	if err != nil {
		return err
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

func (c *SystemClient) postJSON(path string, payload, dest interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	raw, _, err := c.do(http.MethodPost, c.baseURL()+path, body)
	if err != nil {
		return err
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}

// setAuthHeaders sends the unified apt_ access token as a bearer credential.
// X-Use-Cookies: false tells the engine this is a headless API call.
func (c *SystemClient) setAuthHeaders(req *http.Request) {
	if c == nil || req == nil {
		return
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set(headerUseCookies, "false")
	if c.projectID != "" {
		req.Header.Set(headerApitoProjectID, c.projectID)
	}
	if c.tenantID != "" {
		req.Header.Set(headerApitoTenantID, c.tenantID)
	}
}
