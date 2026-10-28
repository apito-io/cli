package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
)

// syncWriteCaps is what the destination token must already have.
// CLI login stays read-only. This check does not grant anything.
func syncWriteCaps(kind string, deploy bool) []string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "schema":
		return []string{"schema.write"}
	case "functions":
		caps := []string{"functions.write"}
		if deploy {
			caps = append(caps, "functions.deploy")
		}
		return caps
	case "content":
		return []string{"data.write"}
	default:
		return nil
	}
}

func missingSyncCaps(have []string, kind string, deploy bool) []string {
	granted := map[string]struct{}{}
	for _, cap := range have {
		cap = strings.TrimSpace(cap)
		if cap != "" {
			granted[cap] = struct{}{}
		}
	}
	var missing []string
	for _, cap := range syncWriteCaps(kind, deploy) {
		if _, ok := granted[cap]; !ok {
			missing = append(missing, cap)
		}
	}
	return missing
}

func syncTokenMessage(account, preset, name string, missing []string, kind string) string {
	label := strings.TrimSpace(name)
	if label == "" {
		label = "this token"
	}
	preset = strings.TrimSpace(preset)
	if preset == "" {
		preset = "unknown"
	}
	need := strings.Join(missing, ", ")
	advice := "Choose Custom and include " + need + "."
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "schema", "functions":
		advice = "Choose the CLI sync preset, or Custom and include " + need + "."
	}
	return fmt.Sprintf(
		"Account %q token %q (preset %s) cannot run %s sync. Missing %s. CLI login does not include that. Run apito login for this account. %s Sync will not write until the new token is saved.",
		account, label, preset, kind, need, advice,
	)
}

type syncTokenView struct {
	Name         string   `json:"name"`
	Preset       string   `json:"preset"`
	Capabilities []string `json:"capabilities"`
}

func readSyncToken(client *SystemClient) (syncTokenView, error) {
	var body struct {
		Token syncTokenView `json:"token"`
	}
	if client == nil {
		return syncTokenView{}, fmt.Errorf("no client")
	}
	if err := client.getJSON("/system/access-tokens/me", &body); err != nil {
		return syncTokenView{}, err
	}
	return body.Token, nil
}

func syncStdinInteractive() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ensureSyncWriteToken checks the destination account token.
// A dry run warns and continues. A real sync stops, and on a terminal offers one re-login.
func ensureSyncWriteToken(client *SyncGraphQLClient, account string, cfg AccountConfig, kind string, deploy, dryRun, autoYes bool) (*SyncGraphQLClient, error) {
	if client == nil || len(syncWriteCaps(kind, deploy)) == 0 {
		return client, nil
	}
	token, err := readSyncToken(&client.SystemClient)
	if err != nil {
		return client, fmt.Errorf("could not read token permissions for %q: %w\nRun apito login and choose CLI sync, or a custom token with the capability this sync needs", account, err)
	}
	missing := missingSyncCaps(token.Capabilities, kind, deploy)
	if len(missing) == 0 {
		return client, nil
	}
	msg := syncTokenMessage(account, token.Preset, token.Name, missing, kind)
	print_warning(msg)
	if dryRun {
		print_warning("Dry run can still read. A real sync stops until this account has the new token.")
		return client, nil
	}
	if autoYes || !syncStdinInteractive() {
		return client, fmt.Errorf("sync blocked for %q: missing %s", account, strings.Join(missing, ", "))
	}
	prompt := promptui.Select{
		Label: "New access token required",
		Items: []string{
			"Log in again for " + account,
			"Stop",
		},
	}
	index, _, err := prompt.Run()
	if err != nil || index != 0 {
		return client, fmt.Errorf("sync blocked for %q: missing %s", account, strings.Join(missing, ", "))
	}
	kindHint := strings.TrimSpace(cfg.Kind)
	if kindHint == "" {
		kindHint = "custom"
	}
	if err := loginBrowser(account, cfg.ServerURL, kindHint); err != nil {
		return client, err
	}
	fresh, err := getAccountConfig(account)
	if err != nil {
		return client, err
	}
	next := newSyncGraphQLClient(fresh.ServerURL, fresh.Token(), int(client.timeout.Seconds()))
	token, err = readSyncToken(&next.SystemClient)
	if err != nil {
		return next, fmt.Errorf("could not read the new token for %q: %w", account, err)
	}
	missing = missingSyncCaps(token.Capabilities, kind, deploy)
	if len(missing) == 0 {
		print_status("New token includes the capability this sync needs.")
		return next, nil
	}
	print_warning(syncTokenMessage(account, token.Preset, token.Name, missing, kind))
	return next, fmt.Errorf("sync blocked for %q: missing %s", account, strings.Join(missing, ", "))
}
