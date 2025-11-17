---
description: Systematically identify and fix broken tests using iterative approach
model: sonnet
---

# Fix Tests

Systematically identify and fix broken tests using structured iteration.

## Current State

Static analysis status:
!`go vet ./...`

Test status:
!`make test 2>&1 | head -100`

Recent changes:
!`git log --oneline -5 -- back/`

## Workflow

### 1. Discovery

Analyze the output above:

- Identify compilation/static analysis errors
- Count failing tests
- Categorize: compilation → assertion → panic → timeout
- Prioritize compilation errors first

### 2. Analysis

Per failure:

- Parse error messages and stack traces
- Read test code and implementation
- Identify root cause:
  - Implementation bug: Fix actual code
  - Outdated test: Update expectations
  - Setup issue: Fix initialization
  - Dependency issue: Database/external service

### 3. Fix

One test at a time:

1. Minimal fix to compile
2. Verify: `go vet`
3. Test: `go test -v ./path/to/package -run TestName`
4. Iterate incrementally
5. Use `editor` agent for complex fixes

### 4. Validation

- Verify no new issues: `go vet ./...`
- Ensure no regressions: `make test`

## Best Practices

- Fix compilation first
- One test at a time
- NEVER delete tests unless asked
- Fix root cause, not symptoms
- Use `editor` agent for complex fixes