# JavaScript Runtime - Service Layer

## Overview

The service layer orchestrates business logic, coordinates between repositories, manages VM execution, and enforces permissions and validation rules.

```mermaid
graph TB
    subgraph "Service Layer"
        ScriptService[Script Service]
        ExecutionService[Execution Service]
        VMPoolService[VM Pool Service]
        SchedulerService[Scheduler Service]
        EventHandlerService[Event Handler Service]
    end

    subgraph "Domain Layer"
        ScriptRepo[ScriptRepository]
        ExecutionRepo[ExecutionRepository]
        VersionRepo[VersionRepository]
    end

    subgraph "Infrastructure"
        VMPool[VM Pool]
        EventBus[Event Bus]
        Cron[Cron Ticker]
    end

    ScriptService --> ScriptRepo
    ScriptService --> VersionRepo
    ScriptService --> ExecutionService
    ExecutionService --> ExecutionRepo
    ExecutionService --> VMPoolService
    VMPoolService --> VMPool
    SchedulerService --> ScriptRepo
    SchedulerService --> ExecutionService
    SchedulerService --> Cron
    EventHandlerService --> ScriptRepo
    EventHandlerService --> ExecutionService
    EventHandlerService --> EventBus
```

## Script Service

**What It Does:**
Manages script lifecycle (CRUD), versioning, validation, and permissions enforcement.

**Responsibilities:**
- Create/update/delete scripts with business validation
- Automatic version creation on updates
- Permission checks via RBAC
- Name and HTTP path uniqueness validation
- Type-specific validation (cron, HTTP, event requirements)

```mermaid
sequenceDiagram
    participant Controller
    participant ScriptService
    participant ScriptRepo
    participant VersionRepo
    participant EventBus

    Controller->>ScriptService: Create(ctx, createDTO)
    ScriptService->>ScriptService: Validate(name, source, type)

    alt Script type = scheduled
        ScriptService->>ScriptService: Validate cron expression
    else Script type = http
        ScriptService->>ScriptService: Validate HTTP path
    else Script type = event
        ScriptService->>ScriptService: Validate event types
    end

    ScriptService->>ScriptRepo: NameExists(name)
    ScriptRepo-->>ScriptService: false

    ScriptService->>ScriptService: Build script entity
    ScriptService->>ScriptRepo: Create(script)
    ScriptService->>VersionRepo: Create(version 1)
    ScriptService->>EventBus: Publish ScriptCreatedEvent

    ScriptService-->>Controller: Created script
```

### Update Flow with Versioning

**What It Does:**
Updates script and creates immutable version snapshot automatically.

**How It Works:**
1. Retrieve existing script by ID
2. Validate update permissions
3. Apply updates to script entity
4. Validate updated script
5. Get next version number
6. Create new version record
7. Update script in database
8. Publish ScriptUpdatedEvent

```mermaid
sequenceDiagram
    participant Controller
    participant ScriptService
    participant ScriptRepo
    participant VersionRepo
    participant EventBus

    Controller->>ScriptService: Update(ctx, id, updateDTO)
    ScriptService->>ScriptRepo: GetByID(id)
    ScriptRepo-->>ScriptService: Existing script

    ScriptService->>ScriptService: Check permissions (canUpdate)
    ScriptService->>ScriptService: Apply updates
    ScriptService->>ScriptService: Validate updated script

    ScriptService->>VersionRepo: GetNextVersionNumber(scriptID)
    VersionRepo-->>ScriptService: nextVersion

    ScriptService->>VersionRepo: Create(new version)
    ScriptService->>ScriptRepo: Update(script)
    ScriptService->>EventBus: Publish ScriptUpdatedEvent

    ScriptService-->>Controller: Updated script
```

## Execution Service

**What It Does:**
Orchestrates script execution, manages execution lifecycle, tracks metrics, and handles errors.

**Responsibilities:**
- Create execution records
- Acquire VM from pool
- Execute script with timeout
- Capture output and metrics
- Handle errors and retries (event-triggered only)
- Update execution status

```mermaid
stateDiagram-v2
    [*] --> Creating: Execute(scriptID)
    Creating --> Pending: Create execution record
    Pending --> Running: Acquire VM
    Running --> Completed: Success
    Running --> Failed: Error
    Running --> Timeout: Time limit exceeded

    Completed --> [*]
    Failed --> DeadLetter: Event-triggered
    Failed --> [*]: Other triggers
    Timeout --> [*]
```

### Execution Flow

**How It Works:**
1. Validate script exists and is active
2. Create execution record (status: pending)
3. Acquire VM from pool
4. Update status to running
5. Execute script in VM with timeout
6. Capture output and metrics
7. Update status to completed/failed
8. Release VM back to pool
9. Publish ExecutionCompletedEvent/ExecutionFailedEvent

```mermaid
sequenceDiagram
    participant Trigger
    participant ExecutionService
    participant ExecutionRepo
    participant VMPoolService
    participant VMPool

    Trigger->>ExecutionService: Execute(scriptID, input)
    ExecutionService->>ExecutionRepo: Create(execution{status:pending})

    ExecutionService->>VMPoolService: Acquire(tenantID)
    VMPoolService->>VMPool: Get or create VM
    VMPool-->>VMPoolService: VM instance
    VMPoolService-->>ExecutionService: VM

    ExecutionService->>ExecutionRepo: Update(status:running)
    ExecutionService->>VMPool: Run(source, input, timeout)

    alt Success
        VMPool-->>ExecutionService: Output + metrics
        ExecutionService->>ExecutionRepo: Update(status:completed)
    else Error
        VMPool-->>ExecutionService: Error
        ExecutionService->>ExecutionRepo: Update(status:failed)
    end

    ExecutionService->>VMPoolService: Release(VM)
    ExecutionService-->>Trigger: Execution result
```

## VM Pool Service

**What It Does:**
Manages VM lifecycle, pool size, warm-up, acquisition, and resource limits.

**Responsibilities:**
- Initialize VM pool on startup
- Warm up VMs with standard library
- Acquire VM for tenant (create if needed)
- Release VM back to pool
- Enforce per-tenant VM limits
- Cleanup idle VMs
- Monitor pool metrics

```mermaid
graph TB
    subgraph "VM Pool Architecture"
        Available[Available VMs<br/>Pre-warmed]
        InUse[In-Use VMs<br/>Executing scripts]
        Creating[Creating VMs<br/>On-demand expansion]
    end

    subgraph "VM Lifecycle"
        Create[Create VM]
        Warmup[Load Standard Library]
        AddToPool[Add to Available Pool]
        Acquire[Acquire for Execution]
        Execute[Execute Script]
        Reset[Reset State]
        Release[Release to Pool]
        Cleanup[Cleanup if Idle]
    end

    Create --> Warmup
    Warmup --> AddToPool
    AddToPool --> Available
    Available --> Acquire
    Acquire --> InUse
    InUse --> Execute
    Execute --> Reset
    Reset --> Release
    Release --> Available
    Available --> Cleanup

    style Available fill:#90EE90
    style InUse fill:#FFB6C1
    style Creating fill:#E0F2FF
```

### Pool Management Strategy

**What It Does:**
Balances VM availability with resource usage through dynamic pool sizing.

**Strategy:**
- **Initial Pool Size**: 10 VMs (configurable)
- **Expansion**: Create new VM if pool empty (up to max: 100)
- **Per-Tenant Limit**: Max 5 concurrent VMs per tenant
- **Idle Timeout**: 5 minutes of inactivity → VM destroyed
- **Warm-up**: Pre-load standard library and common APIs
- **Fair Scheduling**: Round-robin across tenants

```mermaid
sequenceDiagram
    participant Service
    participant VMPoolService
    participant Pool as VM Pool

    Service->>VMPoolService: Acquire(tenantID)

    alt VM available in pool
        VMPoolService->>Pool: Get available VM
        Pool-->>VMPoolService: VM instance
    else Pool empty
        alt Below max pool size
            VMPoolService->>VMPoolService: Create new VM
            VMPoolService->>VMPoolService: Warm up VM
            VMPoolService-->>Service: New VM
        else Max pool size reached
            VMPoolService-->>Service: Error: Pool exhausted
        end
    end

    alt Below per-tenant limit
        VMPoolService-->>Service: VM instance
    else Tenant limit reached
        VMPoolService-->>Service: Error: Tenant limit
    end
```

## Scheduler Service

**What It Does:**
Manages cron-based script execution with next run calculation and overlap prevention.

**Responsibilities:**
- Find scripts due to run every minute
- Calculate next run time from cron expression
- Prevent overlapping executions via lock
- Trigger script execution
- Update next run time and last run status

```mermaid
sequenceDiagram
    participant Ticker as Cron Ticker
    participant Scheduler as Scheduler Service
    participant ScriptRepo
    participant JobsTable as script_scheduled_jobs
    participant ExecutionService

    loop Every minute
        Ticker->>Scheduler: Tick
        Scheduler->>JobsTable: SELECT jobs WHERE next_run_at <= NOW()<br/>AND is_running = false

        loop For each due job
            Scheduler->>JobsTable: UPDATE is_running = true
            Scheduler->>ScriptRepo: GetByID(scriptID)
            ScriptRepo-->>Scheduler: Script

            Scheduler->>ExecutionService: Execute(script, trigger:cron)

            alt Execution completed
                Scheduler->>Scheduler: Calculate next_run_at from cron
                Scheduler->>JobsTable: UPDATE (next_run_at, last_run_status, is_running=false)
            else Execution failed
                Scheduler->>JobsTable: UPDATE (last_run_status='failed', is_running=false)
            end
        end
    end
```

### Cron Expression Calculation

**What It Does:**
Calculates next execution time based on cron expression and timezone.

**Supported Patterns:**
- Standard 5-field cron (minute, hour, day, month, weekday)
- Timezone support (default: UTC)
- Handles daylight saving time transitions

**Examples:**
- `0 0 * * *` - Daily at midnight
- `*/15 * * * *` - Every 15 minutes
- `0 9 * * 1-5` - Weekdays at 9 AM
- `0 0 1 * *` - First day of month

## Event Handler Service

**What It Does:**
Subscribes to domain events and triggers matching scripts with retry logic and dead letter queue.

**Responsibilities:**
- Subscribe to all domain events via EventBus
- Find scripts subscribed to event type
- Trigger script execution with event payload
- Retry failed executions with exponential backoff
- Move persistent failures to dead letter queue

```mermaid
graph TB
    subgraph "Event Flow"
        DomainService[Domain Service]
        EventBus[Event Bus]
        EventHandler[Event Handler Service]
    end

    subgraph "Script Resolution"
        SubsTable[(script_event_subscriptions)]
        Scripts[(scripts)]
    end

    subgraph "Execution"
        ExecutionService[Execution Service]
        VMPool[VM Pool]
    end

    subgraph "Failure Handling"
        Retry[Retry Logic<br/>Exponential Backoff]
        DLQ[(Dead Letter Queue)]
    end

    DomainService -->|Publish event| EventBus
    EventBus -->|Notify| EventHandler
    EventHandler -->|Query by event_type| SubsTable
    SubsTable -->|Join| Scripts
    Scripts -->|Subscribed scripts| EventHandler
    EventHandler --> ExecutionService
    ExecutionService --> VMPool

    VMPool -.Failure.-> Retry
    Retry -.Max retries.-> DLQ
    Retry -.Backoff.-> ExecutionService
```

### Retry Strategy

**What It Does:**
Automatically retries failed event-triggered executions with exponential backoff.

**Strategy:**
- **Max Retries**: 3 attempts
- **Backoff**: 1s, 2s, 4s (exponential)
- **Dead Letter**: After 3 failures, move to DLQ
- **Manual Review**: DLQ entries require manual intervention

```mermaid
sequenceDiagram
    participant EventHandler
    participant ExecutionService
    participant DLQ as Dead Letter Queue

    EventHandler->>ExecutionService: Execute (Attempt 1)
    ExecutionService-->>EventHandler: Failed

    Note over EventHandler: Wait 1 second

    EventHandler->>ExecutionService: Execute (Attempt 2)
    ExecutionService-->>EventHandler: Failed

    Note over EventHandler: Wait 2 seconds

    EventHandler->>ExecutionService: Execute (Attempt 3)
    ExecutionService-->>EventHandler: Failed

    Note over EventHandler: Wait 4 seconds

    EventHandler->>ExecutionService: Execute (Attempt 4 - Final)
    ExecutionService-->>EventHandler: Failed

    EventHandler->>DLQ: INSERT dead letter<br/>(event_type, payload, error, retry_count:3)
```

## Business Rules and Validation

**Script Creation:**
- Name required (non-empty)
- Source code required (non-empty)
- Name must be unique per tenant
- Type-specific validation:
  - Scheduled: Valid cron expression required
  - HTTP: HTTP path required and unique per tenant
  - Event: At least one event type required

**Script Update:**
- User has permission to update script
- Name uniqueness maintained (if changed)
- HTTP path uniqueness maintained (if changed)
- Version created automatically

**Script Execution:**
- Script must exist and be active
- User has permission to execute script (for manual triggers)
- Resource limits enforced (timeout, memory, concurrency)

**Permission Checks:**
- `canCreateScript` - Create new scripts
- `canUpdateScript` - Modify existing scripts
- `canDeleteScript` - Remove scripts
- `canExecuteScript` - Manually trigger scripts
- `canViewExecutions` - View execution history

```mermaid
graph TB
    subgraph "Validation Layers"
        Input[Input Validation<br/>Required fields]
        Business[Business Rules<br/>Uniqueness, type-specific]
        Permissions[RBAC Permissions<br/>canCreate, canUpdate]
        ResourceLimits[Resource Limits<br/>Timeout, memory, concurrency]
    end

    Request[Service Request] --> Input
    Input --> Business
    Business --> Permissions
    Permissions --> ResourceLimits
    ResourceLimits --> Success[Operation Succeeds]

    Input -.Invalid.-> ValidationError
    Business -.Violation.-> BusinessError
    Permissions -.Denied.-> PermissionError
    ResourceLimits -.Exceeded.-> LimitError
```

## Error Handling

**What It Does:**
Consistent error handling across all service methods using `serrors` package.

**Pattern:**
- Define operation constant: `const op serrors.Op = "ServiceName.MethodName"`
- Wrap all errors: `return serrors.E(op, err)`
- Use error kinds: `serrors.KindValidation`, `serrors.KindNotFound`, `serrors.KindPermission`
- Provide context: `serrors.E(op, serrors.KindValidation, "name is required")`

**Error Propagation:**
- Repository errors wrapped with operation context
- Validation errors include field name and reason
- Permission errors include required permission
- Business rule errors include constraint violated

## Acceptance Criteria

### Script Service
- ✅ CRUD operations with validation and permissions
- ✅ Automatic versioning on create and update
- ✅ Name uniqueness validation per tenant
- ✅ HTTP path uniqueness validation per tenant
- ✅ Type-specific validation (cron, HTTP, event)
- ✅ Permission checks via RBAC (canCreate, canUpdate, canDelete)
- ✅ Event publishing (ScriptCreated, ScriptUpdated, ScriptDeleted)

### Execution Service
- ✅ Execute method coordinates full execution lifecycle
- ✅ Status transitions (pending → running → completed/failed)
- ✅ VM acquisition and release
- ✅ Timeout enforcement via context
- ✅ Metrics capture (duration, memory, API calls)
- ✅ Error handling and logging
- ✅ Event publishing (ExecutionStarted, ExecutionCompleted, ExecutionFailed)

### VM Pool Service
- ✅ Initialize pool with configurable size
- ✅ Warm up VMs with standard library
- ✅ Acquire VM with per-tenant limits
- ✅ Release VM and reset state
- ✅ Cleanup idle VMs after timeout
- ✅ Metrics tracking (available, in-use, total)
- ✅ Graceful shutdown with drain period

### Scheduler Service
- ✅ Find jobs due to run every minute
- ✅ Calculate next run from cron expression
- ✅ Prevent overlapping executions via is_running lock
- ✅ Trigger script execution via Execution Service
- ✅ Update next run time and last run status
- ✅ Handle timezone conversions

### Event Handler Service
- ✅ Subscribe to all domain events via EventBus
- ✅ Find scripts by event type (via repository)
- ✅ Trigger execution with event payload
- ✅ Retry failed executions with exponential backoff
- ✅ Move to dead letter queue after max retries
- ✅ Log all event-triggered executions

### Business Rules
- ✅ Script validation enforced before persistence
- ✅ Permission checks via sdkcomposables.CanUser()
- ✅ Resource limits validated
- ✅ Type-specific requirements validated
- ✅ Uniqueness constraints enforced

### Error Handling
- ✅ All methods use serrors.Op for operation tracking
- ✅ Errors wrapped with serrors.E(op, err)
- ✅ Error kinds used (Validation, NotFound, Permission)
- ✅ Contextual error messages with details
- ✅ Proper error propagation through layers

### Integration
- ✅ Services use repository interfaces (not implementations)
- ✅ DI via constructor injection
- ✅ Transaction management for multi-step operations
- ✅ EventBus integration for domain events
- ✅ Composables for tenant context extraction
