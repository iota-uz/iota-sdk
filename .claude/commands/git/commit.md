---
description: "Commit changes with option to push to current branch or create new branch with PR"
model: haiku
---

## Workflow Sequence

1. Ask user about branching strategy
2. Check if CHANGELOG.md should be updated
3. Pre-Commit Preparation
4. Create commits
5. Post-Commit Actions

## Current codebase status:

!`git status --porcelain`

- Current branch: !`git branch --show-current`
- Recent commits: !`git log --oneline -10`

## Branch Strategy

Always ask the user about branch strategy using the AskUserQuestion tool:

```
AskUserQuestion:
- Question: "How would you like to proceed?"
- Header: "Branch Strategy"
- Options:
  - "Push to current branch - Commit and push to current branch"
  - "Create new branch + PR - Create new feature branch first, then commit and push"
- multiSelect: false
```

**If "Push to current branch":**

- Proceed to CHANGELOG Check, then Pre-Commit Preparation, then push to the current branch

**If "Create new branch + PR":**

- Create and checkout new branch based on changes (e.g., `fix/...`, `feat/...`, `refactor/...`)
- Ask user for base branch using AskUserQuestion:
  ```
  AskUserQuestion:
  - Question: "Which base branch should the PR target?"
  - Header: "PR Base Branch"
  - Options:
    - "staging - Standard feature workflow (staging → main)"
    - "main - Hotfix/urgent production fix"
  - multiSelect: false
  ```
- Store the selected base branch (staging or main) for use in PR creation
- Proceed to CHANGELOG Check, then Pre-Commit Preparation, then push with `-u` and create PR

## CHANGELOG Check (After Branch Created)

Evaluate if CHANGELOG.md should be updated. This happens AFTER branch creation but BEFORE pre-commit preparation:

!`sdk-tools changelog check`

**If MUST recommendation:**

- Inform user CHANGELOG update is required
- Use AskUserQuestion to get change description:
  ```
  AskUserQuestion:
  - Question: "Describe this change for CHANGELOG (≤300 characters)"
  - Header: "CHANGELOG Entry"
  - Options:
    - "feat: [your description]"
    - "fix: [your description]"
    - "refactor: [your description]"
    - "perf: [your description]"
    - "security: [your description]"
  - multiSelect: false
  ```
- Update CHANGELOG.md with a new entry (date | type | description)
- Enforce FIFO: Remove oldest if >10 entries
- Stage CHANGELOG.md for commit

**If SHOULD recommendation:**

- Use AskUserQuestion to ask if user wants to update:
  ```
  AskUserQuestion:
  - Question: "This looks like a significant change. Update CHANGELOG.md?"
  - Header: "CHANGELOG Update"
  - Options:
    - "Yes, update CHANGELOG"
    - "No, skip for now"
  - multiSelect: false
  ```
- If "Yes": Follow the same process as MUST criteria
- If "No": Continue to Pre-Commit Preparation

**If SKIP recommendation:**

- Continue to Pre-Commit Preparation without asking

## Pre-Commit Preparation (CRITICAL)

!`sdk-tools git precommit`

Based on the status above, perform necessary preparation:

1. If templ out of sync: `templ generate` (stop if fails)
2. If formatting needed: `make fix fmt` (stop if fails)
3. Always run translation validation: `make check tr` (ask user to proceed if fails)
4. If artifacts/temp files found: Remove them (ask user if unsure which files)
5. If secrets detected: STOP and warn user immediately

Stop and report errors if any step fails. Do not proceed to commits.

## Commit Messages

Use conventional commit format matching repository patterns. Common prefixes:

- `fix:`, `feat:`, `refactor:`, `test:`, `docs:`, `chore:`, `perf:`
- `ci:` - CI/CD configs (Dockerfile, workflows, stack.yml, compose files)
- `style:` - Code formatting only
- `cc:` - Claude Code configs (CLAUDE.md, .claude/**/)

## Changed Files Analysis

!`sdk-tools git changes`

**Important commit rules:**

- Always commit `*_templ.go` (generated but required)
- Always commit `.claude/**/` with `cc:` prefix
- Never commit files in "Should NOT Commit" category above
- Never commit secrets (.env, .pem, .key, credentials.json)

## Creating Commits

Use the categorized files above to group related changes into logical commits with appropriate conventional prefixes. Include CHANGELOG.md if it was updated.

## Post-Commit Actions

### If "Push to Current Branch"

Branch status:
!`sdk-tools git check-branch`

1. **Safety Check for Protected Branches**: If WARNING shown above, ask for explicit confirmation:
   ```
   AskUserQuestion:
   - Question: "You are about to push to {current-branch}. Are you sure?"
   - Header: "Protected Branch"
   - Options:
     - "Yes, push to {current-branch}"
     - "No, cancel push"
   - multiSelect: false
   ```
    - If user selects "No, cancel push": Stop and inform user the push was cancelled
    - If user selects "Yes": Continue to step 2

2. Pull the latest changes from remote (CRITICAL)
3. Push commits to the current branch
4. Report the successful push with commit count

**Error Handling:**

- Pull conflicts → stop and inform user (commits are safe, needs manual conflict resolution)
- Push fails → suggest resolving conflicts or checking network

### If "Create New Branch + PR"

Analyze PR context with the selected base branch:

```bash
# Run analysis with selected base (staging or main from earlier question)
sdk-tools pr context --base <selected-base>
```

1. New branch already created before the pre-commit phase
2. Push new branch with `-u` flag: `git push -u origin <new-branch>`
3. Check if PR already exists for this branch: `gh pr list --head <new-branch> --json number,url`
4. If NO PR exists:
    - Get commit history: `git log <selected-base>..HEAD` (use base from user's selection)
    - Analyze commits and context above to create PR description (see template below)
    - Create PR with selected base: `gh pr create --base <selected-base> --title "..." --body "$(cat <<'EOF'...)"`
    - Return PR URL from the command output
5. If PR exists:
    - Just push commits (PR auto-updates)
    - Return existing PR URL from list output

## PR Description Template

Use the PR context analysis above to create specific, actionable test plans rather than generic placeholders.

```markdown
### Summary

- [Specific changes based on commits]

### Test Plan

**Regression areas:** [Based on changed modules/services]

**Edge cases:** [Based on business logic changes]

**User flows:** [Based on controller/template changes]

**Integration points:** [Based on repository/external service changes]

**Performance/Security:** [If migrations or sensitive data handling changed]

Resolves #<issue-number>
```

