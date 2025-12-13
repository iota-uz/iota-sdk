---
layout: default
title: SuperAdmin
nav_order: 9
has_children: true
description: "SuperAdmin Module - Platform-wide administration and tenant management"
---

# SuperAdmin Module

The SuperAdmin module provides platform-wide administration capabilities for managing tenants, users, analytics, and system operations. It runs as a separate deployment isolated from the main application, ensuring that only authorized super administrators can perform critical operations.

## Overview

The SuperAdmin module enables:

- **Tenant Management**: Create, view, update, and delete tenants across the platform
- **User Management**: View and manage users across all tenants
- **Analytics**: Platform-wide metrics and tenant usage statistics
- **Dashboard**: Comprehensive overview of system health and activity
- **Isolated Deployment**: Separate server for enhanced security and performance

## Document Map

This SuperAdmin documentation includes:

1. **[Business Requirements](./business.md)** - Problem statement, use cases, and business rules
2. **[Technical Architecture](./technical.md)** - Module structure, middleware stack, authentication flow
3. **[Data Model](./data-model.md)** - User types, tenant structure, session management
4. **[Deployment](./deployment.md)** - Environment configuration, Docker, Railway, Kubernetes, and security

## Key Characteristics

### Isolated Deployment
- Runs independently from the main application server
- Only loads `core` and `superadmin` modules for optimal performance and security
- Shared database and environment configuration with main application
- Different subdomain for network isolation (e.g., `admin.yourdomain.com`)

### Global Authentication
- All routes protected by super admin middleware
- Super admin users have no tenant affiliation
- Session-based authentication with configurable expiration
- Access to all tenant data without restrictions

### Platform Operations
- Cross-tenant visibility and analytics
- System-wide settings and monitoring
- Tenant lifecycle management
- User management across tenants

## Integration Points

The SuperAdmin module integrates with:

- **Core Module**: User authentication, sessions, and user data
- **Database**: Shared PostgreSQL instance with main application
- **Middleware Stack**: Global authorization and super admin checks
- **Analytics Service**: Dashboard metrics and tenant usage statistics

## Getting Started

- Review [Business Requirements](./business.md) to understand the domain
- Check [Technical Architecture](./technical.md) for implementation details
- See [Deployment](./deployment.md) for production setup
- Refer to [original SUPERADMIN.md](../SUPERADMIN.md) for additional deployment details

## Security Considerations

- Super admin accounts created only via direct database access
- HTTP-only cookies and secure session management
- Parameterized SQL queries (no raw concatenation)
- Comprehensive audit logging of all platform operations
- Regular security audits and dependency updates

## Related Documentation

- [IOTA SDK Architecture](../index.md) - Overall system architecture
- [Main SUPERADMIN.md](../SUPERADMIN.md) - Original detailed documentation
- [Core Module](../core/index.md) - Authentication and user management
