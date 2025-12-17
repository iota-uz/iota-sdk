---
layout: default
title: Technical Guide
parent: Logging
nav_order: 1
description: "Technical implementation of logging in IOTA SDK"
---

# Logging Technical Guide

This guide covers the implementation details of the IOTA SDK Logging module.

## Architecture

The logging module is built on the Logrus library with custom hooks and formatters for structured logging with automatic context tracking.

### Core Components

#### Logger Interface
```go
// From composables package
logger := composables.UseLogger(ctx)

// Standard methods
logger.Debug(msg string)
logger.Info(msg string)
logger.Warn(msg string)
logger.Error(msg string)
logger.Fatal(msg string)

// Field methods
logger.WithField(key string, value interface{}) *logrus.Entry
logger.WithFields(fields map[string]interface{}) *logrus.Entry
```

#### SourceHook Implementation
Automatically captures source location information:

```go
// Automatically added to all logs
{
    "source_file": "/path/to/file.go",
    "source_line": 42,
    "source_function": "FunctionName"
}
```

#### Context-Aware Fields
Fields automatically extracted from context:

```go
// Tenant ID (if available)
"tenant_id": ctx.Value("tenant_id")

// User ID (if authenticated)
"user_id": ctx.Value("user_id")

// Request ID (for correlation)
"request_id": ctx.Value("request_id")

// Operation name
"operation": ctx.Value("operation")
```

## Usage Patterns

### Basic Logging

#### Simple Log Messages
```go
logger := composables.UseLogger(ctx)

logger.Info("User logged in successfully")
logger.Warn("Rate limit approaching")
logger.Error("Failed to process payment")
```

#### Log with Fields
```go
logger.WithFields(map[string]interface{}{
    "user_id": user.ID(),
    "email": user.Email(),
    "action": "login",
}).Info("User authentication")
```

### Controller Logging

```go
func (c *UserController) GetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    logger := composables.UseLogger(ctx)

    userID := mux.Vars(r)["id"]

    logger.WithField("user_id", userID).Debug("Fetching user")

    user, err := c.userService.GetByID(ctx, userID)
    if err != nil {
        logger.WithField("user_id", userID).Error("Failed to fetch user")
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }

    logger.WithField("user_id", userID).Info("User retrieved successfully")
    // Continue with response...
}
```

### Service Layer Logging

```go
func (s *UserService) Create(ctx context.Context, dto *CreateUserDTO) (User, error) {
    const op serrors.Op = "UserService.Create"
    logger := composables.UseLogger(ctx)

    logger.WithField("email", dto.Email).Debug("Creating new user")

    // Validation
    if err := s.validateEmail(dto.Email); err != nil {
        logger.WithFields(map[string]interface{}{
            "email": dto.Email,
            "error": err.Error(),
        }).Warn("User creation validation failed")
        return nil, serrors.E(op, serrors.KindValidation, err)
    }

    // Create user
    user, err := s.repo.Create(ctx, user)
    if err != nil {
        logger.WithField("email", dto.Email).Error("Failed to create user")
        return nil, serrors.E(op, err)
    }

    logger.WithFields(map[string]interface{}{
        "user_id": user.ID(),
        "email": user.Email(),
    }).Info("User created successfully")

    return user, nil
}
```

### Repository Logging

```go
func (r *UserRepository) Create(ctx context.Context, user User) (User, error) {
    const op serrors.Op = "UserRepository.Create"
    logger := composables.UseLogger(ctx)
    tenantID := composables.UseTenantID(ctx)

    logger.WithFields(map[string]interface{}{
        "tenant_id": tenantID,
        "email": user.Email(),
    }).Debug("Inserting user into database")

    // Database operation...

    return user, nil
}
```

## Configuration

### Environment Variables

```bash
# Log Level (debug, info, warn, error)
LOG_LEVEL=info

# Log Format (json, text)
LOG_FORMAT=json

# Log Output (stdout, stderr, file path)
LOG_OUTPUT=stdout

# Structured logging
STRUCTURED_LOGGING=true

# Include source location
INCLUDE_SOURCE_LOCATION=true
```

### Programmatic Configuration

```go
import "github.com/iota-uz/iota-sdk/pkg/configuration"

conf := configuration.Use()

// Configure logger
conf.Logger().SetLevel(logrus.InfoLevel)
conf.Logger().SetFormatter(&logrus.JSONFormatter{
    TimestampFormat: "2006-01-02T15:04:05Z07:00",
})
```

## Database Schema

The logging module includes a schema for persisting logs:

```sql
CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    level VARCHAR(10) NOT NULL,
    message TEXT NOT NULL,
    fields JSONB,
    source_file VARCHAR(255),
    source_line INTEGER,
    source_function VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_logs_tenant_level (tenant_id, level),
    INDEX idx_logs_created_at (created_at DESC)
);
```

## Field Types

### Standard Fields
- `level`: Log level (debug, info, warn, error)
- `msg`: Log message
- `time`: Timestamp (ISO 8601)
- `source_file`: Source file path
- `source_line`: Line number
- `source_function`: Function name

### Context Fields
- `tenant_id`: UUID
- `user_id`: UUID
- `request_id`: UUID or string
- `operation`: Operation name

### Custom Fields
Any arbitrary fields added via `WithField()` or `WithFields()`

## Performance Considerations

### Structured Logging Overhead
- JSON formatting: ~5-10% overhead vs text logging
- Field extraction: Minimal (context value lookups)
- Source location: ~2-5% overhead with SourceHook

### Best Practices

1. **Use Appropriate Log Levels**
   ```go
   // Debug for development/diagnostics
   logger.Debug("Detailed diagnostic information")

   // Info for important events
   logger.Info("User action completed")

   // Warn for potentially problematic situations
   logger.Warn("Deprecated API usage detected")

   // Error for failures
   logger.Error("Operation failed")
   ```

2. **Avoid Logging Sensitive Data**
   ```go
   // GOOD: Don't log passwords or tokens
   logger.WithField("email", user.Email()).Info("User logged in")

   // BAD: Never log sensitive data
   logger.WithField("password", password).Info("Login attempted") // DON'T DO THIS
   ```

3. **Use Structured Fields for Debugging**
   ```go
   // Good for log aggregation
   logger.WithFields(map[string]interface{}{
       "action": "payment_processed",
       "amount": 100.50,
       "currency": "USD",
       "payment_method": "card",
   }).Info("Payment completed")
   ```

4. **Include Context for Correlation**
   ```go
   // Enables tracing across services
   logger.WithFields(map[string]interface{}{
       "request_id": requestID,
       "trace_id": traceID,
       "span_id": spanID,
   }).Debug("Processing request")
   ```

## Integration with Error Handling

```go
import "github.com/iota-uz/iota-sdk/pkg/serrors"

// Log errors with operation context
logger := composables.UseLogger(ctx)

if err := someOperation(ctx); err != nil {
    logger.WithFields(map[string]interface{}{
        "error": err.Error(),
        "operation": "someOperation",
        "user_id": userID,
    }).Error("Operation failed")

    return serrors.E("operation", err)
}
```

## Testing Logging

```go
import "testing"

func TestUserCreation(t *testing.T) {
    // Create test context with logger
    ctx := composables.WithLogger(context.Background(), logrus.New())

    // Your test code...

    // Verify logs were created (optional)
    // Use log capture middleware or spy
}
```

## Common Issues and Solutions

### Missing Tenant Context
**Problem**: Logs don't include `tenant_id` field
**Solution**: Ensure tenant context is set in middleware
```go
middleware.WithTenant(ctx, tenantID)
```

### Performance Issues
**Problem**: Logging is slowing down application
**Solution**:
- Reduce log level (use `warn` or `error` in production)
- Avoid logging in tight loops
- Use buffered output

### Large Field Values
**Problem**: Large JSON objects in logs causing storage issues
**Solution**: Serialize only necessary fields
```go
// Instead of logging entire object
logger.WithField("user", user).Info("User action")

// Log only relevant fields
logger.WithFields(map[string]interface{}{
    "user_id": user.ID(),
    "email": user.Email(),
    "action": "login",
}).Info("User logged in")
```

## Advanced Topics

### Custom Hooks
Create custom hooks for special log processing:

```go
type CustomHook struct{}

func (h *CustomHook) Levels() []logrus.Level {
    return logrus.AllLevels
}

func (h *CustomHook) Fire(entry *logrus.Entry) error {
    // Custom processing
    return nil
}

// Register hook
logger.AddHook(&CustomHook{})
```

### Log Aggregation
Configure remote log shipping:

```bash
# Send logs to external service
LOG_OUTPUT=syslog://logs.example.com:514
```

### Metrics and Monitoring
Integrate with observability tools:

```go
// Prometheus metrics
logMetrics := prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "log_entries_total",
        Help: "Total number of log entries",
    },
    []string{"level", "module"},
)
```

---

For more information, see the [Logging module overview](./index.md) or consult the [IOTA SDK documentation](../index.md).
