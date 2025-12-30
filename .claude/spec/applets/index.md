# Applet System Specification

**Status:** Draft - Complete Documentation
**Created:** 2024-12-31
**Last Updated:** 2024-12-31

## Overview

The Applet System enables IOTA SDK users to extend platform functionality through installable, runtime-loaded modules without requiring Go code or SDK recompilation. Applets are self-contained packages that can add UI pages, API endpoints, database tables, scheduled tasks, and event handlers.

**Vision:** Achieve plugin-level extensibility similar to Telegram Mini Apps, Shopify Apps, or Notion integrations, while maintaining UI consistency and security.

## Documents

| Document | Purpose | Status |
|----------|---------|--------|
| [business.md](./business.md) | Business requirements, use cases, target audience | Complete |
| [architecture.md](./architecture.md) | System architecture and component design | Complete |
| [runtime-options.md](./runtime-options.md) | JavaScript runtime comparison (Goja, Bun, V8, Deno) | Complete |
| [frontend.md](./frontend.md) | Frontend framework options (React, Next.js, Vue) | Complete |
| [ui-consistency.md](./ui-consistency.md) | UI component library and design system strategy | Complete |
| [manifest.md](./manifest.md) | Applet manifest schema specification | Complete |
| [permissions.md](./permissions.md) | Permission model and security sandboxing | Complete |
| [database.md](./database.md) | Database access and custom table strategy | Complete |
| [distribution.md](./distribution.md) | Packaging, registry, and installation flow | Complete |
| [examples.md](./examples.md) | Reference applet examples based on existing modules | Complete |
| [open-questions.md](./open-questions.md) | Unresolved decisions and trade-offs | Complete |

## Reference Implementations

These existing IOTA SDK modules serve as reference for applet capabilities:

| Module | Location | Key Features |
|--------|----------|--------------|
| **Website/Ali** | `modules/website/` | AI chatbot widget, OpenAI integration, CRM message routing |
| **Shyona** | `shy-trucks/core/modules/shyona/` | Full AI assistant, multi-agent system, GraphQL, knowledge base |

## Quick Links

- **Problem Statement:** [business.md#problem-statement](./business.md#problem-statement)
- **Architecture Options:** [architecture.md#options](./architecture.md#options)
- **Runtime Comparison:** [runtime-options.md#comparison](./runtime-options.md#comparison)
- **Open Questions:** [open-questions.md](./open-questions.md)

## Key Decisions Pending

1. **Runtime Choice:** Goja (embedded) vs Bun (standalone) vs V8 (via cgo)
2. **Frontend Framework:** React/Next.js (compiled) vs HTML+HTMX (templates) vs Web Components
3. **UI Strategy:** Component library vs design tokens vs declarative UI schema
4. **Database Model:** Isolated schemas vs prefixed tables vs SDK API only
5. **Distribution:** File-based vs registry/marketplace

## Glossary

| Term | Definition |
|------|------------|
| **Applet** | A self-contained extension package that adds functionality to IOTA SDK |
| **Manifest** | YAML/JSON configuration file declaring applet metadata, permissions, and structure |
| **Handler** | JavaScript function that responds to HTTP requests, events, or schedules |
| **Widget** | UI component that can be injected into existing SDK pages |
| **Design Token** | CSS variable for consistent styling (colors, spacing, typography) |
