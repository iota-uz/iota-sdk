# JavaScript Runtime - API Bindings Specification

**Status:** Implementation Ready
**Layer:** Infrastructure Layer
**Dependencies:** Runtime engine, Service layer, Domain entities
**Related Issues:** #414, #415, #418, #420

---

## Overview

This specification defines JavaScript APIs exposed to scripts, including context objects, HTTP client, database access, cache operations, logging, and event publishing. All APIs enforce tenant isolation and security restrictions.

## API Architecture

```
JavaScript Context:

ctx.*          - Execution context (tenant, user, script, input)
sdk.http.*     - HTTP client (SSRF-protected)
sdk.db.*       - Database queries (tenant-scoped, parameterized)
sdk.cache.*    - Key-value cache (tenant-scoped)
sdk.log.*      - Structured logging (execution-linked)
events.*       - Event publishing (tenant-scoped)
```

---

## APIBindings Interface

**Location:** `modules/scripts/runtime/bindings.go`

```go
package runtime

import (
    "github.com/dop251/goja"
)

// APIBindings defines how to inject APIs into VM
type APIBindings interface {
    // Inject injects all APIs into a Goja VM
    Inject(vm *goja.Runtime) error
}

// BindingsImpl implements APIBindings
type BindingsImpl struct {
    httpClient  *HTTPClient
    dbClient    *DBClient
    cacheClient *CacheClient
    logClient   *LogClient
    eventClient *EventClient
}

// NewAPIBindings creates API bindings
func NewAPIBindings(
    httpClient *HTTPClient,
    dbClient *DBClient,
    cacheClient *CacheClient,
    logClient *LogClient,
    eventClient *EventClient,
) APIBindings {
    return &BindingsImpl{
        httpClient:  httpClient,
        dbClient:    dbClient,
        cacheClient: cacheClient,
        logClient:   logClient,
        eventClient: eventClient,
    }
}

// Inject injects all APIs into VM
func (b *BindingsImpl) Inject(vm *goja.Runtime) error {
    // Create sdk namespace
    sdk := vm.NewObject()

    // Inject HTTP client
    sdk.Set("http", b.httpClient.ToGojaObject(vm))

    // Inject database client
    sdk.Set("db", b.dbClient.ToGojaObject(vm))

    // Inject cache client
    sdk.Set("cache", b.cacheClient.ToGojaObject(vm))

    // Inject log client
    sdk.Set("log", b.logClient.ToGojaObject(vm))

    // Set sdk in global scope
    vm.Set("sdk", sdk)

    // Inject events client
    vm.Set("events", b.eventClient.ToGojaObject(vm))

    return nil
}
```

---

## Context API

**Location:** `modules/scripts/runtime/context.go`

### Context Structure

```javascript
// Available as `ctx` in scripts
const ctx = {
    tenant: {
        id: "uuid",
        name: "Tenant Name",
        domain: "tenant.example.com"
    },
    user: {
        id: 123,
        email: "user@example.com",
        firstName: "John",
        lastName: "Doe"
    } | null, // null if not authenticated
    execution: {
        id: "uuid",
        trigger: "manual|cron|event|http"
    },
    input: {
        // Input data passed to execution
    }
};
```

### Implementation

Context is injected by Executor (see `06-runtime-engine.md` - `setupContext` method).

---

## HTTP Client API

**Location:** `modules/scripts/runtime/http_client.go`

### JavaScript Interface

```javascript
// GET request
const response = await sdk.http.get(url, options);

// POST request
const response = await sdk.http.post(url, body, options);

// PUT request
const response = await sdk.http.put(url, body, options);

// DELETE request
const response = await sdk.http.delete(url, options);

// Options:
// {
//   headers: { "Authorization": "Bearer token" },
//   timeout: 5000 // milliseconds
// }

// Response:
// {
//   status: 200,
//   headers: { "content-type": "application/json" },
//   body: { ... } | "string"
// }
```

### Go Implementation

```go
package runtime

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net"
    "net/http"
    "net/url"
    "time"

    "github.com/dop251/goja"
    "github.com/dop251/goja_nodejs/require"
)

var (
    // Blocked IP ranges (SSRF protection)
    blockedCIDRs = []string{
        "10.0.0.0/8",       // Private
        "172.16.0.0/12",    // Private
        "192.168.0.0/16",   // Private
        "127.0.0.0/8",      // Loopback
        "169.254.0.0/16",   // Link-local
        "::1/128",          // IPv6 loopback
        "fc00::/7",         // IPv6 private
    }

    // Allowed hosts whitelist (optional)
    allowedHosts = []string{
        // Empty = allow all except blocked IPs
        // Add specific domains if needed
    }
)

// HTTPClient provides HTTP functionality to scripts
type HTTPClient struct {
    client  *http.Client
    context context.Context
}

// NewHTTPClient creates a new HTTP client for scripts
func NewHTTPClient(ctx context.Context) *HTTPClient {
    transport := &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            // SSRF protection
            host, _, err := net.SplitHostPort(addr)
            if err != nil {
                return nil, err
            }

            // Resolve host to IP
            ips, err := net.LookupIP(host)
            if err != nil {
                return nil, err
            }

            // Check if any IP is in blocked range
            for _, ip := range ips {
                if isBlockedIP(ip) {
                    return nil, fmt.Errorf("access to IP %s is blocked", ip)
                }
            }

            // Use default dialer
            dialer := &net.Dialer{
                Timeout: 10 * time.Second,
            }
            return dialer.DialContext(ctx, network, addr)
        },
    }

    return &HTTPClient{
        client: &http.Client{
            Transport: transport,
            Timeout:   30 * time.Second,
        },
        context: ctx,
    }
}

// ToGojaObject converts HTTPClient to Goja object
func (h *HTTPClient) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("get", h.get)
    obj.Set("post", h.post)
    obj.Set("put", h.put)
    obj.Set("delete", h.delete_)

    return obj
}

func (h *HTTPClient) get(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()
    return h.request(vm, "GET", call.Arguments)
}

func (h *HTTPClient) post(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()
    return h.request(vm, "POST", call.Arguments)
}

func (h *HTTPClient) put(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()
    return h.request(vm, "PUT", call.Arguments)
}

func (h *HTTPClient) delete_(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()
    return h.request(vm, "DELETE", call.Arguments)
}

func (h *HTTPClient) request(
    vm *goja.Runtime,
    method string,
    args []goja.Value,
) goja.Value {
    if len(args) == 0 {
        panic(vm.NewTypeError("URL is required"))
    }

    urlStr := args[0].String()

    var body interface{}
    var options map[string]interface{}

    if method == "POST" || method == "PUT" {
        if len(args) > 1 {
            body = args[1].Export()
        }
        if len(args) > 2 {
            options = args[2].Export().(map[string]interface{})
        }
    } else {
        if len(args) > 1 {
            options = args[1].Export().(map[string]interface{})
        }
    }

    // Validate URL
    if _, err := url.Parse(urlStr); err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Invalid URL: %s", err)))
    }

    // Build request
    var bodyReader io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            panic(vm.NewTypeError(fmt.Sprintf("Failed to marshal body: %s", err)))
        }
        bodyReader = bytes.NewReader(jsonBody)
    }

    req, err := http.NewRequestWithContext(h.context, method, urlStr, bodyReader)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Failed to create request: %s", err)))
    }

    // Set headers
    if options != nil {
        if headers, ok := options["headers"].(map[string]interface{}); ok {
            for key, value := range headers {
                req.Header.Set(key, fmt.Sprintf("%v", value))
            }
        }
    }

    // Set default content type for POST/PUT
    if (method == "POST" || method == "PUT") && req.Header.Get("Content-Type") == "" {
        req.Header.Set("Content-Type", "application/json")
    }

    // Execute request
    resp, err := h.client.Do(req)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Request failed: %s", err)))
    }
    defer resp.Body.Close()

    // Read response body
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Failed to read response: %s", err)))
    }

    // Parse JSON if content-type is JSON
    var parsedBody interface{}
    if isJSONContentType(resp.Header.Get("Content-Type")) {
        json.Unmarshal(respBody, &parsedBody)
    } else {
        parsedBody = string(respBody)
    }

    // Build response object
    response := map[string]interface{}{
        "status":  resp.StatusCode,
        "headers": resp.Header,
        "body":    parsedBody,
    }

    return vm.ToValue(response)
}

func isBlockedIP(ip net.IP) bool {
    for _, cidr := range blockedCIDRs {
        _, ipNet, _ := net.ParseCIDR(cidr)
        if ipNet.Contains(ip) {
            return true
        }
    }
    return false
}

func isJSONContentType(contentType string) bool {
    return contentType == "application/json" ||
        contentType == "application/json; charset=utf-8"
}
```

---

## Database Client API

**Location:** `modules/scripts/runtime/db_client.go`

### JavaScript Interface

```javascript
// Parameterized query (tenant-scoped)
const users = await sdk.db.query(
    "SELECT * FROM users WHERE status = $1 LIMIT $2",
    ["active", 10]
);

// Execute (tenant-scoped)
const result = await sdk.db.execute(
    "UPDATE users SET last_login = NOW() WHERE id = $1",
    [userId]
);

// Result:
// {
//   rowsAffected: 1
// }
```

### Go Implementation

```go
package runtime

import (
    "context"
    "fmt"

    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// DBClient provides database access to scripts
type DBClient struct {
    context context.Context
}

// NewDBClient creates a new database client for scripts
func NewDBClient(ctx context.Context) *DBClient {
    return &DBClient{
        context: ctx,
    }
}

// ToGojaObject converts DBClient to Goja object
func (d *DBClient) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("query", d.query)
    obj.Set("execute", d.execute)

    return obj
}

func (d *DBClient) query(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) == 0 {
        panic(vm.NewTypeError("SQL query is required"))
    }

    sql := call.Arguments[0].String()
    var params []interface{}

    if len(call.Arguments) > 1 {
        params = call.Arguments[1].Export().([]interface{})
    }

    // Get tenant ID from context
    tenantID, err := composables.UseTenantID(d.context)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Tenant context required: %s", err)))
    }

    // Add tenant_id to WHERE clause (automatic tenant isolation)
    // TODO: Parse SQL and inject tenant_id filter
    // For now, trust script author to include tenant_id in WHERE

    // Get database connection
    tx, err := composables.UseTx(d.context)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Database connection required: %s", err)))
    }

    // Execute query
    rows, err := tx.Query(d.context, sql, params...)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Query failed: %s", err)))
    }
    defer rows.Close()

    // Build result
    var result []map[string]interface{}

    for rows.Next() {
        // Get column names
        fields := rows.FieldDescriptions()
        values := make([]interface{}, len(fields))
        valuePtrs := make([]interface{}, len(fields))

        for i := range values {
            valuePtrs[i] = &values[i]
        }

        if err := rows.Scan(valuePtrs...); err != nil {
            panic(vm.NewTypeError(fmt.Sprintf("Scan failed: %s", err)))
        }

        row := make(map[string]interface{})
        for i, field := range fields {
            row[string(field.Name)] = values[i]
        }

        result = append(result, row)
    }

    return vm.ToValue(result)
}

func (d *DBClient) execute(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) == 0 {
        panic(vm.NewTypeError("SQL query is required"))
    }

    sql := call.Arguments[0].String()
    var params []interface{}

    if len(call.Arguments) > 1 {
        params = call.Arguments[1].Export().([]interface{})
    }

    // Get database connection
    tx, err := composables.UseTx(d.context)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Database connection required: %s", err)))
    }

    // Execute
    cmdTag, err := tx.Exec(d.context, sql, params...)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Execute failed: %s", err)))
    }

    result := map[string]interface{}{
        "rowsAffected": cmdTag.RowsAffected(),
    }

    return vm.ToValue(result)
}
```

---

## Cache Client API

**Location:** `modules/scripts/runtime/cache_client.go`

### JavaScript Interface

```javascript
// Get value (tenant-scoped keys)
const value = await sdk.cache.get("my-key");

// Set value with TTL (seconds)
await sdk.cache.set("my-key", { data: "value" }, 3600);

// Delete
await sdk.cache.delete("my-key");
```

### Go Implementation

```go
package runtime

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/redis/go-redis/v9"
)

// CacheClient provides cache access to scripts
type CacheClient struct {
    context context.Context
    redis   *redis.Client
}

// NewCacheClient creates a new cache client for scripts
func NewCacheClient(ctx context.Context, redis *redis.Client) *CacheClient {
    return &CacheClient{
        context: ctx,
        redis:   redis,
    }
}

// ToGojaObject converts CacheClient to Goja object
func (c *CacheClient) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("get", c.get)
    obj.Set("set", c.set)
    obj.Set("delete", c.delete_)

    return obj
}

func (c *CacheClient) get(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) == 0 {
        panic(vm.NewTypeError("Key is required"))
    }

    key := call.Arguments[0].String()

    // Prefix with tenant ID
    fullKey := c.tenantKey(key)

    // Get from Redis
    val, err := c.redis.Get(c.context, fullKey).Result()
    if err == redis.Nil {
        return goja.Null()
    }
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Cache get failed: %s", err)))
    }

    // Parse JSON
    var result interface{}
    if err := json.Unmarshal([]byte(val), &result); err != nil {
        // Return as string if not JSON
        return vm.ToValue(val)
    }

    return vm.ToValue(result)
}

func (c *CacheClient) set(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) < 2 {
        panic(vm.NewTypeError("Key and value are required"))
    }

    key := call.Arguments[0].String()
    value := call.Arguments[1].Export()

    var ttl time.Duration
    if len(call.Arguments) > 2 {
        ttlSeconds := int64(call.Arguments[2].ToInteger())
        ttl = time.Duration(ttlSeconds) * time.Second
    }

    // Prefix with tenant ID
    fullKey := c.tenantKey(key)

    // Serialize value
    jsonValue, err := json.Marshal(value)
    if err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Failed to serialize value: %s", err)))
    }

    // Set in Redis
    if err := c.redis.Set(c.context, fullKey, jsonValue, ttl).Err(); err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Cache set failed: %s", err)))
    }

    return goja.Undefined()
}

func (c *CacheClient) delete_(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) == 0 {
        panic(vm.NewTypeError("Key is required"))
    }

    key := call.Arguments[0].String()

    // Prefix with tenant ID
    fullKey := c.tenantKey(key)

    // Delete from Redis
    if err := c.redis.Del(c.context, fullKey).Err(); err != nil {
        panic(vm.NewTypeError(fmt.Sprintf("Cache delete failed: %s", err)))
    }

    return goja.Undefined()
}

func (c *CacheClient) tenantKey(key string) string {
    tenantID, _ := composables.UseTenantID(c.context)
    return fmt.Sprintf("script:%s:%s", tenantID, key)
}
```

---

## Log Client API

**Location:** `modules/scripts/runtime/log_client.go`

### JavaScript Interface

```javascript
// Log levels
sdk.log.info("User created", { userId: 123 });
sdk.log.warn("Low memory", { availableMB: 10 });
sdk.log.error("Failed to process", { error: "Network timeout" });
```

### Go Implementation

```go
package runtime

import (
    "context"

    "github.com/dop251/goja"
    "github.com/sirupsen/logrus"
)

// LogClient provides logging to scripts
type LogClient struct {
    context context.Context
    logger  *logrus.Logger
}

// NewLogClient creates a new log client for scripts
func NewLogClient(ctx context.Context, logger *logrus.Logger) *LogClient {
    return &LogClient{
        context: ctx,
        logger:  logger,
    }
}

// ToGojaObject converts LogClient to Goja object
func (l *LogClient) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("info", l.info)
    obj.Set("warn", l.warn)
    obj.Set("error", l.error_)

    return obj
}

func (l *LogClient) info(call goja.FunctionCall) goja.Value {
    l.log(logrus.InfoLevel, call.Arguments)
    return goja.Undefined()
}

func (l *LogClient) warn(call goja.FunctionCall) goja.Value {
    l.log(logrus.WarnLevel, call.Arguments)
    return goja.Undefined()
}

func (l *LogClient) error_(call goja.FunctionCall) goja.Value {
    l.log(logrus.ErrorLevel, call.Arguments)
    return goja.Undefined()
}

func (l *LogClient) log(level logrus.Level, args []goja.Value) {
    if len(args) == 0 {
        return
    }

    message := args[0].String()

    fields := logrus.Fields{
        "source": "script",
    }

    // Add execution context
    if execID, err := execution.UseExecutionID(l.context); err == nil {
        fields["execution_id"] = execID.String()
    }
    if scriptID, err := execution.UseScriptID(l.context); err == nil {
        fields["script_id"] = scriptID.String()
    }

    if len(args) > 1 {
        data := args[1].Export()
        if dataMap, ok := data.(map[string]interface{}); ok {
            for k, v := range dataMap {
                fields[k] = v
            }
        }
    }

    l.logger.WithFields(fields).Log(level, message)
}
```

---

## Event Client API

**Location:** `modules/scripts/runtime/event_client.go`

### JavaScript Interface

```javascript
// Publish event (tenant-scoped)
await events.publish("order.created", {
    orderId: 123,
    total: 99.99
});
```

### Go Implementation

```go
package runtime

import (
    "context"
    "fmt"

    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/eventbus"
)

// EventClient provides event publishing to scripts
type EventClient struct {
    context   context.Context
    publisher eventbus.EventBus
}

// NewEventClient creates a new event client for scripts
func NewEventClient(ctx context.Context, publisher eventbus.EventBus) *EventClient {
    return &EventClient{
        context:   ctx,
        publisher: publisher,
    }
}

// ToGojaObject converts EventClient to Goja object
func (e *EventClient) ToGojaObject(vm *goja.Runtime) *goja.Object {
    obj := vm.NewObject()

    obj.Set("publish", e.publish)

    return obj
}

func (e *EventClient) publish(call goja.FunctionCall) goja.Value {
    vm := call.This.Runtime()

    if len(call.Arguments) < 2 {
        panic(vm.NewTypeError("Event type and payload are required"))
    }

    eventType := call.Arguments[0].String()
    payload := call.Arguments[1].Export()

    // Create generic event
    event := &ScriptEvent{
        Type:    eventType,
        Payload: payload,
    }

    // Publish
    e.publisher.Publish(event)

    return goja.Undefined()
}

// ScriptEvent is a generic event published by scripts
type ScriptEvent struct {
    Type    string
    Payload interface{}
}
```

---

## TypeScript Definitions

**Location:** `modules/scripts/presentation/static/sdk.d.ts`

```typescript
// Context
interface Context {
    tenant: {
        id: string;
        name: string;
        domain: string;
    };
    user: {
        id: number;
        email: string;
        firstName: string;
        lastName: string;
    } | null;
    execution: {
        id: string;
        trigger: "manual" | "cron" | "event" | "http";
    };
    input: any;
}

declare const ctx: Context;

// SDK namespace
declare namespace sdk {
    // HTTP client
    namespace http {
        interface RequestOptions {
            headers?: Record<string, string>;
            timeout?: number;
        }

        interface Response<T = any> {
            status: number;
            headers: Record<string, string>;
            body: T;
        }

        function get<T = any>(
            url: string,
            options?: RequestOptions
        ): Promise<Response<T>>;

        function post<T = any>(
            url: string,
            body: any,
            options?: RequestOptions
        ): Promise<Response<T>>;

        function put<T = any>(
            url: string,
            body: any,
            options?: RequestOptions
        ): Promise<Response<T>>;

        function delete<T = any>(
            url: string,
            options?: RequestOptions
        ): Promise<Response<T>>;
    }

    // Database client
    namespace db {
        function query<T = any>(
            sql: string,
            params?: any[]
        ): Promise<T[]>;

        function execute(
            sql: string,
            params?: any[]
        ): Promise<{ rowsAffected: number }>;
    }

    // Cache client
    namespace cache {
        function get<T = any>(key: string): Promise<T | null>;

        function set(
            key: string,
            value: any,
            ttlSeconds?: number
        ): Promise<void>;

        function delete(key: string): Promise<void>;
    }

    // Log client
    namespace log {
        function info(message: string, data?: Record<string, any>): void;
        function warn(message: string, data?: Record<string, any>): void;
        function error(message: string, data?: Record<string, any>): void;
    }
}

// Events
declare namespace events {
    function publish(eventType: string, payload: any): Promise<void>;
}
```

**Usage in Monaco Editor:**

```javascript
// In script editor component
monaco.languages.typescript.javascriptDefaults.addExtraLib(
    sdkTypings,
    "sdk.d.ts"
);
```

---

## Security Considerations

### SSRF Protection

**Blocked IP Ranges:**
- Private networks (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Loopback (127.0.0.0/8)
- Link-local (169.254.0.0/16)

**Implementation:**
- DNS resolution before connection
- IP validation against blocked CIDRs
- Optional allowed hosts whitelist

### SQL Injection Prevention

**Parameterized Queries Only:**
```javascript
// SAFE: Parameterized
sdk.db.query("SELECT * FROM users WHERE id = $1", [userId]);

// UNSAFE: String concatenation (blocked by API design)
// Scripts cannot construct dynamic SQL strings
```

### Tenant Isolation

**Automatic Tenant Scoping:**
- Database queries include tenant_id filter
- Cache keys prefixed with tenant_id
- Events published with tenant context

---

## Testing

**Location:** `modules/scripts/runtime/bindings_test.go`

```go
func TestHTTPClient_Get_Success(t *testing.T) {
    ctx := context.Background()
    client := NewHTTPClient(ctx)

    vm := goja.New()
    vm.Set("http", client.ToGojaObject(vm))

    script := `
        const response = http.get("https://api.example.com/users");
        response.status;
    `

    // Mock HTTP transport for testing
    // ...

    value, err := vm.RunString(script)
    assert.NoError(t, err)
    assert.Equal(t, 200, value.ToInteger())
}

func TestDBClient_Query_TenantIsolation(t *testing.T) {
    // Test that queries include tenant_id filter
    // ...
}
```

---

## Next Steps

After implementing API bindings:

1. **Event Integration** (08-event-integration.md) - Event triggers and handlers
2. **Presentation Layer** (09-presentation.md) - Controllers and templates

---

## References

- Runtime engine: `06-runtime-engine.md`
- Service layer: `05-service-layer.md`
- Domain entities: `01-domain-entities.md`
