# Database Access Specification

**Status:** Draft

## Overview

Applets need controlled access to the SDK database for:
1. Reading existing SDK data (users, clients, chats, etc.)
2. Writing to existing SDK tables (with permissions)
3. Creating custom tables for applet-specific data

All access must maintain tenant isolation and security boundaries.

## Access Patterns

### 1. Read Access to SDK Tables

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

**Allowed Operations:**
- INSERT new records
- UPDATE existing records (owned by tenant)
- Soft DELETE (if table supports it)

**Blocked Operations:**
- Hard DELETE without explicit permission
- UPDATE records from other tenants
- Modifying system columns (id, tenant_id, created_at)

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

**Under the Hood:**

```go
func (proxy *DatabaseProxy) Insert(ctx context.Context, table string, data map[string]interface{}) (*Row, error) {
    // 1. Verify write permission
    if !proxy.permissions.CanWrite(table) {
        return nil, ErrTableNotAllowed{Table: table, Operation: "write"}
    }

    // 2. Inject tenant_id (MANDATORY)
    tenantID := composables.UseTenantID(ctx)
    data["tenant_id"] = tenantID

    // 3. Block protected columns
    protectedColumns := []string{"id", "tenant_id", "created_at", "updated_at"}
    for _, col := range protectedColumns {
        if _, exists := data[col]; exists && col != "tenant_id" {
            delete(data, col) // Silently remove, or error
        }
    }

    // 4. Audit log
    proxy.auditLog.Log(AuditEntry{
        AppletID:  ctx.Value("applet_id").(string),
        TenantID:  tenantID,
        Operation: "INSERT",
        Table:     table,
        Data:      data,
    })

    // 5. Execute
    return proxy.pool.Insert(ctx, table, data)
}
```

### 3. Custom Applet Tables

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

**Automatic Columns:**

Every applet table automatically gets:
- `tenant_id` (uuid, NOT NULL, indexed)
- `created_at` (timestamptz, default now())
- `updated_at` (timestamptz, auto-updated)

## Migration Strategy

### Installation Migration

When applet is installed, migrations run:

```go
func (m *MigrationRunner) InstallApplet(applet *Applet) error {
    for _, table := range applet.Manifest.Tables {
        // 1. Generate CREATE TABLE SQL
        sql := generateCreateTableSQL(applet.ID, table)

        // 2. Validate SQL (no dangerous operations)
        if err := validateMigrationSQL(sql); err != nil {
            return err
        }

        // 3. Execute in transaction
        if err := m.pool.Exec(ctx, sql); err != nil {
            return err
        }

        // 4. Record migration
        m.recordMigration(applet.ID, table.Name, "create")
    }
    return nil
}
```

### Schema Updates

Applet updates can modify schema:

```yaml
# manifest.yaml (v1.1.0)
tables:
  - name: "ai_chat_configs"
    columns:
      # ... existing columns ...
      - name: max_tokens        # NEW COLUMN
        type: integer
        default: 2000
```

**Update Migration:**

```go
func (m *MigrationRunner) UpdateApplet(oldVersion, newVersion *Applet) error {
    oldTables := indexTables(oldVersion.Manifest.Tables)
    newTables := indexTables(newVersion.Manifest.Tables)

    for tableName, newTable := range newTables {
        oldTable, exists := oldTables[tableName]
        if !exists {
            // New table - create it
            m.createTable(newVersion.ID, newTable)
            continue
        }

        // Compare columns
        diff := diffColumns(oldTable.Columns, newTable.Columns)

        for _, added := range diff.Added {
            // Add new columns (with defaults only, for safety)
            m.addColumn(newVersion.ID, tableName, added)
        }

        // Removed columns are NOT deleted automatically
        // They are marked as deprecated
        for _, removed := range diff.Removed {
            m.markColumnDeprecated(newVersion.ID, tableName, removed)
        }
    }

    return nil
}
```

### Uninstallation

When applet is uninstalled:

```go
func (m *MigrationRunner) UninstallApplet(applet *Applet) error {
    // Option 1: Soft delete (default)
    // Rename tables with _deleted_ prefix, keep data for recovery

    // Option 2: Hard delete (if explicitly requested)
    // DROP TABLE IF EXISTS ...

    // Option 3: Export and delete
    // Export data to JSON/CSV, then drop
}
```

**Admin Configuration:**

```yaml
# SDK settings
applets:
  uninstall:
    behavior: soft_delete  # soft_delete | hard_delete | export_and_delete
    retention_days: 30     # For soft delete
```

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

**Query Builder Implementation:**

```typescript
// @iota/applet-sdk/db.ts
class QueryBuilder {
  private tableName: string;
  private selects: string[] = [];
  private wheres: WhereClause[] = [];
  private joins: JoinClause[] = [];
  private orderBys: OrderByClause[] = [];
  private limitValue?: number;

  constructor(private sdk: AppletSDK, table: string) {
    this.tableName = table;
  }

  select(...columns: string[]): this {
    this.selects.push(...columns);
    return this;
  }

  where(column: string, operator: string, value: any): this {
    this.wheres.push({ column, operator, value });
    return this;
  }

  async get(): Promise<Row[]> {
    const { sql, params } = this.build();
    return this.sdk.execute('query', { sql, params });
  }

  private build(): { sql: string; params: any[] } {
    // Build parameterized SQL
    // All values become parameters, never interpolated
  }
}
```

## Tenant Isolation Enforcement

### Automatic Filtering

Every query is automatically filtered by tenant:

```sql
-- Original applet query
SELECT * FROM clients WHERE status = 'active'

-- After tenant filter injection
SELECT * FROM clients WHERE status = 'active' AND tenant_id = $TENANT_ID
```

**Implementation Strategies:**

```go
// Strategy 1: SQL Rewriting
func injectTenantFilter(sql string, tenantID uuid.UUID) string {
    // Parse SQL AST
    ast := parseSQL(sql)

    // Find all table references
    tables := findTableReferences(ast)

    // Add tenant_id condition for each table
    for _, table := range tables {
        addWhereCondition(ast, fmt.Sprintf("%s.tenant_id = '%s'", table.Alias, tenantID))
    }

    return ast.String()
}

// Strategy 2: Row-Level Security (RLS)
// Set session variable before query
func (proxy *DatabaseProxy) Query(ctx context.Context, sql string, args ...interface{}) ([]Row, error) {
    tenantID := composables.UseTenantID(ctx)

    // Set RLS context
    proxy.pool.Exec(ctx, "SET app.tenant_id = $1", tenantID)

    // Execute query (RLS policies handle filtering)
    return proxy.pool.Query(ctx, sql, args...)
}
```

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

func (proxy *DatabaseProxy) enforceLimit(sql string, limits QueryLimits) string {
    // Add LIMIT if not present
    if !hasLimit(sql) {
        sql = addLimit(sql, limits.MaxRows)
    }

    // Clamp existing LIMIT
    existingLimit := extractLimit(sql)
    if existingLimit > limits.MaxRows {
        sql = replaceLimit(sql, limits.MaxRows)
    }

    return sql
}
```

### Connection Pooling

```go
// Applets share a limited connection pool
type AppletConnectionPool struct {
    pool          *pgxpool.Pool
    maxPerApplet  int           // Max connections per applet
    totalMax      int           // Total connections for all applets
}

func (p *AppletConnectionPool) Acquire(ctx context.Context, appletID string) (*pgxpool.Conn, error) {
    // Check per-applet limit
    if p.activeConnections[appletID] >= p.maxPerApplet {
        return nil, ErrTooManyConnections
    }

    // Acquire from pool
    return p.pool.Acquire(ctx)
}
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

    cached, err := c.cache.Get(ctx, key).Bytes()
    if err != nil {
        return nil, false
    }

    var rows []Row
    json.Unmarshal(cached, &rows)
    return rows, true
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

// Logged on every operation
func (proxy *DatabaseProxy) logOperation(ctx context.Context, op DatabaseOperation) {
    log := DatabaseAuditLog{
        ID:         uuid.New(),
        Timestamp:  time.Now(),
        AppletID:   ctx.Value("applet_id").(string),
        TenantID:   composables.UseTenantID(ctx),
        UserID:     composables.UseUserID(ctx),
        Operation:  op.Type,
        Table:      op.Table,
        SQL:        sanitizeSQL(op.SQL),
        RowCount:   op.RowCount,
        DurationMs: int(op.Duration.Milliseconds()),
        Success:    op.Error == nil,
    }

    if op.Error != nil {
        errStr := op.Error.Error()
        log.Error = &errStr
    }

    proxy.auditStore.Save(log)
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

Before installation, table schemas are validated:

```go
func validateTableSchema(table TableDefinition) error {
    // Must have primary key
    if !hasPrimaryKey(table) {
        return ErrMissingPrimaryKey
    }

    // Must have tenant_id
    if !hasTenantID(table) {
        return ErrMissingTenantID
    }

    // Validate column types
    for _, col := range table.Columns {
        if !isValidColumnType(col.Type) {
            return ErrInvalidColumnType{Column: col.Name, Type: col.Type}
        }
    }

    // Validate foreign keys point to allowed tables
    for _, col := range table.Columns {
        if col.ForeignKey != nil {
            if !canReferenceTo(col.ForeignKey.Table) {
                return ErrCannotReferenceTable{Table: col.ForeignKey.Table}
            }
        }
    }

    // Check reserved names
    if isReservedTableName(table.Name) {
        return ErrReservedTableName{Name: table.Name}
    }

    return nil
}
```
