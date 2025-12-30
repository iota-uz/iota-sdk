# Architecture Specification: Applet System

**Status:** Draft

## Architecture Overview

The Applet System consists of three major components:

1. **Applet Host** - Go-based runtime that loads and executes applets
2. **Applet Package** - Bundled TypeScript/React code + manifest
3. **SDK Integration Points** - Routes, events, database, UI slots

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           IOTA SDK Application                           │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                         Applet Host                                 │ │
│  │  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────┐   │ │
│  │  │ Package      │  │ Runtime      │  │ Integration            │   │ │
│  │  │ Manager      │  │ Engine       │  │ Layer                  │   │ │
│  │  │ - install    │  │ - execute    │  │ - route injection      │   │ │
│  │  │ - uninstall  │  │ - sandbox    │  │ - event subscription   │   │ │
│  │  │ - update     │  │ - resource   │  │ - UI slot registration │   │ │
│  │  │ - validate   │  │   limits     │  │ - database proxy       │   │ │
│  │  └──────────────┘  └──────────────┘  └────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    │                                     │
│                                    ▼                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                     SDK Core Services                               │ │
│  │  HTTP Router │ EventBus │ Database Pool │ UI Renderer │ Auth      │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │ Loads
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          Applet Package                                  │
│                                                                          │
│  manifest.yaml              # Metadata, permissions, entry points        │
│  ├── dist/                  # Compiled/bundled output                    │
│  │   ├── backend.js         # Server-side bundle (handlers, services)   │
│  │   └── frontend/          # Client-side bundles                        │
│  │       ├── pages/         # Page components (React SSR or CSR)        │
│  │       └── widgets/       # Widget components                          │
│  ├── src/                   # Source code (TypeScript/React)            │
│  │   ├── backend/                                                        │
│  │   └── frontend/                                                       │
│  ├── migrations/            # SQL migrations (optional)                  │
│  └── locales/               # Translation files                          │
└─────────────────────────────────────────────────────────────────────────┘
```

## Architecture Options

### Option A: Embedded Runtime (Goja)

**Description:** JavaScript executed within Go process using Goja VM

```
┌─────────────────────────────────────────┐
│           IOTA SDK (Go Process)          │
│  ┌─────────────────────────────────┐    │
│  │         Applet Host              │    │
│  │  ┌─────────────────────────┐    │    │
│  │  │   Goja VM Pool          │    │    │
│  │  │  ┌─────┐ ┌─────┐       │    │    │
│  │  │  │ VM1 │ │ VM2 │ ...   │    │    │
│  │  │  └─────┘ └─────┘       │    │    │
│  │  └─────────────────────────┘    │    │
│  └─────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

**Pros:**
- No external dependencies
- Direct Go interop (fast function calls)
- Memory safety via Go's GC
- Simpler deployment (single binary)

**Cons:**
- Limited ES6+ support (no async/await natively)
- Slower than V8/Bun
- Complex TypeScript → ES5 transpilation
- No native React SSR

**Best For:** Simple handlers, scripts, automation

### Option B: Standalone Runtime (Bun)

**Description:** JavaScript executed in separate Bun process, communicating via IPC/HTTP

```
┌─────────────────────────┐     IPC/HTTP     ┌─────────────────────────┐
│   IOTA SDK (Go)         │◄───────────────►│   Bun Runtime            │
│  ┌───────────────────┐  │                  │  ┌───────────────────┐  │
│  │ Applet Host       │  │                  │  │ Applet Executor   │  │
│  │ - proxy requests  │  │                  │  │ - TypeScript      │  │
│  │ - manage process  │  │                  │  │ - React SSR       │  │
│  │ - auth context    │  │                  │  │ - Full ES2024     │  │
│  └───────────────────┘  │                  │  └───────────────────┘  │
└─────────────────────────┘                  └─────────────────────────┘
```

**Pros:**
- Full TypeScript/ES2024 support
- Native React/Next.js SSR
- Fast execution (V8-based)
- Rich ecosystem (npm packages)
- Built-in bundler, test runner

**Cons:**
- External process management
- IPC overhead for each request
- Additional deployment complexity
- Resource isolation challenges

**Best For:** Complex applets, React-based UIs, heavy computation

### Option C: Sidecar Runtime (Deno/Node)

**Description:** Similar to Bun but using Deno or Node.js

```
Same as Option B, but with Deno or Node.js as the runtime
```

**Deno Pros:**
- Security-first (explicit permissions)
- TypeScript native
- Web-standard APIs
- Good for sandboxing

**Deno Cons:**
- npm compatibility issues
- Smaller ecosystem than Node

**Node Pros:**
- Mature ecosystem
- Universal compatibility
- Well-understood

**Node Cons:**
- Slower startup than Bun
- No built-in TypeScript
- Legacy baggage

### Option D: Hybrid Architecture

**Description:** Use embedded runtime for simple handlers, sidecar for complex applets

```
┌─────────────────────────────────────────────────────────────────────┐
│                        IOTA SDK (Go)                                 │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                       Applet Host                               │ │
│  │                                                                  │ │
│  │  ┌──────────────────┐           ┌──────────────────────────┐   │ │
│  │  │ Embedded Engine  │           │ Sidecar Manager          │   │ │
│  │  │ (Goja)           │           │                          │   │ │
│  │  │ - simple scripts │           │ - spawn Bun process      │   │ │
│  │  │ - event handlers │           │ - IPC communication      │   │ │
│  │  │ - cron jobs      │           │ - lifecycle management   │   │ │
│  │  └──────────────────┘           └────────────┬─────────────┘   │ │
│  │           │                                   │                 │ │
│  │           │ Simple handlers                   │ Complex applets │ │
│  │           ▼                                   ▼                 │ │
│  │  ┌──────────────────┐           ┌──────────────────────────┐   │ │
│  │  │ Applet A         │           │ Bun Runtime              │   │ │
│  │  │ (webhook handler)│           │ ┌────────────────────┐   │   │ │
│  │  └──────────────────┘           │ │ Applet B (AI Chat) │   │   │ │
│  │  ┌──────────────────┐           │ │ - React SSR        │   │   │ │
│  │  │ Applet C         │           │ │ - TypeScript       │   │   │ │
│  │  │ (cron job)       │           │ └────────────────────┘   │   │ │
│  │  └──────────────────┘           └──────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

**Pros:**
- Best of both worlds
- Efficient for simple cases
- Powerful for complex cases
- Gradual migration path

**Cons:**
- Two systems to maintain
- Decision complexity for developers
- Testing across both runtimes

**Best For:** Production systems with varied applet complexity

## Recommended Architecture

**Short-term (MVP):** Option B (Bun Sidecar)

- Full TypeScript/React support from day one
- Simpler developer experience
- Cleaner separation of concerns

**Long-term:** Option D (Hybrid)

- Embed Goja for simple scripts (already have jsruntime spec)
- Sidecar for complex React-based applets
- Clear guidance on when to use each

## Component Design

### Package Manager

Responsible for applet lifecycle:

```go
type PackageManager interface {
    // Installation
    Install(ctx context.Context, source PackageSource) (*Applet, error)
    Uninstall(ctx context.Context, appletID string) error
    Update(ctx context.Context, appletID string, source PackageSource) error

    // Discovery
    List(ctx context.Context, tenantID string) ([]*Applet, error)
    Get(ctx context.Context, appletID string) (*Applet, error)

    // Lifecycle
    Enable(ctx context.Context, appletID string, tenantID string) error
    Disable(ctx context.Context, appletID string, tenantID string) error

    // Validation
    Validate(ctx context.Context, source PackageSource) (*ValidationResult, error)
}

type PackageSource interface {
    // Could be file path, URL, or registry reference
    Type() string
    Location() string
}
```

### Runtime Engine

Executes applet code:

```go
type RuntimeEngine interface {
    // Execution
    Execute(ctx context.Context, applet *Applet, request *ExecutionRequest) (*ExecutionResult, error)

    // Lifecycle
    Start(ctx context.Context, applet *Applet) error
    Stop(ctx context.Context, applet *Applet) error

    // Health
    Health(ctx context.Context, applet *Applet) (*HealthStatus, error)
}

type ExecutionRequest struct {
    Type        ExecutionType  // HTTP, Event, Scheduled
    Handler     string         // Handler name/path
    Input       interface{}    // Request body, event payload, etc.
    Context     *AppletContext // Tenant, user, permissions
}
```

### Integration Layer

Bridges applets with SDK:

```go
type IntegrationLayer interface {
    // Routes
    RegisterRoutes(applet *Applet, routes []RouteDefinition) error
    UnregisterRoutes(appletID string) error

    // Events
    SubscribeEvents(applet *Applet, events []string) error
    UnsubscribeEvents(appletID string) error

    // UI
    RegisterNavItems(applet *Applet, items []NavItem) error
    RegisterPages(applet *Applet, pages []PageDefinition) error
    RegisterWidgets(applet *Applet, widgets []WidgetDefinition) error

    // Database
    CreateDatabaseProxy(applet *Applet) DatabaseProxy
}
```

## Communication Protocol (for Sidecar)

### HTTP-based (Simple)

```
Go SDK ──HTTP──► Bun Runtime
       ◄─HTTP──

Pros: Simple, debuggable, standard tooling
Cons: Higher latency, serialization overhead
```

### Unix Socket (Recommended)

```
Go SDK ──Unix Socket──► Bun Runtime
       ◄─Unix Socket──

Pros: Lower latency, no network stack
Cons: Platform-specific (not Windows native)
```

### gRPC (Future)

```
Go SDK ──gRPC──► Bun Runtime
       ◄─gRPC──

Pros: Efficient binary protocol, streaming
Cons: More complex setup, proto files
```

### Message Format

```typescript
interface AppletRequest {
  id: string;              // Request correlation ID
  type: 'http' | 'event' | 'scheduled' | 'render';
  handler: string;         // Handler path
  context: {
    tenantId: string;
    userId?: number;
    organizationId?: string;
    permissions: string[];
    locale: string;
  };
  payload: {
    // For HTTP
    method?: string;
    path?: string;
    headers?: Record<string, string>;
    query?: Record<string, string>;
    body?: unknown;

    // For events
    eventType?: string;
    eventData?: unknown;

    // For render
    component?: string;
    props?: unknown;
  };
}

interface AppletResponse {
  id: string;              // Matches request ID
  status: 'success' | 'error';
  data?: {
    // For HTTP
    statusCode?: number;
    headers?: Record<string, string>;
    body?: unknown;

    // For render
    html?: string;
  };
  error?: {
    code: string;
    message: string;
    details?: unknown;
  };
}
```

## Security Boundaries

```
┌─────────────────────────────────────────────────────────────────┐
│                          Trust Boundary                          │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    IOTA SDK Core                           │  │
│  │  - User authentication                                     │  │
│  │  - Tenant isolation                                        │  │
│  │  - Permission enforcement                                  │  │
│  │  - Database connection pool                                │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                    Controlled Interface                          │
│                              │                                   │
│  ┌───────────────────────────▼───────────────────────────────┐  │
│  │                    Applet Sandbox                          │  │
│  │  - Pre-validated tenant context                            │  │
│  │  - Scoped database access (tenant_id enforced)            │  │
│  │  - Approved external HTTP hosts only                      │  │
│  │  - Resource limits (CPU, memory, time)                    │  │
│  │  - No file system access                                  │  │
│  │  - No process spawning                                    │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## Error Handling

```
┌──────────────────────────────────────────────────────────────────┐
│                     Error Hierarchy                               │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  AppletError                                                      │
│  ├── InstallationError                                           │
│  │   ├── ManifestValidationError                                 │
│  │   ├── PermissionDeniedError                                   │
│  │   └── MigrationError                                          │
│  ├── RuntimeError                                                │
│  │   ├── ExecutionTimeoutError                                   │
│  │   ├── MemoryLimitError                                        │
│  │   ├── HandlerNotFoundError                                    │
│  │   └── ExternalAPIError                                        │
│  ├── SecurityError                                               │
│  │   ├── UnauthorizedAccessError                                 │
│  │   ├── TenantIsolationError                                    │
│  │   └── SSRFAttemptError                                        │
│  └── IntegrationError                                            │
│      ├── RouteConflictError                                      │
│      ├── EventSubscriptionError                                  │
│      └── UISlotConflictError                                     │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

## Monitoring & Observability

```
┌─────────────────────────────────────────────────────────────────┐
│                    Applet Metrics                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Execution Metrics:                                              │
│  - applet_request_count{applet, handler, status}                │
│  - applet_request_duration_ms{applet, handler}                  │
│  - applet_error_count{applet, error_type}                       │
│                                                                  │
│  Resource Metrics:                                               │
│  - applet_memory_usage_bytes{applet}                            │
│  - applet_cpu_time_ms{applet}                                   │
│  - applet_external_api_calls{applet, host}                      │
│                                                                  │
│  Health Metrics:                                                 │
│  - applet_status{applet} (enabled, disabled, error)             │
│  - applet_last_execution_time{applet}                           │
│  - applet_consecutive_errors{applet}                            │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```
