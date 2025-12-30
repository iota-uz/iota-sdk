# Permission & Security Model

**Status:** Draft

## Overview

The permission model defines what applets can and cannot do. It operates at three levels:

1. **Installation Permissions:** What the applet declares it needs
2. **Tenant Permissions:** What tenants allow for their organization
3. **User Permissions:** What individual users can do within the applet

## Permission Categories

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

**Read Access:**
- SELECT queries only
- Automatic tenant_id filtering (cannot access other tenants)
- Row limit (max 1000 per query)
- Query timeout (5 seconds)

**Write Access:**
- INSERT, UPDATE, DELETE
- Automatic tenant_id injection
- Audit logging of all changes
- Transaction support

**Create Tables:**
- Requires explicit admin approval
- Tables prefixed with `applet_{applet_id}_`
- Automatic tenant_id column enforcement
- Migration review before execution

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

**Validation:**
- DNS resolution checked before request
- All resolved IPs validated
- Redirect following validates each hop

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

**Subscribe:**
- Receives events matching declared patterns
- Events filtered by tenant_id
- Async execution (doesn't block event bus)

**Publish:**
- Can only publish declared event types
- Events tagged with applet source
- Rate limited (30 events/minute default)

### 4. UI Permissions

```yaml
permissions:
  ui:
    navigation: true
    pages: true
    widgets: true
```

**Navigation:**
- Can add items to sidebar
- Can nest under existing sections
- Cannot override core navigation

**Pages:**
- Can register new routes
- Routes prefixed: `/applets/{applet-id}/...`
- Cannot override existing SDK routes

**Widgets:**
- Can inject into declared slots
- Cannot access DOM outside widget
- Size/position constraints apply

### 5. Secret Permissions

```yaml
permissions:
  secrets:
    - name: "OPENAI_API_KEY"
      required: true
    - name: "WEBHOOK_SECRET"
      required: false
```

**Handling:**
- Secrets stored encrypted per-tenant
- Injected into runtime context
- Never logged or exposed in errors
- Admin-only configuration

## Permission Enforcement

### Installation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Permission Review Flow                        │
│                                                                  │
│  1. Upload/Select Applet Package                                │
│                    │                                             │
│                    ▼                                             │
│  2. Parse Manifest, Extract Permissions                         │
│                    │                                             │
│                    ▼                                             │
│  3. Display Permission Summary to Admin                         │
│     ┌─────────────────────────────────────────────────────┐    │
│     │ This applet requests:                                │    │
│     │                                                      │    │
│     │ ⚠️ DATABASE                                          │    │
│     │   Read: clients, chats, chat_messages               │    │
│     │   Write: clients, chats                             │    │
│     │   Create Tables: YES (requires review)              │    │
│     │                                                      │    │
│     │ 🌐 EXTERNAL HTTP                                     │    │
│     │   api.openai.com, *.dify.ai                         │    │
│     │                                                      │    │
│     │ 📡 EVENTS                                            │    │
│     │   Subscribe: chat.message.created, client.created   │    │
│     │   Publish: ai.response.generated                    │    │
│     │                                                      │    │
│     │ 🔐 SECRETS                                           │    │
│     │   OPENAI_API_KEY (required)                         │    │
│     │   DIFY_API_KEY (optional)                           │    │
│     │                                                      │    │
│     │ [Approve] [Reject] [Review Tables]                  │    │
│     └─────────────────────────────────────────────────────┘    │
│                    │                                             │
│                    ▼                                             │
│  4. Admin Approves (possibly with restrictions)                 │
│                    │                                             │
│                    ▼                                             │
│  5. Install with Approved Permissions                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
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

```go
func (proxy *DatabaseProxy) Query(ctx context.Context, sql string, args ...interface{}) ([]Row, error) {
    // 1. Parse SQL to extract table names
    tables := parseTables(sql)

    // 2. Check permissions for each table
    for _, table := range tables {
        if err := proxy.enforcer.CheckDatabaseAccess(table, "read"); err != nil {
            return nil, err
        }
    }

    // 3. Inject tenant_id filter
    tenantID := composables.UseTenantID(ctx)
    sql = injectTenantFilter(sql, tenantID)

    // 4. Add row limit
    sql = addRowLimit(sql, 1000)

    // 5. Execute with timeout
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    return proxy.pool.Query(ctx, sql, args...)
}
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

```
┌─────────────────────────────────────────────────────────────────┐
│                      Security Layers                             │
│                                                                  │
│  Layer 5: User Permissions (RBAC)                               │
│  └─ What can this user do within the applet?                    │
│                                                                  │
│  Layer 4: Tenant Isolation                                      │
│  └─ All data queries filtered by tenant_id                      │
│                                                                  │
│  Layer 3: Permission Enforcement                                │
│  └─ Applet can only access declared resources                   │
│                                                                  │
│  Layer 2: Runtime Sandboxing                                    │
│  └─ Process isolation, resource limits                          │
│                                                                  │
│  Layer 1: Input Validation                                      │
│  └─ All inputs sanitized, SQL parameterized                     │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

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
