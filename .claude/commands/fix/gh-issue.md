---
description: Fix a GitHub issue following a structured workflow
argument-hint: Issue URL or issue number (e.g., https://github.com/owner/repo/issues/123 or 123)
disable-model-invocation: true
---

# Fix GitHub Issue

You have been given an issue URL or issue number: $ARGUMENTS

## Parse Issue Reference

First, let me determine the issue number:
- If given a full URL (e.g., https://github.com/owner/repo/issues/123), I'll extract the issue number
- If given just a number, I'll use it directly with the current repository

I'll run `gh issue view $ARGUMENTS` to see the issue details.
I'll check the current branch with `git branch --show-current`.
I'll check the git status with `git status --short`.

## Mark Issue as In Progress

I'll move the issue to "In Progress" status in the EAI GitHub Project:
1. Find the issue in the project items: `gh project item-list 6 --owner iota-uz --format json`
2. Extract the item ID for issue #$ARGUMENTS from the JSON output
3. Update the issue status to "In Progress": `gh project item-edit --id <item-id> --field-id PVTSSF_lADOCGNubc4A9AihzgzMyio --project-id PVT_kwDOCGNubc4A9Aih --single-select-option-id 47fc9ee4`

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
- Use `Grep` or `Glob` for code search when needed
- Apply changes following project guidelines
- For UI changes: run `make css` after `.css` or `.templ` modifications
- For template changes: run `templ generate` after `.templ` modifications
- Continuously run tests during implementation

### 6. Testing & Validation
- Run all relevant tests: `go test -v ./path/to/modified/package`
- Run specific test: `go test -v ./path/to/package -run TestName`
- Run linting: `make check lint`
- Format code: `make fix fmt`
- For translations: `make check tr`
- Ensure 100% of tests pass
- Add integration tests if needed

### 7. Commit and Create PR

After all changes are implemented and tested:
- Use `/commit` command to commit changes and create PR
- Command will prompt for branch strategy (push to current vs create new branch + PR)
- Reference the issue number in PR description (e.g., "Resolves #123")
- Ensure all pre-commit checks pass (templ generate, go vet, make fix fmt)

## Important Notes
- Always test changes thoroughly before implementing
- Follow project conventions
- Keep changes focused on the specific issue
- Update documentation if needed
- Use `/commit` command for committing and PR creation
