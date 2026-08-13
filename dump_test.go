package main

import "testing"

func TestIsLocalServerURL(t *testing.T) {
	locals := []string{
		"http://localhost:5050",
		"http://127.0.0.1:8080",
		"https://studio.localhost",
		"http://[::1]:5050",
	}
	for _, u := range locals {
		if !isLocalServerURL(u) {
			t.Fatalf("expected local: %s", u)
		}
	}
	if isLocalServerURL("https://studio.protiva.app") {
		t.Fatal("studio should not be local")
	}
	if !isDumpPush("http://localhost:5050", "https://studio.protiva.app") {
		t.Fatal("remote dest is a push")
	}
	if isDumpPush("https://studio.protiva.app", "http://localhost:5050") {
		t.Fatal("local dest is a pull")
	}
}

func TestDumpBlockingPreflight(t *testing.T) {
	from := SyncProject{ID: "a", Name: "A", ProjectType: "general"}
	to := SyncProject{ID: "b", Name: "B", ProjectType: "general"}
	src := dumpPreflightResponse{Profile: "general", SchemaHash: "abc", PortableDump: true, Engine: "sqlite"}
	dst := dumpPreflightResponse{Profile: "general", SchemaHash: "abc", PortableDump: true, Engine: "sqlite"}
	if err := dumpBlockingPreflight(from, to, src, dst, nil); err != nil {
		t.Fatal(err)
	}
	dst.SchemaHash = "zzz"
	if err := dumpBlockingPreflight(from, to, src, dst, nil); err == nil {
		t.Fatal("expected schema hash mismatch")
	}
	dst.SchemaHash = "abc"
	dst.PortableDump = false
	if err := dumpBlockingPreflight(from, to, src, dst, nil); err == nil {
		t.Fatal("expected portable dump refusal")
	}
}
