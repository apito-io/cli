package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	gover "github.com/hashicorp/go-version"
	"github.com/manifoldco/promptui"
)

// GitHub Container Registry token endpoint
const (
	ghcrTokenURL      = "https://ghcr.io/token?scope=repository:apito-io/%s:pull"
	ghcrTagsURL       = "https://ghcr.io/v2/apito-io/%s/tags/list"
	githubReleasesURL = "https://api.github.com/repos/apito-io/%s/releases/latest"
	engineImageName   = "engine"
	consoleImageName  = "console"
	studioImageName   = "studio"
)

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
}

// GHCRTagsResponse represents the GHCR tags list response
type GHCRTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type GHCRTokenResponse struct {
	Token string `json:"token"`
}

// getLatestEngineVersion fetches the latest engine version from GitHub releases
func getLatestEngineVersion() (string, error) {
	return getLatestVersionFromGitHub(engineImageName)
}

// getLatestConsoleVersion fetches the latest console version from GitHub releases
func getLatestConsoleVersion() (string, error) {
	return getLatestVersionFromGitHub(consoleImageName)
}

// getLatestStudioVersion fetches the newest semver-tagged Studio image from GHCR.
// The registry token endpoint is public for this public package and avoids
// requiring users to configure GitHub credentials just to start Studio.
func getLatestStudioVersion() (string, error) {
	return getLatestGHCRTag(studioImageName)
}

func getLatestGHCRTag(image string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	tokenURL := fmt.Sprintf(ghcrTokenURL, image)
	tokenReq, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GHCR token request: %w", err)
	}
	tokenReq.Header.Set("User-Agent", "apito-cli")
	tokenReq.Header.Set("Accept", "application/json")

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GHCR token: %w", err)
	}
	defer tokenResp.Body.Close()

	if tokenResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GHCR token endpoint returned status %d", tokenResp.StatusCode)
	}

	var token GHCRTokenResponse
	if err := json.NewDecoder(tokenResp.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("failed to decode GHCR token: %w", err)
	}
	if strings.TrimSpace(token.Token) == "" {
		return "", fmt.Errorf("GHCR token response was empty")
	}

	tagsURL := fmt.Sprintf(ghcrTagsURL, image)
	tagsReq, err := http.NewRequest("GET", tagsURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GHCR tags request: %w", err)
	}
	tagsReq.Header.Set("User-Agent", "apito-cli")
	tagsReq.Header.Set("Accept", "application/json")
	tagsReq.Header.Set("Authorization", "Bearer "+token.Token)

	tagsResp, err := client.Do(tagsReq)
	if err != nil {
		return "", fmt.Errorf("failed to fetch GHCR tags: %w", err)
	}
	defer tagsResp.Body.Close()

	if tagsResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GHCR tags endpoint returned status %d", tagsResp.StatusCode)
	}

	var tags GHCRTagsResponse
	if err := json.NewDecoder(tagsResp.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("failed to decode GHCR tags: %w", err)
	}

	return latestSemverTag(tags.Tags)
}

func latestSemverTag(tags []string) (string, error) {
	var latestTag string
	var latestVersion *gover.Version

	for _, tag := range tags {
		rawTag := strings.TrimSpace(tag)
		if rawTag == "" {
			continue
		}

		version, err := gover.NewVersion(strings.TrimPrefix(rawTag, "v"))
		if err != nil {
			continue
		}
		if latestVersion == nil || version.GreaterThan(latestVersion) {
			latestTag = rawTag
			latestVersion = version
		}
	}

	if latestTag != "" {
		return latestTag, nil
	}

	for _, tag := range tags {
		if strings.TrimSpace(tag) == "latest" {
			return "latest", nil
		}
	}

	return "", fmt.Errorf("no usable Studio image tags found")
}

// getLatestVersionFromGitHub fetches the latest version from GitHub releases
func getLatestVersionFromGitHub(component string) (string, error) {
	url := fmt.Sprintf(githubReleasesURL, component)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// GitHub API requires User-Agent header
	req.Header.Set("User-Agent", "apito-cli")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse release data: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no release tag found")
	}

	return release.TagName, nil
}

// compareVersions compares two version strings and returns true if newVer is greater than currentVer
func compareVersions(currentVer, newVer string) (bool, error) {
	// Remove 'v' prefix if present
	currentVer = strings.TrimPrefix(currentVer, "v")
	newVer = strings.TrimPrefix(newVer, "v")

	current, err := gover.NewVersion(currentVer)
	if err != nil {
		return false, fmt.Errorf("invalid current version: %w", err)
	}

	new, err := gover.NewVersion(newVer)
	if err != nil {
		return false, fmt.Errorf("invalid new version: %w", err)
	}

	return new.GreaterThan(current), nil
}

func formatStudioCurrentVersion(tag string, created time.Time, repoTags []string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "unknown"
	}
	if _, err := gover.NewVersion(strings.TrimPrefix(tag, "v")); err == nil {
		return tag
	}

	seen := map[string]bool{}
	var releaseTags []string
	for _, repoTag := range repoTags {
		name := repoTag
		if i := strings.LastIndex(repoTag, ":"); i >= 0 {
			name = repoTag[i+1:]
		}
		name = strings.TrimSpace(name)
		if name == "" || name == tag || name == "latest" || seen[name] {
			continue
		}
		if _, err := gover.NewVersion(strings.TrimPrefix(name, "v")); err != nil {
			continue
		}
		seen[name] = true
		releaseTags = append(releaseTags, name)
	}
	sort.Strings(releaseTags)

	var parts []string
	if len(releaseTags) > 0 {
		parts = append(parts, "release "+strings.Join(releaseTags, ", "))
	}
	if !created.IsZero() {
		parts = append(parts, "built "+created.Local().Format("2006-01-02"))
	}
	if len(parts) == 0 {
		return tag
	}
	return fmt.Sprintf("%s (%s)", tag, strings.Join(parts, ", "))
}

func studioUpgradeAvailable(currentTag, latestTag string) bool {
	currentTag = strings.TrimSpace(currentTag)
	latestTag = strings.TrimSpace(latestTag)

	// Empty StudioVersion and the latest tag are both tracking modes. Their
	// normal pull already refreshes the image on every start.
	if currentTag == "" || currentTag == "latest" || latestTag == "" || currentTag == latestTag {
		return false
	}
	if latestTag == "latest" {
		return true
	}

	currentVersion, currentErr := gover.NewVersion(strings.TrimPrefix(currentTag, "v"))
	latestVersion, latestErr := gover.NewVersion(strings.TrimPrefix(latestTag, "v"))
	if currentErr == nil && latestErr == nil {
		return latestVersion.GreaterThan(currentVersion)
	}

	// Non-semver pins such as "local" are still eligible for an explicit
	// upgrade to the newest published image.
	return true
}

// checkForComponentUpdates checks if updates are available for engine and console
func checkForComponentUpdates() (map[string]ComponentUpdate, error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	updates := make(map[string]ComponentUpdate)

	// Check engine version
	if latestEngine, err := getLatestEngineVersion(); err == nil {
		if cfg.EngineVersion == "" {
			// No current version set, this is an update opportunity
			updates["engine"] = ComponentUpdate{
				Component:      "engine",
				CurrentVersion: "unknown",
				LatestVersion:  latestEngine,
				UpdateRequired: true,
			}
		} else {
			// Compare versions
			if isNewer, err := compareVersions(cfg.EngineVersion, latestEngine); err == nil && isNewer {
				updates["engine"] = ComponentUpdate{
					Component:      "engine",
					CurrentVersion: cfg.EngineVersion,
					LatestVersion:  latestEngine,
					UpdateRequired: true,
				}
			}
		}
	}

	// Check console version
	if latestConsole, err := getLatestConsoleVersion(); err == nil {
		if cfg.ConsoleVersion == "" {
			// No current version set, this is an update opportunity
			updates["console"] = ComponentUpdate{
				Component:      "console",
				CurrentVersion: "unknown",
				LatestVersion:  latestConsole,
				UpdateRequired: true,
			}
		} else {
			// Compare versions
			if isNewer, err := compareVersions(cfg.ConsoleVersion, latestConsole); err == nil && isNewer {
				updates["console"] = ComponentUpdate{
					Component:      "console",
					CurrentVersion: cfg.ConsoleVersion,
					LatestVersion:  latestConsole,
					UpdateRequired: true,
				}
			}
		}
	}

	return updates, nil
}

// ComponentUpdate represents an available update for a component
type ComponentUpdate struct {
	Component      string
	CurrentVersion string
	LatestVersion  string
	UpdateRequired bool
}

// pullDockerImage pulls a specific version of a docker image
func pullDockerImage(image, version string) error {
	imageTag := fmt.Sprintf("ghcr.io/apito-io/%s:%s", image, version)
	print_status(fmt.Sprintf("Pulling Docker image: %s", imageTag))

	cmd := exec.Command("docker", "pull", imageTag)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageTag, err)
	}

	print_success(fmt.Sprintf("Successfully pulled %s", imageTag))
	return nil
}

// updateComponentVersion updates the version for a specific component in config.yml
func updateComponentVersion(component, version string) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	switch strings.ToLower(component) {
	case "engine":
		cfg.EngineVersion = version
	case "console":
		cfg.ConsoleVersion = version
	default:
		return fmt.Errorf("unknown component: %s", component)
	}

	if err := saveCLIConfig(cfg); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	print_success(fmt.Sprintf("Updated %s version to %s in config.yml", component, version))
	return nil
}

// promptForComponentUpdates asks user which components to update
func promptForComponentUpdates(updates map[string]ComponentUpdate) []string {
	if len(updates) == 0 {
		return nil
	}

	print_step("🆕 Updates Available")
	print_status("")

	for _, update := range updates {
		currentDisplay := update.CurrentVersion
		if currentDisplay == "" || currentDisplay == "unknown" {
			currentDisplay = "not set"
		}
		print_status(fmt.Sprintf("  %s: %s → %s",
			strings.Title(update.Component),
			currentDisplay,
			update.LatestVersion))
	}

	print_status("")

	// Create update options
	var options []string
	componentMap := make(map[string]string)

	if engineUpdate, exists := updates["engine"]; exists {
		option := fmt.Sprintf("Update Engine (%s → %s)", engineUpdate.CurrentVersion, engineUpdate.LatestVersion)
		if engineUpdate.CurrentVersion == "" || engineUpdate.CurrentVersion == "unknown" {
			option = fmt.Sprintf("Set Engine version (%s)", engineUpdate.LatestVersion)
		}
		options = append(options, option)
		componentMap[option] = "engine"
	}

	if consoleUpdate, exists := updates["console"]; exists {
		option := fmt.Sprintf("Update Console (%s → %s)", consoleUpdate.CurrentVersion, consoleUpdate.LatestVersion)
		if consoleUpdate.CurrentVersion == "" || consoleUpdate.CurrentVersion == "unknown" {
			option = fmt.Sprintf("Set Console version (%s)", consoleUpdate.LatestVersion)
		}
		options = append(options, option)
		componentMap[option] = "console"
	}

	if len(updates) > 1 {
		options = append(options, "Update Both")
	}
	options = append(options, "Skip Updates")

	selector := promptui.Select{
		Label: "Choose update action",
		Items: options,
		Size:  len(options),
	}

	_, selected, err := selector.Run()
	if err != nil {
		return nil
	}

	if selected == "Skip Updates" {
		return nil
	}

	if selected == "Update Both" {
		return []string{"engine", "console"}
	}

	// Return the selected component
	if component, exists := componentMap[selected]; exists {
		return []string{component}
	}

	return nil
}

// getLatestVersionsFromGitHub fetches both engine and console versions
func getLatestVersionsFromGitHub() (engineVersion, consoleVersion string, err error) {
	engineVersion, engineErr := getLatestEngineVersion()
	consoleVersion, consoleErr := getLatestConsoleVersion()

	if engineErr != nil && consoleErr != nil {
		return "", "", fmt.Errorf("failed to fetch versions: engine=%v, console=%v", engineErr, consoleErr)
	}

	// Return what we could fetch, even if one failed
	return engineVersion, consoleVersion, nil
}

// sortVersions sorts version strings in descending order (newest first)
func sortVersions(versions []string) []string {
	versionObjs := make([]*gover.Version, 0, len(versions))

	for _, v := range versions {
		// Remove 'v' prefix if present
		cleaned := strings.TrimPrefix(v, "v")
		if ver, err := gover.NewVersion(cleaned); err == nil {
			versionObjs = append(versionObjs, ver)
		}
	}

	sort.Sort(sort.Reverse(gover.Collection(versionObjs)))

	sorted := make([]string, len(versionObjs))
	for i, v := range versionObjs {
		sorted[i] = "v" + v.String()
	}

	return sorted
}
