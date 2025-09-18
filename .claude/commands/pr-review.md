---
allowed-tools: Task, mcp__github__get_pull_request, mcp__github__get_pull_request_files, mcp__github__get_pull_request_diff, mcp__github__create_pending_pull_request_review, mcp__github__add_comment_to_pending_review, mcp__github__submit_pending_pull_request_review, mcp__github__get_pull_request_comments, mcp__github__get_pull_request_reviews, Read, Glob
argument-hint: GitHub PR URL (e.g., https://github.com/owner/repo/pull/123)
description: "GitHub PR code review - analyzes changes and posts review comments directly on GitHub"
---

# GitHub PR Review Command

**Usage:**
- `/pr-review https://github.com/owner/repo/pull/123` - Review GitHub PR and post review comments

This command performs comprehensive code review on GitHub pull requests and posts review comments directly using GitHub MCP tools.

## Workflow

### 1. Extract PR Information
- Parse GitHub PR URL to extract owner, repo, and PR number
- Use `mcp__github__get_pull_request` to get PR metadata
- Use `mcp__github__get_pull_request_files` to list changed files
- Use `mcp__github__get_pull_request_diff` to get file diffs

### 2. Delegate to Refactoring Expert
Launch the refactoring-expert agent with:
- PR context (title, description, changed files)
- File diffs and content analysis
- Instructions to provide review comments with file:line references
- Focus on code quality issues that should be addressed in PR

**Task for Agent:** "Conduct comprehensive code review for GitHub PR #[PR_NUMBER] in [OWNER/REPO]. Review the changed files: [FILE_LIST]. Apply all IOTA SDK standards including SQL query management, HTMX workflows, repository patterns, DDD architecture, error handling, testing patterns, and security best practices. Identify issues in three categories: Critical (❌), Minor (🟡), and Style/Nits (🟢). Provide detailed review comments with specific file:line references for each issue found. DO NOT make direct code changes - only provide review feedback."

### 3. Post Review to GitHub
Based on agent's findings:
- Create pending PR review using `mcp__github__create_pending_pull_request_review`
- Add individual line comments using `mcp__github__add_comment_to_pending_review`
- Submit comprehensive review using `mcp__github__submit_pending_pull_request_review`
- Choose appropriate review status:
  - `APPROVE`: No critical issues found
  - `REQUEST_CHANGES`: Critical issues need to be addressed
  - `COMMENT`: Only minor/style suggestions

## Output Format

```
## GitHub PR Review Summary for #[PR_NUMBER]

### Review Comments Posted: [COUNT]
- Critical Issues: [COUNT] ❌
- Minor Issues: [COUNT] 🟡
- Style/Nits: [COUNT] 🟢

### Overall Assessment: [APPROVE|REQUEST_CHANGES|COMMENT]
[Summary of findings and recommendations]

### Key Areas Reviewed:
- Architecture & Design Patterns
- Security & Best Practices  
- Performance Considerations
- Code Quality & Standards
```

## Implementation Steps

1. **Parse PR URL**: Extract owner, repo, and PR number from `$ARGUMENTS`
2. **Fetch PR Details**: Use GitHub MCP tools to get PR information
3. **Analyze Changes**: Delegate to refactoring-expert for comprehensive review
4. **Process Findings**: Convert agent's feedback into structured GitHub comments
5. **Submit Review**: Post comments and overall review to GitHub

## Error Handling
- Validate PR URL format before processing
- Handle GitHub API errors gracefully
- Ensure authentication is properly configured
- Provide clear error messages if PR cannot be accessed

## Additional Instructions

$ARGUMENTS

Begin by parsing the PR URL, then fetch PR details and delegate to the refactoring-expert agent for comprehensive code review.