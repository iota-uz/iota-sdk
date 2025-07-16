# Phase 2: Domain Model & Persistence - Architecture & Design

## Overview
This phase defines the domain model for the `scripts` module, following Domain-Driven Design (DDD) principles. The focus is on designing the aggregates, value objects, and repository interfaces that represent the core business concepts of scripts and their execution, without delving into the persistence implementation.

## Background
- The domain model must be pure and have no knowledge of infrastructure concerns like databases or runtimes.
- Entities should be immutable to ensure predictable state management.
- Repository interfaces define the contract for data access, abstracting the storage mechanism.
- Domain events are a key part of the design, enabling a decoupled and extensible architecture.

## Task 2.1: Domain Aggregates and Value Objects

### Objectives
- Design the `Script` aggregate root, which represents a user-defined script.
- Design the `Execution` aggregate root, which represents a single run of a script.
- Define the value objects that compose the aggregates, such as `ScriptType`, `ExecutionStatus`, and `ScriptMetadata`.

### Detailed Design

#### 1. `Script` Aggregate
The `Script` is the central entity. It encapsulates the script's content, configuration, and identity.

`modules/scripts/domain/aggregates/script/script.go`:
```go
package script

import (
    "context"
    "github.com/google/uuid"
    "time"
)

// Script represents the core domain entity for a user-defined script.
// It is an aggregate root.
type Script interface {
    // Identity and Ownership
    ID() uuid.UUID
    TenantID() uuid.UUID

    // Core Properties
    Name() string
    Description() string
    Content() string
    Type() value_objects.ScriptType
    Version() int
    IsEnabled() bool

    // Metadata and Configuration
    Metadata() value_objects.ScriptMetadata
    Tags() []string
    
    // Audit Information
    CreatedAt() time.Time
    UpdatedAt() time.Time
    CreatedBy() uuid.UUID
    UpdatedBy() uuid.UUID

    // Business Logic (returns a new, modified Script instance)
    UpdateContent(newContent string, byUser uuid.UUID) (Script, error)
    ChangeMetadata(newMetadata value_objects.ScriptMetadata, byUser uuid.UUID) (Script, error)
    Enable(byUser uuid.UUID) Script
    Disable(byUser uuid.UUID) Script
}
```

#### 2. `Execution` Aggregate
The `Execution` aggregate captures the state of a single script run.

`modules/scripts/domain/aggregates/execution/execution.go`:
```go
package execution

import (
    "context"
    "github.com/google/uuid"
    "time"
)

// Execution represents a single run of a script. It is an aggregate root.
type Execution interface {
    // Identity
    ID() uuid.UUID
    ScriptID() uuid.UUID
    TenantID() uuid.UUID

    // State and Outcome
    Status() value_objects.ExecutionStatus
    Result() value_objects.ExecutionResult
    StartedAt() time.Time
    CompletedAt() *time.Time
    Duration() time.Duration

    // Context
    Trigger() value_objects.ExecutionTrigger
    Input() map[string]interface{}
}
```

#### 3. Value Objects
Value objects are immutable components of the aggregates.

`modules/scripts/domain/value_objects/value_objects.go`:
```go
package value_objects

// ScriptType is an enum for the different kinds of scripts.
type ScriptType string
const (
    TypeCron      ScriptType = "cron"
    TypeHTTP      ScriptType = "http"
    TypeOneOff    ScriptType = "one_off"
    TypeEmbedded  ScriptType = "embedded"
)
func (t ScriptType) IsValid() bool { /* ... */ }

// ExecutionStatus is an enum for the state of an execution.
type ExecutionStatus string
const (
    StatusPending   ExecutionStatus = "pending"
    StatusRunning   ExecutionStatus = "running"
    StatusCompleted ExecutionStatus = "completed"
    StatusFailed    ExecutionStatus = "failed"
    StatusTimeout   ExecutionStatus = "timeout"
)
func (s ExecutionStatus) IsValid() bool { /* ... */ }

// ScriptMetadata holds type-specific configuration. It is a value object.
// Using an interface allows for different metadata structures per script type.
type ScriptMetadata interface {
    Validate() error
    IsEqual(other ScriptMetadata) bool
}

// CronMetadata holds the configuration for a cron script.
type CronMetadata struct {
    CronExpression string // e.g., "0 * * * *"
    Timezone       string // e.g., "UTC"
}

// HTTPMetadata holds the configuration for an HTTP endpoint script.
type HTTPMetadata struct {
    Path   string // e.g., "/api/custom/report"
    Method string // e.g., "GET", "POST"
}

// ExecutionResult encapsulates the output of a script run.
type ExecutionResult struct {
    Output string // JSON-encoded output
    Error  string // Error message if the run failed
    Logs   []LogEntry
}

// LogEntry represents a single log line from a script execution.
type LogEntry struct {
    Timestamp time.Time
    Level     string // "info", "error", etc.
    Message   string
}

// ExecutionTrigger describes what caused a script to run.
type ExecutionTrigger struct {
    Type      string // "manual", "cron", "api"
    TriggeredBy uuid.UUID // User ID if triggered manually
}
```

## Task 2.2: Repository and Event Design

### Objectives
- Define the repository interfaces that form the contract for data persistence.
- Design the domain events that will be published by the aggregates.
- Incorporate designs for version history management and safe data access.

### Detailed Design

#### 1. Repository Interfaces
These interfaces define how the domain layer interacts with persistence, following the established IOTA SDK patterns. The design must account for versioning and prevent unbounded data growth.

`modules/scripts/domain/aggregates/script/repository.go`:
```go
package script

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/repo" // Use shared repo types
)

// Repository defines the persistence contract for the Script aggregate.
// All operations are automatically scoped to the tenant within the context.
type Repository interface {
    Create(ctx context.Context, script Script) error
    Update(ctx context.Context, script Script) error
    Delete(ctx context.Context, id uuid.UUID) error
    
    FindByID(ctx context.Context, id uuid.UUID) (Script, error)
    Find(ctx context.Context, params *repo.FindParams) ([]Script, error)
    Count(ctx context.Context, params *repo.FindParams) (int64, error)

    // GetVersionHistory retrieves the metadata for all versions of a script.
    GetVersionHistory(ctx context.Context, scriptID uuid.UUID) ([]VersionInfo, error)
    // PruneVersionHistory deletes old script versions based on a retention policy.
    PruneVersionHistory(ctx context.Context, scriptID uuid.UUID, policy VersionRetentionPolicy) error
}

// VersionInfo contains metadata about a specific version of a script.
type VersionInfo struct {
    Version   int
    UpdatedAt time.Time
    UpdatedBy uuid.UUID
}

// VersionRetentionPolicy defines rules for cleaning up old script versions.
type VersionRetentionPolicy struct {
    MaxVersions   int // Max number of versions to keep.
    RetentionDays int // Max age of versions to keep.
}
```

`modules/scripts/domain/aggregates/execution/repository.go`:
```go
package execution

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/repo"
)

// Repository defines the persistence contract for the Execution aggregate.
type Repository interface {
    Create(ctx context.Context, exec Execution) error
    Update(ctx context.Context, exec Execution) error
    
    FindByID(ctx context.Context, id uuid.UUID) (Execution, error)
    FindForScript(ctx context.Context, scriptID uuid.UUID, params *repo.FindParams) ([]Execution, error)
}
```
**Note on Implementation**: The concrete repository implementation must use parameterized queries to prevent SQL injection, rather than string concatenation. Tenant isolation must be enforced at the database query level.

#### 2. Domain Events
Domain events are published when the state of an aggregate changes.

`modules/scripts/domain/events.go`:
```go
package domain

import (
    "github.com/google/uuid"
    "time"
)

// Event is the base interface for all domain events in the scripts module.
type Event interface {
    EventName() string
    OccurredAt() time.Time
}

// ScriptCreated is published when a new script is created.
type ScriptCreated struct {
    ScriptID uuid.UUID
    TenantID uuid.UUID
    UserID   uuid.UUID
    Timestamp time.Time
}

// ScriptContentUpdated is published when a script's code is changed.
type ScriptContentUpdated struct {
    ScriptID   uuid.UUID
    TenantID   uuid.UUID
    UserID     uuid.UUID
    OldVersion int
    NewVersion int
    Timestamp  time.Time
}

// ExecutionStarted is published when a script begins execution.
type ExecutionStarted struct {
    ExecutionID uuid.UUID
    ScriptID    uuid.UUID
    TenantID    uuid.UUID
    Trigger     value_objects.ExecutionTrigger
    Timestamp   time.Time
}

// ExecutionCompleted is published when a script finishes execution.
type ExecutionCompleted struct {
    ExecutionID uuid.UUID
    ScriptID    uuid.UUID
    TenantID    uuid.UUID
    Status      value_objects.ExecutionStatus // "completed", "failed", or "timeout"
    Duration    time.Duration
    Timestamp   time.Time
}
```

## Task 2.3: Implementation Task Breakdown

### Domain Layer
- [ ] Implement the `Script` aggregate root with a factory function for reconstituting from the database (`FromRepository`).
- [ ] Implement the `Execution` aggregate root.
- [ ] Implement all value objects (`ScriptType`, `ExecutionStatus`, `ScriptMetadata`, etc.) with validation.
- [ ] Define all domain events (`ScriptCreated`, `ExecutionCompleted`, etc.).
- [ ] Write comprehensive unit tests for all domain entities and value objects to ensure immutability and correct business logic.

### Infrastructure Layer (Persistence)
- [ ] Create the complete database schema in `schema/scripts-schema.sql`, including tables for scripts, versions, and executions. Ensure foreign keys and cascade deletes are correctly configured.
- [ ] Implement the `ScriptRepository` using `sqlx`. All query methods must enforce tenant isolation using parameterized queries.
- [ ] Implement the `ExecutionRepository`.
- [ ] Implement the `PruneVersionHistory` method according to a defined policy.
- [ ] Implement mappers to convert between domain entities and database models.
- [ ] Write integration tests for both repositories against a real test database to verify CRUD operations, tenant isolation, and versioning.

### Deliverables Checklist
- [x] Finalized design for `Script` and `Execution` aggregates.
- [x] Defined all necessary value objects (`ScriptType`, `ExecutionStatus`, etc.).
- [x] Defined repository interfaces for `Script` and `Execution`, including version management.
- [x] Designed a comprehensive set of domain events.
- [x] A detailed task list for the implementation of this phase.
- [x] Documentation for the domain model, explaining the purpose and relationships of each component.

## Success Criteria
- The domain model is pure, with a clear separation from other layers.
- The design is consistent with existing DDD patterns in the IOTA SDK.
- The repository design prevents SQL injection and includes a strategy for managing data growth (version pruning).
- The interfaces and types are sufficient to support all required business logic.
- The domain events provide a solid foundation for auditing and integration.
