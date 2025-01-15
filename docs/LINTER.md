# Linter Documentation

This document describes the linting tools and targets available in the project.

## Available Linter Tools

### 1. golangci-lint
The project uses `golangci-lint` as the primary Go code linter. Configuration for this linter can be found in `.golangci.yml`.

To run the standard Go linter:
```bash
make lint
```

### 2. iotalinter
The project includes a custom JSON linter called `iotalinter`. This tool is designed specifically for this project's needs.

#### Configuration Details
The iotalinter configuration in `.golangci.yml` includes:
```yaml
linters-settings:
  iotalinter:
    exclude-dirs:
      - apex
      - test
      - .vscode
    check-zero-byte-files: true
```
Available make targets for iotalinter:

```bash
# Build the JSON linter
make build-iota-linter

# Run the JSON linter
make run-iota-linter

# Clean built binaries
make clean-iota-linter
```

## Usage

1. To run all linters as part of the setup process:
```bash
make setup
```

2. To run only the Go linter:
```bash
make lint
```

3. To use the iotalinter:
```bash
# First, build the linter
make build-iota-linter

# Then run it
make run-iota-linter
```

The iotalinter will scan all JSON files in the project directory and its subdirectories for validation, excluding the directories specified in the configuration.

## Cleaning Up

To clean the iotalinter binary:
```bash
make clean-iota-linter
``` 