---
description: "Edit a backlog task item interactively"
model: sonnet
disable-model-invocation: true
---

You are helping the user edit an existing backlog task. Follow this interactive workflow:

## Step 1: List Backlog Items

Available backlog items:
!`ls -1 .claude/backlog/[0-9][0-9][0-9]-*.md 2>/dev/null | sort`

If no files are found, inform the user that the backlog is empty and exit.

## Step 2: Present Summaries

Read the first 3 lines of each backlog file to extract:
- Filename
- Agent type from `[agent:TYPE]`
- Task summary (first substantive line after agent tag)

Present summaries to the user in this format:
```
001-add-promocode-column-to-policies.md [editor]
→ Add promocode column to /crm/policies page and fix navigation visibility

002-fix-graphql-null-pointer-errors-insurance.md [debugger]
→ Fix GraphQL null pointer errors in insurance module
```

## Step 3: Select Item to Edit

Use AskUserQuestion tool to ask which backlog item to edit:
- Question: "Which backlog item would you like to edit?"
- Header: "Select item"
- Options: List each filename as an option with its summary as description
- multiSelect: false

## Step 4: Show Current Content

Read the selected file and display:
1. Full current content
2. Character count (important: must be ≤ 2000)
3. Warning if approaching limit (> 1800 chars)

Format:
```
Current content (1234 characters):
---
[file content here]
---
```

## Step 5: Ask What to Change

Ask the user: "What would you like to change in this backlog item? Describe the edits you want to make."

Wait for user's response with their desired changes.

## Step 6: Perform Targeted Edit

Based on user's request:
1. Use Edit tool to make targeted changes (not full rewrites)
2. After edit, count characters in new content
3. If new content exceeds 2000 characters:
   - Identify obvious implementation steps that can be removed
   - Remove excessive detail or context
   - Keep root cause/core change description
   - Keep verification steps
   - Apply additional edits to bring under 2000 chars
4. Show before/after character counts

## Step 7: Enforce Quality Guidelines

When editing, ensure the content:
- Focuses on root cause or core change that needs to happen
- Includes clear verification steps
- Skips obvious implementation steps
- Stays under 2000 characters
- Preserves agent type unless user explicitly requests change

If user's requested changes would bloat the file:
- Suggest more concise alternatives
- Remove unnecessary context
- Focus on "what and why" over "how"

## Step 8: Confirm Completion

After successful edit, confirm:
```
Backlog item updated: {FILENAME}
Character count: {BEFORE} → {AFTER} characters
```

## Important Notes

- Always use Edit tool for modifications (never Write, which overwrites completely)
- Character limit of 2000 is hard constraint - enforce strictly
- Focus on actionable changes and verification, not implementation details
- If user requests agent type change, update the first line `[agent:TYPE]`
