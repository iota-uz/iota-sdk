---
allowed-tools: Task, Bash(gh pr view:*), Bash(gh pr diff:*), Bash(gh api:*), Read, Glob
argument-hint: GitHub PR URL (e.g., https://github.com/owner/repo/pull/123)
description: "GitHub PR code review - analyzes changes and posts review comments directly on GitHub"
disable-model-invocation: true
---

# GitHub PR Review Command

Performs comprehensive code review on GitHub PR and posts review comments with inline feedback.

## Workflow

1. **Extract PR Info**: Parse PR URL, fetch metadata and diff using `gh pr view` and `gh pr diff`
2. **Review Changes**: Delegate to refactoring-expert agent; request structured output with path/line/body for each comment
3. **Post Review**: Convert agent output to `gh api` call with inline comments array, post with appropriate status (APPROVE/REQUEST_CHANGES/COMMENT)

## Key Commands

```bash
# Fetch PR metadata and files
gh pr view <number> --json title,body,author,files,headRefName,baseRefName

# Fetch diff
gh pr diff <number>

# Post review with inline comments
gh api -X POST repos/{owner}/{repo}/pulls/{number}/reviews \
  -f event="REQUEST_CHANGES" \
  -f body="Overall feedback" \
  -f 'comments[][path]=src/file.go' \
  -F 'comments[][line]=42' \
  -f 'comments[][body]=Inline comment'

# Simple review without inline comments
gh pr review <number> --approve --body "Approved"
gh pr review <number> --request-changes --body "Changes needed"
gh pr review <number> --comment --body "Suggestions"
```

## Agent Task

Delegate to refactoring-expert with PR metadata and diff. Request output in this format for each issue:
```
path: src/file.go
line: 42
category: Critical/Minor/Style
body: Specific feedback
```

Agent should apply IOTA SDK standards: SQL patterns, HTMX workflows, DDD architecture, error handling, testing, security.

## Instructions

1. Parse PR URL from `$ARGUMENTS` (extract owner/repo/number)
2. Fetch PR details and diff
3. Delegate to refactoring-expert; provide structured output format for comments
4. Build `gh api` POST command from agent output
5. Determine status: REQUEST_CHANGES if critical issues, COMMENT if only minor/style
6. Post review and summarize findings

Note: `gh pr review` does NOT support inline comments. Must use `gh api` for line-specific feedback.
