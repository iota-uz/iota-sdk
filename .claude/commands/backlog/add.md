---
description: "Add a new task to the backlog with agent-assisted prompt building"
model: sonnet
disable-model-invocation: true
---

You are helping the user build a well-crafted task prompt to add to the backlog. Follow this interactive guided
workflow:

## Step 1: Gather Context

Ask the user: "What task would you like to add to the backlog? Please provide a brief description or context."

Wait for the user's response with the task context.

## Step 2: Research & Planning

Based on the user's context, determine what research is needed:

- If the task requires understanding existing code patterns, file locations, or codebase structure → Launch Explore
  agent (thoroughness: medium)
- If the task requires planning implementation steps or architectural decisions → Launch `Plan` agent
- If both are needed → Launch both agents in sequence. `Explore` to understand existing code patterns, `Plan` to
  plan implementation steps.

Provide the agents with the user's context and ask them to research and provide relevant information.

## Step 3: Present Findings

After agents complete, summarize key findings for the user:

- Relevant files/patterns discovered
- Recommended implementation approach
- Any important considerations or dependencies

## Step 4: Craft Prompt

Help the user craft a well-formed prompt based on the research. Ask:
"Based on the research, here's a suggested prompt for this task. Would you like to use this, or would you like to modify
it?"

Present a draft prompt that includes:

- Clear task objective
- Relevant context from agent research
- Specific files or patterns to follow
- Expected outcomes

## Step 5: Select Agent Type

Ask the user to select which agent should execute this task using AskUserQuestion tool:

Options:

- editor - All backend work: domain logic, services, repositories, migrations, controllers, ViewModels, templates, translations
- refactoring-expert - Code quality, simplification, optimization
- qa-tester - Testing, bug detection, quality assurance
- debugger - Error investigation, debugging, issue diagnosis
- general-purpose - Multi-faceted tasks requiring multiple capabilities

## Step 6: Create Individual Backlog File

Once the agent type is selected and the prompt is finalized:

Available backlog items:
!`ls -1 .claude/backlog/*.md | sort -n | tail -1`

1. Determine the next sequence number. If no files exist, start with `001`. Otherwise, increment the highest number by
2. Generate a task slug from the finalized prompt. Take the first 40-50 characters of the prompt, convert to lowercase,
   and remove special characters, replace spaces with hyphens.
   Ex.: "Add PromoCode column to policies page" → "add-promocode-column-to-policies-page"
3. Create the filename with format: `.claude/backlog/{SEQUENCE}-{SLUG}.md`
    - Pad sequence with leading zeros (001, 002, etc.)
    - Example: `.claude/backlog/003-add-promocode-column-to-policies.md`
4. Write the file with this content:
   ```
   [agent:SELECTED_TYPE]
   FINALIZED_PROMPT
   ```

5. Confirm to the user: "Task added to backlog as `{FILENAME}`. Use `/backlog:run` to run all tasks."

## Important Notes

- Be conversational and helpful throughout the process
- Use agents proactively to ensure well-researched prompts
- Ensure prompts are clear, specific, and actionable
- The backlog file should maintain clean formatting with `---` delimiters
