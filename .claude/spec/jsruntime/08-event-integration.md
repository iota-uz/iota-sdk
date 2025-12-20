# JavaScript Runtime - Event Integration Specification

**Status:** Implementation Ready
**Layer:** Infrastructure Layer
**Dependencies:** Event bus, Runtime engine, Service layer
**Related Issues:** #415, #418, #419

---

## Overview

This specification defines how scripts are triggered by domain events, including event routing, filtering, async execution, error handling, and dead letter queue management.

## Event Integration Architecture

```
┌─────────────────────────────────────────────────┐
│           Domain Event Flow                      │
│                                                  │
│  ┌──────────────────────────────────────────┐   │
│  │     Application Event Bus                │   │
│  │  • user.created                          │   │
│  │  • client.created                        │   │
│  │  • payment.completed                     │   │
│  │  • ...                                   │   │
│  └────────────┬─────────────────────────────┘   │
│               │                                  │
│               ▼                                  │
│  ┌──────────────────────────────────────────┐   │
│  │     Event Trigger Handler                │   │
│  │  • Subscribe to event types              │   │
│  │  • Match events to scripts               │   │
│  │  • Apply filters                         │   │
│  │  • Queue for execution                   │   │
│  └────────────┬─────────────────────────────┘   │
│               │                                  │
│               ▼                                  │
│  ┌──────────────────────────────────────────┐   │
│  │     Event Buffer (Async Queue)           │   │
│  │  • Worker pool (configurable)            │   │
│  │  • Concurrent execution                  │   │
│  │  • Graceful shutdown                     │   │
│  └────────────┬─────────────────────────────┘   │
│               │                                  │
│               ▼                                  │
│  ┌──────────────────────────────────────────┐   │
│  │     Script Execution                     │   │
│  │  • ExecutionService.Execute()            │   │
│  │  • Event data as input                   │   │
│  └────────────┬─────────────────────────────┘   │
│               │                                  │
│               ├─Success──────────────────────┐   │
│               │                              │   │
│               ▼                              ▼   │
│  ┌──────────────────────┐     ┌──────────────┐  │
│  │  Execution Complete  │     │  Dead Letter │  │
│  │  • Log result        │     │  Queue (DLQ) │  │
│  │  • Publish event     │     │  • Retry     │  │
│  └──────────────────────┘     │  • Alert     │  │
│                               └──────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## Event Trigger Handler

**Location:** `modules/scripts/infrastructure/events/event_trigger_handler.go`

### Interface

```go
package events

import (
    "context"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/modules/scripts/services"
    "github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// EventTriggerHandler handles domain events and triggers scripts
type EventTriggerHandler struct {
    scriptRepo   script.Repository
    executionSvc *services.ExecutionService
    eventBus     eventbus.EventBus
    buffer       *EventBuffer
}

// NewEventTriggerHandler creates a new event trigger handler
func NewEventTriggerHandler(
    scriptRepo script.Repository,
    executionSvc *services.ExecutionService,
    eventBus eventbus.EventBus,
    bufferSize int,
    workerCount int,
) *EventTriggerHandler {
    handler := &EventTriggerHandler{
        scriptRepo:   scriptRepo,
        executionSvc: executionSvc,
        eventBus:     eventBus,
        buffer:       NewEventBuffer(bufferSize, workerCount),
    }

    // Subscribe to all supported events
    handler.subscribeToEvents()

    return handler
}

// subscribeToEvents subscribes handler to domain events
func (h *EventTriggerHandler) subscribeToEvents() {
    // Core events
    h.eventBus.Subscribe(h.onUserCreated)
    h.eventBus.Subscribe(h.onUserUpdated)
    h.eventBus.Subscribe(h.onSessionCreated)

    // CRM events
    h.eventBus.Subscribe(h.onClientCreated)
    h.eventBus.Subscribe(h.onClientUpdated)
    h.eventBus.Subscribe(h.onChatMessageReceived)

    // Finance events
    h.eventBus.Subscribe(h.onPaymentCreated)
    h.eventBus.Subscribe(h.onTransactionCompleted)
    h.eventBus.Subscribe(h.onExpenseCreated)

    // Custom events from scripts
    h.eventBus.Subscribe(h.onScriptEvent)
}
```

### Event Handlers

```go
// Core events
func (h *EventTriggerHandler) onUserCreated(event *user.CreatedEvent) {
    h.handleEvent(event.Context(), "user.created", map[string]interface{}{
        "userId":    event.Result.ID(),
        "email":     event.Result.Email().String(),
        "firstName": event.Result.FirstName(),
        "lastName":  event.Result.LastName(),
    })
}

func (h *EventTriggerHandler) onUserUpdated(event *user.UpdatedEvent) {
    h.handleEvent(event.Context(), "user.updated", map[string]interface{}{
        "userId":    event.Result.ID(),
        "email":     event.Result.Email().String(),
        "firstName": event.Result.FirstName(),
        "lastName":  event.Result.LastName(),
    })
}

func (h *EventTriggerHandler) onSessionCreated(event *session.CreatedEvent) {
    h.handleEvent(event.Context(), "session.created", map[string]interface{}{
        "sessionId": event.Session.ID().String(),
        "userId":    event.Session.UserID(),
    })
}

// CRM events
func (h *EventTriggerHandler) onClientCreated(event *client.CreatedEvent) {
    h.handleEvent(event.Context(), "client.created", map[string]interface{}{
        "clientId": event.Client.ID().String(),
        "name":     event.Client.Name(),
        "email":    event.Client.Email(),
    })
}

func (h *EventTriggerHandler) onClientUpdated(event *client.UpdatedEvent) {
    h.handleEvent(event.Context(), "client.updated", map[string]interface{}{
        "clientId": event.Client.ID().String(),
        "name":     event.Client.Name(),
        "email":    event.Client.Email(),
    })
}

func (h *EventTriggerHandler) onChatMessageReceived(event *chat.MessageReceivedEvent) {
    h.handleEvent(event.Context(), "chat.message_received", map[string]interface{}{
        "chatId":    event.ChatID.String(),
        "messageId": event.MessageID.String(),
        "content":   event.Content,
    })
}

// Finance events
func (h *EventTriggerHandler) onPaymentCreated(event *payment.CreatedEvent) {
    h.handleEvent(event.Context(), "payment.created", map[string]interface{}{
        "paymentId": event.Payment.ID().String(),
        "amount":    event.Payment.Amount(),
        "currency":  event.Payment.Currency(),
    })
}

func (h *EventTriggerHandler) onTransactionCompleted(event *transaction.CompletedEvent) {
    h.handleEvent(event.Context(), "transaction.completed", map[string]interface{}{
        "transactionId": event.Transaction.ID().String(),
        "amount":        event.Transaction.Amount(),
        "status":        event.Transaction.Status(),
    })
}

func (h *EventTriggerHandler) onExpenseCreated(event *expense.CreatedEvent) {
    h.handleEvent(event.Context(), "expense.created", map[string]interface{}{
        "expenseId": event.Expense.ID().String(),
        "amount":    event.Expense.Amount(),
        "category":  event.Expense.Category(),
    })
}

// Script events (custom events published by scripts)
func (h *EventTriggerHandler) onScriptEvent(event *runtime.ScriptEvent) {
    h.handleEvent(context.Background(), event.Type, event.Payload.(map[string]interface{}))
}
```

### Event Router

```go
// handleEvent processes an event and triggers matching scripts
func (h *EventTriggerHandler) handleEvent(
    ctx context.Context,
    eventType string,
    data map[string]interface{},
) {
    // Find scripts listening to this event type
    params := &script.FindParams{
        Type:      script.TypeEvent,
        EventType: &eventType,
        Enabled:   true,
    }

    scripts, err := h.scriptRepo.GetPaginated(ctx, params)
    if err != nil {
        // Log error but don't fail
        // TODO: Add structured logging
        return
    }

    // Queue each script for execution
    for _, scr := range scripts {
        // Apply filters if configured
        if !h.matchesFilters(scr, data) {
            continue
        }

        // Queue for async execution
        h.buffer.Enqueue(ctx, scr.ID(), data)
    }
}

// matchesFilters checks if event data matches script filters
func (h *EventTriggerHandler) matchesFilters(
    scr script.Script,
    data map[string]interface{},
) bool {
    filters := scr.Filters()
    if len(filters) == 0 {
        return true
    }

    // Apply filters (simple key-value matching)
    for key, expectedValue := range filters {
        actualValue, exists := data[key]
        if !exists || actualValue != expectedValue {
            return false
        }
    }

    return true
}

// Shutdown gracefully shuts down the handler
func (h *EventTriggerHandler) Shutdown(ctx context.Context) error {
    return h.buffer.Shutdown(ctx)
}
```

---

## Event Buffer (Async Queue)

**Location:** `modules/scripts/infrastructure/events/event_buffer.go`

```go
package events

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/services"
)

// EventBuffer queues events for async processing
type EventBuffer struct {
    queue       chan *QueuedEvent
    workerCount int
    executionSvc *services.ExecutionService
    wg          sync.WaitGroup
    shutdown    chan struct{}
}

// QueuedEvent represents an event in the queue
type QueuedEvent struct {
    Context  context.Context
    ScriptID uuid.UUID
    Data     map[string]interface{}
}

// NewEventBuffer creates a new event buffer
func NewEventBuffer(size int, workerCount int) *EventBuffer {
    buffer := &EventBuffer{
        queue:       make(chan *QueuedEvent, size),
        workerCount: workerCount,
        shutdown:    make(chan struct{}),
    }

    // Start workers
    for i := 0; i < workerCount; i++ {
        buffer.wg.Add(1)
        go buffer.worker()
    }

    return buffer
}

// Enqueue adds an event to the queue
func (b *EventBuffer) Enqueue(
    ctx context.Context,
    scriptID uuid.UUID,
    data map[string]interface{},
) {
    select {
    case b.queue <- &QueuedEvent{
        Context:  ctx,
        ScriptID: scriptID,
        Data:     data,
    }:
    default:
        // Queue full, drop event
        // TODO: Add metrics for dropped events
    }
}

// worker processes events from the queue
func (b *EventBuffer) worker() {
    defer b.wg.Done()

    for {
        select {
        case event := <-b.queue:
            b.processEvent(event)
        case <-b.shutdown:
            return
        }
    }
}

// processEvent executes a script for an event
func (b *EventBuffer) processEvent(event *QueuedEvent) {
    // Execute script
    _, err := b.executionSvc.Execute(
        event.Context,
        event.ScriptID,
        event.Data,
    )

    if err != nil {
        // Add to dead letter queue
        b.addToDeadLetterQueue(event, err)
    }
}

// addToDeadLetterQueue adds failed execution to DLQ
func (b *EventBuffer) addToDeadLetterQueue(
    event *QueuedEvent,
    err error,
) {
    // TODO: Implement DLQ persistence
    // For now, just log
}

// Shutdown gracefully shuts down the buffer
func (b *EventBuffer) Shutdown(ctx context.Context) error {
    close(b.shutdown)

    // Wait for workers to finish with timeout
    done := make(chan struct{})
    go func() {
        b.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(30 * time.Second):
        return context.DeadlineExceeded
    }
}
```

---

## Dead Letter Queue

**Location:** `modules/scripts/infrastructure/events/dead_letter_queue.go`

### Database Schema

```sql
-- Migration: Create dead_letter_queue table
CREATE TABLE dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    event_type VARCHAR(255) NOT NULL,
    event_data JSONB NOT NULL,
    error TEXT NOT NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    next_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    INDEX idx_dlq_tenant (tenant_id),
    INDEX idx_dlq_script (script_id),
    INDEX idx_dlq_retry (next_retry_at) WHERE next_retry_at IS NOT NULL
);
```

### Repository Interface

```go
package events

import (
    "context"
    "time"

    "github.com/google/uuid"
)

// DeadLetterEntry represents a failed execution
type DeadLetterEntry struct {
    ID          uuid.UUID
    TenantID    uuid.UUID
    ScriptID    uuid.UUID
    EventType   string
    EventData   map[string]interface{}
    Error       string
    RetryCount  int
    MaxRetries  int
    NextRetryAt *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// DeadLetterQueueRepository manages failed executions
type DeadLetterQueueRepository interface {
    // Add adds an entry to the DLQ
    Add(ctx context.Context, entry *DeadLetterEntry) error

    // GetPendingRetries gets entries ready for retry
    GetPendingRetries(ctx context.Context) ([]*DeadLetterEntry, error)

    // MarkRetried updates retry count and next retry time
    MarkRetried(ctx context.Context, id uuid.UUID, success bool) error

    // Delete removes an entry from DLQ
    Delete(ctx context.Context, id uuid.UUID) error

    // GetForScript gets DLQ entries for a script
    GetForScript(ctx context.Context, scriptID uuid.UUID) ([]*DeadLetterEntry, error)
}
```

### Retry Strategy

```go
package events

import (
    "math"
    "time"
)

// RetryStrategy defines retry behavior
type RetryStrategy struct {
    MaxRetries      int
    InitialDelay    time.Duration
    MaxDelay        time.Duration
    BackoffFactor   float64
}

// DefaultRetryStrategy returns default retry config
func DefaultRetryStrategy() *RetryStrategy {
    return &RetryStrategy{
        MaxRetries:    3,
        InitialDelay:  1 * time.Minute,
        MaxDelay:      1 * time.Hour,
        BackoffFactor: 2.0,
    }
}

// CalculateNextRetry calculates next retry time
func (s *RetryStrategy) CalculateNextRetry(retryCount int) time.Time {
    delay := float64(s.InitialDelay) * math.Pow(s.BackoffFactor, float64(retryCount))

    if delay > float64(s.MaxDelay) {
        delay = float64(s.MaxDelay)
    }

    return time.Now().Add(time.Duration(delay))
}

// ShouldRetry checks if entry should be retried
func (s *RetryStrategy) ShouldRetry(retryCount int) bool {
    return retryCount < s.MaxRetries
}
```

### Retry Worker

```go
package events

import (
    "context"
    "time"
)

// RetryWorker processes DLQ retries
type RetryWorker struct {
    dlqRepo      DeadLetterQueueRepository
    executionSvc *services.ExecutionService
    strategy     *RetryStrategy
    ticker       *time.Ticker
    shutdown     chan struct{}
}

// NewRetryWorker creates a new retry worker
func NewRetryWorker(
    dlqRepo DeadLetterQueueRepository,
    executionSvc *services.ExecutionService,
) *RetryWorker {
    return &RetryWorker{
        dlqRepo:      dlqRepo,
        executionSvc: executionSvc,
        strategy:     DefaultRetryStrategy(),
        ticker:       time.NewTicker(1 * time.Minute),
        shutdown:     make(chan struct{}),
    }
}

// Start starts the retry worker
func (w *RetryWorker) Start(ctx context.Context) {
    go func() {
        for {
            select {
            case <-w.ticker.C:
                w.processRetries(ctx)
            case <-w.shutdown:
                return
            }
        }
    }()
}

// processRetries processes pending retries
func (w *RetryWorker) processRetries(ctx context.Context) {
    entries, err := w.dlqRepo.GetPendingRetries(ctx)
    if err != nil {
        // Log error
        return
    }

    for _, entry := range entries {
        w.retryExecution(ctx, entry)
    }
}

// retryExecution retries a failed execution
func (w *RetryWorker) retryExecution(ctx context.Context, entry *DeadLetterEntry) {
    // Execute script
    _, err := w.executionSvc.Execute(ctx, entry.ScriptID, entry.EventData)

    success := err == nil

    // Update DLQ entry
    if err := w.dlqRepo.MarkRetried(ctx, entry.ID, success); err != nil {
        // Log error
    }

    if !success && w.strategy.ShouldRetry(entry.RetryCount+1) {
        // Schedule next retry
        // Already handled by MarkRetried
    } else if !success {
        // Max retries reached, send alert
        w.sendAlert(entry)
    }
}

// sendAlert sends alert for failed execution
func (w *RetryWorker) sendAlert(entry *DeadLetterEntry) {
    // TODO: Implement alerting (email, Slack, etc.)
}

// Shutdown stops the retry worker
func (w *RetryWorker) Shutdown() {
    w.ticker.Stop()
    close(w.shutdown)
}
```

---

## Supported Event Types

### Core Events

```go
const (
    EventUserCreated       = "user.created"
    EventUserUpdated       = "user.updated"
    EventUserDeleted       = "user.deleted"
    EventSessionCreated    = "session.created"
    EventSessionExpired    = "session.expired"
)
```

### CRM Events

```go
const (
    EventClientCreated         = "client.created"
    EventClientUpdated         = "client.updated"
    EventClientDeleted         = "client.deleted"
    EventChatMessageReceived   = "chat.message_received"
    EventChatMessageSent       = "chat.message_sent"
)
```

### Finance Events

```go
const (
    EventPaymentCreated       = "payment.created"
    EventPaymentCompleted     = "payment.completed"
    EventTransactionCompleted = "transaction.completed"
    EventExpenseCreated       = "expense.created"
    EventDebtCreated          = "debt.created"
)
```

### Custom Events

Scripts can publish custom events via `events.publish()` API.

---

## Configuration

**Environment Variables:**

```bash
# Event processing
EVENT_BUFFER_SIZE=1000
EVENT_WORKER_COUNT=10

# Retry configuration
EVENT_RETRY_MAX_RETRIES=3
EVENT_RETRY_INITIAL_DELAY=1m
EVENT_RETRY_MAX_DELAY=1h
EVENT_RETRY_BACKOFF_FACTOR=2.0
```

**Config Loading:**

```go
// In modules/scripts/module.go
func loadEventConfig() EventConfig {
    return EventConfig{
        BufferSize:  getEnvInt("EVENT_BUFFER_SIZE", 1000),
        WorkerCount: getEnvInt("EVENT_WORKER_COUNT", 10),
        RetryStrategy: &RetryStrategy{
            MaxRetries:    getEnvInt("EVENT_RETRY_MAX_RETRIES", 3),
            InitialDelay:  getEnvDuration("EVENT_RETRY_INITIAL_DELAY", 1*time.Minute),
            MaxDelay:      getEnvDuration("EVENT_RETRY_MAX_DELAY", 1*time.Hour),
            BackoffFactor: getEnvFloat("EVENT_RETRY_BACKOFF_FACTOR", 2.0),
        },
    }
}
```

---

## Module Registration

**Location:** `modules/scripts/module.go`

```go
func (m *Module) RegisterEventHandlers(app *application.Application) {
    // Create event trigger handler
    handler := events.NewEventTriggerHandler(
        m.scriptRepo,
        m.executionService,
        app.EventBus(),
        m.config.EventBufferSize,
        m.config.EventWorkerCount,
    )

    // Store handler for graceful shutdown
    m.eventHandler = handler

    // Start retry worker
    retryWorker := events.NewRetryWorker(
        m.dlqRepo,
        m.executionService,
    )
    retryWorker.Start(context.Background())

    // Store worker for graceful shutdown
    m.retryWorker = retryWorker
}

func (m *Module) Shutdown(ctx context.Context) error {
    if m.eventHandler != nil {
        if err := m.eventHandler.Shutdown(ctx); err != nil {
            return err
        }
    }

    if m.retryWorker != nil {
        m.retryWorker.Shutdown()
    }

    return nil
}
```

---

## Event Filters

### Filter Configuration

Scripts can define filters to match specific events:

```go
// In script entity
type Script interface {
    // ...
    Filters() map[string]interface{}
    SetFilters(filters map[string]interface{}) Script
}
```

### Example Filters

```javascript
// Script only triggers for specific client
{
    "eventType": "client.created",
    "filters": {
        "email": "important@client.com"
    }
}

// Script only triggers for high-value payments
{
    "eventType": "payment.created",
    "filters": {
        "amount": { "gt": 10000 }
    }
}
```

### Filter Matching

```go
func (h *EventTriggerHandler) matchesFilters(
    scr script.Script,
    data map[string]interface{},
) bool {
    filters := scr.Filters()
    if len(filters) == 0 {
        return true
    }

    for key, expectedValue := range filters {
        actualValue, exists := data[key]
        if !exists {
            return false
        }

        // Simple equality check
        if actualValue != expectedValue {
            // TODO: Support operators (gt, lt, in, contains)
            return false
        }
    }

    return true
}
```

---

## Monitoring & Metrics

### Metrics to Track

```go
// Event processing metrics
type EventMetrics struct {
    EventsReceived   int64
    EventsProcessed  int64
    EventsDropped    int64
    EventsFailed     int64
    AverageQueueTime time.Duration
}

// DLQ metrics
type DLQMetrics struct {
    TotalEntries    int64
    PendingRetries  int64
    SuccessfulRetries int64
    FailedRetries   int64
}
```

### Prometheus Integration

```go
// Example Prometheus metrics
var (
    eventsReceived = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "scripts_events_received_total",
            Help: "Total events received",
        },
        []string{"event_type"},
    )

    eventsProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "scripts_events_processed_total",
            Help: "Total events processed",
        },
        []string{"event_type", "status"},
    )
)
```

---

## Testing

**Location:** `modules/scripts/infrastructure/events/event_trigger_handler_test.go`

```go
func TestEventTriggerHandler_OnUserCreated(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t)

    // Create event-triggered script
    scriptSvc := itf.GetService[*services.ScriptService](env)
    scr, _ := scriptSvc.Create(env.Ctx, dtos.CreateScriptDTO{
        Name:      "User Welcome",
        Type:      "event",
        EventType: "user.created",
        Source:    "sdk.log.info('User created', { email: ctx.input.email });",
        Enabled:   true,
    })

    // Create event handler
    handler := events.NewEventTriggerHandler(
        env.Repository(),
        itf.GetService[*services.ExecutionService](env),
        env.EventBus(),
        100,
        2,
    )

    // Publish user.created event
    event := &user.CreatedEvent{
        Result: testUser,
    }
    env.EventBus().Publish(event)

    // Wait for async processing
    time.Sleep(100 * time.Millisecond)

    // Verify execution was created
    executions, _ := env.Repository().GetPaginated(env.Ctx, &execution.FindParams{
        ScriptID: &scr.ID(),
    })

    assert.Len(t, executions, 1)
    assert.Equal(t, execution.StatusCompleted, executions[0].Status())
}
```

---

## Next Steps

After implementing event integration:

1. **Presentation Layer** (09-presentation.md) - Controllers, ViewModels, templates
2. **Cron Scheduler** (10-cron-scheduler.md) - Scheduled script execution

---

## References

- Service layer: `05-service-layer.md`
- Runtime engine: `06-runtime-engine.md`
- API bindings: `07-api-bindings.md`
- Domain entities: `01-domain-entities.md`
- Event bus: `pkg/eventbus/event_bus.go`
