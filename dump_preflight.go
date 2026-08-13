package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

type dumpPreflightResponse struct {
	Profile        string `json:"profile"`
	Engine         string `json:"engine"`
	PortableDump   bool   `json:"portable_dump"`
	SchemaHash     string `json:"schema_hash"`
	SystemEngine   string `json:"system_engine"`
	ProjectName    string `json:"project_name"`
	ProjectID      string `json:"project_id"`
	TargetType     string `json:"target_type"`
	TargetID       string `json:"target_id"`
	WritesSystemDB string `json:"writes_to_system_db"`
	DoesNotWrite   string `json:"does_not_write"`
}

func dumpBlockingPreflight(from, to SyncProject, src, dst dumpPreflightResponse, destTenant *SyncTenant) error {
	fromKey := projectSyncProfileKey(from)
	toKey := projectSyncProfileKey(to)
	if fromKey != toKey {
		return fmt.Errorf("project type mismatch: source is %q but destination is %q", fromKey, toKey)
	}
	if src.Profile != "" && dst.Profile != "" && src.Profile != dst.Profile {
		return fmt.Errorf("profile mismatch: source is %q but destination is %q", src.Profile, dst.Profile)
	}
	if !dst.PortableDump {
		eng := strings.TrimSpace(dst.Engine)
		if eng == "" {
			eng = "unknown"
		}
		return fmt.Errorf("destination engine %q does not support portable dump (sqlite and postgresql only)", eng)
	}
	if src.SchemaHash != "" && dst.SchemaHash != "" && src.SchemaHash != dst.SchemaHash {
		return fmt.Errorf("schema hash mismatch — run apito sync --type schema first\n  source:      %s\n  destination: %s", src.SchemaHash, dst.SchemaHash)
	}
	if fromKey == "saas-per-tenant" && (destTenant == nil || strings.TrimSpace(destTenant.ID) == "") {
		return fmt.Errorf("destination tenant must be resolved by domain before dump")
	}
	return nil
}

func printDumpSystemDBMatrix(toName, toURL string, dst dumpPreflightResponse, destTenant *SyncTenant, profile string) {
	physical := "project DB"
	if profile == "saas-per-tenant" {
		physical = "tenant DB"
		if destTenant != nil {
			physical = fmt.Sprintf("tenant DB id=%s domain=%s", destTenant.ID, destTenant.Domain)
		}
	}
	skip := dst.DoesNotWrite
	if strings.TrimSpace(skip) == "" {
		skip = "users, tokens, other projects, project_schemas, the system file itself"
	}
	writes := dst.WritesSystemDB
	if strings.TrimSpace(writes) == "" {
		writes = "none"
	}
	fmt.Println()
	fmt.Println("Destination instance:  " + toName + "  " + toURL)
	fmt.Println("Destination system DB: " + nz(dst.SystemEngine, "unknown") + "  (this instance only; not copied)")
	fmt.Println("Writes to system DB:   " + writes)
	fmt.Println("Does NOT write:        " + skip)
	fmt.Println("Physical restore:      " + physical + "  engine=" + nz(dst.Engine, "unknown"))
	fmt.Println()
}

func nz(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func isDumpPush(_, toURL string) bool {
	return !isLocalServerURL(toURL)
}

func isLocalServerURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return true
	}
	if strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
