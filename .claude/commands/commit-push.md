---
allowed-tools: |
  Bash(git status:*), Bash(git diff --cached:*),
  Bash(git add:*), Bash(git commit:*),
  Bash(git push:*), Bash(git pull:*),
description: Commit and push changes to the repository
---

# Commit and Push Changes

This command analyzes changed files since the last commit and creates appropriate commits with proper conventional commit messages.

## Commit Message Convention

Use the following prefixes for commit messages:

- `fix:` - Bug fixes
- `feat:` - New features
- `docs:` - Documentation updates
- `ci:` - CI/CD configuration changes
- `style:` - Code formatting, missing semicolons, etc. (no functional changes)
- `perf:` - Performance improvements
- `test:` - Adding or updating tests
- `chore:` - Maintenance tasks that don't affect the codebase functionality
- `refactor:` - Code restructuring without changing functionality

## Guidelines

1. Analyze all changed files to understand the nature of changes
2. Group related changes into logical commits
3. Create multiple commits if changes span different features or fixes
4. Write clear and concise commit messages that explain the "what" and "why"
5. Pull latest changes before pushing to avoid conflicts
