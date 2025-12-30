---
layout: default
title: Specifications
nav_order: 15
has_children: true
description: "Technical specifications and design documents for IOTA SDK features"
---

# Technical Specifications

This section contains detailed technical specifications and design documents for planned and in-development IOTA SDK features. These specifications serve as reference documentation for architecture decisions, implementation details, and future roadmap items.

## Overview

```mermaid
mindmap
  root((IOTA SDK Specs))
    Applet System
      Architecture
      Runtime Options
      Frontend Framework
      UI Consistency
      Permissions
      Database Access
      Distribution
    JS Runtime
      Script Execution
      API Schema
      Data Model
      Security
```

## Available Specifications

### [Applet System](./applets/)
**Status:** Draft | **Priority:** High

A comprehensive plugin/extension system that enables JavaScript-based modules to extend IOTA SDK functionality without Go code or SDK recompilation.

```mermaid
graph LR
    A[Applet Package] --> B[Manifest]
    A --> C[Backend Code]
    A --> D[Frontend Code]
    B --> E[Permissions]
    B --> F[Configuration]
    C --> G[Bun Runtime]
    D --> H[React Components]

    style A fill:#3b82f6,stroke:#1e40af,color:#fff
    style G fill:#10b981,stroke:#047857,color:#fff
    style H fill:#8b5cf6,stroke:#5b21b6,color:#fff
```

**Key Features:**
- Runtime-installable extensions
- React + TypeScript frontends
- Bun as JavaScript runtime
- Tenant-isolated data access
- Permission-based security model

[Explore Applet System Specification →](./applets/)

---

### [JavaScript Runtime](./jsruntime/)
**Status:** Draft | **Priority:** Medium

Embedded JavaScript execution engine for user-defined scripts, scheduled jobs, and dynamic endpoints using Goja.

```mermaid
graph TB
    subgraph "Script Types"
        A[Scheduled Jobs]
        B[HTTP Endpoints]
        C[Event Handlers]
        D[One-off Scripts]
    end

    subgraph "Execution Engine"
        E[Goja VM Pool]
        F[Sandboxed Context]
        G[SDK API Bindings]
    end

    A & B & C & D --> E
    E --> F
    F --> G

    style E fill:#f59e0b,stroke:#d97706,color:#fff
    style F fill:#ef4444,stroke:#b91c1c,color:#fff
```

**Key Features:**
- Embedded Goja JavaScript engine
- Sandboxed execution environment
- Database and HTTP API access
- Scheduled task execution
- Event-driven automation

[Explore JS Runtime Specification →](./jsruntime/)

---

## Specification Status Legend

| Status | Description |
|--------|-------------|
| **Draft** | Initial ideation, gathering requirements |
| **In Review** | Under technical review |
| **Approved** | Ready for implementation |
| **In Progress** | Implementation underway |
| **Complete** | Implemented and documented |

## Comparison: Applet System vs JS Runtime

```mermaid
graph TB
    subgraph "Applet System"
        A1[Full Modules]
        A2[React UI]
        A3[Bun Runtime]
        A4[npm Packages]
        A5[Complex Features]
    end

    subgraph "JS Runtime"
        B1[Simple Scripts]
        B2[No UI]
        B3[Goja Embedded]
        B4[No npm]
        B5[Automation Tasks]
    end

    A1 -.->|"Use for"| C1[AI Chatbots]
    A1 -.->|"Use for"| C2[Analytics Dashboards]
    A1 -.->|"Use for"| C3[Integrations]

    B1 -.->|"Use for"| D1[Webhooks]
    B1 -.->|"Use for"| D2[Cron Jobs]
    B1 -.->|"Use for"| D3[Data Transforms]

    style A1 fill:#3b82f6,stroke:#1e40af,color:#fff
    style B1 fill:#f59e0b,stroke:#d97706,color:#fff
```

| Aspect | Applet System | JS Runtime |
|--------|---------------|------------|
| **Complexity** | Full modules with UI | Simple scripts |
| **Runtime** | Bun (external process) | Goja (embedded) |
| **TypeScript** | Native support | Requires transpilation |
| **npm Packages** | Full access | Not available |
| **UI Components** | React pages/widgets | None |
| **Use Case** | Feature extensions | Automation scripts |

## Contributing to Specifications

Specifications follow a structured process:

1. **Propose** - Create a draft in `.claude/spec/` or `docs/specs/`
2. **Discuss** - Gather feedback and refine requirements
3. **Review** - Technical review by maintainers
4. **Approve** - Finalize for implementation
5. **Implement** - Build the feature
6. **Document** - Update user-facing documentation

---

For questions or feedback on specifications, please [open an issue](https://github.com/iota-uz/iota-sdk/issues) on GitHub.
