---
allowed-tools: |
  Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(go vet:*), Bash(make test:*),
  Bash(gh pr view:*), Bash(gh api:*),
  Read, Edit, Write, Grep, Glob, Task
argument-hint: PR URL or PR number (e.g., https://github.com/owner/repo/pull/123 or 123)
description: Fetch unresolved PR comments and address them with code changes
disable-model-invocation: true
---

# Address PR Review Comments

Address unresolved review comments on a PR by making code changes, then commit and push.

## Workflow

1. **Fetch Comments**: Extract PR URL/number; get unresolved inline comments and reviews
2. **Address Comments**: For each actionable comment, modify code and validate locally
3. **Commit & Push**: Commit changes with clear message; push to branch

## Key Commands

```bash
# Get inline comments
gh api repos/{owner}/{repo}/pulls/{number}/comments

# Get reviews
gh api repos/{owner}/{repo}/pulls/{number}/reviews

# Validate changes locally
go vet ./...
make test

# Commit and push
git add .
git commit -m "fix: address PR review comments"
git push
```

## Process

**For each unresolved comment:**
- Read the relevant file and understand reviewer feedback
- Make necessary code changes
- Validate locally (go vet, make test)
- Repeat until all actionable comments addressed

**Commit:**
```
fix: address PR review comments

- Fixed [issue 1]
- Fixed [issue 2]
```

## Guidelines

- Address actionable comments only; skip questions or discussions
- If comment requires clarification, document it and move on
- Maintain code quality; don't break existing functionality
- If changes introduce issues, revert and try different approach
