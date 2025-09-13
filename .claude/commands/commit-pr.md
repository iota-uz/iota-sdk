---
allowed-tools: |
  Bash(git status:*), Bash(git diff:*), Bash(git log:*),
  Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git pull:*),
  Bash(git checkout:*), Bash(git branch:*), Bash(make fmt:*), Bash(templ generate:*),
  mcp__github__create_pull_request, mcp__github__update_pull_request,
  mcp__github__get_pull_request, mcp__github__list_pull_requests
argument-hint: [optional: --base <branch> for PR base branch]
description: "Commit changes and automatically create/update PR - uses GitHub MCP tools for PR management"
---

# Commit Changes & Create a Pull Request

This command handles the complete workflow from uncommitted changes to pull request creation.


## Context

- Current git status: !`git status --porcelain`
- Changed files: !`git diff --name-only`
- Current git diff: !`git diff`
- Current git branch: !`git branch`
- Recent commits: !`git log --oneline -5`

**🚨 INTELLIGENT BEHAVIOR:**
- **On `staging` branch:** Always creates new feature branch + PR (prevents direct staging commits)
- **On feature branch:** Intelligently detects if PR already exists and creates one only if needed
- **Fully automated:** Command automatically determines when PR creation is appropriate

## Workflow Process

### 1. Pre-Commit Preparation (CRITICAL)
Based on the current git status:
- Analyze changed files to understand the nature of changes
- If `.templ` files were changed, regenerate them using `templ generate`
- If `.go` files were changed, format them using `make fmt`
- If `.toml` files were changed, test them using `make check-tr`. If failed, ask the user how to proceed.
- Delete build artifacts or temporary files (ask user if unsure)
- Group related changes into logical commits
- Create multiple commits if changes span different features or fixes
- Commit `*_templ.go` files even though they are generated
- Pull the latest changes from remote to avoid conflicts (CRITICAL)
- Push commits to the current branch
- Report a successful push with commit count

### 3. Smart Branch Management & PR Detection
**ALWAYS create new branch + PR when on staging:**
If currently on staging branch with uncommitted changes:
1. Create a new feature branch with descriptive name based on changes
2. Apply commit workflow from step 2 on the new branch
3. Push the new branch to remote
4. Create PR from new branch to specified base (default: staging)

**When on feature branch:**
1. Commit and push changes directly to the current branch
2. **Automatically check if PR exists** for the current branch using GitHub MCP tools
3. **If NO PR exists:** Create new PR with multilingual description
4. **If PR exists:** Just push commits (PR will auto-update)

## Commit Message Convention

Use conventional commit prefixes:

- `fix:` - Bug fixes
- `feat:` - New features  
- `docs:` - Documentation updates
- `ci:` - CI/CD configuration changes
- `wip:` - Work in progress (not for the ADV-deployment branch)
- `style:` - Code formatting, missing semicolons, etc. (no functional changes)
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks that don't affect codebase functionality
- `refactor:` - Code restructuring without changing functionality

**Guidelines:**
- Write clear, concise commit messages explaining "what" and "why"
- Keep commit messages under 50 characters when possible
- Use present tense and imperative mood

## Pull Request Creation Process

### 4. PR Title and Description Format
- **Title**: Brief description of changes (under 50 characters)
- **Description**: Multilingual format with comprehensive test plan

### 5. Critical PR Requirements
**NEVER make these mistakes:**
1. **Always specify base branch**: Use `--base staging` (or other specified base)
2. **All sections must be multilingual**: Both English and Russian versions required
3. **Default base branch**: Use staging unless explicitly told otherwise

### 6. PR Description Template

The PR will include both English and Russian versions:

```markdown
## English Version
### Summary
- First change description
- Second change description
- Third change description

### Test Plan
**Regression areas that could break:**
- Area 1 description
- Area 2 description

**Edge cases to verify:**
- Case 1 description
- Case 2 description

**User flows to test:**
1. Step 1 description
2. Step 2 description

**Integration points to verify:**
- Integration 1 description
- Integration 2 description

**Performance/Security considerations:**
- Consideration 1 description
- Consideration 2 description

## Russian Version
### Краткое описание
- Описание первого изменения
- Описание второго изменения  
- Описание третьего изменения

### План тестирования
**Области регрессии, которые могут сломаться:**
- Описание области 1
- Описание области 2

**Граничные случаи для проверки:**
- Описание случая 1
- Описание случая 2

**Пользовательские сценарии для тестирования:**
1. Описание шага 1
2. Описание шага 2

**Точки интеграции для проверки:**
- Описание интеграции 1
- Описание интеграции 2

**Производительность/Безопасность:**
- Описание соображения 1
- Описание соображения 2

Resolves #<issue-number>
```

## Command Execution Flow

### When on staging branch (ALWAYS creates branch + PR):
1. Check git status and analyze changes
2. Run formatting and template generation if needed  
3. Create a new feature branch with a descriptive name
4. Create appropriate commits with conventional messages on new branch
5. Push new branch to remote
6. Get commit history since diverging from base branch
7. Create a pull request with a multilingual description
8. Return PR URL for user reference

### When on feature branch (INTELLIGENT PR detection):
1. Check git status and analyze changes
2. Run formatting and template generation if needed
3. Create appropriate commits with conventional messages
4. Push changes to the current branch
5. **Check if PR already exists** using GitHub MCP tools (`mcp__github__list_pull_requests`)
6. **If NO PR exists:**
   - Get commit history since diverging from the base branch
   - Create pull request with multilingual description using `mcp__github__create_pull_request`
   - Return PR URL for user reference
7. **If PR already exists:**
   - Push commits (PR auto-updates with new commits)
   - **Update PR description** if significant new changes were added using `mcp__github__update_pull_request`
   - Analyze new commits to determine if the PR description needs updating
   - Append new test scenarios if new features/fixes were added
   - Return existing PR URL for user reference

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

- If git conflicts occur during pull, stop and ask the user to resolve
- If PR creation fails, provide a clear error message and suggested fixes  
- If formatting or template generation fails, report specific errors
- Always validate that commits were successful before proceeding to PR creation
