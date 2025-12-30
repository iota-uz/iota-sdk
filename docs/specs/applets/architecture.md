---
layout: default
title: Architecture
parent: Applet System
grand_parent: Specifications
nav_order: 2
description: "System architecture and component design for the Applet System"
---

# Architecture Specification: Applet System

**Status:** Draft

## Architecture Overview

The Applet System consists of three major components:

1. **Applet Host** - Go-based runtime that loads and executes applets
2. **Applet Package** - Bundled TypeScript/React code + manifest
3. **SDK Integration Points** - Routes, events, database, UI slots

```mermaid
flowchart TB
    subgraph SDKApp["IOTA SDK Application"]
        subgraph AppletHost["Applet Host"]
            PM[Package Manager<br/>install / uninstall / update / validate]
            RE[Runtime Engine<br/>execute / sandbox / resource limits]
            IL[Integration Layer<br/>route injection / event subscription<br/>UI slot registration / database proxy]
        end

        subgraph SDKCore["SDK Core Services"]
            HTTP[HTTP Router]
            EVT[EventBus]
            DBP[Database Pool]
            UIR[UI Renderer]
            AUTH[Auth]
        end
    end

    subgraph AppletPkg["Applet Package"]
        MAN[manifest.yaml]
        DIST[dist/]
        BE[backend.js]
        FEP[frontend/pages/]
        FEW[frontend/widgets/]
        SRC[src/]
        MIG[migrations/]
        LOC[locales/]
    end

    AppletHost --> SDKCore
    SDKApp -->|Loads| AppletPkg

    style SDKApp fill:#3b82f6,stroke:#1e40af,color:#fff
    style AppletHost fill:#10b981,stroke:#047857,color:#fff
    style AppletPkg fill:#f59e0b,stroke:#d97706,color:#fff
```

## Architecture Options

### Option A: Embedded Runtime (Goja)

**Description:** JavaScript executed within Go process using Goja VM

```mermaid
flowchart TB
    subgraph GoProcess["IOTA SDK (Go Process)"]
        subgraph Host["Applet Host"]
            subgraph Pool["Goja VM Pool"]
                VM1[VM1]
                VM2[VM2]
                VM3[VM3]
            end
        end
    end

    style GoProcess fill:#3b82f6,stroke:#1e40af,color:#fff
    style Pool fill:#f59e0b,stroke:#d97706,color:#fff
```

| Aspect | Details |
|--------|---------|
| **Pros** | No external dependencies, direct Go interop, memory safety via Go GC, single binary deployment |
| **Cons** | Limited ES6+ support, no async/await natively, slower than V8/Bun, no native React SSR |
| **Best For** | Simple handlers, scripts, automation |

### Option B: Standalone Runtime (Bun)

**Description:** JavaScript executed in separate Bun process, communicating via IPC/HTTP

```mermaid
flowchart LR
    subgraph GoSDK["IOTA SDK (Go)"]
        AH[Applet Host<br/>proxy requests<br/>manage process<br/>auth context]
    end

    subgraph BunRT["Bun Runtime"]
        AE[Applet Executor<br/>TypeScript<br/>React SSR<br/>Full ES2024]
    end

    GoSDK <-->|IPC / HTTP| BunRT

    style GoSDK fill:#3b82f6,stroke:#1e40af,color:#fff
    style BunRT fill:#10b981,stroke:#047857,color:#fff
```

| Aspect | Details |
|--------|---------|
| **Pros** | Full TypeScript/ES2024, native React SSR, fast execution, rich npm ecosystem |
| **Cons** | External process management, IPC overhead, deployment complexity |
| **Best For** | Complex applets, React-based UIs, heavy computation |

### Option C: Sidecar Runtime (Deno/Node)

Similar to Bun but using Deno or Node.js as the runtime.

```mermaid
graph TB
    subgraph Deno["Deno Runtime"]
        D1[Security-first]
        D2[TypeScript native]
        D3[Web-standard APIs]
        D4[Good sandboxing]
    end

    subgraph Node["Node.js Runtime"]
        N1[Mature ecosystem]
        N2[Universal compatibility]
        N3[Well-understood]
        N4[Production proven]
    end

    style Deno fill:#10b981,stroke:#047857,color:#fff
    style Node fill:#22c55e,stroke:#15803d,color:#fff
```

### Option D: Hybrid Architecture (Recommended)

**Description:** Use embedded runtime for simple handlers, sidecar for complex applets

```mermaid
flowchart TB
    subgraph GoSDK["IOTA SDK (Go)"]
        subgraph Host["Applet Host"]
            EE[Embedded Engine<br/>Goja]
            SM[Sidecar Manager]
        end
    end

    subgraph Simple["Simple Handlers"]
        A1[Applet A<br/>webhook handler]
        A2[Applet C<br/>cron job]
    end

    subgraph BunRT["Bun Runtime"]
        AB[Applet B<br/>AI Chat<br/>React SSR<br/>TypeScript]
    end

    EE -->|Simple handlers| Simple
    SM -->|Complex applets| BunRT

    style GoSDK fill:#3b82f6,stroke:#1e40af,color:#fff
    style Simple fill:#f59e0b,stroke:#d97706,color:#fff
    style BunRT fill:#10b981,stroke:#047857,color:#fff
```

| Aspect | Details |
|--------|---------|
| **Pros** | Best of both worlds, efficient for simple cases, powerful for complex cases |
| **Cons** | Two systems to maintain, decision complexity for developers |
| **Best For** | Production systems with varied applet complexity |

## Recommended Architecture

```mermaid
timeline
    title Implementation Timeline
    section Short-term (MVP)
        Option B (Bun Sidecar) : Full TypeScript/React support from day one
    section Long-term
        Option D (Hybrid) : Goja for simple scripts + Bun for complex applets
```

## Component Design

### Package Manager

Responsible for applet lifecycle:

```mermaid
classDiagram
    class PackageManager {
        +Install(ctx, source) Applet
        +Uninstall(ctx, appletID) error
        +Update(ctx, appletID, source) error
        +List(ctx, tenantID) []Applet
        +Get(ctx, appletID) Applet
        +Enable(ctx, appletID, tenantID) error
        +Disable(ctx, appletID, tenantID) error
        +Validate(ctx, source) ValidationResult
    }

    class PackageSource {
        <<interface>>
        +Type() string
        +Location() string
    }

    PackageManager --> PackageSource
```

### Runtime Engine

Executes applet code:

```mermaid
classDiagram
    class RuntimeEngine {
        <<interface>>
        +Execute(ctx, applet, request) ExecutionResult
        +Start(ctx, applet) error
        +Stop(ctx, applet) error
        +Health(ctx, applet) HealthStatus
    }

    class ExecutionRequest {
        +Type ExecutionType
        +Handler string
        +Input interface
        +Context AppletContext
    }

    class ExecutionType {
        <<enumeration>>
        HTTP
        Event
        Scheduled
    }

    RuntimeEngine --> ExecutionRequest
    ExecutionRequest --> ExecutionType
```

### Integration Layer

Bridges applets with SDK:

```mermaid
classDiagram
    class IntegrationLayer {
        <<interface>>
        +RegisterRoutes(applet, routes) error
        +UnregisterRoutes(appletID) error
        +SubscribeEvents(applet, events) error
        +UnsubscribeEvents(appletID) error
        +RegisterNavItems(applet, items) error
        +RegisterPages(applet, pages) error
        +RegisterWidgets(applet, widgets) error
        +CreateDatabaseProxy(applet) DatabaseProxy
    }
```

## Communication Protocol (for Sidecar)

### Protocol Comparison

```mermaid
graph LR
    subgraph HTTP["HTTP-based (Simple)"]
        H1[Simple]
        H2[Debuggable]
        H3[Standard tooling]
        H4[Higher latency]
    end

    subgraph Unix["Unix Socket (Recommended)"]
        U1[Lower latency]
        U2[No network stack]
        U3[Platform-specific]
    end

    subgraph gRPC["gRPC (Future)"]
        G1[Efficient binary]
        G2[Streaming support]
        G3[Complex setup]
    end

    style Unix fill:#10b981,stroke:#047857,color:#fff
```

### Message Flow

```mermaid
sequenceDiagram
    participant SDK as Go SDK
    participant Socket as Unix Socket
    participant Bun as Bun Runtime
    participant Handler as Applet Handler

    SDK->>Socket: AppletRequest (JSON)
    Socket->>Bun: Forward request
    Bun->>Handler: Execute handler
    Handler-->>Bun: Result
    Bun-->>Socket: AppletResponse (JSON)
    Socket-->>SDK: Return response
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
    permissions: string[];
    locale: string;
  };
  payload: {
    method?: string;
    path?: string;
    headers?: Record<string, string>;
    body?: unknown;
  };
}

interface AppletResponse {
  id: string;
  status: 'success' | 'error';
  data?: { statusCode?: number; body?: unknown; };
  error?: { code: string; message: string; };
}
```

## Security Boundaries

```mermaid
flowchart TB
    subgraph Trust["Trust Boundary"]
        subgraph Core["IOTA SDK Core"]
            A1[User authentication]
            A2[Tenant isolation]
            A3[Permission enforcement]
            A4[Database connection pool]
        end

        CI[Controlled Interface]

        subgraph Sandbox["Applet Sandbox"]
            B1[Pre-validated tenant context]
            B2[Scoped database access]
            B3[Approved external HTTP only]
            B4[Resource limits]
            B5[No file system access]
            B6[No process spawning]
        end
    end

    Core --> CI
    CI --> Sandbox

    style Core fill:#3b82f6,stroke:#1e40af,color:#fff
    style Sandbox fill:#ef4444,stroke:#b91c1c,color:#fff
```

## Error Handling

```mermaid
classDiagram
    class AppletError {
        <<abstract>>
    }

    class InstallationError {
        ManifestValidationError
        PermissionDeniedError
        MigrationError
    }

    class RuntimeError {
        ExecutionTimeoutError
        MemoryLimitError
        HandlerNotFoundError
        ExternalAPIError
    }

    class SecurityError {
        UnauthorizedAccessError
        TenantIsolationError
        SSRFAttemptError
    }

    class IntegrationError {
        RouteConflictError
        EventSubscriptionError
        UISlotConflictError
    }

    AppletError <|-- InstallationError
    AppletError <|-- RuntimeError
    AppletError <|-- SecurityError
    AppletError <|-- IntegrationError
```

## Monitoring & Observability

```mermaid
graph TB
    subgraph Metrics["Applet Metrics"]
        subgraph Execution["Execution Metrics"]
            E1[applet_request_count]
            E2[applet_request_duration_ms]
            E3[applet_error_count]
        end

        subgraph Resources["Resource Metrics"]
            R1[applet_memory_usage_bytes]
            R2[applet_cpu_time_ms]
            R3[applet_external_api_calls]
        end

        subgraph Health["Health Metrics"]
            H1[applet_status]
            H2[applet_last_execution_time]
            H3[applet_consecutive_errors]
        end
    end

    style Execution fill:#3b82f6,stroke:#1e40af,color:#fff
    style Resources fill:#f59e0b,stroke:#d97706,color:#fff
    style Health fill:#10b981,stroke:#047857,color:#fff
```

---

## Next Steps

- Review [Runtime Options](./runtime-options.md) for detailed runtime comparison
- See [Permissions](./permissions.md) for security model details
- Check [Distribution](./distribution.md) for packaging and deployment
