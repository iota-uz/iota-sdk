# IOTA SDK - Claude Code Orchestrator Configuration

## Design Philosophy
iota-sdk is a general purpose ERP building engine/solution. When designing anything inside iota-sdk:
- Make it extensible, generalizable, and customizable
- Prefer interfaces over concrete structs at boundaries
- Apply dependency inversion and inject interfaces
- Keep domain and services decoupled from infrastructure details

## Quick Decision Tree

**Task Classification:**
- **1-3 files**: No agents needed - use Read/Edit/Bash directly
- **4-15 files**: 1-2 agents (small-medium features)
- **16-30 files**: 3-4 agents parallel (large features)
- **30+ files**: 5-7+ agents (cross-module)

**Agent Selection:**
- Errors/Failures → `debugger` first
- Go code → `auditor` last (always)
- Database/Schema → `editor`  
- UI/Templates → `editor`
- Production → `auditor` always

## Build/Test Commands

```bash
# After Go changes
go vet ./...

# After template/css changes  
templ generate && just css

# Testing
just test -v                       # All tests (verbose)
go test -v ./path -run TestName    # Single test

# Other
just check tr                      # Validate translations
just db migrate up                 # Apply migrations
```

**Never run `go build`** - use `go vet` instead.

## Module Architecture (DDD)

```
modules/{module}/
├── domain/aggregates/{entity}/    # Entity + events + repository interface
├── infrastructure/persistence/    # DB models, mappers, repository impl
├── services/                      # Business logic orchestration
├── presentation/
│   ├── controllers/               # HTTP handlers + DTOs
│   ├── templates/pages/{entity}/  # list.templ, edit.templ, new.templ
│   ├── viewmodels/                # Presentation models
│   └── locales/                   # en.json, ru.json, uz.json
├── module.go                      # Registration
├── links.go                       # Navigation
└── permissions/constants.go       # RBAC
```

## Creating New Entities

1. **Domain**: `modules/{m}/domain/aggregates/{entity}/` - entity, impl, events, repository interface
2. **Infrastructure**: `infrastructure/persistence/{entity}_repository.go` + models + mappers
3. **Service**: `services/{entity}_service.go` with `NewEntityService(repo, eventPublisher)` constructor
4. **Presentation**: controller + DTOs + viewmodel + mapper
5. **Templates**: `templates/pages/{entity}/` - list.templ, edit.templ, new.templ
6. **Localization**: Add keys to en/ru/uz.json
7. **Registration**: Add to `module.go`, `links.go`
8. **Verify**: `go vet ./...` and `templ generate && just css`

## Multi-Agent Orchestration

**Launch agents in parallel** (same message) with specific scope:

```
# Bug fix
debugger && editor && auditor

# New feature  
editor && auditor

# Database changes
editor && auditor

# Multi-module (parallel)
(editor & editor & editor) && auditor

# Research needed
general-purpose && editor && auditor
```

**Workflow Rules:**
- Always analyze scope first: `go vet ./...`, `find . -name "*.templ"`
- Divide work evenly between agents of same type
- Scale: 1-5 files → 1 agent, 6-15 → 3 agents, 16-30 → 5 agents, 31+ → 7-10 agents
- **>10 agents degrades coordination** - avoid

**Critical coordination rule:**
- **Before agent completes**: Agent must create tasks for ANY unfinished work (stubs, TODOs, partial implementations)
- **After agents complete**: Orchestrator must call `TaskList` and either implement remaining tasks or create new agents
- **Never assume**: Agents completed everything unless explicitly verified via `TaskList`

## E2E Testing

**ALWAYS use `e2e-tester` agent for:**
- Writing Playwright tests
- Debugging failing tests
- Creating fixtures/page objects

```bash
just e2e run      # Interactive UI mode
just e2e ci       # Headless CI mode
cd e2e && npx playwright test tests/module/specific.spec.ts  # Single test
```

## Technology Stack

- **Backend**: Go 1.23.2, IOTA SDK framework, GraphQL
- **Database**: PostgreSQL 13+ (multi-tenant with organization_id)
- **Frontend**: HTMX + Alpine.js + Templ + Tailwind CSS
- **Auth**: Cookie-based sessions with RBAC

## Core Rules

### Task Management (NEVER leave unfinished work unmarked)

**CRITICAL: Do NOT leave stubs, partial implementations, or TODO comments without creating tasks.**

Prohibited patterns:
- `// TODO: implement validation`
- `// FIXME: handle error case`
- `// This will be implemented later`
- Function stubs with `return nil, nil` or `panic("not implemented")`
- Commented-out code awaiting completion
- Partial feature implementations without tracking

When you identify unfinished work, immediately use `TaskCreate`:
- **subject**: "Implement validation for user input"
- **description**: Full context, affected files (e.g., `user_service.go:123`), acceptance criteria
- **activeForm**: "Implementing validation for user input"

Common scenarios requiring TaskCreate:
- Feature partially implemented (stub functions, incomplete logic)
- Missing validation or error handling
- Tests not yet written
- Refactoring needed after POC
- Documentation updates required
- Edge cases not handled

**Agent workflow**: Before completing, agents MUST:
1. Search their changes for stubs/incomplete code
2. Create tasks for ALL unfinished work with file:line references
3. Return task IDs in completion message

**Orchestrator workflow**: After agents complete:
1. Call `TaskList` to verify all unfinished work is tracked
2. Implement tasks or assign to new agents
3. Show user final task list

**Exception**: Preserve existing TODO comments when editing legacy code (but never add new ones).

### Other Rules

- **Multi-tenant isolation**: Always include `tenant_id` in WHERE clauses
- **Error handling**: Use `pkg/serrors` - `serrors.E(op, err)`
- **HTMX**: Check `htmx.IsHxRequest(r)`, use `htmx.SetTrigger(w, "event", payload)`
- **Never read `*_templ.go` files** - they're generated

## Child Module CLAUDE.md Files

- `modules/bichat/CLAUDE.md` - BiChat module specifics
- `modules/bichat/agents/CLAUDE.md` - Agent implementations
- `pkg/bichat/CLAUDE.md` - BiChat foundation framework
- `pkg/lens/CLAUDE.md` - Dashboard framework
