# Phase 1: Core Runtime Foundation (2 days)

## Overview
Establish the foundational JavaScript runtime using Goja, implementing VM pooling, compilation caching, and context integration with IOTA SDK's multi-tenant architecture.

## Background
- Goja is a pure Go JavaScript engine (no CGO required)
- IOTA SDK uses context-based tenant isolation
- All operations must respect the multi-tenant boundaries
- Performance is critical - VM pooling and compilation caching are essential
- Resource limits and security are paramount

## Task 1.1: Basic Goja Integration (Day 1)

### Objectives
- Create the core JavaScript runtime package structure
- Implement VM pooling with health checks
- Add compilation caching to avoid recompiling scripts
- Implement resource tracking and limits
- Ensure proper error handling and panic recovery

### Detailed Steps

#### 1. Create Package Structure
```bash
mkdir -p pkg/jsruntime/{pool,compiler,security,metrics,errors}
```

Create `pkg/jsruntime/interfaces.go`:
```go
package jsruntime

import (
    "context"
    "time"
    "github.com/dop251/goja"
)

type Runtime interface {
    Execute(ctx context.Context, script string) (interface{}, error)
    ExecuteWithSetup(ctx context.Context, script string, setup SetupFunc) (interface{}, error)
    Compile(name, source string) (*goja.Program, error)
    Metrics() RuntimeMetrics
}

type SetupFunc func(ctx context.Context, vm *goja.Runtime) error

type Config struct {
    PoolSize        int
    MaxMemoryMB     int
    MaxCPUTime      time.Duration
    DefaultTimeout  time.Duration
    EnableCache     bool
    CacheSize       int
    EnableMetrics   bool
}

type RuntimeMetrics struct {
    ExecutionsTotal   int64
    ExecutionErrors   int64
    TimeoutsTotal     int64
    CompilationCache  CacheStats
    PoolStats         PoolStats
}

type PoolStats struct {
    Available   int
    InUse       int
    Created     int64
    Destroyed   int64
    Errors      int64
}

type CacheStats struct {
    Hits   int64
    Misses int64
    Size   int
}
```

#### 2. Implement VM Pool
Create `pkg/jsruntime/pool/vm_pool.go`:
```go
package pool

import (
    "context"
    "fmt"
    "sync"
    "sync/atomic"
    "time"
    "github.com/dop251/goja"
    "github.com/google/uuid"
)

type Config struct {
    PoolSize       int
    MaxVMAge       time.Duration
    MaxUseCount    int64
    CreateTimeout  time.Duration
    HealthInterval time.Duration
}

type VM struct {
    Runtime    *goja.Runtime
    ID         string
    Created    time.Time
    LastUsed   time.Time
    UseCount   int64
    Healthy    bool
}

type VMFactory interface {
    Create(ctx context.Context) (*VM, error)
    Reset(vm *VM) error
    Destroy(vm *VM)
}

type VMPool interface {
    Get(ctx context.Context) (*VM, error)
    Put(vm *VM)
    HealthCheck(vm *VM) bool
    Stats() PoolStats
    Close() error
}

type vmPool struct {
    pool      chan *VM
    factory   VMFactory
    config    Config
    stats     *PoolStats
    closed    atomic.Bool
    wg        sync.WaitGroup
    mu        sync.Mutex
}

func NewVMPool(factory VMFactory, config Config) (VMPool, error) {
    if config.PoolSize <= 0 {
        config.PoolSize = 10
    }
    if config.MaxVMAge == 0 {
        config.MaxVMAge = 1 * time.Hour
    }
    if config.MaxUseCount == 0 {
        config.MaxUseCount = 1000
    }
    
    p := &vmPool{
        pool:    make(chan *VM, config.PoolSize),
        factory: factory,
        config:  config,
        stats:   &PoolStats{},
    }
    
    // Pre-populate pool
    for i := 0; i < config.PoolSize; i++ {
        vm, err := p.createVM()
        if err != nil {
            return nil, fmt.Errorf("failed to create initial VM: %w", err)
        }
        p.pool <- vm
    }
    
    // Start health check routine
    p.wg.Add(1)
    go p.healthCheckRoutine()
    
    return p, nil
}

func (p *vmPool) Get(ctx context.Context) (*VM, error) {
    if p.closed.Load() {
        return nil, fmt.Errorf("pool is closed")
    }
    
    select {
    case vm := <-p.pool:
        // Check if VM is still healthy and not too old
        if p.shouldRetire(vm) {
            p.destroyVM(vm)
            return p.createVM()
        }
        
        vm.LastUsed = time.Now()
        vm.UseCount++
        atomic.AddInt64(&p.stats.InUse, 1)
        atomic.AddInt64(&p.stats.Available, -1)
        return vm, nil
        
    case <-ctx.Done():
        return nil, ctx.Err()
        
    case <-time.After(100 * time.Millisecond):
        // Pool exhausted, create new VM
        vm, err := p.createVM()
        if err != nil {
            atomic.AddInt64(&p.stats.Errors, 1)
            return nil, err
        }
        atomic.AddInt64(&p.stats.InUse, 1)
        return vm, nil
    }
}

func (p *vmPool) Put(vm *VM) {
    if p.closed.Load() || vm == nil {
        return
    }
    
    atomic.AddInt64(&p.stats.InUse, -1)
    
    // Reset and check health
    if err := p.factory.Reset(vm); err != nil || !p.HealthCheck(vm) {
        p.destroyVM(vm)
        // Try to maintain pool size
        if newVM, err := p.createVM(); err == nil {
            select {
            case p.pool <- newVM:
                atomic.AddInt64(&p.stats.Available, 1)
            default:
                p.destroyVM(newVM)
            }
        }
        return
    }
    
    select {
    case p.pool <- vm:
        atomic.AddInt64(&p.stats.Available, 1)
    default:
        // Pool full, destroy VM
        p.destroyVM(vm)
    }
}

func (p *vmPool) shouldRetire(vm *VM) bool {
    return time.Since(vm.Created) > p.config.MaxVMAge ||
           vm.UseCount > p.config.MaxUseCount ||
           !vm.Healthy
}

func (p *vmPool) createVM() (*VM, error) {
    ctx, cancel := context.WithTimeout(context.Background(), p.config.CreateTimeout)
    defer cancel()
    
    vm, err := p.factory.Create(ctx)
    if err != nil {
        return nil, err
    }
    
    atomic.AddInt64(&p.stats.Created, 1)
    return vm, nil
}

func (p *vmPool) destroyVM(vm *VM) {
    if vm != nil {
        p.factory.Destroy(vm)
        atomic.AddInt64(&p.stats.Destroyed, 1)
    }
}

func (p *vmPool) HealthCheck(vm *VM) bool {
    if vm == nil || vm.Runtime == nil {
        return false
    }
    
    // Try to execute simple script
    ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
    defer cancel()
    
    done := make(chan bool, 1)
    go func() {
        _, err := vm.Runtime.RunString("1 + 1")
        done <- err == nil
    }()
    
    select {
    case healthy := <-done:
        vm.Healthy = healthy
        return healthy
    case <-ctx.Done():
        vm.Healthy = false
        return false
    }
}

func (p *vmPool) healthCheckRoutine() {
    defer p.wg.Done()
    
    ticker := time.NewTicker(p.config.HealthInterval)
    defer ticker.Stop()
    
    for !p.closed.Load() {
        select {
        case <-ticker.C:
            // Perform health checks on idle VMs
        }
    }
}

func (p *vmPool) Stats() PoolStats {
    return PoolStats{
        Available: int(atomic.LoadInt64(&p.stats.Available)),
        InUse:     int(atomic.LoadInt64(&p.stats.InUse)),
        Created:   atomic.LoadInt64(&p.stats.Created),
        Destroyed: atomic.LoadInt64(&p.stats.Destroyed),
        Errors:    atomic.LoadInt64(&p.stats.Errors),
    }
}

func (p *vmPool) Close() error {
    if !p.closed.CompareAndSwap(false, true) {
        return nil
    }
    
    close(p.pool)
    
    // Destroy all VMs
    for vm := range p.pool {
        p.destroyVM(vm)
    }
    
    p.wg.Wait()
    return nil
}
```

#### 3. Create VM Factory with Limits
Create `pkg/jsruntime/pool/vm_factory.go`:
```go
package pool

import (
    "context"
    "fmt"
    "time"
    "github.com/dop251/goja"
    "github.com/google/uuid"
)

type DefaultVMFactory struct {
    baseSetup    func(*goja.Runtime) error
    memoryLimit  int64
    stackLimit   int
}

func NewDefaultVMFactory(memoryLimit int64, stackLimit int) VMFactory {
    return &DefaultVMFactory{
        memoryLimit: memoryLimit,
        stackLimit:  stackLimit,
    }
}

func (f *DefaultVMFactory) Create(ctx context.Context) (*VM, error) {
    runtime := goja.New()
    
    // Configure runtime
    runtime.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
    runtime.SetMaxCallStackSize(f.stackLimit)
    
    // Set up base environment
    if err := f.setupBaseEnvironment(runtime); err != nil {
        return nil, fmt.Errorf("failed to setup VM environment: %w", err)
    }
    
    vm := &VM{
        Runtime:  runtime,
        ID:       uuid.New().String(),
        Created:  time.Now(),
        LastUsed: time.Now(),
        UseCount: 0,
        Healthy:  true,
    }
    
    return vm, nil
}

func (f *DefaultVMFactory) Reset(vm *VM) error {
    if vm == nil || vm.Runtime == nil {
        return fmt.Errorf("invalid VM")
    }
    
    // Clear user-defined globals but keep built-ins
    _, err := vm.Runtime.RunString(`
        (function() {
            const builtins = [
                'Object', 'Array', 'String', 'Number', 'Boolean', 
                'Date', 'Math', 'JSON', 'RegExp', 'Error', 
                'TypeError', 'ReferenceError', 'SyntaxError',
                'Promise', 'Map', 'Set', 'WeakMap', 'WeakSet',
                'Symbol', 'Proxy', 'Reflect',
                'console', 'undefined', 'null', 'globalThis'
            ];
            
            for (const key of Object.keys(globalThis)) {
                if (!builtins.includes(key)) {
                    delete globalThis[key];
                }
            }
        })();
    `)
    
    return err
}

func (f *DefaultVMFactory) Destroy(vm *VM) {
    // Interrupt any running code
    if vm != nil && vm.Runtime != nil {
        vm.Runtime.Interrupt("VM destroyed")
    }
}

func (f *DefaultVMFactory) setupBaseEnvironment(runtime *goja.Runtime) error {
    // Add console with limited functionality
    console := runtime.NewObject()
    console.Set("log", func(args ...interface{}) {
        // Logging handled by execution service
    })
    console.Set("error", func(args ...interface{}) {
        // Logging handled by execution service
    })
    console.Set("warn", func(args ...interface{}) {
        // Logging handled by execution service
    })
    console.Set("info", func(args ...interface{}) {
        // Logging handled by execution service
    })
    
    runtime.Set("console", console)
    
    // Disable dangerous globals
    runtime.Set("eval", goja.Undefined())
    runtime.Set("Function", goja.Undefined())
    
    return nil
}
```

#### 4. Implement Resource Tracking
Create `pkg/jsruntime/security/resource_tracker.go`:
```go
package security

import (
    "context"
    "errors"
    "sync/atomic"
    "time"
)

var (
    ErrMemoryLimitExceeded  = errors.New("memory limit exceeded")
    ErrCPUTimeLimitExceeded = errors.New("CPU time limit exceeded")
    ErrAPICallLimitExceeded = errors.New("API call limit exceeded")
)

type ResourceTracker struct {
    memoryUsed    atomic.Int64
    cpuTime       atomic.Int64
    apiCallCount  atomic.Int64
    startTime     time.Time
}

type ResourceLimits struct {
    MaxMemoryBytes int64
    MaxCPUTime     time.Duration
    MaxAPICalls    int64
}

func NewResourceTracker() *ResourceTracker {
    return &ResourceTracker{
        startTime: time.Now(),
    }
}

func (rt *ResourceTracker) CheckLimits(limits ResourceLimits) error {
    if rt.memoryUsed.Load() > limits.MaxMemoryBytes {
        return ErrMemoryLimitExceeded
    }
    
    if time.Since(rt.startTime) > limits.MaxCPUTime {
        return ErrCPUTimeLimitExceeded
    }
    
    if rt.apiCallCount.Load() > limits.MaxAPICalls {
        return ErrAPICallLimitExceeded
    }
    
    return nil
}

func (rt *ResourceTracker) TrackAPICall() {
    rt.apiCallCount.Add(1)
}

func (rt *ResourceTracker) TrackMemory(bytes int64) {
    rt.memoryUsed.Add(bytes)
}

func (rt *ResourceTracker) Reset() {
    rt.memoryUsed.Store(0)
    rt.cpuTime.Store(0)
    rt.apiCallCount.Store(0)
    rt.startTime = time.Now()
}
```

#### 5. Implement Compilation Cache
Create `pkg/jsruntime/compiler/compiler.go`:
```go
package compiler

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "sync"
    "sync/atomic"
    "github.com/dop251/goja"
    "github.com/hashicorp/golang-lru/v2"
)

type Compiler struct {
    cache *lru.Cache[string, *goja.Program]
    stats *CacheStats
    mu    sync.RWMutex
}

type CacheStats struct {
    hits   atomic.Int64
    misses atomic.Int64
}

func NewCompiler(cacheSize int) (*Compiler, error) {
    cache, err := lru.New[string, *goja.Program](cacheSize)
    if err != nil {
        return nil, err
    }
    
    return &Compiler{
        cache: cache,
        stats: &CacheStats{},
    }, nil
}

func (c *Compiler) Compile(name, source string) (*goja.Program, error) {
    // Generate cache key
    hash := sha256.Sum256([]byte(source))
    key := hex.EncodeToString(hash[:])
    
    // Check cache
    c.mu.RLock()
    if program, ok := c.cache.Get(key); ok {
        c.mu.RUnlock()
        c.stats.hits.Add(1)
        return program, nil
    }
    c.mu.RUnlock()
    
    // Cache miss - compile
    c.stats.misses.Add(1)
    
    program, err := goja.Compile(name, source, true)
    if err != nil {
        return nil, fmt.Errorf("compilation failed: %w", err)
    }
    
    // Store in cache
    c.mu.Lock()
    c.cache.Add(key, program)
    c.mu.Unlock()
    
    return program, nil
}

func (c *Compiler) Stats() CacheStats {
    return CacheStats{
        Hits:   c.stats.hits.Load(),
        Misses: c.stats.misses.Load(),
        Size:   c.cache.Len(),
    }
}
```

#### 6. Implement Enhanced Runtime
Create `pkg/jsruntime/runtime.go`:
```go
package jsruntime

import (
    "context"
    "fmt"
    "sync"
    "time"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/jsruntime/pool"
    "github.com/iota-uz/iota-sdk/pkg/jsruntime/compiler"
    "github.com/iota-uz/iota-sdk/pkg/jsruntime/security"
)

type runtime struct {
    pool      pool.VMPool
    compiler  *compiler.Compiler
    config    *Config
    metrics   *RuntimeMetrics
    mu        sync.RWMutex
}

func NewRuntime(config *Config) (Runtime, error) {
    // Validate config
    if config.DefaultTimeout == 0 {
        config.DefaultTimeout = 30 * time.Second
    }
    if config.MaxCPUTime == 0 {
        config.MaxCPUTime = 60 * time.Second
    }
    
    // Create VM factory
    factory := pool.NewDefaultVMFactory(
        int64(config.MaxMemoryMB)*1024*1024,
        1000, // stack limit
    )
    
    // Create pool
    poolConfig := pool.Config{
        PoolSize:       config.PoolSize,
        MaxVMAge:       1 * time.Hour,
        MaxUseCount:    1000,
        CreateTimeout:  5 * time.Second,
        HealthInterval: 30 * time.Second,
    }
    
    vmPool, err := pool.NewVMPool(factory, poolConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create VM pool: %w", err)
    }
    
    // Create compiler
    comp, err := compiler.NewCompiler(config.CacheSize)
    if err != nil {
        return nil, fmt.Errorf("failed to create compiler: %w", err)
    }
    
    return &runtime{
        pool:     vmPool,
        compiler: comp,
        config:   config,
        metrics:  &RuntimeMetrics{},
    }, nil
}

func (r *runtime) Execute(ctx context.Context, script string) (interface{}, error) {
    return r.ExecuteWithSetup(ctx, script, nil)
}

func (r *runtime) ExecuteWithSetup(ctx context.Context, script string, setup SetupFunc) (result interface{}, err error) {
    // Track metrics
    start := time.Now()
    defer func() {
        r.recordMetrics(time.Since(start), err)
    }()
    
    // Apply timeout
    ctx, cancel := context.WithTimeout(ctx, r.config.DefaultTimeout)
    defer cancel()
    
    // Get VM from pool
    vm, err := r.pool.Get(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get VM: %w", err)
    }
    defer r.pool.Put(vm)
    
    // Create resource tracker
    tracker := security.NewResourceTracker()
    ctx = context.WithValue(ctx, "resourceTracker", tracker)
    
    // Setup interrupter
    interrupt := make(chan struct{})
    go func() {
        ticker := time.NewTicker(10 * time.Millisecond)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                // Check resource limits
                if err := tracker.CheckLimits(r.getResourceLimits()); err != nil {
                    vm.Runtime.Interrupt(err.Error())
                    return
                }
            case <-ctx.Done():
                vm.Runtime.Interrupt(ctx.Err().Error())
                return
            case <-interrupt:
                return
            }
        }
    }()
    
    // Recover from panics
    defer func() {
        close(interrupt)
        if p := recover(); p != nil {
            err = fmt.Errorf("script panic: %v", p)
        }
    }()
    
    // Apply setup if provided
    if setup != nil {
        if err := setup(ctx, vm.Runtime); err != nil {
            return nil, fmt.Errorf("setup failed: %w", err)
        }
    }
    
    // Compile script
    program, err := r.compiler.Compile("script", script)
    if err != nil {
        return nil, fmt.Errorf("compilation failed: %w", err)
    }
    
    // Execute
    return vm.Runtime.RunProgram(program)
}

func (r *runtime) Compile(name, source string) (*goja.Program, error) {
    return r.compiler.Compile(name, source)
}

func (r *runtime) Metrics() RuntimeMetrics {
    r.mu.RLock()
    defer r.mu.RUnlock()
    
    metrics := *r.metrics
    metrics.PoolStats = r.pool.Stats()
    metrics.CompilationCache = r.compiler.Stats()
    
    return metrics
}

func (r *runtime) recordMetrics(duration time.Duration, err error) {
    if !r.config.EnableMetrics {
        return
    }
    
    r.mu.Lock()
    defer r.mu.Unlock()
    
    r.metrics.ExecutionsTotal++
    if err != nil {
        r.metrics.ExecutionErrors++
        if err == context.DeadlineExceeded {
            r.metrics.TimeoutsTotal++
        }
    }
}

func (r *runtime) getResourceLimits() security.ResourceLimits {
    return security.ResourceLimits{
        MaxMemoryBytes: int64(r.config.MaxMemoryMB) * 1024 * 1024,
        MaxCPUTime:     r.config.MaxCPUTime,
        MaxAPICalls:    1000, // TODO: Make configurable
    }
}
```

#### 7. Add Error Handling
Create `pkg/jsruntime/errors/errors.go`:
```go
package errors

import (
    "errors"
    "github.com/dop251/goja"
)

var (
    ErrTimeout          = errors.New("script execution timeout")
    ErrMemoryExceeded   = errors.New("memory limit exceeded")
    ErrCompilationFailed = errors.New("script compilation failed")
)

type ScriptError struct {
    Type    string
    Message string
    Stack   string
    Line    int
    Column  int
}

func WrapGojaError(err error) *ScriptError {
    if exception, ok := err.(*goja.Exception); ok {
        return &ScriptError{
            Type:    "RuntimeError",
            Message: exception.String(),
            Stack:   exception.String(),
        }
    }
    
    return &ScriptError{
        Type:    "UnknownError",
        Message: err.Error(),
    }
}
```

### Testing Requirements

Create `pkg/jsruntime/runtime_test.go`:
```go
package jsruntime_test

import (
    "context"
    "fmt"
    "sync"
    "testing"
    "time"
    "github.com/stretchr/testify/require"
)

func TestVMPoolWithHealthChecks(t *testing.T) {
    factory := pool.NewDefaultVMFactory(100*1024*1024, 1000)
    config := pool.Config{
        PoolSize:       2,
        MaxVMAge:       100 * time.Millisecond,
        MaxUseCount:    2,
        HealthInterval: 50 * time.Millisecond,
    }
    
    pool, err := pool.NewVMPool(factory, config)
    require.NoError(t, err)
    defer pool.Close()
    
    // Test VM retirement
    vm1, _ := pool.Get(context.Background())
    originalID := vm1.ID
    
    // Use VM multiple times
    pool.Put(vm1)
    vm2, _ := pool.Get(context.Background())
    pool.Put(vm2)
    vm3, _ := pool.Get(context.Background())
    
    // Should get new VM due to use count
    require.NotEqual(t, originalID, vm3.ID)
}

func TestResourceLimits(t *testing.T) {
    runtime, _ := NewRuntime(&Config{
        PoolSize:     1,
        MaxMemoryMB:  10,
        MaxCPUTime:   100 * time.Millisecond,
    })
    
    // Test CPU time limit
    ctx := context.Background()
    _, err := runtime.Execute(ctx, `
        while(true) {
            // Busy loop
        }
    `)
    
    require.Error(t, err)
    require.Contains(t, err.Error(), "CPU time limit exceeded")
}

func TestConcurrentExecution(t *testing.T) {
    runtime, _ := NewRuntime(&Config{
        PoolSize: 5,
    })
    
    var wg sync.WaitGroup
    errors := make(chan error, 10)
    
    // Execute 10 scripts concurrently
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            
            result, err := runtime.Execute(context.Background(), fmt.Sprintf(`
                const n = %d;
                n * n
            `, n))
            
            if err != nil {
                errors <- err
            } else {
                require.Equal(t, int64(n*n), result.ToInteger())
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check no errors
    for err := range errors {
        require.NoError(t, err)
    }
}

func TestCompilationCache(t *testing.T) {
    runtime, _ := NewRuntime(&Config{
        PoolSize:      1,
        EnableCache:   true,
        CacheSize:     10,
        EnableMetrics: true,
    })
    
    // First execution - cache miss
    _, err := runtime.Execute(context.Background(), "1 + 1")
    require.NoError(t, err)
    
    metrics1 := runtime.Metrics()
    require.Equal(t, int64(1), metrics1.CompilationCache.Misses)
    
    // Second execution - cache hit
    _, err = runtime.Execute(context.Background(), "1 + 1")
    require.NoError(t, err)
    
    metrics2 := runtime.Metrics()
    require.Equal(t, int64(1), metrics2.CompilationCache.Hits)
}

func TestPanicRecovery(t *testing.T) {
    runtime, _ := NewRuntime(&Config{PoolSize: 1})
    
    ctx := context.Background()
    _, err := runtime.Execute(ctx, "throw new Error('test panic')")
    
    require.Error(t, err)
    require.Contains(t, err.Error(), "test panic")
    
    // Runtime should still be usable
    result, err := runtime.Execute(ctx, "2 + 2")
    require.NoError(t, err)
    require.Equal(t, int64(4), result.Export())
}
```

### Deliverables Checklist
- [ ] Enhanced VM pool with health checks and retirement
- [ ] Resource tracking and enforcement
- [ ] Proper timeout and cancellation handling
- [ ] Metrics collection and reporting
- [ ] Concurrent execution support
- [ ] Comprehensive error handling
- [ ] Security hardening (no eval, Function)
- [ ] Performance benchmarks
- [ ] Stress tests for pool management

## Task 1.2: Context Integration (Day 2)

### Objectives
- Propagate Go context to JavaScript environment
- Implement tenant isolation at runtime level
- Add user context access from scripts
- Handle context cancellation properly

### Detailed Steps

#### 1. Create Context Bridge
Create `pkg/jsruntime/context/bridge.go`:
```go
package context

import (
    "context"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type ContextBridge struct {
    ctx context.Context
    vm  *goja.Runtime
}

func NewContextBridge(ctx context.Context, vm *goja.Runtime) *ContextBridge {
    return &ContextBridge{ctx: ctx, vm: vm}
}

func (b *ContextBridge) Install() error {
    // Create context object
    ctxObj := b.vm.NewObject()
    
    // Add tenant info
    b.addTenantInfo(ctxObj)
    
    // Add user info
    b.addUserInfo(ctxObj)
    
    // Add request info
    b.addRequestInfo(ctxObj)
    
    // Add deadline info
    b.addDeadline(ctxObj)
    
    // Make context available globally
    b.vm.Set("context", ctxObj)
    
    return nil
}

func (b *ContextBridge) addTenantInfo(obj *goja.Object) {
    tenantID, err := composables.UseTenantID(b.ctx)
    if err == nil {
        obj.Set("tenantID", tenantID.String())
    }
    
    tenant, err := composables.UseTenant(b.ctx)
    if err == nil {
        obj.Set("tenant", map[string]interface{}{
            "id":   tenant.ID.String(),
            "name": tenant.Name,
        })
    }
}

func (b *ContextBridge) addUserInfo(obj *goja.Object) {
    user, err := composables.UseUser(b.ctx)
    if err == nil {
        userObj := b.vm.NewObject()
        userObj.Set("id", user.ID.String())
        userObj.Set("email", user.Email)
        userObj.Set("firstName", user.FirstName)
        userObj.Set("lastName", user.LastName)
        
        // Add permissions
        perms := b.vm.NewArray()
        for i, perm := range user.Permissions {
            perms.SetIndex(i, perm)
        }
        userObj.Set("permissions", perms)
        
        obj.Set("user", userObj)
    }
}

func (b *ContextBridge) addRequestInfo(obj *goja.Object) {
    // Add request ID
    if reqID := b.ctx.Value("requestID"); reqID != nil {
        obj.Set("requestID", reqID)
    }
    
    // Add locale
    if locale := b.ctx.Value("locale"); locale != nil {
        obj.Set("locale", locale)
    }
}

func (b *ContextBridge) addDeadline(obj *goja.Object) {
    if deadline, ok := b.ctx.Deadline(); ok {
        remaining := time.Until(deadline).Milliseconds()
        obj.Set("deadlineMs", remaining)
    }
}
```

#### 2. Implement Tenant Isolation
Create `pkg/jsruntime/security/tenant_isolation.go`:
```go
package security

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type TenantIsolation struct {
    enforceStrict bool
}

func NewTenantIsolation(strict bool) *TenantIsolation {
    return &TenantIsolation{
        enforceStrict: strict,
    }
}

func (t *TenantIsolation) ValidateAccess(ctx context.Context, resourceTenantID uuid.UUID) error {
    currentTenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return fmt.Errorf("no tenant in context: %w", err)
    }
    
    if currentTenantID != resourceTenantID {
        return fmt.Errorf("tenant isolation violation: current=%s, resource=%s", 
            currentTenantID, resourceTenantID)
    }
    
    return nil
}

// Build tenant-scoped query
func (t *TenantIsolation) BuildQuery(ctx context.Context, baseQuery string, args []interface{}) (string, []interface{}) {
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return baseQuery, args
    }
    
    // Prepend tenant ID to args
    newArgs := append([]interface{}{tenantID}, args...)
    
    // Adjust query (assumes placeholder $1 for tenant_id)
    return baseQuery, newArgs
}
```

#### 3. Create Context Setup Function
Update `pkg/jsruntime/runtime.go` to add context installation helper:
```go
// Context integration is now part of ExecuteWithSetup
func InstallContext(ctx context.Context, vm *goja.Runtime) error {
    // Install context bridge
    bridge := context.NewContextBridge(ctx, vm)
    if err := bridge.Install(); err != nil {
        return err
    }
    
    // Make context immutable
    vm.RunString(`Object.freeze(context)`)
    
    return nil
}
```

### Testing Requirements

Create `pkg/jsruntime/context_test.go`:
```go
func TestContextPropagation(t *testing.T) {
    // Create context with tenant and user
    ctx := context.Background()
    ctx = composables.WithTenant(ctx, &Tenant{
        ID:   uuid.New(),
        Name: "Test Tenant",
    })
    ctx = composables.WithUser(ctx, &User{
        ID:    uuid.New(),
        Email: "test@example.com",
    })
    
    runtime := NewRuntime(&Config{PoolSize: 1})
    
    result, err := runtime.ExecuteWithSetup(ctx, `
        context.tenant.name + " - " + context.user.email
    `, InstallContext)
    
    require.NoError(t, err)
    require.Equal(t, "Test Tenant - test@example.com", result.String())
}

func TestTenantIsolation(t *testing.T) {
    ctx1 := composables.WithTenantID(context.Background(), uuid.New())
    ctx2 := composables.WithTenantID(context.Background(), uuid.New())
    
    runtime := NewRuntime(&Config{PoolSize: 1})
    
    // Set data in tenant 1
    runtime.ExecuteWithSetup(ctx1, `globalThis.tenantData = "tenant1"`, InstallContext)
    
    // Try to access from tenant 2
    result, _ := runtime.ExecuteWithSetup(ctx2, `globalThis.tenantData`, InstallContext)
    
    require.Nil(t, result, "should not see other tenant's data")
}

func TestContextCancellation(t *testing.T) {
    ctx, cancel := context.WithCancel(context.Background())
    runtime := NewRuntime(&Config{PoolSize: 1})
    
    go func() {
        time.Sleep(100 * time.Millisecond)
        cancel()
    }()
    
    _, err := runtime.Execute(ctx, `
        while(true) {
            // Infinite loop
        }
    `)
    
    require.Error(t, err)
    require.Contains(t, err.Error(), "context cancelled")
}

func TestContextImmutability(t *testing.T) {
    ctx := composables.WithTenantID(context.Background(), uuid.New())
    runtime := NewRuntime(&Config{PoolSize: 1})
    
    _, err := runtime.ExecuteWithSetup(ctx, `
        context.tenantID = "modified";
        throw new Error("should not reach here");
    `, InstallContext)
    
    require.Error(t, err)
    require.Contains(t, err.Error(), "Cannot assign to read only property")
}
```

### Integration Tests
Create `pkg/jsruntime/integration_test.go`:
```go
func TestFullContextIntegration(t *testing.T) {
    // Setup mock HTTP request context
    req, _ := http.NewRequest("GET", "/test", nil)
    ctx := req.Context()
    
    // Add IOTA context
    ctx = composables.WithTenant(ctx, &Tenant{ID: uuid.New()})
    ctx = composables.WithUser(ctx, &User{
        ID:          uuid.New(),
        Email:       "user@example.com",
        Permissions: []string{"scripts.execute"},
    })
    ctx = context.WithValue(ctx, "requestID", "req-123")
    ctx = context.WithValue(ctx, "locale", "en-US")
    
    runtime := NewRuntime(&Config{PoolSize: 2})
    
    result, err := runtime.ExecuteWithSetup(ctx, `
        JSON.stringify({
            tenantID: context.tenantID,
            userEmail: context.user.email,
            hasPermission: context.user.permissions.includes("scripts.execute"),
            requestID: context.requestID,
            locale: context.locale
        })
    `, InstallContext)
    
    require.NoError(t, err)
    
    var data map[string]interface{}
    json.Unmarshal([]byte(result.String()), &data)
    
    require.NotEmpty(t, data["tenantID"])
    require.Equal(t, "user@example.com", data["userEmail"])
    require.True(t, data["hasPermission"].(bool))
    require.Equal(t, "req-123", data["requestID"])
    require.Equal(t, "en-US", data["locale"])
}
```

### Deliverables Checklist
- [ ] Context bridge implementation
- [ ] Tenant isolation enforcement
- [ ] User context access from scripts
- [ ] Request context propagation
- [ ] Context cancellation handling
- [ ] Integration with IOTA composables
- [ ] Security tests for isolation
- [ ] Performance benchmarks
- [ ] Documentation for context usage

## Success Criteria
1. VM pool maintains consistent performance under load
2. Health checks prevent unhealthy VM usage
3. Resource limits are enforced effectively
4. Context isolation prevents cross-tenant data access
5. All tests pass with > 90% coverage
6. No memory leaks under stress testing
7. Context cancellation properly interrupts long-running scripts
8. Compilation cache hit rate > 90% for repeated scripts

## Notes for Next Phase
- The VM pool and context system will be used by all subsequent phases
- Resource tracking enables quota management per tenant
- Health metrics enable proactive monitoring
- Pool statistics can guide capacity planning
- Context bridge can be extended with more APIs in future phases