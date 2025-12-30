---
layout: default
title: Permissions & Security
parent: Applet System
grand_parent: Specifications
nav_order: 7
description: "Permission model and security boundaries for applets"
---

# Permission & Security Model

**Status:** Draft

## Overview

The permission model defines what applets can and cannot do. It operates at three levels:

```mermaid
graph TB
    subgraph "Permission Levels"
        L1[Installation Permissions<br/>What applet declares it needs]
        L2[Tenant Permissions<br/>What tenants allow]
        L3[User Permissions<br/>What users can do]
    end

    L1 --> L2
    L2 --> L3

    style L1 fill:#3b82f6,stroke:#1e40af,color:#fff
    style L2 fill:#10b981,stroke:#047857,color:#fff
    style L3 fill:#f59e0b,stroke:#d97706,color:#fff
```

## Permission Categories

```mermaid
mindmap
  root((Permissions))
    Database
      Read tables
      Write tables
      Create tables
    HTTP
      External hosts
      Blocked IPs
    Events
      Subscribe
      Publish
    UI
      Navigation
      Pages
      Widgets
    Secrets
      API keys
      Tokens
```

### 1. Database Permissions

```yaml
permissions:
  database:
    read:
      - clients
      - chats
      - chat_messages
    write:
      - clients
      - chats
    createTables: true
```

| Access Type | Capabilities |
|-------------|--------------|
| **Read** | SELECT queries only, automatic tenant_id filtering, row limit (1000), query timeout (5s) |
| **Write** | INSERT, UPDATE, DELETE, automatic tenant_id injection, audit logging |
| **Create Tables** | Requires admin approval, prefixed with `applet_{id}_`, automatic tenant_id column |

### 2. External HTTP Permissions

```yaml
permissions:
  http:
    external:
      - "api.openai.com"
      - "*.dify.ai"
      - "api.stripe.com"
```

**Allowed:**
- HTTPS connections to declared hosts
- Wildcard subdomains (*.example.com)
- Standard HTTP methods

**Blocked (Always):**
- Private IP ranges (10.x, 172.16.x, 192.168.x)
- Localhost (127.0.0.1, ::1)
- Cloud metadata endpoints (169.254.169.254)
- Non-HTTPS connections (configurable)

### 3. Event Permissions

```yaml
permissions:
  events:
    subscribe:
      - "chat.message.created"
      - "client.created"
    publish:
      - "ai.response.generated"
```

### 4. UI Permissions

```yaml
permissions:
  ui:
    navigation: true
    pages: true
    widgets: true
```

### 5. Secret Permissions

```yaml
permissions:
  secrets:
    - name: "OPENAI_API_KEY"
      required: true
    - name: "WEBHOOK_SECRET"
      required: false
```

## Permission Enforcement

### Installation Flow

```mermaid
flowchart TB
    START[Upload Applet Package] --> PARSE[Parse Manifest]
    PARSE --> EXTRACT[Extract Permissions]
    EXTRACT --> DISPLAY[Display Permission Summary]

    DISPLAY --> REVIEW{Admin Review}
    REVIEW -->|Approve| INSTALL[Install with Approved Permissions]
    REVIEW -->|Reject| CANCEL[Cancel Installation]
    REVIEW -->|Review Tables| TABLES[Review Table Definitions]
    TABLES --> REVIEW

    style START fill:#3b82f6,stroke:#1e40af,color:#fff
    style INSTALL fill:#10b981,stroke:#047857,color:#fff
    style CANCEL fill:#ef4444,stroke:#b91c1c,color:#fff
```

### Runtime Enforcement

```go
type PermissionEnforcer struct {
    allowedTables     map[string]TablePermission
    allowedHosts      []string
    allowedEvents     []string
    publishableEvents []string
}

func (e *PermissionEnforcer) CheckDatabaseAccess(table string, operation string) error {
    perm, ok := e.allowedTables[table]
    if !ok {
        return ErrTableNotAllowed{Table: table}
    }
    if operation == "write" && !perm.Write {
        return ErrWriteNotAllowed{Table: table}
    }
    return nil
}

func (e *PermissionEnforcer) CheckHTTPAccess(host string) error {
    for _, allowed := range e.allowedHosts {
        if matchHost(host, allowed) {
            return nil
        }
    }
    return ErrHostNotAllowed{Host: host}
}
```

### Database Query Interception

```mermaid
sequenceDiagram
    participant Applet
    participant Proxy as Database Proxy
    participant Enforcer as Permission Enforcer
    participant DB as Database

    Applet->>Proxy: Query(sql, args)
    Proxy->>Enforcer: CheckDatabaseAccess(tables)
    Enforcer-->>Proxy: OK / Error

    alt Permission Denied
        Proxy-->>Applet: Error: Table not allowed
    else Permission OK
        Proxy->>Proxy: Inject tenant_id filter
        Proxy->>Proxy: Add row limit
        Proxy->>DB: Execute with timeout
        DB-->>Proxy: Results
        Proxy-->>Applet: Filtered results
    end
```

## User-Level Permissions

### Applet Permission Keys

Applets define their own permission keys:

```yaml
appletPermissions:
  - key: "ai-chat.config.read"
    name: { en: "View AI Chat Configuration" }
  - key: "ai-chat.config.write"
    name: { en: "Edit AI Chat Configuration" }
  - key: "ai-chat.assist"
    name: { en: "Use AI Assistant" }
```

### Integration with SDK RBAC

```go
// Applet permissions are registered with SDK
func (a *Applet) RegisterPermissions(app application.Application) {
    for _, perm := range a.Manifest.AppletPermissions {
        app.RegisterPermission(permission.Permission{
            Key:         fmt.Sprintf("applet.%s.%s", a.ID, perm.Key),
            Name:        perm.Name,
            Description: perm.Description,
            Module:      fmt.Sprintf("applet-%s", a.ID),
        })
    }
}

// Checked in handlers
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    if !sdkcomposables.CanUser(r.Context(), "applet.ai-chat.config.write") {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }
    // ... handle request
}
```

## Security Boundaries

### Sandboxing Layers

```mermaid
graph TB
    subgraph "Security Layers"
        L5[Layer 5: User Permissions - RBAC]
        L4[Layer 4: Tenant Isolation]
        L3[Layer 3: Permission Enforcement]
        L2[Layer 2: Runtime Sandboxing]
        L1[Layer 1: Input Validation]
    end

    L5 --> L4
    L4 --> L3
    L3 --> L2
    L2 --> L1

    style L5 fill:#8b5cf6,stroke:#5b21b6,color:#fff
    style L4 fill:#3b82f6,stroke:#1e40af,color:#fff
    style L3 fill:#10b981,stroke:#047857,color:#fff
    style L2 fill:#f59e0b,stroke:#d97706,color:#fff
    style L1 fill:#ef4444,stroke:#b91c1c,color:#fff
```

| Layer | Purpose |
|-------|---------|
| **Layer 5** | What can this user do within the applet? |
| **Layer 4** | All data queries filtered by tenant_id |
| **Layer 3** | Applet can only access declared resources |
| **Layer 2** | Process isolation, resource limits |
| **Layer 1** | All inputs sanitized, SQL parameterized |

### Threat Model

| Threat | Mitigation |
|--------|------------|
| SQL Injection | Parameterized queries only, no raw SQL |
| Cross-Tenant Access | Automatic tenant_id filtering |
| SSRF | Host allowlist, IP validation |
| Resource Exhaustion | CPU/memory/time limits |
| Data Exfiltration | Audit logging, rate limits |
| Privilege Escalation | Permission checks at every layer |
| Code Injection | No eval(), no dynamic code execution |

## Audit Logging

All applet actions are logged:

```go
type AuditLog struct {
    Timestamp   time.Time
    AppletID    string
    TenantID    string
    UserID      *uint
    Action      string    // "db.query", "http.request", "event.publish"
    Resource    string    // Table name, URL, event type
    Details     JSONB     // Query, request body, etc.
    Success     bool
    Error       *string
    DurationMs  int
}
```

## Rate Limiting

```yaml
# Per-applet limits
rateLimits:
  http:
    requestsPerMinute: 1000
    requestsPerHour: 10000
  database:
    queriesPerMinute: 100
    rowsPerMinute: 10000
  events:
    publishPerMinute: 30
  external:
    requestsPerMinute: 60
```

## Emergency Controls

```mermaid
flowchart LR
    ERROR[Errors Detected] --> THRESHOLD{Threshold Exceeded?}
    THRESHOLD -->|Yes| TRIP[Trip Circuit Breaker]
    THRESHOLD -->|No| CONTINUE[Continue]
    TRIP --> COOLDOWN[Cooldown Period]
    COOLDOWN --> RETRY[Retry]
    RETRY --> THRESHOLD

    ADMIN[Admin Action] --> DISABLE[Emergency Disable]
    DISABLE --> STOP[Stop All Handlers]
    STOP --> LOG[Log Incident]
    LOG --> NOTIFY[Notify Admins]

    style TRIP fill:#ef4444,stroke:#b91c1c,color:#fff
    style DISABLE fill:#ef4444,stroke:#b91c1c,color:#fff
```

```go
// Circuit breaker for misbehaving applets
type CircuitBreaker struct {
    ErrorThreshold    int           // Errors before tripping
    ErrorWindow       time.Duration // Window to count errors
    CooldownDuration  time.Duration // How long to stay open
}

// Admin can disable applet instantly
func (m *Manager) EmergencyDisable(appletID string, reason string) error {
    // Stop all handlers
    // Log incident
    // Notify admins
    // Preserve state for investigation
}
```

---

## Next Steps

- Review [Database](./database.md) for data access patterns
- See [Distribution](./distribution.md) for packaging
- Check [Architecture](./architecture.md) for system design
