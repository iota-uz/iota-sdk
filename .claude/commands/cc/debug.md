---
description: Debug and fix Claude Code configuration issues from reported behavior
model: sonnet
disable-model-invocation: true
---

You are debugging Claude Code configuration based on user-reported behavioral issues. Stop here and ask the user to
describe the issue they are experiencing. Wait for the user to report a problem.

## Phase 1: Root Cause Analysis

Launch ONE `Explore` agent (sonnet) to diagnose:

**Agent Task:**

```
Analyze Claude Code configuration issue from user problem: $ARGUMENTS

STEPS:
1. Identify affected domain:
   - Agent behavior (check .claude/agents/*.md)
   - Command workflows (check .claude/commands/**/*.md)
   - Orchestration logic (check CLAUDE.md)
   - Guide content (check .claude/guides/**/*.md)

2. Find root cause per architecture principles:
   - Ambiguous instructions (violates Clarity)
   - Duplicate/conflicting guidance (violates No Duplication)
   - Missing permissions/context (incomplete definition)
   - Token bloat causing truncation (violates Token Efficiency)
   - Wrong agent assignment (violates Separation of Concerns)

3. Identify exact file:line locations with issues

OUTPUT FORMAT:
ROOT CAUSE: [One-line diagnosis]

AFFECTED FILES:
- [file:line] - [what's wrong]

PROPOSED CHANGES:
For each file:
  [file:line]
    CURRENT: [exact text to replace, verbatim]
    NEW: [exact replacement text]
    REASON: [why this fixes it, reference architecture principle]

Return ONLY this structured output, no additional prose.
```

## Phase 2: Present Findings

Display the Explore agent's structured output to user.

**PAUSE HERE** - Wait for user to review proposed changes and continue conversation.

## Phase 3: Implementation

After user confirms, implement changes using Edit tool:

For each proposed change:

1. Read affected file
2. Apply Edit with exact CURRENT → NEW replacement
3. Report: "[file:line] - [change applied]"

## Phase 4: Validation

If cc-token available:

```bash
cc-token count --analyze [modified-file]
```

Report:

- Token impact: [before → after tokens]
- Principles satisfied: [list architecture principles now met]
- Next steps: Test the configuration by triggering the problematic behavior

## Error Handling

**If the problem is unclear:**
Use AskUserQuestion:

- Which Claude action is problematic?
- In which context? (agent, command, main conversation)
- What should Claude do instead?

**If outside CC config scope:**
Report: "This appears to be a model limitation, not a configuration issue. Consider rephrasing user prompts or using a
different approach."

**If multiple possible causes:**
List hypotheses ranked by likelihood, propose highest-impact fix first.

## Guidelines

- Be surgical: Change ONLY problematic instructions
- Preserve intent: Don't remove critical guidance
- Reference principles: Cite architecture.md for every change
- One fix at a time: Avoid sweeping refactors
- Exact replacements: Use verbatim text for Edit tool
