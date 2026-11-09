package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	systemBoilerplateURL  = "https://github.com/apito-io/apito-hello-world-go-plugin.git"
	paymentBoilerplateURL = "https://github.com/apito-io/apito-payment-plugin-template.git"
)

var pluginInitCmd = &cobra.Command{
	Use:   "init [name-or-path]",
	Short: "Clone a system or payment boilerplate",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		if err := runPluginInit(cmd, arg); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "Pick a local plugin, then check, build, or push it",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := runPluginList(cmd); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

var pluginCheckCmd = &cobra.Command{
	Use:   "check [dir]",
	Short: "Check plugin files and the GitHub release workflow",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		dir, err := resolvePluginWorkdir(arg)
		if err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
		if err := checkPlugin(dir); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
		print_success("Plugin check passed: " + dir)
	},
}

var pluginPushCmd = &cobra.Command{
	Use:     "push [dir]",
	Aliases: []string{"deploy"},
	Short:   "Bump the version, commit, tag, and push",
	Long:    `Runs check, then writes the next version into config.yml and sdk.Init. Requires -m. Does not upload a binary.`,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		arg := ""
		if len(args) > 0 {
			arg = args[0]
		}
		if err := runPluginPush(cmd, arg); err != nil {
			print_error(err.Error())
			os.Exit(1)
		}
	},
}

func defaultPluginRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".apito", "plugins"), nil
}

func runPluginInit(cmd *cobra.Command, arg string) error {
	kind, _ := cmd.Flags().GetString("type")
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		if !syncStdinInteractive() {
			return fmt.Errorf("pass --type system or --type payment")
		}
		prompt := promptui.Select{
			Label: "Plugin type",
			Items: []string{"system", "payment"},
		}
		_, selected, err := prompt.Run()
		if err != nil {
			return fmt.Errorf("plugin type cancelled")
		}
		kind = selected
	}
	repo, err := boilerplateURL(kind)
	if err != nil {
		return err
	}

	name := strings.TrimSpace(arg)
	explicitPath := name != "" && (strings.Contains(name, "/") || strings.Contains(name, string(os.PathSeparator)))
	var dest string
	if explicitPath {
		dest, err = filepath.Abs(name)
		if err != nil {
			return err
		}
		name = filepath.Base(dest)
	} else {
		if name == "" {
			if !syncStdinInteractive() {
				return fmt.Errorf("pass a plugin name")
			}
			prompt := promptui.Prompt{Label: "Plugin name (prefixed with hc-)", Validate: validatePluginName}
			name, err = prompt.Run()
			if err != nil {
				return fmt.Errorf("plugin name cancelled")
			}
		}
		if err := validatePluginName(name); err != nil {
			return err
		}
		if !strings.HasPrefix(name, "hc-") {
			name = "hc-" + name
		}
		parent, err := initParent(cmd)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
		dest = filepath.Join(parent, name)
	}
	if err := requireHCPluginID(ensureHCName(name)); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("plugin directory already exists: %s", dest)
	}
	print_step("Create plugin")
	print_status("Cloning " + repo)
	if err := runGitClone(repo, dest); err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Join(dest, ".git")); err != nil {
		print_warning("Failed to remove .git: " + err.Error())
	}
	if err := rewritePluginIdentity(dest, ensureHCName(filepath.Base(dest))); err != nil {
		return err
	}
	print_success("Plugin scaffold created: " + dest)
	print_status("Next: apito plugin build " + dest)
	return nil
}

func ensureHCName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "hc-") {
		return name
	}
	return "hc-" + name
}

func boilerplateURL(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "system":
		return systemBoilerplateURL, nil
	case "payment":
		return paymentBoilerplateURL, nil
	default:
		return "", fmt.Errorf("plugin type must be system or payment")
	}
}

func initParent(cmd *cobra.Command) (string, error) {
	dirFlag, _ := cmd.Flags().GetString("dir")
	dirFlag = strings.TrimSpace(dirFlag)
	if dirFlag == "" {
		return os.Getwd()
	}
	return filepath.Abs(dirFlag)
}

type listedPlugin struct {
	Name    string
	Version string
	Path    string
	Text    string
}

func runPluginList(cmd *cobra.Command) error {
	roots, err := pluginListRoots(cmd)
	if err != nil {
		return err
	}
	for _, root := range roots {
		print_status("Searching " + root)
	}
	rows, err := discoverLocalPlugins(roots)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		print_status("No local plugins")
		print_status("To search another directory, cd there or run: apito plugin --dir <path>")
		return nil
	}
	if !syncStdinInteractive() {
		fmt.Print(formatLocalPluginTable(rows))
		return nil
	}
	row, ok, err := promptPlugin(rows)
	if err != nil {
		return err
	}
	if !ok {
		print_status("Selection cancelled")
		return nil
	}
	return runPluginAction(cmd, row)
}

func promptCancelled(err error) bool {
	return errors.Is(err, promptui.ErrInterrupt) || errors.Is(err, promptui.ErrEOF) || errors.Is(err, promptui.ErrAbort)
}

func pluginPickText(row listedPlugin, showParent bool) string {
	text := fmt.Sprintf("%s  %s", row.Name, row.Version)
	if showParent {
		text += "  " + filepath.Base(filepath.Dir(row.Path))
	}
	return text
}

func pluginPicks(rows []listedPlugin) []listedPlugin {
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Name+"\x00"+row.Version]++
	}
	picks := make([]listedPlugin, len(rows))
	for i, row := range rows {
		row.Text = pluginPickText(row, counts[row.Name+"\x00"+row.Version] > 1)
		picks[i] = row
	}
	return picks
}

func promptPlugin(rows []listedPlugin) (listedPlugin, bool, error) {
	picks := pluginPicks(rows)
	size := len(picks)
	if size > 12 {
		size = 12
	}
	prompt := promptui.Select{
		Label:             "Plugin",
		Items:             picks,
		Size:              size,
		StartInSearchMode: true,
		Templates: &promptui.SelectTemplates{
			Active:   "▸ {{ .Text | cyan }}",
			Inactive: "  {{ .Text }}",
			Selected: "{{ .Text }}",
		},
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(picks[index].Text), strings.ToLower(strings.TrimSpace(input)))
		},
	}
	index, _, err := prompt.Run()
	if promptCancelled(err) {
		return listedPlugin{}, false, nil
	}
	if err != nil {
		return listedPlugin{}, false, err
	}
	return picks[index], true, nil
}

func runPluginAction(cmd *cobra.Command, row listedPlugin) error {
	prompt := promptui.Select{
		Label: row.Name + " " + row.Version,
		Items: []string{"check", "build", "push"},
		Size:  3,
	}
	_, action, err := prompt.Run()
	if promptCancelled(err) {
		print_status("Selection cancelled")
		return nil
	}
	if err != nil {
		return err
	}
	switch action {
	case "check":
		if err := checkPlugin(row.Path); err != nil {
			return err
		}
		print_success("Plugin check passed: " + row.Path)
		return nil
	case "build":
		buildPlugin(row.Path, cmd)
		return nil
	case "push":
		return promptPluginPush(row)
	default:
		return fmt.Errorf("unknown plugin action %s", action)
	}
}

func promptPluginPush(row listedPlugin) error {
	next, err := bumpSemver(row.Version, "patch")
	if err != nil {
		return err
	}
	versionPrompt := promptui.Prompt{
		Label:   "Version",
		Default: next,
		Validate: func(value string) error {
			chosen := strings.TrimPrefix(strings.TrimSpace(value), "v")
			if _, err := parseSemver(chosen); err != nil {
				return err
			}
			return requireNewerVersion(row.Version, chosen)
		},
	}
	version, err := versionPrompt.Run()
	if promptCancelled(err) {
		print_status("Selection cancelled")
		return nil
	}
	if err != nil {
		return err
	}
	messagePrompt := promptui.Prompt{
		Label: "Commit message",
		Validate: func(value string) error {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("commit message is required")
			}
			return nil
		},
	}
	message, err := messagePrompt.Run()
	if promptCancelled(err) {
		print_status("Selection cancelled")
		return nil
	}
	if err != nil {
		return err
	}
	chosen := strings.TrimPrefix(strings.TrimSpace(version), "v")
	return releasePluginVersion(row.Path, strings.TrimSpace(message), chosen)
}

func pluginListRoots(cmd *cobra.Command) ([]string, error) {
	dirFlag, _ := cmd.Flags().GetString("dir")
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return pluginListRootsFrom(strings.TrimSpace(dirFlag), cwd)
}

func pluginListRootsFrom(dirFlag, cwd string) ([]string, error) {
	if dirFlag != "" {
		abs, err := filepath.Abs(dirFlag)
		if err != nil {
			return nil, err
		}
		return []string{abs}, nil
	}
	roots := []string{cwd}
	home, err := defaultPluginRoot()
	if err != nil {
		return roots, nil
	}
	if home != cwd {
		roots = append(roots, home)
	}
	return roots, nil
}

func discoverLocalPlugins(roots []string) ([]listedPlugin, error) {
	seen := map[string]struct{}{}
	var rows []listedPlugin
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return
		}
		if _, ok := seen[abs]; ok {
			return
		}
		seen[abs] = struct{}{}
		if cfg, err := readPluginConfig(abs); err == nil && strings.TrimSpace(cfg.Plugin.ID) != "" {
			rows = append(rows, listedPlugin{Name: cfg.Plugin.ID, Version: cfg.Plugin.Version, Path: abs})
			return
		}
		if depth >= 2 {
			return
		}
		entries, err := os.ReadDir(abs)
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || strings.HasPrefix(name, ".") || name == "node_modules" || name == "ui" || name == "vendor" {
				continue
			}
			walk(filepath.Join(abs, name), depth+1)
		}
	}
	for _, root := range roots {
		walk(root, 0)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name == rows[j].Name {
			return rows[i].Path < rows[j].Path
		}
		return rows[i].Name < rows[j].Name
	})
	return rows, nil
}

func formatLocalPluginTable(rows []listedPlugin) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tPATH")
	for _, row := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\n", row.Name, row.Version, row.Path)
	}
	_ = w.Flush()
	return b.String()
}

func resolvePluginWorkdir(arg string) (string, error) {
	arg = strings.TrimSpace(arg)
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if arg != "" {
		if strings.Contains(arg, "/") || strings.Contains(arg, string(os.PathSeparator)) {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return "", err
			}
			if _, err := readPluginConfig(abs); err != nil {
				return "", err
			}
			return abs, nil
		}
		name := arg
		if !strings.HasPrefix(name, "hc-") {
			name = "hc-" + name
		}
		for _, root := range []string{cwd, mustDefaultPluginRoot()} {
			dir := filepath.Join(root, name)
			if _, err := readPluginConfig(dir); err == nil {
				return dir, nil
			}
			if cfg, err := readPluginConfig(root); err == nil && (cfg.Plugin.ID == arg || cfg.Plugin.ID == name) {
				return root, nil
			}
		}
		return "", fmt.Errorf("plugin %s was not found in %s or %s", arg, cwd, mustDefaultPluginRoot())
	}
	if _, err := readPluginConfig(cwd); err == nil {
		return cwd, nil
	}
	if id, err := solePluginID(cwd); err == nil {
		return filepath.Join(cwd, id), nil
	}
	root := mustDefaultPluginRoot()
	if id, err := solePluginID(root); err == nil {
		return filepath.Join(root, id), nil
	}
	return "", fmt.Errorf("pass a plugin directory")
}

func mustDefaultPluginRoot() string {
	root, err := defaultPluginRoot()
	if err != nil {
		return ""
	}
	return root
}

func checkPlugin(dir string) error {
	cfg, err := readPluginConfig(dir)
	if err != nil {
		return err
	}
	var problems []string
	id := strings.TrimSpace(cfg.Plugin.ID)
	if !strings.HasPrefix(id, "hc-") {
		problems = append(problems, "config.yml id must begin with hc-")
	}
	if strings.ToLower(strings.TrimSpace(cfg.Plugin.Language)) != "go" {
		problems = append(problems, "config.yml language must be go")
	}
	version := strings.TrimSpace(cfg.Plugin.Version)
	if _, err := parseSemver(version); err != nil {
		problems = append(problems, "config.yml version: "+err.Error())
	}
	if strings.TrimSpace(cfg.Plugin.BinaryPath) == "" {
		problems = append(problems, "config.yml binary_path is required")
	}
	repo := strings.TrimSpace(cfg.Plugin.RepositoryURL)
	if !strings.Contains(strings.ToLower(repo), "github.com") {
		problems = append(problems, "config.yml repository_url must be a github.com URL")
	}
	if strings.TrimSpace(cfg.Plugin.HandshakeConfig.MagicCookieKey) == "" || strings.TrimSpace(cfg.Plugin.HandshakeConfig.MagicCookieValue) == "" {
		problems = append(problems, "config.yml handshake cookie is required")
	}
	if err := requireContribution(dir); err != nil {
		problems = append(problems, err.Error())
	}
	for _, file := range []string{"main.go", "go.mod"} {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			problems = append(problems, file+" is required")
		}
	}
	mainPath := filepath.Join(dir, "main.go")
	if main, err := os.ReadFile(mainPath); err == nil {
		foundID, foundVersion := sdkInitIdentity(string(main))
		if foundID != id || foundVersion != version {
			problems = append(problems, fmt.Sprintf("sdk.Init(%q, %q) must match config.yml id %q version %q", foundID, foundVersion, id, version))
		}
	}
	workflow := filepath.Join(dir, ".github", "workflows", "build-release.yml")
	body, err := os.ReadFile(workflow)
	if err != nil {
		problems = append(problems, ".github/workflows/build-release.yml is required")
	} else {
		text := string(body)
		for _, needle := range []string{"v*", "go test", "config.yml", "sdk.Init"} {
			if !strings.Contains(text, needle) {
				problems = append(problems, "build-release.yml must contain "+needle)
			}
		}
	}
	if cfg.Plugin.UIConfig != nil && strings.TrimSpace(cfg.Plugin.UIConfig.EntryPath) != "" {
		if info, err := os.Stat(filepath.Join(dir, "ui")); err != nil || !info.IsDir() {
			problems = append(problems, "ui/ is required when ui_config.entry_path is set")
		}
		entry := filepath.Join(dir, cfg.Plugin.UIConfig.EntryPath)
		if _, err := os.Stat(entry); err != nil {
			problems = append(problems, cfg.Plugin.UIConfig.EntryPath+" is required when ui_config.entry_path is set")
		}
	}
	origin, err := gitOrigin(dir)
	if err != nil || !strings.Contains(strings.ToLower(origin), "github.com") {
		problems = append(problems, "git origin must be a github.com remote")
	}
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(problems, "\n"))
}

func requireContribution(dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	plugin, _ := doc["plugin"].(map[string]any)
	contrib, _ := plugin["contributions"].(map[string]any)
	if contrib == nil {
		return fmt.Errorf("config.yml contributions are required")
	}
	if _, ok := contrib["api"]; ok {
		return nil
	}
	if _, ok := contrib["payment"]; ok {
		return nil
	}
	return fmt.Errorf("config.yml needs an api or payment contribution")
}

var sdkInitPattern = regexp.MustCompile(`sdk\.Init\(\s*"([^"]*)"\s*,\s*"([^"]*)"`)

func sdkInitIdentity(source string) (string, string) {
	match := sdkInitPattern.FindStringSubmatch(source)
	if len(match) != 3 {
		return "", ""
	}
	return match[1], match[2]
}

func gitOrigin(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func runPluginPush(cmd *cobra.Command, arg string) error {
	dir, err := resolvePluginWorkdir(arg)
	if err != nil {
		return err
	}
	message, _ := cmd.Flags().GetString("message")
	minor, _ := cmd.Flags().GetBool("minor")
	major, _ := cmd.Flags().GetBool("major")
	if minor && major {
		return fmt.Errorf("pass only one of --minor or --major")
	}
	kind := "patch"
	if minor {
		kind = "minor"
	}
	if major {
		kind = "major"
	}
	return pushPluginRelease(dir, strings.TrimSpace(message), kind)
}

func pushPluginRelease(dir, message, kind string) error {
	if message == "" {
		return fmt.Errorf("pass -m with the commit message")
	}
	cfg, err := readPluginConfig(dir)
	if err != nil {
		return err
	}
	next, err := bumpSemver(cfg.Plugin.Version, kind)
	if err != nil {
		return err
	}
	return releasePluginVersion(dir, message, next)
}

func releasePluginVersion(dir, message, version string) error {
	if message == "" {
		return fmt.Errorf("pass -m with the commit message")
	}
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	if _, err := parseSemver(version); err != nil {
		return err
	}
	if err := checkPlugin(dir); err != nil {
		return err
	}
	cfg, err := readPluginConfig(dir)
	if err != nil {
		return err
	}
	if err := requireNewerVersion(cfg.Plugin.Version, version); err != nil {
		return err
	}
	if err := writePluginVersion(dir, version); err != nil {
		return err
	}
	if err := gitCommitTagPush(dir, version, message); err != nil {
		return err
	}
	print_success("Pushed " + cfg.Plugin.ID + " " + version)
	return nil
}

func requireNewerVersion(current, next string) error {
	cur, err := parseSemver(current)
	if err != nil {
		return err
	}
	nxt, err := parseSemver(next)
	if err != nil {
		return err
	}
	if nxt[0] > cur[0] || (nxt[0] == cur[0] && (nxt[1] > cur[1] || (nxt[1] == cur[1] && nxt[2] > cur[2]))) {
		return nil
	}
	return fmt.Errorf("%s is not newer than %s", strings.TrimPrefix(strings.TrimSpace(next), "v"), strings.TrimPrefix(strings.TrimSpace(current), "v"))
}

func parseSemver(version string) ([3]int, error) {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("%q is not major.minor.patch", version)
	}
	var parsed [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return [3]int{}, fmt.Errorf("%q is not major.minor.patch", version)
		}
		parsed[i] = n
	}
	return parsed, nil
}

func bumpSemver(version, kind string) (string, error) {
	parsed, err := parseSemver(version)
	if err != nil {
		return "", err
	}
	switch kind {
	case "major":
		parsed[0]++
		parsed[1] = 0
		parsed[2] = 0
	case "minor":
		parsed[1]++
		parsed[2] = 0
	default:
		parsed[2]++
	}
	return fmt.Sprintf("%d.%d.%d", parsed[0], parsed[1], parsed[2]), nil
}

func writePluginVersion(dir, version string) error {
	configPath := filepath.Join(dir, "config.yml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	replaced := replaceVersionField(string(raw), version)
	if replaced == string(raw) {
		return fmt.Errorf("config.yml version field was not updated")
	}
	if err := os.WriteFile(configPath, []byte(replaced), 0o644); err != nil {
		return err
	}
	mainPath := filepath.Join(dir, "main.go")
	main, err := os.ReadFile(mainPath)
	if err != nil {
		return err
	}
	updated := sdkInitPattern.ReplaceAllString(string(main), fmt.Sprintf(`sdk.Init("$1", "%s"`, version))
	if updated == string(main) {
		return fmt.Errorf("sdk.Init version was not updated")
	}
	return os.WriteFile(mainPath, []byte(updated), 0o644)
}

var configVersionPattern = regexp.MustCompile(`(?m)^([ \t]*version:[ \t]*)"?([0-9]+\.[0-9]+\.[0-9]+)"?`)

func replaceVersionField(source, version string) string {
	return configVersionPattern.ReplaceAllString(source, `${1}"`+version+`"`)
}

func gitCommitTagPush(dir, version, message string) error {
	tag := "v" + version
	steps := [][]string{
		{"git", "add", "-A", "--", "."},
		{"git", "commit", "-m", message},
		{"git", "tag", tag},
		{"git", "push", "-u", "origin", "HEAD"},
		{"git", "push", "origin", tag},
	}
	for _, step := range steps {
		cmd := exec.Command(step[0], step[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(step, " "), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
