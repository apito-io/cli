package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

// studioNginxConf is mounted over the image config. Published Studio images
// through v0.1.8 listen on 5055 and proxy to 5051, which is the same port the
// engine binds, so the engine exits with "address already in use".
//
//go:embed studio.nginx.conf
var studioNginxConf []byte

func writeStudioNginx() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding home directory: %w", err)
	}
	path := filepath.Join(home, ".apito", "bin", "studio.nginx.conf")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, studioNginxConf, 0644); err != nil {
		return "", err
	}
	return path, nil
}
