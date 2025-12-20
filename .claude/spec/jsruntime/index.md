# JavaScript Runtime

**Status:** Draft
**Created:** 2024-12-21

## Overview

The JavaScript Runtime feature enables IOTA SDK users to extend platform functionality through user-defined JavaScript code execution in a secure, multi-tenant sandboxed environment. This feature supports scheduled scripts (cron), HTTP endpoint scripts, event-triggered scripts, one-off scripts, and embedded scripts, providing a flexible plugin architecture for custom business logic.

## Documents

| Document | Purpose | Status |
|----------|---------|--------|
| [00-overview.md](./00-overview.md) | Executive summary, glossary, feature capabilities | Complete |
| [business.md](./business.md) | Business requirements and use cases | Draft |
| [decisions.md](./decisions.md) | Technology and architecture decisions | Draft |
| [01-architecture.md](./01-architecture.md) | System design and component diagrams | Complete |
| [02-domain-model.md](./02-domain-model.md) | DDD entities, aggregates, value objects | Complete |
| [03-database-schema.md](./03-database-schema.md) | PostgreSQL schema, indexes, constraints | Complete |
| [04-repository-layer.md](./04-repository-layer.md) | Repository interfaces and patterns | Complete |
| [05-service-layer.md](./05-service-layer.md) | Business logic and orchestration | Complete |
| [06-runtime-engine.md](./06-runtime-engine.md) | VM pooling and execution engine | Complete |
| [07-api-bindings.md](./07-api-bindings.md) | JavaScript SDK APIs | Complete |
| [08-event-integration.md](./08-event-integration.md) | Event-driven architecture | Complete |
| [09-presentation-layer.md](./09-presentation-layer.md) | UI, controllers, templates | Complete |
| [10-security-model.md](./10-security-model.md) | Sandboxing, isolation, limits | Complete |
| [11-advanced-features.md](./11-advanced-features.md) | Monitoring, optimization, health checks | Complete |
| [12-implementation-plan.md](./12-implementation-plan.md) | Phased rollout strategy | Complete |

## Quick Links

- **Problem Statement:** [business.md#problem-statement](./business.md#problem-statement)
- **Use Cases:** [business.md#use-cases](./business.md#use-cases)
- **Architecture:** [01-architecture.md](./01-architecture.md)
- **Technology Decisions:** [decisions.md#decisions](./decisions.md#decisions)
- **Domain Model:** [02-domain-model.md](./02-domain-model.md)
- **Database Schema:** [03-database-schema.md](./03-database-schema.md)
- **Security Model:** [10-security-model.md](./10-security-model.md)
- **Implementation Plan:** [12-implementation-plan.md](./12-implementation-plan.md)

## Related GitHub Issues

**Core Infrastructure:**
- [#411](https://github.com/iota-uz/iota-sdk/issues/411) - JavaScript Runtime Core
- [#412](https://github.com/iota-uz/iota-sdk/issues/412) - VM Pooling & Resource Management
- [#413](https://github.com/iota-uz/iota-sdk/issues/413) - Script Versioning & Audit Trail
- [#148](https://github.com/iota-uz/iota-sdk/issues/148) - Monaco Editor Integration

**Trigger Mechanisms:**
- [#414](https://github.com/iota-uz/iota-sdk/issues/414) - Scheduled Scripts (Cron)
- [#415](https://github.com/iota-uz/iota-sdk/issues/415) - HTTP Endpoint Scripts
- [#416](https://github.com/iota-uz/iota-sdk/issues/416) - Event-Triggered Scripts
- [#417](https://github.com/iota-uz/iota-sdk/issues/417) - One-Off Script Execution

**API & Bindings:**
- [#418](https://github.com/iota-uz/iota-sdk/issues/418) - Standard Library API
- [#419](https://github.com/iota-uz/iota-sdk/issues/419) - Database Access API
- [#420](https://github.com/iota-uz/iota-sdk/issues/420) - HTTP Client API

## Open Questions

- **From business.md:** Pricing model for script execution quotas (free tier, paid tiers)
- **From decisions.md:** VM pool sizing strategy (per-tenant vs global pool)
- **From decisions.md:** Event retry backoff parameters (exponential base, max attempts)
- **From technical:** HTTP endpoint path conflict resolution strategy
- **From security:** Rate limiting per tenant (requests/minute, concurrent executions)
