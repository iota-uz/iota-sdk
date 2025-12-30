---
layout: default
title: Applet System
parent: Specifications
nav_order: 1
has_children: true
description: "Comprehensive plugin system for extending IOTA SDK with JavaScript-based modules"
---

# Applet System Specification

**Status:** Draft - Complete Documentation
**Created:** 2024-12-31
**Last Updated:** 2024-12-31

## Overview

The Applet System enables IOTA SDK users to extend platform functionality through installable, runtime-loaded modules without requiring Go code or SDK recompilation. Applets are self-contained packages that can add UI pages, API endpoints, database tables, scheduled tasks, and event handlers.

```mermaid
graph TB
    subgraph "IOTA SDK"
        SDK[SDK Core]
        DB[(PostgreSQL)]
        UI[Web UI]
    end

    subgraph "Applet Runtime"
        MGR[Applet Manager]
        BUN[Bun Process]
        PERM[Permission Enforcer]
    end

    subgraph "Applet Package"
        MAN[Manifest]
        BE[Backend Code]
        FE[Frontend Code]
        LOC[Locales]
    end

    SDK --> MGR
    MGR --> BUN
    MGR --> PERM
    BUN --> BE
    PERM --> DB
    FE --> UI

    style SDK fill:#3b82f6,stroke:#1e40af,color:#fff
    style BUN fill:#10b981,stroke:#047857,color:#fff
    style MAN fill:#f59e0b,stroke:#d97706,color:#fff
```

**Vision:** Achieve plugin-level extensibility similar to Telegram Mini Apps, Shopify Apps, or Notion integrations, while maintaining UI consistency and security.

## Document Map

```mermaid
flowchart LR
    subgraph "Requirements"
        BUS[Business]
        OQ[Open Questions]
    end

    subgraph "Architecture"
        ARCH[Architecture]
        RT[Runtime Options]
        FE[Frontend]
        UIC[UI Consistency]
    end

    subgraph "Implementation"
        MAN[Manifest]
        PERM[Permissions]
        DBA[Database]
        DIST[Distribution]
    end

    subgraph "Reference"
        EX[Examples]
    end

    BUS --> ARCH
    ARCH --> RT & FE & UIC
    RT --> MAN
    FE --> MAN
    UIC --> MAN
    MAN --> PERM & DBA
    PERM --> DIST
    DBA --> DIST
    DIST --> EX
    OQ -.-> ARCH & MAN & DIST
```

## Documents

| Document | Purpose | Status |
|----------|---------|--------|
| [Business Requirements](./business.md) | Use cases, target audience, success criteria | Complete |
| [Architecture](./architecture.md) | System architecture and component design | Complete |
| [Runtime Options](./runtime-options.md) | JavaScript runtime comparison (Goja, Bun, V8, Deno) | Complete |
| [Frontend](./frontend.md) | Frontend framework options (React, Next.js, Vue) | Complete |
| [UI Consistency](./ui-consistency.md) | UI component library and design system strategy | Complete |
| [Manifest](./manifest.md) | Applet manifest schema specification | Complete |
| [Permissions](./permissions.md) | Permission model and security sandboxing | Complete |
| [Database](./database.md) | Database access and custom table strategy | Complete |
| [Distribution](./distribution.md) | Packaging, registry, and installation flow | Complete |
| [Examples](./examples.md) | Reference applet implementations | Complete |
| [Open Questions](./open-questions.md) | Unresolved decisions and trade-offs | Complete |

## High-Level Architecture

```mermaid
flowchart TB
    subgraph Browser["Browser"]
        USER[User]
        REACT[React UI]
    end

    subgraph GoSDK["Go SDK (Main Process)"]
        ROUTER[HTTP Router]
        CTRL[Controllers]
        AUTH[Auth/RBAC]
        APPMGR[Applet Manager]
        PROXY[DB Proxy]
    end

    subgraph BunRuntime["Bun Runtime (Sidecar)"]
        SOCK[Unix Socket]
        HANDLER[Request Handler]
        SSR[React SSR]
        SERVICES[Applet Services]
    end

    subgraph Storage["Storage"]
        PG[(PostgreSQL)]
        FILES[Package Files]
    end

    USER --> ROUTER
    ROUTER --> AUTH
    AUTH --> CTRL
    CTRL --> APPMGR
    APPMGR <--> SOCK
    SOCK <--> HANDLER
    HANDLER --> SSR
    HANDLER --> SERVICES
    SERVICES --> PROXY
    PROXY --> PG
    APPMGR --> FILES
    SSR --> REACT

    style GoSDK fill:#3b82f6,stroke:#1e40af,color:#fff
    style BunRuntime fill:#10b981,stroke:#047857,color:#fff
    style PG fill:#6366f1,stroke:#4338ca,color:#fff
```

## Key Design Decisions

### Recommended Stack

```mermaid
graph LR
    subgraph Runtime
        BUN[Bun v1.0+]
    end

    subgraph Frontend
        REACT[React 18]
        TS[TypeScript]
        COMP[@iota/components]
    end

    subgraph Backend
        HANDLERS[HTTP Handlers]
        EVENTS[Event Handlers]
        SCHED[Scheduled Tasks]
    end

    subgraph Integration
        IPC[Unix Socket IPC]
        PERM[Permission Enforcement]
        TENANT[Tenant Isolation]
    end

    BUN --> Frontend
    BUN --> Backend
    Backend --> Integration

    style BUN fill:#10b981,stroke:#047857,color:#fff
    style REACT fill:#61dafb,stroke:#0ea5e9,color:#000
    style TS fill:#3178c6,stroke:#1e40af,color:#fff
```

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Runtime** | Bun | Native TypeScript, fast startup, built-in bundler |
| **Frontend** | React + TypeScript | Developer familiarity, SSR support, ecosystem |
| **Communication** | Unix Socket IPC | Low latency, no network overhead |
| **UI Strategy** | Component Library | Guaranteed visual consistency |
| **Permissions** | Table-level + RBAC | Balance of security and flexibility |

## Reference Implementations

These existing IOTA SDK modules serve as reference for applet capabilities:

| Module | Location | Key Features |
|--------|----------|--------------|
| **Website/Ali** | `modules/website/` | AI chatbot widget, OpenAI integration, CRM message routing |
| **Shyona** | `shy-trucks/core/modules/shyona/` | Full AI assistant, multi-agent system, GraphQL, knowledge base |

## Quick Start (Conceptual)

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant CLI as iota-applet CLI
    participant Reg as Registry
    participant SDK as IOTA SDK

    Dev->>CLI: create my-applet
    CLI->>Dev: Scaffold project
    Dev->>Dev: Develop applet
    Dev->>CLI: build --prod
    CLI->>Dev: Package (my-applet-1.0.0.zip)
    Dev->>CLI: publish
    CLI->>Reg: Upload package
    Note over Reg: Review & validation
    Reg-->>CLI: Published
    SDK->>Reg: Browse applets
    SDK->>Reg: Download & install
    SDK->>SDK: Run applet
```

## Glossary

| Term | Definition |
|------|------------|
| **Applet** | A self-contained extension package that adds functionality to IOTA SDK |
| **Manifest** | YAML/JSON configuration file declaring applet metadata, permissions, and structure |
| **Handler** | JavaScript function that responds to HTTP requests, events, or schedules |
| **Widget** | UI component that can be injected into existing SDK pages |
| **Design Token** | CSS variable for consistent styling (colors, spacing, typography) |

---

## Next Steps

1. Review [Business Requirements](./business.md) for use cases
2. Understand [Architecture Options](./architecture.md)
3. Explore [Runtime Comparison](./runtime-options.md)
4. Check [Open Questions](./open-questions.md) for pending decisions

---

For questions or feedback, please [open an issue](https://github.com/iota-uz/iota-sdk/issues) on GitHub.
