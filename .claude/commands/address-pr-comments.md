---
allowed-tools: |
  Bash(git add:*), Bash(git commit:*), Bash(git push:*), Bash(go vet:*), Bash(make test:*),
  Read, Edit, MultiEdit, Write, Grep, Glob, Task,
  mcp__github__get_pull_request, mcp__github__get_pull_request_comments,
  mcp__github__get_pull_request_reviews, mcp__github__get_pull_request_status,
  mcp__github__list_workflow_jobs, mcp__github__get_job_logs
argument-hint: PR URL or PR number (e.g., https://github.com/owner/repo/pull/123 or 123)
description: Fetch unresolved PR comments, address them, fix CI failures, then commit and push changes
---

# Address PR Review Comments & CI Failures

You have been given a PR URL or PR number: $ARGUMENTS

Follow these steps carefully:

## 1. Extract PR Information
- If given a full URL (e.g., https://github.com/owner/repo/pull/123), extract owner, repo, and PR number
- If given just a number, use it directly with the current repository

## 2. Check CI Status
**Use GitHub MCP tools to check CI status:**
- Use `mcp__github__get_pull_request_status` to get overall CI status
- If CI is failing:
  - Use `mcp__github__list_workflow_jobs` with `filter: "latest"` to get failed jobs
  - Use `mcp__github__get_job_logs` with `failed_only: true` to get logs from failed jobs
  - Analyze the failure reasons (test failures, linting errors, build issues)
  - Fix the underlying issues in the code

## 3. Fetch Unresolved Review Comments
**Use GitHub MCP tools to get comments:**
- Use `mcp__github__get_pull_request_comments` to get inline code comments
- Use `mcp__github__get_pull_request_reviews` to get review comments
- Filter for unresolved comments by checking:
  - Comments without "RESOLVED" or "OUTDATED" status
  - Comments from the most recent reviews
  - Comments that haven't been addressed in subsequent commits
- Group comments by file and location for efficient addressing

## 4. Address Each Comment
For each unresolved comment:
- Read the relevant file
- Understand the context and the reviewer's feedback
- Make the necessary code changes to address the comment
- Ensure changes follow the project's coding standards and conventions
- Run local validation:
  - `go vet ./...` for Go code issues
  - `make test` or relevant tests for the changed code (use 10-minute timeout for full suite)

## 5. Fix CI Failures
If CI checks were failing:
- **Test failures**: Fix the failing tests or update test expectations
- **Linting errors**: Run `make check lint` for linting or `make check fmt` for formatting
- **Build errors**: Fix compilation issues
- **Type errors**: Resolve any type checking issues
- Re-run local checks to ensure fixes work

## 6. Commit and Push Changes
After addressing all comments and CI issues:
- Stage all modified files using `git add`
- Create a descriptive commit message:
  ```
  fix: address PR review comments and CI failures
  
  - [List of addressed review comments]
  - [List of fixed CI issues]
  ```
- Push the changes to the PR branch

## 7. Verify CI Status
After pushing:
- Wait a moment for CI to trigger
- Use `mcp__github__get_pull_request_status` to verify CI is now passing
- If still failing, repeat steps 2-6

## 8. Provide Summary
Create a summary report including:
- Number of review comments addressed
- CI issues that were fixed
- Any comments that couldn't be addressed (with reasons)
- Current CI status

## Important Guidelines
- **Priority order**: Fix CI failures first, then address review comments
- Only address comments that are actionable (not questions or discussions)
- If a comment requires clarification, skip it and note it in the summary
- Ensure all changes maintain code quality and don't break existing functionality
- Follow the project's commit message conventions
- If there are conflicting suggestions, use your best judgment and document the decision
- Always verify CI passes after making changes

## Error Handling
- If CI logs are too large, focus on the error summary at the end
- If unable to determine comment resolution status, treat as unresolved
- If fixes introduce new issues, revert and try a different approach
- Document any comments that require discussion with the reviewer

Begin by extracting the PR information and checking both CI status and review comments.