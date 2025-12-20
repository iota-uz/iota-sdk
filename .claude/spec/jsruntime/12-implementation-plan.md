# JavaScript Runtime - Implementation Plan

## Overview

This document provides a phased implementation plan for the JavaScript Runtime feature, breaking down the work into manageable phases with clear dependencies, acceptance criteria, and testing requirements.

## Implementation Phases

### Phase 1: Core Runtime Infrastructure (Foundation)

**Goal**: Build the foundational JavaScript execution engine with basic sandboxing and resource controls.

**Duration**: 1-2 weeks

#### Tasks

- [ ] Create `pkg/jsruntime/` package structure
- [ ] Implement `VM` struct with `goja` integration
- [ ] Implement `VMPool` with acquire/release pattern
- [ ] Add basic sandboxing (remove dangerous globals)
- [ ] Implement `ResourceLimits` configuration
- [ ] Add timeout enforcement via context cancellation
- [ ] Add memory usage monitoring
- [ ] Implement panic recovery with stack traces
- [ ] Add console output capture
- [ ] Write unit tests (>80% coverage)

#### Files to Create

```
pkg/jsruntime/
├── vm.go                    # VM struct, sandboxing, execution
├── vm_pool.go               # VM pool implementation
├── resource_limits.go       # ResourceLimits, ExecutionContext
├── console.go               # Console output capture
├── vm_test.go               # Unit tests
├── vm_pool_test.go          # Pool tests
└── resource_limits_test.go  # Limits tests
```

#### Acceptance Criteria

- ✅ Can execute simple JavaScript with `console.log()`
- ✅ Dangerous globals (`eval`, `fetch`, `require`) are undefined
- ✅ Execution timeouts after configured duration
- ✅ Panic recovery prevents VM crashes
- ✅ Console output is captured and returned
- ✅ VM pool can acquire/release VMs concurrently
- ✅ Memory usage monitoring works (approximate)
- ✅ Unit tests pass with >80% coverage

#### Testing Requirements

```go
// Test Cases
- TestVMSandboxing_DangerousGlobalsRemoved
- TestVMSandboxing_EvalBlocked
- TestVMSandboxing_FunctionConstructorBlocked
- TestVMExecution_SimpleScript
- TestVMExecution_ConsoleOutput
- TestVMExecution_Timeout
- TestVMExecution_PanicRecovery
- TestVMPool_AcquireRelease
- TestVMPool_ConcurrentAccess
- TestResourceLimits_Timeout
- TestResourceLimits_MemoryCheck
```

#### Dependencies

- None (foundational phase)

---

### Phase 2: Domain Model & Persistence (Data Layer)

**Goal**: Implement domain entities, repositories, and database schema for scripts, executions, and versions.

**Duration**: 1-2 weeks

#### Tasks

- [ ] Create `modules/scripts/` module structure
- [ ] Implement `Script` aggregate with interface pattern
- [ ] Implement `Execution` entity
- [ ] Implement `ScriptVersion` entity
- [ ] Define repository interfaces in domain layer
- [ ] Create database migrations (scripts, executions, script_versions)
- [ ] Implement `ScriptRepository` in infrastructure/persistence
- [ ] Implement `ExecutionRepository`
- [ ] Implement `ScriptVersionRepository`
- [ ] Add database indexes for performance
- [ ] Write integration tests for repositories

#### Files to Create

```
modules/scripts/
├── domain/
│   ├── script/
│   │   ├── script.go          # Script interface
│   │   ├── script_impl.go     # Private implementation
│   │   └── repository.go      # Repository interface
│   ├── execution/
│   │   ├── execution.go       # Execution interface
│   │   ├── execution_impl.go
│   │   └── repository.go
│   └── version/
│       ├── version.go         # ScriptVersion interface
│       ├── version_impl.go
│       └── repository.go
├── infrastructure/
│   └── persistence/
│       ├── script_repository.go     # Implementation
│       ├── execution_repository.go
│       ├── version_repository.go
│       ├── script_repository_test.go
│       ├── execution_repository_test.go
│       └── version_repository_test.go
└── permissions/
    └── permissions.go         # RBAC permissions

migrations/
├── XXXXXXXXXX_create_scripts.sql
├── XXXXXXXXXX_create_executions.sql
└── XXXXXXXXXX_create_script_versions.sql
```

#### Database Schema

```sql
-- scripts table
CREATE TABLE scripts (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    org_id INTEGER NOT NULL,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    source TEXT NOT NULL,
    trigger_type VARCHAR(20) NOT NULL CHECK (trigger_type IN ('manual', 'scheduled', 'webhook', 'event')),
    schedule VARCHAR(100),
    webhook_path VARCHAR(255),
    event_type VARCHAR(100),
    enabled BOOLEAN NOT NULL DEFAULT true,
    version INTEGER NOT NULL DEFAULT 1,
    last_executed_at TIMESTAMP,
    execution_count INTEGER NOT NULL DEFAULT 0,
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX idx_scripts_tenant ON scripts(tenant_id);
CREATE INDEX idx_scripts_scheduled ON scripts(tenant_id, trigger_type, enabled, last_executed_at)
    WHERE trigger_type = 'scheduled' AND enabled = true;
CREATE INDEX idx_scripts_webhook ON scripts(tenant_id, trigger_type, webhook_path)
    WHERE trigger_type = 'webhook' AND enabled = true;

-- executions table
CREATE TABLE executions (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    script_id INTEGER NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    input TEXT,
    output TEXT,
    error_message TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_executions_script ON executions(script_id, started_at DESC);
CREATE INDEX idx_executions_status ON executions(tenant_id, status, started_at DESC);

-- script_versions table
CREATE TABLE script_versions (
    id SERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    script_id INTEGER NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    source TEXT NOT NULL,
    created_by INTEGER NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE(script_id, version)
);

CREATE INDEX idx_script_versions ON script_versions(script_id, version DESC);
```

#### Acceptance Criteria

- ✅ Scripts can be created, read, updated, deleted
- ✅ Executions are tracked with status transitions
- ✅ Script versions are created on update
- ✅ Tenant isolation enforced (all queries include tenant_id)
- ✅ Unique constraints prevent duplicate script names per tenant
- ✅ Foreign key constraints maintain referential integrity
- ✅ Indexes improve query performance
- ✅ Repository tests pass with >80% coverage

#### Testing Requirements

```go
// Repository Tests
- TestScriptRepository_Create
- TestScriptRepository_FindByID
- TestScriptRepository_FindAll_WithFilters
- TestScriptRepository_Update
- TestScriptRepository_Delete
- TestScriptRepository_TenantIsolation
- TestScriptRepository_UniqueNameConstraint
- TestExecutionRepository_Create
- TestExecutionRepository_FindByScriptID
- TestExecutionRepository_UpdateStatus
- TestVersionRepository_Create
- TestVersionRepository_FindByScriptID
```

#### Dependencies

- Phase 1 (Core Runtime Infrastructure)

---

### Phase 3: Service Layer (Business Logic)

**Goal**: Implement business logic for script and execution management with permission checks, validation, and event publishing.

**Duration**: 1-2 weeks

#### Tasks

- [ ] Implement `ScriptService` with CRUD operations
- [ ] Implement `ExecutionService` with execution orchestration
- [ ] Add permission checks via `sdkcomposables.CanUser()`
- [ ] Add validation (cron schedule, webhook path, script name)
- [ ] Implement version creation on script update
- [ ] Add event publishing for script lifecycle
- [ ] Add transaction management for multi-step operations
- [ ] Implement execution status transitions
- [ ] Write service tests with ITF framework

#### Files to Create

```
modules/scripts/
├── services/
│   ├── script_service.go
│   ├── execution_service.go
│   ├── script_service_test.go
│   └── execution_service_test.go
└── events/
    └── events.go              # Event type definitions
```

#### Service Methods

**ScriptService**:
```go
- Create(ctx, CreateParams) (Script, error)
- FindByID(ctx, id) (Script, error)
- FindAll(ctx, FindParams) ([]Script, int, error)
- Update(ctx, id, UpdateParams) (Script, error)
- Delete(ctx, id) error
- Enable(ctx, id) error
- Disable(ctx, id) error
```

**ExecutionService**:
```go
- Execute(ctx, scriptID, input) (Execution, error)
- FindByID(ctx, executionID) (Execution, error)
- FindByScriptID(ctx, scriptID, params) ([]Execution, int, error)
- CancelExecution(ctx, executionID) error
```

#### Acceptance Criteria

- ✅ Users without `Script.Create` permission cannot create scripts
- ✅ Users without `Script.Execute` permission cannot execute scripts
- ✅ Invalid cron expressions are rejected
- ✅ Script name validation enforces alphanumeric + underscore/hyphen
- ✅ Script updates create new version entries
- ✅ Events published on script create/update/delete/execute
- ✅ Executions transition through pending→running→completed/failed
- ✅ Service tests pass with >80% coverage

#### Testing Requirements

```go
// Service Tests
- TestScriptService_Create_Success
- TestScriptService_Create_PermissionDenied
- TestScriptService_Create_InvalidName
- TestScriptService_Create_InvalidCron
- TestScriptService_Update_CreatesVersion
- TestScriptService_Delete_RequiresPermission
- TestExecutionService_Execute_Success
- TestExecutionService_Execute_PermissionDenied
- TestExecutionService_Execute_ScriptNotFound
- TestExecutionService_Execute_StatusTransitions
```

#### Dependencies

- Phase 1 (Core Runtime Infrastructure)
- Phase 2 (Domain Model & Persistence)

---

### Phase 4: API Bindings (JavaScript SDK)

**Goal**: Implement SDK API bindings that scripts can call (`sdk.http.*`, `sdk.db.*`, etc.) with security controls.

**Duration**: 1-2 weeks

#### Tasks

- [ ] Implement `ctx` global (tenantId, userId, orgId, input)
- [ ] Implement `sdk.http.*` API with SSRF protection
- [ ] Implement `sdk.db.*` API with tenant isolation
- [ ] Implement `sdk.cache.*` API with key prefixing
- [ ] Implement `sdk.log.*` API with output capture
- [ ] Implement `events.publish()` API
- [ ] Generate TypeScript definitions (`sdk.d.ts`)
- [ ] Add API call counting for rate limiting
- [ ] Write binding tests

#### Files to Create

```
pkg/jsruntime/
├── bindings/
│   ├── context.go         # ctx global
│   ├── http_api.go        # sdk.http.*
│   ├── db_api.go          # sdk.db.*
│   ├── cache_api.go       # sdk.cache.*
│   ├── log_api.go         # sdk.log.*
│   ├── events_api.go      # events.publish()
│   ├── ssrf_protection.go # ValidateURL, SSRFProtectedTransport
│   ├── http_api_test.go
│   ├── db_api_test.go
│   └── cache_api_test.go
└── typescript/
    └── sdk.d.ts           # TypeScript definitions
```

#### API Surface

```javascript
// Context
ctx.tenantId: number
ctx.userId: number
ctx.orgId: number
ctx.input: any

// HTTP API
sdk.http.get(url, options?)
sdk.http.post(url, body, options?)
sdk.http.put(url, body, options?)
sdk.http.delete(url, options?)

// Database API
sdk.db.query(sql, params?)
sdk.db.execute(sql, params?)

// Cache API
sdk.cache.get(key)
sdk.cache.set(key, value, ttlSeconds?)
sdk.cache.delete(key)

// Logging API
sdk.log.info(message, ...args)
sdk.log.warn(message, ...args)
sdk.log.error(message, ...args)

// Events API
events.publish(eventType, payload)
```

#### Acceptance Criteria

- ✅ Scripts can access `ctx.tenantId`, `ctx.userId`, `ctx.input`
- ✅ HTTP requests to private IPs (127.0.0.1, 192.168.x.x) are blocked
- ✅ Database queries require tenant_id filter
- ✅ Cache keys are automatically prefixed with tenant_id
- ✅ Log output is captured and returned
- ✅ Events are published with tenant metadata
- ✅ TypeScript definitions provide autocomplete in Monaco
- ✅ API call limits are enforced (100 calls per execution)
- ✅ Binding tests pass with >80% coverage

#### Testing Requirements

```go
// Binding Tests
- TestHTTPAPI_Get_Success
- TestHTTPAPI_SSRFProtection_LocalhostBlocked
- TestHTTPAPI_SSRFProtection_PrivateIPBlocked
- TestDBAPI_Query_RequiresTenantID
- TestDBAPI_Query_TenantIsolation
- TestCacheAPI_KeyPrefixing
- TestLogAPI_OutputCapture
- TestEventsAPI_PublishWithMetadata
- TestAPICallLimits_Exceeded
```

#### Dependencies

- Phase 1 (Core Runtime Infrastructure)
- Phase 2 (Domain Model & Persistence)
- Phase 3 (Service Layer)

---

### Phase 5: Presentation Layer (UI)

**Goal**: Build user interface for script management with Monaco editor integration and execution history.

**Duration**: 1-2 weeks

#### Tasks

- [ ] Implement `ScriptController` with CRUD handlers
- [ ] Create list template with filtering and pagination
- [ ] Create new/edit templates with Monaco editor
- [ ] Create view template with execution history
- [ ] Implement ViewModels (Script, Execution, Version)
- [ ] Add localization (en.toml, ru.toml, uz.toml)
- [ ] Integrate HTMX for table updates
- [ ] Add form validation with DTOs
- [ ] Register routes with auth middleware
- [ ] Write E2E tests with Playwright

#### Files to Create

```
modules/scripts/
├── presentation/
│   ├── controllers/
│   │   ├── script_controller.go
│   │   └── script_controller_test.go
│   ├── viewmodels/
│   │   ├── script_viewmodel.go
│   │   ├── execution_viewmodel.go
│   │   └── version_viewmodel.go
│   ├── templates/
│   │   └── pages/
│   │       └── scripts/
│   │           ├── index.templ
│   │           ├── new.templ
│   │           ├── edit.templ
│   │           ├── view.templ
│   │           ├── _table.templ
│   │           └── _execution_row.templ
│   └── locales/
│       ├── en.toml
│       ├── ru.toml
│       └── uz.toml

e2e/
└── tests/
    └── scripts/
        ├── create-script.spec.ts
        ├── edit-script.spec.ts
        ├── execute-script.spec.ts
        └── delete-script.spec.ts
```

#### Acceptance Criteria

- ✅ Users can view list of scripts with filtering
- ✅ Monaco editor loads with TypeScript definitions
- ✅ Users can create scripts with trigger type selection
- ✅ Users can edit scripts (creates new version)
- ✅ Users can execute scripts manually (Run button)
- ✅ Execution history updates via HTMX
- ✅ CSRF tokens included in forms
- ✅ Translations work in all 3 languages
- ✅ Permission middleware blocks unauthorized access
- ✅ E2E tests pass in CI mode

#### Testing Requirements

```go
// Controller Tests
- TestScriptController_List_Success
- TestScriptController_List_PermissionDenied
- TestScriptController_Create_ValidForm
- TestScriptController_Create_InvalidCron
- TestScriptController_Edit_LoadsExisting
- TestScriptController_Update_Success
- TestScriptController_Delete_RequiresPermission
- TestScriptController_Execute_ReturnsExecutionID
```

```typescript
// E2E Tests
- test('create script with manual trigger')
- test('create script with cron schedule')
- test('edit script source code')
- test('execute script and see result')
- test('delete script removes from list')
- test('permission denied shows error')
```

#### Dependencies

- Phase 3 (Service Layer)
- Phase 4 (API Bindings)

---

### Phase 6: Advanced Features (Production Polish)

**Goal**: Add production-ready features like scheduling, HTTP endpoints, monitoring, and health checks.

**Duration**: 2-3 weeks

#### Tasks

- [ ] Implement `Scheduler` with cron parsing
- [ ] Implement `EndpointRouter` for webhook triggers
- [ ] Add rate limiting per endpoint
- [ ] Implement Prometheus metrics
- [ ] Add compilation caching with LRU eviction
- [ ] Implement adaptive VM pool sizing
- [ ] Add health check endpoints (liveness/readiness)
- [ ] Optimize database queries with indexes
- [ ] Add dead letter queue for failed executions
- [ ] Write load tests and benchmarks
- [ ] Document all features

#### Files to Create

```
pkg/jsruntime/
├── scheduler.go
├── scheduler_test.go
├── endpoint_router.go
├── endpoint_router_test.go
├── rate_limiter.go
├── metrics.go
├── compilation_cache.go
├── adaptive_pool.go
├── health_checker.go
└── dead_letter_queue.go

docs/
├── javascript-runtime.md
├── api-reference.md
└── examples/
    ├── scheduled-backup.js
    ├── webhook-handler.js
    └── event-listener.js
```

#### Acceptance Criteria

- ✅ Scheduled scripts execute at correct times
- ✅ Webhook endpoints respond with script output
- ✅ Prometheus metrics are exposed and accurate
- ✅ Compilation cache reduces repeated parsing overhead
- ✅ VM pool adapts to load (scales up/down)
- ✅ Health checks return correct status
- ✅ Failed executions retry via dead letter queue
- ✅ Load tests show acceptable performance (>100 RPS)
- ✅ Documentation is complete and accurate

#### Testing Requirements

```go
// Advanced Tests
- TestScheduler_RunDueScripts
- TestScheduler_PreventConcurrentExecution
- TestEndpointRouter_DynamicRouting
- TestEndpointRouter_RateLimiting
- TestMetrics_RecordExecution
- TestCompilationCache_HitMiss
- TestAdaptivePool_Scaling
- TestHealthChecker_AllChecks
- BenchmarkScriptExecution
- TestConcurrentExecutions_100Goroutines
```

#### Dependencies

- Phase 5 (Presentation Layer)
- All previous phases complete

---

## Dependency Diagram

```
Phase 1: Core Runtime Infrastructure
    │
    ├──> Phase 2: Domain Model & Persistence
    │        │
    │        └──> Phase 3: Service Layer
    │                 │
    │                 ├──> Phase 4: API Bindings
    │                 │        │
    │                 │        └──> Phase 5: Presentation Layer
    │                 │                 │
    │                 │                 └──> Phase 6: Advanced Features
    │                 │
    │                 └──────────────────────┘
    │
    └──> Phase 4: API Bindings (parallel with Phase 2-3)
```

**Critical Path**: Phase 1 → Phase 2 → Phase 3 → Phase 5 → Phase 6

**Parallel Work**: Phase 4 can start after Phase 1 completes (doesn't depend on Phase 2/3)

---

## Testing Strategy

### Unit Tests (Per Phase)

- **Phase 1**: VM, VMPool, ResourceLimits
- **Phase 2**: Repositories (CRUD, tenant isolation)
- **Phase 3**: Services (permissions, validation, events)
- **Phase 4**: API Bindings (SSRF, tenant isolation)
- **Phase 5**: Controllers (form parsing, HTMX)
- **Phase 6**: Scheduler, EndpointRouter, Metrics

**Target**: >80% coverage per phase before moving to next

### Integration Tests

```go
// Test full stack integration
func TestFullStack_CreateAndExecuteScript(t *testing.T) {
    // Setup ITF environment
    // Create script via service
    // Execute script via service
    // Verify execution result
    // Check database state
}

func TestFullStack_ScheduledExecution(t *testing.T) {
    // Create scheduled script
    // Wait for scheduler tick
    // Verify execution occurred
}

func TestFullStack_WebhookTrigger(t *testing.T) {
    // Create webhook script
    // Send HTTP request to webhook
    // Verify script executed
    // Check response
}
```

### E2E Tests (Playwright)

```typescript
// e2e/tests/scripts/full-workflow.spec.ts
test('full script workflow', async ({ page }) => {
    // Login
    // Navigate to scripts
    // Create new script
    // Edit script source
    // Execute script
    // View execution history
    // Delete script
});
```

### Performance Tests

```go
// Benchmark script execution
func BenchmarkExecution_SimpleScript(b *testing.B)
func BenchmarkExecution_ComplexScript(b *testing.B)
func BenchmarkExecution_WithAPICalls(b *testing.B)

// Load test
func TestLoad_100ConcurrentExecutions(t *testing.T)
func TestLoad_SustainedThroughput(t *testing.T)
```

---

## Documentation Requirements

### User Documentation

1. **JavaScript Runtime Guide** (`docs/javascript-runtime.md`)
   - Overview and use cases
   - Creating your first script
   - Trigger types explained
   - SDK API reference
   - Security best practices

2. **API Reference** (`docs/api-reference.md`)
   - `ctx` object
   - `sdk.http.*` methods
   - `sdk.db.*` methods
   - `sdk.cache.*` methods
   - `sdk.log.*` methods
   - `events.publish()` method

3. **Examples** (`docs/examples/`)
   - Scheduled data backup script
   - Webhook notification handler
   - Event-triggered automation
   - HTTP API integration

### Developer Documentation

1. **Architecture Overview** (`docs/architecture/javascript-runtime.md`)
   - Component diagram
   - Data flow
   - Security model
   - Performance characteristics

2. **Implementation Guide** (`docs/implementation/`)
   - Adding new SDK APIs
   - Extending VM capabilities
   - Custom trigger types
   - Metrics and monitoring

---

## Rollout Plan

### Phase 1-4: Internal Testing (4-6 weeks)

- Deploy to development environment
- Internal testing by development team
- Performance benchmarking
- Security review

### Phase 5: Beta Release (2 weeks)

- Deploy to staging environment
- Limited beta access for trusted users
- Gather feedback
- Fix critical bugs

### Phase 6: Production Release (1 week)

- Deploy to production
- Monitor metrics closely
- Gradual rollout (feature flag)
- Full documentation published

---

## Success Metrics

### Performance Metrics

- **Script Execution Latency**: P50 < 50ms, P95 < 200ms, P99 < 500ms
- **Throughput**: >100 scripts/second per instance
- **VM Pool Utilization**: 60-80% average
- **Scheduler Precision**: ±1 minute for cron schedules

### Quality Metrics

- **Test Coverage**: >80% across all layers
- **Bug Density**: <1 critical bug per 1000 LOC
- **Uptime**: 99.9% availability
- **Error Rate**: <0.1% failed executions (excluding user errors)

### Adoption Metrics

- **Active Scripts**: >50 scripts created in first month
- **Executions**: >10,000 executions in first month
- **User Satisfaction**: >4.5/5 rating in feedback

---

## Risk Management

### Technical Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| VM memory leaks | High | Implement VM recycling, memory monitoring |
| SSRF bypass | Critical | Thorough security testing, DNS rebinding protection |
| Database performance | Medium | Add indexes, query optimization, connection pooling |
| Scheduler drift | Low | Use robust cron library, monitor precision |

### Operational Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Resource exhaustion | High | Resource limits, adaptive pool sizing, alerts |
| Malicious scripts | Critical | Sandboxing, permission checks, audit logs |
| Migration failures | Medium | Test migrations in staging, rollback plan |

---

## Conclusion

This implementation plan breaks down the JavaScript Runtime feature into **6 manageable phases** over approximately **10-14 weeks**:

1. **Phase 1** (1-2 weeks): Core runtime infrastructure
2. **Phase 2** (1-2 weeks): Domain model and persistence
3. **Phase 3** (1-2 weeks): Service layer
4. **Phase 4** (1-2 weeks): API bindings
5. **Phase 5** (1-2 weeks): Presentation layer
6. **Phase 6** (2-3 weeks): Advanced features

Each phase has:
- ✅ Clear tasks and deliverables
- ✅ Acceptance criteria
- ✅ Testing requirements
- ✅ Dependency mapping

Following this plan ensures a **systematic, testable, and production-ready implementation** of the JavaScript Runtime feature.

