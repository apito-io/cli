package main

import (
	"fmt"
	"strings"

	survey "github.com/AlecAivazis/survey/v2"
)

func runContentSync(fromClient, toClient *SyncGraphQLClient, fromProject, toProject SyncProject, dryRun, autoYes bool) error {
	print_step("Loading models for content sync...")
	models, err := fromClient.ProjectModelsInfo("")
	if err != nil {
		return fmt.Errorf("source models: %w", err)
	}

	userModels := filterUserModels(models)
	if len(userModels) == 0 {
		print_warning("No user models found to sync.")
		return nil
	}

	var selectedNames []string
	if autoYes {
		for _, m := range userModels {
			selectedNames = append(selectedNames, m.Name)
		}
	} else {
		options := make([]string, len(userModels))
		for i, m := range userModels {
			options[i] = m.Name
		}
		prompt := &survey.MultiSelect{
			Message:  "Select models whose content to sync",
			Options:  options,
			PageSize: 12,
		}
		if err := survey.AskOne(prompt, &selectedNames); err != nil {
			return fmt.Errorf("model selection cancelled: %w", err)
		}
	}

	if len(selectedNames) == 0 {
		print_warning("No models selected for content sync.")
		return nil
	}

	selectedSet := make(map[string]struct{}, len(selectedNames))
	modelByName := make(map[string]SyncModel, len(userModels))
	for _, m := range userModels {
		modelByName[strings.ToLower(m.Name)] = m
	}
	for _, name := range selectedNames {
		selectedSet[strings.ToLower(name)] = struct{}{}
	}

	fromTenantID, toTenantID, err := resolveTenantSelection(fromClient, toClient, fromProject, toProject)
	if err != nil {
		return err
	}
	if fromTenantID != "" {
		fromClient = fromClient.WithTenant(fromTenantID)
	}
	if toTenantID != "" {
		toClient = toClient.WithTenant(toTenantID)
	}

	totalRows := 0
	for _, name := range selectedNames {
		count, err := countModelRows(fromClient, name)
		if err != nil {
			return fmt.Errorf("count %q: %w", name, err)
		}
		totalRows += count
	}

	print_status(fmt.Sprintf("Selected %d model(s), ~%d row(s) to copy.", len(selectedNames), totalRows))

	if dryRun {
		print_success("Dry run: content sync plan shown above; no writes performed.")
		return nil
	}

	if !autoYes {
		confirm := false
		if err := survey.AskOne(&survey.Confirm{
			Message: fmt.Sprintf("Copy ~%d row(s) to destination?", totalRows),
			Default: false,
		}, &confirm); err != nil {
			return err
		}
		if !confirm {
			print_warning("Content sync cancelled.")
			return nil
		}
	}

	copied := 0
	for _, name := range selectedNames {
		n, err := copyModelContent(fromClient, toClient, name)
		if err != nil {
			return fmt.Errorf("copy model %q: %w", name, err)
		}
		copied += n
		print_success(fmt.Sprintf("Copied %d row(s) for model %q", n, name))
	}

	for _, name := range selectedNames {
		m, ok := modelByName[strings.ToLower(name)]
		if !ok {
			continue
		}
		if err := syncModelRelations(fromClient, toClient, m); err != nil {
			return fmt.Errorf("relations for %q: %w", name, err)
		}
	}

	print_success(fmt.Sprintf("Content sync complete: %d row(s) copied.", copied))
	return nil
}

func filterUserModels(models []SyncModel) []SyncModel {
	out := make([]SyncModel, 0, len(models))
	for _, m := range models {
		lower := strings.ToLower(m.Name)
		if lower == "user" || lower == "users" || lower == "tenant" || lower == "tenants" {
			continue
		}
		out = append(out, m)
	}
	return out
}

func resolveTenantSelection(fromClient, toClient *SyncGraphQLClient, fromProject, toProject SyncProject) (string, string, error) {
	if projectSyncProfileKey(fromProject) == "general" {
		return "", "", nil
	}

	fromTenants, err := fromClient.GetTenants()
	if err != nil {
		return "", "", fmt.Errorf("list source tenants: %w", err)
	}
	toTenants, err := toClient.GetTenants()
	if err != nil {
		return "", "", fmt.Errorf("list destination tenants: %w", err)
	}
	if len(fromTenants) == 0 {
		return "", "", fmt.Errorf("no tenants on source project")
	}
	if len(toTenants) == 0 {
		return "", "", fmt.Errorf("no tenants on destination project")
	}

	fromID, err := pickTenant("Select source tenant", fromTenants)
	if err != nil {
		return "", "", err
	}
	toID, err := pickTenant("Select destination tenant", toTenants)
	if err != nil {
		return "", "", err
	}
	return fromID, toID, nil
}

func pickTenant(label string, tenants []SyncTenant) (string, error) {
	options := make([]string, len(tenants))
	byLabel := make(map[string]string, len(tenants))
	for i, t := range tenants {
		display := t.Name
		if display == "" {
			display = t.Domain
		}
		if display == "" {
			display = t.ID
		}
		options[i] = fmt.Sprintf("%s (%s)", display, t.ID)
		byLabel[options[i]] = t.ID
	}
	var picked string
	if err := survey.AskOne(&survey.Select{Message: label, Options: options}, &picked); err != nil {
		return "", err
	}
	return byLabel[picked], nil
}

func countModelRows(client *SyncGraphQLClient, modelName string) (int, error) {
	res, err := client.GetModelData(modelName, 1, 1)
	if err != nil {
		return 0, err
	}
	return res.Count, nil
}

func copyModelContent(fromClient, toClient *SyncGraphQLClient, modelName string) (int, error) {
	const pageSize = 50
	page := 1
	total := 0

	for {
		res, err := fromClient.GetModelData(modelName, page, pageSize)
		if err != nil {
			return total, err
		}
		if len(res.Results) == 0 {
			break
		}
		for _, doc := range res.Results {
			docID := doc.ID
			if docID == "" {
				docID = doc.Key
			}
			payload := doc.Data
			if payload == nil {
				payload = map[string]interface{}{}
			}
			if err := toClient.UpsertModelData(modelName, docID, payload, nil); err != nil {
				return total, fmt.Errorf("upsert %s/%s: %w", modelName, docID, err)
			}
			total++
		}
		if len(res.Results) < pageSize {
			break
		}
		page++
	}
	return total, nil
}

func syncModelRelations(fromClient, toClient *SyncGraphQLClient, model SyncModel) error {
	forward := make([]SyncConnection, 0)
	for _, c := range model.Connections {
		if isForwardConnection(c) {
			forward = append(forward, c)
		}
	}
	if len(forward) == 0 {
		return nil
	}

	const pageSize = 50
	page := 1
	for {
		res, err := fromClient.GetModelData(model.Name, page, pageSize)
		if err != nil {
			return err
		}
		if len(res.Results) == 0 {
			break
		}

		for _, doc := range res.Results {
			docID := doc.ID
			if docID == "" {
				docID = doc.Key
			}
			connectMap := make(map[string]interface{})

			for _, conn := range forward {
				related, err := collectRelatedIDs(fromClient, model.Name, docID, conn)
				if err != nil {
					return err
				}
				if len(related) == 0 {
					continue
				}
				key := conn.KnownAs
				if key == "" {
					key = conn.Model
				}
				if conn.Relation == "has_one" {
					connectMap[key] = related[0]
				} else {
					connectMap[key] = related
				}
			}

			if len(connectMap) == 0 {
				continue
			}
			if err := toClient.UpsertModelData(model.Name, docID, nil, connectMap); err != nil {
				return fmt.Errorf("connect %s/%s: %w", model.Name, docID, err)
			}
		}

		if len(res.Results) < pageSize {
			break
		}
		page++
	}
	return nil
}

func collectRelatedIDs(client *SyncGraphQLClient, modelName, docID string, conn SyncConnection) ([]string, error) {
	const pageSize = 100
	page := 1
	ids := make([]string, 0)

	for {
		res, err := client.GetRelatedModelData(modelName, docID, conn, page, pageSize)
		if err != nil {
			return nil, err
		}
		for _, r := range res.Results {
			id := r.ID
			if id == "" {
				id = r.Key
			}
			if id != "" {
				ids = append(ids, id)
			}
		}
		if len(res.Results) < pageSize {
			break
		}
		page++
	}
	return ids, nil
}
