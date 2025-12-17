# Claude Code Commands Reference

Complete reference for creating and managing slash commands in Claude Code.
Always fetch official documentation: https://docs.claude.com/en/docs/claude-code/slash-commands

### File References with @

Commands can reference files using the `@` prefix:

```markdown
## Analyze This File

Review the code in @src/components/Button.tsx and suggest improvements.
```

Benefits:

- Faster command execution
- Easier to share and review
- Direct file content loading. No need for agent to execute a separate tool call using `Read` tool

### Command Organization

Commands can be organized in subdirectories:

```
.claude/commands/
├── deployment/
│   ├── staging.md
│   └── production.md
├── testing/
│   ├── unit.md
│   └── integration.md
└── review.md
```

## Guidelines for Creating Commands

### Naming

- Use clear, descriptive names (kebab-case)
- Write clear `description`: "When this command should be used"

**NEVER use these reserved/built-in command names (unless namespaced under a subfolder):**

`/add-dir`, `/agents`, `/bug`, `/clear`, `/compact`, `/config`, `/cost`, `/doctor`, `/help`, `/init`, `/login`,
`/logout`, `/mcp`, `/memory`, `/model`, `/permissions`, `/pr_comments`, `/review`, `/rewind`, `/status`,
`/terminal-setup`, `/usage`, `/vim`

**Best Practice:** Use descriptive, project-specific names like `/manage-config`, `/check-quality`, `/deploy-staging`,
etc.

### Context Loading

- Use dynamic context loading (!`command`) instead of static data
- Support arguments with `$ARGUMENTS`, `$1`, `$2` when useful

### Token Efficiency & Dynamic Context

**CRITICAL: Test all commands before incorporating them into slash commands.**

Dynamic context (!`command`) directly injects output into the conversation context. Verbose or unfiltered commands waste
tokens and degrade performance.

**Requirements:**

1. **Test First**: Run commands manually to verify output is concise and useful
2. **Limit Output**: Use `head`, `tail`, `grep`, `awk`, `wc -l`, or command-specific flags
3. **Avoid Verbosity**: Never use `-v`, `--verbose`, `--debug` flags in dynamic context
4. **Be Specific**: Query exactly what you need, not everything available

**Examples:**

```markdown
# BAD - Dumps entire file list (could be thousands of lines)
!`find . -name "*.go"`

# GOOD - Limited, counted output
!`find . -name "*.go" | wc -l` (123 Go files found)
!`find . -name "*.go" | head -20` (showing first 20)

# BAD - Verbose test output clutters context
!`go test -v ./...`

# GOOD - Summary only
!`go test ./... 2>&1 | grep -E "(PASS|FAIL|ok|FAIL)"`

# BAD - Entire git log
!`git log`

# GOOD - Recent commits only
!`git log --oneline -10`

# BAD - Full status with all changes
!`git status`

# GOOD - Branch and summary only
!`git status -sb`
```

**Multi-Step Workflows:**

When chaining commands, aggregate output at each step:

```markdown
# BAD - Each step dumps full output
!`find . -name "*_test.go"`
!`find . -name "*_controller.go"`
!`find . -name "*_service.go"`

# GOOD - Single aggregated summary
!`echo "Tests: $(find . -name "*_test.go" | wc -l), Controllers: $(find . -name "*_controller.go" | wc -l), Services: $(find . -name "*_service.go" | wc -l)"`
```

**Research Before Adding:**

- Read tool documentation for output-limiting flags
- Check if tools have `--quiet`, `--summary`, `--short` options
- Test with realistic data volumes, not toy examples
- Measure token impact of dynamic context additions

### Tool Permissions

**IMPORTANT: Permission Hierarchy**

Commands can **only restrict** tools already allowed in `settings.json` or `settings.local.json`, never grant new
permissions:

- **Settings files define the ceiling**: These establish the maximum set of allowed tools for the session
- **Commands can only subset**: `allowed-tools` in a command can only restrict from what's in settings
- **Omitting `allowed-tools`**: If not specified, the command inherits all tools from settings

**Example:**

```yaml
# settings.json allows: [Read, Write, Edit, Bash(git:*)]

# VALID - Command restricts to subset
allowed-tools: |
  Read, Edit

# INVALID - Command tries to expand permissions
allowed-tools: |
  Read, Bash(*)  # Bash(*) not in settings, will be ignored

# VALID - Inherits all from settings
# (omit allowed-tools key entirely)
```

**Recommended for file operations:**

- `Read, Write, Edit, Glob, Grep` - File operations
- `Bash(git:*)`, `Bash(make:*)` - Specific patterns only

**Avoid for project commands:**

- `Bash(ls:*)`, `Bash(cat:*)`, `Bash(sed:*)` - Prefer Read/Glob/Grep tools
- `Bash(*)` - Security risk

**Note:** Meta-commands like `/manage-config` may need `ls/sed/cat` for dynamic context loading, but only if these are
already allowed in settings

## Frontmatter Options

Commands use YAML frontmatter to configure behavior:

| Field                      | Description                                                                                                 | Default                             |
|----------------------------|-------------------------------------------------------------------------------------------------------------|-------------------------------------|
| `allowed-tools`            | List of tools the command can use (can only restrict tools already allowed in settings, never grant new)    | Inherits from conversation settings |
| `description`              | Brief description shown in command list and autocomplete                                                    | Uses first line from command prompt |
| `argument-hint`            | Arguments expected for the command (e.g., `<module> [tests\|coverage\|all]`)                                | None                                |
| `model`                    | Specific model to use (e.g., `haiku`, `sonnet`, `opus`)                                                     | Inherits from conversation          |
| `disable-model-invocation` | Prevents SlashCommand tool from automatically invoking this command (for meta-commands or manual execution) | `false`                             |

**Key Points:**

- Only `allowed-tools` restricts permissions (see § Tool Permissions above)
- `argument-hint` appears during autocomplete to guide users
- `model` override useful for simple commands that can use faster/cheaper models
- `disable-model-invocation` used for commands that generate other commands or configs

## Example: Comprehensive Module Analysis Command

This example demonstrates all key features of slash commands in a single, practical command:

```markdown
---
allowed-tools: |
  Read, Grep, Glob, Bash(go:*)
description: "Analyze Go module $1 with optional focus: tests, coverage, or all"
argument-hint: "<module-name> [tests|coverage|all]"
model: sonnet
---

## Module Analysis: $1

### File Structure

!`echo "Go files: $(find modules/$1 -type f -name "*.go" | wc -l), Test files: $(find modules/$1 -name "*_test.go" | wc -l)"`

Sample files: !`find modules/$1 -type f -name "*.go" | head -10`

### Test Execution (if $2 includes "tests" or "all")

!`cd modules/$1 && go test ./... 2>&1 | tail -20`

### Coverage Report (if $2 includes "coverage" or "all")

!`cd modules/$1 && go test -cover ./... | grep -E "(coverage|ok|FAIL)"`

## Analysis Instructions

Analyze the module structure, review test coverage, and provide:

1. Architecture overview from file organization
2. Test coverage assessment and gaps
3. Code quality recommendations
4. Suggested improvements
```

**Features Demonstrated:**

- **Frontmatter Fields**:
    - `allowed-tools`: Restricts to Read, Grep, Glob, Bash(go:*)
    - `description`: Clear purpose shown in command list
    - `argument-hint`: Shows expected arguments in autocomplete
    - `model`: Uses sonnet for analysis (could use haiku for simpler tasks)
- **Dynamic Context**: `!shell command` loads fresh data at execution time
- **Positional Arguments**: `$1` (required module name), `$2` (optional analysis type)
- **Multiple Contexts**: File listing, test discovery, test execution, coverage analysis
- **Conditional Execution**: Different commands based on `$2` argument value
- **Minimal Permissions**: Only tools needed for the specific task
- **File References**: Can add `@modules/$1/service.go` to pre-load specific files

**Usage Examples:**

- `/analyze-module logistics` - Basic analysis with file structure
- `/analyze-module logistics tests` - Include test execution
- `/analyze-module logistics coverage` - Include coverage report
- `/analyze-module logistics all` - Full analysis with tests and coverage

## Best Practices

1. Single Purpose: Each command should have one clear goal
2. Dynamic Context: Always prefer `!\`command\`` over hardcoded data
3. Token Efficiency: Test commands first, limit output, avoid verbose flags (see § Token Efficiency & Dynamic Context)
4. Minimal Permissions: Grant only the tools needed for the task
5. Clear Description: Make it obvious when the command should be used
6. Argument Support: Use `$1`, `$2` for clarity over `$ARGUMENTS`
7. Documentation: Include examples and expected behavior

## Working Directory & Environment Variables

### Dynamic Context Execution Directory

Commands with dynamic context (`!`command``) execute in Claude's current working directory, which may not be your
project root. Always use explicit paths:

**Recommended patterns:**

```markdown
# GOOD - Relative path from project root
!`bash ./.claude/scripts/analyze.sh`

# GOOD - Explicit cd with proper quoting
!`cd back && make test 2>&1 | tail -20`

# BAD - Assumes current directory
!`bash scripts/analyze.sh`
```

**Important:** Each `!`command`` runs in a subshell - directory changes don't persist between commands.

### Environment Variables in Commands

Slash commands do NOT have direct access to hook-specific variables like `$CLAUDE_PROJECT_DIR`. These are only available
in hooks (see `.claude/guides/claude-code/settings.md` § Hook Environment Variables).

**Available in commands:**

- **Argument placeholders:** `$1`, `$2`, `$ARGUMENTS` (command arguments, not env vars)
- **Standard shell variables:** `$PWD`, `$HOME`, `$USER` (via `!`command`` execution)

**Example - Getting project root in command:**

```markdown
# Use git to find project root
Project root: !`git rev-parse --show-toplevel`

# Or use relative paths from known location
!`bash ./.claude/scripts/script.sh`
```

### Cross-Reference

For hook-specific environment variables (`$CLAUDE_PROJECT_DIR`, `$CLAUDE_CODE_REMOTE`, etc.), see
`.claude/guides/claude-code/settings.md` § Hook Environment Variables
