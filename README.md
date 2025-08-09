<p align="center">
  <img src="https://raw.githubusercontent.com/apito-io/engine/main/docs/cover-photo.png" alt="Apito Logo" />
</p>

# Apito CLI

🚀 **Apito CLI** is a powerful command-line tool for managing projects, functions, and deployments on the Apito platform. It provides a seamless development experience from local development to cloud deployment.

<p align="center">
  <a href="https://apito.io"><strong>Website</strong></a> ·
  <a href="https://docs.apito.io"><strong>Documentation</strong></a> ·
  <a href="https://discord.com/invite/fwHgF8pUpt"><strong>Discord</strong></a>
</p>

<p align="center">
  <a href="https://github.com/apito-io/engine/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
  </a>
  <a href="https://github.com/apito-io/engine/releases">
    <img src="https://img.shields.io/github/v/release/apito-io/cli" alt="Release" />
  </a>
  <a href="https://goreportcard.com/report/github.com/apito-io/cli">
    <img src="https://goreportcard.com/badge/github.com/apito-io/cli" alt="Go Report Card" />
  </a>
  <a href="https://github.com/apito-io/engine/actions">
    <img src="https://github.com/apito-io/engine/workflows/Build%20and%20Release/badge.svg" alt="Build Status" />
  </a>
</p>


## 📦 Installation

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
- If Docker is selected:
  - Generate `~/.apito/docker-compose.yml` (engine + console)
  - Optionally spin up a database via `~/.apito/db-compose.yml` (Postgres/MySQL/MariaDB/SQLServer/MongoDB), or skip if you already have one
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
apito start [--db]
```

This command will (based on run mode stored in `~/.apito/config.yml`):

- Docker mode (default, recommended):
  - Ensure `~/.apito/docker-compose.yml` exists (engine + console)
  - Mount `~/.apito/engine-data -> /go/src/gitlab.com/apito.io/engine/db` and `~/.apito/bin/.env -> /go/src/gitlab.com/apito.io/engine/.env`
  - Start services via `docker compose -f ~/.apito/docker-compose.yml up -d`
  - Optional: `--db` prompts you to select and start a database using `~/.apito/db-compose.yml`
- Manual mode:
  - Download the latest Apito engine to `~/.apito/bin/engine`
  - Download the latest console to `~/.apito/console`
  - Install and configure Caddy to `~/.apito/bin/caddy`
  - Check and free ports 5050/4000 if needed
  - Start engine and serve console locally (managed by PID + logs)

### 3. Deploy to Apito Cloud

```bash
apito deploy -p my-awesome-app
```

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
- Validates system database configuration
- Checks mandatory environment variables (ENVIRONMENT, CORS_ORIGIN, COOKIE_DOMAIN, BRANKA_KEY)
- Validates database connection settings (host, port, user, password)
- Checks port availability (5050, 4000)
- Interactive configuration prompts for missing settings

**What it checks:**

- System database engine configuration (defaults to "embed")
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

**SYNC_TOKEN Setup:**

The first time you create a project, you'll need to provide a SYNC_TOKEN:

1. Go to http://localhost:4000
2. Navigate to Cloud Sync option
3. Copy the generated token
4. Paste it when prompted by the CLI

The token is automatically saved and reused for future requests.

#### `list` - List Resources

Lists projects or functions.

**Usage:**

```bash
apito list [resource] [-p <project>]
```

**Resources:**

- `function` - List functions in a project
- (no resource) - List all projects

**Options:**

- `--project, -p` - Project name (required for listing functions)

**Examples:**

```bash
# List all projects
apito list

# List functions in a specific project
apito list function -p my-ecommerce-app
```

### 🚀 **Development & Execution**

#### `start` - Start Apito Engine and Console

Starts the Apito engine and console with automatic setup and downloads.

**Usage:**

```bash
apito start [--db]
```

**Options:**

- `--db` - Prompt to start a database in Docker mode before services

**Features:**

- **Run Modes**: Docker (default) or Manual, stored in `~/.apito/config.yml`
- **Docker Mode**: Uses compose with persistent volumes and `.env` mounted inside the engine container
- **Manual Mode**: Downloads binaries, installs Caddy, and manages processes with PID/log files
- **Port Management**: Checks 5050/4000 (Manual mode only)
- **Graceful Shutdown**: Stops all services on Ctrl+C

**What it does:**

1. Loads run mode from `~/.apito/config.yml` (defaults to Docker)
2. Docker mode:
   - Compose up engine and console with required volumes
   - Optional `--db` to bring up a local database compose
3. Manual mode:
   - Port check and optional freeing
   - Download engine/console, install Caddy
   - Start engine and serve console
4. Waits for interrupt to stop services

**Examples:**

```bash
# Start Apito with automatic setup
apito start
```

**Access URLs:**

- **Engine API**: http://localhost:5050
- **Console UI**: http://localhost:4000

**System Requirements:**

- Internet connection for downloading components
- Write permissions to `~/.apito/` directory
- Port 5050 and 4000 available

#### `stop` - Stop Services

Stops one or more Apito services.

**Usage:**

```bash
apito stop [engine|console|all]
```

**Examples:**

```bash
# Stop everything
apito stop

# Stop only engine
apito stop engine

# Stop only console
apito stop console
```

#### `restart` - Restart Services

Restarts one or more Apito services.

**Usage:**

```bash
apito restart [engine|console|all]
```

**Examples:**

```bash
apito restart
apito restart engine
apito restart console
```

#### `status` - Show Service Status and Logs

Shows whether services are running and prints the last 50 log lines.

**Usage:**

```bash
apito status [engine|console]
```

**Examples:**

```bash
apito status
apito status engine
apito status console
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

### ☁️ **Deployment**

#### `deploy` - Deploy to Apito Cloud

Deploys your project to Apito Cloud platform.

**Usage:**

```bash
apito deploy -p <project>
```

**Options:**

- `--project, -p` - Project name (required)

**Examples:**

```bash
apito deploy -p my-ecommerce-app
```

**Features:**

- Interactive deployment token setup
- Automatic cloud configuration
- Real-time deployment status

#### `pack` - Package for Deployment

Packages your project for various deployment providers.

**Usage:**

```bash
apito pack <provider> -p <project> [options]
```

**Providers:**

- `apito` - Package for Apito Cloud
- `aws` - Package for AWS (coming soon)
- `google` - Package for Google Cloud (coming soon)

**Options:**

- `--project, -p` - Project name (required)
- `--tag` - Docker image tag (optional)

**Examples:**

```bash
# Package for Apito Cloud
apito pack apito -p my-ecommerce-app

# Package for AWS (when available)
apito pack aws -p my-ecommerce-app
```

### 🔄 **Updates & Maintenance**

#### `update` - Update Components

Updates Apito engine or console to the latest version.

**Usage:**

```bash
apito update <component> -p <project> [options]
```

**Components:**

- `engine` - Update the Apito engine
- `console` - Update the console interface

**Options:**

- `--project, -p` - Project name (required)
- `--version, -v` - Specific version to update to (optional)

**Examples:**

```bash
# Update engine to latest version
apito update engine -p my-ecommerce-app

# Update to specific version
apito update engine -p my-ecommerce-app -v v1.2.3

# Update console
apito update console -p my-ecommerce-app
```

### 🔐 **Authentication & Security**

#### `login` - Authenticate with Apito

Logs in to your Apito account using OAuth.

**Usage:**

```bash
apito login
```

**Features:**

- OAuth-based authentication
- Automatic browser opening
- Token management

#### `change-pass` - Change User Password

Changes the password for a user in your project.

**Usage:**

```bash
apito change-pass -p <project> -u <user>
```

**Options:**

- `--project, -p` - Project name (required)
- `--user, -u` - Username (required)

**Examples:**

```bash
apito change-pass -p my-ecommerce-app -u admin
```

**Features:**

- Secure password input with masking
- Password confirmation
- Minimum password length validation (6 characters)

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
├── db-compose.yml           # Optional database compose (Docker mode)
├── console/                 # Console static files (Manual mode)
├── Caddyfile                # Console server config (Manual mode)
├── logs/
│   ├── engine.log
│   └── console.log
├── run/
│   ├── engine.pid
│   └── console.pid
└── config.yml               # CLI config (e.g., mode: docker|manual)
```

## ⚙️ Configuration

### Environment Variables

The CLI manages configuration in `~/.apito/bin/.env`:

- `ENVIRONMENT` - local/development/staging/production
- `COOKIE_DOMAIN` - e.g., localhost
- `CORS_ORIGIN` - e.g., http://localhost:4000
- `BRANKA_KEY` - Generated secret key
- `APITO_SYSTEM_DB_ENGINE` - embed or external
- `SYSTEM_DB_*` - connection parameters when external
- `APITO_PROJECT_DB_ENGINE` - embedded by default
- `SERVE_PORT` - Engine port (default 5050)
- `CACHE_*`, `KV_ENGINE`, `AUTH_SERVICE_PROVIDER`, `TOKEN_TTL`
- `CADDY_PATH` - Absolute path to caddy (managed by CLI)

### Database Support

- **System Database**: boltDB (default), PostgreSQL, MySQL
- **Project Database**: PostgreSQL, MySQL, MariaDB, Firestore (alpha)

## 🚨 Troubleshooting

### Common Issues

**Permission Denied Error:**

```bash
# The installer will automatically handle permissions
# If you encounter issues, run with sudo:
sudo ./install.sh
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

**Deployment Fails:**

```bash
# Ensure you have a valid deploy token
# Get it from https://app.apito.io
apito deploy -p my-project
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
