---
layout: default
title: Database Access
parent: Applet System
grand_parent: Specifications
nav_order: 8
description: "Database access patterns and tenant isolation for applets"
---

# Database Access Specification

**Status:** Draft

## Overview

Applets need controlled access to the SDK database for:

```mermaid
mindmap
  root((Database Access))
    Read
      SDK tables
      Custom tables
      Filtered by tenant
    Write
      Insert records
      Update records
      Soft delete
    Custom Tables
      Manifest defined
      Auto tenant_id
      Migration support
```

All access must maintain tenant isolation and security boundaries.

## Access Patterns

### 1. Read Access to SDK Tables

```mermaid
sequenceDiagram
    participant Applet
    participant Proxy as Database Proxy
    participant Pool as Connection Pool
    participant DB as PostgreSQL

    Applet->>Proxy: query(sql, args)
    Proxy->>Proxy: Parse tables from SQL
    Proxy->>Proxy: Verify read permission
    Proxy->>Proxy: Inject tenant_id filter
    Proxy->>Proxy: Add row limit (1000)
    Proxy->>Pool: Execute with timeout
    Pool->>DB: SELECT ... WHERE tenant_id = ?
    DB-->>Pool: Results
    Pool-->>Proxy: Rows
    Proxy-->>Applet: Filtered data
```

Applets can request read access to specific SDK tables:

```yaml
permissions:
  database:
    read:
      - users          # Core users table
      - clients        # CRM clients
      - chats          # CRM chats
      - chat_messages  # CRM messages
```

**Implementation:**

```typescript
// Applet code
const clients = await sdk.db.query(`
  SELECT id, first_name, last_name, phone
  FROM clients
  WHERE created_at > $1
  ORDER BY created_at DESC
  LIMIT 100
`, [lastSyncDate]);
```

**Under the Hood:**

```go
func (proxy *DatabaseProxy) Query(ctx context.Context, sql string, args ...interface{}) ([]Row, error) {
    // 1. Parse SQL to extract tables
    tables := parseTables(sql)

    // 2. Verify read permission for each table
    for _, table := range tables {
        if !proxy.permissions.CanRead(table) {
            return nil, ErrTableNotAllowed{Table: table, Operation: "read"}
        }
    }

    // 3. Inject tenant_id filter (CRITICAL)
    tenantID := composables.UseTenantID(ctx)
    sql = injectTenantFilter(sql, tenantID)

    // 4. Add safety limits
    sql = addRowLimit(sql, 1000)     // Max rows
    sql = addTimeout(sql, 5000)       // 5 second timeout

    // 5. Execute
    return proxy.pool.Query(ctx, sql, args...)
}
```

### 2. Write Access to SDK Tables

More restricted than read access:

```yaml
permissions:
  database:
    write:
      - clients        # Can create/update clients
      - chats          # Can create/update chats
      - chat_messages  # Can create messages
```

| Operation | Allowed | Notes |
|-----------|---------|-------|
| INSERT | ✓ | tenant_id auto-injected |
| UPDATE | ✓ | Only own tenant's records |
| Soft DELETE | ✓ | If table supports it |
| Hard DELETE | ✗ | Requires explicit permission |
| Modify system columns | ✗ | id, tenant_id, created_at blocked |

**Implementation:**

```typescript
// Applet code - creating a chat message
const message = await sdk.db.insert('chat_messages', {
  chat_id: chatId,
  content: aiResponse,
  sender_type: 'ai',
  // tenant_id is automatically injected
});
```

### 3. Custom Applet Tables

```mermaid
flowchart TB
    subgraph "Custom Table Creation"
        MANIFEST[Declare in manifest.yaml] --> APPROVAL[Admin Approval]
        APPROVAL --> MIGRATE[Run Migrations]
        MIGRATE --> TABLE[Table Created]
    end

    TABLE --> PREFIX[Prefixed: applet_id_tablename]
    TABLE --> TENANT[Auto tenant_id column]
    TABLE --> AUDIT[Auto created_at/updated_at]

    style APPROVAL fill:#f59e0b,stroke:#d97706,color:#fff
    style TABLE fill:#10b981,stroke:#047857,color:#fff
```

Applets can declare custom tables in their manifest:

```yaml
permissions:
  database:
    createTables: true  # Requires admin approval

tables:
  - name: "ai_chat_configs"
    description: "AI chat configuration per tenant"
    columns:
      - name: id
        type: bigserial
        primary: true
      - name: tenant_id
        type: uuid
        required: true
        index: true
        foreignKey:
          table: tenants
          column: id
          onDelete: CASCADE
      - name: model_name
        type: varchar(100)
        default: "gpt-4"
      - name: system_prompt
        type: text
        nullable: true
      - name: temperature
        type: decimal(3,2)
        default: 0.7
      - name: created_at
        type: timestamptz
        default: now()
      - name: updated_at
        type: timestamptz
        default: now()
    indexes:
      - columns: [tenant_id]
        unique: true
```

**Table Naming Convention:**

All applet tables are prefixed to prevent collisions:

```
applet_{applet_id}_{table_name}

Example: applet_ai_chat_ai_chat_configs
```

## Migration Strategy

### Installation Migration

```mermaid
sequenceDiagram
    participant PM as Package Manager
    participant MR as Migration Runner
    participant DB as Database

    PM->>MR: InstallApplet(applet)
    loop For each table
        MR->>MR: Generate CREATE TABLE SQL
        MR->>MR: Validate SQL (no dangerous ops)
        MR->>DB: Execute in transaction
        MR->>MR: Record migration
    end
    MR-->>PM: Success
```

### Schema Updates

```mermaid
flowchart LR
    OLD[Old Version] --> DIFF[Diff Columns]
    NEW[New Version] --> DIFF
    DIFF --> ADDED[Add new columns]
    DIFF --> REMOVED[Mark deprecated]
    ADDED --> MIGRATE[Apply Migration]
    REMOVED --> MIGRATE

    style MIGRATE fill:#10b981,stroke:#047857,color:#fff
```

### Uninstallation Options

| Option | Description |
|--------|-------------|
| **Soft Delete** | Rename tables with `_deleted_` prefix, keep for 30 days |
| **Hard Delete** | `DROP TABLE IF EXISTS` immediately |
| **Export & Delete** | Export to JSON/CSV, then drop |

## Query Builder API

Instead of raw SQL, applets can use a query builder:

```typescript
// Safe query builder
const clients = await sdk.db.table('clients')
  .select('id', 'first_name', 'last_name', 'phone')
  .where('created_at', '>', lastSyncDate)
  .orderBy('created_at', 'desc')
  .limit(100)
  .get();

// With joins (if permitted)
const messages = await sdk.db.table('chat_messages')
  .join('chats', 'chat_messages.chat_id', '=', 'chats.id')
  .select('chat_messages.*', 'chats.client_id')
  .where('chat_messages.created_at', '>', yesterday)
  .get();

// Inserts
const newMessage = await sdk.db.table('chat_messages')
  .insert({
    chat_id: chatId,
    content: 'Hello!',
    sender_type: 'ai',
  });

// Updates
await sdk.db.table('clients')
  .where('id', clientId)
  .update({
    last_contacted_at: new Date(),
  });
```

## Tenant Isolation Enforcement

### Automatic Filtering

```mermaid
flowchart LR
    subgraph "Query Transformation"
        ORIG["SELECT * FROM clients<br/>WHERE status = 'active'"]
        TRANS["SELECT * FROM clients<br/>WHERE status = 'active'<br/>AND tenant_id = $TENANT_ID"]
    end

    ORIG --> TRANS

    style TRANS fill:#10b981,stroke:#047857,color:#fff
```

**Implementation Strategies:**

| Strategy | Description |
|----------|-------------|
| **SQL Rewriting** | Parse SQL AST, inject tenant_id conditions |
| **Row-Level Security** | Use PostgreSQL RLS policies |

### Cross-Tenant Prevention

Queries that could access other tenants are blocked:

```go
func validateQuery(sql string) error {
    // Block UNION that could bypass filters
    if containsUnion(sql) {
        return ErrUnionNotAllowed
    }

    // Block subqueries without tenant filter
    if hasUnfilteredSubquery(sql) {
        return ErrSubqueryMustBeFiltered
    }

    // Block direct tenant_id manipulation
    if modifiesTenantID(sql) {
        return ErrCannotModifyTenantID
    }

    return nil
}
```

## Performance Safeguards

### Query Limits

```go
type QueryLimits struct {
    MaxRows           int           // 1000 default
    MaxExecutionTime  time.Duration // 5 seconds default
    MaxJoins          int           // 3 default
    MaxSubqueries     int           // 2 default
}
```

### Connection Pooling

```mermaid
graph TB
    subgraph "Connection Pool"
        POOL[Shared Pool]
        A1[Applet A: max 5]
        A2[Applet B: max 5]
        A3[Applet C: max 5]
    end

    POOL --> A1
    POOL --> A2
    POOL --> A3

    style POOL fill:#3b82f6,stroke:#1e40af,color:#fff
```

### Query Caching

```go
// Read queries can be cached
type QueryCache struct {
    cache    *redis.Client
    ttl      time.Duration
    maxSize  int
}

func (c *QueryCache) Get(ctx context.Context, sql string, args []interface{}) ([]Row, bool) {
    key := hashQuery(sql, args, composables.UseTenantID(ctx))
    // Cache key includes tenant_id for isolation
}
```

## Audit Logging

All database operations are logged:

```go
type DatabaseAuditLog struct {
    ID          uuid.UUID
    Timestamp   time.Time
    AppletID    string
    TenantID    uuid.UUID
    UserID      *uint
    Operation   string    // SELECT, INSERT, UPDATE, DELETE
    Table       string
    SQL         string    // Sanitized (no sensitive data)
    RowCount    int
    DurationMs  int
    Success     bool
    Error       *string
}
```

## Data Types Mapping

| Manifest Type | PostgreSQL Type | TypeScript Type |
|---------------|-----------------|-----------------|
| `bigserial` | `BIGSERIAL` | `number` |
| `serial` | `SERIAL` | `number` |
| `uuid` | `UUID` | `string` |
| `varchar(n)` | `VARCHAR(n)` | `string` |
| `text` | `TEXT` | `string` |
| `integer` | `INTEGER` | `number` |
| `bigint` | `BIGINT` | `number` |
| `decimal(p,s)` | `DECIMAL(p,s)` | `number` |
| `boolean` | `BOOLEAN` | `boolean` |
| `timestamptz` | `TIMESTAMPTZ` | `Date` |
| `date` | `DATE` | `string` |
| `jsonb` | `JSONB` | `Record<string, any>` |
| `bytea` | `BYTEA` | `Uint8Array` |

## Schema Validation

```mermaid
flowchart TB
    TABLE[Table Definition] --> PK{Has Primary Key?}
    PK -->|No| ERR1[❌ Missing PK]
    PK -->|Yes| TENANT{Has tenant_id?}
    TENANT -->|No| ERR2[❌ Missing tenant_id]
    TENANT -->|Yes| TYPES{Valid column types?}
    TYPES -->|No| ERR3[❌ Invalid type]
    TYPES -->|Yes| FK{Valid foreign keys?}
    FK -->|No| ERR4[❌ Invalid FK]
    FK -->|Yes| NAME{Reserved name?}
    NAME -->|Yes| ERR5[❌ Reserved name]
    NAME -->|No| PASS[✓ Valid Schema]

    style PASS fill:#10b981,stroke:#047857,color:#fff
    style ERR1 fill:#ef4444,stroke:#b91c1c,color:#fff
    style ERR2 fill:#ef4444,stroke:#b91c1c,color:#fff
    style ERR3 fill:#ef4444,stroke:#b91c1c,color:#fff
    style ERR4 fill:#ef4444,stroke:#b91c1c,color:#fff
    style ERR5 fill:#ef4444,stroke:#b91c1c,color:#fff
```

---

## Next Steps

- Review [Distribution](./distribution.md) for packaging
- See [Permissions](./permissions.md) for security model
- Check [Architecture](./architecture.md) for system design
