# Phase 3: Service Layer & Basic APIs (2 days)

## Overview
Create the service layer that orchestrates business logic, implements validation, integrates with RBAC, publishes domain events, and provides the basic JavaScript SDK APIs that scripts will use.

## Background
- IOTA SDK services use dependency injection via Application interface
- All services must publish domain events via EventBus
- RBAC integration for permission checks
- Services registered in module.go
- JavaScript SDK provides safe access to IOTA resources

## Task 3.1: Script Service (Day 1)

### Objectives
- Create ScriptService with full CRUD operations
- Implement comprehensive validation
- Add RBAC permission checks
- Publish domain events for all operations
- Create execution service for running scripts
- Track execution history

### Detailed Steps

#### 1. Define Service Interface
Create `modules/scripts/services/interfaces.go`:
```go
package services

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/value_objects"
)

type ScriptService interface {
    // CRUD operations
    CreateScript(ctx context.Context, req CreateScriptRequest) (*script.Script, error)
    UpdateScript(ctx context.Context, req UpdateScriptRequest) (*script.Script, error)
    DeleteScript(ctx context.Context, id uuid.UUID) error
    GetScript(ctx context.Context, id uuid.UUID) (*script.Script, error)
    FindScripts(ctx context.Context, params script.FindParams) ([]*script.Script, error)
    
    // Business operations
    EnableScript(ctx context.Context, id uuid.UUID) (*script.Script, error)
    DisableScript(ctx context.Context, id uuid.UUID) (*script.Script, error)
    ValidateScript(ctx context.Context, content string) (*ValidationResult, error)
    
    // Execution
    ExecuteScript(ctx context.Context, id uuid.UUID, params map[string]interface{}) (*ExecutionResult, error)
    GetExecutionHistory(ctx context.Context, scriptID uuid.UUID, limit int) ([]*Execution, error)
}

type ExecutionService interface {
    Execute(ctx context.Context, script *script.Script, params map[string]interface{}) (*ExecutionResult, error)
    ExecuteWithTimeout(ctx context.Context, script *script.Script, params map[string]interface{}, timeout time.Duration) (*ExecutionResult, error)
    ValidateSyntax(content string) error
}

// Request/Response types
type CreateScriptRequest struct {
    Name        string
    Description string
    Type        value_objects.ScriptType
    Content     string
    Tags        []string
    Metadata    map[string]interface{}
}

type UpdateScriptRequest struct {
    ID          uuid.UUID
    Name        *string
    Description *string
    Content     *string
    Tags        []string
    Metadata    map[string]interface{}
}

type ValidationResult struct {
    Valid    bool
    Errors   []ValidationError
    Warnings []ValidationWarning
}

type ValidationError struct {
    Line    int
    Column  int
    Message string
}

type ExecutionResult struct {
    Success   bool
    Output    interface{}
    Error     string
    Duration  time.Duration
    Logs      []LogEntry
}

type Execution struct {
    ID          uuid.UUID
    ScriptID    uuid.UUID
    Status      ExecutionStatus
    StartedAt   time.Time
    CompletedAt *time.Time
    Duration    time.Duration
    Error       string
    Logs        []LogEntry
    Params      map[string]interface{}
}

type ExecutionStatus string

const (
    ExecutionStatusRunning   ExecutionStatus = "running"
    ExecutionStatusCompleted ExecutionStatus = "completed"
    ExecutionStatusFailed    ExecutionStatus = "failed"
    ExecutionStatusTimeout   ExecutionStatus = "timeout"
)
```

#### 2. Implement Script Service
Create `modules/scripts/services/script_service.go`:
```go
package services

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/eventbus"
    "github.com/iota-uz/iota-sdk/pkg/rbac"
    "github.com/iota-uz/iota-sdk/pkg/serrors"
)

type scriptService struct {
    repo            script.Repository
    executionSvc    ExecutionService
    eventPublisher  eventbus.EventBus
    rbac            rbac.RBAC
}

func NewScriptService(
    repo script.Repository,
    executionSvc ExecutionService,
    eventPublisher eventbus.EventBus,
    rbac rbac.RBAC,
) ScriptService {
    return &scriptService{
        repo:           repo,
        executionSvc:   executionSvc,
        eventPublisher: eventPublisher,
        rbac:          rbac,
    }
}

func (s *scriptService) CreateScript(ctx context.Context, req CreateScriptRequest) (*script.Script, error) {
    // Check permissions
    if err := s.rbac.CheckPermission(ctx, "scripts.create"); err != nil {
        return nil, serrors.Forbidden("insufficient permissions to create scripts")
    }
    
    // Get context data
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, serrors.Internal("failed to get tenant: %v", err)
    }
    
    userID, err := composables.UseUserID(ctx)
    if err != nil {
        return nil, serrors.Internal("failed to get user: %v", err)
    }
    
    // Validate script content
    if err := s.executionSvc.ValidateSyntax(req.Content); err != nil {
        return nil, serrors.Validation("invalid script syntax: %v", err)
    }
    
    // Additional validation based on type
    if err := s.validateScriptType(req); err != nil {
        return nil, err
    }
    
    // Create domain entity
    newScript, err := script.NewScript(
        tenantID,
        req.Name,
        req.Type,
        req.Content,
        userID,
    )
    if err != nil {
        return nil, err
    }
    
    // Add metadata and tags
    if req.Description != "" {
        newScript = newScript.UpdateDescription(req.Description, userID)
    }
    
    for _, tag := range req.Tags {
        newScript = newScript.AddTag(tag, userID)
    }
    
    if req.Metadata != nil {
        newScript, err = newScript.UpdateMetadata(req.Metadata, userID)
        if err != nil {
            return nil, err
        }
    }
    
    // Save to repository
    created, err := s.repo.Create(ctx, newScript)
    if err != nil {
        return nil, serrors.Internal("failed to create script: %v", err)
    }
    
    // Publish domain event
    event := script.NewScriptCreatedEvent(created, userID)
    s.eventPublisher.Publish(event)
    
    return &created, nil
}

func (s *scriptService) UpdateScript(ctx context.Context, req UpdateScriptRequest) (*script.Script, error) {
    // Check permissions
    if err := s.rbac.CheckPermission(ctx, "scripts.update"); err != nil {
        return nil, serrors.Forbidden("insufficient permissions to update scripts")
    }
    
    userID, err := composables.UseUserID(ctx)
    if err != nil {
        return nil, err
    }
    
    // Get existing script
    existing, err := s.repo.GetByID(ctx, req.ID)
    if err != nil {
        return nil, serrors.NotFound("script not found")
    }
    
    // Track changes for event
    changes := make(map[string]interface{})
    oldVersion := existing.Version()
    
    // Apply updates
    updated := existing
    
    if req.Name != nil && *req.Name != existing.Name() {
        changes["name"] = map[string]interface{}{
            "old": existing.Name(),
            "new": *req.Name,
        }
        updated = updated.UpdateName(*req.Name, userID)
    }
    
    if req.Description != nil && *req.Description != existing.Description() {
        changes["description"] = map[string]interface{}{
            "old": existing.Description(),
            "new": *req.Description,
        }
        updated = updated.UpdateDescription(*req.Description, userID)
    }
    
    if req.Content != nil && *req.Content != existing.Content() {
        // Validate new content
        if err := s.executionSvc.ValidateSyntax(*req.Content); err != nil {
            return nil, serrors.Validation("invalid script syntax: %v", err)
        }
        
        changes["content"] = true // Don't include actual content in event
        updated, err = updated.UpdateContent(*req.Content, userID)
        if err != nil {
            return nil, err
        }
    }
    
    if req.Tags != nil {
        changes["tags"] = map[string]interface{}{
            "old": existing.Tags(),
            "new": req.Tags,
        }
        updated = updated.UpdateTags(req.Tags, userID)
    }
    
    if req.Metadata != nil {
        changes["metadata"] = true
        updated, err = updated.UpdateMetadata(req.Metadata, userID)
        if err != nil {
            return nil, err
        }
    }
    
    // Save updates
    saved, err := s.repo.Update(ctx, updated)
    if err != nil {
        return nil, serrors.Internal("failed to update script: %v", err)
    }
    
    // Publish update event
    if len(changes) > 0 {
        event := &script.ScriptUpdatedEvent{
            ScriptEvent: script.ScriptEvent{
                ScriptID:  saved.ID(),
                TenantID:  saved.TenantID(),
                Timestamp: time.Now(),
                ActorID:   userID,
            },
            OldVersion: oldVersion,
            NewVersion: saved.Version(),
            Changes:    changes,
        }
        s.eventPublisher.Publish(event)
    }
    
    return &saved, nil
}

func (s *scriptService) ExecuteScript(ctx context.Context, id uuid.UUID, params map[string]interface{}) (*ExecutionResult, error) {
    // Check permissions
    if err := s.rbac.CheckPermission(ctx, "scripts.execute"); err != nil {
        return nil, serrors.Forbidden("insufficient permissions to execute scripts")
    }
    
    // Get script
    script, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, serrors.NotFound("script not found")
    }
    
    // Check if enabled
    if !script.Enabled() {
        return nil, serrors.BadRequest("script is disabled")
    }
    
    // Create execution record
    execution := &Execution{
        ID:        uuid.New(),
        ScriptID:  script.ID(),
        Status:    ExecutionStatusRunning,
        StartedAt: time.Now(),
        Params:    params,
    }
    
    // Execute script
    result, err := s.executionSvc.ExecuteWithTimeout(ctx, &script, params, 30*time.Second)
    
    // Update execution record
    execution.CompletedAt = &time.Time{}
    *execution.CompletedAt = time.Now()
    execution.Duration = execution.CompletedAt.Sub(execution.StartedAt)
    
    if err != nil {
        execution.Status = ExecutionStatusFailed
        execution.Error = err.Error()
        
        // Publish failure event
        event := &script.ScriptFailedEvent{
            ScriptEvent: script.ScriptEvent{
                ScriptID:  script.ID(),
                TenantID:  script.TenantID(),
                Timestamp: time.Now(),
                ActorID:   composables.UseUserID(ctx),
            },
            ExecutionID: execution.ID,
            Error:       err.Error(),
        }
        s.eventPublisher.Publish(event)
    } else {
        execution.Status = ExecutionStatusCompleted
        execution.Logs = result.Logs
        
        // Publish success event
        event := &script.ScriptExecutedEvent{
            ScriptEvent: script.ScriptEvent{
                ScriptID:  script.ID(),
                TenantID:  script.TenantID(),
                Timestamp: time.Now(),
                ActorID:   composables.UseUserID(ctx),
            },
            ExecutionID: execution.ID,
            Duration:    execution.Duration,
            Success:     result.Success,
        }
        s.eventPublisher.Publish(event)
    }
    
    // Save execution history
    s.saveExecution(ctx, execution)
    
    return result, err
}

func (s *scriptService) validateScriptType(req CreateScriptRequest) error {
    switch req.Type {
    case value_objects.ScriptTypeCron:
        // Validate cron expression in metadata
        if req.Metadata == nil || req.Metadata["schedule"] == nil {
            return serrors.Validation("cron scripts require schedule in metadata")
        }
        
        schedule, ok := req.Metadata["schedule"].(string)
        if !ok {
            return serrors.Validation("schedule must be a string")
        }
        
        _, err := value_objects.NewCronSchedule(schedule, "UTC")
        if err != nil {
            return serrors.Validation("invalid cron schedule: %v", err)
        }
        
    case value_objects.ScriptTypeHTTPEndpoint:
        // Validate HTTP metadata
        if req.Metadata == nil || req.Metadata["route"] == nil {
            return serrors.Validation("HTTP endpoint scripts require route in metadata")
        }
        
        route, ok := req.Metadata["route"].(map[string]interface{})
        if !ok {
            return serrors.Validation("route must be an object")
        }
        
        if route["method"] == nil || route["path"] == nil {
            return serrors.Validation("route must have method and path")
        }
        
    case value_objects.ScriptTypeEventHandler:
        // Validate event subscription
        if req.Metadata == nil || req.Metadata["events"] == nil {
            return serrors.Validation("event handler scripts require events in metadata")
        }
    }
    
    return nil
}
```

#### 3. Implement Execution Service
Create `modules/scripts/services/execution_service.go`:
```go
package services

import (
    "context"
    "fmt"
    "sync"
    "time"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/pkg/jsruntime"
)

type executionService struct {
    runtime      *jsruntime.Runtime
    executions   sync.Map // In-memory execution tracking
    maxConcurrent int
    semaphore    chan struct{}
}

func NewExecutionService(runtime *jsruntime.Runtime, maxConcurrent int) ExecutionService {
    return &executionService{
        runtime:       runtime,
        maxConcurrent: maxConcurrent,
        semaphore:    make(chan struct{}, maxConcurrent),
    }
}

func (s *executionService) Execute(ctx context.Context, script *script.Script, params map[string]interface{}) (*ExecutionResult, error) {
    return s.ExecuteWithTimeout(ctx, script, params, 30*time.Second)
}

func (s *executionService) ExecuteWithTimeout(ctx context.Context, script *script.Script, params map[string]interface{}, timeout time.Duration) (*ExecutionResult, error) {
    // Acquire semaphore to limit concurrent executions
    select {
    case s.semaphore <- struct{}{}:
        defer func() { <-s.semaphore }()
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        return nil, fmt.Errorf("too many concurrent executions")
    }
    
    // Create execution context with timeout
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    // Prepare execution environment
    logs := &logCollector{
        logs: make([]LogEntry, 0),
        mu:   sync.Mutex{},
    }
    
    start := time.Now()
    
    // Execute script with enhanced context
    result, err := s.runtime.ExecuteWithContext(ctx, script.Content(), func(vm *goja.Runtime) error {
        // Inject parameters
        if params != nil {
            vm.Set("params", params)
        }
        
        // Inject script metadata
        vm.Set("script", map[string]interface{}{
            "id":      script.ID().String(),
            "name":    script.Name(),
            "type":    script.Type().String(),
            "version": script.Version(),
        })
        
        // Inject logger
        s.injectLogger(vm, logs)
        
        return nil
    })
    
    duration := time.Since(start)
    
    if err != nil {
        // Check if timeout
        if ctx.Err() == context.DeadlineExceeded {
            return &ExecutionResult{
                Success:  false,
                Error:    "execution timeout",
                Duration: duration,
                Logs:     logs.GetLogs(),
            }, fmt.Errorf("script execution timeout after %v", timeout)
        }
        
        return &ExecutionResult{
            Success:  false,
            Error:    err.Error(),
            Duration: duration,
            Logs:     logs.GetLogs(),
        }, err
    }
    
    return &ExecutionResult{
        Success:  true,
        Output:   result,
        Duration: duration,
        Logs:     logs.GetLogs(),
    }, nil
}

func (s *executionService) ValidateSyntax(content string) error {
    // Use Goja to parse and validate syntax
    _, err := goja.Compile("validation", content, true)
    if err != nil {
        return fmt.Errorf("syntax error: %v", err)
    }
    
    // Additional validation
    if err := s.validateSecurity(content); err != nil {
        return err
    }
    
    return nil
}

func (s *executionService) validateSecurity(content string) error {
    // Check for dangerous patterns
    dangerousPatterns := []string{
        "eval(",
        "Function(",
        "require('child_process')",
        "require('fs')",
        "__proto__",
        "constructor.constructor",
    }
    
    for _, pattern := range dangerousPatterns {
        if strings.Contains(content, pattern) {
            return fmt.Errorf("dangerous pattern detected: %s", pattern)
        }
    }
    
    return nil
}

func (s *executionService) injectLogger(vm *goja.Runtime, collector *logCollector) {
    console := vm.NewObject()
    
    // Log levels
    logLevels := []string{"debug", "info", "warn", "error"}
    
    for _, level := range logLevels {
        level := level // Capture for closure
        console.Set(level, func(args ...interface{}) {
            collector.Log(level, args...)
        })
    }
    
    vm.Set("console", console)
}

// Log collector for capturing script output
type logCollector struct {
    logs []LogEntry
    mu   sync.Mutex
}

type LogEntry struct {
    Level     string
    Message   string
    Timestamp time.Time
    Data      []interface{}
}

func (c *logCollector) Log(level string, args ...interface{}) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Format message
    var message string
    if len(args) > 0 {
        message = fmt.Sprint(args[0])
    }
    
    entry := LogEntry{
        Level:     level,
        Message:   message,
        Timestamp: time.Now(),
        Data:      args[1:],
    }
    
    c.logs = append(c.logs, entry)
}

func (c *logCollector) GetLogs() []LogEntry {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    return append([]LogEntry{}, c.logs...)
}
```

#### 4. Add RBAC Permissions
Create `modules/scripts/permissions/constants.go`:
```go
package permissions

const (
    // Script permissions
    ScriptsView    = "scripts.view"
    ScriptsCreate  = "scripts.create"
    ScriptsUpdate  = "scripts.update"
    ScriptsDelete  = "scripts.delete"
    ScriptsExecute = "scripts.execute"
    
    // Execution permissions
    ExecutionsView = "executions.view"
    
    // Admin permissions
    ScriptsManage = "scripts.manage" // Full access
)

// Permission groups
var (
    ScriptViewerPermissions = []string{
        ScriptsView,
        ExecutionsView,
    }
    
    ScriptDeveloperPermissions = []string{
        ScriptsView,
        ScriptsCreate,
        ScriptsUpdate,
        ScriptsExecute,
        ExecutionsView,
    }
    
    ScriptAdminPermissions = []string{
        ScriptsManage,
    }
)
```

### Testing Requirements

Create `modules/scripts/services/script_service_test.go`:
```go
package services_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/require"
    "github.com/stretchr/testify/mock"
)

func TestScriptService_CreateScript(t *testing.T) {
    // Setup mocks
    mockRepo := new(MockScriptRepository)
    mockExecSvc := new(MockExecutionService)
    mockEventBus := new(MockEventBus)
    mockRBAC := new(MockRBAC)
    
    svc := NewScriptService(mockRepo, mockExecSvc, mockEventBus, mockRBAC)
    
    ctx := setupTestContext()
    
    t.Run("successful creation", func(t *testing.T) {
        req := CreateScriptRequest{
            Name:    "Test Script",
            Type:    value_objects.ScriptTypeFunction,
            Content: "console.log('test')",
            Tags:    []string{"test", "demo"},
        }
        
        // Setup expectations
        mockRBAC.On("CheckPermission", ctx, "scripts.create").Return(nil)
        mockExecSvc.On("ValidateSyntax", req.Content).Return(nil)
        mockRepo.On("Create", ctx, mock.Anything).Return(mock.Anything, nil)
        mockEventBus.On("Publish", mock.Anything).Return(nil)
        
        // Execute
        script, err := svc.CreateScript(ctx, req)
        
        // Assert
        require.NoError(t, err)
        require.NotNil(t, script)
        require.Equal(t, req.Name, script.Name())
        require.Equal(t, req.Type, script.Type())
        
        // Verify event published
        mockEventBus.AssertCalled(t, "Publish", mock.MatchedBy(func(event interface{}) bool {
            _, ok := event.(*script.ScriptCreatedEvent)
            return ok
        }))
    })
    
    t.Run("permission denied", func(t *testing.T) {
        mockRBAC.On("CheckPermission", ctx, "scripts.create").Return(rbac.ErrPermissionDenied)
        
        _, err := svc.CreateScript(ctx, CreateScriptRequest{})
        
        require.Error(t, err)
        require.Contains(t, err.Error(), "insufficient permissions")
    })
    
    t.Run("invalid syntax", func(t *testing.T) {
        req := CreateScriptRequest{
            Content: "invalid javascript {{",
        }
        
        mockRBAC.On("CheckPermission", ctx, "scripts.create").Return(nil)
        mockExecSvc.On("ValidateSyntax", req.Content).Return(errors.New("syntax error"))
        
        _, err := svc.CreateScript(ctx, req)
        
        require.Error(t, err)
        require.Contains(t, err.Error(), "invalid script syntax")
    })
}

func TestScriptService_ExecuteScript(t *testing.T) {
    // Test execution with mocks
    mockRepo := new(MockScriptRepository)
    mockExecSvc := new(MockExecutionService)
    mockEventBus := new(MockEventBus)
    mockRBAC := new(MockRBAC)
    
    svc := NewScriptService(mockRepo, mockExecSvc, mockEventBus, mockRBAC)
    
    ctx := setupTestContext()
    scriptID := uuid.New()
    
    t.Run("successful execution", func(t *testing.T) {
        testScript := createTestScript(scriptID, true)
        params := map[string]interface{}{"input": "test"}
        
        expectedResult := &ExecutionResult{
            Success: true,
            Output:  "test output",
            Duration: 100 * time.Millisecond,
            Logs: []LogEntry{
                {Level: "info", Message: "Script started"},
            },
        }
        
        // Setup expectations
        mockRBAC.On("CheckPermission", ctx, "scripts.execute").Return(nil)
        mockRepo.On("GetByID", ctx, scriptID).Return(testScript, nil)
        mockExecSvc.On("ExecuteWithTimeout", ctx, testScript, params, 30*time.Second).Return(expectedResult, nil)
        mockEventBus.On("Publish", mock.Anything).Return(nil)
        
        // Execute
        result, err := svc.ExecuteScript(ctx, scriptID, params)
        
        // Assert
        require.NoError(t, err)
        require.True(t, result.Success)
        require.Equal(t, expectedResult.Output, result.Output)
        
        // Verify success event published
        mockEventBus.AssertCalled(t, "Publish", mock.MatchedBy(func(event interface{}) bool {
            e, ok := event.(*script.ScriptExecutedEvent)
            return ok && e.Success
        }))
    })
    
    t.Run("disabled script", func(t *testing.T) {
        disabledScript := createTestScript(scriptID, false)
        
        mockRBAC.On("CheckPermission", ctx, "scripts.execute").Return(nil)
        mockRepo.On("GetByID", ctx, scriptID).Return(disabledScript, nil)
        
        _, err := svc.ExecuteScript(ctx, scriptID, nil)
        
        require.Error(t, err)
        require.Contains(t, err.Error(), "script is disabled")
    })
    
    t.Run("execution failure", func(t *testing.T) {
        testScript := createTestScript(scriptID, true)
        
        mockRBAC.On("CheckPermission", ctx, "scripts.execute").Return(nil)
        mockRepo.On("GetByID", ctx, scriptID).Return(testScript, nil)
        mockExecSvc.On("ExecuteWithTimeout", ctx, testScript, mock.Anything, 30*time.Second).
            Return(nil, errors.New("runtime error"))
        mockEventBus.On("Publish", mock.Anything).Return(nil)
        
        _, err := svc.ExecuteScript(ctx, scriptID, nil)
        
        require.Error(t, err)
        
        // Verify failure event published
        mockEventBus.AssertCalled(t, "Publish", mock.MatchedBy(func(event interface{}) bool {
            _, ok := event.(*script.ScriptFailedEvent)
            return ok
        }))
    })
}

func TestExecutionService_ValidateSyntax(t *testing.T) {
    runtime := jsruntime.NewRuntime(&jsruntime.Config{PoolSize: 1})
    execSvc := NewExecutionService(runtime, 10)
    
    tests := []struct {
        name    string
        content string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid syntax",
            content: "const x = 1; console.log(x);",
            wantErr: false,
        },
        {
            name:    "invalid syntax",
            content: "const x = {",
            wantErr: true,
            errMsg:  "syntax error",
        },
        {
            name:    "dangerous eval",
            content: "eval('malicious code')",
            wantErr: true,
            errMsg:  "dangerous pattern",
        },
        {
            name:    "dangerous Function constructor",
            content: "new Function('return this')()",
            wantErr: true,
            errMsg:  "dangerous pattern",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := execSvc.ValidateSyntax(tt.content)
            
            if tt.wantErr {
                require.Error(t, err)
                require.Contains(t, err.Error(), tt.errMsg)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Deliverables Checklist
- [ ] Complete ScriptService implementation
- [ ] RBAC integration for all operations
- [ ] Domain event publishing
- [ ] ExecutionService with security validation
- [ ] Execution history tracking
- [ ] Comprehensive validation logic
- [ ] Unit tests with mocks
- [ ] Integration tests
- [ ] Service documentation

## Task 3.2: JavaScript SDK APIs (Day 2)

### Objectives
- Create the `sdk` object available in all scripts
- Implement safe HTTP client
- Add database query capabilities with tenant isolation
- Create cache access for Redis
- Implement structured logging
- Generate TypeScript definitions

### Detailed Steps

#### 1. Create SDK Registry
Create `pkg/jsruntime/apis/registry.go`:
```go
package apis

import (
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/application"
)

type APIRegistry struct {
    apis map[string]API
    app  application.Application
}

type API interface {
    Name() string
    Register(vm *goja.Runtime, ctx context.Context) error
}

func NewAPIRegistry(app application.Application) *APIRegistry {
    registry := &APIRegistry{
        apis: make(map[string]API),
        app:  app,
    }
    
    // Register all APIs
    registry.Register(NewHTTPAPI(app))
    registry.Register(NewDatabaseAPI(app))
    registry.Register(NewCacheAPI(app))
    registry.Register(NewLogAPI(app))
    registry.Register(NewUtilsAPI(app))
    
    return registry
}

func (r *APIRegistry) Register(api API) {
    r.apis[api.Name()] = api
}

func (r *APIRegistry) InstallAll(vm *goja.Runtime, ctx context.Context) error {
    // Create root SDK object
    sdk := vm.NewObject()
    
    // Install each API
    for name, api := range r.apis {
        if err := api.Register(vm, ctx); err != nil {
            return fmt.Errorf("failed to register %s API: %w", name, err)
        }
    }
    
    // Make SDK globally available
    vm.Set("sdk", sdk)
    
    return nil
}
```

#### 2. Implement HTTP API
Create `pkg/jsruntime/apis/http.go`:
```go
package apis

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
    "github.com/dop251/goja"
)

type HTTPAPI struct {
    client *http.Client
    app    application.Application
}

func NewHTTPAPI(app application.Application) API {
    return &HTTPAPI{
        client: &http.Client{
            Timeout: 30 * time.Second,
            CheckRedirect: func(req *http.Request, via []*http.Request) error {
                if len(via) >= 10 {
                    return fmt.Errorf("too many redirects")
                }
                return nil
            },
        },
        app: app,
    }
}

func (api *HTTPAPI) Name() string {
    return "http"
}

func (api *HTTPAPI) Register(vm *goja.Runtime, ctx context.Context) error {
    http := vm.NewObject()
    
    // GET request
    http.Set("get", func(url string, options ...map[string]interface{}) (interface{}, error) {
        opts := api.parseOptions(options...)
        return api.request(ctx, "GET", url, nil, opts)
    })
    
    // POST request
    http.Set("post", func(url string, body interface{}, options ...map[string]interface{}) (interface{}, error) {
        opts := api.parseOptions(options...)
        return api.request(ctx, "POST", url, body, opts)
    })
    
    // PUT request
    http.Set("put", func(url string, body interface{}, options ...map[string]interface{}) (interface{}, error) {
        opts := api.parseOptions(options...)
        return api.request(ctx, "PUT", url, body, opts)
    })
    
    // DELETE request
    http.Set("delete", func(url string, options ...map[string]interface{}) (interface{}, error) {
        opts := api.parseOptions(options...)
        return api.request(ctx, "DELETE", url, nil, opts)
    })
    
    // Generic request
    http.Set("request", func(method, url string, options map[string]interface{}) (interface{}, error) {
        body := options["body"]
        delete(options, "body")
        return api.request(ctx, method, url, body, options)
    })
    
    // Get SDK object and set http
    if sdk := vm.Get("sdk"); sdk != nil {
        sdk.(*goja.Object).Set("http", http)
    }
    
    return nil
}

func (api *HTTPAPI) request(ctx context.Context, method, url string, body interface{}, options map[string]interface{}) (interface{}, error) {
    // Validate URL
    if err := api.validateURL(url); err != nil {
        return nil, err
    }
    
    // Create request
    var bodyReader io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, fmt.Errorf("failed to marshal body: %w", err)
        }
        bodyReader = bytes.NewReader(jsonBody)
    }
    
    req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
    if err != nil {
        return nil, err
    }
    
    // Set headers
    req.Header.Set("User-Agent", "IOTA-Script/1.0")
    req.Header.Set("Content-Type", "application/json")
    
    if headers, ok := options["headers"].(map[string]interface{}); ok {
        for k, v := range headers {
            req.Header.Set(k, fmt.Sprint(v))
        }
    }
    
    // Add timeout
    if timeout, ok := options["timeout"].(float64); ok {
        ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
        defer cancel()
        req = req.WithContext(ctx)
    }
    
    // Execute request
    resp, err := api.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()
    
    // Read response
    respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }
    
    // Parse response
    result := map[string]interface{}{
        "status":     resp.StatusCode,
        "statusText": resp.Status,
        "headers":    api.headersToMap(resp.Header),
    }
    
    // Try to parse JSON
    var jsonBody interface{}
    if err := json.Unmarshal(respBody, &jsonBody); err == nil {
        result["data"] = jsonBody
    } else {
        result["text"] = string(respBody)
    }
    
    return result, nil
}

func (api *HTTPAPI) validateURL(url string) error {
    // Prevent SSRF attacks
    blockedHosts := []string{
        "localhost",
        "127.0.0.1",
        "0.0.0.0",
        "169.254.169.254", // AWS metadata
    }
    
    parsed, err := url.Parse(url)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }
    
    for _, blocked := range blockedHosts {
        if strings.Contains(parsed.Host, blocked) {
            return fmt.Errorf("blocked host: %s", parsed.Host)
        }
    }
    
    // Only allow HTTP/HTTPS
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("only HTTP/HTTPS allowed")
    }
    
    return nil
}
```

#### 3. Implement Database API
Create `pkg/jsruntime/apis/database.go`:
```go
package apis

import (
    "context"
    "database/sql"
    "fmt"
    "github.com/dop251/goja"
    "github.com/jmoiron/sqlx"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type DatabaseAPI struct {
    db  *sqlx.DB
    app application.Application
}

func NewDatabaseAPI(app application.Application) API {
    return &DatabaseAPI{
        db:  app.DB(),
        app: app,
    }
}

func (api *DatabaseAPI) Name() string {
    return "db"
}

func (api *DatabaseAPI) Register(vm *goja.Runtime, ctx context.Context) error {
    db := vm.NewObject()
    
    // Query method - returns multiple rows
    db.Set("query", func(query string, args ...interface{}) ([]map[string]interface{}, error) {
        // Ensure tenant isolation
        safeQuery, err := api.ensureTenantIsolation(ctx, query)
        if err != nil {
            return nil, err
        }
        
        // Validate query
        if err := api.validateQuery(safeQuery); err != nil {
            return nil, err
        }
        
        // Execute query
        rows, err := api.db.QueryxContext(ctx, safeQuery, args...)
        if err != nil {
            return nil, fmt.Errorf("query failed: %w", err)
        }
        defer rows.Close()
        
        // Convert to maps
        var results []map[string]interface{}
        for rows.Next() {
            row := make(map[string]interface{})
            if err := rows.MapScan(row); err != nil {
                return nil, err
            }
            results = append(results, api.sanitizeRow(row))
        }
        
        return results, nil
    })
    
    // QueryOne - returns single row
    db.Set("queryOne", func(query string, args ...interface{}) (map[string]interface{}, error) {
        results, err := db.Get("query").(func(string, ...interface{}) ([]map[string]interface{}, error))(query, args...)
        if err != nil {
            return nil, err
        }
        
        if len(results) == 0 {
            return nil, sql.ErrNoRows
        }
        
        return results[0], nil
    })
    
    // Execute - for INSERT/UPDATE/DELETE
    db.Set("execute", func(query string, args ...interface{}) (map[string]interface{}, error) {
        // Validate as modification query
        if err := api.validateModificationQuery(query); err != nil {
            return nil, err
        }
        
        // Ensure tenant isolation
        safeQuery, err := api.ensureTenantIsolation(ctx, query)
        if err != nil {
            return nil, err
        }
        
        // Execute
        result, err := api.db.ExecContext(ctx, safeQuery, args...)
        if err != nil {
            return nil, fmt.Errorf("execution failed: %w", err)
        }
        
        affected, _ := result.RowsAffected()
        lastID, _ := result.LastInsertId()
        
        return map[string]interface{}{
            "affectedRows": affected,
            "lastInsertId": lastID,
        }, nil
    })
    
    // Transaction support
    db.Set("transaction", func(callback goja.Callable) (interface{}, error) {
        tx, err := api.db.BeginTxx(ctx, nil)
        if err != nil {
            return nil, err
        }
        
        // Create transaction API
        txAPI := api.createTransactionAPI(vm, ctx, tx)
        
        // Execute callback
        result, err := callback(goja.Undefined(), vm.ToValue(txAPI))
        if err != nil {
            tx.Rollback()
            return nil, err
        }
        
        // Commit transaction
        if err := tx.Commit(); err != nil {
            return nil, err
        }
        
        return result, nil
    })
    
    // Get SDK object and set db
    if sdk := vm.Get("sdk"); sdk != nil {
        sdk.(*goja.Object).Set("db", db)
    }
    
    return nil
}

func (api *DatabaseAPI) ensureTenantIsolation(ctx context.Context, query string) (string, error) {
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return "", fmt.Errorf("no tenant context")
    }
    
    // For SELECT queries, add tenant filter
    if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(query)), "SELECT") {
        // Simple approach - in production use proper SQL parser
        if !strings.Contains(query, "WHERE") {
            query += fmt.Sprintf(" WHERE tenant_id = '%s'", tenantID)
        } else {
            query += fmt.Sprintf(" AND tenant_id = '%s'", tenantID)
        }
    }
    
    return query, nil
}

func (api *DatabaseAPI) validateQuery(query string) error {
    // Prevent dangerous queries
    dangerous := []string{
        "DROP",
        "TRUNCATE",
        "CREATE",
        "ALTER",
        "GRANT",
        "REVOKE",
    }
    
    upperQuery := strings.ToUpper(query)
    for _, keyword := range dangerous {
        if strings.Contains(upperQuery, keyword) {
            return fmt.Errorf("dangerous SQL keyword: %s", keyword)
        }
    }
    
    return nil
}

func (api *DatabaseAPI) sanitizeRow(row map[string]interface{}) map[string]interface{} {
    // Convert database types to JavaScript-friendly types
    for k, v := range row {
        switch val := v.(type) {
        case []byte:
            row[k] = string(val)
        case sql.NullString:
            if val.Valid {
                row[k] = val.String
            } else {
                row[k] = nil
            }
        case sql.NullBool:
            if val.Valid {
                row[k] = val.Bool
            } else {
                row[k] = nil
            }
        case sql.NullInt64:
            if val.Valid {
                row[k] = val.Int64
            } else {
                row[k] = nil
            }
        case sql.NullFloat64:
            if val.Valid {
                row[k] = val.Float64
            } else {
                row[k] = nil
            }
        }
    }
    
    return row
}
```

#### 4. Implement Cache API
Create `pkg/jsruntime/apis/cache.go`:
```go
package apis

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    "github.com/dop251/goja"
    "github.com/redis/go-redis/v9"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type CacheAPI struct {
    redis *redis.Client
    app   application.Application
}

func NewCacheAPI(app application.Application) API {
    return &CacheAPI{
        redis: app.Redis(),
        app:   app,
    }
}

func (api *CacheAPI) Name() string {
    return "cache"
}

func (api *CacheAPI) Register(vm *goja.Runtime, ctx context.Context) error {
    cache := vm.NewObject()
    
    // Get value
    cache.Set("get", func(key string) (interface{}, error) {
        fullKey := api.buildKey(ctx, key)
        
        val, err := api.redis.Get(ctx, fullKey).Result()
        if err == redis.Nil {
            return nil, nil
        }
        if err != nil {
            return nil, err
        }
        
        // Try to parse as JSON
        var result interface{}
        if err := json.Unmarshal([]byte(val), &result); err == nil {
            return result, nil
        }
        
        return val, nil
    })
    
    // Set value
    cache.Set("set", func(key string, value interface{}, ttlSeconds ...int) error {
        fullKey := api.buildKey(ctx, key)
        
        // Serialize value
        var data string
        switch v := value.(type) {
        case string:
            data = v
        default:
            jsonData, err := json.Marshal(v)
            if err != nil {
                return fmt.Errorf("failed to serialize value: %w", err)
            }
            data = string(jsonData)
        }
        
        // Set with optional TTL
        ttl := time.Duration(0)
        if len(ttlSeconds) > 0 {
            ttl = time.Duration(ttlSeconds[0]) * time.Second
        }
        
        return api.redis.Set(ctx, fullKey, data, ttl).Err()
    })
    
    // Delete key
    cache.Set("delete", func(keys ...string) error {
        fullKeys := make([]string, len(keys))
        for i, key := range keys {
            fullKeys[i] = api.buildKey(ctx, key)
        }
        
        return api.redis.Del(ctx, fullKeys...).Err()
    })
    
    // Check existence
    cache.Set("exists", func(key string) (bool, error) {
        fullKey := api.buildKey(ctx, key)
        
        count, err := api.redis.Exists(ctx, fullKey).Result()
        if err != nil {
            return false, err
        }
        
        return count > 0, nil
    })
    
    // Increment
    cache.Set("increment", func(key string, delta ...int64) (int64, error) {
        fullKey := api.buildKey(ctx, key)
        
        incr := int64(1)
        if len(delta) > 0 {
            incr = delta[0]
        }
        
        return api.redis.IncrBy(ctx, fullKey, incr).Result()
    })
    
    // Set expiration
    cache.Set("expire", func(key string, seconds int) error {
        fullKey := api.buildKey(ctx, key)
        return api.redis.Expire(ctx, fullKey, time.Duration(seconds)*time.Second).Err()
    })
    
    // Get SDK object and set cache
    if sdk := vm.Get("sdk"); sdk != nil {
        sdk.(*goja.Object).Set("cache", cache)
    }
    
    return nil
}

func (api *CacheAPI) buildKey(ctx context.Context, key string) string {
    // Prefix with tenant ID for isolation
    tenantID, _ := composables.UseTenantID(ctx)
    return fmt.Sprintf("script:%s:%s", tenantID, key)
}
```

#### 5. Implement Log API
Create `pkg/jsruntime/apis/log.go`:
```go
package apis

import (
    "context"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "go.uber.org/zap"
)

type LogAPI struct {
    logger *zap.Logger
    app    application.Application
}

func NewLogAPI(app application.Application) API {
    return &LogAPI{
        logger: app.Logger(),
        app:    app,
    }
}

func (api *LogAPI) Name() string {
    return "log"
}

func (api *LogAPI) Register(vm *goja.Runtime, ctx context.Context) error {
    log := vm.NewObject()
    
    // Create logger with context
    logger := api.enrichLogger(ctx)
    
    // Log levels
    log.Set("debug", func(message string, fields ...interface{}) {
        logger.Debug(message, api.parseFields(fields...)...)
    })
    
    log.Set("info", func(message string, fields ...interface{}) {
        logger.Info(message, api.parseFields(fields...)...)
    })
    
    log.Set("warn", func(message string, fields ...interface{}) {
        logger.Warn(message, api.parseFields(fields...)...)
    })
    
    log.Set("error", func(message string, fields ...interface{}) {
        logger.Error(message, api.parseFields(fields...)...)
    })
    
    // Structured logging
    log.Set("with", func(fields map[string]interface{}) *goja.Object {
        newLogger := logger
        for k, v := range fields {
            newLogger = newLogger.With(zap.Any(k, v))
        }
        
        // Return new log object with enriched logger
        return api.createLogObject(vm, newLogger)
    })
    
    // Get SDK object and set log
    if sdk := vm.Get("sdk"); sdk != nil {
        sdk.(*goja.Object).Set("log", log)
    }
    
    return nil
}

func (api *LogAPI) enrichLogger(ctx context.Context) *zap.Logger {
    logger := api.logger
    
    // Add tenant context
    if tenantID, err := composables.UseTenantID(ctx); err == nil {
        logger = logger.With(zap.String("tenant_id", tenantID.String()))
    }
    
    // Add user context
    if userID, err := composables.UseUserID(ctx); err == nil {
        logger = logger.With(zap.String("user_id", userID.String()))
    }
    
    // Add request ID
    if reqID := ctx.Value("requestID"); reqID != nil {
        logger = logger.With(zap.String("request_id", reqID.(string)))
    }
    
    // Add script context
    if scriptID := ctx.Value("scriptID"); scriptID != nil {
        logger = logger.With(zap.String("script_id", scriptID.(string)))
    }
    
    return logger
}

func (api *LogAPI) parseFields(fields ...interface{}) []zap.Field {
    zapFields := []zap.Field{}
    
    for i := 0; i < len(fields); i += 2 {
        if i+1 < len(fields) {
            key := fmt.Sprint(fields[i])
            zapFields = append(zapFields, zap.Any(key, fields[i+1]))
        }
    }
    
    return zapFields
}
```

#### 6. Generate TypeScript Definitions
Create `pkg/jsruntime/apis/typescript/generate.go`:
```go
package typescript

import (
    "fmt"
    "os"
    "text/template"
)

const sdkTemplate = `
// IOTA SDK TypeScript Definitions
// Auto-generated - do not edit

declare namespace sdk {
    namespace http {
        interface RequestOptions {
            headers?: Record<string, string>;
            timeout?: number;
        }
        
        interface Response {
            status: number;
            statusText: string;
            headers: Record<string, string>;
            data?: any;
            text?: string;
        }
        
        function get(url: string, options?: RequestOptions): Promise<Response>;
        function post(url: string, body: any, options?: RequestOptions): Promise<Response>;
        function put(url: string, body: any, options?: RequestOptions): Promise<Response>;
        function delete(url: string, options?: RequestOptions): Promise<Response>;
        function request(method: string, url: string, options: RequestOptions & { body?: any }): Promise<Response>;
    }
    
    namespace db {
        interface QueryResult {
            affectedRows: number;
            lastInsertId: number;
        }
        
        interface Transaction {
            query<T = any>(sql: string, ...args: any[]): T[];
            queryOne<T = any>(sql: string, ...args: any[]): T;
            execute(sql: string, ...args: any[]): QueryResult;
            rollback(): void;
        }
        
        function query<T = any>(sql: string, ...args: any[]): T[];
        function queryOne<T = any>(sql: string, ...args: any[]): T;
        function execute(sql: string, ...args: any[]): QueryResult;
        function transaction<T>(callback: (tx: Transaction) => T): T;
    }
    
    namespace cache {
        function get<T = any>(key: string): T | null;
        function set(key: string, value: any, ttlSeconds?: number): void;
        function delete(...keys: string[]): void;
        function exists(key: string): boolean;
        function increment(key: string, delta?: number): number;
        function expire(key: string, seconds: number): void;
    }
    
    namespace log {
        interface Logger {
            debug(message: string, ...fields: any[]): void;
            info(message: string, ...fields: any[]): void;
            warn(message: string, ...fields: any[]): void;
            error(message: string, ...fields: any[]): void;
            with(fields: Record<string, any>): Logger;
        }
        
        const debug: Logger["debug"];
        const info: Logger["info"];
        const warn: Logger["warn"];
        const error: Logger["error"];
        function with(fields: Record<string, any>): Logger;
    }
}

// Global context
declare const context: {
    tenantID: string;
    tenant: {
        id: string;
        name: string;
    };
    user: {
        id: string;
        email: string;
        firstName: string;
        lastName: string;
        permissions: string[];
    };
    requestID: string;
    locale: string;
};

// Script metadata
declare const script: {
    id: string;
    name: string;
    type: "cron" | "http_endpoint" | "event_handler" | "function";
    version: number;
};

// Script parameters (when executed with params)
declare const params: Record<string, any>;

// Console (enhanced)
declare namespace console {
    function log(...args: any[]): void;
    function debug(...args: any[]): void;
    function info(...args: any[]): void;
    function warn(...args: any[]): void;
    function error(...args: any[]): void;
}
`

func GenerateTypeScriptDefinitions(outputPath string) error {
    file, err := os.Create(outputPath)
    if err != nil {
        return err
    }
    defer file.Close()
    
    _, err = file.WriteString(sdkTemplate)
    return err
}
```

### Testing Requirements

Create `pkg/jsruntime/apis/integration_test.go`:
```go
func TestSDKIntegration(t *testing.T) {
    // Setup test environment
    app := setupTestApplication()
    runtime := jsruntime.NewRuntime(&jsruntime.Config{PoolSize: 1})
    registry := NewAPIRegistry(app)
    
    ctx := setupTestContext()
    
    t.Run("HTTP API", func(t *testing.T) {
        // Start test HTTP server
        ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Type", "application/json")
            json.NewEncoder(w).Encode(map[string]string{
                "message": "Hello from test server",
                "method":  r.Method,
            })
        }))
        defer ts.Close()
        
        result, err := runtime.ExecuteWithContext(ctx, fmt.Sprintf(`
            const response = await sdk.http.get("%s");
            response.data;
        `, ts.URL), registry.InstallAll)
        
        require.NoError(t, err)
        
        data := result.(map[string]interface{})
        require.Equal(t, "Hello from test server", data["message"])
        require.Equal(t, "GET", data["method"])
    })
    
    t.Run("Database API with tenant isolation", func(t *testing.T) {
        // Insert test data
        tenantID, _ := composables.UseTenantID(ctx)
        app.DB().ExecContext(ctx, 
            "INSERT INTO test_table (id, tenant_id, name) VALUES ($1, $2, $3)",
            uuid.New(), tenantID, "Test Item")
        
        // Insert data for another tenant (should not be visible)
        otherTenantID := uuid.New()
        app.DB().ExecContext(ctx,
            "INSERT INTO test_table (id, tenant_id, name) VALUES ($1, $2, $3)",
            uuid.New(), otherTenantID, "Other Tenant Item")
        
        result, err := runtime.ExecuteWithContext(ctx, `
            const items = sdk.db.query("SELECT name FROM test_table");
            items.map(item => item.name);
        `, registry.InstallAll)
        
        require.NoError(t, err)
        
        names := result.([]interface{})
        require.Len(t, names, 1)
        require.Equal(t, "Test Item", names[0])
    })
    
    t.Run("Cache API", func(t *testing.T) {
        _, err := runtime.ExecuteWithContext(ctx, `
            // Set value
            sdk.cache.set("test-key", { foo: "bar" }, 60);
            
            // Get value
            const value = sdk.cache.get("test-key");
            if (value.foo !== "bar") {
                throw new Error("Cache value mismatch");
            }
            
            // Increment
            sdk.cache.set("counter", 0);
            const count = sdk.cache.increment("counter", 5);
            if (count !== 5) {
                throw new Error("Increment failed");
            }
            
            // Delete
            sdk.cache.delete("test-key", "counter");
        `, registry.InstallAll)
        
        require.NoError(t, err)
    })
    
    t.Run("Log API", func(t *testing.T) {
        // Capture logs
        var logs []string
        
        _, err := runtime.ExecuteWithContext(ctx, `
            sdk.log.info("Test log message", "key1", "value1", "key2", 123);
            
            const logger = sdk.log.with({ requestID: "test-123" });
            logger.warn("Warning with context");
        `, registry.InstallAll)
        
        require.NoError(t, err)
        
        // Verify logs contain context
        require.Contains(t, logs[0], "Test log message")
        require.Contains(t, logs[0], "tenant_id")
        require.Contains(t, logs[0], "user_id")
        
        require.Contains(t, logs[1], "Warning with context")
        require.Contains(t, logs[1], "requestID")
    })
}
```

### Deliverables Checklist
- [ ] API Registry system
- [ ] HTTP API with SSRF protection
- [ ] Database API with tenant isolation
- [ ] Cache API with Redis integration
- [ ] Structured logging API
- [ ] Context integration in all APIs
- [ ] TypeScript definitions
- [ ] Integration tests
- [ ] API documentation and examples

## Success Criteria
1. Service layer properly orchestrates business logic
2. All operations check RBAC permissions
3. Domain events published for audit trail
4. Script validation prevents dangerous code
5. SDK APIs provide safe access to resources
6. Tenant isolation enforced in all APIs
7. TypeScript definitions enable IDE support
8. All tests pass with >80% coverage

## Notes for Next Phase
- Execution service will be enhanced with queue support
- Consider adding more SDK APIs (email, SMS, etc.)
- Script templates can use TypeScript definitions
- Performance monitoring needed for SDK calls
- Rate limiting for external API calls may be needed