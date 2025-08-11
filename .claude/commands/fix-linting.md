---
allowed-tools: |
  Bash(go list:*), Bash(go vet:*), Bash(staticcheck:*),
  Bash(gofumpt:*), Bash(golines:*), Bash(go test:*),
  Bash(gosec:*), Bash(git diff:*), Bash(git apply:*),
  Bash(make lint), Bash(make fmt), Bash(make test),
  Bash(go fmt:*), Bash(git status:*), Bash(git diff --cached:*)
description: Fix linting & formatting issues 
---

## Context
You are tasked with fixing linting and formatting issues in this repository.
Run `make lint` to identify issues, then apply the necessary fixes.

Launch subagents to parallelize work, make sure to devide the work evenly among them.

