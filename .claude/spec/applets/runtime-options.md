# Runtime Options: JavaScript Execution Engines

**Status:** Draft

## Overview

This document compares JavaScript runtime options for executing applet backend code. The choice impacts:

- Developer experience (TypeScript support, debugging)
- Performance (startup time, execution speed)
- Deployment complexity
- Security (sandboxing capabilities)
- Ecosystem access (npm packages)

## Comparison Matrix

| Feature | Goja | Bun | Node.js | Deno | V8 (cgo) |
|---------|------|-----|---------|------|----------|
| **Deployment** | Embedded | Sidecar | Sidecar | Sidecar | Embedded |
| **Go Integration** | Native | IPC | IPC | IPC | FFI (cgo) |
| **TypeScript** | Transpile | Native | Via tsc | Native | Transpile |
| **ES Version** | ES5.1+ | ES2024 | ES2024 | ES2024 | ES2024 |
| **async/await** | Polyfill | Native | Native | Native | Native |
| **npm Packages** | No | Yes | Yes | Partial | No |
| **Startup Time** | ~1ms | ~25ms | ~50ms | ~30ms | ~50ms |
| **Execution Speed** | Slow | Fast | Fast | Fast | Fast |
| **Memory Safety** | Go GC | Separate | Separate | Separate | Manual |
| **Sandboxing** | Language | Process | Process | Built-in | Language |
| **Windows Support** | Yes | Yes | Yes | Yes | Yes |
| **React SSR** | Limited | Native | Native | Native | Limited |

## Detailed Analysis

### Option 1: Goja (Embedded)

**What it is:** Pure Go JavaScript interpreter, no external dependencies

```go
import "github.com/dop251/goja"

func executeScript(source string, context map[string]interface{}) (interface{}, error) {
    vm := goja.New()

    // Inject SDK APIs
    vm.Set("sdk", map[string]interface{}{
        "db": databaseAPI,
        "http": httpAPI,
        "log": loggingAPI,
    })

    // Execute
    result, err := vm.RunString(source)
    return result.Export(), err
}
```

**Capabilities:**
- ES5.1 full support
- ES6+ partial (let/const, arrow functions, template literals, classes)
- No native async/await (requires promise polyfill)
- No native modules (require/import)

**TypeScript Workflow:**
```
TypeScript → tsc/esbuild → ES5 bundle → Goja execution
```

**Pros:**
- Zero external dependencies
- Single binary deployment
- Direct Go function calls
- Memory managed by Go GC
- Already have jsruntime spec

**Cons:**
- Limited ES6+ features
- No async/await (major DX issue)
- Slow execution (10-100x slower than V8)
- Complex bundling for TypeScript
- No npm package ecosystem
- React SSR is impractical

**Best Use Cases:**
- Simple webhook handlers
- Data transformations
- Scheduled tasks
- Event handlers with simple logic

**Not Suitable For:**
- Complex React UIs
- Heavy computation
- Async-heavy code
- npm package dependencies

---

### Option 2: Bun (Sidecar)

**What it is:** Modern JavaScript runtime built on JavaScriptCore (Safari's engine)

```bash
# Install
curl -fsSL https://bun.sh/install | bash

# Run applet
bun run applet-server.ts
```

**Capabilities:**
- Full ES2024 support
- Native TypeScript (no transpilation needed)
- Native JSX/TSX support
- Built-in bundler, test runner, package manager
- npm compatible
- Web-standard APIs (fetch, WebSocket, etc.)

**Architecture:**
```
Go SDK ──Unix Socket──► Bun Process
                        ├── TypeScript execution
                        ├── React SSR
                        └── npm packages
```

**Applet Server Example:**
```typescript
// applet-server.ts
import { serve } from "bun";

const handlers = new Map<string, Function>();

// Load applet handlers
import * as configHandler from "./handlers/config";
handlers.set("config", configHandler.default);

serve({
  unix: "/tmp/applet-ai-chat.sock",
  async fetch(req) {
    const { handler, context, payload } = await req.json();
    const fn = handlers.get(handler);
    if (!fn) {
      return Response.json({ error: "Handler not found" }, { status: 404 });
    }
    const result = await fn(context, payload);
    return Response.json(result);
  },
});
```

**Pros:**
- Native TypeScript support
- Fastest startup among sidecars (~25ms)
- Built-in bundler (no webpack/esbuild needed)
- Full npm compatibility
- Native React SSR
- Web-standard APIs
- Excellent developer experience
- Active development, modern architecture

**Cons:**
- External process to manage
- IPC overhead
- Newer runtime (less battle-tested than Node)
- Requires Bun installation
- Resource isolation per-process

**Best Use Cases:**
- React-based applet UIs
- Complex TypeScript services
- External API integrations
- Any applet needing npm packages

---

### Option 3: Node.js (Sidecar)

**What it is:** The established JavaScript runtime

**Capabilities:**
- Full ES2024 support
- TypeScript via ts-node or pre-compilation
- Largest ecosystem (npm)
- Battle-tested, stable

**Pros:**
- Universal compatibility
- Massive ecosystem
- Well-documented
- Production proven
- Easy to find developers

**Cons:**
- Slower startup than Bun (~50ms)
- Requires TypeScript compilation step
- Legacy APIs alongside modern ones
- Larger memory footprint

**Best Use Cases:**
- When npm compatibility is critical
- Complex applications with many dependencies
- When hiring/familiarity matters

---

### Option 4: Deno (Sidecar)

**What it is:** Secure-by-default JavaScript runtime by Node's creator

**Capabilities:**
- Native TypeScript
- Permission-based security
- Web-standard APIs
- ES modules only

**Security Model:**
```bash
# Explicit permissions
deno run --allow-net=api.openai.com --allow-read=/data applet.ts
```

**Pros:**
- Security-first design (explicit permissions)
- Native TypeScript
- Web-standard APIs
- Good sandboxing

**Cons:**
- npm compatibility requires compatibility layer
- Smaller ecosystem
- Different module resolution
- Less widespread adoption

**Best Use Cases:**
- Security-critical applets
- When sandboxing is paramount
- Modern, standards-focused development

---

### Option 5: V8 via cgo (Embedded)

**What it is:** Google's V8 engine embedded via cgo bindings

```go
import "rogchap.com/v8go"

func executeScript(source string) (string, error) {
    ctx := v8.NewContext()
    val, err := ctx.RunScript(source, "applet.js")
    return val.String(), err
}
```

**Pros:**
- Fast execution (native V8)
- Full ES2024 support
- Embedded in Go process

**Cons:**
- Requires cgo (complicates cross-compilation)
- Large binary size (~20MB for V8)
- Complex memory management
- Debugging is harder
- React SSR still needs work

**Best Use Cases:**
- When embedded + fast execution is required
- Compute-intensive applets
- Not recommended as primary choice

---

## Recommendation

### Primary Runtime: Bun

**Rationale:**

1. **Developer Experience:** Native TypeScript, no build step for development
2. **Performance:** Fastest sidecar option
3. **React Support:** Native JSX/TSX and SSR
4. **Modern:** Built for today's JavaScript ecosystem
5. **Bundler Included:** Simplifies applet packaging

### Secondary Runtime: Goja (for jsruntime scripts)

Keep existing jsruntime spec for simple scripts:

- Webhook handlers
- Event processors
- Scheduled tasks
- One-off executions

**Migration Path:**

1. **Phase 1:** Implement Bun sidecar for complex applets
2. **Phase 2:** Keep Goja for simple scripts (jsruntime)
3. **Phase 3:** Allow applets to declare runtime preference in manifest

```yaml
# manifest.yaml
runtime:
  engine: bun  # or "goja" for simple handlers
  version: ">=1.0.0"
```

## Implementation Details

### Bun Process Management

```go
type BunRuntime struct {
    process *exec.Cmd
    socket  string
    mu      sync.Mutex
}

func (r *BunRuntime) Start(applet *Applet) error {
    r.socket = fmt.Sprintf("/tmp/applet-%s.sock", applet.ID)

    r.process = exec.Command("bun", "run", applet.ServerPath)
    r.process.Env = append(os.Environ(),
        fmt.Sprintf("SOCKET_PATH=%s", r.socket),
        fmt.Sprintf("APPLET_ID=%s", applet.ID),
    )

    return r.process.Start()
}

func (r *BunRuntime) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResponse, error) {
    conn, err := net.Dial("unix", r.socket)
    if err != nil {
        return nil, err
    }
    defer conn.Close()

    // Send request
    if err := json.NewEncoder(conn).Encode(req); err != nil {
        return nil, err
    }

    // Read response
    var resp ExecutionResponse
    if err := json.NewDecoder(conn).Decode(&resp); err != nil {
        return nil, err
    }

    return &resp, nil
}
```

### Health Checks

```go
func (r *BunRuntime) Health() (*HealthStatus, error) {
    resp, err := r.Execute(context.Background(), &ExecutionRequest{
        Type:    "health",
        Handler: "__health__",
    })
    if err != nil {
        return &HealthStatus{Status: "unhealthy", Error: err.Error()}, nil
    }
    return &HealthStatus{Status: "healthy"}, nil
}
```

### Resource Limits

```go
type ResourceLimits struct {
    MaxMemoryMB     int           `yaml:"max_memory_mb"`
    MaxCPUPercent   int           `yaml:"max_cpu_percent"`
    MaxExecutionMs  int           `yaml:"max_execution_ms"`
    MaxConcurrent   int           `yaml:"max_concurrent"`
}

// Applied via cgroups on Linux, process limits on other platforms
```

## Open Questions

1. **Hot Reload:** Should Bun process restart on applet code changes, or use Bun's built-in hot reload?

2. **Process Pool:** One Bun process per applet, or shared pool with isolation?

3. **Startup Strategy:** Start all applets on SDK boot, or lazy-start on first request?

4. **Crash Recovery:** Auto-restart crashed applets? How many retries? Circuit breaker?

5. **Logging:** Capture stdout/stderr from Bun process? Structured logging format?
