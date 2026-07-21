package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestSyncGraphQLClientSetAuthHeaders(t *testing.T) {
	client := newSyncGraphQLClient("https://example.test", "apt_user_token", 30).
		WithProject("project-1").
		WithTenant("tenant-1")
	req, err := http.NewRequest(http.MethodPost, client.graphqlEndpoint(), nil)
	if err != nil {
		t.Fatal(err)
	}

	client.setAuthHeaders(req)

	if got := req.Header.Get("Authorization"); got != "Bearer apt_user_token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get(headerUseCookies); got != "false" {
		t.Fatalf("%s = %q", headerUseCookies, got)
	}
	if got := req.Header.Get(headerApitoProjectID); got != "project-1" {
		t.Fatalf("%s = %q", headerApitoProjectID, got)
	}
	if got := req.Header.Get(headerApitoTenantID); got != "tenant-1" {
		t.Fatalf("%s = %q", headerApitoTenantID, got)
	}
}

func TestRetiredTokenPrefixesRemainRejected(t *testing.T) {
	for _, prefix := range []string{"cli-", "sdk-", "mcp-"} {
		if !IsRetiredTokenPrefix("  " + prefix + "legacy") {
			t.Errorf("%q was not recognized as retired", prefix)
		}
	}
	if IsRetiredTokenPrefix("apt_current") {
		t.Error("apt_ token was incorrectly recognized as retired")
	}
	if !strings.Contains(TokenFormatRetiredMessage, "TOKEN_FORMAT_RETIRED") {
		t.Fatal("retired-token guidance lost the stable error code")
	}
}
