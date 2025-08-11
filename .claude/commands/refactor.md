---
allowed-tools: |
  Bash(go list:*), Bash(go vet:*), Bash(staticcheck:*),
  Bash(gofumpt:*), Bash(golines:*), Bash(go test:*),
  Bash(gosec:*), Bash(git diff:*), Bash(git apply:*),
  Bash(git status:*), Read, Glob, Grep
description: Refactor changed files in the current git diff using idiomatic design principles
---

## Context
- **Git status:** !`git status --porcelain`
- **Changed files:** !`git diff --name-only HEAD`
- **Git diff:** !`git diff HEAD`
- **Static analysis for changed Go files:**
  - Vet: !`go vet ./...`
  - Staticcheck (if available): !`staticcheck ./...`
- **Tests for affected packages:** !`go test -cover ./...`

## Your task
You are a senior Go engineer analyzing the current git diff. Focus on the changed files and their context. For *each* finding, include:

1. **Problem** – brief description and file:line reference.  
2. **Why it matters** – which principle it violates (e.g. large interface, concrete coupling, missing DI).  
3. **Suggested change** – annotated `git diff` (unified) ready to apply.  
4. **Rationale** – one-sentence justification linking back to Go or general best practice.

### Refactor checklist
- Focus on the files in the current git diff and their immediate context.
- Analyze the changed files holistically, considering their relationships.
- You are allowed to break backwards compatibility if it improves the design.
- Keep interfaces minimal and consumer-defined; accept interfaces, return concrete types.  
- Inject dependencies through constructors or functional options, not global vars or `init()`.  
- Prefer struct embedding / composition; avoid deep type hierarchies.  
- Handle errors early; no hidden panics.  
- Maintain package-level boundaries (`internal/`, `cmd/`, `pkg/`).  
- Uphold SOLID/GRASP where they map naturally to Go.  
- Ensure tests remain green; update mocks or stubs as interfaces shrink.
- Find and eliminate dead code or unused functions in the changed files.

Generate changes in batches no larger than ~400 lines per diff to keep the review manageable.

> After presenting the full set of diffs, append a short “next steps” list (e.g., run `go test`, update docs, run `go vet` again).

