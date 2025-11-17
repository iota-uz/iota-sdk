---
description: "Clear backlog items interactively"
model: haiku
disable-model-invocation: true
---

You are helping the user clear backlog items. Follow this interactive workflow:

## Step 1: List Backlog Items

Available backlog items:
!`ls -1 .claude/backlog/[0-9][0-9][0-9]-*.md 2>/dev/null | sort`

If empty, inform the user and exit.

## Step 2: Present Summaries

Read the first 3 lines of each backlog file to extract:
- Filename
- Agent type from `[agent:TYPE]`
- Task summary (first substantive line after agent tag)

Present summaries to the user in this format:
```
001-add-promocode-column-to-policies.md [editor]
- Add promocode column to /crm/policies page and fix navigation visibility

002-fix-graphql-null-pointer-errors-insurance.md [debugger]
- Fix GraphQL null pointer errors in insurance module
```

## Step 3: Ask Delete Strategy

Use AskUserQuestion tool to ask how to proceed:
- Question: "How would you like to clear backlog items?"
- Header: "Delete strategy"
- Options:
  * label: "Delete all", description: "Remove all backlog items"
  * label: "Select items", description: "Choose specific items to delete"
- multiSelect: false

## Step 4: Handle Selection

### If "Delete all":
1. Confirm count: "Deleting all {COUNT} backlog items"
2. Execute: `rm .claude/backlog/[0-9][0-9][0-9]-*.md`
3. Confirm: "All backlog items cleared"
4. Exit (no renumbering needed)

### If "Select items":
1. Use AskUserQuestion tool to select items for deletion:
   - Question: "Which backlog items would you like to delete?"
   - Header: "Select items"
   - Options: List each filename as an option with its summary as description
   - multiSelect: true

2. After receiving selection, confirm: "Deleting {COUNT} items: {FILENAMES}"

3. Delete selected files using rm command for each file

4. Proceed to Step 5 for renumbering

## Step 5: Renumber Remaining Items

After selective deletion, renumber remaining files to close gaps:

1. List remaining files: `ls -1 .claude/backlog/[0-9][0-9][0-9]-*.md 2>/dev/null | sort`

2. For each file, if current number != expected sequence number:
   - Extract base name without number prefix
   - Rename to new sequential number (001, 002, 003, etc.)
   - Example: If 001, 004, 005 remain after deleting 002-003:
     * 001-foo.md stays 001-foo.md
     * 004-bar.md becomes 002-bar.md
     * 005-baz.md becomes 003-baz.md

3. Use mv commands to perform renaming

4. Confirm: "Renumbered {COUNT} remaining items"

## Step 6: Summary

Show final state:
```
Deleted: {COUNT} items
Remaining: {COUNT} items
```

If remaining items exist, list them with new numbers.

## Important Notes

- Use Bash tool to execute file operations
- Preserve original filenames (only change number prefix)
- Ensure sequential numbering starts at 001
- Handle edge cases (no files, single file, etc.) 