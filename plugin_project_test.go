package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpsertProjectPluginSendsEnable(t *testing.T) {
	var got struct {
		Query     string                 `json:"query"`
		Variables map[string]interface{} `json:"variables"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/system/graphql" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if r.Header.Get("X-Apito-Project-Id") != "suchok_wb8ed" {
			t.Fatalf("project header %q", r.Header.Get("X-Apito-Project-Id"))
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &got)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"upsertPlugin":{"id":"hc-eps-payment-plugin","enable":true}}}`))
	}))
	defer srv.Close()

	client := newSystemClient(srv.URL, "apt_test", 5).WithProject("suchok_wb8ed")
	row, err := upsertProjectPlugin(client, "hc-eps-payment-plugin", true)
	if err != nil {
		t.Fatal(err)
	}
	if !row.Enable || row.ID != "hc-eps-payment-plugin" {
		t.Fatalf("row %+v", row)
	}
	if !strings.Contains(got.Query, "upsertPlugin") {
		t.Fatalf("query %s", got.Query)
	}
	if got.Variables["enable"] != true {
		t.Fatalf("variables %+v", got.Variables)
	}
}

func TestUpdateEnginePluginPostsCatalogPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/system/plugin/hc-eps-payment-plugin/update" {
			t.Fatalf("%s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"updated hc-eps-payment-plugin@0.1.0"}`))
	}))
	defer srv.Close()

	message, err := updateEnginePlugin(newSystemClient(srv.URL, "apt_test", 5), "hc-eps-payment-plugin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(message, "updated") {
		t.Fatalf("message %s", message)
	}
}
