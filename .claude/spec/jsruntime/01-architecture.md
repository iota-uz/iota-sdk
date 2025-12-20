# JavaScript Runtime - Architecture

## System Overview

The JavaScript Runtime module is a self-contained IOTA SDK module following Domain-Driven Design (DDD) principles. It integrates with the IOTA SDK application layer, EventBus, and database pool while maintaining clear boundaries and multi-tenant isolation.

## Component Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         IOTA SDK Application                            │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │                      JavaScript Runtime Module                     │  │
│  │                                                                    │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐            │  │
│  │  │ Presentation │  │   Service    │  │   Domain     │            │  │
│  │  │   Layer      │  │    Layer     │  │   Layer      │            │  │
│  │  ├──────────────┤  ├──────────────┤  ├──────────────┤            │  │
│  │  │ Controllers  │─▶│ ScriptSvc    │─▶│ Script       │            │  │
│  │  │ Templates    │  │ ExecutionSvc │  │ Execution    │            │  │
│  │  │ ViewModels   │  │ VMPoolSvc    │  │ Version      │            │  │
│  │  │ Locales      │  │ SchedulerSvc │  │ ValueObjects │            │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘            │  │
│  │                            │                  │                   │  │
│  │                            ▼                  ▼                   │  │
│  │                    ┌──────────────┐  ┌──────────────┐            │  │
│  │                    │Infrastructure│  │   Execution  │            │  │
│  │                    │   Layer      │  │   Engine     │            │  │
│  │                    ├──────────────┤  ├──────────────┤            │  │
│  │                    │ Repositories │  │ VMPool       │            │  │
│  │                    │ - Script     │  │ Sandbox      │            │  │
│  │                    │ - Execution  │  │ APIBindings  │            │  │
│  │                    │ - Version    │  │ ResourceMgr  │            │  │
│  │                    └──────────────┘  └──────────────┘            │  │
│  │                            │                  │                   │  │
│  └────────────────────────────┼──────────────────┼───────────────────┘  │
│                               │                  │                      │
│  ┌────────────────────────────┼──────────────────┼───────────────────┐  │
│  │         Shared Infrastructure                │                    │  │
│  ├────────────────────────────┼──────────────────┼───────────────────┤  │
│  │  pgxpool.Pool              │   EventBus       │   HTTP Router     │  │
│  │  (tenant-scoped queries)   │   (pub/sub)      │   (endpoints)     │  │
│  └────────────────────────────┴──────────────────┴───────────────────┘  │
└─────────────────────────────────────────────────────────────────────────┘
                                   │
                                   ▼
                         ┌──────────────────┐
                         │   PostgreSQL     │
                         ├──────────────────┤
                         │ scripts          │
                         │ executions       │
                         │ versions         │
                         │ subscriptions    │
                         │ scheduled_jobs   │
                         │ http_endpoints   │
                         │ dead_letters     │
                         └──────────────────┘
```

## Data Flow Diagrams

### 1. Scheduled Script Execution Flow

```
┌──────────┐      ┌──────────────┐      ┌──────────────┐      ┌──────────┐
│  Cron    │─────▶│  Scheduler   │─────▶│  VMPool      │─────▶│ Script   │
│  Ticker  │      │  Service     │      │  Service     │      │ Execution│
└──────────┘      └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Get Scheduled│      │ Acquire VM   │      │  Store   │
                  │ Scripts      │      │ from Pool    │      │  Results │
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Filter by    │      │ Inject APIs  │      │  Release │
                  │ Tenant       │      │ & Context    │      │  VM      │
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Check Next   │      │ Execute with │      │  Publish │
                  │ Run Time     │      │ Timeout      │      │  Event   │
                  └──────────────┘      └──────────────┘      └──────────┘
```

### 2. HTTP Endpoint Script Execution Flow

```
┌──────────┐      ┌──────────────┐      ┌──────────────┐      ┌──────────┐
│  HTTP    │─────▶│  Router      │─────▶│  VMPool      │─────▶│ Script   │
│  Request │      │  Middleware  │      │  Service     │      │ Execution│
└──────────┘      └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Authenticate │      │ Acquire VM   │      │  Build   │
                  │ & Authorize  │      │ for Tenant   │      │  Response│
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Lookup Script│      │ Inject HTTP  │      │  Return  │
                  │ by Path      │      │ Request Obj  │      │  to User │
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Check Status │      │ Execute with │      │  Log     │
                  │ & Limits     │      │ Timeout      │      │  Execution│
                  └──────────────┘      └──────────────┘      └──────────┘
```

### 3. Event-Triggered Script Execution Flow

```
┌──────────┐      ┌──────────────┐      ┌──────────────┐      ┌──────────┐
│  Domain  │─────▶│  EventBus    │─────▶│  Script      │─────▶│  VMPool  │
│  Event   │      │  Handler     │      │  Matcher     │      │  Service │
└──────────┘      └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Publish to   │      │ Find Scripts │      │ Execute  │
                  │ Event Type   │      │ by Event Type│      │ in Pool  │
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Filter by    │      │ Filter by    │      │  Store   │
                  │ Tenant       │      │ Tenant       │      │  Results │
                  └──────────────┘      └──────────────┘      └──────────┘
                         │                      │                    │
                         ▼                      ▼                    ▼
                  ┌──────────────┐      ┌──────────────┐      ┌──────────┐
                  │ Queue for    │      │ Execute Each │      │  Retry   │
                  │ Execution    │      │ Script       │      │  on Fail │
                  └──────────────┘      └──────────────┘      └──────────┘
```

## Key Design Decisions

### 1. Why Goja?

**Decision**: Use Goja as the JavaScript runtime engine.

**Rationale:**
- **Pure Go Implementation**: No CGO dependencies, simplified deployment, cross-platform compatibility
- **Memory Safety**: Runs in Go's memory-safe environment, preventing buffer overflows and memory corruption
- **Excellent Go Interoperability**: Seamless mapping between Go and JavaScript types, easy API binding
- **Active Maintenance**: Regular updates, responsive maintainers, strong community
- **ECMAScript 5.1+ Compatibility**: Covers majority of business logic use cases
- **Built-in Optimizations**: JIT compilation, inline caching, escape analysis

**Alternatives Considered:**
- **V8 (via V8Go)**: Rejected due to CGO requirement, complex deployment, larger binary size
- **QuickJS**: Rejected due to C dependency, less mature Go bindings
- **Otto**: Rejected due to abandoned maintenance, ECMAScript 5 only, slower performance

**Trade-offs Accepted:**
- Slower execution compared to V8 (acceptable for business logic, not compute-intensive tasks)
- Limited ES6+ features (workarounds available, most features via polyfills)
- No native async/await (use callbacks or synchronous patterns)

### 2. Why VM Pooling?

**Decision**: Implement a pool of pre-warmed Goja VMs.

**Rationale:**
- **Reduced Latency**: Pre-warmed VMs eliminate cold-start overhead (100ms+ → <10ms)
- **Resource Isolation**: One tenant per VM prevents cross-tenant interference
- **Fair Scheduling**: Pool management ensures fair access across tenants
- **Memory Efficiency**: Reuse VMs across executions, reducing GC pressure
- **Graceful Degradation**: Pool size auto-adjusts under load

**Implementation Strategy:**
- Initial pool size: 10 VMs (configurable via environment)
- Expansion: Add VMs on demand up to max (100 VMs default)
- Idle timeout: 5 minutes of inactivity → VM released
- Per-tenant limit: Max 5 concurrent VMs per tenant (prevent monopolization)
- Warm-up: Pre-load standard library and common APIs

**Pool States:**
```
Available → Acquired → In-Use → Released → Available
                                     ↓
                                 (timeout)
                                     ↓
                              Destroyed
```

### 3. Multi-Tenant Architecture

**Decision**: Enforce tenant isolation at all layers (database, VM, API).

**Rationale:**
- **Security**: Zero cross-tenant data leaks, even under VM compromise
- **Compliance**: Data residency requirements, audit trails per tenant
- **Fair Resource Usage**: Prevent one tenant from starving others
- **Billing**: Per-tenant execution metrics for usage-based pricing

**Implementation:**
- **Database**: `tenant_id` in all tables, row-level security policies
- **VM Context**: Tenant ID injected into global scope, all APIs check tenant context
- **API Bindings**: Database queries automatically scoped to `tenant_id` via composables
- **Execution Tracking**: Separate execution logs per tenant
- **Resource Limits**: Per-tenant quotas (executions/hour, concurrent VMs, storage)

**Tenant Context Injection:**
```javascript
// Available in all scripts via global context
const tenantId = context.tenantId;
const userId = context.userId;
const organizationId = context.organizationId;

// API calls automatically scoped
db.query("SELECT * FROM users"); // Only returns users in current tenant
http.get("/api/data"); // Includes tenant context in request
```

### 4. Integration Points with IOTA SDK

#### 4.1 Application Interface

**Integration**: JavaScript Runtime module registers with `application.Application`.

**Pattern:**
```go
// modules/jsruntime/module.go
type Module struct {
    app            application.Application
    scriptService  services.ScriptService
    vmPoolService  services.VMPoolService
    scheduler      services.SchedulerService
}

func (m *Module) Register(app application.Application) error {
    // Register HTTP routes
    m.registerRoutes(app.Router())

    // Subscribe to lifecycle events
    app.EventBus().Subscribe("app.started", m.onAppStarted)
    app.EventBus().Subscribe("app.stopping", m.onAppStopping)

    // Start background services
    go m.scheduler.Start(app.Context())
    go m.vmPoolService.Start(app.Context())

    return nil
}

func (m *Module) onAppStarted(event eventbus.Event) {
    // Warm up VM pool
    m.vmPoolService.WarmUp(10)

    // Load scheduled scripts
    m.scheduler.LoadSchedules()
}

func (m *Module) onAppStopping(event eventbus.Event) {
    // Graceful shutdown
    m.scheduler.Stop()
    m.vmPoolService.Drain()
}
```

#### 4.2 EventBus Integration

**Integration**: Subscribe to domain events for event-triggered scripts.

**Pattern:**
```go
// modules/jsruntime/services/event_handler_service.go
type EventHandlerService struct {
    eventBus          eventbus.EventBus
    scriptRepo        domain.ScriptRepository
    executionService  ExecutionService
}

func (s *EventHandlerService) Start(ctx context.Context) error {
    // Subscribe to wildcard pattern
    s.eventBus.Subscribe("*", s.handleEvent)
    return nil
}

func (s *EventHandlerService) handleEvent(event eventbus.Event) {
    ctx := event.Context()
    tenantID := composables.UseTenantID(ctx)

    // Find scripts subscribed to this event type
    scripts, err := s.scriptRepo.GetByEventType(ctx, event.Type())
    if err != nil {
        return
    }

    // Execute each script
    for _, script := range scripts {
        go s.executionService.ExecuteAsync(ctx, script.ID(), event.Data())
    }
}
```

#### 4.3 Database Pool Integration

**Integration**: Use shared `pgxpool.Pool` with composables for tenant isolation.

**Pattern:**
```go
// modules/jsruntime/infrastructure/persistence/script_repository.go
type ScriptRepository struct {
    pool *pgxpool.Pool
}

func (r *ScriptRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Script, error) {
    const op serrors.Op = "ScriptRepository.GetByID"

    tx := composables.UseTx(ctx)
    pool := composables.UsePool(ctx, r.pool)
    tenantID := composables.UseTenantID(ctx)

    const query = `
        SELECT id, tenant_id, name, description, source, type, status,
               resource_limits, cron_expression, http_path, http_methods,
               event_types, metadata, created_at, updated_at, created_by
        FROM scripts
        WHERE id = $1 AND tenant_id = $2
    `

    var row ScriptRow
    err := tx.QueryRow(ctx, query, id, tenantID).Scan(...)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    return MapRowToScript(row), nil
}
```

## Module Structure (DDD)

```
modules/jsruntime/
├── domain/
│   ├── aggregates/
│   │   └── script/
│   │       ├── script.go              # Script aggregate interface
│   │       └── script_impl.go         # Script implementation (private)
│   ├── entities/
│   │   ├── execution/
│   │   │   ├── execution.go           # Execution entity interface
│   │   │   └── execution_impl.go      # Execution implementation
│   │   └── version/
│   │       ├── version.go             # Version entity interface
│   │       └── version_impl.go        # Version implementation
│   ├── value_objects/
│   │   ├── script_type.go             # ScriptType enum
│   │   ├── script_status.go           # ScriptStatus enum
│   │   ├── execution_status.go        # ExecutionStatus enum
│   │   ├── resource_limits.go         # ResourceLimits struct
│   │   ├── cron_expression.go         # CronExpression value object
│   │   └── trigger_data.go            # TriggerData value object
│   ├── events/
│   │   ├── script_events.go           # Script lifecycle events
│   │   └── execution_events.go        # Execution events
│   └── repositories/
│       ├── script_repository.go       # Script repository interface
│       ├── execution_repository.go    # Execution repository interface
│       └── version_repository.go      # Version repository interface
├── services/
│   ├── script_service.go              # Script CRUD service
│   ├── script_service_test.go         # Service tests
│   ├── execution_service.go           # Execution orchestration
│   ├── vmpool_service.go              # VM pool management
│   ├── scheduler_service.go           # Cron scheduler
│   └── event_handler_service.go       # Event subscription
├── infrastructure/
│   ├── persistence/
│   │   ├── script_repository.go       # Script repo implementation
│   │   ├── script_repository_test.go  # Repo tests
│   │   ├── execution_repository.go    # Execution repo implementation
│   │   ├── version_repository.go      # Version repo implementation
│   │   └── mappers.go                 # DB row to domain mappers
│   ├── runtime/
│   │   ├── vm_pool.go                 # VM pool implementation
│   │   ├── sandbox.go                 # Goja sandbox wrapper
│   │   └── bindings/
│   │       ├── console.go             # console.log API
│   │       ├── database.go            # db.query API
│   │       └── http.go                # http.get/post API
│   └── scheduler/
│       └── cron_scheduler.go          # Cron job scheduler
├── presentation/
│   ├── controllers/
│   │   ├── script_controller.go       # Script CRUD endpoints
│   │   ├── script_controller_test.go  # Controller tests
│   │   ├── execution_controller.go    # Execution monitoring
│   │   └── editor_controller.go       # Monaco editor integration
│   ├── viewmodels/
│   │   ├── script_viewmodel.go        # Script UI transformation
│   │   └── execution_viewmodel.go     # Execution UI transformation
│   ├── templates/
│   │   ├── pages/
│   │   │   ├── scripts/
│   │   │   │   ├── index.templ        # Script listing
│   │   │   │   ├── create.templ       # Create script form
│   │   │   │   ├── edit.templ         # Edit script (Monaco)
│   │   │   │   └── show.templ         # Script details
│   │   │   └── executions/
│   │   │       ├── index.templ        # Execution history
│   │   │       └── show.templ         # Execution details
│   │   └── components/
│   │       ├── script_form.templ      # Reusable form component
│   │       └── monaco_editor.templ    # Editor wrapper
│   └── locales/
│       ├── en.toml                    # English translations
│       ├── ru.toml                    # Russian translations
│       └── uz.toml                    # Uzbek translations
└── module.go                          # Module registration
```

## Performance Considerations

**VM Pool Tuning:**
- Monitor pool hit rate (target: >95%)
- Adjust pool size based on CPU cores (2x cores as starting point)
- Use worker pool pattern for parallel execution
- Implement circuit breaker for overload protection

**Database Query Optimization:**
- Index on `(tenant_id, status)` for active script lookups
- Index on `(tenant_id, type, status)` for type-based filtering
- Partial index on `cron_expression IS NOT NULL` for scheduler
- Composite index on `(tenant_id, http_path, http_methods)` for endpoint lookup
- Use `EXPLAIN ANALYZE` to validate query plans

**Memory Management:**
- Set max heap size per VM (default: 64MB)
- Monitor memory usage via runtime.MemStats
- Implement memory pressure eviction (release idle VMs)
- Use sync.Pool for frequently allocated objects

**Concurrency Control:**
- Use worker pool with bounded goroutines (max: 1000)
- Implement semaphore for concurrent executions per tenant
- Use context.WithTimeout for all script executions
- Graceful shutdown with drain period (30 seconds)

## Security Architecture

**Defense in Depth:**
1. **Input Validation**: Sanitize script source, validate resource limits
2. **Sandbox Isolation**: Goja VM with restricted global scope
3. **API Authorization**: Check tenant context in all API bindings
4. **Database Isolation**: Row-level security with `tenant_id`
5. **Resource Limits**: CPU timeout, memory limit, rate limiting
6. **Audit Trail**: Log all script changes and executions

**Attack Surface Mitigation:**
- No `eval()` or `Function()` constructor in user scripts
- No access to Go runtime internals
- No file system or network access (except controlled HTTP client)
- No reflection or code generation APIs
- All API bindings go through authorization layer

## Monitoring & Observability

**Metrics to Track:**
- Script executions per second (by tenant, by type)
- VM pool utilization (available, in-use, total)
- Execution duration (p50, p95, p99)
- Error rate (by error type)
- Memory usage per VM
- Queue depth for pending executions

**Logging:**
- Script creation/update/delete with user ID
- Execution start/end with duration
- Errors with stack trace and line number
- Resource limit violations
- API call logs (sanitized for PII)

**Alerting:**
- Error rate > 5% for 5 minutes
- VM pool exhaustion (all VMs in use)
- Execution timeout spike
- Memory usage > 80% of limit
- Database connection pool exhaustion
