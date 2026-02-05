# Prompt authoring principles (BIChat)

Write prompts as **behavioral guidance**, not a registry of facts the runtime already provides.

### What to include

- **Role and mission**: who the agent is and what “good” looks like.
- **Workflow**: the typical steps the agent should follow (explore → act → verify → answer).
- **Decision heuristics**: when to ask for clarification vs proceed; when to delegate; when to summarize.
- **Safety/constraints**: policy boundaries, data sensitivity rules, and what to do when blocked.
- **Output expectations**: the kind of final answer to produce (concise, actionable, structured).

### What to avoid

- **Do not redeclare tools** (names, parameters, descriptions, schemas). Tool metadata is injected automatically.
- **Do not copy database schemas, view lists, or UI details** into prompts. Those change and belong in tooling/context.
- **Do not embed long examples** unless they establish an essential pattern; prefer short, representative examples.
- **Do not duplicate generic LLM instructions** (e.g., “be helpful”) unless it changes behavior in this domain.
- **Avoid verbosity**: long prompts increase cost and reduce adherence.

### Tool usage guidance (without redeclaring tools)

- Focus on **how to combine tools** to accomplish tasks:
  - Explore schema before writing SQL.
  - Validate assumptions with small, safe queries and tight limits.
  - Use visualization/export tools only when they add user value.
- Prefer **small, incremental actions** with checkpoints rather than giant one-shot outputs.

### Style guidelines

- Use a **flat structure** with clear headings and short bullet points.
- Use **domain terms** and precise constraints (industry language, table/view conventions, KPI names).
- Prefer **imperatives** (“Do X”, “Avoid Y”) over abstract advice.
- Prefer **specific thresholds** when they matter (timeouts, limits, truncation behavior) but don’t restate tool schemas.

### Prompt review checklist

- Is any section just a tool catalog or schema dump? Remove it.
- Are workflows clear enough that the agent can act without guessing?
- Are constraints explicit, realistic, and non-contradictory?
- Is the prompt short enough to fit comfortably alongside retrieved context?

