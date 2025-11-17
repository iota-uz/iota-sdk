# Repository Layer Patterns

**Data persistence patterns for IOTA SDK repositories following DDD principles.**

## Purpose

Repositories abstract database operations and provide clean interfaces for data access:
- Interface in domain layer
- Implementation in infrastructure layer
- Tenant isolation
- Transaction support
- Query optimization

## Repository Structure

### Interface Definition

**Location**: `modules/{module}/domain/*/repository.go`

```go
type EntityNameRepository interface {
    FindByID(ctx context.Context, id uuid.UUID) (EntityName, error)
    FindAll(ctx context.Context, filters FilterParams) ([]EntityName, int, error)
    Create(ctx context.Context, e EntityName) error
    Update(ctx context.Context, e EntityName) error
    Delete(ctx context.Context, id uuid.UUID) error
}
```

**Characteristics**:
- Uses domain types (interfaces, not structs)
- No implementation details
- Context-first parameters
- Returns domain entities

### Implementation

**Location**: `modules/{module}/infrastructure/persistence/*_repository.go`

```go
type entityNameRepository struct{}

func NewEntityNameRepository() domain.EntityNameRepository {
    return &entityNameRepository{}
}
```

## Query Patterns

### SQL Constants

**Always define SQL as constants**:

```go
const (
    entityNameFindByIDQuery = `
        SELECT id, tenant_id, name, created_at, updated_at
        FROM entity_names
        WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
    `

    entityNameInsertQuery = repo.Insert(
        "entity_names",
        []string{"id", "tenant_id", "name", "created_at", "updated_at"},
        "id",
    )

    entityNameUpdateQuery = `
        UPDATE entity_names
        SET name = $1, updated_at = $2
        WHERE id = $3 AND tenant_id = $4 AND deleted_at IS NULL
    `

    entityNameDeleteQuery = `
        UPDATE entity_names
        SET deleted_at = NOW()
        WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
    `
)
```

### Dynamic Queries with pkg/repo

**For filtering, sorting, pagination**:

```go
func (r *entityNameRepository) FindAll(
    ctx context.Context,
    filters FilterParams,
) ([]EntityName, int, error) {
    const op serrors.Op = "entityNameRepository.FindAll"

    tenantID := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    // Build base query
    builder := repo.NewQueryBuilder().
        Select("id", "tenant_id", "name", "created_at", "updated_at").
        From("entity_names").
        Where("tenant_id = ?", tenantID).
        Where("deleted_at IS NULL")

    // Apply filters
    if filters.Name != "" {
        builder.Where("name ILIKE ?", "%"+filters.Name+"%")
    }

    if filters.Status != "" {
        builder.Where("status = ?", filters.Status)
    }

    // Apply sorting
    if filters.SortBy != "" {
        direction := "ASC"
        if filters.SortDesc {
            direction = "DESC"
        }
        builder.OrderBy(filters.SortBy + " " + direction)
    }

    // Apply pagination
    builder.
        Limit(filters.Limit).
        Offset(filters.Offset)

    // Build query
    query, args := builder.Build()

    // Execute
    rows, err := tx.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, serrors.E(op, err)
    }
    defer rows.Close()

    // Scan results
    var entities []EntityName
    for rows.Next() {
        entity, err := scanEntity(rows)
        if err != nil {
            return nil, 0, serrors.E(op, err)
        }
        entities = append(entities, entity)
    }

    // Get total count
    countQuery, countArgs := builder.
        ClearSelect().
        Select("COUNT(*)").
        ClearLimit().
        ClearOffset().
        Build()

    var total int
    err = tx.QueryRow(ctx, countQuery, countArgs...).Scan(&total)
    if err != nil {
        return nil, 0, serrors.E(op, err)
    }

    return entities, total, nil
}
```

## Tenant Isolation

### Always Include tenant_id

**Every query must filter by tenant_id**:

```go
func (r *entityNameRepository) FindByID(
    ctx context.Context,
    id uuid.UUID,
) (EntityName, error) {
    const op serrors.Op = "entityNameRepository.FindByID"

    tenantID := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    var entity entityName
    err := tx.QueryRow(
        ctx,
        entityNameFindByIDQuery,
        id,
        tenantID,  // CRITICAL: Always include tenant_id
    ).Scan(
        &entity.id,
        &entity.tenantID,
        &entity.name,
        &entity.createdAt,
        &entity.updatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, serrors.E(op, serrors.KindNotFound, err)
        }
        return nil, serrors.E(op, err)
    }

    return &entity, nil
}
```

### Getting Tenant ID from Context

```go
// Extract tenant ID from context
tenantID := composables.UseTenantID(ctx)

// Use in queries
WHERE tenant_id = $1
```

## Transaction Support

### Using Transactions

**Get transaction from context**:

```go
func (r *entityNameRepository) Create(
    ctx context.Context,
    e EntityName,
) error {
    const op serrors.Op = "entityNameRepository.Create"

    tx := composables.UseTx(ctx)

    _, err := tx.Exec(
        ctx,
        entityNameInsertQuery,
        e.ID(),
        e.TenantID(),
        e.Name(),
        e.CreatedAt(),
        e.UpdatedAt(),
    )

    if err != nil {
        return serrors.E(op, err)
    }

    return nil
}
```

### Service-Level Transaction Coordination

**Transactions managed by services** (see `domain-service.md`):

```go
// Service layer handles transaction lifecycle
func (s *service) ComplexOperation(ctx context.Context) error {
    tx, err := pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx)

    // Repositories use transaction from context
    txCtx := composables.WithTx(ctx, tx)

    if err := s.repo1.Create(txCtx, entity1); err != nil {
        return err
    }

    if err := s.repo2.Update(txCtx, entity2); err != nil {
        return err
    }

    return tx.Commit(ctx)
}
```

## CRUD Operations

### Create

```go
func (r *entityNameRepository) Create(
    ctx context.Context,
    e EntityName,
) error {
    const op serrors.Op = "entityNameRepository.Create"

    tx := composables.UseTx(ctx)

    _, err := tx.Exec(
        ctx,
        entityNameInsertQuery,
        e.ID(),
        e.TenantID(),
        e.Name(),
        e.CreatedAt(),
        e.UpdatedAt(),
    )

    if err != nil {
        return serrors.E(op, err)
    }

    return nil
}
```

### Read

```go
func (r *entityNameRepository) FindByID(
    ctx context.Context,
    id uuid.UUID,
) (EntityName, error) {
    const op serrors.Op = "entityNameRepository.FindByID"

    tenantID := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    var entity entityName
    err := tx.QueryRow(
        ctx,
        entityNameFindByIDQuery,
        id,
        tenantID,
    ).Scan(
        &entity.id,
        &entity.tenantID,
        &entity.name,
        &entity.createdAt,
        &entity.updatedAt,
    )

    if err != nil {
        if errors.Is(err, pgx.ErrNoRows) {
            return nil, serrors.E(op, serrors.KindNotFound, err)
        }
        return nil, serrors.E(op, err)
    }

    return &entity, nil
}
```

### Update

```go
func (r *entityNameRepository) Update(
    ctx context.Context,
    e EntityName,
) error {
    const op serrors.Op = "entityNameRepository.Update"

    tx := composables.UseTx(ctx)

    result, err := tx.Exec(
        ctx,
        entityNameUpdateQuery,
        e.Name(),
        time.Now(),
        e.ID(),
        e.TenantID(),
    )

    if err != nil {
        return serrors.E(op, err)
    }

    if result.RowsAffected() == 0 {
        return serrors.E(op, serrors.KindNotFound, "entity not found")
    }

    return nil
}
```

### Delete (Soft Delete)

```go
func (r *entityNameRepository) Delete(
    ctx context.Context,
    id uuid.UUID,
) error {
    const op serrors.Op = "entityNameRepository.Delete"

    tenantID := composables.UseTenantID(ctx)
    tx := composables.UseTx(ctx)

    result, err := tx.Exec(
        ctx,
        entityNameDeleteQuery,
        id,
        tenantID,
    )

    if err != nil {
        return serrors.E(op, err)
    }

    if result.RowsAffected() == 0 {
        return serrors.E(op, serrors.KindNotFound, "entity not found")
    }

    return nil
}
```

## Error Handling

### Operation Tracking

**Always use `serrors.Op`**:

```go
const op serrors.Op = "repositoryName.MethodName"
```

### Error Wrapping

**Wrap all errors with context**:

```go
// Not found error
if errors.Is(err, pgx.ErrNoRows) {
    return nil, serrors.E(op, serrors.KindNotFound, err)
}

// General database error
if err != nil {
    return serrors.E(op, err)
}

// No rows affected (for UPDATE/DELETE)
if result.RowsAffected() == 0 {
    return serrors.E(op, serrors.KindNotFound, "entity not found")
}
```

## Query Optimization

### Indexes

**Essential indexes** (defined in migrations):

```sql
-- Tenant isolation
CREATE INDEX idx_entity_names_tenant_id ON entity_names(tenant_id);

-- Soft delete filtering
CREATE INDEX idx_entity_names_deleted_at ON entity_names(deleted_at);

-- Filtered index for active records
CREATE INDEX idx_entity_names_status ON entity_names(status)
WHERE deleted_at IS NULL;

-- Foreign keys
CREATE INDEX idx_entity_names_parent_id ON entity_names(parent_id);

-- Commonly filtered fields
CREATE INDEX idx_entity_names_created_at ON entity_names(created_at DESC);
```

### Avoiding N+1 Queries

**Use JOINs or batch loading**:

```go
// Bad: N+1 queries
for _, order := range orders {
    client, _ := clientRepo.FindByID(ctx, order.ClientID())
}

// Good: JOIN query
const ordersWithClientsQuery = `
    SELECT o.*, c.*
    FROM orders o
    JOIN clients c ON o.client_id = c.id
    WHERE o.tenant_id = $1 AND o.deleted_at IS NULL
`

// Or: Batch loading
clientIDs := extractClientIDs(orders)
clients, _ := clientRepo.FindByIDs(ctx, clientIDs)
```

## Testing

### Repository Tests

**Location**: `modules/{module}/infrastructure/persistence/*_repository_test.go`

**Pattern** (see `testing.md` for details):

```go
func TestRepositoryName_Create(t *testing.T) {
    t.Parallel()
    f := setupTest(t)
    repo := persistence.NewRepositoryName()

    entity := domain.New("Test Name")
    err := repo.Create(f.Ctx, entity)

    require.NoError(t, err)
}

func TestRepositoryName_FindByID(t *testing.T) {
    t.Parallel()
    f := setupTest(t)
    repo := persistence.NewRepositoryName()

    // Create
    entity := domain.New("Test Name")
    _ = repo.Create(f.Ctx, entity)

    // Find
    found, err := repo.FindByID(f.Ctx, entity.ID())

    require.NoError(t, err)
    assert.Equal(t, entity.ID(), found.ID())
    assert.Equal(t, entity.Name(), found.Name())
}
```

## Best Practices

### Always

- [ ] Interface in domain layer (`modules/{module}/domain/*/repository.go`)
- [ ] Implementation in infrastructure (`modules/{module}/infrastructure/persistence/*`)
- [ ] `serrors.Op` for operation tracking
- [ ] `composables.UseTx(ctx)` for transactions
- [ ] `composables.UseTenantID(ctx)` for tenant isolation
- [ ] Parameterized queries ($1, $2) - no concatenation
- [ ] SQL as constants, `pkg/repo` for dynamic queries
- [ ] Soft deletes (update deleted_at, not physical DELETE)
- [ ] Check RowsAffected() for UPDATE/DELETE
- [ ] Return `serrors.KindNotFound` for missing records

### Never

- [ ] Raw SQL concatenation (SQL injection risk)
- [ ] Skip tenant_id in queries (data leakage risk)
- [ ] Put business logic in repositories (belongs in services)
- [ ] Use concrete types in repository interface (use domain interfaces)
- [ ] Forget to check RowsAffected()
- [ ] Skip error wrapping with `serrors.E`
- [ ] Use transactions directly in repositories (let services manage)

## Common Patterns

### Pagination

```go
type FilterParams struct {
    Limit    int
    Offset   int
    Page     int
    SortBy   string
    SortDesc bool
}

func (r *repo) FindAll(ctx context.Context, filters FilterParams) ([]Entity, int, error) {
    // Apply LIMIT and OFFSET
    // Return entities and total count
}
```

### Filtering

```go
// Use pkg/repo for dynamic filters
builder := repo.NewQueryBuilder().
    Select("*").
    From("table").
    Where("tenant_id = ?", tenantID)

if filter.Name != "" {
    builder.Where("name ILIKE ?", "%"+filter.Name+"%")
}

query, args := builder.Build()
```

### Batch Operations

```go
func (r *repo) CreateBatch(ctx context.Context, entities []Entity) error {
    tx := composables.UseTx(ctx)

    batch := &pgx.Batch{}
    for _, e := range entities {
        batch.Queue(insertQuery, e.ID(), e.TenantID(), e.Name())
    }

    results := tx.SendBatch(ctx, batch)
    defer results.Close()

    for range entities {
        _, err := results.Exec()
        if err != nil {
            return serrors.E(op, err)
        }
    }

    return nil
}
```

## Integration with Migrations

Repository implementations must match schema defined in migrations (see `migrations.md`):

```sql
-- Migration defines schema
CREATE TABLE entity_names (
    id uuid PRIMARY KEY,
    tenant_id uuid NOT NULL,
    name VARCHAR(255) NOT NULL,
    ...
);

-- Repository queries match schema
SELECT id, tenant_id, name FROM entity_names WHERE ...
```

## Integration with Services

Repositories are injected into services via DI (see `domain-service.md`):

```go
type Service struct {
    repo domain.EntityNameRepository  // Interface, not implementation
}

func NewService(repo domain.EntityNameRepository) *Service {
    return &Service{repository: repo}
}
```
