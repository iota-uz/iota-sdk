# JavaScript Runtime - Overview

## Executive Summary

The JavaScript Runtime feature enables IOTA SDK users to extend platform functionality through user-defined JavaScript code execution in a secure, multi-tenant sandboxed environment. This feature supports scheduled scripts (cron), HTTP endpoint scripts, event-triggered scripts, one-off scripts, and embedded scripts.

**Technology Stack:**
- **Runtime**: Goja (pure Go ECMAScript 5.1+ with select ES6 features)
- **Editor**: Monaco Editor (web-based code editor with IntelliSense)
- **Persistence**: PostgreSQL with multi-tenant isolation
- **Orchestration**: VM pooling, resource limits, timeout enforcement

## Glossary of Terms

**Script**: User-defined JavaScript code stored in the database with metadata, resource limits, and trigger configuration.

**Execution**: A single run of a script, tracked with input/output, status, metrics, and error details.

**VM Pool**: Pre-warmed Goja virtual machines ready to execute scripts, reducing cold-start latency.

**Sandbox**: Isolated execution environment with restricted access to system resources and tenant data.

**Trigger**: Event or condition that initiates script execution (cron schedule, HTTP request, domain event, manual trigger).

**Resource Limits**: Constraints on script execution (max duration, memory, API calls, concurrent executions).

**Tenant Isolation**: Multi-tenant security boundary ensuring scripts can only access data within their tenant context.

**Script Type**: Classification of script by trigger mechanism (Scheduled, HTTP, Event, OneOff, Embedded).

**Execution Status**: State of script run (Pending, Running, Completed, Failed, Timeout, Cancelled).

**Script Status**: Lifecycle state (Draft, Active, Paused, Disabled, Archived).

**Version**: Immutable snapshot of script source code for audit trail and rollback.

**Dead Letter**: Failed event-triggered execution moved to retry queue or manual review.

**Cron Expression**: Schedule definition using standard cron syntax (e.g., `0 0 * * *`).

**HTTP Path**: URL endpoint mapped to a script (e.g., `/api/scripts/my-handler`).

**Event Type**: Domain event name that triggers script execution (e.g., `user.created`, `payment.processed`).

**Metadata**: Key-value pairs for script categorization, tagging, and custom configuration.

## Quick Navigation

1. **[00-overview.md](./00-overview.md)** (this file) - Executive summary, glossary, feature list
2. **[01-architecture.md](./01-architecture.md)** - System design, component diagrams, integration points
3. **[02-domain-model.md](./02-domain-model.md)** - DDD entities, aggregates, value objects, domain events
4. **[03-database-schema.md](./03-database-schema.md)** - PostgreSQL DDL, indexes, constraints, migrations
5. **[04-repository-layer.md](./04-repository-layer.md)** - Repository interfaces, implementations, mappers

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

## High-Level Feature List

### 1. Scheduled Scripts (Cron)
- Define scripts with standard cron expressions
- Automatic execution on schedule
- Timezone support
- Execution history and metrics
- Next run calculation
- Overlapping execution prevention

**Use Cases:**
- Nightly data exports
- Weekly report generation
- Monthly billing runs
- Hourly data synchronization

### 2. HTTP Endpoint Scripts
- Map scripts to HTTP routes
- Support GET, POST, PUT, DELETE methods
- Request/response transformation
- Query parameters and request body access
- Custom HTTP headers
- Authentication/authorization integration

**Use Cases:**
- Custom API endpoints
- Webhooks for third-party integrations
- Data validation endpoints
- Custom business logic APIs

### 3. Event-Triggered Scripts
- Subscribe to domain events
- Automatic execution on event publish
- Event payload access
- Retry logic with dead letter queue
- Ordered vs parallel execution
- Event filtering by tenant

**Use Cases:**
- Send email on user registration
- Update inventory on order completion
- Log audit trail on payment
- Trigger notifications on status change

### 4. One-Off Scripts
- Manual script execution via UI/API
- Ad-hoc data processing
- Testing and debugging
- Input parameter support
- Result inspection

**Use Cases:**
- Data migration scripts
- One-time fixes
- Testing business logic
- Administrative tasks

### 5. Embedded Scripts
- JavaScript execution within Go code
- Programmatic script invocation
- Shared VM pool usage
- Synchronous and asynchronous execution

**Use Cases:**
- Custom validation rules
- Dynamic pricing calculations
- Extensible business logic
- Plugin architecture

## Technology Choices

### Goja JavaScript Engine

**Why Goja:**
- Pure Go implementation (no CGO, simplified deployment)
- ECMAScript 5.1+ with select ES6 features (arrow functions, template literals)
- Memory-safe execution
- Excellent Go interoperability
- Active maintenance and community
- Built-in performance optimizations

**Limitations:**
- No native Promises/async-await (use callbacks or channels)
- Limited ES6+ features compared to V8
- Slower than native JavaScript engines (acceptable for business logic)

**Trade-offs:**
- Simplicity and deployment ease over maximum performance
- Safety and isolation over feature completeness

### Monaco Editor

**Why Monaco:**
- Industry-standard editor (powers VS Code)
- Syntax highlighting for JavaScript
- IntelliSense and autocompletion
- Error detection and validation
- Diff view for version comparison
- Customizable themes
- MIT license

**Integration:**
- Browser-based editor embedded in IOTA SDK UI
- Real-time syntax validation
- Type definitions for available APIs
- Snippet library for common patterns

### PostgreSQL Persistence

**Why PostgreSQL:**
- ACID compliance for script definitions
- JSON/JSONB for flexible metadata storage
- Full-text search for script discovery
- Robust indexing for performance
- Multi-tenant isolation with row-level security
- Audit trail with versioning tables

**Schema Design:**
- Normalized tables for scripts, executions, versions
- Foreign key constraints for referential integrity
- Partial indexes for performance
- CHECK constraints for data validation

### VM Pooling

**Why VM Pooling:**
- Reduced cold-start latency (pre-warmed VMs)
- Resource isolation (one tenant per VM)
- Graceful degradation under load
- Memory limit enforcement
- Fair scheduling across tenants

**Implementation:**
- Pool size based on system resources
- Warm-up on application start
- Lazy expansion under load
- Idle timeout for resource cleanup
- Per-tenant VM limits

## Security Considerations

**Multi-Tenant Isolation:**
- Tenant ID injected into script context
- Database queries automatically scoped to tenant
- No cross-tenant data access
- Separate execution tracking per tenant

**Resource Limits:**
- Maximum execution time (timeout)
- Maximum memory usage
- Maximum API call rate
- Maximum concurrent executions per tenant
- Maximum script size

**Sandbox Restrictions:**
- No file system access
- No network access (except via controlled HTTP client)
- No process spawning
- No native module loading
- No eval() or Function() constructor (configurable)

**Input Validation:**
- Script source code sanitization
- Execution input validation
- SQL injection prevention in API bindings
- XSS prevention in HTTP responses

## Success Criteria

**Performance:**
- Script execution start time < 100ms (warm pool)
- Support 1000+ concurrent executions
- Query response time < 50ms for script listing
- Version diff rendering < 200ms

**Reliability:**
- 99.9% uptime for script execution service
- Zero data loss on script updates
- Graceful degradation under resource pressure
- Automatic retry for transient failures

**Usability:**
- Intuitive script editor with IntelliSense
- Clear error messages with line numbers
- Execution logs with searchable output
- Version history with one-click rollback

**Security:**
- Zero cross-tenant data leaks
- All executions within resource limits
- Audit trail for all script changes
- Security review for all API bindings

## Implementation Roadmap

**Phase 1: Core Infrastructure** (Issues #411, #412, #413)
- Domain model and database schema
- Repository layer with tenant isolation
- VM pooling and resource management
- Script versioning and audit trail

**Phase 2: Execution Triggers** (Issues #414, #415, #416, #417)
- Cron scheduler integration
- HTTP endpoint routing
- EventBus subscription
- One-off execution API

**Phase 3: API Bindings** (Issues #418, #419, #420)
- Standard library (console, JSON, date)
- Database access with query builder
- HTTP client with request/response
- IOTA SDK service access

**Phase 4: UI & Editor** (Issue #148)
- Monaco Editor integration
- Script management UI
- Execution monitoring dashboard
- Version diff viewer

**Phase 5: Production Hardening**
- Comprehensive test coverage
- Performance benchmarking
- Security audit
- Documentation and examples
