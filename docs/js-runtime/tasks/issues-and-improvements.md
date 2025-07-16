# Issues and Improvements for JS Runtime Integration

## Phase 1: Core Runtime Foundation

### Issues Found

1. **Memory Limit Not Enforced**
   - `DefaultVMFactory` has `memoryLimit` field but never uses it
   - No actual memory enforcement mechanism in Goja

2. **Timeout Implementation Flaw**
   - `time.AfterFunc` in factory creates a timer that persists even after VM is returned to pool
   - Could cause unexpected interrupts on reused VMs

3. **Error Handling Inconsistencies**
   - Pool creation ignores errors: `vm, _ := factory.Create()`
   - No error propagation from factory failures

4. **Missing VM Health Checks**
   - No mechanism to detect corrupted/unhealthy VMs
   - No VM age limit or refresh mechanism

5. **Context Bridge Limitations**
   - No way to make context read-only
   - Scripts could potentially modify context object

### Improvements Needed

```go
// Fixed VM Factory with proper lifecycle
type DefaultVMFactory struct {
    memoryTracker *MemoryTracker
    baseTimeout   time.Duration
}

func (f *DefaultVMFactory) Create() (*VM, error) {
    runtime := goja.New()
    
    vm := &VM{
        Runtime:    runtime,
        ID:         uuid.New().String(),
        Created:    time.Now(),
        cancelFunc: nil, // Store cancellation function
    }
    
    // Setup base environment
    if err := f.setupBaseEnvironment(runtime); err != nil {
        return nil, fmt.Errorf("failed to setup VM environment: %w", err)
    }
    
    return vm, nil
}

// Better pool with health checks
type VMPool struct {
    pool         chan *VM
    factory      VMFactory
    maxAge       time.Duration
    healthCheck  func(*VM) bool
    metrics      *PoolMetrics
}

// Proper timeout handling in execution
func (r *Runtime) ExecuteWithContext(ctx context.Context, script string, setup func(*goja.Runtime) error) (interface{}, error) {
    vm := r.pool.Get()
    defer r.pool.Put(vm)
    
    // Create interrupter that respects context
    interrupt := make(chan struct{})
    go func() {
        select {
        case <-ctx.Done():
            vm.Runtime.Interrupt(ctx.Err().Error())
            close(interrupt)
        case <-interrupt:
            // Execution completed
        }
    }()
    defer close(interrupt)
    
    // Execute with setup
    if setup != nil {
        if err := setup(vm.Runtime); err != nil {
            return nil, err
        }
    }
    
    return vm.Runtime.RunString(script)
}
```

## Phase 2: Domain Entity & Repository

### Issues Found

1. **Immutability Pattern Incomplete**
   - Methods like `UpdateTags` not implemented
   - Deep copy of metadata map could be expensive

2. **Missing Factory Method**
   - `FromRepository` method referenced in mapper but not defined
   - Need constructor for reconstituting from DB

3. **SQL Injection Risk**
   - `WrapQuery` in tenant isolation concatenates strings
   - Should use parameterized queries

4. **Version History Design Flaw**
   - No mechanism to limit version history growth
   - Could lead to unbounded storage

5. **Missing Cascade Deletes**
   - Soft delete doesn't handle related entities
   - Schedule/route/subscription cleanup needed

### Improvements Needed

```go
// Add missing factory method
func FromRepository(
    id, tenantID uuid.UUID,
    name, description string,
    scriptType ScriptType,
    content string,
    version int,
    tags []string,
    metadata map[string]interface{},
    enabled bool,
    createdAt, updatedAt time.Time,
    createdBy, updatedBy uuid.UUID,
) Script {
    return &scriptImpl{
        id:          id,
        tenantID:    tenantID,
        name:        name,
        description: description,
        scriptType:  scriptType,
        content:     content,
        version:     version,
        tags:        tags,
        metadata:    metadata,
        enabled:     enabled,
        createdAt:   createdAt,
        updatedAt:   updatedAt,
        createdBy:   createdBy,
        updatedBy:   updatedBy,
    }
}

// Better tenant isolation
type QueryBuilder struct {
    tenantID uuid.UUID
}

func (qb *QueryBuilder) Select(table string, conditions ...string) (string, []interface{}) {
    query := fmt.Sprintf("SELECT * FROM %s WHERE tenant_id = $1", table)
    args := []interface{}{qb.tenantID}
    
    for i, cond := range conditions {
        query += fmt.Sprintf(" AND %s", cond)
        // Conditions should include placeholders
    }
    
    return query, args
}

// Version retention policy
type VersionRetentionPolicy struct {
    MaxVersions      int
    RetentionDays    int
    KeepMilestones   bool
}

func (r *scriptRepository) pruneVersionHistory(ctx context.Context, scriptID uuid.UUID, policy VersionRetentionPolicy) error {
    // Implementation to clean old versions
}
```

## Phase 3: Service Layer & Basic APIs

### Issues Found

1. **Execution Service Design**
   - `ExecuteWithContext` accepts a setup function but it's not in the interface
   - Inconsistent API between runtime and service

2. **Missing Script Parameter Validation**
   - No validation that required params are provided
   - No type checking for parameters

3. **Database API Security**
   - Simple string manipulation for tenant isolation is fragile
   - Need proper SQL parser or query builder

4. **Missing Transaction Rollback**
   - Database transaction API doesn't handle panics
   - Could leave transactions open

5. **HTTP API Missing Features**
   - No request body size limit
   - No connection pooling configuration
   - Missing retry logic

6. **Cache Key Collision Risk**
   - Simple prefix might collide
   - Need namespace separation

### Improvements Needed

```go
// Better execution interface
type ExecutionService interface {
    Execute(ctx context.Context, script *script.Script, params map[string]interface{}) (*ExecutionResult, error)
    ExecuteWithTimeout(ctx context.Context, script *script.Script, params map[string]interface{}, timeout time.Duration) (*ExecutionResult, error)
    ValidateScript(ctx context.Context, script *script.Script, params map[string]interface{}) error
    ValidateSyntax(content string) error
}

// Parameter validation
type ScriptParameter struct {
    Name        string
    Type        string // "string", "number", "boolean", "object"
    Required    bool
    Default     interface{}
    Validator   func(interface{}) error
}

func (s *executionService) validateParameters(script *script.Script, params map[string]interface{}) error {
    // Get parameter definitions from script metadata
    paramDefs, ok := script.Metadata()["parameters"].([]ScriptParameter)
    if !ok {
        return nil // No validation needed
    }
    
    for _, def := range paramDefs {
        value, exists := params[def.Name]
        
        if !exists && def.Required {
            return fmt.Errorf("missing required parameter: %s", def.Name)
        }
        
        if !exists && def.Default != nil {
            params[def.Name] = def.Default
            continue
        }
        
        if def.Validator != nil {
            if err := def.Validator(value); err != nil {
                return fmt.Errorf("invalid parameter %s: %w", def.Name, err)
            }
        }
    }
    
    return nil
}

// Safer database API
type DatabaseAPI struct {
    db          *sqlx.DB
    queryParser QueryParser
    tenantID    uuid.UUID
}

func (api *DatabaseAPI) executeQuery(ctx context.Context, query string, args []interface{}) ([]map[string]interface{}, error) {
    // Parse and validate query
    parsed, err := api.queryParser.Parse(query)
    if err != nil {
        return nil, fmt.Errorf("invalid query: %w", err)
    }
    
    // Ensure tenant isolation
    parsed.AddCondition("tenant_id", api.tenantID)
    
    // Build safe query
    safeQuery, safeArgs := parsed.Build(args)
    
    return api.db.QueryContext(ctx, safeQuery, safeArgs...)
}

// Transaction with panic recovery
func (api *DatabaseAPI) transaction(ctx context.Context, callback goja.Callable) (result interface{}, err error) {
    tx, err := api.db.BeginTxx(ctx, nil)
    if err != nil {
        return nil, err
    }
    
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            err = fmt.Errorf("transaction panic: %v", p)
        } else if err != nil {
            tx.Rollback()
        } else {
            err = tx.Commit()
        }
    }()
    
    // Execute callback
    result, err = callback(goja.Undefined(), vm.ToValue(txAPI))
    return
}
```

## General Architecture Issues

### 1. **Missing Metrics and Monitoring**
- No execution metrics (success rate, duration, etc.)
- No pool utilization metrics
- No cache hit/miss rates

### 2. **Missing Rate Limiting**
- Script execution rate limiting
- API call rate limiting per script
- Resource usage quotas

### 3. **Missing Audit Trail**
- Script execution audit log
- API call audit log
- Security event logging

### 4. **Testing Gaps**
- No stress tests for VM pool
- No concurrent execution tests
- No tenant isolation security tests

### 5. **Documentation Gaps**
- Missing API error codes
- No performance tuning guide
- No security best practices

## Recommended Refactoring

### 1. **Introduce Middleware Pattern**
```go
type ExecutionMiddleware interface {
    Before(ctx context.Context, script *script.Script, params map[string]interface{}) error
    After(ctx context.Context, script *script.Script, result *ExecutionResult) error
}

// Rate limiting, auditing, metrics as middleware
```

### 2. **Resource Manager Abstraction**
```go
type ResourceManager interface {
    AllocateVM(ctx context.Context) (*VM, error)
    ReleaseVM(vm *VM)
    GetQuota(ctx context.Context, tenantID uuid.UUID) *ResourceQuota
    TrackUsage(ctx context.Context, usage ResourceUsage) error
}
```

### 3. **Script Lifecycle Hooks**
```go
type ScriptLifecycle interface {
    OnCreate(ctx context.Context, script *script.Script) error
    OnUpdate(ctx context.Context, old, new *script.Script) error
    OnDelete(ctx context.Context, script *script.Script) error
    OnExecute(ctx context.Context, script *script.Script, params map[string]interface{}) error
}
```

### 4. **Unified Error Handling**
```go
type ScriptError struct {
    Code       string
    Message    string
    Details    map[string]interface{}
    Source     ErrorSource
    Retryable  bool
}

type ErrorSource string

const (
    ErrorSourceSyntax     ErrorSource = "syntax"
    ErrorSourceRuntime    ErrorSource = "runtime"
    ErrorSourcePermission ErrorSource = "permission"
    ErrorSourceResource   ErrorSource = "resource"
    ErrorSourceTimeout    ErrorSource = "timeout"
)
```

## Priority Fixes

1. **High Priority**
   - Fix VM timeout implementation
   - Fix SQL injection risks
   - Add transaction panic recovery
   - Add resource limits

2. **Medium Priority**
   - Add metrics collection
   - Implement rate limiting
   - Add health checks
   - Improve error handling

3. **Low Priority**
   - Add middleware pattern
   - Optimize metadata copying
   - Add performance tuning
   - Enhance documentation

## Next Steps

1. Refactor Phase 1 to fix timeout and memory issues
2. Add security layer for SQL queries in Phase 3
3. Implement resource quotas and limits
4. Add comprehensive metrics and monitoring
5. Create integration test suite for security scenarios