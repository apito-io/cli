package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	keyName     string
	keyRole     string
	keyDuration string
	keyTokenID  string
	keyEnvFile  string
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage project API keys",
}

var keyProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "Project API keys (ak_)",
}

var keyProjectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List project API key metadata",
	RunE: func(cmd *cobra.Command, args []string) error {
		keys, err := listProjectKeys()
		if err != nil {
			return err
		}
		return printJSONOrLines(keys, func() {
			for _, k := range keys {
				fmt.Printf("%s\t%s\t%s\t%s\n", k.TokenID, k.Name, k.Role, k.TokenPrefix)
			}
		})
	},
}

var keyProjectCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a project API key and print the secret once",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(keyName) == "" {
			return fmt.Errorf("--name is required")
		}
		if flagDryRun {
			return printJSONOrLines(map[string]string{"dry_run": "true", "name": keyName, "role": keyRole}, nil)
		}
		ctx, err := requireProjectClient(capProjectTokens, "apito key")
		if err != nil {
			return err
		}
		const query = `mutation CreateKey($name: String!, $role: String!, $duration: String!) {
			generateProjectToken(name: $name, role: $role, duration: $duration) { token }
		}`
		var out struct {
			GenerateProjectToken struct {
				Token string `json:"token"`
			} `json:"generateProjectToken"`
		}
		role := keyRole
		if role == "" {
			role = "admin"
		}
		duration := keyDuration
		if duration == "" {
			duration = "8760h"
		}
		if err := ctx.Client.execute(query, map[string]interface{}{
			"name": keyName, "role": role, "duration": duration,
		}, &out); err != nil {
			return err
		}
		secret := strings.TrimSpace(out.GenerateProjectToken.Token)
		if secret == "" {
			return fmt.Errorf("the server did not return a project key")
		}
		fmt.Fprintln(os.Stderr, "Copy this ak_ project key now. It will not be shown again.")
		fmt.Println(secret)
		if strings.TrimSpace(keyEnvFile) != "" {
			if !flagYes {
				return fmt.Errorf("writing the new key to a file requires --yes")
			}
			line := "APITO_PROJECT_KEY=" + secret + "\n"
			if err := os.WriteFile(keyEnvFile, []byte(line), 0o600); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Wrote the new project key to the env file.")
		}
		return nil
	},
}

var keyProjectRevokeCmd = &cobra.Command{
	Use:   "revoke [token-id]",
	Short: "Revoke a project API key by token id",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 && strings.TrimSpace(keyTokenID) == "" {
			keyTokenID = args[0]
		}
		if strings.TrimSpace(keyTokenID) == "" {
			return fmt.Errorf("pass the token id or --token-id")
		}
		if flagDryRun {
			return printJSONOrLines(map[string]string{"dry_run": "true", "token_id": keyTokenID}, nil)
		}
		ctx, err := requireProjectClient(capProjectTokens, "apito key")
		if err != nil {
			return err
		}
		const query = `mutation RevokeKey($token_id: String!, $duration: String!) {
			deleteProjectToken(token_id: $token_id, duration: $duration) { msg }
		}`
		var out struct {
			DeleteProjectToken struct {
				Msg string `json:"msg"`
			} `json:"deleteProjectToken"`
		}
		if err := ctx.Client.execute(query, map[string]interface{}{
			"token_id": keyTokenID, "duration": "0s",
		}, &out); err != nil {
			return err
		}
		return printJSONOrLines(map[string]string{"token_id": keyTokenID, "message": out.DeleteProjectToken.Msg}, func() {
			fmt.Printf("revoked %s\n", keyTokenID)
		})
	},
}

type projectKeyMeta struct {
	TokenID     string `json:"token_id"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	TokenPrefix string `json:"token_prefix"`
	Expire      string `json:"expire"`
	CreatedAt   string `json:"created_at"`
}

func listProjectKeys() ([]projectKeyMeta, error) {
	ctx, err := requireProjectClient(capProjectTokens, "apito key")
	if err != nil {
		return nil, err
	}
	const query = `query KeyList {
		currentProject {
			tokens { token_id name role token_prefix expire created_at }
		}
	}`
	var out struct {
		CurrentProject struct {
			Tokens []projectKeyMeta `json:"tokens"`
		} `json:"currentProject"`
	}
	if err := ctx.Client.execute(query, nil, &out); err != nil {
		return nil, err
	}
	return out.CurrentProject.Tokens, nil
}

func requireProjectClient(capName, feature string) (commandContext, error) {
	ctx, err := resolveCommandContext()
	if err != nil {
		return commandContext{}, err
	}
	if err := requireCapability(ctx.Caps, capName, feature); err != nil {
		return commandContext{}, err
	}
	if strings.TrimSpace(ctx.ProjectID) == "" {
		return commandContext{}, fmt.Errorf("a project is required. Pass --project")
	}
	return ctx, nil
}

func initKeyFlags() {
	keyProjectCreateCmd.Flags().StringVar(&keyName, "name", "", "Key name")
	keyProjectCreateCmd.Flags().StringVar(&keyRole, "role", "admin", "Project role")
	keyProjectCreateCmd.Flags().StringVar(&keyDuration, "duration", "8760h", "Key duration")
	keyProjectCreateCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the action without calling the server")
	keyProjectCreateCmd.Flags().StringVar(&keyEnvFile, "env-file", "", "Write the new ak_ secret to this file after --yes")
	keyProjectCreateCmd.Flags().BoolVar(&flagYes, "yes", false, "Confirm writing the new secret to --env-file")
	keyProjectRevokeCmd.Flags().StringVar(&keyTokenID, "token-id", "", "Project key id to revoke")
	keyProjectRevokeCmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the action without calling the server")
}
