---
description: "Generate context summary with user intent, key findings, file changes, and resumption guidance"
disable-model-invocation: true
---

!`mkdir -p .claude/dumps`
Dumps directory created.
Available dumps: !`ls -1 .claude/dumps/ 2>/dev/null || echo "No dumps yet"`
Current date & time: !`date +%Y-%m-%d_%H-%M-%S`

## Instructions

- Review "Available dumps" section above for existing files
- Determine next number (DUMP_1.md, DUMP_2.md, DUMP_3.md, etc.)
- Think about the contents of the dump file and decide what to include (Use output format below as a guideline)
- Save to `.claude/dumps/DUMP_[next_number].md`

## Output Format

```markdown
# Context Summary - [Brief Title]

Generated: [timestamp]
Working Directory: [path]

## User Intent

What the user was trying to accomplish:

- Primary goal
- Specific requirements or constraints
- Success criteria

## Key Findings & Decisions

Important discoveries and choices made:

- Technical insights
- Architectural decisions
- Pattern choices and rationale
- Lessons learned

## Files Modified

List of changed files with change summaries:

### path/to/file.go

- Change 1: Description
- Change 2: Description
- Key patterns used

### path/to/another.go

- Change 1: Description

## Remaining Work

What's left to do:

- [ ] Next immediate step
- [ ] Follow-up tasks
- [ ] Testing/verification needed

## Open Questions & Blockers

Issues to resolve:

- Question 1
- Blocker 1 and possible solutions

## Resumption Guide

How to continue this work:

1. Quick commands to validate current state
2. Recommended next actions
3. Files to review first
4. Agent delegations if needed

---

**Quick Resume:**

```bash
# Commands to pick up where you left off
[relevant commands]
```

```