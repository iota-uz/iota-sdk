# Domain & Service Layer Patterns

**Domain-Driven Design patterns for IOTA SDK entities, aggregates, and services.**

## Domain Layer

### Purpose

Core business logic and interfaces:
- Aggregates as interfaces (not structs)
- Repository interfaces (no implementation)
- Domain events and value objects
- Enum types with validation

### Entity/Aggregate Pattern

**Interface-Based Design**:

```go
type Option func (*entityName)

func WithID(id uuid.UUID) Option {
    return func (e *entityName) { e.id = id }
}

func WithName(name string) Option {
    return func (e *entityName) { e.name = name }
}

// Public interface
type EntityName interface {
    ID() uuid.UUID
    TenantID() uuid.UUID
    Name() string
    CreatedAt() time.Time
    UpdatedAt() time.Time
    SetName(string) EntityName
    IsReadyToBeShipped() bool
}

// Constructor with functional options
func New(tenantID uuid.UUID, name string, opts ...Option) EntityName {
    e := &entityName{
        id:       uuid.New(),
        tenantID: tenantID,
        name:     name,
    }
    for _, opt := range opts {
        opt(e)
    }
    return e
}

// Private implementation
type entityName struct {
    id, tenantID uuid.UUID
    name         string
    createdAt    time.Time
    updatedAt    time.Time
}

// Getters
func (e *entityName) ID() uuid.UUID       { return e.id }
func (e *entityName) TenantID() uuid.UUID { return e.tenantID }
func (e *entityName) Name() string        { return e.name }

// Immutable setters (return new instance)
func (e *entityName) SetName(name string) EntityName {
    c := *e
    c.name = name
    c.updatedAt = time.Now()
    return &c
}

// Business rules inside entities
func (e *entityName) IsReadyToBeShipped() bool {
    return e.name != "" && e.createdAt.Before(time.Now().Add(-24*time.Hour))
}
```

### Key Principles

1. **Domain aggregates are interfaces**, not structs
2. **Private struct, public interface**
3. **Immutable setters** (return new instance)
4. **Business rules inside entities**
5. **Functional options** for optional fields

### Repository Interfaces

**Always in domain layer**:

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
- Defined in `modules/{module}/domain/*/repository.go`
- Use domain types (not DB types)
- No implementation details (no SQL, no pgx)
- Implementations in `infrastructure/persistence`

### Value Objects

Type-safe primitives:

```go
type EmailAddress string

func NewEmailAddress(email string) (EmailAddress, error) {
    if !isValidEmail(email) {
        return "", errors.New("invalid email")
    }
    return EmailAddress(email), nil
}

func (e EmailAddress) String() string {
    return string(e)
}
```

### Domain Enums

```go
type Status string

const (
    StatusPending    Status = "pending"
    StatusProcessing Status = "processing"
    StatusCompleted  Status = "completed"
)

func (s Status) Validate() error {
    switch s {
    case StatusPending, StatusProcessing, StatusCompleted:
        return nil
    default:
        return fmt.Errorf("invalid status: %s", s)
    }
}
```

## Service Layer

### Purpose

Orchestrate business operations:
- Service structs with repository interfaces
- Transaction management
- Business workflow coordination
- Validation before persistence
- Permission checks

### Service Pattern

**Basic Structure**:

```go
type EntityNameService struct {
    repository domain.EntityNameRepository
}

func NewEntityNameService(repo domain.EntityNameRepository) *EntityNameService {
    return &EntityNameService{repository: repo}
}
```

**Method Pattern**:

```go
func (s *EntityNameService) Create(ctx context.Context, input Input) (*Entity, error) {
    const op serrors.Op = "EntityNameService.Create"

    // 1. Validate
    if err := input.Validate(); err != nil {
        return nil, serrors.E(op, serrors.KindValidation, err)
    }

    // 2. Business logic
    entity := domain.New(
        composables.UseTenantID(ctx),
        input.Name,
    )

    // 3. Persist
    if err := s.repository.Create(ctx, entity); err != nil {
        return nil, serrors.E(op, err)
    }

    return entity, nil
}
```

### Transaction Coordination

```go
func (s *service) ComplexOperation(ctx context.Context, params Params) error {
    const op serrors.Op = "service.ComplexOperation"

    pool, err := composables.UsePool(ctx)
    if err != nil {
        return serrors.E(op, err)
    }

    // Start transaction
    tx, err := pool.Begin(ctx)
    if err != nil {
        return serrors.E(op, err)
    }
    defer tx.Rollback(ctx) // Will be no-op if committed

    // Use transaction in context
    txCtx := composables.WithTx(ctx, tx)

    // Perform operations
    if err := s.repo1.Create(txCtx, entity1); err != nil {
        return serrors.E(op, err)
    }

    if err := s.repo2.Update(txCtx, entity2); err != nil {
        return serrors.E(op, err)
    }

    // Commit transaction
    if err := tx.Commit(ctx); err != nil {
        return serrors.E(op, err)
    }

    return nil
}
```

### Permission Checks

```go
func (s *service) Update(ctx context.Context, id uuid.UUID, input Input) error {
    const op serrors.Op = "service.Update"

    // Check permissions
    if !sdkcomposables.CanUser(ctx, permissions.UpdateEntity) {
        return serrors.E(op, serrors.KindPermission, "insufficient permissions")
    }

    // Proceed with business logic
    entity, err := s.repository.FindByID(ctx, id)
    if err != nil {
        return serrors.E(op, err)
    }

    updated := entity.SetName(input.Name)

    if err := s.repository.Update(ctx, updated); err != nil {
        return serrors.E(op, err)
    }

    return nil
}
```

### Error Wrapping

**Always use `serrors.E`**:

```go
// Basic wrapping
return serrors.E(op, err)

// With error kind
return serrors.E(op, serrors.KindValidation, err)

// With custom message
return serrors.E(op, serrors.KindNotFound, "entity not found")

// With multiple context
return serrors.E(op, serrors.KindDatabase, fmt.Errorf("failed to create entity: %w", err))
```

### Validation Patterns

**Input validation**:

```go
type CreateInput struct {
    Name  string
    Email string
}

func (i CreateInput) Validate() error {
    if i.Name == "" {
        return errors.New("name is required")
    }
    if i.Email == "" {
        return errors.New("email is required")
    }
    if !isValidEmail(i.Email) {
        return errors.New("invalid email format")
    }
    return nil
}
```

## DI Pattern

### Service Dependency Injection

**Via `di.H` in controllers**:

```go
type EntityNameController struct {
    app      application.Application
    basePath string
}

func (c *EntityNameController) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()
    s.Use(middleware.Authorize(), middleware.WithPageContext())

    // di.H automatically injects dependencies by type
    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
    s.HandleFunc("/{id}", di.H(c.Delete)).Methods(http.MethodDelete)
}

// Dependencies injected by type signature
func (c *EntityNameController) List(
    r *http.Request,
    w http.ResponseWriter,
    u useraggregate.User,
    service *services.EntityNameService,
    logger *logrus.Entry,
) {
    // Implementation
}
```

**Service registration in module**:

```go
func (m *Module) ConfigureServices(services di.ServiceCollection) {
    // Register repository
    services.AddScoped(
        reflect.TypeOf((*domain.EntityNameRepository)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            return persistence.NewEntityNameRepository(), nil
        },
    )

    // Register service
    services.AddScoped(
        reflect.TypeOf((*services.EntityNameService)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            repo := sp.GetService(reflect.TypeOf((*domain.EntityNameRepository)(nil)).Elem()).(domain.EntityNameRepository)
            return services.NewEntityNameService(repo), nil
        },
    )
}
```

## Organization vs Tenant

### Critical Distinction

Many operations require **organization ID**, not just tenant ID:

```go
// Getting organization ID from context
orgID, err := composables.GetOrgID(ctx)
if err != nil {
    return serrors.E(op, err)
}

// Creating entity with organization context
entity := domain.New(
    composables.UseTenantID(ctx),
    orgID,  // Organization ID required
    input.Name,
)
```

### When to Use Organization ID

- Creating business entities (orders, clients, products)
- Filtering data by organization
- Organization-specific business rules
- Multi-organization features

## Best Practices

### Domain Layer

- [ ] Aggregates as interfaces, not structs
- [ ] Functional options for optional fields
- [ ] Private struct, public interface
- [ ] Immutable setters (return new instance)
- [ ] Business rules inside entities
- [ ] Repository interfaces with no implementation
- [ ] Domain has no external dependencies

### Service Layer

- [ ] DI with repository interfaces (not implementations)
- [ ] Business logic and validation implemented
- [ ] Permission checks via `sdkcomposables.CanUser()`
- [ ] Errors wrapped: `serrors.E(op, err)`
- [ ] Transaction management for multi-step operations
- [ ] Services use repository interfaces

### Error Handling

- [ ] Always use `serrors.Op` for operation tracking
- [ ] Always wrap errors with `serrors.E(op, err)`
- [ ] Use appropriate error kinds (KindValidation, KindNotFound, etc.)
- [ ] Provide context in error messages

### Testing

Services and domain logic should be tested with ITF framework (see `testing.md`):

```go
func TestServiceName_Method(t *testing.T) {
    t.Parallel()
    f := setupTest(t, permissions.RequiredPermission)
    svc := getServiceFromEnv[services.ServiceName](f)
    res, err := svc.Method(f.Ctx, "valid")
    require.NoError(t, err)
    require.NotNil(t, res)
}
```

## Common Pitfalls

### Don't

- Use concrete structs for aggregates (use interfaces)
- Put database logic in services (use repositories)
- Put business logic in controllers (use services)
- Forget to check permissions
- Ignore organization vs tenant distinction
- Skip error wrapping with `serrors.E`
- Use direct struct mutation (use immutable setters)

### Do

- Follow DDD principles strictly
- Use functional options for flexibility
- Return new instances from setters (immutability)
- Check permissions in services
- Wrap all errors with operation context
- Test services with ITF framework
- Keep domain layer pure (no external dependencies)
