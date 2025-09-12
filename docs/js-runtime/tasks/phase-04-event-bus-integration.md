# Phase 4: Event Bus Integration - Architecture & Design

## Overview
This phase defines the architecture for integrating the IOTA SDK's event bus with the JavaScript runtime. This will enable scripts to be triggered in response to domain events, creating a powerful, event-driven automation capability.

## Background
- The system must reliably deliver events to the correct script handlers.
- The design should be resilient, with mechanisms for retries and handling failures (Dead Letter Queue).
- Event processing should not block the main application threads.
- The JavaScript API for handling events should be simple and intuitive.

## Task 4.1: Event Subscription and Routing Design

### Objectives
- Design the domain entities and repositories for managing event subscriptions.
- Design an `EventRouter` service to listen to the main event bus and delegate events to script handlers.
- Design a buffered queueing system to process events asynchronously.

### Detailed Design

#### 1. `EventSubscription` Entity
A new entity is required to store the link between a script and an event.

`modules/scripts/domain/entities/event_subscription.go`:
```go
package entities

import "github.com/google/uuid"

// EventSubscription represents a script's interest in a specific domain event.
type EventSubscription struct {
    ID         uuid.UUID
    ScriptID   uuid.UUID
    EventType  string      // e.g., "finance.PaymentCategoryCreated"
    Filter     string      // A simple expression to filter events, e.g., "$.amount > 100"
    IsActive   bool
    RetryPolicy RetryPolicy
}

// RetryPolicy defines how to handle failures for an event handler.
type RetryPolicy struct {
    MaxAttempts int
    // Could be extended with backoff strategy, etc.
}
```

#### 2. `EventSubscriptionRepository` Interface
`modules/scripts/domain/entities/event_subscription_repository.go`:
```go
package entities

// Repository defines the persistence contract for EventSubscription.
type Repository interface {
    Create(ctx context.Context, sub *EventSubscription) error
    Update(ctx context.Context, sub *EventSubscription) error
    Delete(ctx context.Context, id uuid.UUID) error
    FindByEventType(ctx context.Context, eventType string) ([]*EventSubscription, error)
}
```

#### 3. `EventRouter` Service
This service acts as the bridge between the IOTA event bus and the script execution engine.

`modules/scripts/services/event_router.go`:
```go
package services

// EventRouter listens to the global event bus and queues relevant events for script execution.
type EventRouter interface {
    // Start begins listening to the event bus.
    Start(ctx context.Context)
}
```
**Logic Flow**:
1. The `EventRouter` subscribes to all events on the main `eventbus.EventBus`.
2. For each incoming event, it determines the event type.
3. It queries the `EventSubscriptionRepository` to find all active subscriptions for that event type.
4. For each subscription, it evaluates the filter expression against the event payload.
5. If the filter matches, it enqueues an `EventTask` into a processing queue (e.g., an `EventBuffer`).

#### 4. `EventBuffer` and Worker Pool
To avoid blocking the event bus and to handle execution failures gracefully, events are processed by a worker pool via a buffered channel.

`modules/scripts/services/event_buffer.go`:
```go
package services

// EventTask represents a single event to be processed by a script.
type EventTask struct {
    Subscription *entities.EventSubscription
    EventData    interface{}
    RetryCount   int
}

// EventBuffer manages the asynchronous execution of event handler scripts.
type EventBuffer interface {
    Queue(task EventTask)
    Start(ctx context.Context)
}
```

## Task 4.2: Failure Handling and JavaScript API Design

### Objectives
- Design a Dead Letter Queue (DLQ) for events that repeatedly fail processing.
- Design the JavaScript API (`sdk.events`) that scripts will use to interact with the event system.

### Detailed Design

#### 1. Dead Letter Queue (DLQ)
The DLQ stores events that could not be processed after all retry attempts have been exhausted.

`modules/scripts/services/dead_letter_queue.go`:
```go
package services

// DeadLetter represents a failed event task.
type DeadLetter struct {
    ID           uuid.UUID
    Subscription *entities.EventSubscription
    EventData    interface{}
    LastError    string
    Timestamp    time.Time
}

// DeadLetterQueue provides a mechanism to store and manage failed event tasks.
type DeadLetterQueue interface {
    Send(dl DeadLetter) error
    Replay(ctx context.Context, id uuid.UUID) error // Re-queues the event for processing.
    List(ctx context.Context, params *repo.FindParams) ([]DeadLetter, error)
}
```

#### 2. JavaScript `sdk.events` API
This API provides the context for the currently executing event.

```typescript
// TypeScript definition for the events API
declare namespace sdk {
    namespace events {
        /**
         * Returns the domain event that triggered the current script execution.
         * The structure of the event object depends on the event type.
         */
        function current<T = any>(): T;

        /**
         * Emits a new custom event from within a script.
         * @param eventType The name of the custom event (e.g., "custom.my_event").
         * @param data The payload for the event.
         */
        function emit(eventType: string, data: object): Promise<void>;
    }
}
```

## Task 4.3: Implementation Task Breakdown

### Domain and Persistence
- [ ] Implement the `EventSubscription` entity and its repository.
- [ ] Add the necessary tables to the `scripts-schema.sql` file for subscriptions and the dead letter queue.
- [ ] Write integration tests for the `EventSubscriptionRepository`.

### Services and Routing
- [ ] Implement the `EventRouter` service to listen to the event bus and filter events.
- [ ] Implement the `EventBuffer` with a worker pool for asynchronous processing of `EventTask`s.
- [ ] Implement the `DeadLetterQueue` service and its repository.
- [ ] Integrate the `EventRouter`, `EventBuffer`, and `DeadLetterQueue` into the `ExecutionService`.

### JavaScript API
- [ ] Implement the `sdk.events` API binding.
- [ ] Ensure the `current()` function correctly provides the event payload to the script.
- [ ] Implement the `emit()` function to publish new events onto the main IOTA event bus.

### UI and Monitoring
- [ ] Create UI components in the script editor to manage a script's event subscriptions.
- [ ] Create a UI to view and manage the Dead Letter Queue (list, view details, replay).
- [ ] Add Prometheus metrics for event processing (e.g., events processed, queue size, DLQ size, processing latency).

### Deliverables Checklist
- [x] Finalized design for event subscriptions, routing, and failure handling.
- [x] Defined the JavaScript `sdk.events` API.
- [x] A detailed task list for the implementation of this phase.
- [x] Documentation for the event bus integration and how to create event handler scripts.

## Success Criteria
- The system can reliably trigger scripts from domain events.
- Event processing is asynchronous and does not impact application performance.
- Failed event executions are handled gracefully with retries and a DLQ.
- The JavaScript API for handling events is intuitive and well-documented.
- The entire event flow is covered by integration tests.
