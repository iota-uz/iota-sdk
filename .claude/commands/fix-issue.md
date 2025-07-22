---
allowed-tools: Bash(gh issue list:*), Bash(gh issue view:*), Bash(gh issue create:*),
  Bash(git status:*), Bash(git diff:*), Bash(git add:*), Bash(git commit:*),
  Bash(git push:*), Bash(git pull:*), Bash(git checkout:*), Bash(git branch:*),
  Bash(gh pr create:*), Bash(make:*), Bash(go test:*), Bash(templ generate:*),
  Task, TodoWrite, Read, Edit, MultiEdit, Write, Grep, Glob, LS, mcp__bloom__search_code
description: Fix a GitHub issue following a structured workflow
argument-hint: <issue-number>
---

# Fix GitHub Issue #$ARGUMENTS

## Context
- Issue details: !`gh issue view $ARGUMENTS`
- Current branch: !`git branch --show-current`
- Git status: !`git status --short`

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

### 5. Implementation
- Follow the task list systematically
- Use `mcp__bloom__search_code` for semantic code search when needed
- Apply changes following CLAUDE.md guidelines
- For UI changes: run `make css` after `.css` or `.templ` modifications
- For template changes: run `templ generate` after `.templ` modifications
- Continuously run tests during implementation

### 6. Testing & Validation
- Run all relevant tests: `go test -v ./path/to/modified/package`
- Run specific test: `go test -v ./path/to/package -run TestName`
- Run linting: `make lint`
- Format code: `make fmt`
- For translations: `make check-tr`
- Ensure 100% of tests pass
- Add integration tests if needed

## Important Notes
- Always test changes thoroughly before implementing
- Follow project conventions in CLAUDE.md
- Keep changes focused on the specific issue
- Update documentation if needed
- Use separate slash commands for committing and creating PRs
