package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	functionMetaFile   = "meta.json"
	functionSourceFile = "source.ts"
)

// readLocalFunctions scans a local functions directory. Layout:
//
//	{dir}/{functionName}/meta.json   (metadata; source omitted)
//	{dir}/{functionName}/source.ts   (draft Deno/TS source)
func readLocalFunctions(dir string) ([]SyncFunction, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read functions dir %q: %w", dir, err)
	}

	var fns []SyncFunction
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fnDir := filepath.Join(dir, entry.Name())
		metaPath := filepath.Join(fnDir, functionMetaFile)
		metaBytes, err := os.ReadFile(metaPath)
		if err != nil {
			if os.IsNotExist(err) {
				// Skip directories without a meta.json (not a function folder).
				continue
			}
			return nil, fmt.Errorf("read %q: %w", metaPath, err)
		}
		var fn SyncFunction
		if err := json.Unmarshal(metaBytes, &fn); err != nil {
			return nil, fmt.Errorf("parse %q: %w", metaPath, err)
		}
		if strings.TrimSpace(fn.Name) == "" {
			return nil, fmt.Errorf("%q: meta.json is missing \"name\"", metaPath)
		}
		if !strings.EqualFold(fn.Name, entry.Name()) {
			return nil, fmt.Errorf(
				"%q: folder name %q does not match meta.json name %q",
				fnDir, entry.Name(), fn.Name,
			)
		}

		sourcePath := filepath.Join(fnDir, functionSourceFile)
		sourceBytes, err := os.ReadFile(sourcePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read %q: %w", sourcePath, err)
			}
			// source.ts optional; fall back to any inline source in meta.json.
		} else {
			fn.Source = string(sourceBytes)
		}
		if strings.TrimSpace(fn.Source) == "" {
			return nil, fmt.Errorf("%q: no source (expected %s or inline source in meta.json)", fnDir, functionSourceFile)
		}
		fns = append(fns, fn)
	}

	sort.Slice(fns, func(i, j int) bool { return fns[i].Name < fns[j].Name })
	return fns, nil
}

// writeLocalFunction persists one function as {dir}/{name}/{meta.json,source.ts}.
// The REST secret is stripped unless includeSecrets is true.
func writeLocalFunction(dir string, fn SyncFunction, includeSecrets bool) error {
	if strings.TrimSpace(fn.Name) == "" {
		return fmt.Errorf("function has no name")
	}
	fnDir := filepath.Join(dir, fn.Name)
	if err := os.MkdirAll(fnDir, 0o755); err != nil {
		return fmt.Errorf("create %q: %w", fnDir, err)
	}

	source := fn.Source

	meta := fn
	meta.Source = "" // source lives in source.ts, not meta.json
	if !includeSecrets {
		meta.RestAPISecretURLKey = ""
	}
	// Volatile / engine-owned IDs are not portable. Keep active_revision_hash when
	// present as an informational content fingerprint for re-diff after export.
	meta.ActiveRevisionID = ""
	meta.CreatedAt = ""
	meta.UpdatedAt = ""
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta for %q: %w", fn.Name, err)
	}
	metaBytes = append(metaBytes, '\n')
	if err := os.WriteFile(filepath.Join(fnDir, functionMetaFile), metaBytes, 0o644); err != nil {
		return fmt.Errorf("write meta for %q: %w", fn.Name, err)
	}
	if err := os.WriteFile(filepath.Join(fnDir, functionSourceFile), []byte(source), 0o644); err != nil {
		return fmt.Errorf("write source for %q: %w", fn.Name, err)
	}
	return nil
}
