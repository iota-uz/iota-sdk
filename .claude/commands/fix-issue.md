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

### 3. Implementation
- Follow the task list systematically
- Use `mcp__bloom__search_code` for semantic code search when needed
- Apply changes following CLAUDE.md guidelines
- For UI changes: run `make css` after `.css` or `.templ` modifications
- For template changes: run `templ generate` after `.templ` modifications

### 4. Testing & Validation
- Run relevant tests: `go test -v ./path/to/modified/package`
- Run linting: `make lint`
- Format code: `make fmt`
- For translations: `make check-tr`
- Verify all tests pass

### 5. Commit Changes
- Review changes: `git diff`
- Stage files: `git add .`
- Commit with conventional format: `git commit -m "fix: <description> (#$ARGUMENTS)"`
  - Use `fix:` for bug fixes
  - Use `feat:` for new features
  - Use `refactor:` for code improvements
  - Use `docs:` for documentation
  - Use `test:` for test changes

### 6. Create Pull Request
- Push branch: `git push -u origin <branch-name>`
- Create PR: `gh pr create --title "Fix: <description> (#$ARGUMENTS)" --body "Fixes #$ARGUMENTS\n\n## Summary\n<what-was-fixed>\n\n## Testing\n<how-it-was-tested>"`

### 7. Update Issue
- Add comment to issue if needed: `gh issue comment $ARGUMENTS --body "PR created: <pr-url>"`

## Important Notes
- Always test changes thoroughly before creating PR
- Follow project conventions in CLAUDE.md
- Keep changes focused on the specific issue
- Update documentation if needed
