---
description: "Comprehensive code refactoring with agent-based planning. Supports arguments: /refactor [scope]. Agent analyzes code, generates prioritized plan, presents critical steps for approval. Modes: incremental (safe), holistic (redesign), hybrid (balanced)."
model: sonnet
disable-model-invocation: true
---

## 1. Select Refactoring Mode

Ask the user to choose mode upfront using AskUserQuestion:

```
AskUserQuestion(
  question: "Which refactoring mode would you like to use?",
  header: "Mode",
  multiSelect: false,
  options: [
    {
      label: "Incremental",
      description: "Safe improvements only, no breaking changes (~5-10% LOC reduction)"
    },
    {
      label: "Holistic",
      description: "Architectural redesign, breaking changes allowed (~15-30% LOC reduction)"
    },
    {
      label: "Hybrid",
      description: "Balanced approach, selective breaking changes (~10-20% LOC reduction)"
    },
    {
      label: "Auto-Detect",
      description: "Let the agent analyze and recommend the best mode"
    }
  ]
)
```

## 2. Determine Refactoring Scope

Parse scope from arguments or ask user:

If `$ARGUMENTS` provided:

- Use `$ARGUMENTS` as scope (e.g., "trucks & trailers code", "finance module")
- Continue to step 3

If no arguments:

- Ask: "What code do you want to refactor?"
- Show examples: "logistics module", "statement calculations", "driver_service.go", "all controllers"
- Store user response as scope
- Continue to step 3

## 3. Generate Refactoring Plan

Launch Task(subagent_type:refactoring-expert) with a mode-aware prompt that includes the appropriate guardrails:

**Base prompt for all modes:**

```
Generate refactoring plan for the following:

**Scope:** [scope from step 2]
**User Selected Mode:** [mode from step 1]

## Instructions

### 1. DISCOVER FILES
- Find all files matching scope
- Group by layer: services, controllers, viewmodels, templates, repositories
- Count total files and breakdown by layer

### 2. ANALYZE OPPORTUNITIES
Scan for opportunities in these categories: 
- Dead code (use `make check deadcode`)
- DRY violations
- DDD violations
- Overly complex or hard to read code
- Clean code principles violations
- Low value or outdated comments
- Unused translation values in .toml files
- Business Logic in .templ files
- Pour type safety (any, interface{}, etc.)

### 3. [Append mode-specific guardrails here - see below]

### 4. CREATE PLAN

Return a concise plan with ONLY the 5-10 most critical opportunities:

## Refactoring Plan

**Scope:** [scope]
**Files Discovered:** [total count]
**Layers:** [breakdown - e.g., "15 services, 8 controllers, 12 templates, 6 repositories"]
**Mode:** [Incremental/Holistic/Hybrid] [if auto-detect: add "(Recommended)" + rationale]
**Estimated LOC Reduction:** ~X%

### Top 5-10 Opportunities

1. **[Category]** - `[file/location]`
   - **Impact:** [HIGH/MED/LOW]
   - **LOC Saved:** ~X lines
   - **Risk:** [LOW/MED/HIGH]
   - **Change:** [1-2 sentences describing what and why]

2. [Next opportunity...]

**Note:** Keep plan concise for quick user review.
```

**Append to section 3 based on selected mode:**

```
### 3. GUARDRAILS ([Selected Mode])

[Incremental]: NO breaking changes, NO API changes. Focus: dead code, idioms, complexity, safe DRY fixes. Preserve "why" comments. Exclude: architecture changes.

[Holistic]: Breaking changes allowed when simpler. MUST provide migration notes. Prioritize: architecture, coupling elimination, cross-layer violations. Idempotent edits only.

[Hybrid]: Breaking changes ONLY if >15% LOC reduction OR eliminates major coupling. Apply incremental first, holistic for justified areas. Cost/benefit analysis required.

[Auto-Detect]: Analyze all issues, recommend mode: <5 arch issues→Incremental, >10→Holistic, 5-10→Hybrid. Include rationale + issue counts in plan.
```

Wait for the planning agent to complete and return the plan.

## 4. Review Plan & Get Approval

Display the generated plan to the user, then ask for approval:

```
AskUserQuestion(
  question: "Review the refactoring plan above. How would you like to proceed?",
  header: "Action",
  multiSelect: false,
  options: [
    {
      label: "Approve",
      description: "Execute the refactoring plan as presented"
    },
    {
      label: "Modify Plan",
      description: "Request specific changes to the plan"
    },
    {
      label: "Change Mode",
      description: "Select a different mode and regenerate the plan"
    }
  ]
)
```

**Handle user response:**

If "Approve":

- Proceed to step 5 (Execute Refactoring)

If "Modify Plan":

- Ask: "What changes would you like to make to the plan?"
- Collect user feedback
- Relaunch planning agent (step 3) with modification instructions
- Return to step 4

If "Change Mode":

- Return to step 1 (Select Refactoring Mode)
- Regenerate plan with new mode

## 5. Execute Refactoring

Based on the approved plan and selected mode, apply refactoring changes following CLAUDE.md § 2 (Agent Orchestration)
rules:

**Execution Strategy:**

1. Extract the list of opportunities from the approved plan
2. Group opportunities by category and scope
3. Assign work to `editor` agent(s):
    - For focused refactoring: Single `editor` agent
    - For multi-layer refactoring: Multiple `editor` agents in parallel (each handling specific layers)
4. **Always launch `refactoring-expert` LAST** after all implementation changes (§ 2 - Agent Orchestration)

**Verification after execution:**

1. Run targeted tests for modified areas
2. Review agent outputs for completeness

**On failure:**

1. Classify error type: compilation error, test failure, linting issue, or runtime error
2. Launch appropriate agent to fix the issue
3. Re-verify with same checks
4. Repeat until all checks pass
