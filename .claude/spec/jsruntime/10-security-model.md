# JavaScript Runtime - Security Model Specification

## Overview

The security model protects the JavaScript runtime from malicious code, unauthorized access, resource abuse, and cross-tenant data leakage. It implements defense-in-depth with multiple security layers from input validation through SSRF protection.

```mermaid
graph TB
    A[User Request] --> B[Input Validation Layer]
    B --> C[RBAC Permission Layer]
    C --> D[Tenant Isolation Layer]
    D --> E[VM Sandboxing Layer]
    E --> F[Resource Limits Layer]
    F --> G[SSRF Protection Layer]
    G --> H[Audit Trail Layer]
    H --> I[Execution Result]

    style B fill:#ffe1e1
    style C fill:#ffe1e1
    style D fill:#ffe1e1
    style E fill:#ffe1e1
    style F fill:#fff4e1
    style G fill:#ffe1e1
    style H fill:#e1f5ff
```

## What It Does

The security model:
- **Validates** all user input to prevent injection attacks
- **Enforces** RBAC permissions for script operations
- **Isolates** tenant data to prevent cross-tenant access
- **Sandboxes** JavaScript execution to remove dangerous globals
- **Limits** resource usage (CPU, memory, API calls) to prevent abuse
- **Blocks** SSRF attacks on HTTP requests
- **Logs** all executions for audit and compliance

## How It Works

### Layer 1: Input Validation

```mermaid
sequenceDiagram
    participant User
    participant Controller
    participant Validator
    participant Sanitizer
    participant Service

    User->>Controller: Submit form/data
    Controller->>Validator: Validate DTO fields

    alt Invalid Input
        Validator-->>Controller: Validation errors
        Controller-->>User: 400 Bad Request
    else Valid Input
        Validator-->>Controller: OK
        Controller->>Sanitizer: Sanitize strings
        Sanitizer->>Sanitizer: Strip SQL, XSS, path traversal
        Sanitizer-->>Controller: Sanitized DTO
        Controller->>Service: Process DTO
    end
```

**What It Does:**
- Validates input types, lengths, formats, required fields
- Sanitizes strings to prevent SQL injection, XSS, path traversal
- Rejects malformed or malicious input before processing

**Validation Rules:**
- **Script Name**: Required, 3-100 chars, alphanumeric + spaces
- **Script Code**: Required, max 100KB
- **Trigger Type**: Required, enum (manual, scheduled, event, http)
- **Cron Schedule**: Valid cron expression if trigger=scheduled
- **HTTP Path**: Valid URL path if trigger=http
- **Event Type**: Valid event name if trigger=event

**Sanitization:**
- Strip HTML tags from text fields
- Escape SQL special characters
- Validate file paths (no `..`, absolute paths only)
- URL-encode user input in URLs

### Layer 2: RBAC Permissions

```mermaid
graph TB
    subgraph "Permission Hierarchy"
        A[Superadmin] --> B[scripts.*]
        C[Org Admin] --> D[scripts.read]
        C --> E[scripts.create]
        C --> F[scripts.update]
        C --> G[scripts.delete]
        C --> H[scripts.execute]
        I[Developer] --> D
        I --> E
        I --> H
        J[Viewer] --> D
    end

    style A fill:#ffe1e1
    style C fill:#fff4e1
    style I fill:#e1ffe1
    style J fill:#e1f5ff
```

**What It Does:**
- Controls who can create, read, update, delete, execute scripts
- Enforces role-based access at route and service levels
- Prevents privilege escalation

**Permissions:**
- `scripts.read`: View scripts and execution history
- `scripts.create`: Create new scripts
- `scripts.update`: Edit existing scripts
- `scripts.delete`: Delete scripts
- `scripts.execute`: Manually execute scripts

**Permission Checks:**
```
Route Middleware → Check permission → Allow/Deny
Service Method → Check permission → Allow/Deny (defense in depth)
Template Rendering → Check permission → Show/Hide UI elements
```

**Role Assignments:**
- **Superadmin**: All permissions across all tenants
- **Org Admin**: All permissions within their organization
- **Developer**: Read, Create, Execute (no Delete)
- **Viewer**: Read only

### Layer 3: Tenant Isolation

```mermaid
sequenceDiagram
    participant User1 as User (Tenant A)
    participant User2 as User (Tenant B)
    participant Service
    participant Repo as Repository
    participant DB as Database

    User1->>Service: GetScripts()
    Service->>Service: tenantID = ctx.TenantID (A)
    Service->>Repo: FindAll(tenantID=A)
    Repo->>DB: SELECT * WHERE tenant_id = 'A'
    DB-->>Repo: Scripts from Tenant A
    Repo-->>User1: Scripts (Tenant A only)

    User2->>Service: GetScripts()
    Service->>Service: tenantID = ctx.TenantID (B)
    Service->>Repo: FindAll(tenantID=B)
    Repo->>DB: SELECT * WHERE tenant_id = 'B'
    DB-->>Repo: Scripts from Tenant B
    Repo-->>User2: Scripts (Tenant B only)
```

**What It Does:**
- Ensures users only access data from their own tenant
- Prevents cross-tenant data leakage
- Validates tenant context in all database queries

**Enforcement:**
- **Context Propagation**: `composables.UseTenantID(ctx)` in all repositories
- **WHERE Clause**: `tenant_id = $1` in ALL queries (SELECT, UPDATE, DELETE)
- **Foreign Keys**: All tables reference tenants table for integrity
- **Validation**: Reject requests if tenantID missing or invalid

**Database Schema:**
```sql
-- All tables include tenant_id
CREATE TABLE scripts (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    ...
);

-- Composite index for tenant queries
CREATE INDEX idx_scripts_tenant ON scripts(tenant_id, created_at DESC);
```

### Layer 4: VM Sandboxing

```mermaid
graph TB
    subgraph "JavaScript VM Sandbox"
        A[Standard Globals] --> B{Allowed?}
        B -->|Yes| C[Available]
        B -->|No| D[Removed/Undefined]

        C --> E[Math, Date, JSON]
        C --> F[String, Array, Object]
        C --> G[console via safe wrapper]

        D --> H[eval, Function]
        D --> I[require, import]
        D --> J[process, Buffer]
        D --> K[fetch, XMLHttpRequest]
        D --> L[setTimeout, setInterval]
    end

    style E fill:#e1ffe1
    style F fill:#e1ffe1
    style G fill:#e1ffe1
    style H fill:#ffe1e1
    style I fill:#ffe1e1
    style J fill:#ffe1e1
    style K fill:#ffe1e1
    style L fill:#ffe1e1
```

**What It Does:**
- Removes dangerous JavaScript globals that could compromise security
- Provides safe alternatives for approved operations
- Freezes injected context objects to prevent tampering

**Removed Globals:**
- `eval()`, `Function()`: Dynamic code execution
- `require()`, `import`: Module loading
- `process`, `Buffer`: Node.js APIs
- `fetch()`, `XMLHttpRequest`: Uncontrolled network access
- `setTimeout()`, `setInterval()`: Timing attacks
- `localStorage`, `sessionStorage`: Browser storage

**Allowed Globals:**
- `Math`, `Date`, `JSON`: Standard utilities
- `String`, `Array`, `Object`: Data structures
- `console` (safe wrapper): Logging (captured, not printed)

**Safe API Injection:**
```javascript
// Injected via ctx (frozen object)
const ctx = Object.freeze({
    db: { query, insert, update, delete },
    http: { get, post, put, delete },
    cache: { get, set, delete },
    events: { publish },
    logger: { info, warn, error }
});
```

### Layer 5: Resource Limits

```mermaid
stateDiagram-v2
    [*] --> Executing
    Executing --> MemoryCheck: Every 100ms
    MemoryCheck --> Continue: < 128MB
    MemoryCheck --> Terminated: >= 128MB

    Executing --> TimeoutCheck: Every 1s
    TimeoutCheck --> Continue: < 30s
    TimeoutCheck --> Terminated: >= 30s

    Executing --> APICallCheck: Per API call
    APICallCheck --> Continue: < 100 calls
    APICallCheck --> Terminated: >= 100 calls

    Continue --> Executing
    Terminated --> [*]
    Executing --> Success: Return value
    Success --> [*]

    note right of Terminated
        ResourceLimitExceeded error
        Execution aborted
        Partial results discarded
    end note
```

**What It Does:**
- Prevents resource exhaustion from malicious or inefficient scripts
- Enforces CPU, memory, API call, output size limits
- Terminates runaway scripts gracefully

**Limits:**
- **Execution Timeout**: 30 seconds (configurable)
- **Memory Limit**: 128MB per execution (configurable)
- **API Call Limit**: 100 calls per execution (db, http, cache combined)
- **Output Size**: 1MB max return value
- **HTTP Response Size**: 10MB max per request

**Enforcement:**
- **Timeout**: Context cancellation after duration
- **Memory**: Periodic heap size checks (approximate)
- **API Calls**: Counter incremented per API call, checked before execution
- **Output**: JSON marshal size check after return

**Error Responses:**
```
TimeoutExceeded: "Script execution exceeded 30s timeout"
MemoryLimitExceeded: "Script exceeded 128MB memory limit"
APICallLimitExceeded: "Script exceeded 100 API call limit"
OutputTooLarge: "Return value exceeded 1MB size limit"
```

### Layer 6: SSRF Protection

```mermaid
sequenceDiagram
    participant Script
    participant HTTP as ctx.http.get()
    participant Validator
    participant Resolver as DNS Resolver
    participant Checker as IP Checker
    participant External as External API

    Script->>HTTP: get("http://example.com")
    HTTP->>Validator: Validate URL
    Validator->>Validator: Parse URL
    Validator->>Resolver: Resolve domain
    Resolver-->>Validator: IP addresses
    Validator->>Checker: Check IPs

    alt Private IP Detected
        Checker-->>Validator: Private IP (10.0.0.1)
        Validator-->>HTTP: SSRFError
        HTTP-->>Script: Error: SSRF attempt blocked
    else Public IP Only
        Checker-->>Validator: Public IP (93.184.216.34)
        Validator-->>HTTP: OK
        HTTP->>External: HTTP GET
        External-->>HTTP: Response
        HTTP-->>Script: Response data
    end
```

**What It Does:**
- Prevents Server-Side Request Forgery (SSRF) attacks
- Blocks HTTP requests to private IP ranges
- Validates DNS resolution before making requests
- Prevents access to internal services

**Blocked Targets:**
- **Private IPs**: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
- **Loopback**: 127.0.0.0/8, ::1/128
- **Link-Local**: 169.254.0.0/16, fe80::/10
- **Localhost**: localhost, 0.0.0.0
- **Cloud Metadata**: 169.254.169.254 (AWS, GCP, Azure)

**Validation Flow:**
1. Parse URL from script input
2. Resolve domain to IP addresses (DNS lookup)
3. Check all resolved IPs against blocklist
4. If any IP is private/blocked, reject request
5. If all IPs are public, allow request
6. Apply timeout (10s) and size limits (10MB)

**Configuration:**
```go
type SSRFConfig struct {
    BlockPrivateIPs    bool     // Default: true
    BlockedCIDRs       []string // Additional CIDRs to block
    AllowedDomains     []string // Whitelist specific domains
    RequestTimeout     time.Duration // Default: 10s
    MaxResponseSize    int64    // Default: 10MB
}
```

### Layer 7: Audit Trail

```mermaid
erDiagram
    script_executions {
        UUID id PK
        UUID script_id FK
        UUID tenant_id FK
        UUID user_id FK
        String status
        JSON input_data
        JSON output_data
        String error_message
        TEXT stack_trace
        Int duration_ms
        Int memory_used_bytes
        Int api_calls_made
        Timestamp executed_at
        Timestamp created_at
    }

    scripts {
        UUID id PK
    }

    users {
        UUID id PK
    }

    script_executions }|--|| scripts : "belongs to"
    script_executions }|--|| users : "executed by"
    script_executions ||--o{ tenants : "isolated by"
```

**What It Does:**
- Records every script execution with metadata
- Captures input, output, errors, performance metrics
- Enables forensic analysis and compliance auditing
- Supports debugging and monitoring

**Audit Fields:**
- **Script ID**: Which script executed
- **Tenant ID**: Which tenant initiated execution
- **User ID**: Which user triggered execution (if manual)
- **Status**: success, failure, timeout, limit_exceeded
- **Input Data**: Event data or manual input (JSON)
- **Output Data**: Script return value (JSON, truncated if > 1MB)
- **Error Message**: Exception message
- **Stack Trace**: JavaScript stack trace
- **Duration**: Execution time in milliseconds
- **Memory Used**: Approximate heap size
- **API Calls**: Count of db, http, cache calls
- **Timestamps**: executed_at, created_at

**Retention:**
- Keep execution logs for 90 days (configurable)
- Archive old logs to cold storage (S3, etc.)
- Index on (tenant_id, executed_at) for fast queries

## Security Best Practices

### Code Review
- [ ] All user-generated scripts reviewed by admins before enabling
- [ ] Automated static analysis detects common vulnerabilities
- [ ] Scripts flagged if they contain suspicious patterns

### Monitoring
- [ ] Alert on high API call rates (potential abuse)
- [ ] Alert on frequent timeouts (inefficient code)
- [ ] Alert on SSRF attempts (security incident)
- [ ] Track execution duration trends (performance regression)

### Incident Response
- [ ] Disable malicious scripts immediately
- [ ] Revoke permissions for compromised users
- [ ] Audit logs reviewed for suspicious activity
- [ ] Post-incident analysis with remediation plan

## Acceptance Criteria

### Input Validation
- [ ] All DTOs have validation tags (required, min, max, format)
- [ ] Sanitization applied to all string inputs
- [ ] Invalid input returns 400 with clear error messages
- [ ] SQL injection attempts blocked

### RBAC Permissions
- [ ] All routes protected by authentication middleware
- [ ] Permission checks enforce least privilege
- [ ] Unauthorized access returns 403 Forbidden
- [ ] Permissions tested in integration tests

### Tenant Isolation
- [ ] All queries include tenant_id WHERE clause
- [ ] Cross-tenant access returns empty results (not error)
- [ ] Foreign keys enforce referential integrity
- [ ] Tenant context validated in all service methods

### VM Sandboxing
- [ ] Dangerous globals (eval, require, fetch) are undefined
- [ ] Injected ctx object is frozen (immutable)
- [ ] Standard globals (Math, JSON) remain available
- [ ] console.log output captured, not printed to stdout

### Resource Limits
- [ ] Execution timeout enforced (30s default)
- [ ] Memory limit enforced (128MB default)
- [ ] API call limit enforced (100 calls default)
- [ ] Output size limit enforced (1MB default)
- [ ] Limit violations logged to audit trail

### SSRF Protection
- [ ] Private IPs blocked (10.x, 192.168.x, 127.x)
- [ ] Cloud metadata endpoints blocked (169.254.169.254)
- [ ] DNS resolution validates all IPs before request
- [ ] SSRF attempts logged as security incidents
- [ ] Allowed domains whitelist supported

### Audit Trail
- [ ] Every execution logged to database
- [ ] Logs include input, output, error, metrics
- [ ] Logs retained for 90 days minimum
- [ ] Audit logs accessible via admin UI
- [ ] Logs immutable (no updates, append-only)

---

**Threat Modeling:**
- **Code Injection**: Mitigated by VM sandboxing (no eval)
- **SSRF**: Mitigated by IP validation and DNS checks
- **Resource Exhaustion**: Mitigated by resource limits
- **Data Leakage**: Mitigated by tenant isolation
- **Privilege Escalation**: Mitigated by RBAC enforcement
- **Credential Theft**: Mitigated by no access to process.env

**Compliance:**
- **GDPR**: Audit logs track data processing
- **SOC 2**: Multi-tenant isolation, RBAC, audit trails
- **PCI DSS**: Input validation, encryption at rest/transit
