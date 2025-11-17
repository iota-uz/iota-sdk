---
description: "Fix all linting and type errors in the codebase using go vet and linting tools."
model: haiku
disable-model-invocation: true
---

You are tasked with fixing all linting and type errors in the codebase:

All type errors: !`go vet ./... || true`
All linting errors: !`make check lint || true`

## Workflow

1. Implementation:
    - Launch agents with specific scope assignments
    - Each agent fixes assigned linting/type errors

2. Verification:
    - Run `go vet ./...` to verify no remaining type errors
    - Run `make check lint` to verify linting passes
    - Check for any new issues introduced

## Important Notes

- Follow CLAUDE.md § 2 (Agent Orchestration) Sequential Execution: agents && `refactoring-expert`
- DO NOT fix test files separately - include in the main fix workflow
- Preserve existing functionality while fixing errors
- Address unused variables/functions flagged by linter
- Follow project patterns and conventions
