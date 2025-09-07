---
allowed-tools: |
  Bash(git status:*), Bash(git diff:*), Bash(git log:*),
  Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git pull:*),
  Bash(make fmt:*), Bash(templ generate:*)
description: "Commit changes and push to current branch - simplified workflow without PR creation"
---

# Commit Changes & Push

**Usage:**
- `/commit-push` - Analyze changes, commit with proper messages, and push to current branch

This command handles the workflow from uncommitted changes to pushing to remote, without creating pull requests or new branches.

## Workflow Process

### 1. Pre-Commit Preparation (CRITICAL)
**ALWAYS perform these steps before committing:**
- Run `make fmt` to format all Go code
- Run `templ generate` to regenerate template files (always run, not just when .templ modified)
- Pull latest changes to avoid conflicts
- **These steps are MANDATORY** - they ensure code consistency and prevent CI failures

### 2. Change Analysis & Commit Creation
- Analyze all changed files to understand nature of changes
- Spot files that should not be committed to VCS
- Delete build artifacts or temporary files (ask user if unsure)
- **NEVER delete or commit root markdown files** (*.md) like FOLLOW_UP_ISSUES.md or PR-239-REVIEW.md
- Group related changes into logical commits
- Create multiple commits if changes span different features or fixes
- Commit `*_templ.go` files even though they are generated

### 3. Push to Remote
- Push commits to current branch
- Report successful push with commit count

## Commit Message Convention

Use conventional commit prefixes:

- `fix:` - Bug fixes
- `feat:` - New features  
- `docs:` - Documentation updates
- `ci:` - CI/CD configuration changes
- `wip:` - Work in progress
- `style:` - Code formatting, missing semicolons, etc. (no functional changes)
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks that don't affect codebase functionality
- `refactor:` - Code restructuring without changing functionality

**Guidelines:**
- Write clear, concise commit messages explaining "what" and "why"
- Keep commit messages under 50 characters when possible
- Use present tense and imperative mood

## Command Execution Flow

1. Check git status and analyze changes
2. Run formatting and template generation if needed
3. Pull latest changes from remote
4. Create appropriate commits with conventional messages
5. Push changes to current branch
6. Report number of commits pushed

## File Handling Guidelines

**Commit these files:**
- All source code changes (.go, .templ, .ts, .js, etc.)
- Generated template files (*_templ.go)
- Configuration files (.toml, .json, .yaml)
- Database migrations

**Do NOT commit:**
- Build artifacts and binaries
- Temporary files and logs
- IDE-specific files
- Root directory markdown documentation (preserve but don't version)

**When unsure about a file:**
- Ask user for clarification before deleting or committing
- Err on the side of caution for documentation files

## Error Handling

- If git conflicts occur during pull, stop and ask user to resolve
- If push fails due to remote changes, suggest pulling first
- If formatting or template generation fails, report specific errors
- Always validate that commits were successful before proceeding to push

Begin by checking current git status and analyzing the scope of changes.