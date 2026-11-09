package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func runPluginStatus(cmd *cobra.Command, id string) error {
	id = strings.TrimSpace(id)
	if id != "" {
		if err := requireHCPluginID(id); err != nil {
			return err
		}
	}
	name, account, err := pluginAccountFromCmd(cmd)
	if err != nil {
		return err
	}
	dirFlag, _ := cmd.Flags().GetString("dir")
	root, err := pluginRootForAccount(name, account, dirFlag)
	if err != nil {
		return err
	}
	local, err := localPluginVersions(root)
	if err != nil {
		return err
	}
	remote, remoteErr := fetchInstalledPlugins(account)
	print_status(fmt.Sprintf("Account: %s", name))
	print_status(fmt.Sprintf("Server: %s", account.ServerURL))
	print_status(fmt.Sprintf("Plugin path: %s", root))
	if remoteErr != nil {
		print_warning("Engine plugin list failed: " + remoteErr.Error())
	}
	rows := buildPluginStatusRows(local, remote, remoteErr == nil, id)
	if len(rows) == 0 {
		print_status("No plugins")
		return nil
	}
	fmt.Print(formatPluginStatusTable(rows))
	return nil
}

type pluginPresence struct {
	Version string
}

func localPluginVersions(root string) (map[string]pluginPresence, error) {
	names, err := localPluginNames(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]pluginPresence{}, nil
		}
		return nil, err
	}
	out := map[string]pluginPresence{}
	for _, name := range names {
		id := name
		version := ""
		if cfg, err := readPluginConfig(filepath.Join(root, name)); err == nil {
			version = cfg.Plugin.Version
			if strings.HasPrefix(cfg.Plugin.ID, "hc-") {
				id = cfg.Plugin.ID
			}
		}
		out[id] = pluginPresence{Version: version}
	}
	return out, nil
}

func fetchInstalledPlugins(account AccountConfig) ([]PluginStatusInfo, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(account.ServerURL, "/")+"/system/plugin", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+account.Token())
	req.Header.Set("X-Use-Cookies", "false")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET /system/plugin failed (%d): %s", resp.StatusCode, extractAPIMessage(body))
	}
	var response PluginListResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if !response.Success {
		msg := response.Message
		if msg == "" {
			msg = "unsuccessful response"
		}
		return nil, fmt.Errorf("GET /system/plugin failed: %s", msg)
	}
	return response.Plugins, nil
}

type pluginStatusRow struct {
	Name      string
	Local     string
	Remote    string
	Installed string
	Activated string
}

func yesVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return "yes"
	}
	return "yes " + version
}

func buildPluginStatusRows(local map[string]pluginPresence, remote []PluginStatusInfo, remoteOK bool, only string) []pluginStatusRow {
	remoteByID := map[string]PluginStatusInfo{}
	if remoteOK {
		for _, plugin := range remote {
			if strings.HasPrefix(plugin.ID, "hc-") {
				remoteByID[plugin.ID] = plugin
			}
		}
	}
	ids := map[string]struct{}{}
	for id := range local {
		ids[id] = struct{}{}
	}
	for id := range remoteByID {
		ids[id] = struct{}{}
	}
	if only != "" {
		ids = map[string]struct{}{only: {}}
	}
	names := make([]string, 0, len(ids))
	for id := range ids {
		names = append(names, id)
	}
	sort.Strings(names)
	rows := make([]pluginStatusRow, 0, len(names))
	for _, id := range names {
		row := pluginStatusRow{Name: id, Local: "no", Remote: "no", Installed: "no", Activated: "no"}
		if item, ok := local[id]; ok {
			row.Local = yesVersion(item.Version)
		}
		if !remoteOK {
			row.Remote = "?"
			row.Installed = "?"
			row.Activated = "?"
		} else if item, ok := remoteByID[id]; ok {
			row.Remote = yesVersion(item.Version)
			row.Installed = "yes"
			if item.Enable {
				row.Activated = "yes"
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func formatPluginStatusTable(rows []pluginStatusRow) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tLOCAL\tREMOTE\tINSTALLED\tACTIVATED")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", row.Name, row.Local, row.Remote, row.Installed, row.Activated)
	}
	w.Flush()
	return b.String()
}

func listLocalPlugins(accountName, root string) error {
	names, err := localPluginNames(root)
	if err != nil {
		if os.IsNotExist(err) {
			print_status(fmt.Sprintf("No plugins in %s", root))
			return nil
		}
		return err
	}
	found := 0
	for _, name := range names {
		found++
		label := name
		detail := ""
		if cfg, err := readPluginConfig(filepath.Join(root, name)); err == nil {
			if strings.HasPrefix(cfg.Plugin.ID, "hc-") {
				label = cfg.Plugin.ID
			}
			if cfg.Plugin.Version != "" || cfg.Plugin.Language != "" {
				detail = fmt.Sprintf(" v%s (%s)", cfg.Plugin.Version, cfg.Plugin.Language)
			}
		}
		print_status("  " + label + detail)
	}
	if found == 0 {
		print_status(fmt.Sprintf("No plugins in account %s (%s)", accountName, root))
	}
	return nil
}

func runPluginBuild(cmd *cobra.Command, name string) error {
	dir, err := resolvePluginWorkdir(name)
	if err != nil {
		return err
	}
	buildPlugin(dir, cmd)
	return nil
}

func resolveBuildDir(root, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name != "" {
		if dir, err := localPluginDir(root, name); err == nil {
			return dir, nil
		}
		return "", fmt.Errorf("plugin %s is not in %s", name, root)
	}
	if _, err := readPluginConfig(root); err == nil {
		return root, nil
	}
	id, err := solePluginID(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id), nil
}

func localPluginNames(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "hc-") {
			continue
		}
		info, err := os.Stat(filepath.Join(root, name))
		if err != nil || !info.IsDir() {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

func solePluginID(root string) (string, error) {
	names, err := localPluginNames(root)
	if err != nil {
		return "", fmt.Errorf("plugin path %s: %w", root, err)
	}
	ids := names
	if len(ids) == 1 {
		return ids[0], nil
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("no plugin in %s", root)
	}
	return "", fmt.Errorf("pass a plugin id; %s has %s", root, strings.Join(ids, ", "))
}

func runPluginAdd(cmd *cobra.Command, spec string) error {
	name, account, err := pluginAccountFromCmd(cmd)
	if err != nil {
		return err
	}
	account, err = ensurePluginCaps(name, account, []string{"plugins.deploy"})
	if err != nil {
		return err
	}
	print_step("Add plugin")
	print_status(fmt.Sprintf("Installing on %s (%s)", name, account.ServerURL))
	id, err := installCatalogPlugin(account, spec)
	if err != nil {
		return err
	}
	if !accountIsLocal(account) {
		return nil
	}
	dirFlag, _ := cmd.Flags().GetString("dir")
	root, err := pluginRootForAccount(name, account, dirFlag)
	if err != nil {
		return err
	}
	dest := filepath.Join(root, id)
	if err := downloadPluginArchive(account, id, dest, false); err != nil {
		print_warning("Engine install succeeded. Local copy failed: " + err.Error())
	} else {
		print_success("Saved local copy: " + dest)
	}
	return nil
}

func runPluginRemove(cmd *cobra.Command, id string) error {
	if err := requireHCPluginID(id); err != nil {
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
	yes, _ := cmd.Flags().GetBool("yes")
	if !yes && !confirmSensitiveOperation("remove", id, name) {
		return fmt.Errorf("remove cancelled")
	}
	status, body, err := performPluginDeleteRequest(id, name)
	if err != nil {
		return err
	}
	if status >= 300 && status != http.StatusNotFound {
		return fmt.Errorf("remove failed (%d): %s", status, extractAPIMessage(body))
	}
	if accountIsLocal(account) {
		dirFlag, _ := cmd.Flags().GetString("dir")
		root, err := pluginRootForAccount(name, account, dirFlag)
		if err != nil {
			return err
		}
		local := filepath.Join(root, id)
		if _, statErr := os.Stat(local); statErr == nil {
			if err := os.RemoveAll(local); err != nil {
				return err
			}
			print_status("Removed local directory " + local)
		}
	}
	print_success("Removed " + id + " from " + name)
	return nil
}

func runPluginDeploy(cmd *cobra.Command, id string) error {
	fromFlag, _ := cmd.Flags().GetString("from")
	toFlag, _ := cmd.Flags().GetString("to")
	fromName, err := resolveDeployAccount(fromFlag, "FROM")
	if err != nil {
		return err
	}
	toName, err := resolveDeployAccount(toFlag, "TO")
	if err != nil {
		return err
	}
	if fromName == toName {
		return fmt.Errorf("from and to are the same account %q", fromName)
	}
	fromAccount, err := getAccountConfig(fromName)
	if err != nil {
		return err
	}
	toAccount, err := getAccountConfig(toName)
	if err != nil {
		return err
	}
	dirFlag, _ := cmd.Flags().GetString("dir")
	fromRoot, err := pluginRootForAccount(fromName, fromAccount, dirFlag)
	if err != nil {
		return err
	}
	toRoot, err := pluginRootForAccount(toName, toAccount, "")
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		if !accountIsLocal(fromAccount) {
			return fmt.Errorf("pass a plugin id when the source is not a local account")
		}
		id, err = solePluginID(fromRoot)
		if err != nil {
			return err
		}
	}
	if err := requireHCPluginID(id); err != nil {
		return err
	}
	includeSecrets, _ := cmd.Flags().GetBool("include-secrets")
	yes, _ := cmd.Flags().GetBool("yes")
	confirmAccount, confirmInfo := deployConfirmTarget(fromName, toName)
	if !yes && !confirmSensitiveOperation("deploy", id, confirmAccount, confirmInfo...) {
		return fmt.Errorf("deploy cancelled")
	}
	switch {
	case accountIsLocal(fromAccount) && accountIsLocal(toAccount):
		src, err := localPluginDir(fromRoot, id)
		if err != nil {
			return err
		}
		return copyPluginDir(src, filepath.Join(toRoot, id))
	case accountIsLocal(fromAccount):
		toAccount, err = ensurePluginCaps(toName, toAccount, []string{"plugins.deploy"})
		if err != nil {
			return err
		}
		return installCatalogRelease(toAccount, id)
	case accountIsLocal(toAccount):
		fromAccount, err = ensurePluginCaps(fromName, fromAccount, []string{"plugins.deploy"})
		if err != nil {
			return err
		}
		dest := filepath.Join(toRoot, id)
		return downloadPluginArchive(fromAccount, id, dest, includeSecrets)
	default:
		toAccount, err = ensurePluginCaps(toName, toAccount, []string{"plugins.deploy"})
		if err != nil {
			return err
		}
		return installCatalogRelease(toAccount, id)
	}
}

// deployConfirmTarget keeps each account name intact. The confirmation
// lookup uses the destination account. From and To stay display lines.
func deployConfirmTarget(fromName, toName string) (string, []string) {
	return toName, []string{"From: " + fromName, "To: " + toName}
}

func resolveDeployAccount(flag, label string) (string, error) {
	flag = strings.TrimSpace(flag)
	if flag != "" {
		if _, err := getAccountConfig(flag); err != nil {
			return "", err
		}
		return flag, nil
	}
	return selectAccountForSync(label)
}

func installCatalogRelease(account AccountConfig, id string) error {
	inCatalog, err := catalogHasPlugin(account, id)
	if err != nil {
		return err
	}
	if !inCatalog {
		return fmt.Errorf("%s is not in the signed plugin catalog. Tag the plugin repository so GitHub publishes the release, then run: apito plugin add %s", id, id)
	}
	print_status("Installing " + id + " from the signed catalog.")
	_, err = installCatalogPlugin(account, id)
	return err
}

func catalogHasPlugin(account AccountConfig, id string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(account.ServerURL, "/")+"/system/plugin/catalog", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+account.Token())
	req.Header.Set("X-Use-Cookies", "false")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	var body struct {
		RegistryEnabled bool `json:"registry_enabled"`
		Plugins         []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"plugins"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err
	}
	for _, plugin := range body.Plugins {
		if plugin.ID == id && catalogEntryInstallable(body.RegistryEnabled, plugin.Status) {
			return true, nil
		}
	}
	return false, nil
}

func catalogEntryInstallable(registryEnabled bool, status string) bool {
	if !registryEnabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "installed-local", "catalog-stub", "blocked":
		return false
	default:
		return true
	}
}

func downloadPluginArchive(account AccountConfig, id, dest string, includeSecrets bool) error {
	q := url.Values{}
	if includeSecrets {
		q.Set("include_secrets", "true")
	}
	endpoint := strings.TrimRight(account.ServerURL, "/") + "/system/plugin/" + url.PathEscape(id) + "/archive"
	if encoded := q.Encode(); encoded != "" {
		endpoint += "?" + encoded
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+account.Token())
	req.Header.Set("X-Use-Cookies", "false")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("archive download failed (%d): %s", resp.StatusCode, extractAPIMessage(raw))
	}
	tmp, err := os.CreateTemp("", "apito-plugin-*.zip")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	return extractPluginZip(tmpName, dest)
}

func extractPluginZip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	dest = filepath.Clean(dest)
	for _, file := range reader.File {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		name = strings.TrimPrefix(name, "./")
		if name == "" || strings.HasSuffix(name, "/") {
			continue
		}
		if strings.Contains(name, "..") || strings.HasPrefix(name, "/") {
			return fmt.Errorf("archive path rejected: %s", file.Name)
		}
		target := filepath.Join(dest, filepath.FromSlash(name))
		if !strings.HasPrefix(target, dest+string(os.PathSeparator)) && target != dest {
			return fmt.Errorf("archive path rejected: %s", file.Name)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			src.Close()
			return err
		}
		_, copyErr := io.Copy(out, src)
		src.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func copyPluginDir(src, dest string) error {
	resolved, err := filepath.EvalSymlinks(src)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	return filepath.Walk(resolved, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(resolved, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dest, 0o755)
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			in.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		in.Close()
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}
