---
description: "Prune local branches no longer present on remote"
---

# Prune Local Branches

Safely remove local branches that no longer exist on remote, keeping your workspace clean.

## Workflow

1. **Context Loading** (show current state)
2. **Prune Remote Tracking** (remove stale remote refs)
3. **Identify Candidates** (find deletable branches)
4. **Confirm Deletion** (ask user)
5. **Delete Branches** (execute cleanup)

---

## Context Loading

**Current branch:** !`git branch --show-current`

**All local branches:** !`git branch --format='%(refname:short)'`

**Remote tracking branches:** !`git branch -r --format='%(refname:short)'`

---

## Step 1: Prune Remote Tracking Branches

First, remove remote-tracking branches that no longer exist on remote:

```bash
git fetch --prune
```

This updates your local view of remote branches without affecting local branches.

---

## Step 2: Identify Deletion Candidates

Find local branches that meet ALL criteria:

- Not the current branch
- Not a protected branch (ADV-deployment, staging)
- Remote tracking branch no longer exists OR branch is fully merged into main/master

**Command to find gone branches:**

```bash
git branch -vv | grep ': gone]' | awk '{print $1}'
```

**Command to find merged branches:**

```bash
git branch --merged main | grep -v -E '(main|master|develop|staging|^\*)'
```

**IMPORTANT:** Always exclude:

- Current branch (marked with `*`)
- Protected branches: `main`, `master`, `develop`, `staging`
- Any branch with unpushed commits (unless explicitly merged)

---

## Step 3: Confirm with User

Present the list of branches to be deleted and ask for confirmation:

```
AskUserQuestion:
- Question: "Delete these branches: <branch-list>?"
- Header: "Confirm Deletion"
- Options:
  - "Yes, delete all - Remove all identified branches"
  - "No, cancel - Keep all branches"
- multiSelect: false
```

If user selects "No, cancel": Exit without deleting anything.

---

## Step 4: Delete Branches

If user confirms, delete each branch:

```bash
git branch -d <branch-name>
```

**If deletion fails (unmerged work):**

- Show warning: "Branch '<branch-name>' has unmerged changes"
- Ask user if they want to force delete with `-D`:
  ```
  AskUserQuestion:
  - Question: "Branch '<branch-name>' has unmerged work. Force delete?"
  - Header: "Force Delete"
  - Options:
    - "Yes, force delete - Discard unmerged changes (git branch -D)"
    - "No, skip this branch - Keep the branch"
  - multiSelect: false
  ```
- If "Yes": `git branch -D <branch-name>`
- If "No": Skip and continue with next branch

---

## Safety Checks

**NEVER delete:**

- Current branch (will fail, but check anyway)
- Protected branches: main, master, develop, staging
- Branches with pattern matching your team's conventions (ask if unsure)

**Always confirm before:**

- Deleting multiple branches (>3)
- Force deleting branches with unmerged work

---

## Example Output

After successful pruning:

```
Pruned remote tracking branches: 5
Deleted local branches: 3
  - feature/old-implementation
  - fix/temporary-patch
  - test/experiment-branch
Skipped (unmerged): 1
  - feature/work-in-progress
```

---

## Error Handling

**No branches to prune:**

- Message: "All local branches are up to date or in use"
- Suggest: "Run 'git fetch --prune' to check remote status"

**Deletion fails:**

- Show specific error from git
- Suggest: Check if branch has unmerged commits or is currently checked out elsewhere

**Network issues:**

- If `git fetch --prune` fails, warn user
- Suggest: Check network connection and retry
