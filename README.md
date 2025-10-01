<p align="center">
  <img src="https://raw.githubusercontent.com/apito-io/engine/main/docs/cover-photo.png" alt="Apito Logo" />
</p>

# Apito CLI

🚀 **Apito CLI** is a powerful command-line tool for managing projects, plugins, functions, and deployments on the Apito platform. It provides a seamless development experience from local development to cloud deployment with enhanced database management, plugin hot-reload system, and Docker integration.

<p align="center">
  <a href="https://apito.io"><strong>Website</strong></a> ·
  <a href="https://docs.apito.io"><strong>Documentation</strong></a> ·
  <a href="https://discord.com/invite/fwHgF8pUpt"><strong>Discord</strong></a>
</p>

## 📋 Table of Contents

- [🆕 What's New](#-whats-new-in-latest-version)
- [📦 Installation](#-installation)
- [🎯 Getting Started](#-getting-started)
- [📚 Command Reference](#-command-reference)
  - [🔧 System Management](#-system-management)
    - [init](#init---initialize-apito-cli-system)
    - [status](#status---show-service-and-database-status)
    - [logs](#logs---view-service-and-database-logs)
  - [🗄️ Database Management](#️-database-management)
    - [start --db](#start---db---start-database-services)
    - [stop --db](#stop---db---stop-database-services)
    - [restart --db](#restart---db---restart-database-services)
  - [🔌 Plugin Management](#-plugin-management)
    - [config](#config---manage-cli-configuration)
    - [account](#account---manage-multiple-accounts)
    - [plugin](#plugin---manage-hashicorp-plugins)
  - [📊 Monitoring & Observability](#-monitoring--observability)
  - [🔧 Project Management](#-project-management)
  - [🚀 Development & Execution](#-development--execution)
    - [start](#start---start-apito-engine-and-console)
    - [stop](#stop---stop-services)
    - [restart](#restart---restart-services)
  - [🏗️ Building & Packaging](#️-building--packaging)
  - [🔄 Updates & Maintenance](#-updates--maintenance)
- [🗂️ Project Structure](#️-project-structure)
- [⚙️ Configuration](#️-configuration)
- [🚨 Troubleshooting](#-troubleshooting)
- [🔗 Useful Links](#-useful-links)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

<p align="center">
  <a href="https://github.com/apito-io/engine/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
  </a>
  <a href="https://github.com/apito-io/engine/releases">
    <img src="https://img.shields.io/github/v/release/apito-io/cli" alt="Release" />
  </a>
  <a href="https://goreportcard.com/report/github.com/apito-io/cli">
    <img src="https://img.shields.io/github/v/release/apito-io/cli" alt="Go Report Card" />
  </a>
  <a href="https://github.com/apito-io/engine/actions">
    <img src="https://github.com/apito-io/engine/workflows/Build%20and%20Release/badge.svg" alt="Build Status" />
  </a>
</p>

## 🆕 **What's New in Latest Version**

### 🔌 **Plugin Management System**

- **Hot Reload Plugins**: Deploy and update HashiCorp plugins without server restarts
- **Multi-language Support**: Create plugins in Go, JavaScript, or Python
- **Secure Deployment**: Cloud sync key authentication for plugin operations
- **Plugin Lifecycle Management**: Create, deploy, update, start, stop, restart, and delete plugins
- **Template Scaffolding**: Generate plugin scaffolds with best practices built-in
- **Real-time Status Monitoring**: Live plugin status and health checks

### ✨ **Enhanced Database Management**

- **Interactive Database Setup**: Choose between default credentials or custom configuration
- **Multiple Database Support**: PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQL Server
- **Persistent Data Volumes**: Named Docker volumes for data persistence
- **Configuration Overwrite Protection**: Confirmation prompts before overwriting existing settings

### 🐳 **Improved Docker Integration**

- **Comprehensive Docker Checks**: Installation, Docker Compose, and daemon status verification
- **Consistent Container Naming**: `apito-system-{engine}` and `apito-project-{engine}` patterns
- **Volume Management**: Automatic volume creation with matching container names
- **OS-Specific Guidance**: Helpful instructions for Docker installation across platforms

### ⚙️ **Enhanced Configuration System**

- **Centralized .env Management**: New `env.go` with comprehensive environment variable operations
- **Configuration Persistence**: Database settings saved to `~/.apito/bin/.env`
- **Smart Configuration Loading**: Automatic detection of system vs. project database settings
- **Backup and Validation**: Environment file backup and required variable validation

### 🔧 **CLI Command Enhancements**

- **Database-Specific Commands**: `--db system` and `--db project` flags for granular control
- **Separated Concerns**: Database setup moved from `init` to dedicated `start --db` commands
- **Consistent Command Patterns**: Unified `--db` flag across start, stop, and restart commands
- **Enhanced Status Monitoring**: Database status display in `apito status` command
- **Comprehensive Logging**: New `apito logs` command for service and database logs

## 📦 Installation

### Homebrew (macOS and Linux)

```bash
brew tap apito-io/tap
brew install apito-cli
```

### Quick Install (Linux & macOS)

```bash
curl -fsSL https://get.apito.io/install.sh | bash
```

### Manual Install

```bash
# Download the installer
wget -O install.sh https://get.apito.io/install.sh

# Make it executable
chmod +x install.sh

# Run the installer
./install.sh
```

### Verify Installation

```bash
apito --version
```

## 🎯 Getting Started

### 0. Initialize Apito CLI (First Time Setup)

```bash
apito init
```

This command will:

- Create core directories under `~/.apito` (e.g., `bin/`, `engine-data/`, `logs/`, `run/`)
- Create `~/.apito/bin/.env` with default system configuration
- Ask you to choose a run mode: Docker (recommended, default) or Manual, and save it to `~/.apito/config.yml`
- **Database setup is now handled separately** - use `apito start --db system` or `apito start --db project`
- Validate database and environment settings
- Check port availability (5050, 4000) and optionally free them (Manual mode only)
- Guide you through any missing configuration

### 1. Create Your First Project

```bash
apito create project -n my-awesome-app
```

This interactive command will:

- Create a new project directory
- Set up system and project databases
- Download the latest Apito engine
- Configure your project settings

### 2. Start Apito Engine and Console

```bash
apito start [--db system|project]
```

This command will (based on run mode stored in `~/.apito/config.yml`):

- **Docker mode (default, recommended)**:
  - Ensure `~/.apito/docker-compose.yml` exists (engine + console)
  - Mount `~/.apito/engine-data -> /go/src/gitlab.com/apito.io/engine/db` and `~/.apito/bin/.env -> /go/src/gitlab.com/apito.io/engine/.env`
  - Start services via `docker compose -f ~/.apito/docker-compose.yml up -d`
  - **Optional**: `--db system` or `--db project` to start specific database types
- **Manual mode**:
  - Download the latest Apito engine to `~/.apito/bin/engine`
  - Download the latest console to `~/.apito/console`
  - Install and configure Caddy to `~/.apito/bin/caddy`
  - Check and free ports 5050/4000 if needed
  - Start engine and serve console locally (managed by PID + logs)

## 📚 Command Reference

### 🔧 **System Management**

#### `init` - Initialize Apito CLI System

Initializes and validates the Apito CLI system configuration.

**Usage:**

```bash
apito init
```

**Features:**

- Creates `~/.apito` directory if it doesn't exist
- Sets up system configuration file with default values
- **Database setup is now handled by `apito start --db` commands**
- Validates system database configuration
- Checks mandatory environment variables (ENVIRONMENT, CORS_ORIGIN, COOKIE_DOMAIN, BRANKA_KEY)
- Validates database connection settings (host, port, user, password)
- Checks port availability (5050, 4000)
- Interactive configuration prompts for missing settings

**What it checks:**

- System database engine configuration (defaults to "coreDB")
- Database connection parameters (host, port, user, password, database name)
- Environment settings (local, development, staging, production)
- CORS and cookie domain configuration
- BRANKA_KEY generation (auto-generates if not provided)
- Port availability for Apito services (5050, 4000)

**Examples:**

```bash
# First-time setup
apito init

# Re-run to validate configuration
apito init
```

**Default Configuration:**
The init command creates a `.env` file with these default values:

```env
ENVIRONMENT=local
COOKIE_DOMAIN=localhost
CORS_ORIGIN=http://localhost:4000
PLUGIN_PATH=plugins
PUBLIC_KEY_PATH=keys/public.key
PRIVATE_KEY_PATH=keys/private.key
APITO_SYSTEM_DB_ENGINE=embed
BRANKA_KEY=<auto-generated-32-character-key>
```

**BRANKA_KEY Behavior:**

- If BRANKA_KEY is not set, a secure 32-character random key is automatically generated
- If BRANKA_KEY is already set, the existing value is preserved
- The generated key includes uppercase, lowercase, numbers, and special characters

#### `status` - Show Service and Database Status

Shows whether services are running and displays database status when available.

**Usage:**

```bash
apito status [engine|console]
```

**Features:**

- **Service Status**: Shows running status for engine and console services
- **Database Status**: Automatically displays database container status when `db-compose.yml` exists
- **Real-time Information**: Shows current running/stopped status for all containers
- **Docker Integration**: Works seamlessly with Docker and local service modes
- **Automatic Detection**: Automatically detects and shows database status without additional flags

**Examples:**

```bash
# Show all services and database status
apito status

# Show specific service status
apito status engine
apito status console
```

**Output Example:**

```bash
[INFO] Database Status:
[SUCCESS]   apito-system-postgres: Running
[SUCCESS]   apito-project-postgres: Running
[WARNING] engine (docker) is not running
[WARNING] console (docker) is not running
```

#### `logs` - View Service and Database Logs

View logs for Apito services (engine/console) and databases with real-time following and tail control.

**Usage:**

```bash
apito logs [engine|console] [--db system|project] [--follow] [--tail N]
```

**Options:**

- `--db system|project` - Show logs for specific database type
- `--follow, -f` - Follow log output in real-time
- `--tail N, -n N` - Show last N lines (default: 100)

**Features:**

- **Service Logs**: View logs for engine and console services
- **Database Logs**: View logs for system and project databases
- **Real-time Following**: Follow logs as they happen with `--follow` flag
- **Tail Control**: Control how many log lines to show with `--tail` flag
- **Docker & Local Support**: Works seamlessly in both Docker and local service modes
- **Automatic Container Detection**: Automatically finds the correct container for database logs

**Examples:**

```bash
# Database logs
apito logs --db system --tail 10
apito logs --db project --follow
apito logs --db system -f -n 5

# Service logs
apito logs engine --tail 50
apito logs console --follow
apito logs engine -f -n 100

# Default behavior (engine logs, last 100 lines)
apito logs
```

**Output Examples:**

```bash
# Database logs
[INFO] Showing logs for system database (apito-system-postgres):
2025-08-14 05:48:09.188 UTC [1] LOG: database system is ready to accept connections

# Service logs (when services are running)
[INFO] Showing logs for engine service:
# Log output from the engine container
```

### 🗄️ **Database Management**

#### `start --db` - Start Database Services

Start system or project databases with interactive configuration and Docker integration.

**Usage:**

```bash
apito start --db system    # Start system database
apito start --db project  # Start project database
```

**Features:**

- **Interactive Database Selection**: Choose from PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQL Server
- **Credential Options**: Use default credentials or enter custom configuration
- **Smart Configuration**: Automatic detection of existing database settings
- **Overwrite Protection**: Confirmation prompts before overwriting existing configuration
- **Persistent Volumes**: Named Docker volumes for data persistence
- **Consistent Naming**: Container names follow `apito-{dbType}-{engine}` pattern

**Database Engines Supported:**

- **PostgreSQL** (port 5432) - Professional-grade relational database
- **MySQL** (port 3306) - Popular open-source database
- **MariaDB** (port 3307) - MySQL fork with enhanced features
- **MongoDB** (port 27017) - Document-based NoSQL database
- **Redis** (port 6379) - In-memory key-value store
- **SQL Server** (port 1433) - Microsoft's enterprise database

**Configuration Options:**

- **Default Credentials**: Automatic setup with `apito` user, generated password, and `apito` database
- **Custom Configuration**: Full control over host, port, username, password, and database name
- **Engine-Specific Fields**: Special configuration options for databases like MongoDB

**Examples:**

```bash
# Start system database with interactive setup
apito start --db system

# Start project database with interactive setup
apito start --db project

# Start services without database (default behavior)
apito start
```

**Volume Naming Convention:**

- **System Database**: `apito-system-{engine}_data` (e.g., `apito-system-postgres_data`)
- **Project Database**: `apito-project-{engine}_data` (e.g., `apito-project-mysql_data`)

#### `stop --db` - Stop Database Services

Stop specific database services while keeping other services running.

**Usage:**

```bash
apito stop --db system    # Stop system database
apito stop --db project  # Stop project database
```

**Examples:**

```bash
# Stop only the system database
apito stop --db system

# Stop only the project database
apito stop --db project

# Stop all services (default behavior)
apito stop
```

#### `restart --db` - Restart Database Services

Restart specific database services while keeping other services running.

**Usage:**

```bash
apito restart --db system    # Restart system database
apito restart --db project  # Restart project database
```

**Examples:**

```bash
# Restart only the system database
apito restart --db system

# Restart only the project database
apito restart --db project

# Restart all services (default behavior)
apito restart
```

### 🔌 **Plugin Management**

Apito CLI provides a comprehensive plugin management system that allows you to create, deploy, update, and manage HashiCorp-based plugins with hot reload capabilities.

#### `config` - Manage CLI Configuration

Configure accounts, server URLs, cloud sync keys, and other CLI settings for plugin management.

**Usage:**

```bash
apito config <command> [options]
```

**Commands:**

- `set <key> <value>` - Set a configuration value
- `set account <account-name> <url|key> <value>` - Set account-specific configuration
- `get [key]` - Get configuration value(s)
- `init` - Initialize configuration interactively
- `reset` - Reset configuration to defaults

**Configuration Keys:**

- `timeout` - Request timeout in seconds (default: 30)
- `default_plugin` - Default plugin for operations
- `mode` - CLI run mode (docker or manual)
- `default_account` - Default account for plugin operations

**Account Configuration:**

- `url` - Apito server URL for the account
- `key` - Cloud sync key for the account

**Examples:**

```bash
# Interactive configuration setup (creates accounts)
apito config init

# Set account-specific server URL
apito config set account production url https://api.apito.io

# Set account-specific cloud sync key
apito config set account production key abc123...

# Set default account
apito config set default_account production

# View all configuration including accounts
apito config get

# View specific setting
apito config get default_account

# View all accounts
apito config get account

# Reset all configuration
apito config reset
```

#### `account` - Manage Multiple Accounts

Manage multiple Apito accounts for different environments (production, staging, local, etc.).

**Usage:**

```bash
apito account <command> [options]
```

**Commands:**

- `create <account-name>` - Create a new account with interactive setup
- `list` - List all configured accounts
- `select <account-name>` - Set default account for plugin operations
- `test <account-name>` - Test account connection and credentials
- `delete <account-name>` - Delete an account configuration

**Examples:**

```bash
# Create a new account
apito account create production

# List all accounts
apito account list

# Test account connection
apito account test production

# Set default account
apito account select production

# Delete an account
apito account delete staging
```

**Account Workflow:**

```bash
# 1. Create accounts for different environments
apito account create production
apito account create staging
apito account create local

# 2. Configure each account
apito config set account production url https://api.apito.io
apito config set account production key prod-key-123

apito config set account staging url https://staging-api.apito.io
apito config set account staging key staging-key-456

apito config set account local url http://localhost:5050
apito config set account local key local-key-789

# 3. Test account connections
apito account test production
apito account test staging
apito account test local

# 4. Set default account
apito account select production

# 5. Use plugin commands (will use default account)
apito plugin deploy
apito plugin list
```

#### `plugin` - Manage HashiCorp Plugins

Create, deploy, update, and manage HashiCorp-based plugins with hot reload capabilities.

**⚠️ Confirmation Required**: Sensitive operations (`deploy`, `update`, `delete`, `stop`, `restart`) require confirmation and will display detailed information about the operation, including:

- Action being performed
- Plugin ID and version
- Target account and server URL
- Additional plugin details (language, type)

**Usage:**

```bash
apito plugin <command> [options]
```

**Commands:**

- `create` - Create a new plugin scaffold
- `build [directory]` - Build plugin based on language configuration
- `deploy [directory]` - Deploy plugin to server ⚠️ _Requires confirmation_
- `update <plugin-id> [directory]` - Update existing plugin ⚠️ _Requires confirmation_
- `list` - List all plugins on server
- `status <plugin-id>` - Get plugin status
- `delete <plugin-id>` - Delete plugin from server ⚠️ _Requires confirmation_
- `restart <plugin-id>` - Restart plugin ⚠️ _Requires confirmation_
- `stop <plugin-id>` - Stop plugin ⚠️ _Requires confirmation_
- `start <plugin-id>` - Start plugin
- `env` - Show build environment information

**Options:**

- `--account, -a` - Specify account to use for plugin operations (optional, uses default if not specified)
- `--dir, -d` - Plugin directory (for deploy/update commands)

**Features:**

- **Hot Reload**: Deploy and update plugins without server restarts
- **Process Isolation**: HashiCorp go-plugin framework for fault tolerance
- **Multi-language Support**: Go, JavaScript, Python plugin development
- **Secure Deployment**: Cloud sync key authentication
- **Real-time Status**: Live plugin status monitoring and health checks
- **Template Generation**: Scaffold creation with best practices
- **Confirmation System**: Sensitive operations require explicit confirmation with detailed operation information

**Plugin Creation Workflow:**

```bash
# 1. Configure CLI for plugin management
apito config init
# This creates your first account interactively

# 2. Create additional accounts for different environments (optional)
apito account create staging
apito config set account staging url https://staging-api.apito.io
apito config set account staging key staging-key-456

# 3. Set default account
apito account select production

# 4. Create new plugin scaffold
apito plugin create
# Follow prompts to select:
# - Plugin name (e.g., hc-my-awesome-plugin)
# - Language (Go, JavaScript, Python)
# - Plugin type (System, Project, Custom)

# 5. Develop your plugin
cd hc-my-awesome-plugin
# Edit main.go or main.js based on your language choice
# Modify config.yml with plugin metadata

# 6. Build plugin
apito plugin build

# 7. Deploy plugin to server (uses default account)
apito plugin deploy

# 8. Check plugin status
apito plugin status hc-my-awesome-plugin
```

**Plugin Development:**

```bash
# Create plugin scaffold
apito plugin create
# > Enter plugin name: hc-file-processor
# > Select language: Go
# > Select type: System

# Build plugin (automatically detects language)
apito plugin build
# > Choose build method: System Go / Docker
# > (For Go) Choose build type: Debug / Development / Production

# Deploy from current directory (requires confirmation)
apito plugin deploy

# Deploy from specific directory
apito plugin deploy ./path/to/plugin

# Deploy to specific account
apito plugin deploy --account staging
apito plugin deploy -a production

# Update existing plugin (requires confirmation)
apito plugin update hc-file-processor

# List plugins from specific account
apito plugin list --account staging
apito plugin list -a production

# Check plugin status from specific account
apito plugin status hc-file-processor --account staging
```

**Plugin Build System:**

The build system automatically detects the plugin language from `config.yml` and provides appropriate build options:

**Go Plugins:**

```bash
# Build Go plugin with interactive options
apito plugin build
# Options:
# - System vs Docker build
# - Debug (with debug symbols)
# - Development (basic build, GOOS=linux)
# - Production (static binary, CGO_ENABLED=0)

# Examples of actual build commands used:
# Debug: go build -gcflags="all=-N -l" -o hc-plugin-name .
# Development: GOOS=linux go build -o hc-plugin-name .
# Production: CGO_ENABLED=0 go build -ldflags "-s" -a -o hc-plugin-name .
```

**JavaScript Plugins:**

```bash
# Build JavaScript plugin
apito plugin build
# Automatically runs:
# - npm install (install dependencies)
# - node --check index.js (syntax validation)
```

**Python Plugins:**

```bash
# Build Python plugin
apito plugin build
# Automatically runs:
# - pip3 install -r requirements.txt (if exists)
# - python3 -m py_compile main.py (syntax validation)
```

**Build Environment:**

```bash
# Check available build tools
apito plugin env
# Shows status of:
# - Go runtime
# - Node.js runtime
# - Python runtime
# - Docker availability
# - System architecture
```

**Plugin Management:**

```bash
# List all plugins
apito plugin list

# Get detailed plugin status
apito plugin status hc-file-processor

# Control plugin lifecycle (requires confirmation)
apito plugin restart hc-file-processor
apito plugin stop hc-file-processor
apito plugin start hc-file-processor

# Remove plugin (requires confirmation)
apito plugin delete hc-file-processor
```

**Plugin Configuration (`config.yml`):**

```yaml
plugin:
  id: "hc-your-plugin-name"
  language: "go" # go, js, python
  title: "Your Plugin Title"
  icon: "🔌"
  description: "Plugin description"
  type: "system" # system, project, custom
  role: "custom"
  exported_variable: "NormalPlugin"
  enable: true
  debug: false
  version: "1.0.0"
  author: "Your Name"
  repository_url: "https://github.com/your/repo"
  branch: "main"
  binary_path: "hc-your-plugin-name"
  handshake_config:
    protocol_version: 1
    magic_cookie_key: "APITO_PLUGIN"
    magic_cookie_value: "apito_plugin_magic_cookie_v1"
  env_vars:
    - key: "PLUGIN_DEBUG_MODE"
      value: "false"
```

**Plugin Types:**

- **System Plugins**: Core functionality, authentication, storage drivers
- **Project Plugins**: Project-specific business logic and workflows
- **Custom Plugins**: User-defined functionality and integrations

**Supported Languages:**

- **Go**: Native HashiCorp go-plugin support with maximum performance
- **JavaScript**: Node.js-based plugins with npm ecosystem access
- **Python**: Python plugins with pip package management

### 📊 **Monitoring & Observability**

#### **Enhanced Status Monitoring**

The CLI now provides comprehensive visibility into your Apito infrastructure:

- **Service Status**: Monitor engine and console service health
- **Database Status**: Real-time database container status monitoring
- **Automatic Detection**: Database status automatically displayed when available
- **Docker Integration**: Seamless integration with Docker container management

#### **Comprehensive Logging System**

Access logs for all components with powerful filtering and real-time capabilities:

- **Service Logs**: Engine and console application logs
- **Database Logs**: Database container logs with engine-specific formatting
- **Real-time Following**: Follow logs as they happen for live debugging
- **Tail Control**: View specific numbers of log lines for focused analysis
- **Multi-mode Support**: Works in both Docker and local service modes

#### **Monitoring Commands**

```bash
# Check overall system health
apito status

# Monitor specific services
apito status engine
apito status console

# View real-time logs
apito logs --db system --follow
apito logs engine --follow

# Analyze recent activity
apito logs --db project --tail 100
apito logs console --tail 50
```

#### **Use Cases**

- **Development Debugging**: Follow logs in real-time during development
- **Production Monitoring**: Check service health and database status
- **Troubleshooting**: Analyze recent logs for error investigation
- **Performance Analysis**: Monitor database and service performance
- **Deployment Verification**: Confirm services are running correctly after deployment

### 🔧 **Project Management**

#### `create` - Create New Resources

Creates new projects, functions, or models via API calls to the Apito server.

**Usage:**

```bash
apito create <resource> [options]
```

**Resources:**

- `project` - Create a new Apito project via API
- `function` - Create a new function (coming soon)
- `model` - Create a new data model (coming soon)

**Options:**

- `--name, -n` - Name of the resource
- `--project, -p` - Project name (alternative to --name)

**Features:**

- Interactive project creation with prompts
- Database type selection with visual icons
- Automatic SYNC_TOKEN management
- HTTP API integration with authentication
- Real-time project creation on Apito server

**Database Options:**

- **Embed & SQL** (mdi:database) - Default embedded database
- **MySQL** (logos:mysql) - MySQL database
- **MariaDB** (logos:mariadb) - MariaDB database
- **PostgreSQL** (logos:postgresql) - PostgreSQL database
- **Couchbase** (logos:couchbase) - Couchbase database
- **Oracle** (logos:oracle) - Oracle database
- **Firestore** (logos:firebase) - Firebase Firestore
- **MongoDB** (logos:mongodb) - MongoDB database
- **DynamoDB** (logos:aws-dynamodb) - AWS DynamoDB

**Examples:**

```bash
# Create a new project with prompts
apito create project

# Create a project with name flag
apito create project -p my-ecommerce-app

# Create a project with name flag (alternative)
apito create project -n my-ecommerce-app
```

### 🚀 **Development & Execution**

#### `start` - Start Apito Engine and Console

Starts the Apito engine and console with automatic setup and downloads.

**Usage:**

```bash
apito start [--db system|project]
```

**Options:**

- `--db system` - Start system database with interactive setup
- `--db project` - Start project database with interactive setup

**Features:**

- **Run Modes**: Docker (default) or Manual, stored in `~/.apito/config.yml`
- **Docker Mode**: Uses compose with persistent volumes and `.env` mounted inside the engine container
- **Manual Mode**: Downloads binaries, installs Caddy, and manages processes with PID/log files
- **Port Management**: Checks 5050/4000 (Manual mode only)
- **Graceful Shutdown**: Stops all services on Ctrl+C
- **Database Integration**: Optional database startup with `--db` flag

**What it does:**

1. Loads run mode from `~/.apito/config.yml` (defaults to Docker)
2. **Database Setup** (if `--db` flag specified):
   - Interactive database engine selection
   - Credential configuration (default or custom)
   - Docker container creation with persistent volumes
   - Configuration saved to `~/.apito/bin/.env`
3. **Service Startup**:
   - Docker mode: Compose up engine and console with required volumes
   - Manual mode: Port check, download components, start services
4. Waits for interrupt to stop services

**Examples:**

```bash
# Start Apito with automatic setup
apito start

# Start with system database
apito start --db system

# Start with project database
apito start --db project
```

**Access URLs:**

- **Engine API**: http://localhost:5050
- **Console UI**: http://localhost:4000

**System Requirements:**

- Internet connection for downloading components
- Write permissions to `~/.apito/` directory
- Port 5050 and 4000 available
- Docker and Docker Compose (for database features)

#### `stop` - Stop Services

Stops one or more Apito services.

**Usage:**

```bash
apito stop [engine|console|all] [--db system|project]
```

**Options:**

- `--db system` - Stop only the system database
- `--db project` - Stop only the project database

**Examples:**

```bash
# Stop everything
apito stop

# Stop only engine
apito stop engine

# Stop only console
apito stop console

# Stop only system database
apito stop --db system

# Stop only project database
apito stop --db project
```

#### `restart` - Restart Services

Restarts one or more Apito services.

**Usage:**

```bash
apito restart [engine|console|all] [--db system|project]
```

**Options:**

- `--db system` - Restart only the system database
- `--db project` - Restart only the project database

**Examples:**

```bash
# Restart everything
apito restart

# Restart only engine
apito restart engine

# Restart only console
apito restart console

# Restart only system database
apito restart --db system

# Restart only project database
apito restart --db project
```

### 🏗️ **Building & Packaging**

#### `build` - Build Project

Builds your project for different deployment targets.

**Usage:**

```bash
apito build <target> -p <project> [options]
```

**Targets:**

- `docker` - Build Docker image
- `zip` - Create deployment package

**Options:**

- `--project, -p` - Project name (required)
- `--tag, -t` - Docker image tag (optional, for docker builds)

**Examples:**

```bash
# Build Docker image
apito build docker -p my-ecommerce-app
apito build docker -p my-ecommerce-app -t v1.0.0

# Create ZIP package
apito build zip -p my-ecommerce-app
```

### 🔄 **Updates & Maintenance**

#### `update` - Update Components

Update Apito engine, console, or the CLI itself.

**Usage:**

```bash
apito update <engine|console|self> [-v <version>]
```

**Examples:**

```bash
# Update engine to latest version
apito update engine

# Update console to a specific version
apito update console -v v1.2.3

# Update the CLI itself to the latest version
apito update self
```

## 🗂️ Project Structure

Apito CLI sets up the following structure:

```
~/.apito/
├── bin/
│   ├── engine               # Engine binary (Manual mode)
│   ├── caddy                # Caddy binary (Manual mode)
│   └── .env                 # System configuration mounted into engine container
├── engine-data/             # Persistent engine data volume (Docker mode)
├── docker-compose.yml       # Engine + Console compose (Docker mode)
├── db-compose.yml           # Database compose (generated when --db flag used)
├── console/                 # Console static files (Manual mode)
├── Caddyfile                # Console server config (Manual mode)
├── logs/                    # Service log files (Manual mode)
│   ├── engine.log          # Engine service logs
│   └── console.log         # Console service logs
├── run/                     # Process PID files (Manual mode)
│   ├── engine.pid
│   └── console.pid
└── config.yml               # CLI config (mode, plugin server URL, cloud sync key)
```

**Logging & Monitoring:**

- **Service Logs**: Local service logs stored in `~/.apito/logs/` (Manual mode)
- **Container Logs**: Docker container logs accessible via `apito logs` command
- **Database Logs**: Database container logs with real-time following capability
- **Status Monitoring**: Real-time service and database status via `apito status`

## ⚙️ Configuration

### Environment Variables

The CLI manages two types of configuration:

#### **CLI Configuration (`~/.apito/config.yml`)**

- `mode` - CLI run mode (docker or manual)
- `default_account` - Default account for plugin operations
- `timeout` - Request timeout in seconds (default: 30)
- `default_plugin` - Default plugin for operations
- `accounts` - Account configurations map
  - `account_name` - Account configuration
    - `server_url` - Apito server URL for the account
    - `cloud_sync_key` - Authentication key for the account

#### **Engine Configuration (`~/.apito/bin/.env`)**

The CLI manages engine configuration in `~/.apito/bin/.env`:

#### **System Configuration:**

- `ENVIRONMENT` - local/development/staging/production
- `COOKIE_DOMAIN` - e.g., localhost
- `CORS_ORIGIN` - e.g., http://localhost:4000
- `BRANKA_KEY` - Generated secret key
- `APITO_SYSTEM_DB_ENGINE` - embed, postgres, mysql, mariadb, mongodb, redis, sqlserver
- `SYSTEM_DB_HOST` - System database host
- `SYSTEM_DB_PORT` - System database port
- `SYSTEM_DB_NAME` - System database name
- `SYSTEM_DB_USER` - System database username
- `SYSTEM_DB_PASSWORD` - System database password
- `SYSTEM_DATABASE_URL` - Complete system database connection string

#### **Project Configuration:**

- `APITO_PROJECT_DB_ENGINE` - postgres, mysql, mariadb, mongodb, redis, sqlserver
- `PROJECT_DB_HOST` - Project database host
- `PROJECT_DB_PORT` - Project database port
- `PROJECT_DB_NAME` - Project database name
- `PROJECT_DB_USER` - Project database username
- `PROJECT_DB_PASSWORD` - Project database password
- `PROJECT_DATABASE_URL` - Complete project database connection string

#### **Service Configuration:**

- `SERVE_PORT` - Engine port (default 5050)
- `CACHE_*`, `KV_ENGINE`, `AUTH_SERVICE_PROVIDER`, `TOKEN_TTL`
- `CADDY_PATH` - Absolute path to caddy (managed by CLI)

### Database Support

#### **System Database:**

- **Embedded**: boltDB (default)
- **External**: PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQL Server

#### **Project Database:**

- **External**: PostgreSQL, MySQL, MariaDB, MongoDB, Redis, SQL Server
- **Cloud**: Firestore (alpha), DynamoDB (alpha)

### Docker Integration

#### **Container Naming Convention:**

- **System Database**: `apito-system-{engine}` (e.g., `apito-system-postgres`)
- **Project Database**: `apito-project-{engine}` (e.g., `apito-project-mysql`)

#### **Volume Naming Convention:**

- **System Database**: `apito-system-{engine}_data` (e.g., `apito-system-postgres_data`)
- **Project Database**: `apito-project-{engine}_data` (e.g., `apito-project-mysql_data`)

#### **Docker Requirements:**

- Docker Engine 20.10+
- Docker Compose v2 (recommended) or v1
- Docker daemon running

## 🚨 Troubleshooting

### Common Issues

**Permission Denied Error:**

```bash
# The installer will automatically handle permissions
# If you encounter issues, run with sudo:
sudo ./install.sh
```

**Docker Not Available:**

```bash
# Check Docker installation
docker --version

# Check Docker Compose
docker compose version

# Check Docker daemon status
docker info

# Install Docker if needed:
# macOS: https://docs.docker.com/desktop/install/mac-install/
# Linux: https://docs.docker.com/engine/install/
```

**Database Connection Issues:**

```bash
# Check database container status
docker ps | grep apito

# View database logs using the new logs command
apito logs --db system --tail 20
apito logs --db project --follow

# View database logs directly (alternative)
docker logs apito-system-postgres

# Check volume persistence
docker volume ls | grep apito

# Restart database service
apito restart --db system
```

**Project Not Found:**

```bash
# List all projects to see available ones
apito list

# Create a new project if needed
apito create project -n my-new-project
```

**Engine or Console Won't Start:**

```bash
# Check if services are running and view logs
apito status

# Stop services
apito stop

# Then try running again
apito start
```

**Configuration Overwrite Issues:**

```bash
# The CLI now asks for confirmation before overwriting
# If you need to reset configuration:
rm ~/.apito/bin/.env
apito init
```

**Logs and Monitoring Issues:**

```bash
# Check if database containers are running
apito status

# View real-time database logs
apito logs --db system --follow
apito logs --db project --follow

# View last N lines of logs
apito logs --db system --tail 50
apito logs --db project --tail 50

# Check service logs
apito logs engine --tail 100
apito logs console --follow

# If logs command fails, check container status
docker ps --filter "name=apito-"
```

**Volume Management:**

```bash
# List all apito volumes
docker volume ls | grep apito

# Backup a specific database volume
docker run --rm -v apito-system-postgres_data:/data -v $(pwd):/backup alpine tar czf /backup/postgres_backup.tar.gz -C /data .

# Remove a specific database volume (after stopping container)
docker volume rm apito-system-postgres_data
```

**Plugin Management Issues:**

```bash
# Plugin configuration issues
apito config get                     # Check CLI configuration
apito account list                   # List all accounts
apito config init                    # Reconfigure CLI settings

# Plugin deployment failures
apito plugin list                    # Check server connectivity
apito plugin status <plugin-id>     # Check specific plugin status

# Account-related issues
apito account create <name>          # Create new account
apito account test <name>            # Test account connection
apito account select <name>          # Set default account
apito config set account <name> url <url>  # Set account URL
apito config set account <name> key <key>  # Set account key

# Plugin server connection issues
curl -H "Authorization: Bearer <sync-key>" \
     https://your-server.com/plugin/v2/health

# Check plugin configuration file
cat hc-your-plugin/config.yml       # Validate YAML syntax
```

**Plugin Development Issues:**

```bash
# Plugin creation failures
apito plugin create                  # Try creating new scaffold

# Build environment issues
apito plugin env                     # Check available build tools

# Build issues (for Go plugins)
cd hc-your-plugin
apito plugin build                   # Use CLI build system
# Alternative manual build:
go mod tidy                         # Fix dependencies
go build -o hc-your-plugin .        # Test local build

# Build issues (for JavaScript plugins)
cd hc-your-plugin-js
npm install                         # Install dependencies manually
node --check index.js               # Check syntax
apito plugin build                  # Use CLI build system

# Build issues (for Python plugins)
cd hc-your-plugin-py
pip3 install -r requirements.txt    # Install dependencies manually
python3 -m py_compile main.py       # Check syntax
apito plugin build                  # Use CLI build system

# Docker build issues
docker --version                    # Check Docker availability
apito plugin build                  # Select system build if Docker fails

# Plugin deployment authentication
apito config set cloud_sync_key <your-key>  # Update auth key
apito config set server_url <your-url>      # Update server URL
```

**Plugin Runtime Issues:**

```bash
# Plugin not starting
apito plugin restart <plugin-id>    # Restart plugin
apito plugin status <plugin-id>     # Check error messages

# Plugin performance issues
apito plugin stop <plugin-id>       # Stop plugin
apito plugin start <plugin-id>      # Start plugin fresh

# Account switching for different environments
apito account test staging          # Test staging account first
apito plugin deploy --account staging  # Deploy to staging (requires confirmation)
apito plugin deploy -a production      # Deploy to production (requires confirmation)

# Or switch default account
apito account select staging        # Switch to staging account
apito plugin deploy                 # Deploy to staging (uses default, requires confirmation)
apito account select production     # Switch back to production
apito plugin deploy                 # Deploy to production (uses default, requires confirmation)

# Plugin logs (server-side)
# Check your Apito server logs for plugin-specific errors
```

## 🔗 Useful Links

- **Apito Platform**: https://app.apito.io
- **Documentation**: https://docs.apito.io
- **GitHub**: https://github.com/apito-io/cli
- **Support**: https://github.com/apito-io/cli/issues

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Happy coding with Apito! 🎉**
