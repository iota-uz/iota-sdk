# JavaScript Runtime - Security Model Specification

## Overview

This specification defines comprehensive security controls for the JavaScript runtime, including VM sandboxing, resource limits, SSRF protection, tenant isolation, RBAC permissions, input validation, and audit trails.

## Security Layers

```
┌─────────────────────────────────────────────────┐
│           Input Validation Layer                │
│  (SQL injection, XSS, path traversal)           │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│          RBAC Permission Layer                  │
│  (Create, Read, Update, Delete, Execute)        │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│           Tenant Isolation Layer                │
│  (tenant_id enforcement, no cross-tenant)       │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│           VM Sandboxing Layer                   │
│  (Removed globals, frozen context)              │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│          Resource Limits Layer                  │
│  (CPU, memory, API calls, output size)          │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│           SSRF Protection Layer                 │
│  (Private IP blocking, DNS validation)          │
└─────────────────────────────────────────────────┘
                      ↓
┌─────────────────────────────────────────────────┐
│             Audit Trail Layer                   │
│  (Execution logs, version history)              │
└─────────────────────────────────────────────────┘
```

## VM Sandboxing

### Removed Dangerous Globals

The VM removes access to all dangerous Node.js/Browser globals that could compromise security:

```go
package jsruntime

// Removed globals to prevent:
// - Network access bypass
// - File system access
// - Process manipulation
// - Code injection
// - Module loading bypass

var removedGlobals = []string{
    // Network-related
    "fetch",
    "XMLHttpRequest",
    "WebSocket",
    "Request",
    "Response",
    "Headers",

    // Node.js-specific
    "process",
    "require",
    "module",
    "exports",
    "__dirname",
    "__filename",
    "global",
    "Buffer",

    // File system
    "fs",
    "path",
    "child_process",

    // Dangerous constructors
    "eval",          // Use vm.preventExtensions
    "Function",      // Use vm.preventExtensions

    // Timers (controlled separately)
    "setInterval",   // Prevent infinite loops
    "setTimeout",    // Replaced with controlled version

    // Import/Export
    "import",
    "importScripts",
    "Worker",
    "SharedWorker",
}

// Allowed safe globals
var allowedGlobals = []string{
    // Standard JavaScript objects
    "Object",
    "Array",
    "String",
    "Number",
    "Boolean",
    "Date",
    "Math",
    "JSON",
    "RegExp",
    "Error",
    "Promise",

    // Utility
    "console",     // Controlled, logs captured
    "parseInt",
    "parseFloat",
    "isNaN",
    "isFinite",
    "decodeURI",
    "encodeURI",
    "decodeURIComponent",
    "encodeURIComponent",

    // Custom SDK namespaces (injected)
    "sdk",         // Our API bindings
    "ctx",         // Execution context
    "events",      // Event publishing
}
```

### VM Initialization

```go
package jsruntime

import (
    "context"
    "fmt"

    "github.com/dop251/goja"
)

type SandboxedVM struct {
    runtime *goja.Runtime
    ctx     context.Context
}

func NewSandboxedVM(ctx context.Context) (*SandboxedVM, error) {
    runtime := goja.New()

    // Remove dangerous globals
    for _, global := range removedGlobals {
        runtime.Set(global, goja.Undefined())
    }

    // Prevent eval and Function constructor
    runtime.Set("eval", goja.Undefined())
    runtime.Set("Function", goja.Undefined())

    // Freeze standard prototypes to prevent modification
    _, err := runtime.RunString(`
        Object.freeze(Object.prototype);
        Object.freeze(Array.prototype);
        Object.freeze(String.prototype);
        Object.freeze(Number.prototype);
        Object.freeze(Boolean.prototype);
        Object.freeze(Date.prototype);
        Object.freeze(Math);
        Object.freeze(JSON);
        Object.freeze(RegExp.prototype);
    `)
    if err != nil {
        return nil, fmt.Errorf("failed to freeze prototypes: %w", err)
    }

    return &SandboxedVM{
        runtime: runtime,
        ctx:     ctx,
    }, nil
}

// InjectSafeGlobals adds controlled SDK bindings
func (vm *SandboxedVM) InjectSafeGlobals(bindings map[string]interface{}) error {
    for name, value := range bindings {
        if err := vm.runtime.Set(name, value); err != nil {
            return fmt.Errorf("failed to inject %s: %w", name, err)
        }

        // Freeze injected objects to prevent tampering
        if _, err := vm.runtime.RunString(fmt.Sprintf("Object.freeze(%s);", name)); err != nil {
            return fmt.Errorf("failed to freeze %s: %w", name, err)
        }
    }
    return nil
}
```

## Resource Limits

### Configuration

```go
package jsruntime

import "time"

// ResourceLimits defines execution constraints
type ResourceLimits struct {
    // CPU time limit (wall clock time)
    MaxExecutionTime time.Duration

    // Memory limit (approximate, based on heap size)
    MaxMemoryBytes int64

    // Maximum number of SDK API calls per execution
    MaxAPICalls int

    // Maximum output size (stdout + result)
    MaxOutputBytes int

    // Maximum number of console.log calls
    MaxConsoleLogs int
}

// DefaultResourceLimits provides safe defaults
var DefaultResourceLimits = ResourceLimits{
    MaxExecutionTime: 5 * time.Second,
    MaxMemoryBytes:   128 * 1024 * 1024, // 128 MB
    MaxAPICalls:      100,
    MaxOutputBytes:   1024 * 1024, // 1 MB
    MaxConsoleLogs:   1000,
}

// GetResourceLimits returns limits for a tenant (configurable per tenant)
func GetResourceLimits(ctx context.Context, tenantID uint) ResourceLimits {
    // TODO: Load from tenant settings table
    // For now, return defaults
    return DefaultResourceLimits
}
```

### Enforcement

```go
package jsruntime

import (
    "context"
    "fmt"
    "runtime"
    "sync/atomic"
    "time"
)

// ExecutionContext tracks resource usage during script execution
type ExecutionContext struct {
    ctx            context.Context
    cancel         context.CancelFunc
    limits         ResourceLimits
    apiCallCount   int32
    consoleLogCount int32
    outputSize     int32
    startMemory    uint64
}

// NewExecutionContext creates a monitored execution context
func NewExecutionContext(parentCtx context.Context, limits ResourceLimits) *ExecutionContext {
    ctx, cancel := context.WithTimeout(parentCtx, limits.MaxExecutionTime)

    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    return &ExecutionContext{
        ctx:         ctx,
        cancel:      cancel,
        limits:      limits,
        startMemory: memStats.Alloc,
    }
}

// CheckAPICallLimit increments and validates API call count
func (ec *ExecutionContext) CheckAPICallLimit() error {
    count := atomic.AddInt32(&ec.apiCallCount, 1)
    if int(count) > ec.limits.MaxAPICalls {
        return fmt.Errorf("exceeded API call limit: %d/%d", count, ec.limits.MaxAPICalls)
    }
    return nil
}

// CheckConsoleLogLimit increments and validates console.log count
func (ec *ExecutionContext) CheckConsoleLogLimit() error {
    count := atomic.AddInt32(&ec.consoleLogCount, 1)
    if int(count) > ec.limits.MaxConsoleLogs {
        return fmt.Errorf("exceeded console.log limit: %d/%d", count, ec.limits.MaxConsoleLogs)
    }
    return nil
}

// CheckOutputSize validates total output size
func (ec *ExecutionContext) CheckOutputSize(additionalBytes int) error {
    newSize := atomic.AddInt32(&ec.outputSize, int32(additionalBytes))
    if int(newSize) > ec.limits.MaxOutputBytes {
        return fmt.Errorf("exceeded output size limit: %d/%d bytes", newSize, ec.limits.MaxOutputBytes)
    }
    return nil
}

// CheckMemoryUsage estimates current memory usage
func (ec *ExecutionContext) CheckMemoryUsage() error {
    var memStats runtime.MemStats
    runtime.ReadMemStats(&memStats)

    currentUsage := memStats.Alloc - ec.startMemory
    if int64(currentUsage) > ec.limits.MaxMemoryBytes {
        return fmt.Errorf("exceeded memory limit: %d/%d bytes", currentUsage, ec.limits.MaxMemoryBytes)
    }
    return nil
}

// Context returns the underlying context (for cancellation)
func (ec *ExecutionContext) Context() context.Context {
    return ec.ctx
}

// Cleanup releases resources
func (ec *ExecutionContext) Cleanup() {
    ec.cancel()
}
```

### Panic Recovery

```go
package jsruntime

import (
    "fmt"
    "runtime/debug"

    "github.com/iota-uz/iota-sdk/pkg/serrors"
)

// ExecuteWithRecovery wraps script execution with panic recovery
func (vm *VM) ExecuteWithRecovery(execCtx *ExecutionContext, source string) (result string, err error) {
    const op = serrors.Op("jsruntime.VM.ExecuteWithRecovery")

    defer func() {
        if r := recover(); r != nil {
            stack := debug.Stack()
            err = serrors.E(op, serrors.KindInternal, fmt.Sprintf("script panic: %v\nStack: %s", r, stack))
        }
    }()

    // Create timeout channel
    done := make(chan struct{})
    var execErr error
    var execResult string

    go func() {
        defer close(done)
        execResult, execErr = vm.execute(execCtx, source)
    }()

    select {
    case <-done:
        return execResult, execErr
    case <-execCtx.Context().Done():
        return "", serrors.E(op, serrors.KindTimeout, "script execution timeout")
    }
}
```

## SSRF Protection

### Private IP Detection

```go
package jsruntime

import (
    "fmt"
    "net"
    "net/url"
    "strings"
)

// isPrivateIP checks if IP is in private/local range
func isPrivateIP(ip net.IP) bool {
    if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
        return true
    }

    privateRanges := []string{
        "10.0.0.0/8",       // RFC1918
        "172.16.0.0/12",    // RFC1918
        "192.168.0.0/16",   // RFC1918
        "169.254.0.0/16",   // RFC3927 link-local
        "127.0.0.0/8",      // Loopback
        "::1/128",          // IPv6 loopback
        "fc00::/7",         // IPv6 private
        "fe80::/10",        // IPv6 link-local
    }

    for _, cidr := range privateRanges {
        _, ipnet, _ := net.ParseCIDR(cidr)
        if ipnet.Contains(ip) {
            return true
        }
    }

    return false
}

// ValidateURL checks URL for SSRF vulnerabilities
func ValidateURL(targetURL string, allowedHosts []string) error {
    parsed, err := url.Parse(targetURL)
    if err != nil {
        return fmt.Errorf("invalid URL: %w", err)
    }

    // Only allow HTTP/HTTPS
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("unsupported scheme: %s (only http/https allowed)", parsed.Scheme)
    }

    hostname := parsed.Hostname()

    // Check against allowed hosts whitelist (if provided)
    if len(allowedHosts) > 0 {
        allowed := false
        for _, host := range allowedHosts {
            if strings.EqualFold(hostname, host) || strings.HasSuffix(hostname, "."+host) {
                allowed = true
                break
            }
        }
        if !allowed {
            return fmt.Errorf("host not in whitelist: %s", hostname)
        }
    }

    // Resolve hostname to IP
    ips, err := net.LookupIP(hostname)
    if err != nil {
        return fmt.Errorf("failed to resolve hostname: %w", err)
    }

    // Block if any resolved IP is private
    for _, ip := range ips {
        if isPrivateIP(ip) {
            return fmt.Errorf("private IP address blocked: %s resolves to %s", hostname, ip)
        }
    }

    return nil
}
```

### HTTP Client with SSRF Protection

```go
package jsruntime

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "time"
)

// SSRFProtectedTransport wraps http.Transport with IP validation
type SSRFProtectedTransport struct {
    base         *http.Transport
    allowedHosts []string
}

func NewSSRFProtectedTransport(allowedHosts []string) *SSRFProtectedTransport {
    return &SSRFProtectedTransport{
        base: &http.Transport{
            DialContext: (&net.Dialer{
                Timeout:   10 * time.Second,
                KeepAlive: 30 * time.Second,
            }).DialContext,
            MaxIdleConns:          100,
            IdleConnTimeout:       90 * time.Second,
            TLSHandshakeTimeout:   10 * time.Second,
            ExpectContinueTimeout: 1 * time.Second,
        },
        allowedHosts: allowedHosts,
    }
}

func (t *SSRFProtectedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // Validate URL before allowing request
    if err := ValidateURL(req.URL.String(), t.allowedHosts); err != nil {
        return nil, fmt.Errorf("SSRF protection blocked request: %w", err)
    }

    // Override DialContext to re-validate IP at connection time (prevent DNS rebinding)
    transport := t.base.Clone()
    transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
        host, port, err := net.SplitHostPort(addr)
        if err != nil {
            return nil, err
        }

        // Resolve and validate IP again
        ips, err := net.LookupIP(host)
        if err != nil {
            return nil, err
        }

        for _, ip := range ips {
            if isPrivateIP(ip) {
                return nil, fmt.Errorf("DNS rebinding attack prevented: %s resolved to private IP %s", host, ip)
            }
        }

        // Proceed with original dialer
        return (&net.Dialer{
            Timeout:   10 * time.Second,
            KeepAlive: 30 * time.Second,
        }).DialContext(ctx, network, addr)
    }

    return transport.RoundTrip(req)
}

// GetSSRFProtectedClient returns HTTP client with SSRF protections
func GetSSRFProtectedClient(allowedHosts []string) *http.Client {
    return &http.Client{
        Transport: NewSSRFProtectedTransport(allowedHosts),
        Timeout:   30 * time.Second,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            if len(via) >= 10 {
                return fmt.Errorf("too many redirects")
            }
            // Re-validate each redirect target
            return ValidateURL(req.URL.String(), allowedHosts)
        },
    }
}
```

## Tenant Isolation

### Database Query Enforcement

```go
package jsruntime

import (
    "context"
    "fmt"
    "strings"

    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/serrors"
)

// EnforceTenantIsolation validates and rewrites SQL to include tenant_id
func EnforceTenantIsolation(ctx context.Context, sql string, params []interface{}) (string, []interface{}, error) {
    const op = serrors.Op("jsruntime.EnforceTenantIsolation")

    tenantID := composables.UseTenantID(ctx)
    if tenantID == 0 {
        return "", nil, serrors.E(op, serrors.KindPermission, "missing tenant context")
    }

    // Parse SQL to detect table references
    lowerSQL := strings.ToLower(sql)

    // Block dangerous operations
    if strings.Contains(lowerSQL, "drop ") ||
        strings.Contains(lowerSQL, "truncate ") ||
        strings.Contains(lowerSQL, "alter ") ||
        strings.Contains(lowerSQL, "create ") {
        return "", nil, serrors.E(op, serrors.KindValidation, "DDL statements not allowed")
    }

    // Check if tenant_id is already in WHERE clause
    if !strings.Contains(lowerSQL, "tenant_id") {
        return "", nil, serrors.E(op, serrors.KindValidation, "query must include tenant_id filter")
    }

    // Append tenant_id to params (script must use placeholders correctly)
    // This is a basic check - production should use SQL parser
    return sql, params, nil
}

// ExecuteQuery wraps database query with tenant isolation
func (api *DBAPI) ExecuteQuery(ctx context.Context, sql string, params []interface{}) ([]map[string]interface{}, error) {
    const op = serrors.Op("jsruntime.DBAPI.ExecuteQuery")

    // Enforce tenant isolation
    validatedSQL, validatedParams, err := EnforceTenantIsolation(ctx, sql, params)
    if err != nil {
        return nil, serrors.E(op, err)
    }

    // Proceed with query execution (implementation in 07-api-bindings.md)
    return api.query(ctx, validatedSQL, validatedParams)
}
```

### Cache Key Prefixing

```go
package jsruntime

import (
    "context"
    "fmt"

    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// TenantCacheKey prefixes cache key with tenant_id
func TenantCacheKey(ctx context.Context, key string) string {
    tenantID := composables.UseTenantID(ctx)
    return fmt.Sprintf("tenant:%d:%s", tenantID, key)
}

// CacheAPI automatically prefixes keys
type CacheAPI struct {
    cache cache.Cache
}

func (api *CacheAPI) Get(ctx context.Context, key string) (interface{}, error) {
    prefixedKey := TenantCacheKey(ctx, key)
    return api.cache.Get(ctx, prefixedKey)
}

func (api *CacheAPI) Set(ctx context.Context, key string, value interface{}, ttl int) error {
    prefixedKey := TenantCacheKey(ctx, key)
    return api.cache.Set(ctx, prefixedKey, value, time.Duration(ttl)*time.Second)
}

func (api *CacheAPI) Delete(ctx context.Context, key string) error {
    prefixedKey := TenantCacheKey(ctx, key)
    return api.cache.Delete(ctx, prefixedKey)
}
```

### Event Publishing Scope

```go
package jsruntime

import (
    "context"

    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/yourorg/yourapp/pkg/event"
)

// PublishEvent ensures events are scoped to tenant
func PublishEvent(ctx context.Context, eventType string, payload map[string]interface{}) error {
    tenantID := composables.UseTenantID(ctx)
    orgID := composables.GetOrgID(ctx)
    userID := composables.GetUserID(ctx)

    // Add tenant metadata to event
    enrichedPayload := map[string]interface{}{
        "tenant_id": tenantID,
        "org_id":    orgID,
        "user_id":   userID,
        "data":      payload,
    }

    return event.Publish(ctx, eventType, enrichedPayload)
}
```

## RBAC Permissions

### Permission Definitions

```go
package permissions

import "github.com/iota-uz/iota-sdk/pkg/permission"

const (
    ResourceScript permission.Resource = "Script"
    ActionExecute  permission.Action   = "Execute"
)

var (
    // Script management permissions
    ScriptCreate = permission.MustCreate(
        "Script.Create",
        ResourceScript,
        permission.ActionCreate,
        permission.ModifierAll,
    )

    ScriptRead = permission.MustCreate(
        "Script.Read",
        ResourceScript,
        permission.ActionRead,
        permission.ModifierAll,
    )

    ScriptUpdate = permission.MustCreate(
        "Script.Update",
        ResourceScript,
        permission.ActionUpdate,
        permission.ModifierAll,
    )

    ScriptDelete = permission.MustCreate(
        "Script.Delete",
        ResourceScript,
        permission.ActionDelete,
        permission.ModifierAll,
    )

    // Execution permission (separate from management)
    ScriptExecute = permission.MustCreate(
        "Script.Execute",
        ResourceScript,
        ActionExecute,
        permission.ModifierAll,
    )

    // Read-only execution history
    ScriptExecutionRead = permission.MustCreate(
        "Script.Execution.Read",
        ResourceScript,
        permission.ActionRead,
        permission.ModifierAll,
    )
)

// AllScriptPermissions for admin roles
func AllScriptPermissions() []permission.Permission {
    return []permission.Permission{
        ScriptCreate,
        ScriptRead,
        ScriptUpdate,
        ScriptDelete,
        ScriptExecute,
        ScriptExecutionRead,
    }
}
```

### Permission Checks in Service Layer

```go
package services

import (
    "context"

    sdkcomposables "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/serrors"
    "github.com/yourorg/yourapp/modules/scripts/permissions"
)

func (s *ScriptService) Create(ctx context.Context, params CreateParams) (script.Script, error) {
    const op = serrors.Op("services.ScriptService.Create")

    // Check permission
    if !sdkcomposables.CanUser(ctx, permissions.ScriptCreate) {
        return nil, serrors.E(op, serrors.KindPermission, "user lacks Script.Create permission")
    }

    // Proceed with creation...
    return s.repo.Create(ctx, params)
}

func (s *ExecutionService) Execute(ctx context.Context, scriptID uint, input map[string]interface{}) (execution.Execution, error) {
    const op = serrors.Op("services.ExecutionService.Execute")

    // Check execution permission
    if !sdkcomposables.CanUser(ctx, permissions.ScriptExecute) {
        return nil, serrors.E(op, serrors.KindPermission, "user lacks Script.Execute permission")
    }

    // Proceed with execution...
    return s.executeScript(ctx, scriptID, input)
}
```

## Input Validation

### Script Name Validation

```go
package validation

import (
    "fmt"
    "regexp"
)

var scriptNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,100}$`)

func ValidateScriptName(name string) error {
    if !scriptNameRegex.MatchString(name) {
        return fmt.Errorf("script name must be 3-100 alphanumeric characters, underscores, or hyphens")
    }
    return nil
}
```

### SQL Injection Prevention

```go
package jsruntime

import (
    "fmt"
    "strings"
)

// ValidateSQLParams ensures only parameterized queries
func ValidateSQLParams(sql string, params []interface{}) error {
    // Count placeholders ($1, $2, etc.)
    placeholderCount := 0
    for i := 1; i <= len(params)+1; i++ {
        if strings.Contains(sql, fmt.Sprintf("$%d", i)) {
            placeholderCount++
        }
    }

    if placeholderCount != len(params) {
        return fmt.Errorf("parameter count mismatch: found %d placeholders, got %d params", placeholderCount, len(params))
    }

    // Block SQL keywords in string literals (basic check)
    lowerSQL := strings.ToLower(sql)
    for _, param := range params {
        if str, ok := param.(string); ok {
            lowerParam := strings.ToLower(str)
            if strings.Contains(lowerParam, "drop ") ||
                strings.Contains(lowerParam, "'; ") ||
                strings.Contains(lowerParam, "-- ") {
                return fmt.Errorf("suspicious SQL in parameter: %s", str)
            }
        }
    }

    return nil
}
```

### XSS Prevention in Output

```go
package jsruntime

import (
    "html"
    "strings"
)

// SanitizeOutput escapes HTML to prevent XSS
func SanitizeOutput(output string) string {
    return html.EscapeString(output)
}

// SanitizeJSON ensures JSON output doesn't contain script tags
func SanitizeJSON(jsonStr string) string {
    // Replace script tags even in JSON strings
    return strings.ReplaceAll(
        strings.ReplaceAll(jsonStr, "<script", "&lt;script"),
        "</script", "&lt;/script",
    )
}
```

### Path Traversal Prevention

```go
package jsruntime

import (
    "fmt"
    "path/filepath"
    "strings"
)

// ValidatePath prevents path traversal attacks
func ValidatePath(path string) error {
    // Normalize path
    clean := filepath.Clean(path)

    // Block parent directory references
    if strings.Contains(clean, "..") {
        return fmt.Errorf("path traversal detected: %s", path)
    }

    // Block absolute paths
    if filepath.IsAbs(clean) {
        return fmt.Errorf("absolute paths not allowed: %s", path)
    }

    return nil
}
```

## Audit Trail

### Execution Logging

```go
package services

import (
    "context"
    "encoding/json"

    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/yourorg/yourapp/modules/scripts/domain/execution"
)

// LogExecution creates audit record for script execution
func (s *ExecutionService) LogExecution(ctx context.Context, exec execution.Execution) error {
    tenantID := composables.UseTenantID(ctx)
    userID := composables.GetUserID(ctx)
    orgID := composables.GetOrgID(ctx)

    auditData := map[string]interface{}{
        "tenant_id":    tenantID,
        "user_id":      userID,
        "org_id":       orgID,
        "script_id":    exec.GetScriptID(),
        "execution_id": exec.GetID(),
        "status":       exec.GetStatus(),
        "started_at":   exec.GetStartedAt(),
        "completed_at": exec.GetCompletedAt(),
        "error":        exec.GetErrorMessage(),
    }

    auditJSON, _ := json.Marshal(auditData)
    s.logger.Info("script_execution", "data", string(auditJSON))

    return nil
}
```

### Version History Tracking

```go
package services

import (
    "context"

    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/yourorg/yourapp/modules/scripts/domain/script"
    "github.com/yourorg/yourapp/modules/scripts/domain/version"
)

// CreateVersion archives script source on update
func (s *ScriptService) CreateVersion(ctx context.Context, script script.Script) error {
    userID := composables.GetUserID(ctx)

    newVersion := version.New(
        version.WithScriptID(script.GetID()),
        version.WithVersion(script.GetVersion()),
        version.WithSource(script.GetSource()),
        version.WithCreatedBy(userID),
    )

    return s.versionRepo.Create(ctx, newVersion)
}
```

### CRUD Operation Logging

```go
package services

import (
    "context"
    "encoding/json"

    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// AuditAction logs CRUD operations
func (s *ScriptService) AuditAction(ctx context.Context, action string, scriptID uint, changes map[string]interface{}) {
    tenantID := composables.UseTenantID(ctx)
    userID := composables.GetUserID(ctx)

    auditData := map[string]interface{}{
        "tenant_id": tenantID,
        "user_id":   userID,
        "action":    action,
        "script_id": scriptID,
        "changes":   changes,
    }

    auditJSON, _ := json.Marshal(auditData)
    s.logger.Info("script_audit", "data", string(auditJSON))
}

func (s *ScriptService) Update(ctx context.Context, id uint, params UpdateParams) (script.Script, error) {
    // ... permission checks ...

    // Fetch current state
    current, _ := s.repo.FindByID(ctx, id)

    // Update script
    updated, err := s.repo.Update(ctx, id, params)
    if err != nil {
        return nil, err
    }

    // Log changes
    changes := map[string]interface{}{
        "before": map[string]interface{}{
            "name":    current.GetName(),
            "enabled": current.IsEnabled(),
        },
        "after": map[string]interface{}{
            "name":    updated.GetName(),
            "enabled": updated.IsEnabled(),
        },
    }
    s.AuditAction(ctx, "update", id, changes)

    return updated, nil
}
```

## Security Testing Checklist

### Unit Tests

```go
package jsruntime_test

import (
    "testing"

    "github.com/yourorg/yourapp/pkg/jsruntime"
)

func TestVMSandboxing(t *testing.T) {
    tests := []struct {
        name       string
        script     string
        shouldFail bool
    }{
        {
            name:       "eval blocked",
            script:     `eval("console.log('hacked')")`,
            shouldFail: true,
        },
        {
            name:       "Function constructor blocked",
            script:     `new Function("return 1")()`,
            shouldFail: true,
        },
        {
            name:       "require blocked",
            script:     `require('fs')`,
            shouldFail: true,
        },
        {
            name:       "process blocked",
            script:     `process.exit(1)`,
            shouldFail: true,
        },
        {
            name:       "fetch blocked",
            script:     `fetch('http://example.com')`,
            shouldFail: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}

func TestResourceLimits(t *testing.T) {
    tests := []struct {
        name        string
        script      string
        expectError string
    }{
        {
            name:        "timeout exceeded",
            script:      `while(true) {}`,
            expectError: "timeout",
        },
        {
            name: "API call limit exceeded",
            script: `
                for (let i = 0; i < 200; i++) {
                    sdk.db.query("SELECT 1");
                }
            `,
            expectError: "API call limit",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}

func TestSSRFProtection(t *testing.T) {
    tests := []struct {
        name        string
        url         string
        expectError bool
    }{
        {
            name:        "localhost blocked",
            url:         "http://localhost/admin",
            expectError: true,
        },
        {
            name:        "127.0.0.1 blocked",
            url:         "http://127.0.0.1/internal",
            expectError: true,
        },
        {
            name:        "10.0.0.0 blocked",
            url:         "http://10.0.0.1/secret",
            expectError: true,
        },
        {
            name:        "192.168.0.0 blocked",
            url:         "http://192.168.1.1/router",
            expectError: true,
        },
        {
            name:        "public IP allowed",
            url:         "https://api.example.com/data",
            expectError: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := jsruntime.ValidateURL(tt.url, nil)
            if (err != nil) != tt.expectError {
                t.Errorf("expected error=%v, got error=%v", tt.expectError, err)
            }
        })
    }
}
```

### Integration Tests

```go
package services_test

import (
    "context"
    "testing"

    "github.com/yourorg/yourapp/modules/scripts/services"
    "github.com/yourorg/yourapp/pkg/itf"
)

func TestTenantIsolation(t *testing.T) {
    t.Parallel()

    env := itf.Setup(t, itf.WithPermissions(permissions.AllScriptPermissions()...))
    scriptSvc := itf.GetService[*services.ScriptService](env)

    // Create script in tenant A
    ctxA := itf.NewContextWithTenant(t, env, 1)
    scriptA, _ := scriptSvc.Create(ctxA, services.CreateParams{
        Name:   "Tenant A Script",
        Source: "console.log('A')",
    })

    // Attempt to access from tenant B
    ctxB := itf.NewContextWithTenant(t, env, 2)
    _, err := scriptSvc.FindByID(ctxB, scriptA.GetID())

    if err == nil {
        t.Fatal("expected error when accessing cross-tenant script")
    }
}
```

## Summary

This security model provides:

1. **VM Sandboxing**: Removed dangerous globals, frozen prototypes, prevented eval/Function
2. **Resource Limits**: CPU time, memory, API calls, output size enforcement
3. **SSRF Protection**: Private IP blocking, DNS validation, redirect checking
4. **Tenant Isolation**: Database queries, cache keys, event scoping
5. **RBAC Permissions**: Fine-grained access control for CRUD and execution
6. **Input Validation**: SQL injection, XSS, path traversal prevention
7. **Audit Trail**: Execution logs, version history, CRUD operation tracking

All layers work together to ensure secure, isolated, and auditable script execution in a multi-tenant environment.
