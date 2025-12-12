---
layout: default
title: Technical Architecture
parent: Core Module
nav_order: 2
description: "Technical implementation details, layer separation, and API contracts"
---

# Technical Architecture: Core Module

## Layer Separation

The Core Module follows Domain-Driven Design with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────┐
│              PRESENTATION LAYER                          │
│  Controllers → ViewModels → Templates                    │
│  /users, /roles, /groups, /settings, /login              │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│               SERVICE LAYER                              │
│  UserService, RoleService, GroupService, SessionService  │
│  - Business logic                                         │
│  - Permission validation via composables.CanUser()       │
│  - Transaction management                                │
│  - Event publishing                                      │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│              DOMAIN LAYER                                │
│  User, Role, Group, Permission Aggregates               │
│  - Business rules                                        │
│  - Value objects (Email, Phone)                         │
│  - Repository interfaces                                │
│  - Domain events                                        │
└──────────────────┬──────────────────────────────────────┘
                   │
┌──────────────────▼──────────────────────────────────────┐
│          INFRASTRUCTURE LAYER                            │
│  PostgreSQL Repositories, Query Repositories             │
│  - Database access                                       │
│  - Query optimization                                    │
│  - Transaction handling via composables.UseTx()         │
│  - Tenant isolation via composables.UseTenantID()       │
└──────────────────┬──────────────────────────────────────┘
                   │
              PostgreSQL DB
```

## Directory Structure

```
modules/core/
├── domain/
│   ├── aggregates/
│   │   ├── user/
│   │   │   ├── user.go                 # User aggregate interface
│   │   │   ├── user_repository.go      # Repository interface
│   │   │   ├── value_objects.go        # Email, phone values
│   │   │   ├── user_validator.go       # Validation rules
│   │   │   └── user_events.go          # Domain events
│   │   ├── role/
│   │   │   ├── role.go
│   │   │   ├── role_repository.go
│   │   │   └── role_events.go
│   │   ├── group/
│   │   │   ├── group.go
│   │   │   ├── group_repository.go
│   │   │   └── group_events.go
│   │   └── project/
│   ├── entities/
│   │   ├── permission/
│   │   │   └── permission.go           # Permission entity
│   │   ├── session/
│   │   │   └── session.go              # Session entity
│   │   └── upload/
│   │       └── upload.go               # Upload tracking
│   └── value_objects/
│       ├── internet/
│       │   └── email.go, phone.go
│       └── tax/
│           └── tin.go, pin.go
│
├── infrastructure/
│   ├── persistence/
│   │   ├── schema/
│   │   │   └── core-schema.sql         # Migrations
│   │   ├── user_repository.go          # User repository impl
│   │   ├── role_repository.go          # Role repository impl
│   │   ├── group_repository.go         # Group repository impl
│   │   ├── session_repository.go       # Session repository impl
│   │   └── models/models.go            # Database models
│   └── query/
│       ├── user_query_repository.go    # User queries
│       ├── role_query_repository.go    # Role queries
│       └── group_query_repository.go   # Group queries
│
├── services/
│   ├── user_service.go                 # User business logic
│   ├── role_service.go                 # Role business logic
│   ├── group_service.go                # Group business logic
│   ├── auth_service.go                 # Authentication
│   ├── session_service.go              # Session management
│   ├── permission_service.go           # Permission management
│   ├── tenant_service.go               # Tenant management
│   ├── currency_service.go             # Currency data
│   ├── upload_service.go               # File handling
│   └── excel_export_service.go         # Data export
│
├── presentation/
│   ├── controllers/
│   │   ├── user_controller.go
│   │   ├── role_controller.go
│   │   ├── group_controller.go
│   │   ├── login_controller.go
│   │   ├── account_controller.go
│   │   ├── dashboard_controller.go
│   │   ├── upload_controller.go
│   │   └── settings_controller.go
│   ├── templates/pages/
│   │   ├── users/
│   │   ├── roles/
│   │   ├── groups/
│   │   ├── dashboard/
│   │   └── account/
│   ├── viewmodels/
│   │   └── (transformation logic)
│   └── locales/
│       ├── en.json
│       ├── ru.json
│       └── uz.json
│
└── permissions/
    └── constants.go                    # Permission definitions
```

## Key Implementation Patterns

### User Aggregate

```go
// Domain-driven user aggregate
type User interface {
    ID() uint
    FirstName() string
    LastName() string
    Email() string
    HasPassword() bool
    IsActive() bool
    Roles() []Role
    Permissions() []Permission

    SetEmail(email string) User
    SetFirstName(name string) User
    SetPassword(password string) User
    // ... more setters (immutable pattern)
}

// Private struct implementation
type user struct {
    id       uint
    email    string
    password string
    roles    []Role
    // ...
}

// Functional options for construction
func New(opts ...Option) User {
    u := &user{...}
    for _, opt := range opts {
        opt(u)
    }
    return u
}

func WithEmail(email string) Option {
    return func(u *user) {
        u.email = email
    }
}
```

### Service Business Logic

```go
// UserService with dependency injection
type UserService struct {
    repo      user.Repository
    validator user.Validator
    publisher eventbus.EventBus
}

// Permission checking
func (s *UserService) GetByID(ctx context.Context, id uint) (user.User, error) {
    // Check permission
    if err := composables.CanUser(ctx, permissions.UserRead); err != nil {
        return nil, err
    }

    // Get from repository
    return s.repo.GetByID(ctx, id)
}

// Transactional operations
func (s *UserService) Create(ctx context.Context, data user.User) (user.User, error) {
    if err := composables.CanUser(ctx, permissions.UserCreate); err != nil {
        return nil, err
    }

    var created user.User
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        if err := s.validator.ValidateCreate(txCtx, data); err != nil {
            return err
        }

        var err error
        created, err = s.repo.Create(txCtx, data)
        return err
    })

    if err == nil {
        s.publisher.Publish(user.NewCreatedEvent(ctx, created))
    }
    return created, err
}
```

### Repository Implementation

```go
// User repository implementation
func (r *userRepository) GetByID(ctx context.Context, id uint) (user.User, error) {
    const op = "user.Repository.GetByID"

    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    row := composables.UseTx(ctx, r.db).QueryRow(
        `SELECT id, email, first_name, last_name, password
         FROM users
         WHERE id = $1 AND tenant_id = $2`,
        id, tenantID,
    )

    var u user.User
    if err := scanUser(row, &u); err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, serrors.E(op, serrors.KindNotFound)
        }
        return nil, serrors.E(op, err)
    }
    return u, nil
}
```

### Permission Checking Pattern

```go
// Composables for permission management
// File: pkg/composables/permission.go

// Check if user has permission
func CanUser(ctx context.Context, permission string) error {
    user := ctx.Value("user").(User)
    perms := ctx.Value("permissions").([]string)

    for _, p := range perms {
        if p == permission {
            return nil
        }
    }
    return ErrPermissionDenied
}
```

## Permission Matrix

### User Permissions

| Permission | Resource | Action | Modifier | Description |
|------------|----------|--------|----------|-------------|
| `users:create:all` | Users | Create | All | Create any user |
| `users:read:all` | Users | Read | All | View all users |
| `users:read:own` | Users | Read | Own | View own profile |
| `users:update:all` | Users | Update | All | Update any user |
| `users:update:own` | Users | Update | Own | Update own profile |
| `users:delete:all` | Users | Delete | All | Delete any user |

### Role Permissions

| Permission | Resource | Action | Modifier | Description |
|------------|----------|--------|----------|-------------|
| `roles:create:all` | Roles | Create | All | Create roles |
| `roles:read:all` | Roles | Read | All | View all roles |
| `roles:update:all` | Roles | Update | All | Update roles |
| `roles:delete:all` | Roles | Delete | All | Delete roles |

### Group Permissions

| Permission | Resource | Action | Modifier | Description |
|------------|----------|--------|----------|-------------|
| `groups:create:all` | Groups | Create | All | Create groups |
| `groups:read:all` | Groups | Read | All | View groups |
| `groups:update:all` | Groups | Update | All | Update groups |
| `groups:delete:all` | Groups | Delete | All | Delete groups |

## API Contracts

### User Endpoints

```
GET    /users                    # List users (paginated)
GET    /users/:id               # Get user details
POST   /users                   # Create user
PUT    /users/:id               # Update user
DELETE /users/:id               # Delete user
POST   /users/:id/roles         # Assign roles
DELETE /users/:id/roles/:roleId # Unassign role
```

### Role Endpoints

```
GET    /roles                   # List roles
GET    /roles/:id              # Get role details
POST   /roles                  # Create role
PUT    /roles/:id              # Update role
DELETE /roles/:id              # Delete role
GET    /roles/:id/permissions  # Get role permissions
POST   /roles/:id/permissions  # Add permission to role
```

### Group Endpoints

```
GET    /groups                 # List groups
GET    /groups/:id            # Get group details
POST   /groups                # Create group
PUT    /groups/:id            # Update group
DELETE /groups/:id            # Delete group
POST   /groups/:id/users      # Add user to group
DELETE /groups/:id/users/:uid # Remove user from group
```

### Authentication Endpoints

```
POST   /login                  # Authenticate user
POST   /logout                 # Invalidate session
GET    /account               # Get user account
PUT    /account               # Update account
```

## Error Handling

All services use `serrors` package for error handling:

```go
const op serrors.Op = "UserService.GetByID"

// Wrap errors with context
if err != nil {
    return nil, serrors.E(op, err)
}

// Use error kinds
if notFound {
    return nil, serrors.E(op, serrors.KindNotFound)
}

// Provide context
if invalid {
    return nil, serrors.E(op, serrors.KindValidation, "email is required")
}
```

## Multi-tenancy Implementation

### Tenant Isolation

1. **Database Level**
   - All tables include `tenant_id` foreign key
   - WHERE clauses always filter by `tenant_id`

2. **Context Level**
   ```go
   // Get tenant from context
   tenantID := composables.UseTenantID(ctx)

   // All queries filtered by tenant
   db.QueryRow(
       "SELECT * FROM users WHERE id = $1 AND tenant_id = $2",
       userID, tenantID,
   )
   ```

3. **Service Level**
   - Permission checks include tenant context
   - User queries scoped to tenant
   - Session tokens tied to tenant

## Performance Considerations

### Database Indexes

```sql
-- User queries
CREATE INDEX users_tenant_id_idx ON users(tenant_id);
CREATE INDEX users_first_name_idx ON users(first_name);
CREATE INDEX users_last_name_idx ON users(last_name);

-- Session lookups
CREATE INDEX sessions_tenant_id_idx ON sessions(tenant_id);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

-- Permission lookups
CREATE INDEX role_permissions_role_id_idx ON role_permissions(role_id);
CREATE INDEX role_permissions_permission_id_idx ON role_permissions(permission_id);
```

### Query Optimization

1. **Batch loading**: Load user roles/permissions with single query
2. **Caching**: Permission checks cached for session duration
3. **Pagination**: Large datasets paginated with limit/offset
4. **Connection pooling**: Database connection reuse via composables.UseTx()
