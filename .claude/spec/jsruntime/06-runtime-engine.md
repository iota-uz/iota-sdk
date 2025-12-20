# JavaScript Runtime - Runtime Engine Specification

**Status:** Implementation Ready
**Layer:** Infrastructure Layer
**Dependencies:** Goja library, Domain entities, Service layer
**Related Issues:** #414, #418, #420

---

## Overview

This specification defines the Goja-based JavaScript runtime engine, including VM pooling, script execution, resource limits, compilation caching, and security sandboxing.

## Architecture

### Components

```
┌─────────────────────────────────────────────────┐
│              Runtime Engine                      │
│  ┌───────────────────────────────────────────┐  │
│  │          VM Pool Manager                  │  │
│  │  • Pool size configuration                │  │
│  │  • VM acquisition/release                 │  │
│  │  • Lifecycle management                   │  │
│  └───────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────┐  │
│  │          Script Executor                  │  │
│  │  • Execution orchestration                │  │
│  │  • Timeout handling                       │  │
│  │  • Memory monitoring                      │  │
│  │  • Panic recovery                         │  │
│  └───────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────┐  │
│  │       Compilation Cache                   │  │
│  │  • LRU cache for compiled scripts         │  │
│  │  • Bytecode storage in DB                 │  │
│  │  • Cache invalidation                     │  │
│  └───────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────┐  │
│  │         Sandbox Configuration             │  │
│  │  • Blocked globals                        │  │
│  │  • Allowed globals                        │  │
│  │  • Custom console implementation          │  │
│  └───────────────────────────────────────────┘  │
└─────────────────────────────────────────────────┘
```

---

## Dependencies

**External Libraries:**

```go
import (
    "github.com/dop251/goja"
    "github.com/hashicorp/golang-lru/v2"
)
```

**Add to go.mod:**

```bash
go get github.com/dop251/goja@latest
go get github.com/hashicorp/golang-lru/v2@latest
```

---

## Runtime Interface

**Location:** `modules/scripts/runtime/runtime.go`

```go
package runtime

import (
    "context"
)

// Runtime defines the JavaScript runtime interface
type Runtime interface {
    // ValidateSyntax checks if script has valid JavaScript syntax
    ValidateSyntax(source string) error

    // Execute runs a script with given input and returns output
    Execute(ctx context.Context, source string, input map[string]interface{}) (map[string]interface{}, error)

    // Shutdown gracefully shuts down the runtime
    Shutdown() error
}

// Config holds runtime configuration
type Config struct {
    // VM pool size
    PoolSize int

    // Resource limits
    MaxExecutionTimeMs int
    MaxMemoryMB        int
    MaxAPICalls        int

    // Compilation cache
    CacheSize int

    // API bindings
    Bindings APIBindings
}

// ResourceLimits tracks resource usage during execution
type ResourceLimits struct {
    MaxExecutionTimeMs int
    MaxMemoryMB        int
    MaxAPICalls        int

    // Runtime counters
    apiCallCount int
}
```

---

## VM Pool Implementation

**Location:** `modules/scripts/runtime/pool.go`

```go
package runtime

import (
    "context"
    "errors"
    "sync"
    "time"

    "github.com/dop251/goja"
)

var (
    ErrPoolClosed     = errors.New("VM pool is closed")
    ErrAcquireTimeout = errors.New("timeout acquiring VM from pool")
)

// Pool manages a pool of pre-configured Goja VMs
type Pool struct {
    vms      chan *goja.Runtime
    size     int
    factory  func() *goja.Runtime
    bindings APIBindings
    closed   bool
    mu       sync.RWMutex
}

// NewPool creates a new VM pool
func NewPool(size int, bindings APIBindings) *Pool {
    pool := &Pool{
        vms:      make(chan *goja.Runtime, size),
        size:     size,
        bindings: bindings,
    }

    pool.factory = func() *goja.Runtime {
        return pool.createVM()
    }

    // Pre-populate pool
    for i := 0; i < size; i++ {
        pool.vms <- pool.factory()
    }

    return pool
}

// createVM creates a new Goja VM with sandbox configuration
func (p *Pool) createVM() *goja.Runtime {
    vm := goja.New()

    // Configure sandbox
    p.configureSandbox(vm)

    // Inject API bindings
    p.bindings.Inject(vm)

    return vm
}

// configureSandbox sets up security restrictions
func (p *Pool) configureSandbox(vm *goja.Runtime) {
    // Block dangerous globals
    blockedGlobals := []string{
        "fetch",
        "XMLHttpRequest",
        "WebSocket",
        "process",
        "require",
        "eval",
        "Function",
        "__dirname",
        "__filename",
        "import",
        "importScripts",
    }

    for _, global := range blockedGlobals {
        vm.Set(global, goja.Undefined())
    }

    // Set up custom console
    console := &Console{
        logs: make([]LogEntry, 0),
    }
    vm.Set("console", console.ToGojaObject(vm))

    // Disable Object.freeze, Object.seal (can prevent API injection)
    // Keep Math, Date, JSON available
}

// Acquire gets a VM from the pool
func (p *Pool) Acquire(ctx context.Context) (*goja.Runtime, error) {
    p.mu.RLock()
    if p.closed {
        p.mu.RUnlock()
        return nil, ErrPoolClosed
    }
    p.mu.RUnlock()

    select {
    case vm := <-p.vms:
        return vm, nil
    case <-ctx.Done():
        return nil, ErrAcquireTimeout
    case <-time.After(30 * time.Second):
        return nil, ErrAcquireTimeout
    }
}

// Release returns a VM to the pool
func (p *Pool) Release(vm *goja.Runtime) {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if p.closed {
        return
    }

    // Reset VM state
    vm.ClearInterrupt()

    // Return to pool
    select {
    case p.vms <- vm:
    default:
        // Pool full, discard VM (will be GC'd)
    }
}

// Shutdown closes the pool and releases all VMs
func (p *Pool) Shutdown() {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.closed {
        return
    }

    p.closed = true
    close(p.vms)

    // Drain pool
    for range p.vms {
        // VMs will be garbage collected
    }
}
```

---

## Executor Implementation

**Location:** `modules/scripts/runtime/executor.go`

```go
package runtime

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/dop251/goja"
)

var (
    ErrExecutionTimeout = errors.New("script execution timeout")
    ErrMemoryLimit      = errors.New("script exceeded memory limit")
    ErrAPICallLimit     = errors.New("script exceeded API call limit")
)

// Executor executes scripts with resource limits
type Executor struct {
    pool   *Pool
    limits ResourceLimits
}

// NewExecutor creates a new script executor
func NewExecutor(pool *Pool, limits ResourceLimits) *Executor {
    return &Executor{
        pool:   pool,
        limits: limits,
    }
}

// Execute runs a script with resource limits
func (e *Executor) Execute(
    ctx context.Context,
    source string,
    input map[string]interface{},
) (result map[string]interface{}, err error) {
    // Acquire VM from pool
    vm, err := e.pool.Acquire(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to acquire VM: %w", err)
    }
    defer e.pool.Release(vm)

    // Set up execution context
    if err := e.setupContext(vm, ctx, input); err != nil {
        return nil, err
    }

    // Set up timeout
    execCtx, cancel := context.WithTimeout(ctx,
        time.Duration(e.limits.MaxExecutionTimeMs)*time.Millisecond)
    defer cancel()

    // Monitor execution in goroutine
    done := make(chan bool, 1)
    go func() {
        <-execCtx.Done()
        vm.Interrupt("timeout")
        close(done)
    }()

    // Recover from panics
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("script panic: %v", r)
        }
    }()

    // Compile and run script
    script, err := goja.Compile("", source, true)
    if err != nil {
        return nil, fmt.Errorf("compilation error: %w", err)
    }

    value, err := vm.RunProgram(script)
    if err != nil {
        // Check if it's a timeout
        if execCtx.Err() == context.DeadlineExceeded {
            return nil, ErrExecutionTimeout
        }
        return nil, fmt.Errorf("runtime error: %w", err)
    }

    // Extract result
    result, err = e.extractResult(vm, value)
    if err != nil {
        return nil, err
    }

    return result, nil
}

// setupContext injects context into VM
func (e *Executor) setupContext(
    vm *goja.Runtime,
    ctx context.Context,
    input map[string]interface{},
) error {
    // Build context object
    ctxObj := map[string]interface{}{
        "tenant":    e.buildTenantContext(ctx),
        "user":      e.buildUserContext(ctx),
        "execution": e.buildExecutionContext(ctx),
        "input":     input,
    }

    // Inject into VM
    if err := vm.Set("ctx", ctxObj); err != nil {
        return fmt.Errorf("failed to set context: %w", err)
    }

    return nil
}

func (e *Executor) buildTenantContext(ctx context.Context) map[string]interface{} {
    // Extract tenant from context
    tenantID, _ := composables.UseTenantID(ctx)

    return map[string]interface{}{
        "id": tenantID.String(),
        // Add more tenant info as needed
    }
}

func (e *Executor) buildUserContext(ctx context.Context) interface{} {
    user, err := composables.UseUser(ctx)
    if err != nil {
        return nil
    }

    return map[string]interface{}{
        "id":        user.ID(),
        "email":     user.Email().String(),
        "firstName": user.FirstName(),
        "lastName":  user.LastName(),
    }
}

func (e *Executor) buildExecutionContext(ctx context.Context) map[string]interface{} {
    execID, _ := execution.UseExecutionID(ctx)
    scriptID, _ := execution.UseScriptID(ctx)

    return map[string]interface{}{
        "id":       execID.String(),
        "scriptId": scriptID.String(),
    }
}

// extractResult converts Goja value to Go map
func (e *Executor) extractResult(
    vm *goja.Runtime,
    value goja.Value,
) (map[string]interface{}, error) {
    if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
        return map[string]interface{}{}, nil
    }

    // Export to Go value
    exported := value.Export()

    // Convert to map
    if result, ok := exported.(map[string]interface{}); ok {
        return result, nil
    }

    // Wrap non-map results
    return map[string]interface{}{
        "value": exported,
    }, nil
}
```

---

## Compilation Cache

**Location:** `modules/scripts/runtime/cache.go`

```go
package runtime

import (
    "crypto/sha256"
    "encoding/hex"
    "sync"

    "github.com/dop251/goja"
    lru "github.com/hashicorp/golang-lru/v2"
)

// CompilationCache caches compiled scripts
type CompilationCache struct {
    cache *lru.Cache[string, *goja.Program]
    mu    sync.RWMutex
}

// NewCompilationCache creates a new compilation cache
func NewCompilationCache(size int) (*CompilationCache, error) {
    cache, err := lru.New[string, *goja.Program](size)
    if err != nil {
        return nil, err
    }

    return &CompilationCache{
        cache: cache,
    }, nil
}

// Get retrieves a compiled script from cache
func (c *CompilationCache) Get(source string) (*goja.Program, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    key := c.hash(source)
    return c.cache.Get(key)
}

// Put stores a compiled script in cache
func (c *CompilationCache) Put(source string, program *goja.Program) {
    c.mu.Lock()
    defer c.mu.Unlock()

    key := c.hash(source)
    c.cache.Add(key, program)
}

// Invalidate removes a script from cache
func (c *CompilationCache) Invalidate(source string) {
    c.mu.Lock()
    defer c.mu.Unlock()

    key := c.hash(source)
    c.cache.Remove(key)
}

// hash generates a cache key from source
func (c *CompilationCache) hash(source string) string {
    h := sha256.New()
    h.Write([]byte(source))
    return hex.EncodeToString(h.Sum(nil))
}

// Clear clears the entire cache
func (c *CompilationCache) Clear() {
    c.mu.Lock()
    defer c.mu.Unlock()

    c.cache.Purge()
}
```

---

## Console Implementation

**Location:** `modules/scripts/runtime/console.go`

```go
package runtime

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/dop251/goja"
)

// LogLevel represents log severity
type LogLevel string

const (
    LogLevelInfo  LogLevel = "info"
    LogLevelWarn  LogLevel = "warn"
    LogLevelError LogLevel = "error"
)

// LogEntry represents a console log entry
type LogEntry struct {
    Level     LogLevel    `json:"level"`
    Message   string      `json:"message"`
    Data      interface{} `json:"data,omitempty"`
    Timestamp time.Time   `json:"timestamp"`
}

// Console captures console.log output from scripts
type Console struct {
    logs []LogEntry
}

// ToGojaObject converts Console to Goja object
func (c *Console) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("log", c.log)
    obj.Set("info", c.info)
    obj.Set("warn", c.warn)
    obj.Set("error", c.error_)

    return obj
}

func (c *Console) log(call goja.FunctionCall) goja.Value {
    c.addLog(LogLevelInfo, call.Arguments)
    return goja.Undefined()
}

func (c *Console) info(call goja.FunctionCall) goja.Value {
    c.addLog(LogLevelInfo, call.Arguments)
    return goja.Undefined()
}

func (c *Console) warn(call goja.FunctionCall) goja.Value {
    c.addLog(LogLevelWarn, call.Arguments)
    return goja.Undefined()
}

func (c *Console) error_(call goja.FunctionCall) goja.Value {
    c.addLog(LogLevelError, call.Arguments)
    return goja.Undefined()
}

func (c *Console) addLog(level LogLevel, args []goja.Value) {
    if len(args) == 0 {
        return
    }

    message := args[0].String()

    var data interface{}
    if len(args) > 1 {
        data = args[1].Export()
    }

    c.logs = append(c.logs, LogEntry{
        Level:     level,
        Message:   message,
        Data:      data,
        Timestamp: time.Now(),
    })
}

// GetLogs returns all captured logs
func (c *Console) GetLogs() []LogEntry {
    return c.logs
}

// Clear clears all logs
func (c *Console) Clear() {
    c.logs = make([]LogEntry, 0)
}

// String returns logs as JSON string
func (c *Console) String() string {
    data, _ := json.MarshalIndent(c.logs, "", "  ")
    return string(data)
}
```

---

## Runtime Implementation

**Location:** `modules/scripts/runtime/goja_runtime.go`

```go
package runtime

import (
    "context"
    "fmt"

    "github.com/dop251/goja"
)

// GojaRuntime implements Runtime interface using Goja
type GojaRuntime struct {
    config   Config
    pool     *Pool
    executor *Executor
    cache    *CompilationCache
}

// NewGojaRuntime creates a new Goja-based runtime
func NewGojaRuntime(config Config) (*GojaRuntime, error) {
    // Create compilation cache
    cache, err := NewCompilationCache(config.CacheSize)
    if err != nil {
        return nil, fmt.Errorf("failed to create cache: %w", err)
    }

    // Create VM pool
    pool := NewPool(config.PoolSize, config.Bindings)

    // Create executor
    executor := NewExecutor(pool, ResourceLimits{
        MaxExecutionTimeMs: config.MaxExecutionTimeMs,
        MaxMemoryMB:        config.MaxMemoryMB,
        MaxAPICalls:        config.MaxAPICalls,
    })

    return &GojaRuntime{
        config:   config,
        pool:     pool,
        executor: executor,
        cache:    cache,
    }, nil
}

// ValidateSyntax checks if script has valid JavaScript syntax
func (r *GojaRuntime) ValidateSyntax(source string) error {
    // Try to compile
    _, err := goja.Compile("", source, true)
    if err != nil {
        return fmt.Errorf("syntax error: %w", err)
    }
    return nil
}

// Execute runs a script with given input
func (r *GojaRuntime) Execute(
    ctx context.Context,
    source string,
    input map[string]interface{},
) (map[string]interface{}, error) {
    // Check cache first
    if program, found := r.cache.Get(source); found {
        return r.executeProgram(ctx, program, input)
    }

    // Compile
    program, err := goja.Compile("", source, true)
    if err != nil {
        return nil, fmt.Errorf("compilation error: %w", err)
    }

    // Cache compiled program
    r.cache.Put(source, program)

    return r.executeProgram(ctx, program, input)
}

func (r *GojaRuntime) executeProgram(
    ctx context.Context,
    program *goja.Program,
    input map[string]interface{},
) (map[string]interface{}, error) {
    // Use executor to run with resource limits
    return r.executor.Execute(ctx, program.String(), input)
}

// Shutdown gracefully shuts down the runtime
func (r *GojaRuntime) Shutdown() error {
    r.pool.Shutdown()
    r.cache.Clear()
    return nil
}
```

---

## Configuration

**Environment Variables:**

```bash
# Runtime configuration
SCRIPTS_ENABLED=true
SCRIPT_VM_POOL_SIZE=10
SCRIPT_TIMEOUT_MS=30000
SCRIPT_MAX_MEMORY_MB=128
SCRIPT_MAX_API_CALLS=100
SCRIPT_CACHE_SIZE=1000
```

**Config Loading:**

```go
// In modules/scripts/module.go
func loadRuntimeConfig() runtime.Config {
    return runtime.Config{
        PoolSize:           getEnvInt("SCRIPT_VM_POOL_SIZE", 10),
        MaxExecutionTimeMs: getEnvInt("SCRIPT_TIMEOUT_MS", 30000),
        MaxMemoryMB:        getEnvInt("SCRIPT_MAX_MEMORY_MB", 128),
        MaxAPICalls:        getEnvInt("SCRIPT_MAX_API_CALLS", 100),
        CacheSize:          getEnvInt("SCRIPT_CACHE_SIZE", 1000),
        Bindings:           NewAPIBindings(), // From 07-api-bindings.md
    }
}
```

---

## Testing

**Location:** `modules/scripts/runtime/executor_test.go`

```go
func TestExecutor_Execute_Success(t *testing.T) {
    bindings := NewMockAPIBindings()
    pool := NewPool(2, bindings)
    executor := NewExecutor(pool, ResourceLimits{
        MaxExecutionTimeMs: 5000,
        MaxMemoryMB:        64,
        MaxAPICalls:        10,
    })

    source := `
        const result = 2 + 2;
        return { result: result };
    `

    result, err := executor.Execute(context.Background(), source, nil)
    assert.NoError(t, err)
    assert.Equal(t, 4.0, result["result"])
}

func TestExecutor_Execute_Timeout(t *testing.T) {
    bindings := NewMockAPIBindings()
    pool := NewPool(2, bindings)
    executor := NewExecutor(pool, ResourceLimits{
        MaxExecutionTimeMs: 100, // 100ms timeout
    })

    source := `
        while (true) {
            // Infinite loop
        }
    `

    _, err := executor.Execute(context.Background(), source, nil)
    assert.ErrorIs(t, err, ErrExecutionTimeout)
}

func TestExecutor_Execute_SyntaxError(t *testing.T) {
    bindings := NewMockAPIBindings()
    pool := NewPool(2, bindings)
    executor := NewExecutor(pool, ResourceLimits{})

    source := `const x = [`

    _, err := executor.Execute(context.Background(), source, nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "compilation error")
}
```

---

## Performance Considerations

### VM Pooling

- **Pool size:** 10 VMs per instance (configurable)
- **Warm VMs:** Pre-configured with bindings
- **Reuse:** Avoid costly VM creation per execution

### Compilation Caching

- **LRU cache:** 1000 compiled scripts (configurable)
- **Key:** SHA-256 hash of source code
- **Invalidation:** On script content update

### Resource Limits

- **CPU time:** 30 seconds max (interrupt at timeout)
- **Memory:** 128MB max (monitored, not enforced by Goja)
- **API calls:** 100 max per execution (tracked in bindings)

---

## Security

### Sandboxing

**Blocked Globals:**
- `fetch`, `XMLHttpRequest`, `WebSocket` (use sdk.http instead)
- `process`, `require`, `import` (no filesystem access)
- `eval`, `Function` (prevent code injection)

**Allowed Globals:**
- `console` (custom implementation)
- `JSON`, `Math`, `Date` (safe built-ins)
- `Array`, `Object`, `String`, `Number` (primitives)

### Interrupt Handling

```go
// Timeout interrupt
go func() {
    <-ctx.Done()
    vm.Interrupt("timeout")
}()
```

### Panic Recovery

```go
defer func() {
    if r := recover(); r != nil {
        err = fmt.Errorf("script panic: %v", r)
    }
}()
```

---

## Next Steps

After implementing runtime engine:

1. **API Bindings** (07-api-bindings.md) - Inject SDK APIs into VMs
2. **Event Integration** (08-event-integration.md) - Event-triggered execution

---

## References

- Goja documentation: https://github.com/dop251/goja
- Service layer: `05-service-layer.md`
- Domain entities: `01-domain-entities.md`
