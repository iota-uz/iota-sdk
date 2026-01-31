# BI-Chat Hooks & Event System

The hooks package provides an extensible event system for observability, cost tracking, and custom integrations in the BI-Chat framework.

## Overview

The event system follows a **publish-subscribe pattern** where:
- Components publish **Events** to an **EventBus**
- **EventHandlers** subscribe to specific event types or all events
- Events flow through the system without blocking publishers
- Multiple handlers can process the same event independently

## Core Components

### Event Interface

All events implement the `Event` interface:

```go
type Event interface {
    Type() string           // Event type identifier (e.g., "llm.request")
    Timestamp() time.Time   // When the event occurred
    SessionID() uuid.UUID   // Chat session identifier
    TenantID() uuid.UUID    // Tenant identifier (for multi-tenancy)
}
```

### EventBus

The EventBus distributes events to registered handlers:

```go
type EventBus interface {
    // Publish sends an event to all registered handlers
    Publish(ctx context.Context, event Event) error

    // Subscribe registers a handler for specific event types
    Subscribe(handler EventHandler, types ...string) (unsubscribe func())

    // SubscribeAll registers a handler for all events
    SubscribeAll(handler EventHandler) (unsubscribe func())
}
```

**Thread-Safety**: The default EventBus implementation is fully thread-safe.

### EventHandler

Handlers process events:

```go
type EventHandler interface {
    Handle(ctx context.Context, event Event) error
}

// Function adapter for easy handler creation
type EventHandlerFunc func(ctx context.Context, event Event) error
```

## Event Types

### LLM Events

**LLMRequestEvent** - Emitted before sending a request to the LLM provider:
```go
event := events.NewLLMRequestEvent(
    sessionID, tenantID,
    "claude-3-5-sonnet", "anthropic",
    10, // message count
    5,  // tool count
    1000, // estimated tokens
)
```

**LLMResponseEvent** - Emitted after receiving an LLM response:
```go
event := events.NewLLMResponseEvent(
    sessionID, tenantID,
    "claude-3-5-sonnet", "anthropic",
    1000, // prompt tokens
    2000, // completion tokens
    3000, // total tokens
    1500, // latency ms
    "stop", // finish reason
    2, // tool calls
)
```

**LLMStreamEvent** - Emitted for each chunk during streaming:
```go
event := events.NewLLMStreamEvent(
    sessionID, tenantID,
    "claude-3-5-sonnet", "anthropic",
    "chunk text", // chunk content
    0, // chunk index
    false, // is final chunk
)
```

### Tool Events

**ToolStartEvent** - Tool execution started:
```go
event := events.NewToolStartEvent(
    sessionID, tenantID,
    "execute_sql", // tool name
    `{"query": "SELECT * FROM users"}`, // arguments JSON
    "call-123", // call ID
)
```

**ToolCompleteEvent** - Tool execution completed:
```go
event := events.NewToolCompleteEvent(
    sessionID, tenantID,
    "execute_sql",
    `{"query": "SELECT * FROM users"}`,
    "call-123",
    "result data", // tool result
    250, // duration ms
)
```

**ToolErrorEvent** - Tool execution failed:
```go
event := events.NewToolErrorEvent(
    sessionID, tenantID,
    "execute_sql",
    `{"query": "SELECT * FROM users"}`,
    "call-123",
    "connection timeout", // error message
    5000, // duration before failure
)
```

### Context Events

**ContextCompileEvent** - Context compiled for LLM:
```go
event := events.NewContextCompileEvent(
    sessionID, tenantID,
    "anthropic", // provider
    150000, // total tokens
    map[string]int{"history": 100000, "pinned": 50000}, // tokens by kind
    25, // block count
    true, // compacted
    false, // truncated
    3, // excluded blocks
)
```

**ContextCompactEvent** - History summarized to fit budget:
```go
event := events.NewContextCompactEvent(
    sessionID, tenantID,
    50, // original messages
    10, // compacted to
    20000, // tokens saved
    "Summary of previous conversation...", // summary text
)
```

**ContextOverflowEvent** - Context exceeded token budget:
```go
event := events.NewContextOverflowEvent(
    sessionID, tenantID,
    200000, // requested tokens
    180000, // available tokens
    "truncate", // strategy
    true, // resolved
)
```

### Session Events

**SessionCreateEvent** - New session created:
```go
event := events.NewSessionCreateEvent(
    sessionID, tenantID,
    12345, // user ID
    "Financial Analysis Chat", // title
)
```

**MessageSaveEvent** - Message persisted:
```go
event := events.NewMessageSaveEvent(
    sessionID, tenantID,
    messageID,
    "assistant", // role
    1500, // content length
    2, // tool calls
)
```

**InterruptEvent** - Agent interrupted for human input:
```go
event := events.NewInterruptEvent(
    sessionID, tenantID,
    "ask_user_question", // interrupt type
    "ParentAgent", // agent name
    "Which customer should I analyze?", // question
    "checkpoint-abc123", // checkpoint ID
)
```

## Built-in Handlers

### LoggingHandler

Logs events to a logrus logger with human-readable formatting:

```go
import (
    "github.com/sirupsen/logrus"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
)

logger := logrus.New()
handler := handlers.NewLoggingHandler(logger, logrus.InfoLevel)

bus.SubscribeAll(handler)
```

**Example Output**:
```
INFO[0001] LLM request to anthropic/claude-3-5-sonnet with 10 messages, 5 tools (~1000 tokens)
INFO[0002] Tool execution started: execute_sql (call_id=call-123)
INFO[0003] Tool execution completed: execute_sql (250ms, result=150 chars)
```

### MetricsHandler

Exports Prometheus metrics for monitoring:

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
)

registry := prometheus.NewRegistry()
handler := handlers.NewMetricsHandler(registry)

bus.SubscribeAll(handler)
```

**Metrics Exported**:
- `bichat_llm_requests_total` - LLM API request count
- `bichat_llm_response_duration_ms` - LLM response latency histogram
- `bichat_llm_tokens_total` - Token consumption (prompt/completion)
- `bichat_tool_executions_total` - Tool execution count by status
- `bichat_tool_duration_ms` - Tool execution duration histogram
- `bichat_tool_errors_total` - Tool error count
- `bichat_context_compilations_total` - Context compilation count
- `bichat_context_tokens` - Context token histogram
- `bichat_context_overflows_total` - Context overflow count
- `bichat_sessions_created_total` - Session creation count
- `bichat_messages_saved_total` - Message save count
- `bichat_interrupts_total` - HITL interrupt count

### AsyncHandler

Wraps any handler for asynchronous processing (prevents blocking):

```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"

slowHandler := handlers.NewLoggingHandler(logger, logrus.InfoLevel)
asyncHandler := handlers.NewAsyncHandler(slowHandler, 1000) // 1000 event buffer

bus.SubscribeAll(asyncHandler)

// Later, when shutting down:
asyncHandler.Close() // Waits for pending events
```

**Use Case**: When handlers do slow operations (e.g., database writes, external API calls), wrap them with AsyncHandler to avoid blocking event publishers.

## Cost Tracking

The **CostTracker** tracks LLM usage costs by listening to `LLMResponseEvent`:

```go
import "github.com/iota-uz/iota-sdk/pkg/bichat/hooks"

// Define pricing
pricing := hooks.NewStaticModelPricing()
pricing.AddPrice("claude-3-5-sonnet", 3.0, 15.0) // $3/$15 per 1M tokens
pricing.AddPrice("gpt-4", 10.0, 30.0)

// Create tracker
tracker := hooks.NewCostTracker(pricing)

// Register with event bus
bus.SubscribeAll(tracker)

// Later, retrieve costs:
tenantCost := tracker.GetTenantCost(tenantID)
fmt.Printf("Total cost: $%.6f\n", tenantCost.TotalCost)
fmt.Printf("Prompt tokens: %d\n", tenantCost.PromptTokens)
fmt.Printf("Completion tokens: %d\n", tenantCost.CompletionTokens)
fmt.Printf("Requests: %d\n", tenantCost.RequestCount)

sessionCost := tracker.GetSessionCost(sessionID)
fmt.Printf("Session cost: $%.6f\n", sessionCost.TotalCost)
```

**Custom Pricing**:
```go
type DatabasePricing struct {
    db *sql.DB
}

func (p *DatabasePricing) GetPrice(model string) (inputPer1M, outputPer1M float64, err error) {
    // Fetch pricing from database
    err = p.db.QueryRow("SELECT input_price, output_price FROM model_pricing WHERE model = $1", model).
        Scan(&inputPer1M, &outputPer1M)
    return
}

tracker := hooks.NewCostTracker(&DatabasePricing{db: db})
```

## Usage Examples

### Basic Setup

```go
import (
    "context"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/events"
    "github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
    "github.com/sirupsen/logrus"
    "github.com/prometheus/client_golang/prometheus"
)

// Create event bus
bus := hooks.NewEventBus()

// Add logging
logger := logrus.New()
loggingHandler := handlers.NewLoggingHandler(logger, logrus.InfoLevel)
bus.SubscribeAll(loggingHandler)

// Add metrics
registry := prometheus.NewRegistry()
metricsHandler := handlers.NewMetricsHandler(registry)
bus.SubscribeAll(metricsHandler)

// Add cost tracking
pricing := hooks.NewStaticModelPricing()
pricing.AddPrice("claude-3-5-sonnet", 3.0, 15.0)
costTracker := hooks.NewCostTracker(pricing)
bus.SubscribeAll(costTracker)

// Publish events
ctx := context.Background()
event := events.NewLLMRequestEvent(sessionID, tenantID, "claude-3-5-sonnet", "anthropic", 10, 5, 1000)
bus.Publish(ctx, event)
```

### Custom Event Handler

```go
type AuditHandler struct {
    db *sql.DB
}

func (h *AuditHandler) Handle(ctx context.Context, event hooks.Event) error {
    // Log to audit table
    _, err := h.db.ExecContext(ctx,
        "INSERT INTO audit_log (event_type, session_id, tenant_id, timestamp, data) VALUES ($1, $2, $3, $4, $5)",
        event.Type(),
        event.SessionID(),
        event.TenantID(),
        event.Timestamp(),
        // Serialize event to JSON...
    )
    return err
}

// Register
auditHandler := &AuditHandler{db: db}
bus.SubscribeAll(auditHandler)
```

### Filtered Subscription

```go
// Only process LLM and tool events
handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
    fmt.Printf("Event: %s at %s\n", event.Type(), event.Timestamp())
    return nil
})

bus.Subscribe(handler, "llm.request", "llm.response", "tool.start", "tool.complete")
```

### Conditional Processing

```go
handler := hooks.EventHandlerFunc(func(ctx context.Context, event hooks.Event) error {
    // Only process events for specific tenant
    if event.TenantID() == myTenantID {
        // Process event...
    }
    return nil
})

bus.SubscribeAll(handler)
```

## Best Practices

### 1. Handler Performance

**Problem**: Slow handlers block event processing.

**Solution**: Use AsyncHandler for I/O-bound operations:
```go
slowHandler := &DatabaseAuditHandler{db: db}
asyncHandler := handlers.NewAsyncHandler(slowHandler, 1000)
bus.SubscribeAll(asyncHandler)
```

### 2. Error Handling

**Problem**: Handler errors should not crash the system.

**Solution**: EventBus ignores handler errors by design. Log errors inside your handler:
```go
func (h *MyHandler) Handle(ctx context.Context, event hooks.Event) error {
    if err := h.process(event); err != nil {
        h.logger.Errorf("Failed to process event: %v", err)
        return err // Logged but won't stop other handlers
    }
    return nil
}
```

### 3. Memory Management

**Problem**: AsyncHandler buffers can grow unbounded.

**Solution**:
- Set appropriate buffer size based on event rate
- Monitor dropped events (TODO: add dropped event metric)
- Call `Close()` on shutdown to drain buffer

```go
asyncHandler := handlers.NewAsyncHandler(handler, 1000) // Adjust buffer size
defer asyncHandler.Close() // Ensure drain on shutdown
```

### 4. Tenant Isolation

**Problem**: Events from one tenant shouldn't affect another.

**Solution**: Filter by tenant in handlers:
```go
func (h *TenantSpecificHandler) Handle(ctx context.Context, event hooks.Event) error {
    if event.TenantID() != h.allowedTenantID {
        return nil // Skip events from other tenants
    }
    // Process...
}
```

### 5. Testing

**Problem**: Events make testing harder.

**Solution**: Use a test event bus:
```go
func TestMyService(t *testing.T) {
    bus := hooks.NewEventBus()

    // Track events for assertions
    var events []hooks.Event
    bus.SubscribeAll(hooks.EventHandlerFunc(func(ctx context.Context, e hooks.Event) error {
        events = append(events, e)
        return nil
    }))

    // Run test...

    // Assert events
    if len(events) != 2 {
        t.Errorf("Expected 2 events, got %d", len(events))
    }
}
```

## Architecture Decisions

### Why String Event Types?

- **Extensibility**: Users can define custom event types without modifying the SDK
- **Simplicity**: No need to register event types in a central registry
- **Compatibility**: Easy to serialize/deserialize across language boundaries

### Why Synchronous Publish?

- **Predictability**: Events are processed in order
- **Simplicity**: No complex concurrency bugs
- **Performance**: For fast handlers, synchronous is faster (no channel overhead)
- **AsyncHandler**: Available for slow handlers when needed

### Why Ignore Handler Errors?

- **Resilience**: One broken handler shouldn't break the entire system
- **Observability**: Handlers are primarily for observability, not critical business logic
- **Flexibility**: Handlers can implement their own error recovery strategies

## Future Enhancements

- [ ] Event replay for debugging
- [ ] Event persistence (event sourcing)
- [ ] Metrics for dropped async events
- [ ] Event filtering DSL
- [ ] Batch event publishing
- [ ] Dead letter queue for failed events
