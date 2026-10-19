package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type mcpFetchError struct {
	err error
}

func (e mcpFetchError) Error() string { return e.err.Error() }
func (e mcpFetchError) Unwrap() error { return e.err }

const (
	defaultMCPReleaseURL = "https://api.github.com/repos/apito-io/apito-mcp/releases/latest"
	maxMCPArchiveBytes   = 50 << 20
)

type mcpCurrent struct {
	Version string `json:"version"`
	Path    string `json:"path"`
}

func ensureMCPRuntime(update bool) error {
	if !update {
		if path, err := installedMCPServer(); err == nil && path != "" {
			return nil
		}
	}
	if err := downloadLatestMCPRuntime(); err != nil {
		var fetch mcpFetchError
		if errors.As(err, &fetch) {
			if path, pathErr := installedMCPServer(); pathErr == nil && path != "" {
				fmt.Fprintf(os.Stderr, "warning: MCP runtime update failed (%s). Using %s\n", redactSecrets(err.Error()), path)
				return nil
			}
			return fmt.Errorf("MCP runtime is not installed and could not be downloaded: %w", err)
		}
		return err
	}
	return nil
}

func installedMCPServer() (string, error) {
	dir, err := mcpCacheDir()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "current.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var current mcpCurrent
	if err := json.Unmarshal(raw, &current); err != nil {
		return "", err
	}
	path := strings.TrimSpace(current.Path)
	if path == "" {
		return "", nil
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", nil
	}
	return path, nil
}

func mcpCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".apito", "mcp")
	if err := os.MkdirAll(filepath.Join(dir, "versions"), 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func mcpReleaseEndpoint() string {
	if raw := strings.TrimSpace(os.Getenv("APITO_MCP_RELEASE_URL")); raw != "" {
		return raw
	}
	return defaultMCPReleaseURL
}

func downloadLatestMCPRuntime() error {
	endpoint := mcpReleaseEndpoint()
	if err := validateMCPDownloadURL(endpoint); err != nil {
		return err
	}
	body, err := mcpDownload(endpoint)
	if err != nil {
		return err
	}
	release, err := parseMCPRelease(body)
	if err != nil {
		return err
	}
	version, err := safeMCPVersion(release.Version)
	if err != nil {
		return err
	}
	if err := validateMCPDownloadURL(release.ArchiveURL); err != nil {
		return err
	}
	if err := validateMCPDownloadURL(release.ChecksumURL); err != nil {
		return err
	}
	cache, err := mcpCacheDir()
	if err != nil {
		return err
	}
	archivePath := filepath.Join(cache, version+".download")
	if err := mcpDownloadFile(release.ArchiveURL, archivePath); err != nil {
		_ = os.Remove(archivePath)
		return err
	}
	defer os.Remove(archivePath)
	sums, err := mcpDownload(release.ChecksumURL)
	if err != nil {
		return err
	}
	if err := verifyMCPArchive(archivePath, string(sums), release.ArchiveName); err != nil {
		return err
	}
	return publishMCPArchive(cache, version, archivePath)
}

type mcpReleaseAsset struct {
	Version     string
	ArchiveURL  string
	ChecksumURL string
	ArchiveName string
}

func parseMCPRelease(raw []byte) (mcpReleaseAsset, error) {
	var doc struct {
		TagName     string `json:"tag_name"`
		Version     string `json:"version"`
		ArchiveURL  string `json:"archive_url"`
		ChecksumURL string `json:"checksum_url"`
		Assets      []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return mcpReleaseAsset{}, fmt.Errorf("MCP release manifest is not valid JSON")
	}
	if strings.TrimSpace(doc.ArchiveURL) != "" {
		version := firstNonEmpty(doc.Version, doc.TagName)
		return mcpReleaseAsset{
			Version:     version,
			ArchiveURL:  strings.TrimSpace(doc.ArchiveURL),
			ChecksumURL: strings.TrimSpace(doc.ChecksumURL),
			ArchiveName: filepath.Base(strings.TrimSpace(doc.ArchiveURL)),
		}, nil
	}
	var archiveName, archiveURL, checksumURL string
	for _, asset := range doc.Assets {
		name := strings.TrimSpace(asset.Name)
		switch {
		case name == "SHA256SUMS" || strings.HasSuffix(name, ".sha256"):
			checksumURL = strings.TrimSpace(asset.BrowserDownloadURL)
		case strings.HasPrefix(name, "apito-mcp-") && strings.HasSuffix(name, ".tar.gz"):
			archiveName = name
			archiveURL = strings.TrimSpace(asset.BrowserDownloadURL)
		}
	}
	if archiveURL == "" || checksumURL == "" || strings.TrimSpace(doc.TagName) == "" {
		return mcpReleaseAsset{}, fmt.Errorf("MCP release is missing a versioned archive and SHA256SUMS")
	}
	return mcpReleaseAsset{
		Version:     doc.TagName,
		ArchiveURL:  archiveURL,
		ChecksumURL: checksumURL,
		ArchiveName: archiveName,
	}, nil
}

func publishMCPArchive(cache, version, archivePath string) error {
	versions := filepath.Join(cache, "versions")
	tmpDir, err := os.MkdirTemp(versions, ".partial-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := extractMCPArchive(file, tmpDir); err != nil {
		return err
	}
	server := filepath.Join(tmpDir, "server.mjs")
	info, err := os.Stat(server)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return fmt.Errorf("MCP archive does not contain server.mjs")
	}
	finalDir := filepath.Join(versions, version)
	backup := finalDir + ".previous"
	_ = os.RemoveAll(backup)
	hadFinal := false
	if _, err := os.Stat(finalDir); err == nil {
		hadFinal = true
		if err := os.Rename(finalDir, backup); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpDir, finalDir); err != nil {
		if hadFinal {
			_ = os.Rename(backup, finalDir)
		}
		return err
	}
	_ = os.RemoveAll(backup)
	current := mcpCurrent{Version: version, Path: filepath.Join(finalDir, "server.mjs")}
	raw, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	pointer := filepath.Join(cache, "current.json")
	tmpPointer := pointer + ".tmp"
	if err := os.WriteFile(tmpPointer, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPointer, pointer)
}

func extractMCPArchive(r io.Reader, dest string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("MCP archive is not gzip")
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	dest = filepath.Clean(dest)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := pathCleanArchive(hdr.Name)
		if name == "" || name == "." {
			continue
		}
		if strings.HasPrefix(name, "..") || strings.Contains(name, ".."+string(os.PathSeparator)) || filepath.IsAbs(hdr.Name) {
			return fmt.Errorf("MCP archive path rejected: %s", hdr.Name)
		}
		target := filepath.Join(dest, name)
		if target != dest && !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
			return fmt.Errorf("MCP archive path rejected: %s", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, maxMCPArchiveBytes+1)); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			info, err := os.Stat(target)
			if err != nil {
				return err
			}
			if info.Size() > maxMCPArchiveBytes {
				return fmt.Errorf("MCP archive file is too large")
			}
		default:
			return fmt.Errorf("MCP archive entry type rejected: %s", hdr.Name)
		}
	}
}

func pathCleanArchive(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	return filepath.Clean(name)
}

func verifyMCPArchive(path, sums, archiveName string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	got := hex.EncodeToString(hash.Sum(nil))
	archiveName = filepath.Base(strings.TrimSpace(archiveName))
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 1 && fields[0] == got {
			return nil
		}
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[len(fields)-1], "*")
		if filepath.Base(name) != archiveName {
			continue
		}
		if !strings.EqualFold(fields[0], got) {
			return fmt.Errorf("MCP archive checksum mismatch")
		}
		return nil
	}
	return fmt.Errorf("MCP archive checksum was not published for %s", archiveName)
}

func validateMCPDownloadURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("invalid MCP download URL")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		host := parsed.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
		return fmt.Errorf("refusing non-HTTPS MCP download from %s", parsed.Host)
	default:
		return fmt.Errorf("refusing MCP download scheme %s", parsed.Scheme)
	}
}

func mcpHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many MCP download redirects")
			}
			return validateMCPDownloadURL(req.URL.String())
		},
	}
}

func mcpDownload(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "apito-cli")
	req.Header.Set("Accept", "application/vnd.github+json, application/json")
	resp, err := mcpHTTPClient().Do(req)
	if err != nil {
		return nil, mcpFetchError{err: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, mcpFetchError{err: fmt.Errorf("MCP download returned %s", resp.Status)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxMCPArchiveBytes+1))
	if err != nil {
		return nil, mcpFetchError{err: err}
	}
	if len(body) > maxMCPArchiveBytes {
		return nil, fmt.Errorf("MCP download is too large")
	}
	return body, nil
}

func mcpDownloadFile(rawURL, dest string) error {
	body, err := mcpDownload(rawURL)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, body, 0o600)
}

func safeMCPVersion(version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" || strings.Contains(version, "..") || strings.ContainsAny(version, `/\`) {
		return "", fmt.Errorf("invalid MCP version %q", version)
	}
	return version, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
