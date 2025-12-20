# JavaScript Runtime - API Bindings

## Overview

API bindings expose controlled JavaScript APIs to scripts for database access, HTTP requests, caching, logging, and event publishing with tenant isolation and security enforcement.

```mermaid
graph TB
    subgraph "JavaScript APIs (Exposed to Scripts)"
        Context[context<br/>tenantId, userId, scriptId, input]
        HTTP[sdk.http<br/>SSRF-protected requests]
        DB[sdk.db<br/>Tenant-scoped queries]
        Cache[sdk.cache<br/>Key-value storage]
        Log[sdk.log<br/>Structured logging]
        Events[events<br/>Publish domain events]
    end

    subgraph "Go Implementation (Backend)"
        HTTPClient[HTTP Client<br/>Allowlist validation]
        DBClient[Database Client<br/>Parameterized queries]
        CacheClient[Cache Client<br/>Redis/Memory]
        Logger[Logger<br/>Execution context]
        EventPublisher[Event Publisher<br/>EventBus]
    end

    subgraph "Security Layer"
        TenantIsolation[Tenant ID Injection]
        SSRFProtection[SSRF Prevention]
        SQLInjection[SQL Injection Prevention]
        RateLimiting[API Rate Limiting]
    end

    Context --> TenantIsolation
    HTTP --> HTTPClient
    HTTP --> SSRFProtection
    DB --> DBClient
    DB --> SQLInjection
    DB --> TenantIsolation
    Cache --> CacheClient
    Cache --> TenantIsolation
    Log --> Logger
    Events --> EventPublisher
    Events --> TenantIsolation

    HTTPClient --> RateLimiting
    DBClient --> RateLimiting
```

## Context API

**What It Provides:**
Execution context information available to every script via the `context` global object.

**Available Properties:**
- `context.tenantId` - Current tenant UUID (read-only)
- `context.userId` - Authenticated user ID (read-only)
- `context.organizationId` - Organization UUID (read-only)
- `context.scriptId` - Executing script UUID (read-only)
- `context.executionId` - Current execution UUID (read-only)
- `context.input` - Input parameters passed to script
- `context.trigger` - Trigger information (type, event data, HTTP request)

```mermaid
classDiagram
    class Context {
        +string tenantId
        +number userId
        +string organizationId
        +string scriptId
        +string executionId
        +object input
        +TriggerData trigger
    }

    class TriggerData {
        +string type
        +string eventType
        +object eventPayload
        +HTTPRequest httpRequest
        +string cronExpression
    }

    class HTTPRequest {
        +string method
        +string path
        +object headers
        +object query
        +object body
    }

    Context --> TriggerData
    TriggerData --> HTTPRequest
```

**Usage Example (conceptual):**
- Access tenant: `const tenantId = context.tenantId`
- Access input: `const name = context.input.name`
- Check trigger: `if (context.trigger.type === 'http')`

## HTTP Client API

**What It Provides:**
SSRF-protected HTTP client for making external API calls from scripts.

**Methods:**
- `sdk.http.get(url, options)` - GET request
- `sdk.http.post(url, body, options)` - POST request
- `sdk.http.put(url, body, options)` - PUT request
- `sdk.http.delete(url, options)` - DELETE request
- `sdk.http.patch(url, body, options)` - PATCH request

**Options:**
- `headers` - Custom HTTP headers (object)
- `query` - Query parameters (object)
- `timeout` - Request timeout in milliseconds (default: 10000)

```mermaid
sequenceDiagram
    participant Script
    participant HTTPClient
    participant SSRFValidator
    participant ExternalAPI

    Script->>HTTPClient: sdk.http.get(url, options)
    HTTPClient->>SSRFValidator: Validate URL

    alt URL allowed (public internet, allowlist)
        SSRFValidator-->>HTTPClient: Valid
        HTTPClient->>ExternalAPI: HTTP GET
        ExternalAPI-->>HTTPClient: Response
        HTTPClient-->>Script: {status, headers, body}
    else URL blocked (private IP, localhost, cloud metadata)
        SSRFValidator-->>HTTPClient: Blocked
        HTTPClient-->>Script: Error: SSRF protection
    end
```

### SSRF Protection

**What It Does:**
Prevents scripts from accessing internal/private network resources.

**Blocked URLs:**
- Private IP ranges (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Localhost (127.0.0.1, ::1)
- Link-local addresses (169.254.0.0/16)
- Cloud metadata endpoints (169.254.169.254)
- Internal DNS names

**Allowed URLs:**
- Public internet (default)
- Configured allowlist domains
- Specific partner APIs

```mermaid
graph TB
    subgraph "URL Validation Flow"
        URL[URL Input]
        Parse[Parse URL]
        CheckIP[Resolve & Check IP]
        CheckAllowlist[Check Allowlist]
    end

    subgraph "Blocked"
        Private[Private IP]
        Localhost[Localhost]
        Metadata[Cloud Metadata]
        LinkLocal[Link-Local]
    end

    subgraph "Allowed"
        Public[Public Internet]
        Allowlist[Allowlist Domain]
    end

    URL --> Parse
    Parse --> CheckIP

    CheckIP -->|Private IP| Private
    CheckIP -->|Localhost| Localhost
    CheckIP -->|Metadata IP| Metadata
    CheckIP -->|Link-local| LinkLocal
    CheckIP -->|Public IP| CheckAllowlist

    CheckAllowlist -->|In allowlist| Allowlist
    CheckAllowlist -->|Public & not blocked| Public

    Private -.Reject.-> Error[Error: SSRF]
    Localhost -.Reject.-> Error
    Metadata -.Reject.-> Error
    LinkLocal -.Reject.-> Error

    Allowlist -.Accept.-> Request[Make HTTP Request]
    Public -.Accept.-> Request
```

## Database API

**What It Provides:**
Tenant-scoped database queries with automatic tenant_id injection and SQL injection prevention.

**Methods:**
- `sdk.db.query(sql, params)` - Execute SELECT query
- `sdk.db.execute(sql, params)` - Execute INSERT/UPDATE/DELETE
- `sdk.db.queryOne(sql, params)` - Get single row
- `sdk.db.transaction(fn)` - Execute in transaction

**Security Features:**
- Automatic tenant_id injection in WHERE clause
- Parameterized queries only (no string concatenation)
- Read-only access for SELECT (no DDL/DCL)
- Row count limits (max 1000 rows)
- Query timeout (max 5 seconds)

```mermaid
sequenceDiagram
    participant Script
    participant DBClient
    participant TenantValidator
    participant PostgreSQL

    Script->>DBClient: sdk.db.query(sql, params)
    DBClient->>DBClient: Validate SQL (no DDL/DCL)
    DBClient->>TenantValidator: Inject tenant_id
    TenantValidator-->>DBClient: Modified SQL + params

    DBClient->>PostgreSQL: Execute parameterized query<br/>WHERE tenant_id = $1 AND ...
    PostgreSQL-->>DBClient: Rows (max 1000)

    DBClient->>DBClient: Map rows to JavaScript objects
    DBClient-->>Script: Array of objects
```

### Query Patterns

**Supported Queries:**
- SELECT with filters, joins, aggregations
- INSERT with returning clause
- UPDATE with WHERE clause
- DELETE with WHERE clause
- Transactions (BEGIN, COMMIT, ROLLBACK)

**Restricted Operations:**
- No DDL (CREATE, ALTER, DROP)
- No DCL (GRANT, REVOKE)
- No TRUNCATE or VACUUM
- No file system access (COPY, pg_read_file)

```mermaid
graph TB
    subgraph "Query Validation"
        Input[SQL Query]
        Parse[Parse SQL]
        CheckType[Check Query Type]
    end

    subgraph "Allowed"
        SELECT[SELECT]
        INSERT[INSERT]
        UPDATE[UPDATE]
        DELETE[DELETE]
    end

    subgraph "Blocked"
        DDL[DDL CREATE/ALTER/DROP]
        DCL[DCL GRANT/REVOKE]
        System[System Functions]
    end

    Input --> Parse
    Parse --> CheckType

    CheckType -->|SELECT| SELECT
    CheckType -->|INSERT| INSERT
    CheckType -->|UPDATE| UPDATE
    CheckType -->|DELETE| DELETE
    CheckType -->|DDL/DCL| DDL
    CheckType -->|System| System

    SELECT -.Add tenant_id.-> Execute[Execute with Tenant Filter]
    INSERT -.Add tenant_id.-> Execute
    UPDATE -.Add tenant_id.-> Execute
    DELETE -.Add tenant_id.-> Execute

    DDL -.Reject.-> Error[Error: Unauthorized]
    DCL -.Reject.-> Error
    System -.Reject.-> Error
```

## Cache API

**What It Provides:**
Tenant-scoped key-value cache for temporary data storage.

**Methods:**
- `sdk.cache.get(key)` - Retrieve value by key
- `sdk.cache.set(key, value, ttl)` - Store value with TTL (seconds)
- `sdk.cache.delete(key)` - Remove key
- `sdk.cache.exists(key)` - Check if key exists
- `sdk.cache.increment(key, delta)` - Atomic increment
- `sdk.cache.expire(key, ttl)` - Update TTL

**Automatic Prefixing:**
- Keys automatically prefixed with `tenant:{tenantId}:script:{scriptId}:`
- Prevents cross-tenant and cross-script key collisions
- Transparent to script (script sees unprefixed keys)

```mermaid
sequenceDiagram
    participant Script
    participant CacheClient
    participant Redis

    Script->>CacheClient: sdk.cache.set('user:123', data, 3600)
    CacheClient->>CacheClient: Prefix key:<br/>tenant:{id}:script:{id}:user:123
    CacheClient->>Redis: SET prefixed_key data EX 3600
    Redis-->>CacheClient: OK
    CacheClient-->>Script: true

    Script->>CacheClient: sdk.cache.get('user:123')
    CacheClient->>CacheClient: Prefix key
    CacheClient->>Redis: GET prefixed_key
    Redis-->>CacheClient: data
    CacheClient-->>Script: data
```

## Logging API

**What It Provides:**
Structured logging with automatic execution context linkage.

**Methods:**
- `sdk.log.debug(message, metadata)` - Debug level
- `sdk.log.info(message, metadata)` - Info level
- `sdk.log.warn(message, metadata)` - Warning level
- `sdk.log.error(message, metadata)` - Error level

**Automatic Context:**
- tenant_id, user_id, organization_id
- script_id, execution_id
- Timestamp, log level
- Custom metadata from script

```mermaid
graph TB
    subgraph "Logging Flow"
        Script[Script calls sdk.log.info]
        Logger[Logger Implementation]
        Enrich[Enrich with Context]
        Format[Format as JSON]
        Write[Write to stdout]
    end

    subgraph "Log Entry"
        Timestamp[timestamp]
        Level[level: info]
        Message[message]
        Context[tenant_id, script_id, execution_id]
        Metadata[Custom metadata]
    end

    Script --> Logger
    Logger --> Enrich
    Enrich --> Format
    Format --> Write

    Enrich -.Adds.-> Timestamp
    Enrich -.Adds.-> Level
    Enrich -.Adds.-> Context
    Script -.Provides.-> Message
    Script -.Provides.-> Metadata
```

## Events API

**What It Provides:**
Publish domain events to EventBus with tenant isolation.

**Methods:**
- `events.publish(eventType, payload)` - Publish event
- Event types follow convention: `module.entity.action`

**Automatic Enrichment:**
- Tenant ID automatically injected
- Timestamp added
- Source script ID tracked
- Event ID generated

```mermaid
sequenceDiagram
    participant Script
    participant EventsAPI
    participant EventBus
    participant Subscribers

    Script->>EventsAPI: events.publish('user.created', {userId: 123})
    EventsAPI->>EventsAPI: Enrich event<br/>(tenant_id, timestamp, script_id)
    EventsAPI->>EventsAPI: Validate event type
    EventsAPI->>EventBus: Publish enriched event

    EventBus->>Subscribers: Notify (tenant-filtered)
    EventBus-->>EventsAPI: Published
    EventsAPI-->>Script: true
```

## Rate Limiting

**What It Does:**
Prevents abuse by limiting API call frequency per script execution.

**Limits:**
- Database queries: 60 per minute
- HTTP requests: 60 per minute
- Cache operations: 120 per minute
- Event publishes: 30 per minute
- Log entries: 100 per minute

**How It Works:**
- Token bucket algorithm per execution
- Refills at configured rate
- Blocks when bucket empty
- Returns error with retry-after

```mermaid
graph TB
    subgraph "Rate Limiter (Per Execution)"
        Request[API Call]
        CheckBucket[Check Token Bucket]
        ConsumeToken[Consume Token]
        Allow[Allow Request]
        Deny[Deny Request]
    end

    subgraph "Token Bucket"
        Capacity[Capacity: 60]
        Current[Current: X]
        RefillRate[Refill: 1/second]
    end

    Request --> CheckBucket
    CheckBucket -->|Tokens available| ConsumeToken
    CheckBucket -->|Bucket empty| Deny

    ConsumeToken --> Current
    ConsumeToken --> Allow

    RefillRate -.Adds tokens.-> Current

    Deny -.Error.-> Script[Error: Rate limit exceeded]
    Allow -.Success.-> Execute[Execute API Call]
```

## API Injection Process

**What It Does:**
Injects all APIs into Goja VM global scope before script execution.

**Injection Steps:**
1. Create API client instances (HTTP, DB, Cache, Logger, Events)
2. Inject execution context (`context` global)
3. Bind HTTP client methods to `sdk.http` namespace
4. Bind database methods to `sdk.db` namespace
5. Bind cache methods to `sdk.cache` namespace
6. Bind logger methods to `sdk.log` namespace
7. Bind event publisher to `events` global
8. Set tenant ID in all API clients

```mermaid
sequenceDiagram
    participant VMPool
    participant VM as Goja VM
    participant Bindings as API Bindings

    VMPool->>Bindings: InjectAPIs(vm, ctx)

    Bindings->>VM: Set context global
    Note over VM: context.tenantId = "..."<br/>context.userId = 123

    Bindings->>VM: Set sdk.http namespace
    Note over VM: sdk.http.get = func(...)<br/>sdk.http.post = func(...)

    Bindings->>VM: Set sdk.db namespace
    Note over VM: sdk.db.query = func(...)<br/>sdk.db.execute = func(...)

    Bindings->>VM: Set sdk.cache namespace
    Note over VM: sdk.cache.get = func(...)<br/>sdk.cache.set = func(...)

    Bindings->>VM: Set sdk.log namespace
    Note over VM: sdk.log.info = func(...)<br/>sdk.log.error = func(...)

    Bindings->>VM: Set events global
    Note over VM: events.publish = func(...)

    Bindings-->>VMPool: APIs injected
```

## Acceptance Criteria

### Context API
- ✅ context.tenantId available (read-only, non-null)
- ✅ context.userId available (read-only, may be null)
- ✅ context.organizationId available (read-only, may be null)
- ✅ context.scriptId available (read-only, non-null)
- ✅ context.executionId available (read-only, non-null)
- ✅ context.input provides script input parameters
- ✅ context.trigger provides trigger information

### HTTP Client API
- ✅ Support GET, POST, PUT, DELETE, PATCH methods
- ✅ Custom headers and query parameters
- ✅ Request timeout configurable (default 10s)
- ✅ SSRF protection blocks private IPs
- ✅ SSRF protection blocks cloud metadata
- ✅ Allowlist domains bypass SSRF checks
- ✅ Rate limiting (60 requests/minute)

### Database API
- ✅ sdk.db.query executes SELECT with tenant filter
- ✅ sdk.db.execute for INSERT/UPDATE/DELETE
- ✅ sdk.db.queryOne returns single row
- ✅ sdk.db.transaction for atomic operations
- ✅ Automatic tenant_id injection in WHERE
- ✅ Parameterized queries only (SQL injection prevention)
- ✅ Row limit (max 1000 rows)
- ✅ Query timeout (max 5 seconds)
- ✅ Block DDL/DCL operations

### Cache API
- ✅ sdk.cache.get/set/delete/exists methods
- ✅ sdk.cache.increment for atomic counters
- ✅ sdk.cache.expire to update TTL
- ✅ Automatic key prefixing (tenant + script)
- ✅ TTL in seconds (default: no expiration)
- ✅ Rate limiting (120 operations/minute)

### Logging API
- ✅ sdk.log.debug/info/warn/error methods
- ✅ Automatic context enrichment (tenant, script, execution)
- ✅ Custom metadata support
- ✅ JSON formatted output
- ✅ Rate limiting (100 logs/minute)

### Events API
- ✅ events.publish method
- ✅ Automatic tenant ID injection
- ✅ Timestamp and event ID generation
- ✅ Source script ID tracking
- ✅ Event type validation
- ✅ Rate limiting (30 events/minute)

### Security
- ✅ All APIs enforce tenant isolation
- ✅ No cross-tenant data access possible
- ✅ SSRF protection for HTTP calls
- ✅ SQL injection prevention for database
- ✅ Rate limiting prevents abuse
- ✅ API errors include safe error messages (no stack traces to scripts)
