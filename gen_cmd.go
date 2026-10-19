package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	genDir        string
	genLanguage   string
	genFramework  string
	genSchema     string
	genModels     string
	genOut        string
	genUIPackage  string
	genDraft      bool
	genForce      bool
	genClean      bool
	genCheck      bool
	genRelations  bool
	genInstall    bool
	runTSCodegen  = defaultTSCodegen
	runGoCodegen  = defaultGoCodegen
	promoteGenDir = defaultPromoteGenDir
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate a typed client from the published schema",
	Long: `apito gen writes a credential-free client into the selected project.

TypeScript output is TypeScript/TSX from the same apito-codegen generator used
by the JavaScript SDK. The generator discovers models and bounded relations
from the published schema. Pass --framework none for types and an SDK without
React Query hooks. Astro and React include hooks.

Dart output writes a credential-free schema snapshot for the Flutter
build_runner generator. It does not generate Dart itself.

Go output runs the consumer-project adapter. It does not run go generate inside
the SDK repository.`,
	RunE: runGen,
}

var typesCmd = &cobra.Command{
	Use:   "types",
	Short: "Alias of apito gen",
	RunE:  runGen,
}

func initGenFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&genDir, "dir", "", "Project directory (default: current directory)")
	cmd.Flags().StringVar(&genLanguage, "language", "", "typescript, go, or dart")
	cmd.Flags().StringVar(&genFramework, "framework", "", "astro, react, or none")
	cmd.Flags().StringVar(&genSchema, "schema", "", "Published GraphQL introspection JSON file")
	cmd.Flags().StringVar(&genModels, "models", "", "Comma-separated model names")
	cmd.Flags().StringVar(&genUIPackage, "ui-package", "", "UI package for TypeScript app artifacts")
	cmd.Flags().StringVar(&genOut, "out", "", "Output directory")
	cmd.Flags().BoolVar(&genDraft, "draft", false, "Use the staged draft schema")
	cmd.Flags().BoolVar(&genForce, "force", false, "Overwrite owned or colliding files")
	cmd.Flags().BoolVar(&genClean, "clean", false, "Delete previously managed files that this run does not recreate")
	cmd.Flags().BoolVar(&genCheck, "check", false, "Fail when the output directory does not match a fresh generation")
	cmd.Flags().BoolVar(&genRelations, "relations", true, "Include relations")
	cmd.Flags().BoolVar(&genInstall, "install", false, "Install the TypeScript SDK with the detected package manager")
	cmd.Flags().BoolVar(&flagDryRun, "dry-run", false, "Print the plan without writing files")
}

func runGen(cmd *cobra.Command, args []string) error {
	dir := genDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = cwd
	}
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	language, detectedFramework, err := detectLanguage(dir, genLanguage)
	if err != nil {
		return err
	}
	framework := strings.ToLower(strings.TrimSpace(genFramework))
	if framework == "" {
		framework = detectedFramework
	}
	switch framework {
	case "astro", "react", "none":
	default:
		return fmt.Errorf("--framework must be astro, react, or none")
	}
	if language == "go" || language == "dart" {
		framework = "none"
	}
	if genDraft {
		ctx, err := resolveCommandContext()
		if err != nil {
			return err
		}
		if !ctx.Caps.Has(capSchemaStaging) {
			return fmt.Errorf("draft schema requires schema staging support on this server. Omit --draft to use the published schema")
		}
	}
	out := genOut
	if out == "" {
		switch language {
		case "go":
			out = filepath.Join(dir, "internal", "apito")
		case "dart":
			out = filepath.Join(dir, "apito_schema.json")
		default:
			out = filepath.Join(dir, "src", "lib", "apito-generated")
		}
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(dir, out)
	}
	profile := "unselected"
	if strings.TrimSpace(flagAccount) != "" || strings.TrimSpace(flagProject) != "" {
		if ctx, err := resolveCommandContext(); err == nil && ctx.ProjectID != "" {
			if project, err := ctx.syncClient().CurrentProject(); err == nil && project != nil {
				profile = backendProfile(*project)
			}
		}
	}
	plan := map[string]interface{}{
		"dir": dir, "language": language, "framework": framework, "out": out,
		"backend_profile": profile,
		"draft":           genDraft, "models": splitCSV(genModels), "relations": genRelations,
		"package_manager": detectPackageManager(dir),
	}
	if flagDryRun {
		return printJSONOrLines(plan, func() {
			fmt.Printf("language=%s framework=%s out=%s\n", language, framework, out)
		})
	}
	schemaPath := genSchema
	var projectID, tenantID string
	if schemaPath == "" {
		ctx, err := resolveCommandContext()
		if err != nil {
			return err
		}
		if err := requireCapability(ctx.Caps, capPublishedIntrospection, "apito gen"); err != nil {
			return err
		}
		raw, err := fetchIntrospection(ctx)
		if err != nil {
			return err
		}
		f, err := os.CreateTemp("", "apito-introspection-*.json")
		if err != nil {
			return err
		}
		schemaPath = f.Name()
		if _, err := f.Write(raw); err != nil {
			f.Close()
			return err
		}
		f.Close()
		defer os.Remove(schemaPath)
		projectID, tenantID = ctx.ProjectID, ctx.TenantID
	}
	if language == "dart" {
		dest := schemaSnapshotDest(out)
		if genCheck {
			if err := checkSchemaSnapshot(schemaPath, dest); err != nil {
				return err
			}
			return printJSONOrLines(map[string]interface{}{"out": dest, "check": "passed"}, func() {
				fmt.Printf("schema snapshot matches %s\n", dest)
			})
		}
		if err := writeSchemaSnapshot(schemaPath, dest); err != nil {
			return err
		}
		return printJSONOrLines(map[string]interface{}{"out": dest, "language": "dart"}, func() {
			fmt.Printf("wrote schema snapshot %s\n", dest)
		})
	}
	stage, err := os.MkdirTemp(filepath.Dir(out), ".apito-gen-staging-")
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return err
			}
			stage, err = os.MkdirTemp(filepath.Dir(out), ".apito-gen-staging-")
		}
		if err != nil {
			return err
		}
	}
	defer os.RemoveAll(stage)
	switch language {
	case "typescript":
		artifacts := "app"
		if framework == "none" {
			artifacts = "sdk"
		}
		if err := runTSCodegen(schemaPath, stage, genModels, artifacts); err != nil {
			return err
		}
	case "go":
		modulePath, err := readGoModule(dir)
		if err != nil {
			return err
		}
		if err := runGoCodegen(schemaPath, stage, genModels, modulePath, true); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported language %s", language)
	}
	files, err := stagedFiles(stage)
	if err != nil {
		return err
	}
	for _, rel := range files {
		secret, err := fileHasSecret(filepath.Join(stage, rel))
		if err != nil {
			return err
		}
		if secret {
			return fmt.Errorf("refusing to write %s because it contains a credential", rel)
		}
	}
	if genCheck {
		if err := diffGenerated(out, stage, files); err != nil {
			return err
		}
		return printJSONOrLines(map[string]interface{}{"out": out, "check": "passed"}, func() {
			fmt.Printf("codegen check passed (%s)\n", out)
		})
	}
	previous, err := loadManifest(out)
	if err != nil {
		return err
	}
	if err := checkGeneratedCollisions(out, files, previous, genForce); err != nil {
		return err
	}
	if err := promoteGenDir(stage, out, files); err != nil {
		return fmt.Errorf("generation was not promoted: %w", err)
	}
	next := map[string]string{}
	for _, rel := range files {
		sum, err := hashFile(filepath.Join(out, rel))
		if err != nil {
			return err
		}
		next[rel] = sum
	}
	if genClean {
		if err := cleanManagedFiles(out, previous, next); err != nil {
			return err
		}
	}
	schemaRaw, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}
	manifest := genManifest{
		SchemaDigest: hashBytes(schemaRaw),
		ProjectID:    projectID,
		TenantID:     tenantID,
		Language:     language,
		Framework:    framework,
		Generator:    "apito-gen",
		Models:       splitCSV(genModels),
		PackageMgr:   detectPackageManager(dir),
		Files:        next,
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestPath(out), append(raw, '\n'), 0o644); err != nil {
		return err
	}
	if genInstall && language == "typescript" {
		if err := installTS(dir, detectPackageManager(dir)); err != nil {
			return err
		}
	}
	return printJSONOrLines(map[string]interface{}{"out": out, "files": files, "framework": framework}, func() {
		fmt.Printf("generated %s (%s)\n", out, framework)
	})
}

func tsCodegenArgs(schemaPath, out, models, artifacts string) []string {
	args := []string{
		"--schema", schemaPath,
		"--out", out,
		"--artifacts", artifacts,
		"--skip-document-validation",
	}
	if strings.TrimSpace(models) != "" {
		args = append(args, "--models", models)
	}
	if !genRelations {
		args = append(args, "--no-relations")
	}
	if pkg := strings.TrimSpace(genUIPackage); pkg != "" {
		args = append(args, "--ui-package", pkg)
	}
	return args
}

func defaultTSCodegen(schemaPath, out, models, artifacts string) error {
	args := tsCodegenArgs(schemaPath, out, models, artifacts)
	work := genDir
	if strings.TrimSpace(work) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		work = cwd
	}
	var cmd *exec.Cmd
	if codegen := os.Getenv("APITO_CODEGEN_BIN"); codegen != "" {
		cmd = exec.Command(codegen, args...)
	} else if detectPackageManager(work) == "pnpm" {
		cmd = exec.Command("pnpm", append([]string{"exec", "apito-codegen"}, args...)...)
	} else {
		bin, err := exec.LookPath("npx")
		if err != nil {
			return fmt.Errorf("npx is required to run apito-codegen. Install Node.js and @apito-io/js-admin-sdk")
		}
		cmd = exec.Command(bin, append([]string{"--yes", "apito-codegen"}, args...)...)
	}
	cmd.Dir = work
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apito-codegen failed: %w", err)
	}
	return nil
}

func defaultGoCodegen(schemaPath, out, models, modulePath string, prune bool) error {
	args := []string{"run", "github.com/apito-io/go-admin-sdk/cmd/apito-gen",
		"-schema", schemaPath, "-out", out, "-module", modulePath}
	if models != "" {
		args = append(args, "-models", models)
	}
	if prune {
		args = append(args, "-prune")
	}
	if bin := os.Getenv("APITO_GO_GEN"); bin != "" {
		cmd := exec.Command(bin, args[2:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go codegen failed: %w", err)
	}
	return nil
}

func defaultPromoteGenDir(stage, out string, files []string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	for _, rel := range files {
		src := filepath.Join(stage, rel)
		dst := filepath.Join(out, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		raw, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func stagedFiles(stage string) ([]string, error) {
	var files []string
	err := filepath.Walk(stage, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		if rel == "apito.config.json" || rel == ".codegen-work" || strings.HasPrefix(rel, ".codegen-work"+string(filepath.Separator)) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	return files, err
}

func diffGenerated(out, stage string, files []string) error {
	var drifted []string
	for _, rel := range files {
		staged, err := os.ReadFile(filepath.Join(stage, rel))
		if err != nil {
			return err
		}
		current, err := os.ReadFile(filepath.Join(out, rel))
		if err != nil || !bytes.Equal(staged, current) {
			drifted = append(drifted, rel)
		}
	}
	if len(drifted) > 0 {
		return fmt.Errorf("generated output drifted: %s", strings.Join(drifted, ", "))
	}
	return nil
}

func backendProfile(project SyncProject) string {
	kind := strings.ToLower(strings.TrimSpace(project.ProjectType))
	switch {
	case project.PerTenantSeparateDatabase:
		return "saas-per-tenant"
	case kind == "saas" || project.TenantModelName != "":
		return "saas-shared"
	default:
		return "general"
	}
}

func splitCSV(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func readGoModule(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("go generation needs a go.mod in %s", dir)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("go.mod in %s has no module path", dir)
}

func installTS(dir, mgr string) error {
	if mgr == "" {
		mgr = "npm"
	}
	cmd := exec.Command(mgr, "install", "@apito-io/js-admin-sdk")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
