# sdk-tools

A comprehensive CLI toolkit for streamlining development workflows on the IOTA SDK platform when working with Claude Code.

## Features

- **Authentication**: OAuth login and token management
- **Git Workflow**: Branch checking, commit categorization, pre-commit validation
- **Pull Requests**: Analyze PR context (modules, services, migrations)
- **CI/CD Integration**: View failed CI runs and error logs
- **CHANGELOG Management**: Automatic requirement analysis and entry management
- **GraphQL**: Execute GraphQL queries against APIs

## Installation

```bash
cd .claude/tools
go install .
```

This installs the `sdk-tools` binary to `$GOPATH/bin/sdk-tools`. Make sure `$GOPATH/bin` is in your PATH.

## Usage

```bash
sdk-tools --help
```

### Commands

#### Authentication

```bash
# Login with preset credentials
sdk-tools auth login --port 3000
```

#### Git Utilities

```bash
# Categorize changed files by type
sdk-tools git changes
sdk-tools git changes --json
sdk-tools git changes --only backend,tests

# Check current branch protection status
sdk-tools git check-branch
sdk-tools git check-branch --strict  # Exit with code 1 if on protected branch

# Run pre-commit validation checks
sdk-tools git precommit
sdk-tools git precommit --fix  # Automatically fix issues
```

#### Pull Request Analysis

```bash
# Analyze PR context (changed modules, services, migrations)
sdk-tools pr context
sdk-tools pr context --base main
sdk-tools pr context --json
```

#### CI/CD Analysis

```bash
# Show CI failures for current branch
sdk-tools ci failures
sdk-tools ci failures --run-id 12345
sdk-tools ci failures --json
```

#### CHANGELOG Management

```bash
# Check if CHANGELOG update is required
sdk-tools changelog check
sdk-tools changelog check --json
sdk-tools changelog check --explain

# Add new entry
sdk-tools changelog add --title "Feature" --description "Description here"
sdk-tools changelog add --title "Feature" --description "Description" --dry-run

# List recent entries
sdk-tools changelog list
sdk-tools changelog list --count 5
sdk-tools changelog list --json

# Validate CHANGELOG format
sdk-tools changelog validate
sdk-tools changelog validate --json
```

#### GraphQL

```bash
# Execute GraphQL query
sdk-tools gql --query "{ viewer { login } }" --token your_token
sdk-tools gql --file query.gql --token your_token
```

## Architecture

### Directory Structure

```
.claude/tools/
├── main.go              # Entry point
├── go.mod
├── go.sum
├── cmd/                 # Command implementations
│   ├── root.go
│   ├── auth/
│   ├── git/
│   ├── pr/
│   ├── ci/
│   ├── gql/
│   └── changelog/
└── internal/            # Internal packages
    ├── auth/
    ├── changelog/
    ├── git/
    ├── github/
    ├── graphql/
    ├── output/
    └── precommit/
```

### Design Patterns

- **Cobra Commands**: All commands use Cobra for CLI structure
- **JSON Output**: All commands support `--json` flag for machine-readable output
- **Color Output**: Text output uses fatih/color for terminal colors
- **Error Handling**: Consistent error wrapping with context

## Development

### Building

```bash
cd .claude/tools
go build -o sdk-tools .
```

### Testing

```bash
cd .claude/tools
go test ./...
```

### Adding New Commands

1. Create a new package under `cmd/`
2. Define command in `cmd/{feature}/{feature}.go`
3. Implement subcommands as needed
4. Register in `cmd/root.go`

### Adding New Internal Packages

1. Create new package under `internal/`
2. Implement functionality
3. Import and use in commands

## Configuration

Configuration file can be placed at `~/.sdk-tools.yaml`:

```yaml
# Example configuration
github:
  token: your_token_here
api:
  url: https://api.example.com
```

Or use `--config` flag:

```bash
sdk-tools --config /path/to/config.yaml git changes
```

## Integration with Claude Code

The `sdk-tools` CLI integrates with Claude Code workflows by:

1. Automating git validation and categorization
2. Providing CHANGELOG requirement analysis
3. Analyzing CI failures for faster debugging
4. Supporting JSON output for machine-readable results

## Dependencies

- github.com/spf13/cobra - CLI framework
- github.com/spf13/viper - Configuration management
- github.com/fatih/color - Terminal colors

## License

See LICENSE file in project root.
