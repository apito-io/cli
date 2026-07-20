package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type FunctionChangeKind string

const (
	FunctionAdd       FunctionChangeKind = "add"
	FunctionUpdate    FunctionChangeKind = "update"
	FunctionDeploy    FunctionChangeKind = "deploy"
	FunctionUnchanged FunctionChangeKind = "unchanged"
)

// FunctionChange describes how a single source function differs from the destination.
type FunctionChange struct {
	Kind       FunctionChangeKind
	Name       string
	Source     SyncFunction
	DestExists bool
	Summary    string
}

// functionComparable is the transferable subset used for equality (volatile
// engine-owned fields like active_revision_id / secret / timestamps are excluded).
type functionComparable struct {
	Description       string                     `json:"description"`
	Source            string                     `json:"source"`
	TriggerType       string                     `json:"trigger_type"`
	Language          string                     `json:"language"`
	GraphQLSchemaType string                     `json:"graphql_schema_type"`
	Capabilities      []string                   `json:"capabilities"`
	RuntimeConfig     *SyncFunctionRuntimeConfig `json:"runtime_config"`
	Request           *SyncFunctionModelRef      `json:"request"`
	Response          *SyncFunctionModelRef      `json:"response"`
	EnvVars           []SyncFunctionEnvVar       `json:"env_vars"`
}

func normalizeFunction(fn SyncFunction) functionComparable {
	caps := append([]string(nil), fn.Capabilities...)
	sort.Strings(caps)
	env := append([]SyncFunctionEnvVar(nil), fn.EnvVars...)
	sort.Slice(env, func(i, j int) bool { return env[i].Key < env[j].Key })
	return functionComparable{
		Description:       fn.Description,
		Source:            fn.Source,
		TriggerType:       fn.TriggerType,
		Language:          fn.Language,
		GraphQLSchemaType: fn.GraphQLSchemaType,
		Capabilities:      caps,
		RuntimeConfig:     fn.RuntimeConfig,
		Request:           fn.Request,
		Response:          fn.Response,
		EnvVars:           env,
	}
}

// functionEqualForSync reports whether two functions are equivalent for sync purposes.
func functionEqualForSync(a, b SyncFunction) bool {
	ab, _ := json.Marshal(normalizeFunction(a))
	bb, _ := json.Marshal(normalizeFunction(b))
	return string(ab) == string(bb)
}

// functionSourceHash returns the SHA-256 hex of draft source (same as DeployDraft artifact_hash).
func functionSourceHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return hex.EncodeToString(sum[:])
}

// needsLiveDeploy reports whether a published source requires dest to (re)publish
// so dest active_revision_hash matches sha256(source draft).
func needsLiveDeploy(src, dst SyncFunction) bool {
	if strings.TrimSpace(src.ActiveRevisionID) == "" {
		return false
	}
	want := functionSourceHash(src.Source)
	got := strings.TrimSpace(dst.ActiveRevisionHash)
	return got == "" || got != want
}

func capabilityDrift(src, dst SyncFunction) string {
	srcSet := map[string]struct{}{}
	for _, c := range src.Capabilities {
		srcSet[c] = struct{}{}
	}
	dstSet := map[string]struct{}{}
	for _, c := range dst.Capabilities {
		dstSet[c] = struct{}{}
	}
	var added, removed []string
	for c := range srcSet {
		if _, ok := dstSet[c]; !ok {
			added = append(added, c)
		}
	}
	for c := range dstSet {
		if _, ok := srcSet[c]; !ok {
			removed = append(removed, c)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	parts := make([]string, 0, 2)
	if len(added) > 0 {
		parts = append(parts, "+"+strings.Join(added, ",+"))
	}
	if len(removed) > 0 {
		parts = append(parts, "-"+strings.Join(removed, ",-"))
	}
	return strings.Join(parts, " ")
}

func deployChangeSummary(src, dst SyncFunction) string {
	reason := "live revision missing"
	if strings.TrimSpace(dst.ActiveRevisionHash) != "" {
		reason = "hash drift"
	}
	return fmt.Sprintf("Deploy function %q (%s)", src.Name, reason)
}

// computeFunctionDiff builds the per-function change list (add / update / deploy) from source vs dest.
func computeFunctionDiff(source, dest []SyncFunction) []FunctionChange {
	dstMap := make(map[string]SyncFunction, len(dest))
	for _, f := range dest {
		dstMap[strings.ToLower(f.Name)] = f
	}

	names := make([]string, 0, len(source))
	srcMap := make(map[string]SyncFunction, len(source))
	for _, f := range source {
		key := strings.ToLower(f.Name)
		srcMap[key] = f
		names = append(names, key)
	}
	sort.Strings(names)

	changes := make([]FunctionChange, 0, len(names))
	for _, key := range names {
		src := srcMap[key]
		dst, exists := dstMap[key]
		if !exists {
			changes = append(changes, FunctionChange{
				Kind:       FunctionAdd,
				Name:       src.Name,
				Source:     src,
				DestExists: false,
				Summary:    fmt.Sprintf("Add function %q", src.Name),
			})
			continue
		}
		if functionEqualForSync(src, dst) {
			if needsLiveDeploy(src, dst) {
				changes = append(changes, FunctionChange{
					Kind:       FunctionDeploy,
					Name:       src.Name,
					Source:     src,
					DestExists: true,
					Summary:    deployChangeSummary(src, dst),
				})
				continue
			}
			changes = append(changes, FunctionChange{
				Kind:       FunctionUnchanged,
				Name:       src.Name,
				Source:     src,
				DestExists: true,
				Summary:    fmt.Sprintf("Function %q unchanged", src.Name),
			})
			continue
		}
		summary := fmt.Sprintf("Update function %q", src.Name)
		if drift := capabilityDrift(src, dst); drift != "" {
			summary += fmt.Sprintf(" (capabilities: %s)", drift)
		}
		if needsLiveDeploy(src, dst) || strings.TrimSpace(src.ActiveRevisionID) != "" {
			summary += " (will deploy revision)"
		}
		changes = append(changes, FunctionChange{
			Kind:       FunctionUpdate,
			Name:       src.Name,
			Source:     src,
			DestExists: true,
			Summary:    summary,
		})
	}
	return changes
}

// actionableFunctionChanges filters out unchanged entries.
func actionableFunctionChanges(changes []FunctionChange) []FunctionChange {
	out := make([]FunctionChange, 0, len(changes))
	for _, ch := range changes {
		if ch.Kind == FunctionUnchanged {
			continue
		}
		out = append(out, ch)
	}
	return out
}
