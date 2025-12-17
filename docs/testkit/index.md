---
layout: default
title: Testkit
nav_order: 13
has_children: true
description: "Testing utilities and integration test framework for IOTA SDK"
---

# Testkit

The IOTA SDK Testkit module provides comprehensive testing utilities and the Integration Test Framework (ITF) for building and executing tests across all layers of your application.

## Overview

The Testkit module (`/testkit`) provides:

- **Integration Test Framework (ITF)**: Full application setup for testing with database, services, and controllers
- **Test Data Population**: Populate test databases with realistic data
- **Test Endpoints**: REST endpoints for resetting and seeding test data
- **Test Fixtures**: Reusable test data and setup helpers
- **Service Testing**: Direct service testing with full context
- **Controller Testing**: HTTP request/response testing with fluent API

## Key Features

### Full Test Environment Setup
Initialize complete test environments with:
- PostgreSQL database connections
- Dependency injection
- Service and controller registration
- Tenant and organization context

### Test Data Management
- Populate databases with test fixtures
- Reset database between tests
- Create test entities programmatically
- Manage relationships and foreign keys

### Test Endpoints (Development Only)
RESTful endpoints for test management:
- `/test/reset` - Reset database to clean state
- `/test/populate` - Populate with test data
- `/test/seed` - Seed specific test scenarios

### Fluent Testing API
Chain assertions for readable tests:

```go
suite := controllertest.New(t, module)
suite.GET("/users").Expect(t).Status(200).Contains("Users")
```

## Module Structure

```
modules/testkit/
├── domain/
│   └── schemas/
│       └── populate_schema.go      # Test data population schemas
├── services/
│   ├── populate_service.go         # Data population logic
│   ├── reset_service.go            # Database reset functionality
│   └── test_data_service.go        # Test data management
├── presentation/
│   └── controllers/
│       └── test_endpoints_controller.go  # REST endpoints
└── module.go                       # Module registration
```

## Quick Start

### Setting Up Tests

```go
import (
    "testing"
    "github.com/iota-uz/iota-sdk/modules/testkit/itf"
)

func TestUserService(t *testing.T) {
    t.Parallel()

    // Setup test environment
    env := itf.Setup(t,
        itf.WithModules(myModule),
        itf.WithPermissions(user.AdminRole),
    )
    defer env.Close()

    // Get service from environment
    userService := itf.GetService[*services.UserService](env)

    // Test code...
}
```

### Testing Controllers

```go
func TestUserController(t *testing.T) {
    t.Parallel()

    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // Test GET
    suite.GET("/users").
        Expect(t).
        Status(200).
        Contains("Users List")

    // Test POST
    suite.POST("/users").
        JSON(map[string]string{
            "firstName": "John",
            "lastName": "Doe",
            "email": "john@example.com",
        }).
        Expect(t).
        Status(201)
}
```

### Testing Services

```go
func TestCreateUser(t *testing.T) {
    t.Parallel()

    env := itf.Setup(t, itf.WithModules(coreModule))
    defer env.Close()

    userService := itf.GetService[*services.UserService](env)
    ctx := env.Context()

    user, err := userService.Create(ctx, &services.CreateUserDTO{
        FirstName: "Jane",
        LastName: "Smith",
        Email: "jane@example.com",
    })

    if err != nil {
        t.Fatalf("Failed to create user: %v", err)
    }

    if user.Email() != "jane@example.com" {
        t.Errorf("Expected email jane@example.com, got %s", user.Email())
    }
}
```

## Test Database Configuration

Tests use a separate database instance to avoid interfering with development or production:

```bash
# .env.test
DATABASE_URL=postgres://user:pass@localhost/iota_erp_test
DATABASE_POOL_SIZE=5
ENABLE_TEST_ENDPOINTS=true
```

## Next Steps

- Read the [Technical Guide](./technical.md) for detailed implementation patterns
- Learn about [Controller Testing](../advanced/controller-testing.md)
- Check out the [Integration Test Framework Guide](../testing.md)

---

For more information, visit the [IOTA SDK GitHub repository](https://github.com/iota-uz/iota-sdk).
