---
layout: default
title: Technical Guide
parent: Testkit
nav_order: 1
description: "Technical implementation of testing in IOTA SDK"
---

# Testkit Technical Guide

This guide covers the implementation details and advanced usage of the IOTA SDK Testkit module.

## Integration Test Framework (ITF)

### Environment Setup

```go
import "github.com/iota-uz/iota-sdk/modules/testkit/itf"

// Basic setup
env := itf.Setup(t, itf.WithModules(module1, module2))
defer env.Close()

// With custom options
env := itf.Setup(t,
    itf.WithModules(userModule, financeModule),
    itf.WithPermissions(role.AdminRole),
    itf.WithDatabase(customDbConfig),
    itf.WithConnectionPool(10),
)
```

### Accessing Test Resources

```go
// Get service by type
userService := itf.GetService[*services.UserService](env)

// Get repository by type
userRepo := itf.GetRepository[*repositories.UserRepository](env)

// Get database
db := env.DB()

// Get context
ctx := env.Context()

// Get application
app := env.App()
```

## Test Data Population

### PopulateService Usage

```go
populateService := itf.GetService[*services.PopulateService](env)

result, err := populateService.Execute(ctx, &schemas.PopulateRequest{
    Tenant: &schemas.TenantSpec{
        ID: "tenant-uuid",
        Name: "Test Tenant",
    },
    Data: &schemas.DataSpec{
        Users: []schemas.UserSpec{
            {
                FirstName: "John",
                LastName: "Doe",
                Email: "john@example.com",
            },
        },
        Invoices: []schemas.InvoiceSpec{
            {
                Number: "INV-001",
                Amount: 1000.00,
                Status: "draft",
            },
        },
    },
    Options: &schemas.PopulateOptions{
        ReturnIds: true,
    },
})

// result contains created entity IDs
userID := result["users"][0].(string)
```

### Creating Test Data Programmatically

```go
func setupTestUser(t *testing.T, service *services.UserService) string {
    env := itf.Setup(t)
    ctx := env.Context()

    user, err := service.Create(ctx, &services.CreateUserDTO{
        FirstName: "Test",
        LastName: "User",
        Email: "test@example.com",
        Password: "secure-password",
    })

    if err != nil {
        t.Fatalf("Failed to create test user: %v", err)
    }

    return user.ID().String()
}
```

## Controller Testing

### Controller Test Suite

```go
import "github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"

func TestUserController(t *testing.T) {
    t.Parallel()

    // Create test suite
    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // Test as anonymous user
    suite.GET("/users").
        Expect(t).
        Status(302). // Redirect to login
        RedirectTo("/login")
}
```

### Authentication in Tests

```go
func TestAuthenticatedEndpoints(t *testing.T) {
    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // Create test user
    testUser := &user.User{
        ID: uuid.New(),
        Email: "test@example.com",
        FirstName: "Test",
        LastName: "User",
    }

    // Set as authenticated user
    suite.AsUser(testUser).
        GET("/profile").
        Expect(t).
        Status(200)
}
```

### Form Testing

```go
func TestUserForm(t *testing.T) {
    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // Test valid form
    suite.POST("/users").
        Form(url.Values{
            "firstName": {"John"},
            "lastName": {"Doe"},
            "email": {"john@example.com"},
        }).
        Expect(t).
        Status(201)

    // Test validation errors
    suite.POST("/users").
        Form(url.Values{
            "email": {"invalid-email"},
        }).
        Expect(t).
        Status(422).
        HTML().
        HasErrorFor("email") // true
}
```

### File Upload Testing

```go
func TestFileUpload(t *testing.T) {
    suite := controllertest.New(t, uploadModule)

    fileContent := []byte("file contents")
    suite.POST("/upload").
        File("document", "test.txt", fileContent).
        Expect(t).
        Status(200)
}
```

### HTMX Testing

```go
func TestHTMXRequest(t *testing.T) {
    suite := controllertest.New(t, module)

    // Mark as HTMX request
    suite.POST("/search").
        HTMX().
        Form(url.Values{"q": {"query"}}).
        Expect(t).
        Status(200).
        HTML().
        Element("//div[@class='results']").
        Exists()
}
```

## Repository Testing

### Repository Test Patterns

```go
func TestUserRepositoryCreate(t *testing.T) {
    t.Parallel()

    env := itf.Setup(t, itf.WithModules(coreModule))
    defer env.Close()

    repo := itf.GetRepository[*repositories.UserRepository](env)
    ctx := env.Context()

    // Create user
    user := user.New("John", "Doe", "john@example.com")
    created, err := repo.Create(ctx, user)

    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }

    if created.ID() == uuid.Nil {
        t.Error("Created user has no ID")
    }
}
```

### CRUD Operations

```go
func TestUserCRUD(t *testing.T) {
    t.Parallel()

    env := itf.Setup(t)
    defer env.Close()

    repo := itf.GetRepository[*repositories.UserRepository](env)
    ctx := env.Context()

    // CREATE
    user := user.New("Jane", "Smith", "jane@example.com")
    created, _ := repo.Create(ctx, user)

    // READ
    fetched, _ := repo.GetByID(ctx, created.ID())
    if fetched.Email() != "jane@example.com" {
        t.Error("Email mismatch")
    }

    // UPDATE
    updated := created.WithEmail("jane.smith@example.com")
    _, _ = repo.Update(ctx, updated)

    // DELETE
    _ = repo.Delete(ctx, created.ID())
}
```

### Pagination Testing

```go
func TestRepositoryPagination(t *testing.T) {
    env := itf.Setup(t)
    defer env.Close()

    repo := itf.GetRepository[*repositories.UserRepository](env)
    ctx := env.Context()

    // Create multiple users
    for i := 0; i < 25; i++ {
        repo.Create(ctx, user.New(
            fmt.Sprintf("User%d", i),
            "Test",
            fmt.Sprintf("user%d@example.com", i),
        ))
    }

    // Test pagination
    params := &repo.FindParams{
        Limit: 10,
        Offset: 0,
    }

    users, total, _ := repo.GetPaginated(ctx, params)

    if len(users) != 10 {
        t.Errorf("Expected 10 users, got %d", len(users))
    }

    if total != 25 {
        t.Errorf("Expected total 25, got %d", total)
    }
}
```

## Service Testing

### Service Test Patterns

```go
func TestUserServiceCreate(t *testing.T) {
    t.Parallel()

    env := itf.Setup(t,
        itf.WithModules(coreModule),
        itf.WithPermissions(user.AdminRole),
    )
    defer env.Close()

    svc := itf.GetService[*services.UserService](env)
    ctx := env.Context()

    result, err := svc.Create(ctx, &services.CreateUserDTO{
        FirstName: "Alice",
        LastName: "Johnson",
        Email: "alice@example.com",
    })

    if err != nil {
        t.Fatalf("Create failed: %v", err)
    }

    if result.Email() != "alice@example.com" {
        t.Error("Email mismatch")
    }
}
```

### Testing Business Logic

```go
func TestPaymentProcessing(t *testing.T) {
    env := itf.Setup(t)
    defer env.Close()

    paymentSvc := itf.GetService[*services.PaymentService](env)
    ctx := env.Context()

    // Create payment
    payment, err := paymentSvc.Create(ctx, &services.CreatePaymentDTO{
        Amount: 150.00,
        Currency: "USD",
        Reference: "INV-001",
    })

    if payment.Status() != "pending" {
        t.Error("New payment should be pending")
    }

    // Process payment
    processed, err := paymentSvc.Process(ctx, payment.ID())

    if processed.Status() != "completed" {
        t.Error("Processed payment should be completed")
    }
}
```

### Permission Testing

```go
func TestPermissionDenied(t *testing.T) {
    env := itf.Setup(t,
        itf.WithPermissions(role.UserRole), // Limited permissions
    )
    defer env.Close()

    adminSvc := itf.GetService[*services.AdminService](env)
    ctx := env.Context()

    // Should fail due to permissions
    _, err := adminSvc.DeleteUser(ctx, userID)

    if err == nil {
        t.Error("Expected permission error")
    }
}
```

## Test Endpoints (Test-Only)

These endpoints are only available when `ENABLE_TEST_ENDPOINTS=true`:

### Reset Database
```bash
POST /test/reset
Content-Type: application/json

{
    "reset_sequences": true
}
```

### Populate Test Data
```bash
POST /test/populate
Content-Type: application/json

{
    "tenant": {
        "id": "tenant-uuid",
        "name": "Test Tenant"
    },
    "data": {
        "users": [...],
        "invoices": [...]
    }
}
```

## Best Practices

### 1. Use `t.Parallel()`
```go
func TestUserService(t *testing.T) {
    t.Parallel() // Enable parallel execution

    env := itf.Setup(t)
    defer env.Close()

    // Test code...
}
```

### 2. Clean Up Resources
```go
env := itf.Setup(t)
defer env.Close() // Always close environment
```

### 3. Test Error Cases
```go
// Test validation error
_, err := svc.Create(ctx, &dto.Email{Email: "invalid"})
if err == nil {
    t.Error("Expected validation error")
}

// Test not found
_, err := svc.GetByID(ctx, uuid.Nil)
if err != ErrNotFound {
    t.Error("Expected not found error")
}
```

### 4. Use Descriptive Names
```go
// Good
func TestUserServiceCreateWithValidEmail(t *testing.T) {}
func TestUserControllerDeleteUnauthorized(t *testing.T) {}

// Bad
func TestUser(t *testing.T) {}
func TestCreate(t *testing.T) {}
```

### 5. Table-Driven Tests
```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {"valid email", "test@example.com", false},
        {"invalid email", "not-an-email", true},
        {"empty email", "", true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Common Issues and Solutions

### Database Connection Failures
**Problem**: Tests fail with database connection errors
**Solution**: Ensure test database is running and configured in `.env.test`

### Test Isolation Issues
**Problem**: Tests pass individually but fail when run together
**Solution**: Use `t.Parallel()` properly and ensure `defer env.Close()`

### Flaky Tests
**Problem**: Tests randomly fail
**Solution**: Check for time-dependent assertions or race conditions

### Context Timeout
**Problem**: Test times out
**Solution**: Use proper context with timeout:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

## Running Tests

```bash
# Run all tests
go test ./...

# Run specific test
go test ./modules/core -run TestUserService

# Run with verbose output
go test -v ./...

# Run in parallel with specific count
go test -parallel 4 ./...

# Run with coverage
go test -cover ./...

# Run with race detection
go test -race ./...
```

---

For more information, see the [Testkit module overview](./index.md) or the [main documentation](../index.md).
