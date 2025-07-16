# Phase 1: Core Runtime - Architecture & Design

## Overview
This phase defines the core architecture, interfaces, and data structures for the JavaScript runtime foundation. The focus is on establishing a robust, secure, and performant design for executing sandboxed JavaScript within the IOTA SDK's multi-tenant environment.

## Background
- The system must be built around clear interfaces to ensure modularity and testability.
- Types should be explicit and well-defined to guide implementation and ensure safety.
- The core components are the Runtime itself, a pool for managing VM instances, and a compiler for optimizing script execution.

## Task 1.1: Core Interfaces and Types

### Objectives
- Define the primary interfaces for the `Runtime`, `VMPool`, and `Compiler`.
- Specify the data structures for configuration, metrics, and execution context.
- Outline the error handling strategy and custom error types.
- Incorporate designs for VM health checks, lifecycle management, and robust timeout handling.

### Detailed Design

#### 1. `jsruntime` Package Interfaces
The primary interface for interacting with the JS runtime will be `jsruntime.Runtime`.

`pkg/jsruntime/runtime.go`:
```go
package jsruntime

import (
    "context"
    "github.com/dop251/goja"
)

// Runtime is the main entry point for executing JavaScript.
// It manages the VM pool, compilation, and secure execution.
type Runtime interface {
    // Execute runs a script within a sandboxed VM, applying security and resource limits.
    // The provided context is used for timeouts and cancellation, which is more robust
    // than timer-based interruption.
    Execute(ctx context.Context, scriptContent string, options ...ExecutionOption) (goja.Value, error)

    // Compile pre-compiles a script source, returning a program that can be executed
    // more efficiently later. This is used for caching.
    Compile(name, source string) (*goja.Program, error)

    // Metrics returns statistics about the runtime's performance and health.
    Metrics() RuntimeMetrics
}

// ExecutionOption defines a functional option for configuring a single execution.
type ExecutionOption func(*ExecutionConfig)

// ExecutionConfig holds configuration for a single script execution.
type ExecutionConfig struct {
    // SetupFunc is a function called to prepare the VM just before execution,
    // used for injecting context-specific APIs and values.
    SetupFunc func(ctx context.Context, vm *goja.Runtime) error
    
    // CompiledProgram allows executing a pre-compiled script to improve performance.
    CompiledProgram *goja.Program
}
```

#### 2. VM Pool Interfaces
The `VMPool` is responsible for managing the lifecycle of Goja VM instances. The design must include health checks and retirement policies.

`pkg/jsruntime/pool/pool.go`:
```go
package pool

import (
    "context"
    "github.com/dop251/goja"
    "time"
)

// VM represents a single, isolated Goja virtual machine instance.
type VM struct {
    Instance  *goja.Runtime
    ID        string
    CreatedAt time.Time
    UseCount  int
    Healthy   bool
}

// VMPool manages a collection of reusable VM instances.
type VMPool interface {
    // Get retrieves a healthy VM from the pool, waiting if necessary.
    // It must not return VMs that are old, overused, or unhealthy.
    Get(ctx context.Context) (*VM, error)

    // Put returns a VM to the pool for reuse.
    Put(vm *VM)

    // Stats returns statistics about the pool's state.
    Stats() PoolStats
    
    // Close shuts down the pool and cleans up all VMs.
    Close()
}

// VMFactory is responsible for creating, resetting, and destroying VMs.
type VMFactory interface {
    Create(ctx context.Context) (*VM, error)
    Reset(vm *VM) error
    Destroy(vm *VM)
    // HealthCheck determines if a VM is in a usable state.
    HealthCheck(vm *VM) bool
}

// PoolConfig defines the configuration for the VM pool, including retirement policies.
type PoolConfig struct {
    PoolSize       int
    MaxVMAge       time.Duration // e.g., 1 hour
    MaxUseCount    int           // e.g., 1000 executions
    HealthInterval time.Duration // How often to check idle VMs
}
```

#### 3. Compiler Interface
The `Compiler` handles caching of compiled scripts.

`pkg/jsruntime/compiler/compiler.go`:
```go
package compiler

import "github.com/dop251/goja"

// Compiler is responsible for compiling and caching JavaScript source code.
type Compiler interface {
    // Compile takes a script source and returns a compiled program.
    // It should use an internal cache to avoid recompiling identical scripts.
    Compile(name, source string) (*goja.Program, error)

    // Stats returns statistics about the cache's performance.
    Stats() CacheStats
}
```

#### 4. Configuration and Metrics Structures
These structs define the configuration and observable metrics for the runtime.

`pkg/jsruntime/types.go`:
```go
package jsruntime

import "github.com/iota-uz/iota-sdk/pkg/jsruntime/pool"
import "github.com/iota-uz/iota-sdk/pkg/jsruntime/compiler"

// Config defines the configuration for the entire JavaScript runtime.
type Config struct {
    PoolSize        int
    MaxMemoryMB     int
    DefaultTimeout  time.Duration
    EnableCache     bool
    CacheSize       int
}

// RuntimeMetrics provides a snapshot of the runtime's health and performance.
type RuntimeMetrics struct {
    ExecutionsTotal   int64
    ExecutionErrors   int64
    TimeoutsTotal     int64
    PoolStats         pool.PoolStats
    CompilationStats  compiler.CacheStats
}
```

#### 5. Error Handling
Custom error types provide more context for handling failures.

`pkg/jsruntime/errors/errors.go`:
```go
package errors

import "fmt"

// ScriptError represents an error that occurred during script execution.
type ScriptError struct {
    // Type indicates the category of error (e.g., "CompilationError", "RuntimeError", "TimeoutError").
    Type      string 
    Message   string
    Line      int
    Column    int
    Retryable bool
}

func (e *ScriptError) Error() string {
    return fmt.Sprintf("%s: %s (line: %d, col: %d)", e.Type, e.Message, e.Line, e.Column)
}

// Pre-defined error types
var (
    ErrTimeout        = &ScriptError{Type: "TimeoutError", Message: "script execution timed out", Retryable: true}
    ErrMemoryExceeded = &ScriptError{Type: "ResourceError", Message: "memory limit exceeded", Retryable: false}
    // ... other resource-related errors
)
```

## Task 1.2: Context and Security Design

### Objectives
- Define how the Go `context.Context` is bridged to the JavaScript environment.
- Design the security model, including resource limits and sandboxing.
- Specify the structure of the global `context` object available within scripts, ensuring it is read-only.

### Detailed Design

#### 1. Context Bridge Design
A `ContextBridge` will be responsible for reading data from a Go `context.Context` and injecting it into a Goja VM.

`pkg/jsruntime/context/bridge.go`:
```go
package context

import (
    "context"
    "github.com/dop251/goja"
)

// Bridge reads from a Go context and installs a corresponding `context` object
// into a Goja VM.
type Bridge interface {
    // Install creates and injects the read-only context object.
    Install(vm *goja.Runtime) error
}

// NewBridge creates a bridge for the given Go context.
func NewBridge(ctx context.Context) Bridge {
    // ... implementation
}
```

#### 2. The JavaScript `context` Object
This object will be globally available and **immutable** within the script. This is a critical security feature to prevent scripts from altering their execution context.

```typescript
// TypeScript definition for the global context object
declare const context: Readonly<{
    // Information about the current tenant
    tenant: Readonly<{
        id: string;
        name: string;
    }>;
    
    // Information about the user executing the script
    user: Readonly<{
        id: string;
        email: string;
        permissions: readonly string[];
    }>;
    
    // Information about the current execution
    execution: Readonly<{
        id: string;
        type: 'cron' | 'http' | 'one_off' | 'embedded';
        triggeredBy: 'user' | 'system' | 'api';
    }>;
    
    // The request ID for tracing purposes
    requestID: string;
}>;
```
The implementation will use `Object.freeze()` in JavaScript to enforce immutability.

#### 3. Security and Resource Management
Resource limits will be enforced by an interrupter that monitors the VM during execution. This interrupter must be tied to the `context.Context` of the `Execute` call to ensure it is properly garbage collected and does not affect reused VMs.

`pkg/jsruntime/security/limiter.go`:
```go
package security

import (
    "context"
    "github.com/dop251/goja"
    "time"
)

// ResourceLimits defines the execution constraints for a script.
type ResourceLimits struct {
    MaxMemoryBytes int64
    MaxCPUTime     time.Duration
    MaxAPICalls    int
}

// StartLimiter begins monitoring a VM's execution against a set of limits.
// It uses the Go context for cancellation and returns a function to stop the monitoring.
// This approach ensures that the monitoring stops as soon as the execution is complete or the context is cancelled.
func StartLimiter(ctx context.Context, vm *goja.Runtime, limits ResourceLimits) (stop func()) {
    // ... implementation that periodically checks resources and calls vm.Interrupt()
}
```

## Task 1.3: Implementation Task Breakdown

### Core Runtime Foundation
- [ ] Create `pkg/jsruntime` package structure.
- [ ] Implement the `VMPool` with health checks and retirement policies (age, use count).
- [ ] Implement the `VMFactory` with proper timeout and memory limit configurations.
- [ ] Implement the `Compiler` with an LRU cache for compiled scripts.
- [ ] Implement the main `Runtime` that integrates the pool and compiler.
- [ ] Ensure the execution loop correctly uses `context.Context` for cancellation and robust timeout handling.
- [ ] Add comprehensive error handling and panic recovery to the execution loop.
- [ ] Write unit tests for the VM pool, factory, and compiler.

### Context and Security
- [ ] Implement the `ContextBridge` to create the `context` object.
- [ ] Ensure the `context` object is made read-only within the JavaScript environment using `Object.freeze()`.
- [ ] Implement the `StartLimiter` to enforce CPU and memory limits.
- [ ] Integrate the `ContextBridge` and `StartLimiter` into the `Runtime.Execute` method.
- [ ] Write integration tests to verify context propagation and immutability.
- [ ] Write security-focused tests to ensure resource limits are enforced and that context cancellation correctly interrupts script execution.

### Deliverables Checklist
- [x] Finalized interfaces for `Runtime`, `VMPool`, and `Compiler`.
- [x] Defined all necessary data structures for configuration and metrics.
- [x] Designed the `context` object and the `ContextBridge` with immutability.
- [x] Outlined the security model for resource limiting and robust timeout handling.
- [x] Specified the error handling strategy with custom error types.
- [x] A detailed task list for the implementation of this phase.

## Success Criteria
- The defined interfaces and types are clear, comprehensive, and sufficient for building the runtime.
- The design promotes modularity, testability, and security.
- The design explicitly addresses previously identified issues like timeout handling and context immutability.
- The proposed structure aligns with the existing architecture of the IOTA SDK.
- The plan is approved and ready for implementation in the subsequent phases.
