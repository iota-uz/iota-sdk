---
description: "Execute backlog tasks sequentially using specified agents (all or selected)"
model: sonnet
disable-model-invocation: true
---

You are tasked with executing items from the backlog. All available backlog items:

!`ls -1 .claude/backlog/*.md 2>&1 | sort -n`

If no files are found, stop and inform the user.

## Step 1: Mode Selection

Use `AskUserQuestion` to ask the user whether to run all items or select specific ones:

**Question:** "How would you like to execute backlog tasks?"
**Header:** "Execution mode"
**Options:**

- "Run all items" → Execute all backlog tasks sequentially
- "Select items" → Choose specific tasks to execute

If the user selects "Run all items", proceed to Step 2 with all backlog files.

If the user selects "Select items":

1. Read each backlog file to extract the first 50-100 characters as a preview
2. Use AskUserQuestion with multiSelect: true to let user choose items:
    - **Question:** "Which backlog items would you like to execute?"
    - **Header:** "Task selection"
    - **multiSelect:** true
    - **Options:** One option per backlog file, with:
        - **label:** Filename (e.g., "001-fix-auth.md")
        - **description:** First 50-100 chars of task content (excluding `[agent:TYPE]` line)
3. Proceed to Step 3 with only the selected files

## Step 2: Execution Instructions

Parse the backlog content and execute each task sequentially:

### 1. Parse Tasks

Read each task file from the sorted list of backlog files:

For each task file (in numeric order):

1. Read the file contents using the Read tool
2. First line contains `[agent:TYPE]` where TYPE is the agent to use
3. The remaining lines (starting from line 2) are the task prompt
4. Extract both the agent type and the full task prompt

### 2. Execute Tasks Sequentially

For each task in order:

1. Extract the agent type from `[agent:TYPE]` & the task prompt (everything after the first line)
2. Use the Task tool to launch the specified agent with the task prompt
3. Wait for the agent to complete before proceeding to the next task

### 3. Execution Pattern

**IMPORTANT:** Execute tasks sequentially (one after another), NOT in parallel.

Use this pattern:

```
Task 1 (editor) → Wait for completion → Task 2 (editor) → Wait for completion → Task 3 (debugger) → etc.
```

**DO NOT** execute tasks in parallel:

```
// INCORRECT
Task 1 & Task 2 & Task 3
```

### 4. Backlog Files Management

**IMPORTANT:** Do NOT modify or delete backlog files during execution. Leave them unchanged.

After successful execution, users can manually:

- Delete individual task files from `.claude/backlog/` directory
- Archive completed tasks to `.claude/backlog/archive/` subdirectory
- Use `/backlog:delete` command to clear completed tasks

## Step 3: Error Handling

If a task fails or an agent encounters an error:

1. Report the error clearly
2. Ask the user if they want to:
    - Continue with the remaining tasks
    - Stop execution
    - Skip the failed task and continue

## Step 4: Completion

After all tasks are executed, provide a summary:

- Total tasks executed
- Success count
- Failure count (if any)
- List of completed task files
- Reminder that users can manually delete or archive completed task files from `.claude/backlog/` if needed
