---
allowed-tools: Bash(gh issue list:*), Bash(gh issue view:*), Bash(gh issue create:*), Bash(gh issue edit:*),
  Bash(gh project list:*), Bash(gh project item-list:*), Bash(gh project item-edit:*), Bash(gh project field-list:*),
  Bash(git status:*), Bash(git diff:*), Bash(git add:*), Bash(git commit:*),
  Bash(git push:*), Bash(git pull:*), Bash(git checkout:*), Bash(git branch:*),
  Bash(gh pr create:*), Bash(make:*), Bash(go test:*), Bash(templ generate:*),
  Task, TodoWrite, Read, Edit, MultiEdit, Write, Grep, Glob, LS, mcp__bloom__search_code
description: Fix a GitHub issue following a structured workflow
argument-hint: Issue URL or issue number (e.g., https://github.com/owner/repo/issues/123 or 123)
---

# Fix GitHub Issue

You have been given an issue URL or issue number: $ARGUMENTS

## Context
Current branch: !`git branch --show-current`
Git status: !`git status --short`

## Parse Issue Reference

First, let me determine the issue number:
- If given a full URL (e.g., https://github.com/owner/repo/issues/123), I'll extract the issue number
- If given just a number, I'll use it directly with the current repository

I'll run `gh issue view $ARGUMENTS --comments` to see the issue details.

## Mark Issue as In Progress

I'll move the issue to "In Progress" status in the SHY ELD GitHub Project:
1. Find the issue in the project items: `gh project item-list 11 --owner iota-uz --format json`
2. Extract the item ID for issue #$ARGUMENTS from the JSON output
3. Update the issue status to "In Progress": `gh project item-edit --id <item-id> --field-id PVTSSF_lADOCGNubc4BAYI8zgzMyio --project-id PVT_kwDOCGNubc4BAYI8 --single-select-option-id 47fc9ee4`

## Workflow

### 1. Analyze Issue Requirements
- Review the issue description and requirements above
- Create a detailed task list using TodoWrite tool
- Identify which files and modules need changes

### 2. Setup Feature Branch
- Ensure latest changes: `git checkout staging && git pull`
- Create feature branch: `git checkout -b fix/issue-$ARGUMENTS-<brief-description>`

### 3. Implementation Strategy
- Determine if TDD is appropriate for this issue:
  - Bug fixes: Write failing test that reproduces the bug first
  - New features: Write tests for expected behavior before implementation
  - Refactoring: Ensure existing tests pass, add new tests if needed
  - UI-only changes: TDD may not apply, focus on manual testing

### 4. Test-Driven Development (when applicable)
- Write failing tests first:
  - For bugs: Create test that reproduces the issue
  - For features: Write tests for new functionality
  - Use table-driven tests with descriptive names
  - Follow pattern: `TestFunctionName_Scenario`
- Run tests to confirm they fail: `go test -v ./path/to/package -run TestName`
- Implement minimal code to make tests pass
- Refactor while keeping tests green

### 5. Testing & Validation
- Run all relevant tests: `go test -v ./path/to/modified/package`
- Run specific test: `go test -v ./path/to/package -run TestName`
- Run linting: `make check lint`
- Format code: `make check fmt`
- For translations: `make check tr`
- Ensure 100% of tests pass
- Add integration tests if needed

## Important Notes
- Always test changes thoroughly before implementing
- Follow project conventions in CLAUDE.md
- Keep changes focused on the specific issue
- Update documentation if needed
- Use separate slash commands for committing and creating PRs
