# Phase 1: Core Runtime Foundation (2 days) - Version 2

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

#### 2. Define Core Interfaces
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

type VM struct {
    Runtime    *goja.Runtime
    ID         string
    Created    time.Time
    LastUsed   time.Time
    UseCount   int64
    Healthy    bool
}

type VMPool interface {
    Get(ctx context.Context) (*VM, error)
    Put(vm *VM)
    HealthCheck(vm *VM) bool
    Stats() PoolStats
    Close() error
}

type PoolStats struct {
    Available   int
    InUse       int
    Created     int64
    Destroyed   int64
    Errors      int64
}

type RuntimeMetrics struct {
    ExecutionsTotal   int64
    ExecutionErrors   int64
    TimeoutsTotal     int64
    CompilationCache  CacheStats
    PoolStats         PoolStats
}
```

#### 3. Implement Enhanced VM Pool
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
            p.performHealthChecks()
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

#### 4. Create Enhanced VM Factory
Create `pkg/jsruntime/pool/vm_factory.go`:
```go
package pool

import (
    "context"
    "fmt"
    "github.com/dop251/goja"
    "github.com/google/uuid"
)

type VMFactory interface {
    Create(ctx context.Context) (*VM, error)
    Reset(vm *VM) error
    Destroy(vm *VM)
}

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

#### 5. Implement Resource Tracking
Create `pkg/jsruntime/security/resource_tracker.go`:
```go
package security

import (
    "context"
    "sync/atomic"
    "time"
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
)

type runtime struct {
    pool      VMPool
    compiler  *Compiler
    config    *Config
    metrics   *RuntimeMetrics
    mu        sync.RWMutex
}

type Config struct {
    PoolSize        int
    MaxMemoryMB     int
    MaxCPUTime      time.Duration
    DefaultTimeout  time.Duration
    EnableCache     bool
    CacheSize       int
    EnableMetrics   bool
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
    factory := NewDefaultVMFactory(
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
    compiler, err := NewCompiler(config.CacheSize)
    if err != nil {
        return nil, fmt.Errorf("failed to create compiler: %w", err)
    }
    
    return &runtime{
        pool:     vmPool,
        compiler: compiler,
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
        MaxAPICalls:    1000, // Configurable
    }
}
```

### Testing Requirements

Create comprehensive tests with the new architecture:

```go
func TestVMPoolWithHealthChecks(t *testing.T) {
    factory := NewMockVMFactory()
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

[Previous content remains largely the same but with these key improvements:]

### Key Improvements

1. **Immutable Context Object**
```go
func (b *ContextBridge) Install() error {
    // Create frozen context object
    ctxObj := b.vm.NewObject()
    
    b.addTenantInfo(ctxObj)
    b.addUserInfo(ctxObj)
    b.addRequestInfo(ctxObj)
    
    // Freeze the object to prevent modifications
    b.vm.RunString(`Object.freeze(context)`)
    
    b.vm.Set("context", ctxObj)
    
    return nil
}
```

2. **Enhanced Security Validation**
```go
type SecurityValidator struct {
    allowedHosts []string
    blockedHosts []string
}

func (sv *SecurityValidator) ValidateAPICall(ctx context.Context, api, method string) error {
    // Check if API call is allowed
    tracker := ctx.Value("resourceTracker").(*ResourceTracker)
    tracker.TrackAPICall()
    
    // Check permissions
    if !sv.hasPermission(ctx, api, method) {
        return ErrPermissionDenied
    }
    
    return nil
}
```

3. **Context Propagation with Deadline**
```go
func (b *ContextBridge) addDeadline(obj *goja.Object) {
    if deadline, ok := b.ctx.Deadline(); ok {
        remaining := time.Until(deadline).Milliseconds()
        obj.Set("deadlineMs", remaining)
    }
}
```

## Success Criteria
1. VM pool maintains consistent performance under load
2. Health checks prevent unhealthy VM usage
3. Resource limits are enforced effectively
4. Context isolation prevents cross-tenant data access
5. All tests pass with > 90% coverage
6. No memory leaks under stress testing
7. Graceful degradation under resource pressure

## Notes for Next Phase
- Enhanced security measures will be used in all APIs
- Resource tracking enables quota management
- Health metrics enable proactive monitoring
- Pool statistics guide capacity planning