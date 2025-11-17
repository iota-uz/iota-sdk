---
description: "Create a pull request for the current branch"
model: sonnet
---

# Create Pull Request

Creates a pull request for the current branch with comprehensive test plan.

Current branch: !`git branch --show-current`
Git status: !`git status -sb`
Existing PR: !`gh pr view --json state,url,title 2>&1 | jq 'select(.state == "OPEN")' || echo "No open PR found"`

## Workflow

### 1. Check Existing PR

If open PR exists in dynamic context above (shows url/title):
- Show URL to user
- Exit (no duplicate PR creation)

If no open PR ("No open PR found"): Proceed with creation

### 2. Confirm Base Branch

Use AskUserQuestion to confirm target branch:
- Question: "Which branch should this PR target?"
- Header: "Base Branch"
- Options: main, staging
- multiSelect: false

### 3. Push Branch

Check remote status: `git ls-remote --heads origin <current-branch>`
- If not exists or behind: `git push -u origin <current-branch>`
- If up to date: Skip

### 4. Analyze Changes

Get context:
- Commits: `git log <base-branch>..HEAD --oneline`
- Files changed: `git diff <base-branch>...HEAD --stat`

Review ALL commits and identify:
- Main features/fixes
- Affected modules/domains
- Regression areas
- Edge cases
- User flows
- Integration points
- Performance/security impact

### 5. Create PR

Generate description covering:

**Summary** (3-5 bullets):
- Key changes based on commits
- Specific modules/files affected

**Test Plan**:
- **Regression areas**: What existing functionality might break
- **Edge cases**: Boundary conditions, null/empty values, validation
- **User flows**: Step-by-step actions to test
- **Integration points**: External services, APIs, database queries
- **Performance/Security**: Query optimization, auth checks, N+1 queries

Use `gh pr create` with HEREDOC:
- `--title`: Concise summary from commits
- `--body`: Generated description via `$(cat <<'EOF' ... EOF)`
- `--base`: Selected base branch

Return PR URL

