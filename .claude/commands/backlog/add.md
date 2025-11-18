---
description: "Add high-quality task to backlog with multi-role expert planning"
model: sonnet
disable-model-invocation: true
---

You help the user build high-quality, decision-focused backlog items by acting as UI/UX expert, product manager, and technical lead. Follow this workflow:

## Phase 1: Context Gathering

Ask: "What task would you like to add to the backlog? Provide a brief description or context."

Wait for user response. Then ask: "What type of task is this?"

Use AskUserQuestion with options:
- **Feature** - New functionality or enhancement
- **Bug Fix** - Error, failure, or incorrect behavior
- **Refactor** - Code quality, simplification, optimization
- **Performance** - Slow queries, latency, resource issues

## Phase 2: Multi-Role Expert Analysis

Based on task type and context, launch agents in parallel to gather critical information.

Use `researcher` agent when a task involves library/framework research (IOTA SDK components, external APIs, Go libraries).

### Expert Analysis by Task Type

**All Tasks - Tech Lead Analysis:**

Launch `Explore` agent (thoroughness: high, model: sonnet):
```
Act as Tech Lead analyzing [TASK]. Find:

Critical files (exact paths):
- Controllers, services, repositories, migrations affected
- Existing similar features to follow

Architectural decisions:
- What patterns to follow vs avoid
- What to reuse vs build new
- Database schema changes needed
- Integration points (which services/methods to call)

Technical constraints:
- RBAC permissions required
- Multi-tenancy considerations (tenant_id isolation)
- Performance requirements (indexes, query optimization)
- Security constraints

Exclude:
- How to write Go code
- Standard CRUD operations
- Obvious validation rules
```

**Features with UI - UI/UX Expert Analysis:**

Launch `Explore` agent (thoroughness: medium, model: haiku):
```
Analyze UI/UX patterns for [FEATURE]. Find in presentation/templates/:

- Existing similar features (forms, modals, multi-step flows)
- User flow patterns (happy path + edge cases)
- Error handling in templates
- HTMX + Alpine.js patterns for this use case

Focus ONLY on presentation layer patterns.
```

**All Tasks - Product Manager Analysis:**

Launch `Plan` agent (model: sonnet):
```
Act as PM defining scope for [TASK]. Provide:

- Business value (1-2 sentences - why this matters)
- Acceptance criteria (specific, testable)
- Scope boundaries (what's IN vs OUT of scope)
- Dependencies or blockers
- Success metrics

Be concise. Focus on WHAT to build, not HOW.
```

**Parallel execution patterns:**
- Feature with UI: `Explore(tech) & Explore(ui) & Plan(pm)` (add `& researcher` if library research needed)
- Backend-only: `Explore(tech) & Plan(pm)` (add `& researcher` if library research needed)
- Bug fix: `Explore(tech)` (add `& researcher` if library research needed)

## Phase 3: Decision Synthesis

After agents complete, extract key decisions into categories:

**Architectural Decisions:**
- Patterns to follow (existing repository/service to reuse)
- DDD layer responsibilities
- What to build new vs reuse

**Scope Decisions:**
- IN scope: What's included
- OUT of scope: What's explicitly excluded and why

**Technical Constraints:**
- Database (migrations, indexes, constraints)
- RBAC permissions required
- Multi-tenancy (tenant_id handling)
- Performance (query optimization, caching)
- Security (validation, authorization)

**UX Decisions:** *(UI tasks only)*
- Flow type (multistep vs single page)
- Validation approach (inline vs on-submit)
- UI pattern to follow (modal, drawer, inline)

**Integration Decisions:**
- Which existing services/methods to call
- Events to emit (eventbus patterns)
- API contracts or interface requirements

**Choke Points:**
- Migration dependencies (must run before deploy)
- Blocking PRs or external dependencies
- Required database indexes

Present synthesized decisions to the user for review. Allow modifications before finalizing.

## Phase 4: Template Population

Generate a backlog item using an adaptive template. Apply exclusion filter.

### Adaptive Template

```markdown
[agent:AGENT_TYPE]
[model:MODEL]

## Task
[One-line objective - clear, specific, actionable]

## Context
[Business value in 1-2 sentences]

## Key Decisions

### Architectural
- [Pattern/approach decision]
- [What to reuse vs build new]

### Scope
- IN: [What's included]
- OUT: [What's excluded and why]

### Technical
- [Database/schema decision]
- [Performance/security constraint]

## Critical Files
- `path/to/controller.go` - [Why it matters]
- `path/to/service.go` - [Pattern to follow]
- `path/to/repository.go` - [Method to use]
- `migrations/YYYYMMDDHHMMSS_name.sql` - [Schema change]

## User Flow
*(UI tasks only)*
1. [Happy path step]
2. [Edge case handling]

## Acceptance Criteria
- [ ] [Specific, testable criterion]
- [ ] [Edge case handling verified]
- [ ] [Integration point tested]

## Technical Notes
*(Include if applicable)*
- [API contract requirement]
- [RBAC permission needed]
- [Event to emit]
- [Library-specific pattern from researcher]
```

### Exclusion Filter

**EXCLUDE** (agents can discover in <30s):
- How to write Go code syntax
- Standard CRUD operation patterns
- Obvious validation rules (email format, required fields)
- Template/HTMX/Alpine.js syntax
- Generic error handling patterns
- Standard logging approaches
- Import statements
- File organization within modules (DDD structure)
- Translation key creation process

**INCLUDE** (critical decisions):
- Specific architectural patterns to follow
- Scope boundaries and exclusions
- Technical constraints (cross-tenant uniqueness, not just validation)
- Integration points (which existing services to call)
- Critical file paths agent must modify
- Non-obvious business rules
- Performance requirements (indexes, query optimization)
- Security constraints (RBAC, data isolation)
- Library-specific patterns (IOTA SDK component APIs)

**Filter examples:**

REMOVE: "Create controller method that handles POST requests"
KEEP: "Use LoadController.Update pattern, not Create (load exists)"

REMOVE: "Add validation for email format"
KEEP: "Email unique across ALL tenants, not per-tenant (global constraint)"

REMOVE: "Return 404 if entity not found"
KEEP: "Soft-delete: hide from UI but keep in DB for audit compliance"

REMOVE: "Handle database errors"
KEEP: "If unique constraint violated, return error code 409 for frontend"

Present template to user: "Based on research, here's the backlog item. Review and modify if needed."

## Phase 5: Agent & Model Selection

Use AskUserQuestion for agent type and model selection.

**Question 1: Agent Type**

"Which agent should execute this task?"

Options:
- **editor** - All backend work: domain, services, repositories, migrations, controllers, ViewModels, templates, translations
- **refactoring-expert** - Code quality, simplification, optimization
- **qa-tester** - Testing, bug detection, quality assurance
- **debugger** - Error investigation, debugging, issue diagnosis
- **general-purpose** - Multi-faceted tasks requiring multiple capabilities

**Question 2: Model Complexity**

"What model complexity is needed?"

Options:
- **haiku** - Well-defined task with clear decisions already made (recommended after thorough planning)
- **sonnet** - Complex task requiring architectural thinking and judgment
- **inherit** - Use parent conversation's model (default)

**Selection guidance:**
- Backlog item has detailed decisions → haiku
- Backlog item has open architectural questions → sonnet
- Default after thorough planning → haiku (decisions captured)

## Phase 6: File Creation

Available backlog items:
!`ls -1 .claude/backlog/*.md 2>/dev/null | sort -n | tail -1 || echo "No backlog items yet"`

1. Determine the next sequence number (001 if none exists, else increment highest)
2. Generate a slug from the task title (first 40-50 chars, lowercase, remove special chars, hyphens for spaces)
3. Create filename: `.claude/backlog/{SEQ}-{SLUG}.md`
   - Pad sequence with zeros (001, 002, etc.)
   - Example: `.claude/backlog/003-add-multi-driver-assignment.md`
4. Write a file with:
   - `[agent:SELECTED_AGENT_TYPE]` from Phase 5
   - `[model:SELECTED_MODEL]` from Phase 5
   - Populated template content from Phase 4
5. Confirm: "Task added to backlog as `{FILENAME}`"

## Guidelines Summary

**Principle:** If an agent with codebase access can discover it in <30 seconds, don't include it.

**Focus on:**
- Architectural patterns chosen from existing code
- Scope boundaries (IN vs OUT)
- Technical constraints (performance, security, compliance)
- Critical file paths where changes are needed
- Non-obvious business rules
- Integration points (services/methods to call)
- Database decisions (schema, constraints, indexes)
- UX flow decisions (multi-step, validation approach)
- RBAC requirements
- Event emissions
- Library-specific patterns (IOTA SDK components, external APIs)

**Exclude:**
- Standard syntax and CRUD patterns
- Obvious validation rules
- Generic error handling
- File structure (agents know DDD layers)
- Translation processes

Be conversational and helpful. Use agents proactively for well-researched, decision-focused backlog items.
