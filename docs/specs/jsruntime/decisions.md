# Decisions Log: JavaScript Runtime

**Status:** Draft

## Decisions

| Date | Decision | Options | Chosen | Rationale |
|------|----------|---------|--------|-----------|
| 2024-12-21 | JavaScript Engine | V8 (via cgo), QuickJS, Goja, Otto | Goja | Pure Go (no CGO), memory-safe, active maintenance, ES6 support |
| 2024-12-21 | VM Pooling Strategy | On-demand creation, Global pool, Per-tenant pools | Global pool with per-tenant quotas | Simpler implementation, fair resource allocation, easier monitoring |
| 2024-12-21 | Sandboxing Approach | OS-level (containers), Language-level (remove globals), Process isolation | Language-level sandboxing | Lightweight, low overhead, sufficient for scripting use case |
| 2024-12-21 | Event Processing | Synchronous, Async queue with workers, Stream processing (Kafka) | Async queue with workers | Decouples event publishing from script execution, enables retry logic, scales horizontally |
| 2024-12-21 | Script Versioning | Git integration, Database snapshots, No versioning | Database snapshots | Simpler to implement, no external dependencies, sufficient for audit trail |
| 2024-12-21 | Code Editor | Textarea, CodeMirror, Monaco Editor, Ace Editor | Monaco Editor | Industry standard (VS Code), IntelliSense support, diff view, extensible |
| 2024-12-21 | API Binding Style | REST-like SDK wrappers, Direct repository access, GraphQL | REST-like SDK wrappers | Abstraction prevents breaking changes, enforces tenant isolation, familiar API |

## Decision Details

### 2024-12-21: JavaScript Engine Selection

**Context:** Need to execute user-defined JavaScript code in a secure, multi-tenant Go application. Engine must be embeddable, memory-safe, and support modern JavaScript features.

**Options:**

1. **V8 (via cgo):**
   - **Pros:** Industry-standard, fastest performance, full ECMAScript support, Chrome DevTools integration
   - **Cons:** Requires CGO (complicates builds), large binary size (~20MB), memory management complexity, harder to debug

2. **QuickJS:**
   - **Pros:** Small footprint, fast startup, ES2020 support
   - **Cons:** Requires CGO, less mature Go bindings, limited community support

3. **Goja (Pure Go):**
   - **Pros:** No CGO dependencies, excellent Go interoperability, active maintenance, ES6 support (classes, arrow functions, template literals), memory-safe execution
   - **Cons:** Slower than V8/QuickJS, limited ES2015+ features (no async/await, Promises require polyfill)

4. **Otto:**
   - **Pros:** Pure Go, simple API
   - **Cons:** Unmaintained (last commit 2021), ES5 only, poor performance

**Decision:** Goja

**Rationale:**
- **Zero CGO:** Simplified deployment across platforms (Linux, macOS, Windows) without C compiler dependencies
- **Go Native:** Direct access to Go types, functions, and structs without serialization overhead
- **Active Maintenance:** Regular updates, security patches, community support
- **Sufficient Performance:** For scripting use case (automation, webhooks), not CPU-intensive workloads
- **Memory Safety:** Go's garbage collector handles VM cleanup, preventing memory leaks
- **Trade-off Accepted:** Limited ES2015+ features acceptable since most automation scripts use ES5/ES6 subset

### 2024-12-21: VM Pooling Strategy

**Context:** Creating new Goja VM instances on every script execution incurs ~500ms cold-start latency. Need to reduce latency while maintaining tenant isolation and resource fairness.

**Options:**

1. **On-Demand Creation:**
   - **Pros:** Simple implementation, no idle resource consumption
   - **Cons:** High latency (500ms per execution), poor user experience

2. **Global Pool (Shared):**
   - **Pros:** Low latency (<100ms), efficient resource usage, simple management
   - **Cons:** Requires per-tenant quotas to prevent resource hogging, fairness challenges

3. **Per-Tenant Pools:**
   - **Pros:** Perfect isolation, predictable performance per tenant
   - **Cons:** Complex implementation, high idle resource consumption (100s of tenants × 10 VMs = 1000s of VMs), unfair for low-usage tenants

**Decision:** Global pool with per-tenant quotas

**Rationale:**
- **Performance:** Pre-warmed VMs reduce p95 latency from 500ms to <100ms
- **Simplicity:** Single pool manager, easier monitoring and debugging
- **Fairness:** Per-tenant concurrent execution limits (e.g., max 10 scripts/tenant) prevent resource starvation
- **Scalability:** Pool size adjusts dynamically based on demand (min 10, max 100 VMs)
- **Future-Proof:** Can migrate to per-tenant pools later if multi-tenancy requires stricter isolation

**Implementation Notes:**
- Pool maintains 10 warm VMs by default
- Idle VMs garbage collected after 5 minutes
- Tenant quota: max 10 concurrent executions, max 100 executions/minute
- Metrics: `vm_pool_size`, `vm_pool_acquisitions`, `vm_pool_wait_time`

### 2024-12-21: Sandboxing Approach

**Context:** User-defined scripts must not access file system, spawn processes, or make unrestricted network calls. Need to balance security with usability.

**Options:**

1. **OS-Level Sandboxing (Docker containers, gVisor):**
   - **Pros:** Strongest isolation, kernel-level security
   - **Cons:** High overhead (500MB per container), slow startup (1-2s), complex orchestration

2. **Language-Level Sandboxing (Remove dangerous globals):**
   - **Pros:** Lightweight, low latency, sufficient for scripting
   - **Cons:** Requires careful implementation, potential bypasses if globals not fully removed

3. **Process Isolation (Separate process per script):**
   - **Pros:** OS-level isolation, crash recovery
   - **Cons:** High overhead, slow IPC, complex error handling

**Decision:** Language-level sandboxing

**Rationale:**
- **Performance:** No container overhead, <1ms sandboxing latency
- **Simplicity:** Remove `eval`, `Function()`, `require`, native `fetch` from global scope
- **Controlled APIs:** Provide whitelisted HTTP client (`iota.http.get()`) with SSRF protection
- **Sufficient Security:** Scripts cannot escape VM, access file system, or spawn processes
- **Trade-offs Accepted:** Determined attackers may find bypasses, but risk is low for trusted tenant users

**Sandbox Rules:**
- ✅ Allowed: Pure JavaScript (loops, functions, objects, arrays, JSON, Math, Date)
- ✅ Allowed: SDK APIs (`iota.db.query()`, `iota.http.post()`, `console.log()`)
- ❌ Blocked: `eval()`, `Function()`, `require()`, `import`, `fetch()`, `XMLHttpRequest`
- ❌ Blocked: File system access (`fs`, `readFile`, `writeFile`)
- ❌ Blocked: Process spawning (`exec`, `spawn`, `child_process`)

### 2024-12-21: Event Processing Architecture

**Context:** Event-triggered scripts must execute asynchronously to avoid blocking domain services. Need reliable delivery with retry logic.

**Options:**

1. **Synchronous Execution:**
   - **Pros:** Simple implementation, immediate feedback
   - **Cons:** Blocks domain service, high latency, no retry on failure

2. **Async Queue with Workers:**
   - **Pros:** Decouples event publishing from script execution, enables retry, configurable worker pool
   - **Cons:** Moderate implementation complexity, eventual consistency

3. **Stream Processing (Kafka, RabbitMQ):**
   - **Pros:** High throughput, distributed processing, battle-tested
   - **Cons:** High operational complexity, external dependency, overkill for MVP

**Decision:** Async queue with workers

**Rationale:**
- **Decoupling:** Domain services publish events and continue immediately (no blocking)
- **Reliability:** Failed executions retry with exponential backoff (2s, 4s, 8s, 16s, 32s)
- **Scalability:** Worker pool size configurable (default 10 workers, max 100)
- **Simplicity:** In-memory queue for MVP (PostgreSQL-backed queue for production)
- **Future-Proof:** Can migrate to Kafka/RabbitMQ later if throughput exceeds 1000 events/sec

**Implementation Notes:**
- Worker pool consumes from buffered channel (size 1000)
- Retry logic: 5 attempts with exponential backoff (base 2s, max 32s)
- Dead Letter Queue (DLQ): Failed events after max retries moved to `script_execution_dlq` table
- Metrics: `events_published`, `events_processed`, `events_dlq`, `worker_pool_utilization`

### 2024-12-21: Script Versioning Strategy

**Context:** Need to track script code changes for audit trail, compliance, and rollback capability. Must support frequent updates without performance degradation.

**Options:**

1. **Git Integration (GitHub API, GitLab):**
   - **Pros:** Industry-standard version control, branching, pull requests
   - **Cons:** External dependency, complex API integration, not all users have Git knowledge

2. **Database Snapshots:**
   - **Pros:** Simple implementation, no external dependencies, sufficient for audit trail
   - **Cons:** No branching/merging, basic diff view only

3. **No Versioning:**
   - **Pros:** Simplest implementation
   - **Cons:** No audit trail, cannot rollback, compliance issues

**Decision:** Database snapshots

**Rationale:**
- **Simplicity:** Each script update creates new row in `script_versions` table with full source code
- **Audit Trail:** Immutable history with timestamps, user_id, change description
- **Rollback:** One-click restore to previous version via UI
- **Performance:** Minimal overhead (single INSERT per update), JSONB compression for storage efficiency
- **Compliance:** Satisfies audit requirements for regulatory industries (finance, healthcare)
- **Trade-offs Accepted:** No branching/merging, but not required for scripting use case

**Schema:**
```sql
CREATE TABLE script_versions (
    id BIGSERIAL PRIMARY KEY,
    script_id BIGINT NOT NULL REFERENCES scripts(id),
    version_number INT NOT NULL,
    source_code TEXT NOT NULL,
    change_description TEXT,
    created_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE (script_id, version_number)
);
```

### 2024-12-21: Code Editor Selection

**Context:** Script management UI needs browser-based code editor with syntax highlighting, autocomplete, and error detection. Must be embeddable in IOTA SDK templates.

**Options:**

1. **Textarea (Plain HTML):**
   - **Pros:** Zero dependencies, works everywhere
   - **Cons:** No syntax highlighting, no autocomplete, poor developer experience

2. **CodeMirror:**
   - **Pros:** Lightweight (50KB), extensible, good performance
   - **Cons:** Limited IntelliSense, requires custom autocomplete implementation

3. **Monaco Editor (VS Code):**
   - **Pros:** Industry-standard, built-in IntelliSense, diff view, debugger integration, extensible
   - **Cons:** Large bundle size (2MB), requires build step

4. **Ace Editor:**
   - **Pros:** Mature, good performance
   - **Cons:** Limited TypeScript support, smaller community

**Decision:** Monaco Editor

**Rationale:**
- **Best-in-Class UX:** Same editor used in VS Code, GitHub Codespaces
- **IntelliSense:** Autocomplete for SDK APIs (`iota.db.query()`, `iota.http.post()`)
- **Diff View:** Side-by-side comparison for version history
- **Error Detection:** Real-time syntax errors and warnings
- **Extensibility:** Can add custom themes, keybindings, snippets
- **Trade-offs Accepted:** 2MB bundle size acceptable for admin UI (not customer-facing), CDN caching mitigates load time

**Integration:**
- Load Monaco from CDN (`https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0`)
- Configure JavaScript language mode with SDK API types
- Add custom snippets for common patterns (cron script, HTTP endpoint, event handler)
- Implement autosave (debounced 2s after last keystroke)

### 2024-12-21: API Binding Style

**Context:** Scripts need to query database, send HTTP requests, and interact with IOTA SDK services. API design must balance flexibility, security, and maintainability.

**Options:**

1. **REST-like SDK Wrappers:**
   - **Pros:** Abstraction prevents breaking changes, enforces tenant isolation, familiar API
   - **Cons:** Limited flexibility, cannot execute arbitrary SQL

2. **Direct Repository Access:**
   - **Pros:** Maximum flexibility, full SQL power
   - **Cons:** High risk of SQL injection, no abstraction from schema changes, breaks tenant isolation

3. **GraphQL:**
   - **Pros:** Flexible queries, strong typing, schema introspection
   - **Cons:** Complex implementation, GraphQL server overhead, learning curve

**Decision:** REST-like SDK Wrappers

**Rationale:**
- **Security:** Parameterized queries prevent SQL injection, tenant_id automatically injected
- **Stability:** SDK wrappers abstract database schema, scripts don't break on migrations
- **Familiarity:** JavaScript developers understand REST-like APIs (`iota.db.query()`, `iota.http.post()`)
- **Performance:** Optimized queries with caching, connection pooling

**API Examples:**
```javascript
// Database queries
const users = await iota.db.query('users', { limit: 10, where: { active: true } });
const user = await iota.db.findById('users', 123);
await iota.db.insert('clients', { name: 'Acme Corp', email: 'info@acme.com' });

// HTTP requests
const response = await iota.http.post('https://api.stripe.com/v1/charges', {
    headers: { 'Authorization': 'Bearer sk_test_...' },
    body: { amount: 1000, currency: 'usd' }
});

// Logging
console.log('Payment processed:', response.data);
```

## Open Decisions

- **[TBD]** VM pool sizing: How many VMs per tenant? (e.g., max 10 concurrent, max 100 total)
- **[TBD]** Event retry backoff: What base and max values? (current: 2s base, 32s max, 5 attempts)
- **[TBD]** HTTP client timeout: Default timeout for `iota.http.*` calls? (current: 30s)
- **[TBD]** Execution log retention: How long to keep execution history? (current: 30 days, configurable)
- **[TBD]** NPM package support: Which packages to whitelist? (e.g., lodash, moment.js, axios)
