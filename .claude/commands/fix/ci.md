---
description: Identify and fix failing CI workflows for current branch
model: sonnet
---

# Fix CI Failures

Systematically identify and fix failing CI workflows for the current branch.

## CI Status Overview

!`shy-tools ci failures`

## Workflow

### 1. Analyze Error Logs

Identify root cause from error logs above:
- **Compilation**: Type errors, undefined variables, imports
- **Test failures**: Assertions, logic errors, race conditions
- **Infrastructure**: Database, network, timeouts
- **Config**: Environment variables, permissions

### 2. Investigate & Fix

- Complex failures: Use `debugger` agent for analysis
- Implementation: Use `editor` agent for fixes
- Priority: Compilation errors first, then test failures

### 3. Validate

Local testing (if applicable):
```bash
cd back && go vet ./...
cd back && go test -v ./path/to/package -run TestName
```

Re-run workflow (ask user first):
```bash
gh run rerun <run-id>
gh run watch <run-id>
```

## Common Failure Patterns

**Backend (PostgreSQL/PgBouncer):**
- Health checks, connection pools, transaction isolation
- Missing fixtures, race conditions, timeouts (10m default)
- Environment variables, auth credentials, Go version mismatch

**Frontend (Docker Compose):**
- Build failures in `docker-compose.testing.yml`
- Service dependencies, network issues

## Additional Commands

```bash
# More context if needed
gh run view <run-id> --json jobs --jq '.jobs'  # All job statuses
gh run view <run-id> --log-failed              # Full failed logs
gh run view <run-id> --web                     # Open in browser
```
