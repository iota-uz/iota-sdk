---
allowed-tools: Task, Bash(git status:*), Bash(git diff:*), Bash(go vet:*), Bash(make:*), Read, Glob
argument-hint: [optional: specific files or patterns to focus on, or empty for uncommitted changes]
description: "Comprehensive code review and production-grade refactoring orchestrator - /refactor-review [files/packages] or /refactor-review for uncommitted changes"
---

# Production-Grade Code Review & Refactoring Orchestrator

**Usage:**
- `/refactor-review` - Review and refactor all uncommitted changes (staged and unstaged)
- `/refactor-review file1.go file2.go` - Review and refactor specific files
- `/refactor-review ./pkg/module` - Review and refactor specific package/directory
- `/refactor-review $ARGUMENTS` - Review and refactor specified files or directories

This command orchestrates a comprehensive code review and refactoring workflow by leveraging the specialized refactoring-expert agent.

## Workflow

### 1. Identify Target Files
- If no arguments provided (`$ARGUMENTS` is empty): Run `git status` and `git diff` to identify uncommitted changes
- If arguments provided: Use the specified files/packages from `$ARGUMENTS`
- Focus on code files (ignore generated files, logs, etc.)

### 2. Delegate to Refactoring Expert
Launch the refactoring-expert agent with detailed instructions to:
- Conduct a comprehensive code review of the target files
- Apply production-grade refactoring following IOTA SDK standards
- Identify and fix critical issues, minor issues, and style improvements
- Verify all changes with appropriate validation commands

### 3. Verification
After the refactoring agent completes its work:
- Run `go vet ./...` to ensure Go code quality
- Run any additional validation commands specified in CLAUDE.md
- Review the final changes for completeness

## Task Instructions for Refactoring Agent

When launching the refactoring-expert agent, provide the following task:

**Task:** "Conduct comprehensive code review and production-grade refactoring for the following files: [FILE_LIST]. Apply all IOTA SDK standards including SQL query management, HTMX workflows, repository patterns, DDD architecture, error handling, testing patterns, and security best practices. Identify issues in three categories: Critical (❌), Minor (🟡), and Style/Nits (🟢). Implement fixes for all identified issues and provide a detailed report with file:line references."

## Output Format

The command will present the refactoring agent's findings in this format:

```
## Code Review & Refactoring Summary
[Agent's summary of changes and assessment]

### Critical Issues Found & Fixed ❌
[Critical issues and their resolutions]

### Minor Issues Found & Fixed 🟡  
[Important improvements implemented]

### Style Improvements & Nits Fixed 🟢
[Style and best practice improvements]

## Architecture Notes
[Observations about design patterns and architectural improvements]

## Security Considerations
[Security analysis and implemented fixes]

## Performance Notes
[Performance-related observations and optimizations]

## Verification Results
[Results of validation commands and testing]
```

## Additional Instructions

$ARGUMENTS

Begin by identifying target files, then launch the refactoring-expert agent with the identified files and let it handle all review and refactoring work.