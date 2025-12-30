---
layout: default
title: Technical Spec
parent: JS Runtime
grand_parent: Specifications
nav_order: 3
description: "Technical architecture and implementation details for the JavaScript Runtime"
---

# Technical Spec: JavaScript Runtime

**Status:** Implementation Ready

## Architecture Overview

The JavaScript Runtime module is a self-contained IOTA SDK module following Domain-Driven Design (DDD) principles. It provides JavaScript execution capabilities with Goja VM engine, multi-tenant isolation, cron scheduling, HTTP endpoints, and event-driven script execution.

```mermaid
graph TB
    subgraph "IOTA SDK Application"
        subgraph "JavaScript Runtime Module"
            subgraph "Presentation Layer"
                Controllers[Controllers]
                Templates[Templates]
                ViewModels[ViewModels]
                Locales[Locales]
            end

            subgraph "Service Layer"
                ScriptSvc[Script Service]
                ExecutionSvc[Execution Service]
                VMPoolSvc[VM Pool Service]
                SchedulerSvc[Scheduler Service]
                EventHandlerSvc[Event Handler Service]
            end

            subgraph "Domain Layer"
                Script[Script Aggregate]
                Execution[Execution Entity]
                Version[Version Entity]
                ValueObjects[Value Objects]
            end

            subgraph "Infrastructure Layer"
                Repositories[Repositories]
                VMPool[VM Pool]
                Sandbox[Sandbox]
                APIBindings[API Bindings]
            end
        end

        subgraph "Shared Infrastructure"
            DBPool[pgxpool.Pool]
            EventBus[EventBus]
            HTTPRouter[HTTP Router]
        end
    end

    Database[(PostgreSQL)]

    Controllers --> ScriptSvc
    Controllers --> ExecutionSvc
    ScriptSvc --> Script
    ExecutionSvc --> Execution
    Script --> Repositories
    Execution --> Repositories
    ScriptSvc --> VMPoolSvc
    VMPoolSvc --> VMPool
    VMPool --> Sandbox
    Repositories --> DBPool
    DBPool --> Database
    SchedulerSvc --> EventBus
    EventBus --> Database
```

**Integration Points:**
- **Application Lifecycle**: Module registers routes, subscribes to lifecycle events (`app.started`, `app.stopping`)
- **EventBus**: Subscribes to all domain events, triggers matching scripts asynchronously
- **Database Pool**: Shares `pgxpool.Pool` with tenant isolation via composables
- **HTTP Router**: Registers script management routes and dynamic script endpoints

**Design Decision: Goja Runtime Engine**
- Pure Go implementation (no CGO dependencies)
- Memory safety via Go's environment
- Excellent Go-JavaScript interoperability
- ECMAScript 5.1+ compatibility with optimizations
- Trade-off: Slower than V8, but simpler deployment and cross-platform compatibility

## Implementation

### Domain Layer

```mermaid
classDiagram
    class Script {
        +UUID ID
        +UUID TenantID
        +String Name
        +String Source
        +ScriptType Type
        +ScriptStatus Status
        +ResourceLimits Limits
        +Create()
        +Update()
        +Activate()
        +Pause()
    }

    class Execution {
        +UUID ID
        +UUID ScriptID
        +ExecutionStatus Status
        +TriggerType Trigger
        +JSONB Input
        +JSONB Output
        +Start()
        +Complete()
        +Fail()
    }

    class Version {
        +UUID ID
        +UUID ScriptID
        +Int VersionNumber
        +String Source
        +Timestamp CreatedAt
    }

    Script "1" --> "*" Execution
    Script "1" --> "*" Version
```

**Aggregates:**
- **Script**: Root entity with execution triggers, resource limits, status
  - Functional options pattern for construction
  - Immutable setters (return new instances)
  - Business rules: name uniqueness per tenant, type-specific validation
  - Statuses: draft, active, paused, disabled, archived

**Entities:**
- **Execution**: Single script run with input/output, status, metrics
  - Lifecycle: pending → running → completed/failed/timeout
  - Captures duration, memory usage, API call count
- **Version**: Immutable snapshot of script source for audit trail
  - Sequential versioning (1, 2, 3, ...)
  - Created automatically on script create/update

**Value Objects:**
- **ScriptType**: scheduled, http, event, oneoff, embedded
- **ScriptStatus**: draft, active, paused, disabled, archived
- **ExecutionStatus**: pending, running, completed, failed, timeout, cancelled
- **ResourceLimits**: timeout, memory, API calls, output size
- **CronExpression**: cron syntax validation and next run calculation
- **TriggerData**: HTTP path/methods, event types, cron schedule

**Repository Interfaces** (defined in domain):
- `ScriptRepository`: CRUD, type-specific queries, pagination, search
- `ExecutionRepository`: CRUD, status queries, cleanup
- `VersionRepository`: retrieval, auto-increment versioning

### Service Layer

```mermaid
sequenceDiagram
    participant Client
    participant ExecutionSvc as Execution Service
    participant VMPool as VM Pool
    participant Sandbox
    participant Script

    Client->>ExecutionSvc: Execute(scriptID, input)
    ExecutionSvc->>ExecutionSvc: Create Execution (pending)
    ExecutionSvc->>VMPool: Acquire VM
    VMPool-->>ExecutionSvc: VM instance
    ExecutionSvc->>ExecutionSvc: Update status (running)
    ExecutionSvc->>Sandbox: Execute with timeout
    Sandbox->>Script: Run JavaScript
    Script-->>Sandbox: Result/Error
    Sandbox-->>ExecutionSvc: Output + Metrics
    ExecutionSvc->>VMPool: Release VM
    ExecutionSvc->>ExecutionSvc: Update status (completed/failed)
    ExecutionSvc-->>Client: Execution result
```

**Script Service:**
- CRUD operations with business validation
- Automatic version creation on updates
- Permission checks via `sdkcomposables.CanUser()`
- Name and HTTP path uniqueness validation
- Type-specific validation (cron, HTTP, event requirements)

**Execution Service:**
- Orchestrates script execution lifecycle
- Acquires VM from pool, executes with timeout
- Captures output, metrics, errors
- Updates execution status (pending → running → completed/failed)
- Publishes domain events (ExecutionStarted, ExecutionCompleted)

**VM Pool Service:**
- Manages VM lifecycle (create, warm-up, acquire, release, cleanup)
- Pool sizing: initial 10 VMs, max 100 VMs, per-tenant limit 5 VMs
- Idle timeout: 5 minutes → VM destroyed
- Warm-up: pre-load standard library and SDK APIs

**Scheduler Service:**
- Finds scripts due to run every minute
- Calculates next run time from cron expression
- Prevents overlapping executions via `is_running` lock
- Triggers execution via Execution Service
- Updates `next_run_at` and `last_run_status`

**Event Handler Service:**
- Subscribes to all domain events via EventBus wildcard (`*`)
- Matches events to scripts by event type and tenant
- Executes matching scripts asynchronously with event payload
- Retry logic: exponential backoff (2s, 4s, 8s), max 3 attempts
- Dead Letter Queue: captures permanently failed events

### Infrastructure Layer

**Repositories** (PostgreSQL):
- Interfaces in domain, implementations in infrastructure
- `composables.UseTx()` for transactions
- `composables.UseTenantID()` for tenant isolation (all queries include `WHERE tenant_id = $1`)
- Parameterized queries ($1, $2), SQL as constants
- `pkg/repo.QueryBuilder` for dynamic filters
- Mappers: database rows → domain entities (handle JSONB, arrays, nullables)

**VM Pool:**

```mermaid
stateDiagram-v2
    [*] --> Available: Create
    Available --> Acquired: Acquire
    Acquired --> Executing: Start Execution
    Executing --> Released: Complete
    Released --> Available: Reset
    Released --> Destroyed: Idle Timeout
    Available --> Destroyed: Idle Timeout
    Destroyed --> [*]
```

- Pre-warmed Goja VMs for reduced latency (100ms+ → <10ms)
- VM states: available → acquired → executing → released → destroyed
- Dynamic expansion under load, contraction during idle
- Fair scheduling: round-robin across tenants

**Sandbox:**
- Restricted global scope (no `eval`, `require`, `import`, `fs`, `process`)
- Allowed globals: `console`, `JSON`, `Math`, `Date`, standard types
- Injected SDK APIs: `ctx.db`, `ctx.http`, `ctx.cache`, `ctx.events`, `ctx.logger`
- Frozen context objects to prevent tampering

**API Bindings:**
- Database: query, insert, update, delete (tenant-scoped automatically)
- HTTP: get, post, put, delete (SSRF protection)
- Cache: get, set, delete
- Events: publish
- Logger: info, warn, error

**Runtime Engine:**
- Compilation cache: LRU 1000 programs, cache key = source hash, 90%+ hit rate
- Resource limits: timeout (30s), memory (64MB), API calls (100), output (1MB)
- Error handling: syntax, runtime, timeout, panic recovery
- Performance: cold start <500ms, warm start <100ms, cached <50ms

**Cron Scheduler:**
- Standard 5-field cron syntax (minute, hour, day, month, weekday)
- Timezone support (default: UTC)
- Concurrency prevention via `sync.Map` lock

### Presentation Layer

**Controllers:**
- Script CRUD: list, create, update, delete
- Execution: manual trigger, view history
- HTTP endpoint handler: dynamic script-based routes
- Auth middleware via `middleware.Authorize()`
- DI with `di.H` for service dependencies

**ViewModels:**
- Transform domain entities to UI-friendly structures
- Located in `modules/jsruntime/presentation/viewmodels/`
- Pure transformation logic (no business logic)

**Templates:**
- Script listing, create/edit forms, execution history
- Monaco Editor integration for code editing
- HTMX interactions via `pkg/htmx` package
- CSRF tokens in forms

**Translations:**
- Multi-language support: en.toml, ru.toml, uz.toml
- Hierarchical keys: `JSRuntime.Form.FieldName`
- Enum patterns: `JSRuntime.Enums.ScriptType.SCHEDULED`

## Permissions

**Permission Keys:**
- `scripts.read` - View scripts and execution history
- `scripts.create` - Create new scripts
- `scripts.update` - Edit existing scripts
- `scripts.delete` - Delete scripts
- `scripts.execute` - Manually execute scripts

**Role Access:**

| Role | Access Level |
|------|--------------|
| Superadmin | Full access (all tenants) |
| Org Admin | Full access (own organization) |
| Developer | Read, Create, Execute (no Delete) |
| Viewer | Read only |

## Performance Considerations

```mermaid
graph LR
    subgraph "Performance Targets"
        COLD[Cold Start<br/>&lt;500ms]
        WARM[Warm Start<br/>&lt;100ms]
        CACHED[Cached<br/>&lt;50ms]
        POOL[Pool Hit<br/>&gt;95%]
        THROUGHPUT[Throughput<br/>1000+ concurrent]
    end

    style COLD fill:#f59e0b,stroke:#d97706,color:#fff
    style WARM fill:#10b981,stroke:#047857,color:#fff
    style CACHED fill:#3b82f6,stroke:#1e40af,color:#fff
```

**VM Pool Optimization:**
- Target pool hit rate: >95%
- Pool size: 2x CPU cores (starting point)
- Utilization target: 60-80%
- Worker pool pattern for parallel execution
- Circuit breaker for overload protection

**Database Query Optimization:**
- Index on `(tenant_id, status)` for active script lookups
- Index on `(tenant_id, type, status)` for type filtering
- Partial index on `cron_expression IS NOT NULL` for scheduler
- Composite index on `(tenant_id, http_path, http_methods)` for HTTP routing
- Use `EXPLAIN ANALYZE` to validate query plans

**Memory Management:**
- Max heap size per VM: 64MB
- Monitor via `runtime.MemStats`
- Memory pressure eviction (release idle VMs)
- `sync.Pool` for frequently allocated objects

**Concurrency Control:**
- Worker pool with bounded goroutines (max: 1000)
- Semaphore for concurrent executions per tenant
- `context.WithTimeout` for all script executions
- Graceful shutdown with drain period (30 seconds)

**Compilation Cache:**
- LRU cache: 1000 programs
- Cache hit rate: >90% target
- Invalidation on script updates
- Optional bytecode persistence to database

## Security Considerations

```mermaid
graph TB
    subgraph "Defense in Depth"
        L1[Input Validation]
        L2[RBAC Permissions]
        L3[Tenant Isolation]
        L4[VM Sandboxing]
        L5[Resource Limits]
        L6[SSRF Protection]
        L7[Audit Trail]
    end

    L1 --> L2 --> L3 --> L4 --> L5 --> L6 --> L7

    style L1 fill:#3b82f6,stroke:#1e40af,color:#fff
    style L3 fill:#10b981,stroke:#047857,color:#fff
    style L4 fill:#f59e0b,stroke:#d97706,color:#fff
    style L6 fill:#ef4444,stroke:#b91c1c,color:#fff
```

**Defense in Depth Layers:**

1. **Input Validation**
   - Validate types, lengths, formats, required fields
   - Sanitize strings (strip HTML, escape SQL, no path traversal)
   - Script code max 100KB

2. **RBAC Permissions**
   - Route middleware checks permissions
   - Service layer checks permissions (defense in depth)
   - Template rendering hides unauthorized actions

3. **Tenant Isolation**
   - `tenant_id` in all database queries via `composables.UseTenantID(ctx)`
   - Row-level security at database level
   - Zero cross-tenant data access possible

4. **VM Sandboxing**
   - Remove dangerous globals: `eval`, `Function`, `require`, `import`, `fs`, `process`
   - Allow safe globals: `console`, `JSON`, `Math`, `Date`
   - Freeze injected context objects

5. **Resource Limits**
   - Execution timeout: 30s (configurable)
   - Memory limit: 64MB (configurable)
   - API call limit: 100 calls per execution
   - Output size: 1MB max

6. **SSRF Protection**
   - Block private IPs: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
   - Block loopback: 127.0.0.0/8, ::1/128
   - Block cloud metadata: 169.254.169.254
   - DNS resolution validates all IPs before HTTP request

7. **Audit Trail**
   - Log all script changes (create, update, delete)
   - Log all executions (input, output, errors, metrics)
   - Retention: 90 days minimum
   - Immutable logs (append-only)

**Threat Mitigation:**
- Code Injection: Mitigated by VM sandboxing (no eval)
- SSRF: Mitigated by IP validation and DNS checks
- Resource Exhaustion: Mitigated by resource limits
- Data Leakage: Mitigated by tenant isolation
- Privilege Escalation: Mitigated by RBAC enforcement

## Dependencies

**Internal:**
- `modules/core` - User, organization, tenant management
- `pkg/composables` - Tenant context, transactions, pagination
- `pkg/htmx` - HTMX helpers for controllers
- `pkg/repo` - Dynamic query builder
- `pkg/serrors` - Error handling
- `application.Application` - Module registration, lifecycle
- `EventBus` - Event subscriptions and publishing

**External:**
- `github.com/dop251/goja` - JavaScript runtime engine
- `github.com/robfig/cron/v3` - Cron expression parsing
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/prometheus/client_golang` - Metrics export

## Testing Strategy

| Test Type | Focus | Coverage |
|-----------|-------|----------|
| **Unit** | Domain entities, value objects, mappers | Validation, business rules |
| **Repository** | CRUD, tenant isolation, constraints | Data access patterns |
| **Service** | Business logic, permissions, versioning | End-to-end workflows |
| **Integration** | Cron scheduler, event bus, HTTP endpoints | System interactions |
| **Performance** | VM pool, latency, throughput | SLO compliance |
| **Security** | Cross-tenant, SSRF, sandbox escape | Threat mitigation |

## Open Questions

- Should we support ES6+ features via Babel transpilation?
- What retention period for execution logs in production?
- Should we allow user-defined npm packages (with sandboxing)?
- How to handle long-running scripts (>30s)?
- Should we support WebAssembly execution in addition to JavaScript?

---

## Next Steps

- Review [Data Model](./data-model.md) for database schema
- See [API Schema](./api-schema.md) for endpoint definitions
- Check [Decisions](./decisions.md) for technology choices
