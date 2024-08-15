# Apito CLI

Apito CLI is a command-line tool to manage projects, functions, and more for the Apito platform.

## Installation

To install apito cli run the following command ( Linux & MacOS )
```sh
curl -sSL https://get.apito.io/cli/install.sh | sh
```

## Commands

### `create`

Create a new project, function, or model.

- **Usage:**
  ```sh
  apito create project -n <projectName>

- **Options**:
    - `--name, -n` : The name for the function or model (optional).

- **Examples**:
    ```sh
    apito create project -n myApp

### `list`

List projects or functions.

- **Usage:**
  ```sh
  apito list [--project <projectName>]

- **Options**:
    - `--project, -p` : The project name (optional).

- **Examples**:
    ```sh
    apito list
    apito list --project myApp -f

### `deploy`
Deploy the project to Apito Cloud

- **Usage:**
  ```sh
  apito deploy --project <projectName> 
  
- **Options**:
    - `--project, -p` : The project name (required).
    - `--cloud, -c` : Support for Other Cloud is Coming Soon.

- **Examples**:
    ```sh
    apito deploy --project myApp

### `pack`
Deploy the project to a specified provider.

- **Usage:**
  ```sh
  apito pack --project <projectName> --provider <provider> [--tag <dockerTag>]
  
- **Options**:
    - `--project, -p` : The project name (required).
    - `--provider` : The deployment provider (docker/zip) (required).
    - `--tag` : Docker image tag (optional, for Docker provider).

- **Examples**:
    ```sh
    apito pack --project myApp --provider docker
    apito pack --project myApp --provider docker --tag customTag
    apito pack --project myApp --provider zip

### `change-pass`
Change the password for a user.

- **Usage:**
- **Options**:
    - `--project, -p` : The project name (required).
    - `--user, -u` : The username (required).

- **Examples**: 
    ```sh
    apito change-pass --project myApp --user admin

## Additional Information

- The CLI saves configuration files in the ~/.apito directory, similar to how GitHub saves its configuration.
- The create command will prompt for necessary details and save configuration in a .env file in the project directory.
- The deploy command will automatically detect the runtime environment and download the appropriate release asset from the Apito GitHub repository.