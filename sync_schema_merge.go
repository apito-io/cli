package main

import (
	"encoding/json"
	"sort"
	"strings"
)

type SchemaVersioningStatus struct {
	Enabled           bool   `json:"enabled"`
	ActiveVersion     int    `json:"active_version"`
	HasDraft          bool   `json:"has_draft"`
	ChangesetID       string `json:"changeset_id"`
	ChangesetStatus   string `json:"changeset_status"`
	PendingOperations int    `json:"pending_operations"`
}

type destSchemaContext struct {
	Live      []SyncModel
	Draft     []SyncModel
	Effective []SyncModel
	Status    *SchemaVersioningStatus
}

func loadDestinationSchema(client *SyncGraphQLClient) (destSchemaContext, error) {
	live, err := client.ProjectModelsInfo("")
	if err != nil {
		return destSchemaContext{}, err
	}

	ctx := destSchemaContext{Live: live}
	status, err := client.SchemaVersioningStatus()
	if err == nil && status != nil {
		ctx.Status = status
	}

	draft, err := client.SchemaPreviewModels("draft")
	if err == nil && len(draft) > 0 {
		ctx.Draft = draft
	}

	ctx.Effective = mergeEffectiveSchema(live, draft)
	return ctx, nil
}

func parseModelsFromSchemaJSON(raw string) ([]SyncModel, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil, nil
	}
	var payload struct {
		Models []SyncModel `json:"models"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return payload.Models, nil
}

func mergeEffectiveSchema(live, draft []SyncModel) []SyncModel {
	byName := make(map[string]SyncModel, len(live)+len(draft))
	order := make([]string, 0, len(live)+len(draft))

	addKey := func(name string) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			return
		}
		for _, existing := range order {
			if existing == key {
				return
			}
		}
		order = append(order, key)
	}

	for _, m := range live {
		key := strings.ToLower(m.Name)
		byName[key] = m
		addKey(m.Name)
	}
	for _, dm := range draft {
		key := strings.ToLower(dm.Name)
		if existing, ok := byName[key]; ok {
			byName[key] = mergeSyncModel(existing, dm)
		} else {
			byName[key] = dm
		}
		addKey(dm.Name)
	}

	sort.Strings(order)
	out := make([]SyncModel, 0, len(order))
	for _, key := range order {
		if m, ok := byName[key]; ok {
			out = append(out, m)
		}
	}
	return out
}

func mergeSyncModel(live, draft SyncModel) SyncModel {
	out := live
	if draft.SinglePage {
		out.SinglePage = draft.SinglePage
	}
	out.Fields = mergeSyncFields(live.Fields, draft.Fields)
	out.Connections = mergeModelConnections(out.Name, live.Connections, draft.Connections)
	return out
}

func mergeModelConnections(modelName string, live, draft []SyncConnection) []SyncConnection {
	byKey := make(map[string]SyncConnection, len(live)+len(draft))
	order := make([]string, 0, len(live)+len(draft))

	addConn := func(c SyncConnection) {
		key := connectionKey(modelName, c)
		if _, seen := byKey[key]; !seen {
			order = append(order, key)
		}
		byKey[key] = c
	}

	for _, c := range live {
		addConn(c)
	}
	for _, c := range draft {
		addConn(c)
	}

	out := make([]SyncConnection, 0, len(order))
	for _, key := range order {
		if c, ok := byKey[key]; ok {
			out = append(out, c)
		}
	}
	return out
}

func mergeSyncFields(live, draft []SyncField) []SyncField {
	byID := make(map[string]SyncField, len(live)+len(draft))
	order := make([]string, 0, len(live)+len(draft))

	addKey := func(id string) {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" {
			return
		}
		for _, existing := range order {
			if existing == key {
				return
			}
		}
		order = append(order, key)
	}

	for _, f := range live {
		byID[strings.ToLower(f.Identifier)] = f
		addKey(f.Identifier)
	}
	for _, f := range draft {
		key := strings.ToLower(f.Identifier)
		byID[key] = f
		addKey(f.Identifier)
	}

	out := make([]SyncField, 0, len(order))
	for _, key := range order {
		if f, ok := byID[key]; ok {
			out = append(out, f)
		}
	}
	return out
}

func effectiveModelNames(models []SyncModel) map[string]struct{} {
	out := make(map[string]struct{}, len(models))
	for _, m := range models {
		if name := strings.TrimSpace(m.Name); name != "" {
			out[strings.ToLower(name)] = struct{}{}
		}
	}
	return out
}
