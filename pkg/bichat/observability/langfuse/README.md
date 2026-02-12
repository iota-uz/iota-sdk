# Langfuse Observability Provider

Langfuse integration for BiChat observability, providing tracing, cost tracking, and debugging for LLM interactions.

## Overview

The Langfuse provider implements the `observability.Provider` interface, sending observations to Langfuse for:
- **Tracing**: Session traces with generations, spans, and events
- **Cost Analysis**: Token usage and cost calculation
- **Debugging**: LLM request/response inspection
- **Performance**: Latency and duration tracking

## Configuration

The provider uses clean dependency injection - you create the Langfuse client and pass it to the provider.

```go
import (
    "github.com/henomis/langfuse-go"
    "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
)

// Step 1: Set environment variables for Langfuse SDK
os.Setenv("LANGFUSE_HOST", "https://cloud.langfuse.com")
os.Setenv("LANGFUSE_PUBLIC_KEY", os.Getenv("LANGFUSE_PUBLIC_KEY"))
os.Setenv("LANGFUSE_SECRET_KEY", os.Getenv("LANGFUSE_SECRET_KEY"))

// Step 2: Create Langfuse client
client := langfuse.New(ctx).WithFlushInterval(5 * time.Second)

// Step 3: Create provider configuration
config := langfuse.Config{
    Enabled:     true,
    Environment: "production",
    Version:     "1.0.0",
    SampleRate:  1.0,  // 100% sampling
}

// Step 4: Create provider with injected client
provider, err := langfuse.NewLangfuseProvider(client, config)
if err != nil {
    log.Fatal(err)
}
defer provider.Shutdown(ctx)
```

### Why Dependency Injection?

- **Testability**: Inject mock clients for testing (see Testing section)
- **Clean Architecture**: No side effects in constructor
- **Flexibility**: Full control over client configuration
- **Explicit Dependencies**: Clear separation of client creation vs provider logic

## Usage

### Recording Observations

```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/observability"

// Record LLM generation
provider.RecordGeneration(ctx, observability.GenerationObservation{
    ID:               "gen-123",
    SessionID:        sessionID,
    TenantID:         tenantID,
    Model:            "claude-3-5-sonnet-20241022",
    PromptTokens:     1000,
    CompletionTokens: 500,
    TotalTokens:      1500,
    Duration:         2 * time.Second,
    Timestamp:        time.Now(),
})

// Record span (tool execution, processing step)
provider.RecordSpan(ctx, observability.SpanObservation{
    ID:        "span-123",
    SessionID: sessionID,
    TenantID:  tenantID,
    Name:      "sql_execution",
    Duration:  100 * time.Millisecond,
    Input:     "SELECT * FROM users",
    Output:    "[{...}]",
})

// Record event (error, warning, info)
provider.RecordEvent(ctx, observability.EventObservation{
    ID:        "event-123",
    SessionID: sessionID,
    TenantID:  tenantID,
    Name:      "validation_error",
    Level:     "error",
})
```

### Cost Tracking

The provider automatically calculates costs based on token usage:

```go
obs := observability.GenerationObservation{
    Model:            "gpt-4",
    PromptTokens:     1000,
    CompletionTokens: 500,
    Attributes: map[string]interface{}{
        "input_price_per_1m":  10.00,  // $10 per 1M tokens
        "output_price_per_1m": 30.00,  // $30 per 1M tokens
    },
}

provider.RecordGeneration(ctx, obs)
// Cost: (1000/1M * $10) + (500/1M * $30) = $0.025
```

## Testing with Mock Client

The package provides `MockLangfuseClient` for testing without real Langfuse API calls.

### Basic Usage

```go
import (
    "testing"
    "github.com/iota-uz/iota-sdk/pkg/bichat/observability/langfuse"
)

func TestMyService(t *testing.T) {
    // Create mock client
    mock := langfuse.NewMockClient()

    // Inject mock into provider (clean dependency injection)
    config := langfuse.Config{
        Enabled:    true,
        SampleRate: 1.0,
    }
    provider, err := langfuse.NewLangfuseProvider(mock, config)
    if err != nil {
        t.Fatal(err)
    }

    // Use provider in your tests...
    provider.RecordGeneration(ctx, obs)

    // Assert calls were made to the mock
    calls := mock.GetGenerationCalls()
    if len(calls) != 1 {
        t.Errorf("expected 1 generation call, got %d", len(calls))
    }

    if calls[0].Generation.Model != "gpt-4" {
        t.Errorf("expected model gpt-4, got %s", calls[0].Generation.Model)
    }
}
```

### Error Injection

```go
func TestErrorHandling(t *testing.T) {
    mock := langfuse.NewMockClient().
        WithGenerationError(errors.New("API error"))

    // Your code should handle the error gracefully
    result, err := mock.Generation(&model.Generation{}, nil)

    if err == nil {
        t.Error("expected error, got nil")
    }
}
```

### Response Customization

```go
func TestCustomResponse(t *testing.T) {
    customResp := &model.Generation{
        ID:      "custom-123",
        TraceID: "trace-456",
    }

    mock := langfuse.NewMockClient().
        WithGenerationResponse(customResp)

    result, _ := mock.Generation(&model.Generation{}, nil)

    if result.ID != "custom-123" {
        t.Errorf("expected custom ID, got %s", result.ID)
    }
}
```

### Call Count Assertions

```go
func TestCallCounts(t *testing.T) {
    mock := langfuse.NewMockClient()

    // Make calls...
    mock.Generation(&model.Generation{}, nil)
    mock.Generation(&model.Generation{}, nil)
    mock.Span(&model.Span{}, nil)

    // Assert counts
    if mock.GenerationCallCount() != 2 {
        t.Errorf("expected 2 generation calls, got %d", mock.GenerationCallCount())
    }

    if mock.SpanCallCount() != 1 {
        t.Errorf("expected 1 span call, got %d", mock.SpanCallCount())
    }
}
```

### Reset Between Tests

```go
func TestWithReset(t *testing.T) {
    mock := langfuse.NewMockClient()

    // First test case
    mock.Generation(&model.Generation{}, nil)
    if mock.GenerationCallCount() != 1 {
        t.Error("expected 1 call")
    }

    // Reset for next test case
    mock.Reset()

    // Second test case
    if mock.GenerationCallCount() != 0 {
        t.Error("expected 0 calls after reset")
    }
}
```

### Thread-Safe Testing

The mock client is thread-safe for concurrent testing:

```go
func TestConcurrent(t *testing.T) {
    mock := langfuse.NewMockClient()
    var wg sync.WaitGroup

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            mock.Generation(&model.Generation{}, nil)
        }()
    }

    wg.Wait()

    if mock.GenerationCallCount() != 10 {
        t.Errorf("expected 10 calls, got %d", mock.GenerationCallCount())
    }
}
```

## Mock Client API

### Creation

```go
mock := langfuse.NewMockClient()
```

### Error Injection (Builder Pattern)

```go
mock.WithGenerationError(err)
mock.WithGenerationEndError(err)
mock.WithSpanError(err)
mock.WithSpanEndError(err)
mock.WithEventError(err)
mock.WithTraceError(err)
```

### Response Customization (Builder Pattern)

```go
mock.WithGenerationResponse(resp)
mock.WithGenerationEndResponse(resp)
mock.WithSpanResponse(resp)
mock.WithSpanEndResponse(resp)
mock.WithEventResponse(resp)
mock.WithTraceResponse(resp)
```

### Call Tracking

```go
// Get call details
calls := mock.GetGenerationCalls()
calls := mock.GetGenerationEndCalls()
calls := mock.GetSpanCalls()
calls := mock.GetSpanEndCalls()
calls := mock.GetEventCalls()
calls := mock.GetTraceCalls()
calls := mock.GetFlushCalls()

// Get call counts
count := mock.GenerationCallCount()
count := mock.GenerationEndCallCount()
count := mock.SpanCallCount()
count := mock.SpanEndCallCount()
count := mock.EventCallCount()
count := mock.TraceCallCount()
count := mock.FlushCallCount()
```

### Reset

```go
mock.Reset()  // Clears all state, errors, and responses
```

## Features

### Automatic Trace Creation

The provider automatically creates traces if they don't exist:

```go
// First observation for a session creates trace
provider.RecordGeneration(ctx, obs)  // Creates trace automatically
provider.RecordSpan(ctx, obs2)       // Uses existing trace
```

### Sampling

Control what percentage of observations are sent:

```go
config := langfuse.Config{
    SampleRate: 0.1,  // Send 10% of observations
}
```

### Non-Blocking Errors

All errors are logged but not propagated - observability failures should not break the application:

```go
// Even if Langfuse is down, this returns nil
err := provider.RecordGeneration(ctx, obs)
// err is always nil - check logs for failures
```

### Token Counting

Tracks multiple token types:

```go
obs := observability.GenerationObservation{
    PromptTokens:     1000,
    CompletionTokens: 500,
    TotalTokens:      1500,
    Attributes: map[string]interface{}{
        "cache_write_tokens": 200,
        "cache_read_tokens":  300,
    },
}
```

## Best Practices

1. **Always defer Shutdown**: Ensures pending observations are flushed
   ```go
   provider, _ := langfuse.NewLangfuseProvider(ctx, config)
   defer provider.Shutdown(ctx)
   ```

2. **Use mock in tests**: Never call real Langfuse API in tests
   ```go
   mock := langfuse.NewMockClient()
   ```

3. **Set appropriate sample rate**: Reduce costs in high-traffic production
   ```go
   config.SampleRate = 0.1  // 10% sampling in production
   ```

4. **Include metadata**: Add context for debugging
   ```go
   obs.Attributes = map[string]interface{}{
       "environment": "production",
       "version":     "1.0.0",
       "user_tier":   "premium",
   }
   ```

5. **Use Reset() in table-driven tests**: Clean state between cases
   ```go
   for _, tc := range testCases {
       t.Run(tc.name, func(t *testing.T) {
           mock.Reset()  // Clean slate for each test
           // ... test code
       })
   }
   ```

## Environment Variables

The Langfuse SDK reads credentials from environment variables:

```bash
export LANGFUSE_HOST="https://cloud.langfuse.com"
export LANGFUSE_PUBLIC_KEY="pk-lf-..."
export LANGFUSE_SECRET_KEY="sk-lf-..."
```

Or set in code:

```go
config := langfuse.Config{
    Host:      "https://cloud.langfuse.com",
    PublicKey: "pk-lf-...",
    SecretKey: "sk-lf-...",
}
```

## See Also

- [Langfuse Documentation](https://langfuse.com/docs)
- [BiChat Observability Guide](../README.md)
- [Mock Testing Examples](./mocks_test.go)
