# JavaScript Runtime - Advanced Features Specification

## Overview

This specification defines advanced production features including cron scheduling, HTTP endpoint registration, monitoring/metrics, performance optimization, and health checks.

## Cron Scheduler

### Architecture

```
┌─────────────────────────────────────────────────┐
│              Cron Scheduler                     │
│  (Ticker runs every minute)                     │
└─────────────────────────────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │ Load Scheduled Scripts  │
        │ (WHERE enabled=true     │
        │  AND trigger='scheduled'│
        │  AND schedule IS SET)   │
        └─────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │  Parse Cron Expression  │
        │  (robfig/cron/v3)       │
        └─────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │   Check Due Scripts     │
        │  (next run <= now)      │
        └─────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │  Prevent Concurrent     │
        │  Execution (sync.Map)   │
        └─────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │  Execute Asynchronously │
        │  (goroutine)            │
        └─────────────────────────┘
                      ↓
        ┌─────────────────────────┐
        │  Update last_executed   │
        │  (on completion)        │
        └─────────────────────────┘
```

### Implementation

```go
package jsruntime

import (
    "context"
    "sync"
    "time"

    "github.com/robfig/cron/v3"
    "github.com/yourorg/yourapp/modules/scripts/domain/script"
    "github.com/yourorg/yourapp/modules/scripts/services"
)

// Scheduler manages scheduled script execution
type Scheduler struct {
    scriptRepo   script.Repository
    executionSvc *services.ExecutionService
    ticker       *time.Ticker
    stop         chan struct{}
    running      sync.Map // map[uint]bool to prevent concurrent execution
    cronParser   cron.Parser
}

// NewScheduler creates a new scheduler instance
func NewScheduler(
    scriptRepo script.Repository,
    executionSvc *services.ExecutionService,
) *Scheduler {
    return &Scheduler{
        scriptRepo:   scriptRepo,
        executionSvc: executionSvc,
        ticker:       time.NewTicker(1 * time.Minute),
        stop:         make(chan struct{}),
        cronParser: cron.NewParser(
            cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
        ),
    }
}

// Start begins the scheduler loop
func (s *Scheduler) Start(ctx context.Context) {
    go func() {
        for {
            select {
            case <-s.ticker.C:
                s.runDueScripts(ctx)
            case <-s.stop:
                s.ticker.Stop()
                return
            }
        }
    }()
}

// Stop halts the scheduler
func (s *Scheduler) Stop() {
    close(s.stop)
}

// runDueScripts checks and executes all due scheduled scripts
func (s *Scheduler) runDueScripts(ctx context.Context) {
    // Fetch all enabled scheduled scripts
    scripts, _, err := s.scriptRepo.FindAll(ctx, script.FindParams{
        TriggerType: "scheduled",
        Enabled:     true,
        Limit:       1000, // Max scheduled scripts per tenant
    })
    if err != nil {
        // Log error but continue
        return
    }

    now := time.Now()

    for _, scr := range scripts {
        // Parse cron schedule
        schedule, err := s.cronParser.Parse(scr.GetSchedule())
        if err != nil {
            // Log invalid cron expression
            continue
        }

        // Get last execution time
        lastExec := scr.GetLastExecutedAt()
        if lastExec == nil {
            // Never executed, use creation time
            createdAt := scr.GetCreatedAt()
            lastExec = &createdAt
        }

        // Calculate next run time
        nextRun := schedule.Next(*lastExec)

        // Check if script is due
        if nextRun.After(now) {
            continue // Not due yet
        }

        // Prevent concurrent execution of same script
        if _, running := s.running.LoadOrStore(scr.GetID(), true); running {
            continue // Already running
        }

        // Execute asynchronously
        go func(script script.Script) {
            defer s.running.Delete(script.GetID())

            // Create tenant context
            execCtx := context.Background()
            execCtx = composables.WithTenantID(execCtx, script.GetTenantID())
            execCtx = composables.WithOrgID(execCtx, script.GetOrgID())

            // Execute script
            _, err := s.executionSvc.Execute(execCtx, script.GetID(), nil)
            if err != nil {
                // Log execution error
            }
        }(scr)
    }
}
```

### Cron Expression Validation

```go
package services

import (
    "fmt"

    "github.com/robfig/cron/v3"
)

// ValidateCronSchedule checks if cron expression is valid
func ValidateCronSchedule(schedule string) error {
    parser := cron.NewParser(
        cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
    )

    _, err := parser.Parse(schedule)
    if err != nil {
        return fmt.Errorf("invalid cron expression: %w", err)
    }

    return nil
}

// GetNextRunTime calculates next execution time
func GetNextRunTime(schedule string, lastRun time.Time) (time.Time, error) {
    parser := cron.NewParser(
        cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
    )

    parsed, err := parser.Parse(schedule)
    if err != nil {
        return time.Time{}, err
    }

    return parsed.Next(lastRun), nil
}
```

### Dead Letter Queue for Failed Executions

```go
package jsruntime

import (
    "context"
    "time"

    "github.com/yourorg/yourapp/modules/scripts/domain/execution"
)

// DeadLetterQueue stores failed scheduled executions for retry
type DeadLetterQueue struct {
    maxRetries   int
    retryDelay   time.Duration
    executionSvc *services.ExecutionService
}

func NewDeadLetterQueue(executionSvc *services.ExecutionService) *DeadLetterQueue {
    return &DeadLetterQueue{
        maxRetries:   3,
        retryDelay:   5 * time.Minute,
        executionSvc: executionSvc,
    }
}

// Enqueue adds failed execution for retry
func (dlq *DeadLetterQueue) Enqueue(ctx context.Context, exec execution.Execution) {
    retryCount := exec.GetRetryCount()
    if retryCount >= dlq.maxRetries {
        // Max retries exceeded, log and abandon
        return
    }

    // Schedule retry after delay
    time.AfterFunc(dlq.retryDelay, func() {
        dlq.retry(ctx, exec)
    })
}

func (dlq *DeadLetterQueue) retry(ctx context.Context, exec execution.Execution) {
    // Increment retry count
    // Re-execute script
    _, err := dlq.executionSvc.Execute(ctx, exec.GetScriptID(), nil)
    if err != nil {
        // Failed again, re-enqueue
        dlq.Enqueue(ctx, exec)
    }
}
```

## HTTP Endpoint Registration

### Dynamic Route Registration

```go
package jsruntime

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/gorilla/mux"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/middleware"
    "github.com/yourorg/yourapp/modules/scripts/domain/script"
    "github.com/yourorg/yourapp/modules/scripts/services"
)

// EndpointRouter manages HTTP endpoints for webhook-triggered scripts
type EndpointRouter struct {
    scriptRepo     script.Repository
    executionSvc   *services.ExecutionService
    authMiddleware mux.MiddlewareFunc
    router         *mux.Router
    registeredPaths map[string]uint // path -> script_id
}

func NewEndpointRouter(
    scriptRepo script.Repository,
    executionSvc *services.ExecutionService,
    authMiddleware mux.MiddlewareFunc,
) *EndpointRouter {
    return &EndpointRouter{
        scriptRepo:      scriptRepo,
        executionSvc:    executionSvc,
        authMiddleware:  authMiddleware,
        registeredPaths: make(map[string]uint),
    }
}

// Register sets up the base webhook route
func (r *EndpointRouter) Register(router *mux.Router) {
    r.router = router

    // Webhook endpoint prefix
    subRouter := router.PathPrefix("/api/webhooks").Subrouter()

    // Optional authentication (configurable per script)
    subRouter.Use(r.authMiddleware)

    // Dynamic handler for all webhook paths
    subRouter.PathPrefix("/").HandlerFunc(r.handleScriptEndpoint)

    // Reload endpoints on startup
    r.ReloadEndpoints(context.Background())
}

// ReloadEndpoints loads all webhook scripts and builds path mapping
func (r *EndpointRouter) ReloadEndpoints(ctx context.Context) error {
    // Fetch all enabled webhook scripts
    scripts, _, err := r.scriptRepo.FindAll(ctx, script.FindParams{
        TriggerType: "webhook",
        Enabled:     true,
        Limit:       1000,
    })
    if err != nil {
        return err
    }

    // Clear existing mappings
    r.registeredPaths = make(map[string]uint)

    // Register each script's webhook path
    for _, scr := range scripts {
        path := scr.GetWebhookPath()
        if path == "" {
            continue
        }

        // Normalize path (ensure leading slash)
        if !strings.HasPrefix(path, "/") {
            path = "/" + path
        }

        r.registeredPaths[path] = scr.GetID()
    }

    return nil
}

// handleScriptEndpoint dynamically routes to the correct script
func (r *EndpointRouter) handleScriptEndpoint(w http.ResponseWriter, req *http.Request) {
    ctx := req.Context()
    path := req.URL.Path

    // Strip /api/webhooks prefix
    path = strings.TrimPrefix(path, "/api/webhooks")

    // Lookup script ID by path
    scriptID, exists := r.registeredPaths[path]
    if !exists {
        http.Error(w, "Webhook endpoint not found", http.StatusNotFound)
        return
    }

    // Parse request body
    var inputData map[string]interface{}
    if req.Body != nil && req.ContentLength > 0 {
        if err := json.NewDecoder(req.Body).Decode(&inputData); err != nil {
            // If not JSON, store raw body as string
            inputData = map[string]interface{}{
                "body": req.Body,
            }
        }
    }

    // Add request metadata to input
    inputData["method"] = req.Method
    inputData["headers"] = req.Header
    inputData["query"] = req.URL.Query()

    // Execute script asynchronously
    execution, err := r.executionSvc.Execute(ctx, scriptID, inputData)
    if err != nil {
        http.Error(w, fmt.Sprintf("Execution failed: %v", err), http.StatusInternalServerError)
        return
    }

    // Wait for execution to complete (with timeout)
    execCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    result, err := r.waitForExecution(execCtx, execution.GetID())
    if err != nil {
        http.Error(w, fmt.Sprintf("Execution timeout: %v", err), http.StatusGatewayTimeout)
        return
    }

    // Return result as JSON
    w.Header().Set("Content-Type", "application/json")
    if result.GetStatus() == "failed" {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": result.GetErrorMessage(),
        })
        return
    }

    // Parse output as JSON if possible
    var output interface{}
    if err := json.Unmarshal([]byte(result.GetOutput()), &output); err != nil {
        // Not JSON, return as string
        output = result.GetOutput()
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "result": output,
    })
}

// waitForExecution polls for execution completion
func (r *EndpointRouter) waitForExecution(ctx context.Context, executionID uint) (execution.Execution, error) {
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-ticker.C:
            exec, err := r.executionSvc.FindByID(ctx, executionID)
            if err != nil {
                return nil, err
            }

            status := exec.GetStatus()
            if status == "completed" || status == "failed" {
                return exec, nil
            }
        }
    }
}
```

### Rate Limiting per Endpoint

```go
package jsruntime

import (
    "net/http"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

// EndpointRateLimiter applies per-script rate limits
type EndpointRateLimiter struct {
    limiters map[uint]*rate.Limiter // script_id -> limiter
    mu       sync.RWMutex
}

func NewEndpointRateLimiter() *EndpointRateLimiter {
    return &EndpointRateLimiter{
        limiters: make(map[uint]*rate.Limiter),
    }
}

// GetLimiter returns rate limiter for script (creates if not exists)
func (rl *EndpointRateLimiter) GetLimiter(scriptID uint) *rate.Limiter {
    rl.mu.RLock()
    limiter, exists := rl.limiters[scriptID]
    rl.mu.RUnlock()

    if exists {
        return limiter
    }

    rl.mu.Lock()
    defer rl.mu.Unlock()

    // Double-check after acquiring write lock
    if limiter, exists := rl.limiters[scriptID]; exists {
        return limiter
    }

    // Create new limiter: 10 requests per second, burst of 20
    limiter = rate.NewLimiter(rate.Limit(10), 20)
    rl.limiters[scriptID] = limiter
    return limiter
}

// Middleware wraps handler with rate limiting
func (rl *EndpointRateLimiter) Middleware(scriptID uint) mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            limiter := rl.GetLimiter(scriptID)
            if !limiter.Allow() {
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

## Monitoring & Metrics (Prometheus)

### Metric Definitions

```go
package jsruntime

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Script execution metrics
    scriptExecutions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "script_executions_total",
            Help: "Total number of script executions",
        },
        []string{"tenant_id", "script_id", "trigger_type", "status"},
    )

    scriptExecutionDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "script_execution_duration_seconds",
            Help:    "Script execution duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to 16s
        },
        []string{"tenant_id", "script_id", "trigger_type"},
    )

    scriptExecutionErrors = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "script_execution_errors_total",
            Help: "Total number of script execution errors",
        },
        []string{"tenant_id", "script_id", "error_type"},
    )

    // VM pool metrics
    vmPoolSize = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "jsruntime_vm_pool_size",
            Help: "Total number of VMs in pool",
        },
    )

    vmPoolInUse = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "jsruntime_vm_pool_in_use",
            Help: "Number of VMs currently in use",
        },
    )

    vmPoolWaitTime = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name:    "jsruntime_vm_pool_wait_seconds",
            Help:    "Time spent waiting for VM from pool",
            Buckets: prometheus.ExponentialBuckets(0.0001, 2, 12), // 0.1ms to 400ms
        },
    )

    // API call metrics
    scriptAPICalls = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "script_api_calls_total",
            Help: "Total number of SDK API calls from scripts",
        },
        []string{"tenant_id", "api_type"}, // api_type: http, db, cache, etc.
    )

    scriptAPICallDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "script_api_call_duration_seconds",
            Help:    "SDK API call duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 12), // 1ms to 4s
        },
        []string{"tenant_id", "api_type"},
    )

    // Resource usage metrics
    scriptMemoryUsage = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "script_memory_usage_bytes",
            Help:    "Script memory usage in bytes",
            Buckets: prometheus.ExponentialBuckets(1024, 2, 20), // 1KB to 512MB
        },
        []string{"tenant_id", "script_id"},
    )

    scriptCPUTime = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "script_cpu_time_seconds",
            Help:    "Script CPU time in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15),
        },
        []string{"tenant_id", "script_id"},
    )

    // Scheduler metrics
    schedulerTicks = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "scheduler_ticks_total",
            Help: "Total number of scheduler ticks",
        },
    )

    scheduledScriptsDue = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "scheduled_scripts_due",
            Help: "Number of scheduled scripts currently due",
        },
    )

    scheduledScriptsRunning = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "scheduled_scripts_running",
            Help: "Number of scheduled scripts currently running",
        },
    )
)
```

### Metric Instrumentation

```go
package jsruntime

import (
    "context"
    "strconv"
    "time"

    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// RecordExecution instruments script execution
func RecordExecution(ctx context.Context, scriptID uint, triggerType string, status string, duration time.Duration) {
    tenantID := composables.UseTenantID(ctx)

    labels := prometheus.Labels{
        "tenant_id":    strconv.Itoa(int(tenantID)),
        "script_id":    strconv.Itoa(int(scriptID)),
        "trigger_type": triggerType,
        "status":       status,
    }

    scriptExecutions.With(labels).Inc()

    durationLabels := prometheus.Labels{
        "tenant_id":    strconv.Itoa(int(tenantID)),
        "script_id":    strconv.Itoa(int(scriptID)),
        "trigger_type": triggerType,
    }
    scriptExecutionDuration.With(durationLabels).Observe(duration.Seconds())
}

// RecordAPICall instruments SDK API calls
func RecordAPICall(ctx context.Context, apiType string, duration time.Duration) {
    tenantID := composables.UseTenantID(ctx)

    labels := prometheus.Labels{
        "tenant_id": strconv.Itoa(int(tenantID)),
        "api_type":  apiType,
    }

    scriptAPICalls.With(labels).Inc()
    scriptAPICallDuration.With(labels).Observe(duration.Seconds())
}

// RecordVMPoolMetrics updates VM pool metrics
func (p *VMPool) RecordMetrics() {
    vmPoolSize.Set(float64(p.maxSize))
    vmPoolInUse.Set(float64(p.maxSize - len(p.vms)))
}
```

### Example Prometheus Query Dashboard

```yaml
# Grafana Dashboard Queries

# Script Execution Rate
rate(script_executions_total[5m])

# Script Error Rate
rate(script_execution_errors_total[5m]) / rate(script_executions_total[5m])

# P95 Execution Duration
histogram_quantile(0.95, rate(script_execution_duration_seconds_bucket[5m]))

# VM Pool Utilization
jsruntime_vm_pool_in_use / jsruntime_vm_pool_size

# Top Slowest Scripts
topk(10, avg(rate(script_execution_duration_seconds_sum[5m])) by (script_id))

# API Call Distribution
sum(rate(script_api_calls_total[5m])) by (api_type)
```

## Performance Optimization

### Script Compilation Caching

```go
package jsruntime

import (
    "sync"

    "github.com/dop251/goja"
    lru "github.com/hashicorp/golang-lru"
)

// CompilationCache stores compiled scripts
type CompilationCache struct {
    cache *lru.Cache
    mu    sync.RWMutex
}

// NewCompilationCache creates LRU cache for compiled scripts
func NewCompilationCache(size int) (*CompilationCache, error) {
    cache, err := lru.New(size)
    if err != nil {
        return nil, err
    }

    return &CompilationCache{
        cache: cache,
    }, nil
}

// Get retrieves compiled program from cache
func (cc *CompilationCache) Get(scriptID uint, version int) (*goja.Program, bool) {
    cc.mu.RLock()
    defer cc.mu.RUnlock()

    key := compilationKey(scriptID, version)
    if val, ok := cc.cache.Get(key); ok {
        return val.(*goja.Program), true
    }
    return nil, false
}

// Set stores compiled program in cache
func (cc *CompilationCache) Set(scriptID uint, version int, program *goja.Program) {
    cc.mu.Lock()
    defer cc.mu.Unlock()

    key := compilationKey(scriptID, version)
    cc.cache.Add(key, program)
}

// Invalidate removes script from cache
func (cc *CompilationCache) Invalidate(scriptID uint) {
    cc.mu.Lock()
    defer cc.mu.Unlock()

    // LRU doesn't support prefix deletion, so we'd need to track keys
    // For simplicity, clear entire cache on invalidation
    cc.cache.Purge()
}

func compilationKey(scriptID uint, version int) string {
    return fmt.Sprintf("%d:%d", scriptID, version)
}
```

### VM Pool Tuning

```go
package jsruntime

import (
    "context"
    "runtime"
)

// AdaptiveVMPool adjusts pool size based on load
type AdaptiveVMPool struct {
    *VMPool
    minSize int
    maxSize int
}

func NewAdaptiveVMPool(ctx context.Context, minSize, maxSize int) (*AdaptiveVMPool, error) {
    pool, err := NewVMPool(ctx, minSize)
    if err != nil {
        return nil, err
    }

    return &AdaptiveVMPool{
        VMPool:  pool,
        minSize: minSize,
        maxSize: maxSize,
    }, nil
}

// Scale adjusts pool size based on utilization
func (p *AdaptiveVMPool) Scale() {
    inUse := p.maxSize - len(p.vms)
    utilization := float64(inUse) / float64(p.maxSize)

    if utilization > 0.8 && p.maxSize < p.maxSize {
        // High utilization, grow pool
        p.grow()
    } else if utilization < 0.2 && p.maxSize > p.minSize {
        // Low utilization, shrink pool
        p.shrink()
    }
}

func (p *AdaptiveVMPool) grow() {
    // Add one VM to pool
    vm, err := NewSandboxedVM(p.ctx)
    if err != nil {
        return
    }
    p.vms <- vm
    p.maxSize++
}

func (p *AdaptiveVMPool) shrink() {
    // Remove one VM from pool
    select {
    case <-p.vms:
        p.maxSize--
    default:
        // Pool empty, can't shrink
    }
}
```

### Database Query Optimization

```go
package persistence

import (
    "context"
    "database/sql"

    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// Optimized query with indexes
const findScheduledScriptsQuery = `
    SELECT
        id, name, source, schedule, last_executed_at, tenant_id, org_id
    FROM
        scripts
    WHERE
        tenant_id = $1
        AND trigger_type = 'scheduled'
        AND enabled = true
        AND (last_executed_at IS NULL OR last_executed_at < NOW() - INTERVAL '1 minute')
    ORDER BY
        last_executed_at ASC NULLS FIRST
    LIMIT $2
`

// Database indexes for performance:
/*
CREATE INDEX idx_scripts_scheduled ON scripts(tenant_id, trigger_type, enabled, last_executed_at)
    WHERE trigger_type = 'scheduled' AND enabled = true;

CREATE INDEX idx_scripts_webhook ON scripts(tenant_id, trigger_type, webhook_path)
    WHERE trigger_type = 'webhook' AND enabled = true;

CREATE INDEX idx_executions_script ON executions(script_id, started_at DESC);

CREATE INDEX idx_executions_status ON executions(tenant_id, status, started_at DESC);
*/
```

### Connection Pooling

```go
package infrastructure

import (
    "database/sql"
    "time"
)

// ConfigureDBPool optimizes database connection pool
func ConfigureDBPool(db *sql.DB) {
    // Maximum number of open connections
    db.SetMaxOpenConns(25)

    // Maximum number of idle connections
    db.SetMaxIdleConns(10)

    // Maximum lifetime of a connection
    db.SetConnMaxLifetime(5 * time.Minute)

    // Maximum idle time for a connection
    db.SetConnMaxIdleTime(1 * time.Minute)
}
```

## Health Checks

### Health Check Endpoints

```go
package jsruntime

import (
    "context"
    "encoding/json"
    "net/http"
    "time"
)

// HealthChecker monitors runtime health
type HealthChecker struct {
    vmPool       *VMPool
    db           *sql.DB
    scheduler    *Scheduler
}

func NewHealthChecker(vmPool *VMPool, db *sql.DB, scheduler *Scheduler) *HealthChecker {
    return &HealthChecker{
        vmPool:    vmPool,
        db:        db,
        scheduler: scheduler,
    }
}

// HealthStatus represents system health
type HealthStatus struct {
    Healthy   bool              `json:"healthy"`
    Timestamp time.Time         `json:"timestamp"`
    Checks    map[string]Check  `json:"checks"`
}

type Check struct {
    Status  string `json:"status"` // "pass", "warn", "fail"
    Message string `json:"message"`
}

// Check performs all health checks
func (hc *HealthChecker) Check(ctx context.Context) HealthStatus {
    checks := make(map[string]Check)
    healthy := true

    // VM Pool health
    vmCheck := hc.checkVMPool()
    checks["vm_pool"] = vmCheck
    if vmCheck.Status == "fail" {
        healthy = false
    }

    // Database connectivity
    dbCheck := hc.checkDatabase(ctx)
    checks["database"] = dbCheck
    if dbCheck.Status == "fail" {
        healthy = false
    }

    // Scheduler status
    schedulerCheck := hc.checkScheduler()
    checks["scheduler"] = schedulerCheck
    if schedulerCheck.Status == "fail" {
        healthy = false
    }

    return HealthStatus{
        Healthy:   healthy,
        Timestamp: time.Now(),
        Checks:    checks,
    }
}

func (hc *HealthChecker) checkVMPool() Check {
    available := len(hc.vmPool.vms)
    total := hc.vmPool.maxSize

    if available == 0 {
        return Check{Status: "fail", Message: "VM pool exhausted"}
    }

    utilization := float64(total-available) / float64(total)
    if utilization > 0.9 {
        return Check{Status: "warn", Message: "VM pool utilization > 90%"}
    }

    return Check{Status: "pass", Message: fmt.Sprintf("%d/%d VMs available", available, total)}
}

func (hc *HealthChecker) checkDatabase(ctx context.Context) Check {
    ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
    defer cancel()

    if err := hc.db.PingContext(ctx); err != nil {
        return Check{Status: "fail", Message: fmt.Sprintf("Database unreachable: %v", err)}
    }

    return Check{Status: "pass", Message: "Database connected"}
}

func (hc *HealthChecker) checkScheduler() Check {
    // Check if scheduler is running (basic check)
    select {
    case <-hc.scheduler.stop:
        return Check{Status: "fail", Message: "Scheduler stopped"}
    default:
        return Check{Status: "pass", Message: "Scheduler running"}
    }
}

// Handler provides HTTP endpoint
func (hc *HealthChecker) Handler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status := hc.Check(r.Context())

        w.Header().Set("Content-Type", "application/json")
        if !status.Healthy {
            w.WriteHeader(http.StatusServiceUnavailable)
        }

        json.NewEncoder(w).Encode(status)
    }
}
```

### Readiness vs Liveness Probes

```go
package jsruntime

// Liveness probe: Is the service running?
func (hc *HealthChecker) Liveness() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Simple check: service is alive if it can respond
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("OK"))
    }
}

// Readiness probe: Can the service handle requests?
func (hc *HealthChecker) Readiness() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status := hc.Check(r.Context())

        if !status.Healthy {
            w.WriteHeader(http.StatusServiceUnavailable)
            json.NewEncoder(w).Encode(status)
            return
        }

        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Ready"))
    }
}
```

### Registration in Router

```go
package main

import (
    "github.com/gorilla/mux"
)

func RegisterHealthChecks(router *mux.Router, checker *jsruntime.HealthChecker) {
    router.HandleFunc("/health", checker.Handler()).Methods("GET")
    router.HandleFunc("/health/live", checker.Liveness()).Methods("GET")
    router.HandleFunc("/health/ready", checker.Readiness()).Methods("GET")
}
```

## Load Testing Example

```go
package jsruntime_test

import (
    "context"
    "sync"
    "testing"
    "time"
)

func BenchmarkScriptExecution(b *testing.B) {
    // Setup
    env := itf.Setup(b, itf.WithPermissions(permissions.AllScriptPermissions()...))
    executionSvc := itf.GetService[*services.ExecutionService](env)

    ctx := env.Context()
    scriptID := createTestScript(ctx, env)

    b.ResetTimer()

    // Run N executions
    for i := 0; i < b.N; i++ {
        _, err := executionSvc.Execute(ctx, scriptID, nil)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func TestConcurrentExecutions(t *testing.T) {
    env := itf.Setup(t, itf.WithPermissions(permissions.AllScriptPermissions()...))
    executionSvc := itf.GetService[*services.ExecutionService](env)

    ctx := env.Context()
    scriptID := createTestScript(ctx, env)

    concurrency := 100
    var wg sync.WaitGroup
    wg.Add(concurrency)

    start := time.Now()

    for i := 0; i < concurrency; i++ {
        go func() {
            defer wg.Done()
            _, err := executionSvc.Execute(ctx, scriptID, nil)
            if err != nil {
                t.Error(err)
            }
        }()
    }

    wg.Wait()
    duration := time.Since(start)

    t.Logf("Executed %d scripts concurrently in %v", concurrency, duration)
    t.Logf("Throughput: %.2f executions/second", float64(concurrency)/duration.Seconds())
}
```

## Summary

This specification provides:

1. **Cron Scheduler**: Minute-based ticker, cron expression parsing, concurrent execution prevention, dead letter queue
2. **HTTP Endpoints**: Dynamic route registration, webhook path mapping, rate limiting, request/response handling
3. **Monitoring**: Prometheus metrics for executions, VM pool, API calls, resource usage, scheduler
4. **Performance Optimization**: Compilation caching, adaptive VM pool, database query optimization, connection pooling
5. **Health Checks**: VM pool, database, scheduler status checks with liveness/readiness probes
6. **Load Testing**: Benchmarks and concurrent execution tests

These features transform the JavaScript runtime from a basic execution engine into a production-ready, scalable, and observable system.

