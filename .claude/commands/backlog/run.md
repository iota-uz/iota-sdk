---
description: "Execute backlog tasks sequentially using specified agents"
model: sonnet
disable-model-invocation: true
---

Execute backlog items autonomously without stopping until all selected tasks are processed.

Available backlog items:

!`ls -1 .claude/backlog/*.md 2>&1 | sort -n`

If no files are found, stop and inform the user.

## Step 1: Upfront Configuration

Ask all questions before starting execution.

### 1.1 Mode Selection

Use `AskUserQuestion`:

- **Question:** "How would you like to execute backlog tasks?"
- **Header:** "Execution mode"
- **Options:**
    - "Run all items" → Execute all backlog tasks sequentially
    - "Select items" → Choose specific tasks to execute

### 1.2 Item Selection (if applicable)

If the user selected "Select items":

1. Read each backlog file to extract the first 50-100 characters as preview
2. Use `AskUserQuestion` with `multiSelect: true`:
    - **Question:** "Which backlog items would you like to execute?"
    - **Header:** "Task selection"
    - **multiSelect:** true
    - **Options:** One per backlog file:
        - **label:** Filename (e.g., "001-fix-auth.md")
        - **description:** First 50-100 chars (exclude `[agent:TYPE]` and `[model:MODEL]` lines)

After questions are answered, proceed to autonomous execution.

## Step 2: Autonomous Execution

Execute all selected tasks sequentially. Do NOT stop on errors.

### 2.1 Parse Tasks

For each task file (numeric order):

1. Read file contents using Read tool
2. Parse first line: `[agent:TYPE]` extracts the agent type
3. Parse second line: `[model:MODEL]` extracts the model (haiku/sonnet/inherit)
4. Parse remaining lines: task prompt (line 3 onwards)

### 2.2 Execute Sequentially

For each task:

1. Extract agent type from `[agent:TYPE]`, model from `[model:MODEL]`, and task prompt
2. Launch agent using Task tool with extracted prompt and model
3. Track execution result (success/failure)
4. If an error occurs: collect error details, continue to the next task
5. Wait for completion before proceeding to the next task

**Execution pattern:**

```
Task 1 (editor) → Wait → Task 2 (editor) → Wait → Task 3 (debugger) → Wait → etc.
```

**Do NOT execute in parallel.**

### 2.3 Error Collection

When a task fails:

1. Record task filename
2. Record agent type used
3. Record error message/details
4. Continue to the next task without stopping

### 2.4 Backlog Files

Do NOT modify or delete backlog files during execution.

## Step 3: Final Report

After all tasks are processed, present summary:

### Success Summary

- Total tasks executed: X
- Successful: Y
- Failed: Z

### Error Details (if any)

For each failed task:

- **File:** filename.md
- **Agent:** agent type
- **Error:** error message/details

If all tasks succeed, confirm completion without errors.
