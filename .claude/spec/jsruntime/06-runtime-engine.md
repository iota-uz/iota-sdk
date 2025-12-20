# JavaScript Runtime - Runtime Engine

## Overview

The runtime engine provides JavaScript execution using Goja VM with pooling, resource limits, compilation caching, and security sandboxing.

```mermaid
graph TB
    subgraph "Runtime Engine"
        VMPool[VM Pool Manager<br/>Lifecycle & Allocation]
        Executor[Script Executor<br/>Orchestration]
        Cache[Compilation Cache<br/>LRU + DB]
        Sandbox[Sandbox Config<br/>Global restrictions]
    end

    subgraph "Goja VM"
        VM1[VM Instance 1]
        VM2[VM Instance 2]
        VMN[VM Instance N]
    end

    subgraph "Resources"
        Memory[Memory Monitor]
        Timeout[Timeout Handler]
        Panic[Panic Recovery]
    end

    VMPool --> VM1
    VMPool --> VM2
    VMPool --> VMN
    Executor --> VMPool
    Executor --> Memory
    Executor --> Timeout
    Executor --> Panic
    Cache --> Executor
    Sandbox --> VM1
    Sandbox --> VM2
    Sandbox --> VMN
```

## VM Pool Manager

**What It Does:**
Manages a pool of pre-warmed Goja VM instances for reduced latency and fair resource distribution.

**How It Works:**
- Pre-creates configurable number of VM instances on startup
- Warm-up includes loading standard library and common APIs
- Acquires VMs on-demand with per-tenant limits
- Releases VMs back to pool after execution
- Cleans up idle VMs after timeout
- Tracks metrics (available, in-use, total)

```mermaid
stateDiagram-v2
    [*] --> Creating: Initialize Pool
    Creating --> Available: VM Warmed Up
    Available --> Acquired: Script Request
    Acquired --> Executing: Run Script
    Executing --> Resetting: Execution Complete
    Resetting --> Available: State Cleared
    Available --> Destroyed: Idle Timeout
    Destroyed --> [*]

    Executing --> Failed: Error/Timeout
    Failed --> Resetting: Cleanup
```

### Pool Configuration

**Default Settings:**
- Initial pool size: 10 VMs
- Maximum pool size: 100 VMs
- Per-tenant limit: 5 concurrent VMs
- Idle timeout: 5 minutes
- Warm-up time: <500ms per VM

**Dynamic Expansion:**
- Creates new VM if pool empty and below max
- Expands during high load
- Contracts during idle periods
- Fair scheduling across tenants

```mermaid
graph LR
    subgraph "Pool States"
        Empty[Empty Pool]
        Available[VMs Available]
        Full[Pool Exhausted]
    end

    subgraph "Actions"
        Create[Create New VM]
        Acquire[Return VM to Caller]
        Wait[Queue Request]
    end

    Empty -->|Below max| Create
    Empty -->|At max| Wait
    Available --> Acquire
    Full --> Wait

    Create --> Available
```

## Script Executor

**What It Does:**
Orchestrates script execution with timeout enforcement, memory monitoring, and panic recovery.

**Responsibilities:**
- Validate script syntax before execution
- Inject context (tenant, user, organization)
- Set resource limits (timeout, memory)
- Execute script in sandboxed VM
- Capture output and metrics
- Handle errors and panics gracefully

```mermaid
sequenceDiagram
    participant Service
    participant Executor
    participant Cache
    participant VMPool
    participant VM as Goja VM

    Service->>Executor: Execute(source, input)
    Executor->>Executor: ValidateSyntax(source)

    Executor->>Cache: Get compiled program
    alt Cache hit
        Cache-->>Executor: Compiled program
    else Cache miss
        Executor->>VM: Compile(source)
        VM-->>Executor: Program
        Executor->>Cache: Store program
    end

    Executor->>VMPool: Acquire VM
    VMPool-->>Executor: VM instance

    Executor->>VM: Inject context (tenant, user, org)
    Executor->>VM: Set timeout & memory limits
    Executor->>VM: Run program with input

    alt Success
        VM-->>Executor: Output + metrics
    else Error
        VM-->>Executor: Error details
    else Timeout
        VM-->>Executor: Timeout error
    else Panic
        Executor->>Executor: Recover from panic
        Executor-->>Service: Panic error
    end

    Executor->>VMPool: Release VM
    Executor-->>Service: Result
```

### Resource Limits Enforcement

**What It Does:**
Enforces strict resource limits to prevent abuse and ensure fair usage.

**Limits Applied:**
- **Execution Time**: Context timeout (default 30s)
- **Memory**: Goja runtime.MemoryLimit (default 64MB)
- **API Calls**: Rate limiter (default 60/minute)
- **Output Size**: Max result size (default 1MB)
- **Concurrent Runs**: Per-script concurrency (default 5)

```mermaid
graph TB
    subgraph "Resource Monitoring"
        Start[Start Execution]
        CheckTime[Check Elapsed Time]
        CheckMemory[Check Memory Usage]
        CheckAPICalls[Check API Call Rate]
        CheckOutput[Check Output Size]
    end

    subgraph "Limit Actions"
        ContinueExec[Continue Execution]
        TimeoutKill[Kill: Timeout]
        MemoryKill[Kill: Memory Exceeded]
        RateLimit[Throttle: Rate Limit]
        OutputTruncate[Truncate: Output Too Large]
    end

    Start --> CheckTime
    CheckTime -->|Within limit| CheckMemory
    CheckTime -->|Exceeded| TimeoutKill

    CheckMemory -->|Within limit| CheckAPICalls
    CheckMemory -->|Exceeded| MemoryKill

    CheckAPICalls -->|Within limit| CheckOutput
    CheckAPICalls -->|Exceeded| RateLimit

    CheckOutput -->|Within limit| ContinueExec
    CheckOutput -->|Exceeded| OutputTruncate
```

## Compilation Cache

**What It Does:**
Caches compiled JavaScript programs to reduce compilation overhead on repeated executions.

**Strategy:**
- LRU cache for frequently executed scripts
- Cache key: hash of source code
- Cache size: 1000 programs (configurable)
- Optional bytecode storage in database
- Automatic invalidation on script updates

```mermaid
graph TB
    subgraph "Compilation Flow"
        Request[Execute Request]
        CacheLookup[Check LRU Cache]
        Compile[Compile with Goja]
        Store[Store in Cache]
        Execute[Execute Program]
    end

    subgraph "Cache Management"
        LRU[LRU Eviction]
        Invalidate[Invalidate on Update]
        DBBackup[DB Bytecode Storage]
    end

    Request --> CacheLookup
    CacheLookup -->|Hit| Execute
    CacheLookup -->|Miss| Compile
    Compile --> Store
    Store --> Execute
    Store --> DBBackup

    LRU -.Evict oldest.-> Store
    Invalidate -.Clear entry.-> CacheLookup

    style CacheLookup fill:#90EE90
    style Compile fill:#FFB6C1
```

### Cache Benefits

**Performance Improvements:**
- **First run**: Parse + Compile + Execute (~50ms overhead)
- **Cached runs**: Execute only (~5ms overhead)
- **90% hit rate** for frequently used scripts
- **10x faster** for cached scripts

## Sandbox Configuration

**What It Does:**
Restricts VM global scope to prevent access to dangerous APIs and ensure security.

**Blocked Globals:**
- `require()` - No module loading
- `import()` - No dynamic imports
- `eval()` - No code evaluation (configurable)
- `Function()` - No function constructor
- File system APIs (no `fs` module)
- Process APIs (no `process`, `child_process`)
- Network APIs (except controlled HTTP client)

**Allowed Globals:**
- `console` - Custom implementation with logging
- `JSON` - Parse/stringify
- `Math` - Mathematical functions
- `Date` - Date/time operations
- `Array`, `Object`, `String`, `Number` - Standard types
- Custom SDK APIs (via injection)

```mermaid
graph TB
    subgraph "Sandboxed Global Scope"
        Safe[Safe Globals<br/>console, JSON, Math, Date]
        SDK[SDK APIs<br/>db, http, events]
        Custom[Custom Bindings<br/>context, logger]
    end

    subgraph "Blocked Access"
        Require[require BLOCKED]
        Import[import BLOCKED]
        Eval[eval BLOCKED]
        FS[File System BLOCKED]
        Process[Process BLOCKED]
        Network[Raw Network BLOCKED]
    end

    VM[Goja VM] --> Safe
    VM --> SDK
    VM --> Custom

    VM -.Attempts.-> Require
    VM -.Attempts.-> Import
    VM -.Attempts.-> Eval
    VM -.Attempts.-> FS
    VM -.Attempts.-> Process
    VM -.Attempts.-> Network

    Require -.Throws Error.-> VM
    Import -.Throws Error.-> VM
    Eval -.Throws Error.-> VM
    FS -.Throws Error.-> VM
    Process -.Throws Error.-> VM
    Network -.Throws Error.-> VM

    style Safe fill:#90EE90
    style SDK fill:#90EE90
    style Custom fill:#90EE90
    style Require fill:#FFB6C1
    style Import fill:#FFB6C1
    style Eval fill:#FFB6C1
```

## VM Lifecycle

**What It Does:**
Manages complete lifecycle from creation to destruction with proper cleanup.

**Lifecycle Stages:**
1. **Creation**: New Goja VM instance allocated
2. **Warm-up**: Load standard library and SDK APIs
3. **Ready**: Added to available pool
4. **Acquisition**: Removed from pool for execution
5. **Execution**: Script runs with resource limits
6. **Reset**: Clear state and custom globals
7. **Release**: Return to available pool
8. **Destruction**: Garbage collected after idle timeout

```mermaid
sequenceDiagram
    participant Pool as VM Pool
    participant VM as Goja VM
    participant GC as Go Garbage Collector

    Note over Pool: Initialization
    Pool->>VM: Create new VM
    VM-->>Pool: VM instance
    Pool->>VM: Load standard library
    Pool->>VM: Inject SDK APIs
    Pool->>VM: Configure sandbox

    Note over Pool: Execution Cycle
    Pool->>VM: Acquire for script
    VM->>VM: Execute script
    VM-->>Pool: Execution result
    Pool->>VM: Reset state
    Pool->>VM: Clear custom globals

    Note over Pool: Idle Timeout
    Pool->>VM: Check last used time
    alt Idle > 5 minutes
        Pool->>VM: Remove from pool
        VM->>GC: Mark for collection
        GC->>VM: Destroy
    else Still active
        Pool->>Pool: Keep in pool
    end
```

## Error Handling and Recovery

**What It Does:**
Gracefully handles errors, panics, and timeouts without crashing the application.

**Error Types:**
- **Syntax Errors**: Detected during compilation, returned before execution
- **Runtime Errors**: JavaScript exceptions caught and returned as errors
- **Timeout Errors**: Context cancellation triggers graceful shutdown
- **Panic Recovery**: Go panics caught and converted to errors
- **Memory Errors**: Out-of-memory conditions handled gracefully

```mermaid
graph TB
    subgraph "Error Sources"
        Syntax[Syntax Error<br/>Invalid JavaScript]
        Runtime[Runtime Error<br/>Exception in script]
        Timeout[Timeout Error<br/>Execution too long]
        Panic[Panic<br/>Unexpected failure]
        Memory[Memory Error<br/>Limit exceeded]
    end

    subgraph "Error Handling"
        Catch[Catch Error]
        Log[Log Error Details]
        Cleanup[Cleanup Resources]
        Return[Return Error to Caller]
    end

    Syntax --> Catch
    Runtime --> Catch
    Timeout --> Catch
    Panic --> Catch
    Memory --> Catch

    Catch --> Log
    Log --> Cleanup
    Cleanup --> Return

    Return -.Update Status.-> ExecutionRecord[Execution Record]
    Return -.Metrics.-> Monitoring[Monitoring System]
```

### Panic Recovery Pattern

**What It Does:**
Captures Go panics during script execution and converts them to structured errors.

**How It Works:**
1. Defer recovery function before execution
2. Execute script in protected context
3. On panic, capture stack trace
4. Convert panic to error with context
5. Clean up VM state
6. Return error to caller

## Performance Optimization

**Strategies Applied:**
- **VM Pooling**: Eliminate VM creation overhead (~100ms → <10ms)
- **Compilation Caching**: Reduce parse/compile time (90% hit rate)
- **Lazy Loading**: Load APIs only when needed
- **Memory Limits**: Prevent runaway memory usage
- **Concurrent Execution**: Multiple VMs execute scripts in parallel

**Performance Targets:**
- Cold start (new VM): <500ms
- Warm start (pooled VM): <100ms
- Cached execution: <50ms
- Throughput: 1000+ concurrent executions

```mermaid
graph LR
    subgraph "Optimization Techniques"
        Pool[VM Pooling<br/>-90% latency]
        Cache[Compilation Cache<br/>-80% compile time]
        Parallel[Parallel Execution<br/>+10x throughput]
        Limits[Resource Limits<br/>Prevent abuse]
    end

    subgraph "Performance Gains"
        FastStart[Fast Start<br/><100ms]
        HighThroughput[High Throughput<br/>1000+ concurrent]
        LowLatency[Low Latency<br/><50ms cached]
    end

    Pool --> FastStart
    Cache --> LowLatency
    Parallel --> HighThroughput
    Limits --> HighThroughput
```

## Acceptance Criteria

### VM Pool Manager
- ✅ Initialize pool with configurable size on startup
- ✅ Pre-warm VMs with standard library and SDK APIs
- ✅ Acquire VM with per-tenant concurrency limits
- ✅ Release VM and reset state after execution
- ✅ Cleanup idle VMs after timeout period
- ✅ Track metrics (available, in-use, total VMs)
- ✅ Graceful shutdown with drain period

### Script Executor
- ✅ Validate JavaScript syntax before execution
- ✅ Inject tenant, user, organization context
- ✅ Enforce timeout via context cancellation
- ✅ Monitor memory usage and enforce limits
- ✅ Capture output and execution metrics
- ✅ Handle errors and panics gracefully
- ✅ Return structured error with context

### Compilation Cache
- ✅ LRU cache for compiled programs (1000 entries)
- ✅ Cache key based on source code hash
- ✅ Store in memory for fast access
- ✅ Optional bytecode persistence to database
- ✅ Automatic invalidation on script updates
- ✅ 90%+ cache hit rate for frequently used scripts

### Sandbox Configuration
- ✅ Block dangerous globals (require, import, eval, fs, process)
- ✅ Allow safe globals (console, JSON, Math, Date)
- ✅ Inject SDK APIs (database, HTTP, events)
- ✅ Custom console implementation with logging
- ✅ Prevent access to Go runtime internals
- ✅ No code generation or reflection APIs

### Resource Limits
- ✅ Execution timeout enforced via context (default 30s)
- ✅ Memory limit enforced via Goja (default 64MB)
- ✅ API call rate limiting (default 60/minute)
- ✅ Output size limit (default 1MB)
- ✅ Per-script concurrency limit (default 5)

### Error Handling
- ✅ Syntax errors detected during compilation
- ✅ Runtime errors caught and structured
- ✅ Timeout errors returned with context
- ✅ Panics recovered and converted to errors
- ✅ Memory errors handled gracefully
- ✅ All errors logged with stack traces

### Performance
- ✅ Cold start (new VM) <500ms
- ✅ Warm start (pooled VM) <100ms
- ✅ Cached execution <50ms
- ✅ Support 1000+ concurrent executions
- ✅ Compilation cache hit rate >90%
- ✅ VM pool utilization 60-80% under normal load
