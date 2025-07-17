# Phase 3: Service Layer - Architecture & Design

## Overview
This phase defines the architecture of the service layer for the `scripts` module. The service layer orchestrates the business logic, acting as a mediator between the presentation layer and the domain model. It is responsible for transaction management, authorization, and publishing domain events.

## Background
- The service layer contains the application's business logic and use cases.
- It depends on the domain layer's repository interfaces, not on concrete implementations.
- Services are the primary clients of the domain aggregates. They load aggregates from repositories, invoke domain logic, and save the results.
- All service methods must be authorized and operate within a transactional boundary.

## Task 3.1: Service Interface and DTO Design

### Objectives
- Design the interfaces for `ScriptService` and `ExecutionService`.
- Define the Data Transfer Objects (DTOs) used to communicate with the service layer.
- Specify the responsibilities of each service.

### Detailed Design

#### 1. `ScriptService` Interface
The `ScriptService` handles the management (CRUD) of scripts.

`modules/scripts/services/script_service.go`:
```go
package services

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
)

// ScriptService defines the application's use cases for managing scripts.
type ScriptService interface {
    // CreateNewScript creates a new script from the provided data.
    // It performs validation, authorization, and publishes a ScriptCreated event.
    CreateNewScript(ctx context.Context, dto CreateScriptDTO) (script.Script, error)

    // UpdateScriptContent changes the code of an existing script.
    // This creates a new version of the script and publishes a ScriptContentUpdated event.
    UpdateScriptContent(ctx context.Context, dto UpdateScriptContentDTO) (script.Script, error)

    // UpdateScriptMetadata changes the configuration of a script (e.g., its cron schedule or HTTP path).
    UpdateScriptMetadata(ctx context.Context, dto UpdateScriptMetadataDTO) (script.Script, error)

    // GetScriptByID retrieves a single script by its ID.
    GetScriptByID(ctx context.Context, scriptID uuid.UUID) (script.Script, error)

    // FindScripts searches for scripts based on a set of criteria.
    FindScripts(ctx context.Context, params *repo.FindParams) ([]script.Script, int64, error)
    
    // DeleteScript removes a script.
    DeleteScript(ctx context.Context, scriptID uuid.UUID) error
}
```

#### 2. `ExecutionService` Interface
The `ExecutionService` handles the execution of scripts and manages their lifecycle.

`modules/scripts/services/execution_service.go`:
```go
package services

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/execution"
)

// ExecutionService defines the use cases for running scripts and querying their history.
type ExecutionService interface {
    // TriggerScriptExecution starts a new script run.
    // This is the primary entry point for all script executions (manual, cron, http).
    // It creates an Execution aggregate and publishes ExecutionStarted/ExecutionCompleted events.
    TriggerScriptExecution(ctx context.Context, dto TriggerExecutionDTO) (execution.Execution, error)

    // GetExecutionDetails retrieves the details of a single script run.
    GetExecutionDetails(ctx context.Context, executionID uuid.UUID) (execution.Execution, error)

    // FindExecutionsForScript finds the execution history for a specific script.
    FindExecutionsForScript(ctx context.Context, scriptID uuid.UUID, params *repo.FindParams) ([]execution.Execution, int64, error)
}
```

#### 3. Data Transfer Objects (DTOs)
DTOs are simple data structures used to pass data into and out of the service layer. They are distinct from the domain model.

`modules/scripts/services/dtos.go`:
```go
package services

import (
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/value_objects"
)

// CreateScriptDTO is used to create a new script.
type CreateScriptDTO struct {
    Name        string
    Description string
    Type        value_objects.ScriptType
    Content     string
    Metadata    value_objects.ScriptMetadata // e.g., CronMetadata or HTTPMetadata
}

// UpdateScriptContentDTO is used to change a script's code.
type UpdateScriptContentDTO struct {
    ScriptID    uuid.UUID
    NewContent  string
    ChangeSummary string // Optional summary of changes for auditing.
}

// UpdateScriptMetadataDTO is used to change a script's configuration.
type UpdateScriptMetadataDTO struct {
    ScriptID      uuid.UUID
    NewMetadata   value_objects.ScriptMetadata
}

// TriggerExecutionDTO is used to start a script run.
type TriggerExecutionDTO struct {
    ScriptID  uuid.UUID
    Trigger   value_objects.ExecutionTrigger
    Input     map[string]interface{} // Parameters to pass to the script.
}
```

## Task 3.2: Service Logic and Orchestration Design

### Objectives
- Outline the internal logic of a typical service method.
- Define how services interact with the domain layer, repositories, and event bus.
- Specify the authorization and transaction management strategy, including robust error and panic handling.
- Design a middleware pattern for cross-cutting concerns like logging, metrics, and validation.

### Detailed Design

#### 1. Service Method Logic Flow
A typical service method, such as `CreateNewScript`, will follow these steps, ideally wrapped in a transactional and panic-recovering closure:
1.  **Start Transaction**: Begin a database transaction that can be automatically rolled back on panic or error.
2.  **Authorization**: Check if the user in the context has the required permission (e.g., `scripts.create`) using the RBAC component.
3.  **Validation**: Validate the input DTO. This includes checking the syntax of the script content via the `jsruntime`.
4.  **Create Aggregate**: Instantiate a new `Script` aggregate using its factory method (`script.New(...)`). The domain entity enforces its own invariants.
5.  **Persist**: Use the `script.Repository` to save the new aggregate.
6.  **Publish Events**: Create and publish one or more domain events (e.g., `ScriptCreated`) using the event bus. This is done *before* committing the transaction to ensure atomicity (if using an outbox pattern).
7.  **Commit Transaction**: Commit the database transaction.
8.  **Return Result**: Return the newly created `Script` entity or an error.

#### 2. `ExecutionService` Orchestration
The `TriggerScriptExecution` method is more complex and orchestrates several components:
1.  **Authorization & Validation**: Check for `scripts.execute` permission and validate input parameters against definitions in the script's metadata.
2.  **Start Transaction**.
3.  **Load Script Aggregate**: Fetch the `Script` to be executed from the `script.Repository`.
4.  **Create Execution Aggregate**: Create a new `Execution` aggregate in a `Pending` or `Running` state.
5.  **Persist Execution**: Save the new `Execution` aggregate.
6.  **Publish `ExecutionStarted` Event**.
7.  **Commit Transaction**.
8.  **Delegate to Runtime**: Asynchronously pass the script content and execution context to the `jsruntime.Runtime` for actual execution. This is done *after* committing the transaction to unlock the database row.
9.  **Handle Result**: When the runtime completes, the `ExecutionService` receives the result (output or error).
10. **Update Execution Aggregate**: It then starts a *new transaction* to load the `Execution` aggregate, update its status (`Completed`, `Failed`), and save it back to the repository.
11. **Publish `ExecutionCompleted` Event**.
12. **Commit Final Transaction**.

#### 3. Authorization and Permissions
- A new set of permissions will be defined in `modules/scripts/permissions/constants.go`.
- Examples: `scripts.create`, `scripts.execute`, `scripts.view_history`.
- Each service method will begin with a call to an RBAC component to check the current user's permissions.

```go
// Example permission check in a service method
func (s *scriptService) CreateNewScript(ctx context.Context, dto CreateScriptDTO) (script.Script, error) {
    if err := s.authorizer.Check(ctx, permissions.ScriptsCreate); err != nil {
        return nil, err // e.g., serrors.Forbidden
    }
    // ... rest of the logic
}
```

#### 4. Service Middleware Pattern
To handle cross-cutting concerns cleanly, a middleware pattern can be applied to service methods.

```go
// A generic wrapper for service method execution
func (s *scriptService) withTx(ctx context.Context, fn func(txCtx context.Context) error) error {
    // 1. Start transaction
    // 2. Defer rollback/commit logic with panic recovery
    // 3. Call fn(txCtx)
    // 4. Return result
}

// Usage
func (s *scriptService) CreateNewScript(ctx context.Context, dto CreateScriptDTO) (script.Script, error) {
    var newScript script.Script
    err := s.withTx(ctx, func(txCtx context.Context) error {
        // ... authorization, validation, domain logic ...
        // All repository calls here use the txCtx
        created, err := s.repo.Create(txCtx, domainScript)
        if err == nil {
            newScript = created
        }
        return err
    })
    return newScript, err
}
```

## Task 3.3: Implementation Task Breakdown

### Service Layer
- [ ] Implement the `ScriptService` with full CRUD logic, including validation and authorization.
- [ ] Implement the `ExecutionService` with the described orchestration logic.
- [ ] Ensure all state-changing methods publish the appropriate domain events via the event bus.
- [ ] Implement a transactional wrapper or middleware to ensure atomicity and panic recovery for service methods.
- [ ] Define and register all necessary RBAC permissions in `permissions/constants.go`.
- [ ] Write comprehensive service-layer tests using mock repositories and event buses to verify business logic, authorization, and event publishing.

### Basic API Bindings
- [ ] Create the API registry in `pkg/jsruntime/apis/registry.go` to manage all exposed APIs.
- [ ] Implement the `sdk.http` API with security checks (e.g., SSRF protection, request size limits).
- [ ] Implement the `sdk.cache` API with tenant-scoped keys to prevent data leakage.
- [ ] Implement the `sdk.log` API that integrates with the application's structured logger, automatically adding script context.
- [ ] **Crucially**, for the `sdk.db` API, the design must not rely on simple string manipulation. It should use a proper query builder or parser to safely inject tenant ID conditions, preventing any risk of SQL injection.
- [ ] Write integration tests for each API binding to ensure they work as expected and respect security boundaries.
- [ ] Create a command to generate TypeScript definitions (`sdk.d.ts`) for the exposed APIs to aid in script development.

### Deliverables Checklist
- [x] Finalized interfaces for `ScriptService` and `ExecutionService`.
- [x] Defined all DTOs for service layer communication.
- [x] Designed the logical flow for core service methods, including a robust transaction and error handling strategy.
- [x] Outlined the orchestration logic for script execution.
- [x] Specified the strategy for authorization and transaction management.
- [x] A detailed task list for the implementation of this phase.
- [x] Documentation for the service layer architecture and responsibilities.

## Success Criteria
- The service interfaces are clear and represent distinct business use cases.
- The DTOs provide a stable contract for the API without exposing domain internals.
- The orchestration design is robust, transactional, and accounts for asynchronous execution.
- The authorization strategy is clearly defined and integrated into the service logic.
- The design for the JavaScript SDK APIs prioritizes security, especially for database access.
