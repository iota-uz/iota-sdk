# Claude Code Agents Reference

Complete reference for creating and managing specialized agents in Claude Code.

**Official Documentation:** https://docs.claude.com/en/docs/claude-code/sub-agents

## Agent Format

```markdown
---
allowed-tools: |
  Tool1, Tool2, Tool3
description: "When this agent should be invoked"
model: inherit  # Optional: inherit (default), sonnet, opus, or haiku
---

# Agent Name

## Role
Single, focused purpose (one responsibility only)

## When to Use
Specific triggering scenarios

## Responsibilities
- Focused task 1
- Focused task 2

## Knowledge Base
Domain-specific patterns and practices
```

### Key Features

- **Single Responsibility:** ONE clear purpose per agent
- **Context Isolation:** Separate windows prevent pollution
- **Clear Triggers:** Description defines activation conditions
- **Model Selection:** Choose optimal model for task (inherit, sonnet, opus, haiku)
- **Version Control:** Commit project agents to `.claude/agents/`

## Agent Locations

Agents can be stored in two locations:

| Location | Scope | Use Case | Committed to Git |
|----------|-------|----------|------------------|
| `.claude/agents/*.md` | Project-level | Shared team agents, project-specific workflows | ✅ Yes |
| `~/.claude/agents/*.md` | User-level | Personal agents, cross-project utilities | ❌ No |

**Best Practice:**
- **Project agents** - Domain experts (database, UI, testing) that team shares
- **User agents** - Personal productivity tools, custom workflows, learning aids

## Guidelines for Creating Agents

### Single Responsibility

- **ONE clear purpose** (non-negotiable)
- **Isolated Context:** Separate context windows prevent pollution
- **No Overlap:** Verify zero responsibility overlap with existing agents

### Description

- Write clear description: "When this agent should be invoked" in frontmatter
- Include triggering scenarios and conditions

### Domain Expertise

- Include relevant domain knowledge (e.g., IOTA SDK patterns)
- Document file-type mandates and boundaries
- Specify exact file patterns the agent handles

### Model Selection

Agents can specify which model to use:

| Model | Use Case | Cost | Recommendation |
|-------|----------|------|----------------|
| `inherit` | Use parent conversation's model | Varies | ✅ Default - recommended for most agents |
| `sonnet` | Balanced performance and cost | Medium | ✅ Best for almost all use cases |
| `opus` | Maximum capability for complex tasks | High | ⚠️ Rarely needed - only for extremely complex reasoning |
| `haiku` | Fast, lightweight tasks | Low | ⚠️ Limited capability - only for simple, well-defined tasks |

**Best Practice:** Use `inherit` (default) or explicitly set `sonnet`. Sonnet provides excellent performance for nearly all agent tasks. Only use `opus` for exceptionally complex reasoning tasks, or `haiku` for trivial operations.

### Tool Permissions

**Recommended patterns:**
- ✅ `Read(/path/to/module/**)` - Restrict to specific paths
- ✅ `Bash(go test:*)`, `Bash(go vet:*)` - Specific patterns
- ✅ Tool inheritance when appropriate

**Security risks:**
- ❌ `Bash(*)` - Never grant unrestricted access
- ❌ `Read` without path restriction - Overly permissive

## Built-in Agents

**Note:** `general-purpose` is a built-in agent type provided by Claude Code. It does not need to be defined in `.claude/agents/` and is always available for complex, multi-step tasks.

## Examples

### Example 1: Focused Module Agent

```markdown
---
allowed-tools: |
  Read(back/internal/modules/insurance/**), Bash(go test:*), Bash(go vet:*)
description: "Fix failing tests in insurance module only"
---

# Insurance Test Fixer

## Role
Fix failing unit tests in insurance module exclusively.

## When to Use
When `make test` shows insurance module test failures.

## Responsibilities
- Analyze test failures in back/internal/modules/insurance/
- Fix broken tests without changing business logic
- Ensure tests pass with `cd back && go test ./internal/modules/insurance/...`

## Knowledge Base
- Uses IOTA SDK testing patterns (itf.Setup, itf.MockDB)
- Follows table-driven test conventions
- Maintains test isolation and cleanup
```

### Example 2: Repository Agent

```markdown
---
allowed-tools: |
  Read(back/internal/**/*_repository.go), Read(back/internal/**/*_repository_test.go), Edit, Bash(go test:*)
description: "Manage repository layer: interfaces, implementations, and tests"
---

# Repository Expert

## Role
Create, update, and optimize repository layer code.

## When to Use
- Creating new repository interfaces or implementations
- Optimizing database queries
- Fixing repository-related bugs
- Writing repository tests

## Responsibilities
- Define repository interfaces in domain layer
- Implement repositories in infrastructure/persistence layer
- Write comprehensive repository tests
- Optimize query performance
- Ensure tenant isolation in queries

## Knowledge Base
- IOTA SDK repo patterns (QueryBuilder, filters)
- PostgreSQL query optimization
- Transaction handling with pgx
- Multi-tenant query patterns
```

### Example 3: UI Component Agent

```markdown
---
allowed-tools: |
  Read(back/internal/modules/**/*_controller.go), Read(back/internal/modules/**/*_viewmodel.go), Read(back/internal/modules/**/*.templ), Edit, Bash(templ generate:*)
description: "Manage UI layer: controllers, view-models, and templates"
---

# UI Editor

## Role
Create and update presentation layer components.

## When to Use
- Creating new pages or forms
- Updating controllers or view-models
- Modifying Templ templates
- Implementing HTMX interactions

## Responsibilities
- Build controllers with proper auth guards
- Create view-models for data transformation
- Write Templ templates using IOTA SDK components
- Implement HTMX workflows
- Handle i18n with translation keys

## Knowledge Base
- IOTA SDK component library (Form, Table, Modal)
- Templ syntax and best practices
- HTMX patterns (hx-get, hx-post, hx-target)
- Alpine.js for client-side reactivity
- Tailwind CSS design system
```

## Performance Considerations

**Context Rebuilding Latency:**
- Each agent starts with a fresh context window (not shared with main conversation)
- Initial agent invocation has slight latency while context is built
- Subsequent tool calls within same agent are faster
- Trade-off: Isolated context prevents pollution but requires rebuild time

**Optimization Tips:**
- Group related tasks in single agent invocation rather than multiple short invocations
- Use `inherit` or `sonnet` model for best balance of speed and capability
- Design agents with clear, focused responsibilities to minimize context size

## Best Practices

1. **One Responsibility:** Each agent handles exactly one domain
2. **Clear Boundaries:** Document which files/patterns the agent manages
3. **Minimal Permissions:** Grant only tools needed for the domain
4. **Domain Knowledge:** Include patterns, conventions, and best practices
5. **Triggering Conditions:** Make it clear when to invoke the agent
6. **No Overlap:** Verify no other agent handles the same files
7. **Model Selection:** Use `inherit` or `sonnet` unless you have specific needs

## Quick Reference Examples

### Minimal Agent with Restricted Scope

```markdown
---
allowed-tools: |
  Read(back/internal/modules/insurance/**), Bash(go test:*), Bash(go vet:*)
description: "Fix failing tests in insurance module only"
---

# Insurance Test Fixer

## Role
Fix failing unit tests in insurance module exclusively.

## When to Use
When `make test` shows insurance module test failures.

## Responsibilities
- Analyze test failures in back/internal/modules/insurance/
- Fix broken tests without changing business logic
```

**Key Points:**
- Restricted `Read` path (insurance module only)
- Limited Bash patterns (testing commands only)
- Single responsibility (test fixing)
- Clear triggering condition

### Multi-Layer Agent Example

```markdown
---
allowed-tools: |
  Read(back/**/*_controller.go), Read(back/**/*_viewmodel.go), Read(back/**/*.templ), Edit, Bash(templ generate:*)
description: "Manage presentation layer: controllers, view-models, templates"
---

# UI Editor

## Role
Create and update presentation layer components.

## When to Use
- Creating new pages or forms
- Updating controllers or view-models
- Modifying Templ templates

## Responsibilities
- Build controllers with proper auth guards
- Create view-models for data transformation
- Write Templ templates using IOTA SDK components

## Knowledge Base
- IOTA SDK component library
- Templ syntax and best practices
- HTMX patterns (hx-get, hx-post, hx-target)
```

**Key Points:**
- Multiple file patterns (controllers, viewmodels, templates)
- Layer-specific scope (presentation only)
- Domain knowledge included
- Clear file-type mandates

## Agent vs Command

Use agents when:
- You need context isolation from the main conversation
- The task requires specialized expertise
- You want parallel execution without interference
- The task involves complex, multi-step workflows

Use commands when:
- You need dynamic context loading at invocation time
- The task is a simple workflow that doesn't need isolation
- You want to extend Claude's interface with custom shortcuts

See `.claude/guides/claude-code/commands.md` for command creation guidance.
