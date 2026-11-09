package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var pluginUpsertCmd = &cobra.Command{
	Use:          "upsert <plugin-id>",
	Short:        "Attach a plugin to a project and set its enable flag",
	Long:         `Calls upsertPlugin on the account engine for --project. --enable defaults to true. This does not upload a binary and does not change plugin env.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		enable, _ := cmd.Flags().GetBool("enable")
		return runProjectPluginEnable(cmd, args[0], enable)
	},
}

var pluginActivateCmd = &cobra.Command{
	Use:          "activate <plugin-id>",
	Short:        "Activate a plugin on a project",
	Long:         `Same as upsert --enable. The project id comes from --project, then the account's saved project.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProjectPluginEnable(cmd, args[0], true)
	},
}

var pluginDeactivateCmd = &cobra.Command{
	Use:          "deactivate <plugin-id>",
	Short:        "Deactivate a plugin on a project",
	Long:         `Calls upsertPlugin with enable false. The engine copy stays installed.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runProjectPluginEnable(cmd, args[0], false)
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:          "update <plugin-id>",
	Short:        "Install the approved catalog version on the engine",
	Long:         `Calls POST /system/plugin/:id/update. The engine downloads the signed catalog release. This does not change a project's enable flag.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runEnginePluginUpdate(cmd, args[0])
	},
}

func runProjectPluginEnable(cmd *cobra.Command, pluginID string, enable bool) error {
	pluginID = strings.TrimSpace(pluginID)
	if err := requireHCPluginID(pluginID); err != nil {
		return err
	}
	client, projectID, err := pluginProjectClient(cmd, []string{"plugins.write"})
	if err != nil {
		return err
	}
	row, err := upsertProjectPlugin(client, pluginID, enable)
	if err != nil {
		return err
	}
	state := "deactivated"
	if row.Enable {
		state = "activated"
	}
	print_success(fmt.Sprintf("%s %s on %s", pluginID, state, projectID))
	return nil
}

func runEnginePluginUpdate(cmd *cobra.Command, pluginID string) error {
	pluginID = strings.TrimSpace(pluginID)
	if err := requireHCPluginID(pluginID); err != nil {
		return err
	}
	name, account, err := pluginAccountFromCmd(cmd)
	if err != nil {
		return err
	}
	account, err = ensurePluginCaps(name, account, []string{"plugins.deploy"})
	if err != nil {
		return err
	}
	client := newSystemClient(account.ServerURL, account.Token(), pluginTimeout(cmd))
	message, err := updateEnginePlugin(client, pluginID)
	if err != nil {
		return err
	}
	print_success(message)
	return nil
}

type projectPluginRow struct {
	ID     string `json:"id"`
	Enable bool   `json:"enable"`
}

func upsertProjectPlugin(client *SystemClient, pluginID string, enable bool) (projectPluginRow, error) {
	if client == nil || strings.TrimSpace(client.projectID) == "" {
		return projectPluginRow{}, fmt.Errorf("project is required. Pass --project")
	}
	const query = `mutation($id: String!, $enable: Boolean) { upsertPlugin(id: $id, enable: $enable) { id enable } }`
	var out struct {
		UpsertPlugin projectPluginRow `json:"upsertPlugin"`
	}
	err := client.execute(query, map[string]interface{}{
		"id":     pluginID,
		"enable": enable,
	}, &out)
	if err != nil {
		return projectPluginRow{}, err
	}
	if out.UpsertPlugin.ID == "" {
		return projectPluginRow{}, fmt.Errorf("upsertPlugin returned no plugin")
	}
	return out.UpsertPlugin, nil
}

func updateEnginePlugin(client *SystemClient, pluginID string) (string, error) {
	var out struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	path := "/system/plugin/" + pluginID + "/update"
	if err := client.postJSON(path, map[string]interface{}{}, &out); err != nil {
		return "", err
	}
	if !out.Success {
		if out.Message == "" {
			out.Message = "plugin update failed"
		}
		return "", fmt.Errorf("%s", out.Message)
	}
	if out.Message == "" {
		out.Message = "updated " + pluginID
	}
	return out.Message, nil
}

func pluginProjectClient(cmd *cobra.Command, caps []string) (*SystemClient, string, error) {
	name, account, err := pluginAccountFromCmd(cmd)
	if err != nil {
		return nil, "", err
	}
	projectID, _ := cmd.Flags().GetString("project")
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		projectID = strings.TrimSpace(account.ProjectID)
	}
	if projectID == "" {
		return nil, "", fmt.Errorf("project is required. Pass --project")
	}
	account, err = ensurePluginCaps(name, account, caps)
	if err != nil {
		return nil, "", err
	}
	client := newSystemClient(account.ServerURL, account.Token(), pluginTimeout(cmd)).WithProject(projectID)
	return client, projectID, nil
}

func pluginTimeout(cmd *cobra.Command) int {
	_ = cmd
	cfg, err := loadCLIConfig()
	if err != nil || cfg.Timeout <= 0 {
		return 30
	}
	return cfg.Timeout
}

func init() {
	pluginUpsertCmd.Flags().Bool("enable", true, "Project enable flag")
}
