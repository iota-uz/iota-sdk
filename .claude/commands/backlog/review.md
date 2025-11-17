---
description: "Review backlog items for completion using refactoring-expert"
model: haiku
disable-model-invocation: true
---

You are tasked with reviewing backlog items to verify implementation quality and completeness. All available backlog items:

!`ls -1 .claude/backlog/*.md 2>&1 | sort -n`

If no files are found, stop and inform the user.

## Step 1: Initialize Review

Prepare to track the completion status for each backlog item. You will review each item sequentially using the refactoring-expert agent.

## Step 2: Sequential Review Process

For each backlog file in numeric order:

1. Read the file contents using the Read tool
2. Extract the agent type from `[agent:TYPE]` (first line)
3. Extract the task prompt (all lines after the first line)
4. Launch the `refactoring-expert` agent using the Task tool with this prompt:

```
Review the implementation for the following task and determine:

1. Code quality: Is the code well-structured, maintainable, and following best practices?
2. Completeness: Are all requirements from the task fully implemented?
3. Implementation status: Is this task FULLY COMPLETE or INCOMPLETE?

Task context:
{TASK_PROMPT}

Provide your assessment focusing on:
- What was implemented (if anything)
- Quality of the implementation
- Missing functionality or requirements
- Final verdict: COMPLETE or INCOMPLETE

If you cannot find relevant implementation or the task appears not to have been started, mark as INCOMPLETE.
```

5. Wait for the agent to complete before proceeding to the next item
6. Track the filename and completion status based on agent feedback
7. **DO NOT delete files during this step** - only collect completion status

## Step 3: Present Complete Items

After reviewing all items, compile a list of items that the refactoring-expert agent marked as COMPLETE.

If no items are complete, inform the user and exit.

If items are complete, present them to the user in this format:

```
Review complete. The following items are fully implemented:

001-add-promocode-column-to-policies.md
- Status: COMPLETE
- Summary: [brief summary from agent feedback]

002-fix-graphql-null-pointer-errors.md
- Status: COMPLETE
- Summary: [brief summary from agent feedback]
```

## Step 4: Confirm Deletion

Use the AskUserQuestion tool to ask which complete items should be deleted:

- **Question:** "Which complete backlog items would you like to delete?"
- **Header:** "Delete items"
- **Options:** List each COMPLETE filename as an option with its summary as description
- **multiSelect:** true

## Step 5: Delete Confirmed Items

After receiving user selection:

1. Confirm: "Deleting {COUNT} items: {FILENAMES}"
2. Delete each selected file using rm command: `rm .claude/backlog/{FILENAME}`
3. Provide summary:

```
Deleted: {COUNT} items
Remaining backlog items: {COUNT}
```

If remaining items exist, list them.

## Important Notes

- Execute reviews sequentially (one after another), NOT in parallel
- Wait for each refactoring-expert agent to complete before moving to the next item
- Track completion status from agent feedback - look for a clear COMPLETE / INCOMPLETE verdict
- Only suggest deletion for items marked COMPLETE by the agent
- Use Bash tool with rm command to delete files
- Do NOT renumber remaining files (unlike delete.md, this preserves the original numbering)
