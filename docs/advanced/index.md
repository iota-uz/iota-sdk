---
layout: default
title: Advanced
nav_order: 14
has_children: true
description: "Advanced features and capabilities in IOTA SDK"
---

# Advanced Features

This section covers advanced features and capabilities of the IOTA SDK, including JavaScript runtime integration, CRUD automation, Excel export, rate limiting, and advanced controller testing.

## Overview

The Advanced section includes documentation for:

1. **[JavaScript Runtime](./js-runtime.md)** - Execute user-defined scripts with Goja runtime
2. **[CRUD Package](./crud-package.md)** - Generic CRUD operations with schema-driven development
3. **[Excel Exporter](./excel-exporter.md)** - Export data to Excel with formatting options
4. **[Rate Limiting](./rate-limiting.md)** - Request throttling and DDoS protection
5. **[Controller Testing](./controller-testing.md)** - Fluent API for testing HTTP controllers

## Feature Matrix

| Feature | Status | Use Case |
|---------|--------|----------|
| JavaScript Runtime | Advanced | Custom business logic, scheduled jobs, dynamic endpoints |
| CRUD Package | Production | Rapid entity development with schema-driven approach |
| Excel Exporter | Production | Data export, reporting, analytics |
| Rate Limiting | Production | API protection, DDoS defense, resource management |
| Controller Testing | Production | HTTP endpoint testing, integration testing |

## Quick Reference

### JavaScript Runtime
Execute custom scripts with access to database, services, and HTTP:

```javascript
// Scheduled job example
async function main() {
    const clients = await services.clients.list({ limit: 100 });
    for (const client of clients) {
        await sendNotification(client);
    }
}
```

### CRUD Package
Define entities with schema-driven approach:

```go
schema := crud.NewSchema(
    "users",
    []crud.Field{
        crud.NewStringField("firstName"),
        crud.NewStringField("email"),
    },
)
```

### Excel Exporter
Export query results to Excel:

```go
upload, _ := excelService.ExportFromQuery(
    ctx,
    "SELECT * FROM users WHERE active = true",
    "active_users",
    true,
)
```

### Rate Limiting
Protect endpoints with rate limits:

```go
router.Use(middleware.IPRateLimitPeriod(100, time.Minute))
```

### Controller Testing
Test HTTP endpoints:

```go
suite.POST("/users").
    JSON(data).
    Expect(t).
    Status(201)
```

## When to Use Each Feature

### JavaScript Runtime
Use when:
- You need user-customizable business logic
- Building scheduled automation (cron jobs)
- Creating dynamic HTTP endpoints
- Implementing event handlers
- Enabling advanced filtering/transformation

### CRUD Package
Use when:
- Building new entities quickly
- Need standard CRUD operations
- Want schema-driven development
- Building admin interfaces
- Reducing boilerplate code

### Excel Exporter
Use when:
- Exporting data for reporting
- Users need Excel integration
- Building analytics dashboards
- Creating audit trails
- Data migration/backup

### Rate Limiting
Use when:
- Protecting public APIs
- Preventing brute force attacks
- Managing resource usage
- Implementing DDoS protection
- Implementing user quotas

### Controller Testing
Use when:
- Testing HTTP endpoints
- Validating form handling
- Testing authentication
- Testing HTMX interactions
- Integration testing

## Integration Patterns

### Combining Features
These features work together seamlessly:

```go
// Example: Schema-driven CRUD with export
schema := crud.NewSchema("products", fields)
service := schema.Service()

// Enable rate limiting
router.Use(middleware.UserRateLimitPeriod(1000, time.Hour))

// Add Excel export
controller.excelExport = excelService.ExportFromQuery

// Test the controller
suite.GET("/products/export").Expect(t).Status(200)
```

## Performance Considerations

| Feature | Overhead | Scalability |
|---------|----------|------------|
| JavaScript Runtime | 10-50ms per execution | Scales with VM pool |
| CRUD Package | Minimal | Depends on schema complexity |
| Excel Exporter | Proportional to data size | Limited by memory |
| Rate Limiting | <1ms per request | Scales with storage backend |
| Controller Testing | Test-only overhead | Linear with test count |

## Security Considerations

- **JavaScript Runtime**: Sandboxed execution with resource limits
- **CRUD Package**: Automatic validation and sanitization
- **Excel Exporter**: Respects row limits and timeouts
- **Rate Limiting**: Prevents DoS and brute force attacks
- **Controller Testing**: Only available in test environments

## Next Steps

Choose the feature you want to learn more about:

- **[JavaScript Runtime](./js-runtime.md)** - For custom business logic
- **[CRUD Package](./crud-package.md)** - For rapid development
- **[Excel Exporter](./excel-exporter.md)** - For data export
- **[Rate Limiting](./rate-limiting.md)** - For API protection
- **[Controller Testing](./controller-testing.md)** - For testing HTTP endpoints

---

For more information, visit the [IOTA SDK GitHub repository](https://github.com/iota-uz/iota-sdk).
