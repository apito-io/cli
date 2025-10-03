# Apito CLI Architecture

## Binary Information

- **Type**: Mach-O 64-bit executable arm64
- **Size**: ~13.7 MB
- **Go Version**: 1.25.0
- **Build Mode**: CGO_ENABLED=1
- **Architecture**: ARM64 (Apple Silicon)
- **OS**: Darwin (macOS)

## Command Structure

### Root Command (`apito`)

```
apito [command] [flags]
```

### Available Commands

- `account` - Manage accounts
- `build` - Build project for docker or zip
- `config` - Manage CLI configuration
- `create` - Create a new project or plugin
- `init` - Initialize Apito CLI system configuration
- `logs` - Show logs for Apito services and databases
- `plugin` - Manage plugins (create, deploy, update, list, status)
- `restart` - Restart Apito services
- `self-upgrade` - Check for updates and upgrade the CLI
- `start` - Start the Apito engine and console
- `status` - Show running status for Apito services
- `stop` - Stop Apito services
- `update` - Update apito engine, console, or self

## Plugin Deploy Architecture

### Command Flow

```
apito plugin deploy [plugin-directory] [flags]
```

### Flags

- `-a, --account string` - Account to use for deployment
- `-d, --dir string` - Plugin directory (alternative to positional argument)

### Execution Flow

#### 1. Command Parsing

```go
// main.go
rootCmd.AddCommand(pluginCmd)

// plugin_deploy.go
pluginCmd.AddCommand(pluginDeployCmd)
```

#### 2. Flag Processing

```go
Run: func(cmd *cobra.Command, args []string) {
    // Get plugin directory
    pluginDir, _ := cmd.Flags().GetString("dir")
    if pluginDir == "" && len(args) > 0 {
        pluginDir = args[0]
    }
    if pluginDir == "" {
        pluginDir = "."
    }

    // Get account name from flag
    accountName, _ := cmd.Flags().GetString("account")
    deployPlugin(pluginDir, accountName)
}
```

#### 3. Configuration Loading

```go
func deployPlugin(pluginDir, accountName string) {
    // Check server configuration
    if !checkServerConfig(accountName) {
        return
    }

    // Load plugin configuration
    config, err := readPluginConfig(pluginDir)
    if err != nil {
        print_error("Failed to load plugin configuration: " + err.Error())
        return
    }
}
```

#### 4. Account Configuration

```go
func checkServerConfig(accountName string) bool {
    // Get account configuration
    _, err := getAccountConfig(accountName)
    if err != nil {
        print_error("Account configuration error: " + err.Error())
        return false
    }
    return true
}

func getAccountConfig(accountName string) (AccountConfig, error) {
    cfg, err := loadCLIConfig()
    if err != nil {
        return AccountConfig{}, err
    }

    // Use provided account name or default
    if accountName == "" {
        accountName = cfg.DefaultAccount
    }

    // Interactive account selection if needed
    if accountName == "" {
        // ... interactive selection logic
    }

    account, exists := cfg.Accounts[accountName]
    if !exists {
        return AccountConfig{}, fmt.Errorf("account '%s' does not exist", accountName)
    }

    // Validate account configuration
    if account.ServerURL == "" {
        return AccountConfig{}, fmt.Errorf("account '%s' has no server URL configured", accountName)
    }

    if account.CloudSyncKey == "" {
        return AccountConfig{}, fmt.Errorf("account '%s' has no cloud sync key configured", accountName)
    }

    return account, nil
}
```

#### 5. Confirmation System

```go
func confirmSensitiveOperation(operation, pluginID, accountName string, pluginInfo ...string) bool {
    cfg, err := loadCLIConfig()
    if err != nil {
        print_error("Failed to load configuration: " + err.Error())
        return false
    }

    // Get account info
    var account AccountConfig
    var actualAccountName string
    if accountName != "" {
        if acc, exists := cfg.Accounts[accountName]; exists {
            account = acc
            actualAccountName = accountName
        } else {
            print_error(fmt.Sprintf("Account '%s' does not exist", accountName))
            return false
        }
    } else {
        // Use default account
        if cfg.DefaultAccount != "" {
            if acc, exists := cfg.Accounts[cfg.DefaultAccount]; exists {
                account = acc
                actualAccountName = cfg.DefaultAccount
            }
        }
    }

    // Display operation details
    print_step(fmt.Sprintf("⚠️  Confirmation Required: %s", strings.Title(operation)))
    print_status("")
    print_status("Operation Details:")
    print_status(fmt.Sprintf("  Action: %s", strings.Title(operation)))
    print_status(fmt.Sprintf("  Plugin: %s", pluginID))

    // Show account info
    if actualAccountName != "" {
        print_status(fmt.Sprintf("  Account: %s", actualAccountName))
        print_status(fmt.Sprintf("  Server: %s", account.ServerURL))
    } else {
        print_status("  Account: (default - none set)")
    }

    // Show additional plugin info if provided
    if len(pluginInfo) > 0 {
        for _, info := range pluginInfo {
            print_status(fmt.Sprintf("  %s", info))
        }
    }

    print_status("")
    print_warning("This operation cannot be undone!")

    // Confirmation prompt
    confirmPrompt := promptui.Prompt{
        Label:     fmt.Sprintf("Are you sure you want to %s plugin '%s'? (y/N)", operation, pluginID),
        IsConfirm: true,
        Default:   "n",
    }

    if _, err := confirmPrompt.Run(); err != nil {
        print_status("Operation cancelled")
        return false
    }

    return true
}
```

#### 6. Plugin Deployment

```go
// Ask for confirmation before deployment
if !confirmSensitiveOperation("deploy", pluginID, accountName,
    fmt.Sprintf("Version: %s", config.Plugin.Version),
    fmt.Sprintf("Language: %s", config.Plugin.Language),
    fmt.Sprintf("Type: %s", config.Plugin.Type)) {
    return
}

// Create deployment package
packagePath, err := createDeploymentPackage(pluginDir, config)
if err != nil {
    print_error("Failed to create deployment package: " + err.Error())
    return
}
defer os.Remove(packagePath) // Clean up

// Deploy to server
response, err := deployToServer(packagePath, config, false, pluginDir, accountName)
if err != nil {
    print_error("Failed to deploy plugin: " + err.Error())
    return
}
```

#### 7. Server Communication

```go
func deployToServer(packagePath string, config *PluginConfig, isUpdate bool, pluginDir, accountName string) (*PluginOperationResponse, error) {
    account, err := getAccountConfig(accountName)
    if err != nil {
        return nil, fmt.Errorf("failed to get account configuration: %w", err)
    }

    serverURL := account.ServerURL
    cloudSyncKey := account.CloudSyncKey

    // Create multipart form data
    var buf bytes.Buffer
    writer := multipart.NewWriter(&buf)

    // Add plugin package file
    file, err := os.Open(packagePath)
    if err != nil {
        return nil, fmt.Errorf("failed to open package file: %w", err)
    }
    defer file.Close()

    part, err := writer.CreateFormFile("plugin", filepath.Base(packagePath))
    if err != nil {
        return nil, fmt.Errorf("failed to create form file: %w", err)
    }

    _, err = io.Copy(part, file)
    if err != nil {
        return nil, fmt.Errorf("failed to copy file: %w", err)
    }

    // Add plugin metadata
    metadata := map[string]interface{}{
        "id":       config.Plugin.ID,
        "version":  config.Plugin.Version,
        "language": config.Plugin.Language,
        "type":     config.Plugin.Type,
        "isUpdate": isUpdate,
    }

    metadataJSON, _ := json.Marshal(metadata)
    writer.WriteField("metadata", string(metadataJSON))

    writer.Close()

    // Create HTTP request
    endpoint := "/system/plugin/deploy"
    if isUpdate {
        endpoint = "/system/plugin/update"
    }

    req, err := http.NewRequest("POST", serverURL+endpoint, &buf)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", writer.FormDataContentType())
    req.Header.Set("X-Apito-Sync-Key", cloudSyncKey)

    // Send request
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    // Parse response
    var response PluginOperationResponse
    if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &response, nil
}
```

## Configuration Architecture

### Configuration Structure

```go
type AccountConfig struct {
    ServerURL    string `yaml:"server_url"`     // Apito server URL for plugin management
    CloudSyncKey string `yaml:"cloud_sync_key"` // Cloud sync key for authentication
}

type CLIConfig struct {
    Mode           string                   `yaml:"mode"`                     // "docker" or "manual"
    DefaultAccount string                   `yaml:"default_account"`          // Default account name
    DefaultPlugin  string                   `yaml:"default_plugin,omitempty"` // Default plugin for operations
    Timeout        int                      `yaml:"timeout,omitempty"`        // Request timeout in seconds
    Accounts       map[string]AccountConfig `yaml:"accounts"`                 // Account configurations

    // Legacy fields for backward compatibility
    ServerURL    string `yaml:"server_url,omitempty"`     // Legacy: Apito server URL
    CloudSyncKey string `yaml:"cloud_sync_key,omitempty"` // Legacy: Cloud sync key
}
```

### Configuration File Location

- **Path**: `~/.apito/config.yml`
- **Format**: YAML
- **Auto-creation**: Directory and file created automatically if missing

### Configuration Loading

```go
func loadCLIConfig() (*CLIConfig, error) {
    path, err := configFilePath()
    if err != nil {
        return nil, err
    }

    cfg := &CLIConfig{
        Mode:     "docker",
        Timeout:  30,
        Accounts: make(map[string]AccountConfig),
    }

    if _, err := os.Stat(path); os.IsNotExist(err) {
        return cfg, nil
    }

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, err
    }

    // Initialize accounts map if nil
    if cfg.Accounts == nil {
        cfg.Accounts = make(map[string]AccountConfig)
    }

    // Migration logic for legacy configuration
    if len(cfg.Accounts) == 0 && (cfg.ServerURL != "" || cfg.CloudSyncKey != "") {
        cfg.Accounts["default"] = AccountConfig{
            ServerURL:    cfg.ServerURL,
            CloudSyncKey: cfg.CloudSyncKey,
        }
        cfg.DefaultAccount = "default"
    }

    return cfg, nil
}
```

## Dependencies

### Core Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/pflag` - Flag parsing
- `github.com/manifoldco/promptui` - Interactive prompts
- `gopkg.in/yaml.v3` - YAML configuration
- `github.com/joho/godotenv` - Environment variables

### Plugin SDK Dependencies

- `github.com/apito-io/go-apito-plugin-sdk` - Plugin development SDK
- `github.com/hashicorp/go-plugin` - HashiCorp plugin framework
- `github.com/hashicorp/go-hclog` - Logging
- `google.golang.org/grpc` - gRPC communication
- `google.golang.org/protobuf` - Protocol buffers

### System Dependencies

- `golang.org/x/sys` - System calls
- `github.com/cavaliergopher/grab/v3` - File downloads
- `github.com/chzyer/readline` - Line editing
- `github.com/eiannone/keyboard` - Keyboard input

## Build Information

### Build Flags

- `-buildmode=exe` - Executable build mode
- `-compiler=gc` - Go compiler
- `CGO_ENABLED=1` - CGO enabled
- `GOARCH=arm64` - ARM64 architecture
- `GOOS=darwin` - macOS target
- `GOARM64=v8.0` - ARM64 version

### Version Information

- **Module**: github.com/apito-io/cli
- **Version**: v0.2.3+dirty
- **VCS**: Git
- **Revision**: 2a3b2a7cca47a49441f1b40a78964607cb987fc4
- **Build Time**: 2025-09-28T19:22:10Z

## Runtime Architecture

### Memory Layout

- **Binary Size**: ~13.7 MB
- **Static Linking**: Most dependencies statically linked
- **Dynamic Libraries**: Only system libraries (libSystem, CoreFoundation, Security)

### Execution Model

- **Single-threaded**: Main execution thread
- **HTTP Client**: Concurrent HTTP requests for server communication
- **File I/O**: Synchronous file operations
- **User Input**: Blocking interactive prompts

### Error Handling

- **Graceful Degradation**: Operations fail gracefully with clear error messages
- **User Feedback**: Colored output with status indicators
- **Logging**: Structured logging with different levels (INFO, WARNING, ERROR, SUCCESS)

## Security Considerations

### Authentication

- **Cloud Sync Key**: Used for server authentication
- **Header**: `X-Apito-Sync-Key` for API requests
- **Storage**: Keys stored in local configuration file
- **Transmission**: HTTPS for all server communication

### File System

- **Configuration**: Stored in user home directory
- **Permissions**: 0755 for directories, 0644 for files
- **Temporary Files**: Automatic cleanup after operations

### Network

- **HTTPS Only**: All server communication over HTTPS
- **Timeout**: 30-second timeout for HTTP requests
- **User Agent**: Custom user agent for requests
