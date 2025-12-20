# JavaScript Runtime - Service Layer Specification

**Status:** Implementation Ready
**Layer:** Service Layer
**Dependencies:** Domain entities, Repository interfaces, Event bus
**Related Issues:** #411, #412, #413, #415, #418, #419

---

## Overview

This specification defines the service layer for JavaScript runtime functionality, including script management and execution orchestration. Services coordinate business logic, permission checks, validation, and event publishing.

## Service Architecture

### ScriptService

**Purpose:** Manage script CRUD operations, versioning, and content updates.

**Location:** `modules/scripts/services/script_service.go`

**Dependencies:**
```go
type ScriptService struct {
    repo        script.Repository
    versionRepo version.Repository
    runtime     runtime.Runtime
    publisher   eventbus.EventBus
}

func NewScriptService(
    repo script.Repository,
    versionRepo version.Repository,
    runtime runtime.Runtime,
    publisher eventbus.EventBus,
) *ScriptService {
    return &ScriptService{
        repo:        repo,
        versionRepo: versionRepo,
        runtime:     runtime,
        publisher:   publisher,
    }
}
```

**Methods:**

#### Create

```go
func (s *ScriptService) Create(ctx context.Context, data CreateScriptDTO) (script.Script, error) {
    const op serrors.Op = "ScriptService.Create"

    // Permission check
    if err := composables.CanUser(ctx, permissions.ScriptCreate); err != nil {
        return nil, serrors.E(op, err)
    }

    // Validate syntax before saving
    if err := s.runtime.ValidateSyntax(data.Source); err != nil {
        return nil, serrors.E(op, serrors.KindValidation,
            "syntax error in script", err)
    }

    var created script.Script
    err := composables.InTx(ctx, func(txCtx context.Context) error {
        // Create script entity
        scr := script.New(
            data.Name,
            script.ScriptType(data.Type),
            data.Source,
            script.WithDescription(data.Description),
            script.WithSchedule(data.Schedule),
            script.WithEventType(data.EventType),
            script.WithEnabled(data.Enabled),
        )

        // Save to repository
        result, err := s.repo.Create(txCtx, scr)
        if err != nil {
            return err
        }
        created = result

        // Create initial version
        ver := version.New(
            result.ID(),
            1,
            data.Source,
            version.WithCreatedBy(composables.MustUseUser(txCtx).ID()),
        )
        if _, err := s.versionRepo.Create(txCtx, ver); err != nil {
            return err
        }

        return nil
    })
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // Publish creation event
    s.publisher.Publish(&script.CreatedEvent{
        Script: created,
        User:   composables.MustUseUser(ctx),
    })

    return created, nil
}
```

#### Update

```go
func (s *ScriptService) Update(
    ctx context.Context,
    id uuid.UUID,
    data UpdateScriptDTO,
) error {
    const op serrors.Op = "ScriptService.Update"

    if err := composables.CanUser(ctx, permissions.ScriptUpdate); err != nil {
        return serrors.E(op, err)
    }

    return composables.InTx(ctx, func(txCtx context.Context) error {
        existing, err := s.repo.GetByID(txCtx, id)
        if err != nil {
            return err
        }

        // Apply updates (metadata only, not source)
        updated := data.Apply(existing)

        if err := s.repo.Update(txCtx, updated); err != nil {
            return err
        }

        s.publisher.Publish(&script.UpdatedEvent{
            Script: updated,
            User:   composables.MustUseUser(ctx),
        })

        return nil
    })
}
```

#### UpdateContent

```go
func (s *ScriptService) UpdateContent(
    ctx context.Context,
    id uuid.UUID,
    source string,
) error {
    const op serrors.Op = "ScriptService.UpdateContent"

    if err := composables.CanUser(ctx, permissions.ScriptUpdate); err != nil {
        return serrors.E(op, err)
    }

    // Validate syntax
    if err := s.runtime.ValidateSyntax(source); err != nil {
        return serrors.E(op, serrors.KindValidation,
            "syntax error in script", err)
    }

    return composables.InTx(ctx, func(txCtx context.Context) error {
        existing, err := s.repo.GetByID(txCtx, id)
        if err != nil {
            return err
        }

        // Get latest version number
        latestVer, err := s.versionRepo.GetLatestVersion(txCtx, id)
        if err != nil {
            return err
        }

        // Create new version
        newVersion := version.New(
            id,
            latestVer.Number()+1,
            source,
            version.WithCreatedBy(composables.MustUseUser(txCtx).ID()),
        )
        if _, err := s.versionRepo.Create(txCtx, newVersion); err != nil {
            return err
        }

        // Update script source
        updated := existing.SetSource(source)
        if err := s.repo.Update(txCtx, updated); err != nil {
            return err
        }

        s.publisher.Publish(&script.ContentUpdatedEvent{
            ScriptID: id,
            Version:  newVersion.Number(),
            User:     composables.MustUseUser(ctx),
        })

        return nil
    })
}
```

#### Delete

```go
func (s *ScriptService) Delete(ctx context.Context, id uuid.UUID) error {
    const op serrors.Op = "ScriptService.Delete"

    if err := composables.CanUser(ctx, permissions.ScriptDelete); err != nil {
        return serrors.E(op, err)
    }

    return composables.InTx(ctx, func(txCtx context.Context) error {
        existing, err := s.repo.GetByID(txCtx, id)
        if err != nil {
            return err
        }

        // Delete versions first (FK constraint)
        if err := s.versionRepo.DeleteAllForScript(txCtx, id); err != nil {
            return err
        }

        if err := s.repo.Delete(txCtx, id); err != nil {
            return err
        }

        s.publisher.Publish(&script.DeletedEvent{
            ScriptID: id,
            User:     composables.MustUseUser(ctx),
        })

        return nil
    })
}
```

#### GetByID

```go
func (s *ScriptService) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (script.Script, error) {
    const op serrors.Op = "ScriptService.GetByID"

    if err := composables.CanUser(ctx, permissions.ScriptRead); err != nil {
        return nil, serrors.E(op, err)
    }

    return s.repo.GetByID(ctx, id)
}
```

#### GetPaginated

```go
func (s *ScriptService) GetPaginated(
    ctx context.Context,
    params *script.FindParams,
) ([]script.Script, int64, error) {
    const op serrors.Op = "ScriptService.GetPaginated"

    if err := composables.CanUser(ctx, permissions.ScriptRead); err != nil {
        return nil, 0, serrors.E(op, err)
    }

    scripts, err := s.repo.GetPaginated(ctx, params)
    if err != nil {
        return nil, 0, serrors.E(op, err)
    }

    total, err := s.repo.Count(ctx, params)
    if err != nil {
        return nil, 0, serrors.E(op, err)
    }

    return scripts, total, nil
}
```

---

### ExecutionService

**Purpose:** Execute scripts and manage execution lifecycle.

**Location:** `modules/scripts/services/execution_service.go`

**Dependencies:**
```go
type ExecutionService struct {
    scriptRepo    script.Repository
    executionRepo execution.Repository
    runtime       runtime.Runtime
    publisher     eventbus.EventBus
}

func NewExecutionService(
    scriptRepo script.Repository,
    executionRepo execution.Repository,
    runtime runtime.Runtime,
    publisher eventbus.EventBus,
) *ExecutionService {
    return &ExecutionService{
        scriptRepo:    scriptRepo,
        executionRepo: executionRepo,
        runtime:       runtime,
        publisher:     publisher,
    }
}
```

**Methods:**

#### Execute

```go
func (s *ExecutionService) Execute(
    ctx context.Context,
    scriptID uuid.UUID,
    input map[string]interface{},
) (*execution.Execution, error) {
    const op serrors.Op = "ExecutionService.Execute"

    if err := composables.CanUser(ctx, permissions.ScriptExecute); err != nil {
        return nil, serrors.E(op, err)
    }

    // Get script
    scr, err := s.scriptRepo.GetByID(ctx, scriptID)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // Check if enabled
    if !scr.Enabled() {
        return nil, serrors.E(op, serrors.KindValidation,
            "script is disabled")
    }

    // Create execution record
    exec := execution.New(
        scriptID,
        execution.TriggerManual,
        execution.WithInput(input),
        execution.WithTriggeredBy(composables.MustUseUser(ctx).ID()),
    )

    exec, err = s.executionRepo.Create(ctx, exec)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // Publish execution started event
    s.publisher.Publish(&execution.StartedEvent{
        Execution: exec,
    })

    // Execute asynchronously
    go s.executeAsync(context.Background(), exec, scr)

    return exec, nil
}

func (s *ExecutionService) executeAsync(
    ctx context.Context,
    exec execution.Execution,
    scr script.Script,
) {
    // Build execution context
    execCtx := s.buildExecutionContext(ctx, exec, scr)

    // Run script
    result, err := s.runtime.Execute(execCtx, scr.Source(), exec.Input())

    // Update execution status
    if err != nil {
        exec = exec.SetStatus(execution.StatusFailed).
            SetError(err.Error())
    } else {
        exec = exec.SetStatus(execution.StatusCompleted).
            SetOutput(result)
    }

    exec = exec.SetCompletedAt(time.Now())

    // Save result
    if updateErr := s.executionRepo.Update(ctx, exec); updateErr != nil {
        // Log error but don't fail (execution already complete)
        // TODO: Add structured logging
    }

    // Publish completion event
    s.publisher.Publish(&execution.CompletedEvent{
        Execution: exec,
        Success:   err == nil,
    })
}

func (s *ExecutionService) buildExecutionContext(
    ctx context.Context,
    exec execution.Execution,
    scr script.Script,
) context.Context {
    // Copy tenant, user, session to execution context
    // Add execution metadata
    execCtx := context.Background()

    if tenant, err := composables.UseTenantID(ctx); err == nil {
        execCtx = composables.WithTenantID(execCtx, tenant)
    }

    if user, err := composables.UseUser(ctx); err == nil {
        execCtx = composables.WithUser(execCtx, user)
    }

    if pool, err := composables.UsePool(ctx); err == nil {
        execCtx = composables.WithPool(execCtx, pool)
    }

    // Add execution-specific context
    execCtx = execution.WithExecutionID(execCtx, exec.ID())
    execCtx = execution.WithScriptID(execCtx, scr.ID())

    return execCtx
}
```

#### GetByID

```go
func (s *ExecutionService) GetByID(
    ctx context.Context,
    id uuid.UUID,
) (execution.Execution, error) {
    const op serrors.Op = "ExecutionService.GetByID"

    if err := composables.CanUser(ctx, permissions.ScriptRead); err != nil {
        return nil, serrors.E(op, err)
    }

    return s.executionRepo.GetByID(ctx, id)
}
```

#### GetForScript

```go
func (s *ExecutionService) GetForScript(
    ctx context.Context,
    scriptID uuid.UUID,
    params *execution.FindParams,
) ([]execution.Execution, error) {
    const op serrors.Op = "ExecutionService.GetForScript"

    if err := composables.CanUser(ctx, permissions.ScriptRead); err != nil {
        return nil, serrors.E(op, err)
    }

    params.ScriptID = &scriptID
    return s.executionRepo.GetPaginated(ctx, params)
}
```

---

## DTOs

### CreateScriptDTO

**Location:** `modules/scripts/services/dtos/create_script_dto.go`

```go
type CreateScriptDTO struct {
    Name        string `validate:"required,max=255"`
    Description string `validate:"max=1000"`
    Type        string `validate:"required,oneof=cron manual event http"`
    Source      string `validate:"required"`
    Schedule    string `validate:"omitempty,cron"` // Required if Type=cron
    EventType   string `validate:"omitempty"`      // Required if Type=event
    Enabled     bool   `validate:"omitempty"`
}

func (dto *CreateScriptDTO) Validate(ctx context.Context) error {
    if err := constants.Validate.Struct(dto); err != nil {
        return serrors.ProcessValidatorErrors(
            err.(validator.ValidationErrors),
            dto.fieldIDMapping,
        )
    }

    // Custom validation
    if dto.Type == "cron" && dto.Schedule == "" {
        return serrors.NewFieldRequiredError("Schedule", "Scripts.Form.Schedule")
    }

    if dto.Type == "event" && dto.EventType == "" {
        return serrors.NewFieldRequiredError("EventType", "Scripts.Form.EventType")
    }

    return nil
}

func (dto *CreateScriptDTO) fieldIDMapping(field string) string {
    return fmt.Sprintf("Scripts.Form.%s", field)
}
```

### UpdateScriptDTO

**Location:** `modules/scripts/services/dtos/update_script_dto.go`

```go
type UpdateScriptDTO struct {
    Name        string `validate:"required,max=255"`
    Description string `validate:"max=1000"`
    Schedule    string `validate:"omitempty,cron"`
    EventType   string `validate:"omitempty"`
    Enabled     bool   `validate:"omitempty"`
}

func (dto *UpdateScriptDTO) Validate(ctx context.Context) error {
    if err := constants.Validate.Struct(dto); err != nil {
        return serrors.ProcessValidatorErrors(
            err.(validator.ValidationErrors),
            dto.fieldIDMapping,
        )
    }
    return nil
}

func (dto *UpdateScriptDTO) Apply(scr script.Script) script.Script {
    return scr.
        SetName(dto.Name).
        SetDescription(dto.Description).
        SetSchedule(dto.Schedule).
        SetEventType(dto.EventType).
        SetEnabled(dto.Enabled)
}

func (dto *UpdateScriptDTO) fieldIDMapping(field string) string {
    return fmt.Sprintf("Scripts.Form.%s", field)
}
```

### ExecuteScriptDTO

**Location:** `modules/scripts/services/dtos/execute_script_dto.go`

```go
type ExecuteScriptDTO struct {
    ScriptID uuid.UUID              `validate:"required"`
    Input    map[string]interface{} `validate:"omitempty"`
}

func (dto *ExecuteScriptDTO) Validate(ctx context.Context) error {
    if err := constants.Validate.Struct(dto); err != nil {
        return serrors.ProcessValidatorErrors(
            err.(validator.ValidationErrors),
            dto.fieldIDMapping,
        )
    }
    return nil
}

func (dto *ExecuteScriptDTO) fieldIDMapping(field string) string {
    return fmt.Sprintf("Scripts.Execute.%s", field)
}
```

---

## Validation Patterns

### Syntax Validation

Scripts must be validated before saving:

```go
// In ScriptService.Create and UpdateContent
if err := s.runtime.ValidateSyntax(data.Source); err != nil {
    return serrors.E(op, serrors.KindValidation,
        "syntax error in script", err)
}
```

### Schedule Validation

Cron expressions validated using existing cron library:

```go
import "github.com/robfig/cron/v3"

func validateCronSchedule(schedule string) error {
    parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom |
        cron.Month | cron.Dow)
    if _, err := parser.Parse(schedule); err != nil {
        return err
    }
    return nil
}
```

Register custom validator tag:

```go
// In init() or setup
constants.Validate.RegisterValidation("cron", func(fl validator.FieldLevel) bool {
    schedule := fl.Field().String()
    return validateCronSchedule(schedule) == nil
})
```

---

## Permission Checks

**Required Permissions:**

```go
// modules/scripts/permissions/permissions.go
package permissions

import "github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"

const (
    ScriptCreate  permission.Permission = "scripts.create"
    ScriptRead    permission.Permission = "scripts.read"
    ScriptUpdate  permission.Permission = "scripts.update"
    ScriptDelete  permission.Permission = "scripts.delete"
    ScriptExecute permission.Permission = "scripts.execute"
)
```

**Permission Checks in Services:**

```go
// All methods check permissions early
if err := composables.CanUser(ctx, permissions.ScriptCreate); err != nil {
    return nil, serrors.E(op, err)
}
```

---

## Event Publishing

**Domain Events:**

```go
// Published by ScriptService
script.CreatedEvent
script.UpdatedEvent
script.ContentUpdatedEvent
script.DeletedEvent

// Published by ExecutionService
execution.StartedEvent
execution.CompletedEvent
```

**Event Pattern:**

```go
s.publisher.Publish(&script.CreatedEvent{
    Script: created,
    User:   composables.MustUseUser(ctx),
})
```

---

## Transaction Management

**All Write Operations Use Transactions:**

```go
err := composables.InTx(ctx, func(txCtx context.Context) error {
    // Create script
    result, err := s.repo.Create(txCtx, scr)
    if err != nil {
        return err
    }

    // Create initial version
    ver := version.New(result.ID(), 1, data.Source)
    if _, err := s.versionRepo.Create(txCtx, ver); err != nil {
        return err
    }

    return nil
})
```

**Rollback on Error:**

Transaction automatically rolls back if any operation fails.

---

## Error Handling

**Structured Errors:**

```go
const op serrors.Op = "ScriptService.Create"

// Validation errors
return serrors.E(op, serrors.KindValidation, "syntax error", err)

// Not found
return serrors.E(op, serrors.KindNotFound, "script not found")

// Permission denied
return serrors.E(op, serrors.KindPermission, "access denied")

// Database errors
return serrors.E(op, err)
```

---

## Testing Strategy

### Service Tests

**Location:** `modules/scripts/services/script_service_test.go`

**Test Coverage:**

```go
func TestScriptService_Create_Success(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t,
        itf.WithPermissions(permissions.ScriptCreate),
    )

    service := itf.GetService[*services.ScriptService](env)

    dto := dtos.CreateScriptDTO{
        Name:   "Test Script",
        Type:   "manual",
        Source: "console.log('hello');",
    }

    result, err := service.Create(env.Ctx, dto)
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, "Test Script", result.Name())
}

func TestScriptService_Create_SyntaxError(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t,
        itf.WithPermissions(permissions.ScriptCreate),
    )

    service := itf.GetService[*services.ScriptService](env)

    dto := dtos.CreateScriptDTO{
        Name:   "Invalid Script",
        Type:   "manual",
        Source: "console.log(", // Syntax error
    }

    _, err := service.Create(env.Ctx, dto)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "syntax error")
}

func TestScriptService_Create_NoPermission(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t) // No permissions

    service := itf.GetService[*services.ScriptService](env)

    dto := dtos.CreateScriptDTO{
        Name:   "Test Script",
        Type:   "manual",
        Source: "console.log('hello');",
    }

    _, err := service.Create(env.Ctx, dto)
    assert.Error(t, err)
    assert.ErrorIs(t, err, composables.ErrForbidden)
}

func TestExecutionService_Execute_Success(t *testing.T) {
    t.Parallel()
    env := itf.Setup(t,
        itf.WithPermissions(permissions.ScriptExecute),
    )

    scriptSvc := itf.GetService[*services.ScriptService](env)
    execSvc := itf.GetService[*services.ExecutionService](env)

    // Create script
    scr, _ := scriptSvc.Create(env.Ctx, dtos.CreateScriptDTO{
        Name:   "Test",
        Type:   "manual",
        Source: "return { result: 42 };",
        Enabled: true,
    })

    // Execute
    exec, err := execSvc.Execute(env.Ctx, scr.ID(), nil)
    assert.NoError(t, err)
    assert.NotNil(t, exec)

    // Wait for async execution (in tests, use synchronous execution)
    // Poll for completion or use test-specific synchronous mode
}
```

---

## Dependencies

**Import Paths:**

```go
import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/entities/execution"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/entities/version"
    "github.com/iota-uz/iota-sdk/modules/scripts/runtime"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/eventbus"
    "github.com/iota-uz/iota-sdk/pkg/serrors"
)
```

---

## Integration with ITF

**Service Registration:**

```go
// In modules/scripts/module.go
func (m *Module) RegisterServices(c *di.Container) {
    c.Provide(services.NewScriptService)
    c.Provide(services.NewExecutionService)
}
```

**Test Setup:**

```go
env := itf.Setup(t,
    itf.WithModule(scripts.NewModule()),
    itf.WithPermissions(permissions.ScriptCreate),
)

service := itf.GetService[*services.ScriptService](env)
```

---

## Next Steps

After implementing service layer:

1. **Runtime Engine** (06-runtime-engine.md) - Goja VM pool and executor
2. **API Bindings** (07-api-bindings.md) - JavaScript APIs for scripts
3. **Event Integration** (08-event-integration.md) - Event triggers and handlers

---

## References

- Domain entities: `01-domain-entities.md`
- Repository layer: `02-repository-layer.md`
- Service pattern: `modules/core/services/user_service.go`
- DTO pattern: `modules/core/presentation/controllers/dtos/user_dto.go`
- ITF framework: `.claude/guides/backend/testing.md`
