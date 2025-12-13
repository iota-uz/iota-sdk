---
layout: default
title: Data Model
parent: SuperAdmin
nav_order: 3
description: "SuperAdmin Module Data Model"
---

# Data Model

## Overview

The SuperAdmin module works with the existing data model from the core module, extending it with super admin functionality. The key distinction is between regular users (scoped to tenants) and super admin users (platform-wide access).

## User Entity

Users are stored in the `users` table in the core module. The SuperAdmin module extends this with the `type` field for role management.

### User Table Schema

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    password_hash VARCHAR(255),
    avatar_url VARCHAR(500),
    type VARCHAR(50) NOT NULL DEFAULT 'regular',  -- 'regular' or 'superadmin'
    tenant_id UUID,  -- NULL for superadmin users
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Indexes for performance
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_type ON users(type);
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);
```

### User Types

```go
const (
    TypeRegular    = "regular"    // Normal user scoped to single tenant
    TypeSuperAdmin = "superadmin" // Platform administrator
)
```

## User Aggregate (Domain Model)

The User aggregate provides interface-based access to user data:

```go
type User interface {
    ID() uint
    Email() string
    FirstName() string
    LastName() string
    Type() string          // "regular" or "superadmin"
    TenantID() uuid.UUID
    AvatarURL() string
    DeletedAt() *time.Time
    CreatedAt() time.Time
    UpdatedAt() time.Time

    // Methods for updates (return new instance - immutable)
    SetFirstName(name string) User
    SetLastName(name string) User
    SetAvatarURL(url string) User
    SetEmail(email string) User
    SoftDelete() User
}
```

## Super Admin User Example

Creating a super admin user requires direct database access:

```sql
-- Create super admin user
INSERT INTO users (
    id,
    email,
    first_name,
    last_name,
    type,
    tenant_id,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'admin@platform.com',
    'Super',
    'Admin',
    'superadmin',
    NULL,  -- No tenant affiliation
    NOW(),
    NOW()
);
```

## Session Entity

Sessions are stored in the `sessions` table in the core module.

### Session Table Schema

```sql
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id BIGINT NOT NULL,
    tenant_id UUID,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

-- Indexes
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### Session Properties

- **ID**: UUID for session cookie
- **User ID**: Links to user record
- **Tenant ID**: User's tenant (NULL for super admin)
- **Expires At**: Configurable expiration (default 30 days)
- **Created At**: Session creation timestamp

## Tenant Entity

Tenants are managed by the superadmin module but stored in the core module's `tenants` table.

### Tenant Table Schema

```sql
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    domain VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active',  -- active, trial, suspended, deleted
    plan VARCHAR(50) DEFAULT 'free',
    max_users INT,
    storage_gb DECIMAL(10, 2),
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_tenants_name ON tenants(name);
CREATE INDEX idx_tenants_status ON tenants(status);
CREATE INDEX idx_tenants_deleted_at ON tenants(deleted_at);
```

### Tenant Statuses

```go
const (
    StatusActive    = "active"     // Currently active tenant
    StatusTrial     = "trial"      // Trial period
    StatusSuspended = "suspended"  // Suspended for non-payment
    StatusDeleted   = "deleted"    // Soft deleted
)
```

## Analytics Entity

Analytics data is computed on-demand from aggregated queries and not persisted.

### TenantInfo (Computed)

```go
type TenantInfo interface {
    ID() uuid.UUID
    Name() string
    Status() string
    UserCount() int
    StorageUsed() int64
    CreatedAt() time.Time
    UpdatedAt() time.Time
    Plan() string
}
```

### DashboardMetrics (Computed)

```go
type DashboardMetrics struct {
    TenantCount             int
    UserCount               int
    DAU                     int  // Daily Active Users
    WAU                     int  // Weekly Active Users
    MAU                     int  // Monthly Active Users
    SessionCount            int
    UserSignupsTimeSeries   []TimeSeries
    TenantSignupsTimeSeries []TimeSeries
}
```

### TimeSeries (Computed)

```go
type TimeSeries struct {
    Date  time.Time
    Count int
}
```

## Relationships

```
┌──────────────┐
│   Tenants    │
│              │
│ id (PK)      │
│ name         │
│ status       │
│ plan         │
│ deleted_at   │
└───────┬──────┘
        │
        │ 1:N (tenant_id)
        │
┌───────▼──────┐
│    Users     │
│              │
│ id (PK)      │
│ email        │
│ type         │ ◄─── 'superadmin' = no tenant
│ tenant_id(FK)│
│ deleted_at   │
└───────┬──────┘
        │
        │ 1:N (user_id)
        │
┌───────▼──────────┐
│    Sessions      │
│                  │
│ id (PK)          │
│ user_id (FK)     │
│ tenant_id (FK)   │
│ expires_at       │
└──────────────────┘
```

## Access Patterns

### Regular User

```
User
├── Type: "regular"
├── TenantID: <uuid>      ◄─── Scoped to specific tenant
├── Email: user@company.com
└── Session
    ├── UserID: <id>
    └── TenantID: <uuid>  ◄─── Same tenant
```

### Super Admin User

```
User
├── Type: "superadmin"
├── TenantID: NULL        ◄─── No tenant affiliation
├── Email: admin@platform.com
└── Session
    ├── UserID: <id>
    └── TenantID: NULL    ◄─── Access to all tenants
```

## Key Differences from Regular Tenants

### Tenant Isolation

**Regular Users**:
- Queries filtered by `WHERE tenant_id = :tenant_id`
- Cannot see data from other tenants
- Data isolation enforced at query level

**Super Admin Users**:
- Queries without tenant filter
- Can see all tenant data
- Cross-tenant aggregations available

### User Management

**Regular Users**:
- Can manage users within their tenant
- Cannot create super admin accounts
- Limited to their organization

**Super Admin Users**:
- Can manage users across all tenants
- Can create new super admin accounts
- Full platform visibility

### Session Context

**Regular Users**:
```go
tenantID := composables.UseTenantID(ctx)  // Returns user's tenant
```

**Super Admin Users**:
```go
tenantID := composables.UseTenantID(ctx)  // Returns NULL
// Access all tenants without restriction
```

## Query Patterns

### Finding Super Admin Users

```go
// Query users with super admin type
SELECT * FROM users WHERE type = 'superadmin'
```

### Getting Tenant Users

```go
// Query users within a specific tenant
SELECT * FROM users
WHERE tenant_id = $1
  AND type = 'regular'
  AND deleted_at IS NULL
```

### Cross-Tenant Analytics

```go
// Count users per tenant
SELECT tenant_id, COUNT(*) as user_count
FROM users
WHERE deleted_at IS NULL
GROUP BY tenant_id
```

### Session Validation

```go
// Validate session and get user
SELECT u.*, s.tenant_id
FROM users u
JOIN sessions s ON s.user_id = u.id
WHERE s.id = $1
  AND s.expires_at > NOW()
  AND u.deleted_at IS NULL
```

## Immutability Pattern

The User aggregate follows immutable patterns - getters return copies, not references:

```go
// Create user
user := domain.NewUser("email@example.com")

// Update creates new instance
user = user.SetFirstName("John")
user = user.SetLastName("Doe")

// Persist changes
repository.Save(ctx, user)
```

## Audit Trail

All operations should be logged:

```go
// Log super admin action
logger.WithFields(logrus.Fields{
    "super_admin_id": superAdminID,
    "action": "create_tenant",
    "tenant_id": tenantID,
    "timestamp": time.Now(),
}).Info("Super admin created tenant")
```

## Constraints & Validations

### Email Validation

- Must be valid email format
- Must be unique across platform
- Case-insensitive uniqueness

### User Type Constraints

- Super admin users: `tenant_id` must be NULL
- Regular users: `tenant_id` must reference valid tenant

### Session Constraints

- Session expires based on `SESSION_DURATION` env var
- Expired sessions removed automatically
- Invalid tokens return 401 Unauthorized

### Tenant Constraints

- Tenant name required and non-empty
- Status must be valid enum value
- Max users must be positive integer
- Can only be hard-deleted if soft-deleted first
