---
layout: default
title: Core Module
nav_order: 2
has_children: true
description: "Foundation module providing authentication, user management, roles, groups, and system administration"
---

# Core Module

The **Core Module** is the foundational component of IOTA SDK, providing essential platform functionality for all other modules. It handles user authentication, authorization, session management, and the core administration features needed to operate a multi-tenant business platform.

## Module Overview

The Core Module manages the critical infrastructure that every other module depends on:

- **Authentication & Sessions**: User login/logout, session tokens, IP tracking
- **User Management**: User profiles, activation, password management
- **Roles & Permissions**: RBAC implementation with granular permission control
- **Groups**: User organization and bulk permission management
- **Settings**: System-wide and tenant-specific configuration
- **Dashboard**: Real-time business metrics and system overview
- **File Management**: Upload handling, image processing

## Architecture

```
modules/core/
├── domain/
│   ├── aggregates/
│   │   ├── user/              # User entity with authentication
│   │   ├── role/              # Role definitions and permissions
│   │   ├── group/             # User groups for organization
│   │   └── project/           # Project management
│   ├── entities/
│   │   ├── permission/        # Permission definitions
│   │   ├── session/           # Session tracking
│   │   └── upload/            # File upload tracking
│   └── value_objects/
│       ├── internet/          # Email, phone value objects
│       └── tax/               # TIN, PIN for taxation
├── infrastructure/
│   ├── persistence/
│   │   ├── schema/            # Database migrations
│   │   └── repositories/      # Data access layer
│   └── query/                 # Query repositories
├── services/                  # Business logic layer
├── presentation/
│   ├── controllers/           # HTTP request handlers
│   ├── templates/             # Templ-based UI templates
│   └── locales/              # I18n translation files
└── permissions/               # Permission constants
```

## Integration Points

| Module | Integration | Purpose |
|--------|-------------|---------|
| **All Modules** | User/Tenant Context | Authorization, tenant isolation |
| **Finance** | User Permissions | Access control for financial operations |
| **Warehouse** | User Permissions | Access control for inventory |
| **CRM** | User Permissions | Access control for customer relations |
| **HRM** | User/Group Management | Employee organizational structure |
| **Projects** | User/Group Management | Project team assignment |

## Key Entities

### Users
- System accounts with email authentication
- Profile management (name, contact info, avatar)
- Role assignment and direct permissions
- Session tracking with login history

### Roles
- System-defined and custom roles
- Permission bundling for efficient access control
- Tenant-scoped and system-scoped roles
- Inheritance patterns for role hierarchy

### Groups
- User organization and management
- Bulk permission assignment
- Team/department representation
- Both system and custom groups

### Sessions
- Secure session management with JWT-style tokens
- IP and user agent tracking for security
- Configurable expiration
- Real-time session monitoring

### Permissions
- Granular resource-action-modifier permissions
- Three-layer check: Role > Group > User
- Resource-based (users, roles, groups, etc.)
- Action-based (create, read, update, delete)

## Quick Links

- **Documentation Map**: See [Business Requirements](business.md) for domain context
- **Technical Details**: See [Technical Architecture](technical.md) for implementation patterns
- **Database Schema**: See [Data Model](data-model.md) for entity relationships
- **User Workflows**: See [User Experience](ux.md) for interface flows

## Common Operations

### User Authentication
```go
// Service handles login verification
authService.Authenticate(ctx, email, password)
```

### Permission Checking
```go
// Check if user has permission
composables.CanUser(ctx, permissions.UserCreate)
```

### User Management
```go
// Create user with roles
userService.Create(ctx, userData)
userService.AssignRoles(ctx, userID, roleIDs)
```

### Session Management
```go
// Manage user sessions
sessionService.Create(ctx, userID)
sessionService.Revoke(ctx, token)
```

## Module Statistics

- **Tables**: 15+ (users, roles, groups, sessions, permissions, uploads, etc.)
- **Services**: 12+ (user, role, group, session, auth, permission services)
- **Controllers**: 8+ (users, roles, groups, settings, login, account, dashboard)
- **Permissions**: 50+ granular permissions across all resources
- **Repositories**: 10+ with full CRUD and advanced query support

## Highlights

- **Multi-tenant Isolation**: Complete data isolation per tenant at the database level
- **RBAC System**: Flexible role-based access control with permission inheritance
- **Session Management**: Secure session handling with configurable expiration
- **Event Publishing**: Domain events for user and role changes
- **Validation**: Comprehensive validation on user creation and updates
- **File Uploads**: Integrated file storage for user avatars and documents
