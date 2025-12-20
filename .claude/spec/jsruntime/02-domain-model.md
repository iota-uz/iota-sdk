# JavaScript Runtime - Domain Model

## Overview

The JavaScript Runtime domain model follows DDD principles with aggregates as interfaces, entities with immutable setters, value objects for type safety, and domain events for lifecycle tracking.

```mermaid
graph TB
    subgraph "Aggregates"
        Script[Script Aggregate<br/>Root Entity]
    end

    subgraph "Entities"
        Execution[Execution Entity<br/>Script Run]
        Version[Version Entity<br/>Audit Trail]
    end

    subgraph "Value Objects"
        ScriptType[ScriptType<br/>Enum]
        ScriptStatus[ScriptStatus<br/>Enum]
        ExecutionStatus[ExecutionStatus<br/>Enum]
        TriggerType[TriggerType<br/>Enum]
        ResourceLimits[ResourceLimits<br/>Struct]
        CronExpression[CronExpression<br/>Validated]
        TriggerData[TriggerData<br/>Struct]
        ExecutionMetrics[ExecutionMetrics<br/>Struct]
    end

    subgraph "Domain Events"
        ScriptEvents[Script Events<br/>Created, Updated, Deleted]
        ExecutionEvents[Execution Events<br/>Started, Completed, Failed]
    end

    Script --> ScriptType
    Script --> ScriptStatus
    Script --> ResourceLimits
    Script --> CronExpression
    Script --> Version

    Execution --> ExecutionStatus
    Execution --> TriggerType
    Execution --> TriggerData
    Execution --> ExecutionMetrics

    Script -.publishes.-> ScriptEvents
    Execution -.publishes.-> ExecutionEvents
```

## Script Aggregate

**What It Is:**
The Script aggregate is the root entity representing a user-defined JavaScript program with metadata, resource limits, and trigger configuration. It serves as the consistency boundary for all script-related operations.

**Core Attributes:**
- **Identity**: ID (UUID), TenantID, OrganizationID
- **Basic Properties**: Name, Description, Source code
- **Classification**: Type (scheduled/HTTP/event/oneoff/embedded), Status (draft/active/paused/disabled/archived)
- **Resource Management**: ResourceLimits (timeout, memory, concurrency, rate limits)
- **Trigger Configuration**: CronExpression (for scheduled), HTTPPath/HTTPMethods (for HTTP endpoints), EventTypes (for event-driven)
- **Metadata**: Key-value pairs for tagging, custom configuration
- **Audit Trail**: CreatedAt, UpdatedAt, CreatedBy

```mermaid
classDiagram
    class Script {
        <<interface>>
        +ID() UUID
        +TenantID() UUID
        +OrganizationID() UUID
        +Name() string
        +Description() string
        +Source() string
        +Type() ScriptType
        +Status() ScriptStatus
        +ResourceLimits() ResourceLimits
        +CronExpression() CronExpression
        +HTTPPath() string
        +HTTPMethods() []string
        +EventTypes() []string
        +Metadata() map[string]string
        +Tags() []string
        +CreatedAt() Time
        +UpdatedAt() Time
        +CreatedBy() uint
        +SetName(string) Script
        +SetDescription(string) Script
        +SetSource(string) Script
        +SetStatus(ScriptStatus) Script
        +SetResourceLimits(ResourceLimits) Script
        +CanExecute() bool
        +Validate() error
        +IsScheduled() bool
        +IsHTTPEndpoint() bool
        +IsEventTriggered() bool
    }

    class ScriptType {
        <<enumeration>>
        Scheduled
        HTTP
        Event
        OneOff
        Embedded
    }

    class ScriptStatus {
        <<enumeration>>
        Draft
        Active
        Paused
        Disabled
        Archived
    }

    class ResourceLimits {
        +MaxExecutionTime Duration
        +MaxMemoryBytes int64
        +MaxConcurrentRuns int
        +MaxAPICallsPerMinute int
        +MaxOutputSizeBytes int64
        +Validate() error
    }

    Script --> ScriptType
    Script --> ScriptStatus
    Script --> ResourceLimits
```

**Business Rules:**
- Name and source code are required (non-empty)
- Tenant ID is required (multi-tenant isolation)
- Type-specific validation enforced:
  - **Scheduled scripts** must have valid cron expression
  - **HTTP endpoint scripts** must have HTTP path defined
  - **Event-triggered scripts** must have at least one event type
- Only scripts with `Active` status can execute
- All setters return new instance (immutability pattern)

**Behavior:**
- `CanExecute()` - Returns true only when status is Active
- `Validate()` - Enforces all business rules before persistence
- `IsScheduled()` / `IsHTTPEndpoint()` / `IsEventTriggered()` - Type checking helpers
- Functional options pattern for flexible construction
- Immutable setters ensure state changes create new instances

## Execution Entity

**What It Is:**
The Execution entity represents a single run of a script with input, output, status, metrics, and timing information. It provides complete traceability for script executions.

**Core Attributes:**
- **Identity**: ID (UUID), ScriptID (reference), TenantID
- **Status Tracking**: ExecutionStatus (pending/running/completed/failed/timeout/cancelled)
- **Trigger Information**: TriggerType, TriggerData (event payload, HTTP request, cron trigger)
- **Input/Output**: Input parameters (map), Output result (any type), Error message (if failed)
- **Performance Metrics**: Duration, memory usage, API call count, database query count
- **Timestamps**: StartedAt, CompletedAt (nullable until finished)

```mermaid
stateDiagram-v2
    [*] --> Pending: Created
    Pending --> Running: Execution starts
    Running --> Completed: Success
    Running --> Failed: Error
    Running --> Timeout: Time limit exceeded
    Running --> Cancelled: User cancellation

    Completed --> [*]
    Failed --> [*]
    Timeout --> [*]
    Cancelled --> [*]
```

**Business Rules:**
- Execution starts in `Pending` status
- Status transitions must follow valid state machine
- Duration calculated from start to completion (or current time if running)
- CompletedAt is null until execution finishes
- Metrics captured at completion (duration, memory, API calls)

**Behavior:**
- `IsRunning()` - Check if execution is in progress
- `IsCompleted()` - Check if execution finished successfully
- `IsFailed()` - Check if execution ended in failure or timeout
- `Duration()` - Calculate execution duration (completed time - started time, or now - started time if still running)
- Immutable setters for status, output, error, metrics, completion time

```mermaid
classDiagram
    class Execution {
        <<interface>>
        +ID() UUID
        +ScriptID() UUID
        +TenantID() UUID
        +Status() ExecutionStatus
        +TriggerType() TriggerType
        +TriggerData() TriggerData
        +Input() map[string]interface
        +Output() interface
        +Error() string
        +Metrics() ExecutionMetrics
        +StartedAt() Time
        +CompletedAt() *Time
        +SetStatus(ExecutionStatus) Execution
        +SetOutput(interface) Execution
        +SetError(string) Execution
        +SetMetrics(ExecutionMetrics) Execution
        +SetCompletedAt(Time) Execution
        +IsRunning() bool
        +IsCompleted() bool
        +IsFailed() bool
        +Duration() Duration
    }

    class ExecutionStatus {
        <<enumeration>>
        Pending
        Running
        Completed
        Failed
        Timeout
        Cancelled
    }

    class TriggerType {
        <<enumeration>>
        Cron
        HTTP
        Event
        Manual
        API
    }

    class ExecutionMetrics {
        +Duration Duration
        +MemoryUsedBytes int64
        +APICallCount int
        +DatabaseQueryCount int
    }

    Execution --> ExecutionStatus
    Execution --> TriggerType
    Execution --> ExecutionMetrics
```

## Version Entity

**What It Is:**
The Version entity provides an immutable audit trail of script source code changes. Each time a script is updated, a new version is created with the complete source snapshot.

**Core Attributes:**
- **Identity**: ID (UUID), ScriptID (reference), TenantID
- **Version Tracking**: VersionNumber (auto-increment), Source (complete code snapshot)
- **Change Tracking**: ChangeDescription (human-readable explanation)
- **Audit Fields**: CreatedAt, CreatedBy (user who made the change)

**Business Rules:**
- Version is immutable (no setters, only getters)
- Version number increments sequentially for each script
- Complete source code stored for rollback capability
- Change description optional but recommended for audit clarity

**Behavior:**
- Read-only entity (no state mutations)
- Provides historical record for compliance and debugging
- Enables script rollback to previous versions
- Supports diff operations between versions

```mermaid
classDiagram
    class Version {
        <<interface>>
        +ID() UUID
        +ScriptID() UUID
        +TenantID() UUID
        +VersionNumber() int
        +Source() string
        +ChangeDescription() string
        +CreatedAt() Time
        +CreatedBy() uint
    }

    class Script {
        +ID() UUID
    }

    Version --> Script: references
```

## Value Objects

### ScriptType Enum

**What It Defines:**
Classification of scripts by their execution trigger mechanism.

```mermaid
graph LR
    ScriptType --> Scheduled[Scheduled<br/>Cron-based execution]
    ScriptType --> HTTP[HTTP<br/>API endpoint]
    ScriptType --> Event[Event<br/>EventBus-triggered]
    ScriptType --> OneOff[OneOff<br/>Manual execution]
    ScriptType --> Embedded[Embedded<br/>Programmatic invocation]
```

**Values:**
- **Scheduled**: Executed on cron schedule (e.g., daily reports)
- **HTTP**: Executed on HTTP request (e.g., webhooks)
- **Event**: Executed on domain events (e.g., user.created)
- **OneOff**: Manually triggered via UI/API (e.g., data migration)
- **Embedded**: Programmatically invoked from Go code (e.g., custom validation)

**Validation:**
- `IsValid()` method checks if value is one of the defined types

### ScriptStatus Enum

**What It Defines:**
Lifecycle state of a script.

```mermaid
stateDiagram-v2
    [*] --> Draft: Created
    Draft --> Active: Deployed
    Active --> Paused: Temporarily disabled
    Paused --> Active: Re-enabled
    Active --> Disabled: Permanently disabled
    Disabled --> Archived: Historical record
    Archived --> [*]
```

**Values:**
- **Draft**: Being edited, not yet deployable
- **Active**: Running and executable, available for triggers
- **Paused**: Temporarily disabled, can be re-enabled
- **Disabled**: Permanently disabled, manual intervention required to reactivate
- **Archived**: Historical record only, cannot be re-enabled

**Validation:**
- `IsValid()` method checks if value is one of the defined states

### ExecutionStatus Enum

**What It Defines:**
Current state of a script execution.

**Values:**
- **Pending**: Queued for execution, waiting for VM availability
- **Running**: Currently executing in VM
- **Completed**: Successful completion with output
- **Failed**: Error during execution (exception, validation failure)
- **Timeout**: Exceeded maximum execution time
- **Cancelled**: Manually stopped by user

**Validation:**
- `IsValid()` method checks if value is one of the defined states

### TriggerType Enum

**What It Defines:**
Mechanism that initiated a script execution.

**Values:**
- **Cron**: Scheduled execution via cron scheduler
- **HTTP**: HTTP request to registered endpoint
- **Event**: Domain event from EventBus
- **Manual**: User-initiated execution via UI
- **API**: Programmatic invocation from Go code

### ResourceLimits Struct

**What It Defines:**
Constraints on script execution to prevent resource exhaustion and ensure fair usage across tenants.

```mermaid
graph TB
    subgraph "Resource Limits"
        MaxExecTime[Max Execution Time<br/>Default: 30s]
        MaxMemory[Max Memory<br/>Default: 64MB]
        MaxConcurrent[Max Concurrent Runs<br/>Default: 5]
        MaxAPICalls[Max API Calls/Min<br/>Default: 60]
        MaxOutputSize[Max Output Size<br/>Default: 1MB]
    end

    VM[Script VM] --> MaxExecTime
    VM --> MaxMemory
    VM --> MaxConcurrent
    VM --> MaxAPICalls
    VM --> MaxOutputSize

    style MaxExecTime fill:#FFE4E1
    style MaxMemory fill:#FFE4B5
    style MaxConcurrent fill:#E0FFE0
    style MaxAPICalls fill:#E0F2FF
    style MaxOutputSize fill:#E6E6FA
```

**Fields:**
- **MaxExecutionTime**: Maximum duration for script execution (timeout enforcement)
- **MaxMemoryBytes**: Maximum memory allocation per execution
- **MaxConcurrentRuns**: Maximum parallel executions of same script
- **MaxAPICallsPerMinute**: Rate limit for API calls (database, HTTP client)
- **MaxOutputSizeBytes**: Maximum size of execution output

**Defaults:**
- Execution time: 30 seconds
- Memory: 64 MB
- Concurrent runs: 5
- API calls: 60 per minute
- Output size: 1 MB

**Validation:**
- All numeric values must be positive
- Enforced at VM execution time
- Customizable per script or tenant

### CronExpression Value Object

**What It Is:**
Validated cron expression for scheduled script execution.

**Behavior:**
- Parses and validates cron syntax using `robfig/cron` library
- Supports standard 5-field cron format (minute, hour, day, month, weekday)
- Calculates next execution time from current time
- Immutable once created

**Validation:**
- Syntax validation on creation
- Invalid expressions rejected with error
- Prevents storage of malformed cron expressions

**Methods:**
- `String()` - Returns original expression
- `Next(time.Time)` - Calculates next execution time after given time

### TriggerData Value Object

**What It Is:**
Context information about what triggered a script execution.

**Fields:**
- **EventType**: Name of domain event (for event-triggered)
- **HTTPMethod**: HTTP method (GET, POST, etc.) for HTTP-triggered
- **HTTPPath**: HTTP path for HTTP-triggered
- **CronTrigger**: Cron expression that triggered execution
- **Payload**: Additional context data (event payload, HTTP request body)

**Usage:**
- Provides full context to script about its trigger
- Available to script via `context.trigger` object
- Enables scripts to handle different trigger types

### ExecutionMetrics Struct

**What It Is:**
Performance and resource usage metrics captured during script execution.

**Fields:**
- **Duration**: Total execution time (start to completion)
- **MemoryUsedBytes**: Peak memory consumption
- **APICallCount**: Number of API calls made (database, HTTP)
- **DatabaseQueryCount**: Number of database queries executed

**Usage:**
- Captured automatically by VM execution engine
- Stored with execution record for analysis
- Used for performance monitoring and optimization
- Enables billing based on resource consumption

## Domain Events

### Script Lifecycle Events

```mermaid
sequenceDiagram
    participant User
    participant ScriptService
    participant EventBus
    participant Subscribers

    User->>ScriptService: Create script
    ScriptService->>EventBus: Publish ScriptCreatedEvent
    EventBus->>Subscribers: Notify (tenant_id filtered)

    User->>ScriptService: Update script
    ScriptService->>EventBus: Publish ScriptUpdatedEvent
    EventBus->>Subscribers: Notify (tenant_id filtered)

    User->>ScriptService: Change status
    ScriptService->>EventBus: Publish ScriptStatusChangedEvent
    EventBus->>Subscribers: Notify (tenant_id filtered)

    User->>ScriptService: Delete script
    ScriptService->>EventBus: Publish ScriptDeletedEvent
    EventBus->>Subscribers: Notify (tenant_id filtered)
```

**Event Types:**
- **ScriptCreatedEvent**: `jsruntime.script.created`
  - Payload: ScriptID, TenantID, Name, Type, CreatedBy, CreatedAt
  - Published when new script is created

- **ScriptUpdatedEvent**: `jsruntime.script.updated`
  - Payload: ScriptID, TenantID, UpdatedBy, UpdatedAt, VersionNumber
  - Published when script source or configuration changes

- **ScriptDeletedEvent**: `jsruntime.script.deleted`
  - Payload: ScriptID, TenantID, DeletedBy, DeletedAt
  - Published when script is permanently removed

- **ScriptStatusChangedEvent**: `jsruntime.script.status_changed`
  - Payload: ScriptID, TenantID, OldStatus, NewStatus, ChangedBy, ChangedAt
  - Published when script status transitions (draft → active, active → paused, etc.)

### Execution Lifecycle Events

```mermaid
sequenceDiagram
    participant Trigger
    participant ExecutionService
    participant EventBus
    participant Monitoring

    Trigger->>ExecutionService: Start execution
    ExecutionService->>EventBus: Publish ExecutionStartedEvent
    EventBus->>Monitoring: Log start

    alt Success
        ExecutionService->>EventBus: Publish ExecutionCompletedEvent
        EventBus->>Monitoring: Log completion
    else Failure
        ExecutionService->>EventBus: Publish ExecutionFailedEvent
        EventBus->>Monitoring: Log failure & alert
    else Timeout
        ExecutionService->>EventBus: Publish ExecutionTimeoutEvent
        EventBus->>Monitoring: Log timeout & alert
    end
```

**Event Types:**
- **ExecutionStartedEvent**: `jsruntime.execution.started`
  - Payload: ExecutionID, ScriptID, TenantID, TriggerType, StartedAt
  - Published when script execution begins

- **ExecutionCompletedEvent**: `jsruntime.execution.completed`
  - Payload: ExecutionID, ScriptID, TenantID, Duration, CompletedAt
  - Published on successful execution

- **ExecutionFailedEvent**: `jsruntime.execution.failed`
  - Payload: ExecutionID, ScriptID, TenantID, Error, FailedAt
  - Published when execution encounters error

- **ExecutionTimeoutEvent**: `jsruntime.execution.timeout`
  - Payload: ExecutionID, ScriptID, TenantID, Duration, TimeoutAt
  - Published when execution exceeds time limit

**All Events Include:**
- Tenant ID for multi-tenant event filtering
- Timestamps for chronological ordering
- Relevant entity IDs for correlation

## Acceptance Criteria

### Script Aggregate
- ✅ Implements all getter methods for attributes
- ✅ All setters return new instance (immutability pattern)
- ✅ `Validate()` enforces business rules (name, source, tenant ID required)
- ✅ Type-specific validation (cron for scheduled, path for HTTP, events for event-triggered)
- ✅ `CanExecute()` returns true only when status is Active
- ✅ Constructor uses functional options pattern
- ✅ Private struct implementation, public interface contract
- ✅ Zero external dependencies in domain layer

### Execution Entity
- ✅ Tracks execution lifecycle (pending → running → completed/failed)
- ✅ Stores input, output, error message, performance metrics
- ✅ Immutable setters for status, output, error, metrics, completion time
- ✅ `Duration()` calculates elapsed time (completed - started, or now - started)
- ✅ Business methods for status checks (IsRunning, IsCompleted, IsFailed)
- ✅ Supports all trigger types (cron, HTTP, event, manual, API)

### Version Entity
- ✅ Immutable audit trail (no setters, only getters)
- ✅ Version number increments on each script change
- ✅ Stores complete source code snapshot for rollback
- ✅ Change description for human-readable audit trail
- ✅ Created by user ID for accountability

### Value Objects
- ✅ ScriptType enum with 5 types (Scheduled, HTTP, Event, OneOff, Embedded)
- ✅ ScriptStatus enum with 5 states (Draft, Active, Paused, Disabled, Archived)
- ✅ ExecutionStatus enum with 6 states (Pending, Running, Completed, Failed, Timeout, Cancelled)
- ✅ TriggerType enum with 5 types (Cron, HTTP, Event, Manual, API)
- ✅ ResourceLimits with sensible defaults (30s timeout, 64MB memory, 5 concurrent, 60 API calls/min, 1MB output)
- ✅ CronExpression validated using `robfig/cron` parser
- ✅ All enums implement `IsValid()` method for validation

### Domain Events
- ✅ Events for script lifecycle (created, updated, deleted, status changed)
- ✅ Events for execution lifecycle (started, completed, failed, timeout)
- ✅ All events include tenant ID for multi-tenant event filtering
- ✅ Event types follow naming convention: `jsruntime.{entity}.{action}`
- ✅ Events published via EventBus for decoupled system integration
