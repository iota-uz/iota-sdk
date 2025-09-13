---
allowed-tools: |
  Bash(git status:*), Bash(git diff:*), Bash(git log:*),
  Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(git pull:*),
  Bash(make fmt:*), Bash(templ generate:*), Bash(make check-tr:*)
description: "Commit changes and push to current branch - simplified workflow without PR creation"
---


## Context

- Current git status: !`git status --porcelain`
- Changed files: !`git diff --name-only`
- Current git diff: !`git diff`
- Current git branch: !`git branch`
- Recent commits: !`git log --oneline -5`

## Commit Changes & Push

Based on the current git status:
- Analyze changed files to understand the nature of changes
- If `.go` files were changed, format them using `make fmt`
- If `.toml` files were changed, test them using `make check-tr`. If failed, ask the user how to proceed.
- If `.templ` files were changed, regenerate them using `templ generate` (always run templ generate after make fmt)
- Delete build artifacts or temporary files (ask user if unsure)
- Group related changes into logical commits
- Create multiple commits if changes span different features or fixes
- Commit `*_templ.go` files even though they are generated
- Pull the latest changes from remote to avoid conflicts (CRITICAL)
- Push commits to the current branch
- Report a successful push with commit count

## Commit Messages
- Clear, concise messages explaining "what" and "why"
- Under 50 characters when possible
- Present tense and imperative mood

## Never commit
- Build artifacts (ex.: binaries, docker images, etc.)
- Test coverage reports (ex.: coverage.out, coverage.html)
- Temporary files (ex.: some_file.test, some_file.out, some_file.go.old, etc.)
- Log files (ex.: logs/app.log, logs/app.log.old, etc.)
- IDE files (ex.: *.swp, *.swo, *.swn, etc.)
- Root markdown files (ex.: FOLLOW_UP_ISSUES.md / PR-239-REVIEW.md)
- CSV/Images/Docs unless explicitly told to do so
- Generated files (except `*_templ.go`)

### Commit Message Convention

Use conventional commit prefixes:
- `fix:` - Bug fixes
- `feat:` - New features  
- `docs:` - Documentation updates
- `ci:` - CI/CD configuration changes
- `wip:` - Work in progress
- `style:` - Code formatting (no functional changes)
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks
- `refactor:` - Code restructuring without changing functionality

### Error Handling

- Git conflicts during pull → stop and ask the user to resolve
- Push fails due to remote changes → suggest pulling first  
- Formatting/generation fails → report specific errors
- Always validate commits succeeded before pushing