package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type genManifest struct {
	SchemaDigest string            `json:"schema_digest"`
	ProjectID    string            `json:"project_id,omitempty"`
	TenantID     string            `json:"tenant_id,omitempty"`
	Language     string            `json:"language"`
	Framework    string            `json:"framework"`
	Generator    string            `json:"generator"`
	Models       []string          `json:"models,omitempty"`
	PackageMgr   string            `json:"package_manager,omitempty"`
	Files        map[string]string `json:"files"`
}

func manifestPath(out string) string {
	return filepath.Join(out, ".apito-gen-manifest.json")
}

func loadManifest(out string) (*genManifest, error) {
	raw, err := os.ReadFile(manifestPath(out))
	if err != nil {
		if os.IsNotExist(err) {
			return &genManifest{Files: map[string]string{}}, nil
		}
		return nil, err
	}
	var m genManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Files == nil {
		m.Files = map[string]string{}
	}
	return &m, nil
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileHasSecret(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	text := string(raw)
	return strings.Contains(text, "apt_") || strings.Contains(text, "ak_"), nil
}

func detectPackageManager(dir string) string {
	switch {
	case fileExists(filepath.Join(dir, "pnpm-lock.yaml")):
		return "pnpm"
	case fileExists(filepath.Join(dir, "yarn.lock")):
		return "yarn"
	case fileExists(filepath.Join(dir, "bun.lock")) || fileExists(filepath.Join(dir, "bun.lockb")):
		return "bun"
	case fileExists(filepath.Join(dir, "package-lock.json")):
		return "npm"
	default:
		return ""
	}
}

func detectLanguage(dir, explicit string) (language, framework string, err error) {
	explicit = strings.ToLower(strings.TrimSpace(explicit))
	if explicit == "javascript" || explicit == "js" {
		return "", "", fmt.Errorf("the TypeScript generator emits TypeScript and TSX. Pure JavaScript output is not available")
	}
	if explicit == "python" || explicit == "py" {
		return "", "", fmt.Errorf("Python generation waits for an official Apito SDK")
	}
	if explicit == "dart" || explicit == "flutter" {
		return "dart", "none", nil
	}
	if explicit == "typescript" || explicit == "ts" || explicit == "go" {
		language = explicit
		if language == "ts" {
			language = "typescript"
		}
	}
	hasNode := fileExists(filepath.Join(dir, "package.json"))
	hasGo := fileExists(filepath.Join(dir, "go.mod"))
	if language == "" {
		switch {
		case hasNode && hasGo:
			return "", "", fmt.Errorf("both package.json and go.mod are present. Pass --language typescript or --language go")
		case hasGo:
			language = "go"
		case hasNode:
			language = "typescript"
		default:
			return "", "", fmt.Errorf("could not detect a project. Pass --language typescript or --language go")
		}
	}
	framework = "none"
	if language == "typescript" {
		switch {
		case fileExists(filepath.Join(dir, "astro.config.mjs")) || fileExists(filepath.Join(dir, "astro.config.ts")):
			framework = "astro"
		case fileExists(filepath.Join(dir, "package.json")) && packageJSONMentions(dir, "react"):
			framework = "react"
		}
	}
	return language, framework, nil
}

func packageJSONMentions(dir, needle string) bool {
	raw, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	return strings.Contains(string(raw), `"`+needle+`"`)
}

func checkGeneratedCollisions(out string, staged []string, previous *genManifest, force bool) error {
	for _, rel := range staged {
		dest := filepath.Join(out, rel)
		if _, err := os.Stat(dest); err != nil {
			continue
		}
		ownedHash, owned := previous.Files[rel]
		if !owned && !force {
			return fmt.Errorf("refusing to overwrite %s because it is not owned by a previous apito gen manifest. Pass --force to replace it", rel)
		}
		if owned && !force {
			current, err := hashFile(dest)
			if err != nil {
				return err
			}
			if current != ownedHash {
				return fmt.Errorf("refusing to overwrite modified generated file %s. Pass --force", rel)
			}
		}
	}
	return nil
}

func cleanManagedFiles(out string, previous *genManifest, next map[string]string) error {
	for rel, hash := range previous.Files {
		if _, keep := next[rel]; keep {
			continue
		}
		path := filepath.Join(out, rel)
		current, err := hashFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if current != hash {
			return fmt.Errorf("refusing to delete %s because it no longer matches the manifest", rel)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
