# Apito CLI

Apito CLI is a command-line tool to manage projects, functions, and more for the Apito platform.

## Commands

### `create`

Create a new project, function, or model.

- **Usage:**
  ```sh
  apito create --project <projectName>
  apito create --project <projectName> --name <functionName> [--function]
  apito create --project <projectName> --name <modelName> [--model]

- **Options**:
    - `--project, -p` : The project name (required).
    - `--name, -n` : The name for the function or model (optional).
    - `--function` : Specify if creating a function (optional).
    - `--model` : Specify if creating a model (optional).

- **Examples**:
    ```sh
    apito create --project myApp
    apito create --project myApp --name createInvoice --function
    apito create --project myApp --name testModel --model

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
    apito list --project myApp

### `login`
Login to Apito CLI using OAuth.

- **Usage:**
  ```sh
  apito login
  

### `change-pass`
Change the password for a user.

- **Usage:**
- **Options**:
    - `--project, -p` : The project name (required).
    - `--user, -u` : The username (required).

- **Examples**: 
    ```sh
    apito change-pass --project myApp --user admin

### `deploy`
Deploy the project to a specified provider.

- **Usage:**
  ```sh
  apito deploy --project <projectName> --provider <provider> [--tag <dockerTag>]
  
- **Options**:
    - `--project, -p` : The project name (required).
    - `--provider` : The deployment provider (docker/zip/aws/google) (required).
    - `--tag` : Docker image tag (optional, for Docker provider).

- **Examples**:
    ```sh
    apito deploy --project myApp --provider docker
    apito deploy --project myApp --provider docker --tag customTag
    apito deploy --project myApp --provider zip

## Additional Information

- The CLI saves configuration files in the ~/.apito directory, similar to how GitHub saves its configuration.
- The create command will prompt for necessary details and save configuration in a .env file in the project directory.
- The deploy command will automatically detect the runtime environment and download the appropriate release asset from the Apito GitHub repository.