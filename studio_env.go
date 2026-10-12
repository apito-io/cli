package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

const (
	studioEngineEnvName  = ".env.engine"
	studioConsoleEnvName = ".env.console"
	studioDataMount      = "/data"
)

// studioLinkedKeys are rewritten on every start so the env files match the
// Studio container: nginx on 5055, engine on 5051, database on /data.
var studioLinkedKeys = []string{
	"COOKIE_DOMAIN",
	"CORS_ORIGIN",
	"PLUGIN_PATH",
	"PRIVATE_KEY_PATH",
	"PUBLIC_KEY_PATH",
	"SERVE_PORT",
	"DEFAULT_DATABASE_DIR",
	"SQLITE_DATA_DIR",
	"CACHE_DB",
	"KV_ENGINE",
	"KV_DATABASE",
	"QUEUE_ENGINE",
	"QUEUE_DATABASE",
	"SYSTEM_DB_ENGINE",
	"SYSTEM_DB_NAME",
	"BOOTSTRAP_ADMIN_EMAIL",
}

func studioEngineEnv() map[string]string {
	return map[string]string{
		"ENVIRONMENT":                  "local",
		"AUTH_SERVICE_PROVIDER":        "local",
		"BRANKA_KEY":                   "",
		"COOKIE_DOMAIN":                "localhost",
		"CORS_ORIGIN":                  "http://localhost:" + studioPublishPort,
		"PLUGIN_PATH":                  studioDataMount + "/plugins",
		"PRIVATE_KEY_PATH":             studioDataMount + "/keys/private.key",
		"PUBLIC_KEY_PATH":              studioDataMount + "/keys/public.key",
		"SERVE_PORT":                   studioEnginePort,
		"TOKEN_TTL":                    "60",
		"APITO_ADMIN_RESET_SECRET":     "",
		"DEFAULT_DATABASE_DIR":         studioDataMount,
		"SQLITE_DATA_DIR":              studioDataMount,
		"CACHE_DB":                     "memory",
		"CACHE_DB_NAME":                "apito_cache.db",
		"CACHE_TTL":                    "600",
		"KV_ENGINE":                    "coreDB",
		"KV_DATABASE":                  "apito_kv.db",
		"QUEUE_ENGINE":                 "coreDB",
		"QUEUE_DATABASE":               "apito_queue.db",
		"SYSTEM_DB_ENGINE":             "sqlite",
		"SYSTEM_DB_NAME":               "apito_system.db",
		"PROJECT_DB_ENGINE":            "coreDB",
		"PROJECT_DB_NAME":              "apito_project.db",
		"DEFAULT_SAAS_PROJECT_DB_NAME": "apito_saas_project.db",
		"REALTIME_ENGINE":              "memory",
		"EMAIL_PROVIDER":               "noop",
		"BOOTSTRAP_ADMIN_EMAIL":        "oss@apito.io",
	}
}

func studioConsoleEnv() map[string]string {
	origin := "http://localhost:" + studioPublishPort
	return map[string]string{
		"VITE_REST_API":         origin,
		"VITE_GRAPH_API":        origin + "/system/graphql",
		"VITE_PUBLIC_GRAPH_API": origin + "/system/graphql",
		"VITE_GRAPH_SUBS_API":   "ws://localhost:" + studioPublishPort + "/system/graphql/subscription",
		"VITE_AUTH_PROVIDER":    "local",
		"VITE_COOKIE_DOMAIN":    "localhost",
		"VITE_PRODUCT":          "apito",
	}
}

func ensureStudioFiles() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("error finding home directory: %w", err)
	}
	return ensureStudioFilesIn(home)
}

func ensureStudioFilesIn(home string) (bool, error) {
	binDir := filepath.Join(home, ".apito", "bin")
	dbDir := filepath.Join(home, ".apito", "db")
	keyDir := filepath.Join(dbDir, "keys")
	for _, dir := range []string{
		binDir,
		dbDir,
		keyDir,
		filepath.Join(dbDir, "plugins"),
		filepath.Join(dbDir, "logs"),
		filepath.Join(dbDir, "sqlite", "system"),
		filepath.Join(dbDir, "sqlite", "projects"),
		filepath.Join(dbDir, "sqlite", "tenants"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return false, fmt.Errorf("error creating %s: %w", dir, err)
		}
	}

	created := false
	enginePath := filepath.Join(binDir, studioEngineEnvName)
	legacyPath := filepath.Join(binDir, ".env")
	if err := removeIfDir(enginePath); err != nil {
		return false, err
	}
	if err := removeIfDir(legacyPath); err != nil {
		return false, err
	}

	engineExisted := fileExists(enginePath)
	current, err := readEnvFile(enginePath)
	if err != nil {
		return false, err
	}
	if !engineExisted {
		legacy, err := readEnvFile(legacyPath)
		if err != nil {
			return false, err
		}
		if strings.TrimSpace(current["BRANKA_KEY"]) == "" {
			current["BRANKA_KEY"] = legacy["BRANKA_KEY"]
		}
		if strings.TrimSpace(current["APITO_ADMIN_RESET_SECRET"]) == "" {
			current["APITO_ADMIN_RESET_SECRET"] = legacy["APITO_ADMIN_RESET_SECRET"]
		}
		created = true
	}

	desired := applyStudioEngineEnv(current)
	if !engineExisted || !sameStringMap(current, desired) {
		if err := writeEngineEnvFiles(home, desired); err != nil {
			return false, err
		}
		if engineExisted {
			print_status("Updated Studio engine settings in ~/.apito/bin/.env.engine")
		} else {
			print_status("Wrote ~/.apito/bin/.env.engine")
		}
		if strings.TrimSpace(current["BRANKA_KEY"]) == "" {
			print_status("Generated BRANKA_KEY")
		}
	}

	consolePath := filepath.Join(binDir, studioConsoleEnvName)
	if err := removeIfDir(consolePath); err != nil {
		return false, err
	}
	consoleExisted := fileExists(consolePath)
	currentConsole, err := readEnvFile(consolePath)
	if err != nil {
		return false, err
	}
	desiredConsole := studioConsoleEnv()
	if !consoleExisted || !sameStringMap(currentConsole, desiredConsole) {
		if err := writeEnvFile(consolePath, desiredConsole, 0644); err != nil {
			return false, err
		}
		if consoleExisted {
			print_status("Updated Studio console settings in ~/.apito/bin/.env.console")
		} else {
			print_status("Wrote ~/.apito/bin/.env.console")
			created = true
		}
	}

	keysCreated, err := ensureStudioKeyPair(keyDir)
	if err != nil {
		return false, err
	}
	if keysCreated {
		print_status("Generated Studio signing keys in ~/.apito/db/keys")
		created = true
	}
	return created, nil
}

func printStudioLogin() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	envMap, err := readEnvFile(filepath.Join(home, ".apito", "bin", studioEngineEnvName))
	if err != nil {
		return
	}
	email := strings.TrimSpace(envMap["BOOTSTRAP_ADMIN_EMAIL"])
	password := strings.TrimSpace(envMap["BOOTSTRAP_ADMIN_PASSWORD"])
	if email == "" || password == "" {
		return
	}
	fmt.Println()
	print_success("Studio admin login")
	fmt.Println("  Email:    " + email)
	fmt.Println("  Password: " + password)
	fmt.Println("  Login:    http://localhost:" + studioPublishPort + "/auth/login")
	fmt.Println()
}

func generateStudioAdminPassword() string {
	const alphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	const length = 20
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return generateSecurePassword() + generateSecurePassword()
	}
	out := make([]byte, length)
	for i, b := range buf {
		out[i] = alphabet[int(b)%len(alphabet)]
	}
	return string(out)
}

func applyStudioEngineEnv(existing map[string]string) map[string]string {
	out := make(map[string]string, len(existing)+len(studioEngineEnv()))
	for key, value := range existing {
		out[key] = value
	}
	defaults := studioEngineEnv()
	for _, key := range studioLinkedKeys {
		out[key] = defaults[key]
	}
	for key, value := range defaults {
		if strings.TrimSpace(out[key]) == "" && value != "" {
			out[key] = value
		}
	}
	if strings.TrimSpace(out["BRANKA_KEY"]) == "" {
		out["BRANKA_KEY"] = generateBrankaKey()
	}
	if strings.TrimSpace(out["APITO_ADMIN_RESET_SECRET"]) == "" {
		out["APITO_ADMIN_RESET_SECRET"] = generateSecurePassword()
	}
	if strings.TrimSpace(out["BOOTSTRAP_ADMIN_PASSWORD"]) == "" {
		out["BOOTSTRAP_ADMIN_PASSWORD"] = generateStudioAdminPassword()
	}
	return out
}

func writeEngineEnvFiles(home string, envMap map[string]string) error {
	binDir := filepath.Join(home, ".apito", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("error creating bin directory: %w", err)
	}
	for _, name := range []string{studioEngineEnvName, ".env"} {
		if err := writeEnvFile(filepath.Join(binDir, name), envMap, 0600); err != nil {
			return err
		}
	}
	return nil
}

func writeEnvFile(path string, envMap map[string]string, mode os.FileMode) error {
	if err := godotenv.Write(envMap, path); err != nil {
		return fmt.Errorf("error writing %s: %w", path, err)
	}
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("error setting permissions on %s: %w", path, err)
	}
	return nil
}

func ensureStudioKeyPair(keyDir string) (bool, error) {
	privPath := filepath.Join(keyDir, "private.key")
	pubPath := filepath.Join(keyDir, "public.key")
	if fileExists(privPath) && fileExists(pubPath) {
		return false, nil
	}
	if fileExists(privPath) {
		return true, writePublicKeyFromPrivate(privPath, pubPath)
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return false, fmt.Errorf("error generating signing key: %w", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(privPath, privPEM, 0600); err != nil {
		return false, fmt.Errorf("error writing private key: %w", err)
	}
	if err := writePublicPEM(&key.PublicKey, pubPath); err != nil {
		return false, err
	}
	return true, nil
}

func writePublicKeyFromPrivate(privPath, pubPath string) error {
	raw, err := os.ReadFile(privPath)
	if err != nil {
		return err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return fmt.Errorf("signing key at %s is not a PEM file", privPath)
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsed, pkcs8Err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if pkcs8Err != nil {
			return fmt.Errorf("error reading signing key: %w", err)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("signing key is not an RSA key")
		}
		key = rsaKey
	}
	return writePublicPEM(&key.PublicKey, pubPath)
}

func writePublicPEM(pub *rsa.PublicKey, path string) error {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return fmt.Errorf("error encoding public key: %w", err)
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), 0644); err != nil {
		return fmt.Errorf("error writing public key: %w", err)
	}
	return nil
}

func readEnvFile(path string) (map[string]string, error) {
	if !fileExists(path) {
		return map[string]string{}, nil
	}
	envMap, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}
	return envMap, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func removeIfDir(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("error removing directory %s: %w", path, err)
		}
	}
	return nil
}

func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
