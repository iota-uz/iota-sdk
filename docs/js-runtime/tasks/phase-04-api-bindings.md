# Phase 4: JavaScript API Bindings & Event Integration (2 days)

## Overview
This phase focuses on creating the bridge between the Go runtime and the JavaScript execution environment. It involves building a secure, context-aware API surface that scripts can use to interact with the IOTA SDK, including accessing services, publishing events, and using utilities like logging and caching.

## Background
- The JavaScript API must be carefully designed to prevent security vulnerabilities.
- All API calls must be tenant-scoped and respect the user's permissions.
- The API should feel natural and idiomatic to JavaScript developers.
- We need to generate TypeScript definitions to provide a good developer experience.

## Task 4.1: Context Bridge and Service Bindings (Day 1)

### Objectives
- Inject execution context (tenant, user, script) into the JavaScript environment.
- Create a mechanism to expose Go services to JavaScript.
- Implement bindings for core services like `clients` and `products`.
- Ensure all service calls are asynchronous and return Promises in JavaScript.

### Detailed Steps

#### 1. Implement Context Bridge
Modify `infrastructure/runtime/context_bridge.go`:
```go
package runtime

import (
    "context"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

// InjectContext exposes Go context values to the Goja VM.
func InjectContext(ctx context.Context, vm *goja.Runtime, scriptID string, executionID string) error {
    // Expose tenant information
    tenant, err := composables.UseTenant(ctx)
    if err != nil {
        return err
    }
    vm.Set("tenant", map[string]interface{}{
        "id":   tenant.ID(),
        "name": tenant.Name(),
    })

    // Expose user information
    user, err := composables.UseUser(ctx)
    if err != nil {
        return err
    }
    vm.Set("user", map[string]interface{}{
        "id":          user.ID(),
        "email":       user.Email(),
        "name":        user.Name(),
        "permissions": user.Permissions(), // For client-side checks
    })

    // Expose script and execution info
    vm.Set("script", map[string]interface{}{
        "id": scriptID,
    })
    vm.Set("execution", map[string]interface{}{
        "id": executionID,
    })

    return nil
}
```

#### 2. Create Service Binding Framework
Create `infrastructure/runtime/api_bindings.go`:
```go
package runtime

import (
    "context"
    "github.com/dop251/goja"
    "github.com/iota-uz/iota-sdk/pkg/application"
)

// ServiceBinder exposes registered application services to a Goja VM.
type ServiceBinder struct {
    app application.Application
    vm  *goja.Runtime
    ctx context.Context
}

func NewServiceBinder(app application.Application, vm *goja.Runtime, ctx context.Context) *ServiceBinder {
    return &ServiceBinder{app: app, vm: vm, ctx: ctx}
}

// ExposeServices makes configured services available under the `services` global object.
func (b *ServiceBinder) ExposeServices() error {
    servicesObj := b.vm.NewObject()

    // Example: Expose Client service
    if clientService, ok := b.app.Service("ClientService").(client.Service); ok {
        b.exposeClientService(servicesObj, clientService)
    }
    
    // Example: Expose Product service
    if productService, ok := b.app.Service("ProductService").(product.Service); ok {
        b.exposeProductService(servicesObj, productService)
    }

    return b.vm.Set("services", servicesObj)
}

// wrapAsync wraps a Go function that returns (result, error) into a JavaScript Promise.
func (b *ServiceBinder) wrapAsync(goFunc func() (interface{}, error)) goja.Value {
    promise, resolve, reject := b.vm.NewPromise()

    go func() {
        result, err := goFunc()
        if err != nil {
            reject(err)
        } else {
            resolve(result)
        }
    }()

    return b.vm.ToValue(promise)
}
```

#### 3. Implement Specific Service Bindings
Continue in `infrastructure/runtime/api_bindings.go`:
```go
// exposeClientService creates the `services.clients` object.
func (b *ServiceBinder) exposeClientService(servicesObj *goja.Object, service client.Service) {
    clientObj := b.vm.NewObject()

    // list method
    clientObj.Set("list", func(call goja.FunctionCall) goja.Value {
        return b.wrapAsync(func() (interface{}, error) {
            // TODO: Convert JS filter object to Go FindParams
            var params client.FindParams
            return service.GetPaginated(b.ctx, &params)
        })
    })

    // get method
    clientObj.Set("get", func(call goja.FunctionCall) goja.Value {
        idStr := call.Argument(0).String()
        return b.wrapAsync(func() (interface{}, error) {
            id, err := uuid.Parse(idStr)
            if err != nil {
                return nil, serrors.Validation("invalid client ID format")
            }
            return service.GetByID(b.ctx, id)
        })
    })
    
    // create method
    clientObj.Set("create", func(call goja.FunctionCall) goja.Value {
        var dto client.CreateDTO
        // TODO: Map JS object from call.Argument(0) to the Go DTO
        return b.wrapAsync(func() (interface{}, error) {
            return service.Create(b.ctx, dto)
        })
    })

    servicesObj.Set("clients", clientObj)
}

// exposeProductService creates the `services.products` object.
func (b *ServiceBinder) exposeProductService(servicesObj *goja.Object, service product.Service) {
    // ... similar implementation for product service methods ...
}
```

### Testing Requirements
- Unit test `InjectContext` to ensure all context values are correctly set in the VM.
- Write integration tests that execute a simple script calling a bound service method (e.g., `services.clients.get(...)`).
- Mock the service layer to verify that the correct Go methods are called with the correct parameters.
- Test error handling: ensure that errors from the Go service are correctly propagated as rejected Promises in JavaScript.

## Task 4.2: Utility and Event Bindings (Day 2)

### Objectives
- Implement bindings for `events.publish`.
- Implement bindings for a tenant-scoped key-value `storage` (using Redis).
- Implement bindings for structured `console` logging.
- Implement common `utils` like UUID generation.
- Generate TypeScript definitions for the entire API surface.

### Detailed Steps

#### 1. Implement Event Bus Binding
In `infrastructure/runtime/api_bindings.go`:
```go
// ExposeEventBus creates the `events.publish` function.
func (b *ServiceBinder) ExposeEventBus() error {
    eventsObj := b.vm.NewObject()
    
    eventsObj.Set("publish", func(call goja.FunctionCall) goja.Value {
        eventName := call.Argument(0).String()
        payload := call.Argument(1).Export() // Export JS object to Go map/slice

        return b.wrapAsync(func() (interface{}, error) {
            // Create a generic event
            event := eventbus.NewGenericEvent(eventName, payload, b.ctx)
            b.app.EventPublisher().Publish(event)
            return nil, nil
        })
    })

    return b.vm.Set("events", eventsObj)
}
```

#### 2. Implement Storage (Cache) Binding
Create `infrastructure/runtime/storage_binding.go`:
```go
package runtime

// ... (similar structure to ServiceBinder)

// ExposeStorage creates the `storage` global object.
func (b *StorageBinder) ExposeStorage() error {
    storageObj := b.vm.NewObject()
    tenantID, _ := composables.UseTenantID(b.ctx)
    
    // get method
    storageObj.Set("get", func(call goja.FunctionCall) goja.Value {
        key := call.Argument(0).String()
        return b.wrapAsync(func() (interface{}, error) {
            // Prefix key with tenantID for isolation
            val, err := b.app.Cache().Get(b.ctx, tenantID.String()+":"+key).Result()
            // ... handle error and unmarshal if JSON ...
            return val, err
        })
    })

    // set method
    storageObj.Set("set", func(call goja.FunctionCall) goja.Value {
        key := call.Argument(0).String()
        value := call.Argument(1).Export()
        // ... TTL logic ...
        return b.wrapAsync(func() (interface{}, error) {
            // ... marshal value and set in cache with tenant prefix ...
            return nil, nil
        })
    })

    return b.vm.Set("storage", storageObj)
}
```

#### 3. Implement Console and Utils Bindings
- **Console**: Create a `console` binding that wraps the application's structured logger (e.g., Zap). Each log call (`console.log`, `console.error`) should automatically enrich the log entry with the execution context (tenantID, scriptID, etc.).
- **Utils**: Create a `utils` object with helper functions like `utils.uuid()` which calls `uuid.NewString()`.

#### 4. Generate TypeScript Definitions
Create a new command or `make` target that generates a `iota-sdk.d.ts` file.
```bash
# cmd/document/generate_ts.go
func main() {
    // Use reflection or a structured definition to generate the TS file
    // from the API binding implementations.
    // Output should match the spec.
}
```
This definition file will be crucial for the web-based editor in the next phase.

### Testing Requirements
- Test `events.publish` by executing a script and verifying that a corresponding event is captured by a mock event listener.
- Test `storage` methods by setting a value from a script and then reading it back, both from within the script and from outside (using a Redis client) to verify tenant-scoping.
- Test that `console.log` from a script produces a structured log entry with the correct context fields.
- Verify the generated `iota-sdk.d.ts` file is accurate and complete.

### Deliverables Checklist
- [ ] Context is correctly injected into the VM.
- [ ] Core services are securely exposed to JavaScript.
- [ ] `events.publish` is functional.
- [ ] `storage` API provides tenant-scoped caching.
- [ ] `console` API provides structured logging.
- [ ] `utils` API provides common helpers.
- [ ] A command to generate TypeScript definitions is implemented.
- [ ] Comprehensive integration tests for all API bindings.
- [ ] Documentation for the JavaScript API.