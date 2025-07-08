---
allowed-tools: |
  Bash(go list:*), Bash(go vet:*), Bash(staticcheck:*),
  Bash(gofumpt:*), Bash(golines:*), Bash(go test:*),
  Bash(gosec:*), Bash(git diff:*), Bash(git apply:*)
description: Refactor a Go package using idiomatic design principles
---

## Context
- **Package root:** `$ARGUMENTS`
- **Directory tree:** !`go list -f '{{.Dir}}' "$ARGUMENTS"`
  !`tree -I 'vendor|testdata' $(go list -f '{{.Dir}}' "$ARGUMENTS")`
- **Static analysis:**
  - Vet: !`go vet "$ARGUMENTS/..."`
  - Staticcheck: !`staticcheck "$ARGUMENTS/..."`
  - Lint-friendly format: !`golines -m 120 $(go list -f '{{.Dir}}' "$ARGUMENTS")/**/*.go`
- **Tests & coverage (if present):** !`go test -cover "$ARGUMENTS/..."`

## Your task
You are a senior Go engineer. For *each* finding, include:

1. **Problem** – brief description and file:line reference.  
2. **Why it matters** – which principle it violates (e.g. large interface, concrete coupling, missing DI).  
3. **Suggested change** – annotated `git diff` (unified) ready to apply.  
4. **Rationale** – one-sentence justification linking back to Go or general best practice.

### Refactor checklist
- Analyze the package holistically, not just isolated files.
- You are allowed to break backwards compatibility.
- Keep interfaces minimal and consumer-defined; accept interfaces, return concrete types.  
- Inject dependencies through constructors or functional options, not global vars or `init()`.  
- Prefer struct embedding / composition; avoid deep type hierarchies.  
- Handle errors early; no hidden panics.  
- Maintain package-level boundaries (`internal/`, `cmd/`, `pkg/`).  
- Uphold SOLID/GRASP where they map naturally to Go.  
- Ensure tests remain green; update mocks or stubs as interfaces shrink.
- Find and eliminate dead code or unused functions.

Generate changes in batches no larger than ~400 lines per diff to keep the review manageable.

> After presenting the full set of diffs, append a short “next steps” list (e.g., run `go test`, update docs, run `go vet` again).

