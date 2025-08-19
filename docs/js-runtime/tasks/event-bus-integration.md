# Event Bus Integration - Detailed Implementation Plan

## Overview
This document provides detailed implementation steps for integrating IOTA SDK's event bus with the JavaScript runtime, allowing scripts to listen to and react to domain events.

## Architecture

### Event Flow
```
Domain Layer → EventBus → Event Router → Script Registry → VM Execution → Event Handler
                              ↓
                     Event Buffer/Queue
                              ↓
                      Dead Letter Queue
```

## Implementation Tasks

### 1. Event Subscription Registry

Create `modules/scripts/domain/entities/event_subscription/`:

```go
// event_subscription.go
type EventSubscription struct {
    ID         uuid.UUID
    ScriptID   uuid.UUID
    EventType  string      // e.g., "PaymentCategoryCreatedEvent"
    Filter     string      // JSON filter expression
    Priority   int         // Execution order
    RetryCount int         // Max retry attempts
    Active     bool
}

// event_subscription_repository.go
type Repository interface {
    GetByScriptID(ctx context.Context, scriptID uuid.UUID) ([]EventSubscription, error)
    GetByEventType(ctx context.Context, eventType string) ([]EventSubscription, error)
    Create(ctx context.Context, sub EventSubscription) (EventSubscription, error)
    Update(ctx context.Context, sub EventSubscription) (EventSubscription, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### 2. Event Router Service

Create `modules/scripts/services/event_router_service.go`:

```go
type EventRouterService struct {
    eventBus     eventbus.EventBus
    scriptSvc    *ScriptService
    subRepo      event_subscription.Repository
    executionSvc *ExecutionService
    buffer       *EventBuffer
}

func (s *EventRouterService) Start(ctx context.Context) error {
    // Subscribe to all events
    s.eventBus.Subscribe(s.handleEvent)
    return nil
}

func (s *EventRouterService) handleEvent(event interface{}) {
    eventType := reflect.TypeOf(event).Name()
    
    // Get active subscriptions for this event type
    subs, err := s.subRepo.GetByEventType(ctx, eventType)
    if err != nil {
        log.Error("Failed to get subscriptions", "error", err)
        return
    }
    
    // Queue events for processing
    for _, sub := range subs {
        if sub.Active && s.matchesFilter(event, sub.Filter) {
            s.buffer.Queue(EventTask{
                Subscription: sub,
                Event:       event,
                Timestamp:   time.Now(),
            })
        }
    }
}
```

### 3. Event Buffer & Processing

Create `modules/scripts/services/event_buffer.go`:

```go
type EventBuffer struct {
    queue      chan EventTask
    workers    int
    executor   *ExecutionService
    dlq        *DeadLetterQueue
}

type EventTask struct {
    Subscription event_subscription.EventSubscription
    Event        interface{}
    Timestamp    time.Time
    RetryCount   int
}

func (b *EventBuffer) Start(ctx context.Context) {
    for i := 0; i < b.workers; i++ {
        go b.worker(ctx)
    }
}

func (b *EventBuffer) worker(ctx context.Context) {
    for {
        select {
        case task := <-b.queue:
            if err := b.processEvent(ctx, task); err != nil {
                if task.RetryCount < task.Subscription.RetryCount {
                    task.RetryCount++
                    b.queue <- task // Retry
                } else {
                    b.dlq.Send(task, err) // Send to DLQ
                }
            }
        case <-ctx.Done():
            return
        }
    }
}
```

### 4. JavaScript Event API

Create `pkg/jsruntime/apis/events.go`:

```go
type EventsAPI struct {
    vm           *goja.Runtime
    routerSvc    *EventRouterService
    currentEvent interface{}
}

func (api *EventsAPI) Register(vm *goja.Runtime) error {
    events := vm.NewObject()
    
    // Get current event being processed
    events.Set("current", func() interface{} {
        return api.currentEvent
    })
    
    // Subscribe to events (for HTTP endpoint scripts)
    events.Set("subscribe", func(eventType string, filter map[string]interface{}) error {
        // Only allowed for HTTP endpoint scripts
        return api.createSubscription(eventType, filter)
    })
    
    // Emit custom events
    events.Set("emit", func(eventType string, data map[string]interface{}) error {
        return api.emitEvent(eventType, data)
    })
    
    vm.Set("sdk.events", events)
    return nil
}
```

### 5. Event Handler Script Execution

Update `modules/scripts/services/execution_service.go`:

```go
func (s *ExecutionService) ExecuteEventHandler(
    ctx context.Context, 
    script Script, 
    event interface{},
) (*ExecutionResult, error) {
    // Get VM from pool
    vm := s.vmPool.Get()
    defer s.vmPool.Put(vm)
    
    // Set up event context
    eventAPI := &EventsAPI{
        vm:           vm,
        currentEvent: event,
    }
    eventAPI.Register(vm)
    
    // Set execution timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Execute with monitoring
    start := time.Now()
    result, err := s.executeWithMonitoring(ctx, vm, script.Content)
    
    // Record execution
    s.recordExecution(ctx, &Execution{
        ScriptID:  script.ID,
        Type:      "event_handler",
        Status:    determineStatus(err),
        Duration:  time.Since(start),
        Error:     errorToString(err),
        EventData: event,
    })
    
    return result, err
}
```

### 6. Event Filter DSL

Create `modules/scripts/services/event_filter.go`:

```go
type EventFilter struct {
    expression string
}

// Simple DSL for filtering events
// Example: "$.tenantID == '123' && $.amount > 100"
func (f *EventFilter) Matches(event interface{}) (bool, error) {
    vm := goja.New()
    
    // Convert event to JSON for easy access
    eventJSON, err := json.Marshal(event)
    if err != nil {
        return false, err
    }
    
    var eventMap map[string]interface{}
    json.Unmarshal(eventJSON, &eventMap)
    
    // Make event available as $
    vm.Set("$", eventMap)
    
    // Evaluate filter expression
    result, err := vm.RunString(f.expression)
    if err != nil {
        return false, err
    }
    
    return result.ToBoolean(), nil
}
```

### 7. Dead Letter Queue

Create `modules/scripts/services/dead_letter_queue.go`:

```go
type DeadLetterQueue struct {
    repo DeadLetterRepository
}

type DeadLetter struct {
    ID           uuid.UUID
    ScriptID     uuid.UUID
    Event        interface{}
    Error        string
    Timestamp    time.Time
    RetryCount   int
}

func (dlq *DeadLetterQueue) Send(task EventTask, err error) {
    letter := &DeadLetter{
        ID:         uuid.New(),
        ScriptID:   task.Subscription.ScriptID,
        Event:      task.Event,
        Error:      err.Error(),
        Timestamp:  time.Now(),
        RetryCount: task.RetryCount,
    }
    
    if err := dlq.repo.Create(context.Background(), letter); err != nil {
        log.Error("Failed to save to DLQ", "error", err)
    }
}

// Replay failed events
func (dlq *DeadLetterQueue) Replay(ctx context.Context, letterID uuid.UUID) error {
    letter, err := dlq.repo.GetByID(ctx, letterID)
    if err != nil {
        return err
    }
    
    // Re-queue the event
    return dlq.routerSvc.ReplayEvent(ctx, letter.Event, letter.ScriptID)
}
```

### 8. Event Script Examples

```javascript
// Example: Send SMS on payment received
const event = sdk.events.current();

if (event.amount > 10000) {
    const user = await sdk.db.query(
        "SELECT phone FROM users WHERE id = $1",
        [event.userID]
    );
    
    await sdk.sms.send({
        to: user.phone,
        message: `Payment of ${event.amount} received`
    });
    
    sdk.log.info("SMS sent for high-value payment", {
        userID: event.userID,
        amount: event.amount
    });
}

// Example: Update cache on entity change
const category = sdk.events.current();
await sdk.cache.delete(`category:${category.id}`);
await sdk.cache.delete('categories:list');

// Example: Webhook on order completion
const order = sdk.events.current();
await sdk.http.post('https://webhook.site/notify', {
    orderID: order.id,
    status: order.status,
    total: order.total
});
```

### 9. UI Components

Create `modules/scripts/presentation/templates/components/event_subscriptions.templ`:

```templ
templ EventSubscriptions(scriptID string, subscriptions []EventSubscriptionVM) {
    <div class="space-y-4">
        <h3 class="text-lg font-medium">Event Subscriptions</h3>
        
        <div class="space-y-2">
            for _, sub := range subscriptions {
                @EventSubscriptionRow(sub)
            }
        </div>
        
        <button 
            class="btn btn-secondary"
            hx-get={ fmt.Sprintf("/scripts/%s/subscriptions/new", scriptID) }
            hx-target="#modal"
        >
            Add Event Subscription
        </button>
    </div>
}

templ EventSubscriptionRow(sub EventSubscriptionVM) {
    <div class="flex items-center justify-between p-3 border rounded">
        <div>
            <div class="font-medium">{ sub.EventType }</div>
            if sub.Filter != "" {
                <div class="text-sm text-gray-600">Filter: { sub.Filter }</div>
            }
        </div>
        
        <div class="flex items-center gap-2">
            <span class={ 
                "px-2 py-1 text-xs rounded",
                templ.If(sub.Active, "bg-green-100 text-green-800", "bg-gray-100 text-gray-800"),
            }>
                { templ.If(sub.Active, "Active", "Inactive") }
            </span>
            
            <button
                class="btn btn-sm"
                hx-delete={ fmt.Sprintf("/scripts/subscriptions/%s", sub.ID) }
                hx-confirm="Remove this event subscription?"
            >
                Remove
            </button>
        </div>
    </div>
}
```

### 10. Monitoring & Metrics

Add to `modules/scripts/services/metrics.go`:

```go
var (
    eventProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "script_events_processed_total",
            Help: "Total number of events processed by scripts",
        },
        []string{"event_type", "script_id", "status"},
    )
    
    eventLatency = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "script_event_processing_duration_seconds",
            Help: "Event processing duration in seconds",
        },
        []string{"event_type", "script_id"},
    )
    
    eventQueueSize = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "script_event_queue_size",
            Help: "Current size of event processing queue",
        },
    )
)
```

## Testing Strategy

### Unit Tests
- Event filter matching
- Event buffer queuing
- Dead letter queue operations
- Event API functions

### Integration Tests
- End-to-end event flow
- Multiple subscribers
- Event replay
- Concurrent event processing

### Performance Tests
- High event volume
- Large number of subscribers
- Complex filter expressions
- Memory usage under load

## Security Considerations

1. **Event Data Access**: Scripts only see events for their tenant
2. **Filter Injection**: Validate filter expressions to prevent code injection
3. **Resource Limits**: Limit number of subscriptions per script
4. **Event Size**: Limit event payload size to prevent memory issues
5. **Replay Protection**: Audit and limit event replay operations

## Rollout Plan

1. Deploy event router without any subscriptions
2. Test with internal scripts first
3. Enable for selected tenants
4. Monitor performance and errors
5. Gradual rollout to all tenants
6. Enable event replay feature last