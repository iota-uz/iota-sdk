# JavaScript Runtime Integration for IOTA SDK

## Overview
This document outlines the integration of a JavaScript runtime into IOTA SDK to enable customizability through user-editable scripts directly in the Web UI. This feature will follow IOTA SDK's existing Domain-Driven Design (DDD) architecture and integrate seamlessly with the current event-driven system.

## Runtime Choice: Goja

### Selected Runtime: [Goja](https://github.com/dop251/goja)

**Advantages of Goja:**
- Pure Go implementation (no CGO required)
- ECMAScript 5.1 compliant with ES6/ES2015 features
- Excellent Go interoperability
- No external dependencies
- Better for deployment and cross-compilation
- Lower memory footprint compared to V8/Node bindings
- Synchronous execution model fits well with Go's concurrency patterns
- Can be easily integrated with IOTA SDK's context-based architecture

**Why not CGO-based solutions (Node.js/QuickJS bindings):**
- CGO complicates deployment and cross-compilation
- Increased binary size
- Potential stability issues with native bindings
- More complex error handling across FFI boundaries
- Harder to sandbox and control resource usage
- Additional dependencies for production deployments
- Would complicate IOTA SDK's clean architecture

## Core Capabilities

### 1. Scheduled Script Execution (Cron Jobs)
- **Purpose**: Run periodic tasks like fetching client records and sending SMS notifications
- **Features**:
  - Configurable cron expressions via Web UI
  - Script enable/disable toggle
  - Execution history and logs
  - Error handling and retry mechanisms
  - Resource limits (CPU time, memory)

### 2. One-off Script Execution
- **Purpose**: Run ad-hoc scripts for maintenance, data migration, or testing
- **Features**:
  - Execute scripts on-demand from UI
  - Input parameters support
  - Real-time output streaming
  - Execution timeout controls

### 3. Web-based Script Editor
- **Features**:
  - Syntax highlighting for JavaScript
  - Code completion and IntelliSense
  - Error highlighting and linting
  - Version history/drafts
  - Script templates/snippets
  - Dark/light theme support

### 4. HTTP Endpoint Creation
- **Purpose**: Create custom API endpoints using JavaScript
- **Features**:
  - Route definition (GET, POST, PUT, DELETE)
  - Request/response handling
  - Middleware support
  - Rate limiting per endpoint
  - API documentation generation

### 5. Embedded Script Execution
- **Purpose**: Generic mechanism to run JS scripts within IOTA SDK runtime
- **Use cases**:
  - Custom validation rules
  - Data transformation pipelines
  - Business logic extensions
  - Event handlers

## Architecture Design

### Module Structure (Following IOTA SDK DDD Pattern)
```
modules/scripts/
├── domain/
│   ├── aggregates/
│   │   ├── script/
│   │   │   ├── script.go              # Script interface (immutable domain entity)
│   │   │   ├── script_impl.go         # Script implementation with option pattern
│   │   │   ├── script_events.go       # CreatedEvent, UpdatedEvent, DeletedEvent, ExecutedEvent
│   │   │   └── script_repository.go   # Repository interface with FindParams
│   │   └── execution/
│   │       ├── execution.go           # Execution interface
│   │       ├── execution_impl.go      # Execution implementation
│   │       ├── execution_events.go    # StartedEvent, CompletedEvent, FailedEvent
│   │       └── execution_repository.go
│   ├── entities/
│   │   └── endpoint/
│   │       ├── endpoint.go            # HTTP endpoint configuration
│   │       └── endpoint_repository.go
│   └── value_objects/
│       ├── script_type.go             # Enum: Cron, OneOff, Endpoint, Embedded
│       ├── execution_status.go        # Enum: Pending, Running, Success, Failed, Timeout
│       ├── cron_expression.go         # Cron expression with validation
│       └── runtime_context.go         # Execution context with limits
├── infrastructure/
│   ├── runtime/
│   │   ├── goja_runtime.go           # Goja VM management
│   │   ├── sandbox.go                # Security sandbox implementation
│   │   ├── api_bindings.go           # JS API surface (db, http, etc.)
│   │   └── context_bridge.go         # Go context to JS context bridge
│   ├── persistence/
│   │   ├── models/models.go          # Database models
│   │   ├── script_repository.go      # Script repository implementation
│   │   ├── execution_repository.go   # Execution repository implementation
│   │   ├── scripts_mappers.go        # Domain<->DB mappers using pkg/mapping
│   │   ├── schema/scripts-schema.sql # Database schema
│   │   └── setup_test.go
│   └── scheduler/
│       ├── cron_scheduler.go         # Cron job scheduler
│       └── endpoint_router.go        # Dynamic HTTP endpoint registration
├── services/
│   ├── script_service.go             # Script CRUD with event publishing
│   ├── execution_service.go          # Script execution orchestration
│   ├── runtime_service.go            # Runtime pool management
│   └── setup_test.go
├── presentation/
│   ├── controllers/
│   │   ├── script_controller.go      # Script management endpoints
│   │   ├── execution_controller.go   # Execution history/control
│   │   ├── runtime_controller.go     # Runtime API endpoints
│   │   ├── dtos/
│   │   │   ├── script_dto.go         # Script DTOs with validation
│   │   │   └── execution_dto.go      # Execution DTOs
│   │   └── setup_test.go
│   ├── templates/
│   │   ├── pages/scripts/
│   │   │   ├── list.templ           # Script list with HTMX
│   │   │   ├── edit.templ           # Script editor (Monaco)
│   │   │   ├── new.templ            # New script form
│   │   │   └── executions.templ     # Execution history
│   │   └── components/
│   │       ├── editor.templ         # Monaco editor component
│   │       └── status_badge.templ   # Execution status badge
│   ├── viewmodels/
│   │   ├── script_viewmodel.go      # Script presentation models
│   │   └── execution_viewmodel.go   # Execution presentation models
│   ├── mappers/mappers.go           # Domain to presentation mapping
│   └── locales/
│       ├── en.json
│       ├── ru.json
│       └── uz.json
├── permissions/constants.go          # RBAC permissions
├── links.go                         # Navigation items
└── module.go                        # Module registration

### Database Schema (Following IOTA SDK Patterns)
```sql
-- Scripts table with tenant isolation
CREATE TABLE scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL, -- 'cron', 'endpoint', 'one_off', 'embedded'
    content TEXT NOT NULL,
    cron_expression VARCHAR(255),
    endpoint_path VARCHAR(255),
    endpoint_method VARCHAR(10),
    is_active BOOLEAN DEFAULT true,
    timeout_seconds INTEGER DEFAULT 30,
    max_memory_mb INTEGER DEFAULT 128,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),
    version INTEGER DEFAULT 1,
    CONSTRAINT unique_script_name_per_tenant UNIQUE (tenant_id, name),
    CONSTRAINT unique_endpoint_per_tenant UNIQUE (tenant_id, endpoint_path, endpoint_method)
);

-- Script executions table with tenant isolation
CREATE TABLE script_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    script_id UUID REFERENCES scripts(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL, -- 'pending', 'running', 'success', 'failed', 'timeout'
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,
    output TEXT,
    error TEXT,
    triggered_by VARCHAR(50), -- 'cron', 'manual', 'api', 'system'
    triggered_by_user UUID REFERENCES users(id),
    execution_context JSONB -- Store runtime context, parameters
);

-- Script versions table (for audit trail)
CREATE TABLE script_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    script_id UUID REFERENCES scripts(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),
    change_summary TEXT,
    CONSTRAINT unique_version_per_script UNIQUE (script_id, version)
);

-- Indexes for performance
CREATE INDEX idx_scripts_tenant_type ON scripts(tenant_id, type);
CREATE INDEX idx_scripts_tenant_active ON scripts(tenant_id, is_active);
CREATE INDEX idx_executions_tenant_script ON script_executions(tenant_id, script_id);
CREATE INDEX idx_executions_tenant_status ON script_executions(tenant_id, status);
CREATE INDEX idx_executions_started_at ON script_executions(started_at DESC);
```

## Domain Entity Design (Following IOTA SDK Patterns)

### Script Domain Entity
```go
// domain/aggregates/script/script.go
type Script interface {
    ID() uuid.UUID
    TenantID() uuid.UUID
    Name() string
    Description() string
    Type() ScriptType
    Content() string
    CronExpression() string
    EndpointPath() string
    EndpointMethod() string
    IsActive() bool
    TimeoutSeconds() int
    MaxMemoryMB() int
    CreatedAt() time.Time
    UpdatedAt() time.Time
    CreatedBy() uuid.UUID
    Version() int
    
    // Immutable update methods
    UpdateContent(content string) Script
    UpdateConfiguration(opts ...Option) Script
    Activate() Script
    Deactivate() Script
}

// domain/aggregates/script/script_repository.go
type Repository interface {
    Count(ctx context.Context, params *FindParams) (int64, error)
    GetAll(ctx context.Context) ([]Script, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]Script, error)
    GetByID(ctx context.Context, id uuid.UUID) (Script, error)
    GetByEndpoint(ctx context.Context, method, path string) (Script, error)
    GetActiveByType(ctx context.Context, scriptType ScriptType) ([]Script, error)
    Create(ctx context.Context, script Script) (Script, error)
    Update(ctx context.Context, script Script) (Script, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Service Layer Integration
```go
// services/script_service.go
type ScriptService struct {
    repo      script.Repository
    publisher eventbus.EventBus
}

func (s *ScriptService) Create(ctx context.Context, dto script.CreateDTO) (script.Script, error) {
    user, err := composables.UseUser(ctx)
    if err != nil {
        return nil, err
    }
    
    // Create domain entity
    script := script.New(
        dto.Name,
        script.WithType(dto.Type),
        script.WithContent(dto.Content),
        script.WithTenantID(user.TenantID()),
        script.WithCreatedBy(user.ID()),
    )
    
    // Persist
    created, err := s.repo.Create(ctx, script)
    if err != nil {
        return nil, err
    }
    
    // Publish domain event
    s.publisher.Publish(script.CreatedEvent{
        Sender:  user,
        Session: session,
        Data:    dto,
        Result:  created,
    })
    
    return created, nil
}
```

## JavaScript API Design

### Context-Aware API Surface

The JavaScript runtime will provide a context-aware API that respects IOTA SDK's tenant isolation and security model:

```javascript
// Execution context (automatically injected)
const context = {
    tenant: {
        id: "uuid",
        name: "Tenant Name"
    },
    user: {
        id: "uuid",
        email: "user@example.com",
        name: "User Name"
    },
    script: {
        id: "uuid",
        name: "Script Name",
        type: "cron|endpoint|embedded"
    },
    execution: {
        id: "uuid",
        triggeredBy: "cron|manual|api|system"
    }
};

// Database access (tenant-scoped)
const db = {
    // All queries are automatically scoped to current tenant
    query: async (sql, params) => {...},
    transaction: async (callback) => {...},
    
    // Helper methods for common operations
    findOne: async (table, conditions) => {...},
    findMany: async (table, conditions, options) => {...},
    insert: async (table, data) => {...},
    update: async (table, conditions, data) => {...},
    delete: async (table, conditions) => {...}
};

// Service access (via IOTA SDK services)
const services = {
    // Access to registered IOTA SDK services
    // Services are automatically scoped to tenant context
    clients: {
        list: async (filters) => {...},
        get: async (id) => {...},
        create: async (data) => {...},
        update: async (id, data) => {...},
        delete: async (id) => {...}
    },
    products: {
        list: async (filters) => {...},
        get: async (id) => {...},
        updateStock: async (id, quantity) => {...}
    },
    orders: {
        create: async (data) => {...},
        process: async (id) => {...},
        cancel: async (id) => {...}
    }
    // Additional services exposed based on module configuration
};

// Event publishing (integrated with IOTA EventBus)
const events = {
    publish: async (eventType, data) => {...},
    // Events are namespaced to prevent conflicts
    // Example: "scripts.custom.order_processed"
};

// HTTP client (with security restrictions)
const http = {
    // Requests are subject to whitelist and rate limiting
    get: async (url, options) => {...},
    post: async (url, data, options) => {...},
    put: async (url, data, options) => {...},
    delete: async (url, options) => {...}
};

// Storage (tenant-scoped key-value store)
const storage = {
    get: async (key) => {...},
    set: async (key, value, ttl) => {...},
    delete: async (key) => {...},
    // Keys are automatically prefixed with tenant ID
};

// Logging (structured, indexed)
const console = {
    log: (...args) => {...},
    error: (...args) => {...},
    warn: (...args) => {...},
    info: (...args) => {...},
    // Logs include execution context metadata
};

// Utilities
const utils = {
    // UUID generation
    uuid: () => {...},
    
    // Date/time helpers
    date: {
        now: () => new Date(),
        format: (date, format) => {...},
        parse: (dateStr, format) => {...},
        addDays: (date, days) => {...}
    },
    
    // Crypto utilities
    crypto: {
        hash: (data, algorithm) => {...},
        randomBytes: (length) => {...}
    },
    
    // Template rendering (for emails, SMS)
    template: {
        render: (template, data) => {...}
    }
};
```

### Example Scripts

#### Cron Job: Send SMS to Inactive Clients
```javascript
// Runs daily at 9 AM
// Cron expression: "0 9 * * *"
async function main() {
    console.info(`Starting inactive client reminder job for tenant: ${context.tenant.name}`);
    
    const thirtyDaysAgo = utils.date.addDays(utils.date.now(), -30);
    
    // Using service API (recommended approach)
    const inactiveClients = await services.clients.list({
        filters: [
            { field: 'last_activity', operator: 'lt', value: thirtyDaysAgo },
            { field: 'status', operator: 'eq', value: 'active' }
        ],
        limit: 100
    });
    
    let sentCount = 0;
    for (const client of inactiveClients) {
        try {
            // Use template for consistent messaging
            const message = await utils.template.render('inactive_client_reminder', {
                name: client.name,
                lastVisit: utils.date.format(client.last_activity, 'MMM DD')
            });
            
            // Send via integrated SMS service
            await services.notifications.sendSMS({
                to: client.phone,
                message: message,
                metadata: {
                    client_id: client.id,
                    campaign: 'reactivation'
                }
            });
            
            // Publish event for analytics
            await events.publish('client.reminder_sent', {
                clientId: client.id,
                type: 'inactivity',
                channel: 'sms'
            });
            
            sentCount++;
            console.log(`Sent reminder to ${client.name} (${client.id})`);
        } catch (error) {
            console.error(`Failed to send reminder to ${client.name}: ${error.message}`);
        }
    }
    
    // Store execution results
    await storage.set('last_reminder_run', {
        timestamp: utils.date.now(),
        processed: inactiveClients.length,
        sent: sentCount
    }, 86400); // TTL: 24 hours
    
    return {
        success: true,
        processed: inactiveClients.length,
        sent: sentCount
    };
}
```

#### HTTP Endpoint: Custom Order Report
```javascript
// GET /api/scripts/reports/orders
// Endpoint configuration: method=GET, path=/api/scripts/reports/orders
async function handleRequest(req) {
    // Validate permissions
    if (!context.user.permissions.includes('reports.view')) {
        return {
            status: 403,
            body: { error: 'Insufficient permissions' }
        };
    }
    
    // Parse and validate query parameters
    const startDate = req.query.startDate || utils.date.addDays(utils.date.now(), -30);
    const endDate = req.query.endDate || utils.date.now();
    const groupBy = req.query.groupBy || 'day';
    
    // Generate cache key
    const cacheKey = `order_report_${startDate}_${endDate}_${groupBy}`;
    
    // Check cache
    const cached = await storage.get(cacheKey);
    if (cached) {
        console.info('Returning cached report');
        return {
            status: 200,
            headers: { 'X-Cache': 'HIT' },
            body: cached
        };
    }
    
    // Generate report using service layer
    const orders = await services.orders.getReport({
        startDate: startDate,
        endDate: endDate,
        groupBy: groupBy,
        includeMetrics: ['count', 'revenue', 'average_value']
    });
    
    // Transform data for presentation
    const report = {
        period: {
            start: startDate,
            end: endDate
        },
        summary: {
            total_orders: orders.reduce((sum, day) => sum + day.count, 0),
            total_revenue: orders.reduce((sum, day) => sum + day.revenue, 0),
            average_order_value: orders.reduce((sum, day) => sum + day.revenue, 0) / 
                                orders.reduce((sum, day) => sum + day.count, 0)
        },
        data: orders,
        generated_at: utils.date.now(),
        generated_by: context.user.email
    };
    
    // Cache for 5 minutes
    await storage.set(cacheKey, report, 300);
    
    // Log analytics event
    await events.publish('report.generated', {
        type: 'orders',
        user: context.user.id,
        parameters: { startDate, endDate, groupBy }
    });
    
    return {
        status: 200,
        headers: { 
            'Content-Type': 'application/json',
            'X-Cache': 'MISS'
        },
        body: report
    };
}
```

#### Embedded Script: Order Validation Rule
```javascript
// Used within order processing workflow
async function validateOrder(order) {
    console.info(`Validating order ${order.id}`);
    
    const errors = [];
    
    // Check inventory availability
    for (const item of order.items) {
        const product = await services.products.get(item.product_id);
        if (!product) {
            errors.push(`Product ${item.product_id} not found`);
            continue;
        }
        
        if (product.stock < item.quantity) {
            errors.push(`Insufficient stock for ${product.name}: requested ${item.quantity}, available ${product.stock}`);
        }
        
        // Custom business rule: check minimum order quantity
        const minQty = await storage.get(`min_order_qty_${product.category}`);
        if (minQty && item.quantity < minQty) {
            errors.push(`Minimum order quantity for ${product.name} is ${minQty}`);
        }
    }
    
    // Validate customer credit limit
    const customer = await services.clients.get(order.customer_id);
    if (customer.credit_limit > 0) {
        const outstanding = await services.invoices.getOutstandingAmount(customer.id);
        if (outstanding + order.total > customer.credit_limit) {
            errors.push(`Order exceeds customer credit limit`);
        }
    }
    
    // Check delivery constraints
    if (order.delivery_date) {
        const dayOfWeek = new Date(order.delivery_date).getDay();
        const blockedDays = await storage.get('blocked_delivery_days') || [];
        if (blockedDays.includes(dayOfWeek)) {
            errors.push(`Delivery not available on selected day`);
        }
    }
    
    return {
        valid: errors.length === 0,
        errors: errors
    };
}
```

## Integration with IOTA SDK Architecture

### Module Registration
```go
// modules/scripts/module.go
func (m *Module) Register(app application.Application) error {
    // Register RBAC permissions
    app.RBAC().Register(
        permissions.ScriptCreate,
        permissions.ScriptRead,
        permissions.ScriptUpdate,
        permissions.ScriptDelete,
        permissions.ScriptExecute,
        permissions.ScriptManageEndpoints,
    )
    
    // Register database migrations
    app.Migrations().RegisterSchema(&MigrationFiles)
    
    // Register localization files
    app.RegisterLocaleFiles(&LocaleFiles)
    
    // Initialize repositories
    scriptRepo := persistence.NewScriptRepository()
    executionRepo := persistence.NewExecutionRepository()
    
    // Initialize runtime manager
    runtimeManager := runtime.NewRuntimeManager(runtime.Config{
        MaxVMs:           10,
        VMTimeout:        30 * time.Second,
        MaxMemoryPerVM:   128 * 1024 * 1024, // 128MB
        EnableCache:      true,
    })
    
    // Register services
    app.RegisterServices(
        services.NewScriptService(scriptRepo, app.EventPublisher()),
        services.NewExecutionService(executionRepo, runtimeManager, app.EventPublisher()),
        services.NewRuntimeService(runtimeManager, app),
    )
    
    // Register controllers
    app.RegisterControllers(
        controllers.NewScriptController(app),
        controllers.NewExecutionController(app),
        controllers.NewRuntimeController(app),
    )
    
    // Register navigation
    app.RegisterNavItems(ScriptsNavItem)
    
    // Register quick links
    app.QuickLinks().Add(
        spotlight.NewQuickLink(nil, ScriptsLink.Name, ScriptsLink.Href),
    )
    
    // Start background services
    go services.StartCronScheduler(app)
    go services.StartEndpointRouter(app)
    
    return nil
}
```

### Service Exposure to JavaScript Runtime
```go
// infrastructure/runtime/api_bindings.go
type ServiceBinding struct {
    app application.Application
    vm  *goja.Runtime
    ctx context.Context
}

func (s *ServiceBinding) ExposeServices() error {
    services := s.vm.NewObject()
    
    // Expose client service
    clientService := s.app.Service((*client.Service)(nil)).(*client.Service)
    s.exposeClientService(services, clientService)
    
    // Expose other services based on configuration
    // This allows fine-grained control over what scripts can access
    
    return s.vm.Set("services", services)
}

func (s *ServiceBinding) exposeClientService(obj *goja.Object, service *client.Service) {
    clientObj := s.vm.NewObject()
    
    // Wrap service methods with context and error handling
    clientObj.Set("list", s.wrapAsync(func(filters map[string]interface{}) (interface{}, error) {
        // Convert JS filters to Go FindParams
        params := s.convertToFindParams(filters)
        return service.GetPaginated(s.ctx, params)
    }))
    
    clientObj.Set("get", s.wrapAsync(func(id string) (interface{}, error) {
        uuid, err := uuid.Parse(id)
        if err != nil {
            return nil, err
        }
        return service.GetByID(s.ctx, uuid)
    }))
    
    obj.Set("clients", clientObj)
}
```

## Security Considerations

### Sandboxing Implementation
```go
// infrastructure/runtime/sandbox.go
type Sandbox struct {
    vm          *goja.Runtime
    timeout     time.Duration
    memoryLimit int64
    startTime   time.Time
    ctx         context.Context
    cancel      context.CancelFunc
}

func (s *Sandbox) Execute(script string) (goja.Value, error) {
    // Set resource limits
    s.vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
    s.vm.SetMaxCallStackSize(1000)
    
    // Monitor execution time
    go func() {
        <-time.After(s.timeout)
        s.cancel()
    }()
    
    // Execute with panic recovery
    var result goja.Value
    var err error
    
    done := make(chan bool)
    go func() {
        defer func() {
            if r := recover(); r != nil {
                err = fmt.Errorf("script panic: %v", r)
            }
            done <- true
        }()
        
        result, err = s.vm.RunString(script)
    }()
    
    select {
    case <-done:
        return result, err
    case <-s.ctx.Done():
        return nil, errors.New("script execution timeout")
    }
}
```

### RBAC Permissions
```go
// permissions/constants.go
var (
    ScriptCreate = permission.New("scripts.create", "Scripts")
    ScriptRead   = permission.New("scripts.read", "Scripts")
    ScriptUpdate = permission.New("scripts.update", "Scripts")
    ScriptDelete = permission.New("scripts.delete", "Scripts")
    ScriptExecute = permission.New("scripts.execute", "Scripts")
    ScriptManageEndpoints = permission.New("scripts.manage_endpoints", "Scripts")
)
```

### Audit Trail Implementation
- All script executions logged with context in `script_executions` table
- Script versions tracked in `script_versions` table
- Integration with IOTA SDK's event system for comprehensive audit trail
- Execution logs include user, tenant, timestamp, duration, and outcome

## Runtime Architecture & Critical Design Decisions

### VM Pool Management
```go
// infrastructure/runtime/runtime_manager.go
type RuntimeManager struct {
    pool        chan *VMInstance
    config      Config
    app         application.Application
    metrics     *RuntimeMetrics
}

type VMInstance struct {
    vm          *goja.Runtime
    inUse       bool
    lastUsed    time.Time
    execCount   int
    tenantID    uuid.UUID
}

// Get VM from pool with tenant isolation
func (rm *RuntimeManager) GetVM(ctx context.Context) (*VMInstance, error) {
    tenantID, _ := composables.UseTenantID(ctx)
    
    select {
    case vm := <-rm.pool:
        // Reset VM if switching tenants
        if vm.tenantID != tenantID {
            vm.Reset()
            vm.tenantID = tenantID
        }
        return vm, nil
    case <-time.After(5 * time.Second):
        return nil, errors.New("no available VMs in pool")
    }
}
```

### Critical Design Decisions

1. **Synchronous vs Asynchronous Execution**
   - Decision: Use synchronous execution with goroutines for async operations
   - Rationale: Simplifies JavaScript code and error handling
   - Implementation: Wrap async Go operations in promises

2. **VM Pooling Strategy**
   - Decision: Maintain a pool of pre-initialized VMs per tenant
   - Rationale: Reduces cold start latency for script execution
   - Trade-off: Higher memory usage for better performance

3. **Service Access Control**
   - Decision: Explicit service registration per script type
   - Rationale: Fine-grained security control
   - Implementation: Service whitelist configuration

4. **Database Access Pattern**
   - Decision: Use service layer instead of direct SQL access
   - Rationale: Maintains domain boundaries and security
   - Alternative: Provide read-only SQL access with query validation

5. **Error Handling**
   - Decision: All errors returned as structured objects
   - Rationale: Consistent error handling in JavaScript
   - Pattern: `{ success: false, error: { code: 'ERR001', message: '...' } }`

### Performance Optimizations

1. **Script Compilation Cache**
```go
type ScriptCache struct {
    compiled map[string]*goja.Program
    mu       sync.RWMutex
}

func (c *ScriptCache) GetOrCompile(script Script) (*goja.Program, error) {
    c.mu.RLock()
    if compiled, ok := c.compiled[script.ID()]; ok {
        c.mu.RUnlock()
        return compiled, nil
    }
    c.mu.RUnlock()
    
    // Compile and cache
    program, err := goja.Compile(script.Name(), script.Content(), true)
    if err != nil {
        return nil, err
    }
    
    c.mu.Lock()
    c.compiled[script.ID()] = program
    c.mu.Unlock()
    
    return program, nil
}
```

2. **Connection Pooling for Script Database Access**
   - Separate connection pool for script executions
   - Read-replica routing for read-heavy scripts
   - Connection limits per tenant

3. **Memory Management**
   - Periodic VM garbage collection
   - Memory usage tracking per execution
   - Automatic VM recycling after N executions

## UI/UX Design

### Script List Page
- Table view with columns: Name, Type, Status, Last Run, Actions
- Quick filters: Active/Inactive, By Type
- Bulk actions: Enable/Disable, Delete

### Script Editor Page
- Monaco editor (VS Code editor) integration
- Split view: Code | Preview/Test
- Script metadata panel (name, description, type, schedule)
- Test execution panel with input/output

### Execution History Page
- Timeline view of executions
- Filter by script, status, date range
- Execution details: Duration, output, errors

### Dashboard Widget
- Quick stats: Active scripts, Recent executions, Failed jobs
- Quick actions: Run script, View logs

## Error Handling & Monitoring

### Error Categories and Handling

1. **Compilation Errors**
```go
type CompilationError struct {
    ScriptID   uuid.UUID
    Line       int
    Column     int
    Message    string
    SourceLine string
}

// Presented to user in editor with inline highlighting
```

2. **Runtime Errors**
```go
type RuntimeError struct {
    ScriptID    uuid.UUID
    ExecutionID uuid.UUID
    Type        string // "timeout", "memory", "panic", "user"
    Message     string
    StackTrace  string
    Context     map[string]interface{}
}

// Logged and available in execution history
```

3. **Service Errors**
```javascript
// In JavaScript, errors are wrapped for consistency
try {
    await services.clients.create(data);
} catch (error) {
    // error object structure:
    // {
    //   code: 'VALIDATION_ERROR',
    //   message: 'Client email already exists',
    //   field: 'email',
    //   details: {}
    // }
}
```

### Monitoring Integration

1. **Metrics Collection**
```go
type ScriptMetrics struct {
    ExecutionCount      prometheus.Counter
    ExecutionDuration   prometheus.Histogram
    MemoryUsage        prometheus.Gauge
    ErrorRate          prometheus.Counter
    ActiveScripts      prometheus.Gauge
}

// Exposed at /metrics endpoint for Prometheus
```

2. **OpenTelemetry Integration**
```go
func (s *ExecutionService) Execute(ctx context.Context, scriptID uuid.UUID) error {
    ctx, span := tracer.Start(ctx, "script.execute",
        trace.WithAttributes(
            attribute.String("script.id", scriptID.String()),
            attribute.String("script.type", script.Type()),
        ),
    )
    defer span.End()
    
    // Execution logic with span events
    span.AddEvent("script.compilation.start")
    // ...
    span.AddEvent("script.execution.complete")
}
```

3. **Structured Logging**
```go
logger.WithFields(logrus.Fields{
    "script_id":    script.ID(),
    "execution_id": execution.ID(),
    "tenant_id":    tenantID,
    "duration_ms":  duration.Milliseconds(),
    "memory_used":  memoryUsed,
    "status":       status,
}).Info("Script execution completed")
```

### Alerting Rules

1. **High Error Rate**: Alert when script error rate > 10% over 5 minutes
2. **Long Running Scripts**: Alert when execution time > 90% of timeout
3. **Memory Pressure**: Alert when VM memory usage > 80% of limit
4. **Pool Exhaustion**: Alert when VM pool utilization > 90%

## Development Experience

### Script Development Workflow

1. **Local Development**
```javascript
// scripts/dev-server.js
// Local development server that mimics production API
const { createDevServer } = require('@iota-sdk/scripts-dev');

createDevServer({
    script: './my-script.js',
    mockData: './mock-data.json',
    watch: true,
    port: 3001
});
```

2. **Testing Framework**
```javascript
// scripts/__tests__/my-script.test.js
import { testScript } from '@iota-sdk/scripts-test';

describe('Inactive Client Reminder', () => {
    it('should send SMS to inactive clients', async () => {
        const result = await testScript('./inactive-reminder.js', {
            mockServices: {
                clients: {
                    list: async () => [
                        { id: '123', name: 'Test Client', phone: '+1234567890' }
                    ]
                }
            }
        });
        
        expect(result.sent).toBe(1);
    });
});
```

3. **VS Code Extension**
- Syntax highlighting for IOTA Script API
- IntelliSense for available services
- Inline error highlighting
- Run/debug scripts directly from editor

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)
- Goja runtime integration
- Basic script CRUD operations
- Simple script execution

### Phase 2: Scheduled Execution (Week 3)
- Cron scheduler implementation
- Execution history tracking
- Basic monitoring

### Phase 3: Web Editor (Week 4)
- Monaco editor integration
- Syntax highlighting and validation
- Version history

### Phase 4: HTTP Endpoints (Week 5)
- Dynamic route registration
- Request/response handling
- API documentation

### Phase 5: Security & Polish (Week 6)
- Sandboxing improvements
- Performance optimization
- UI/UX refinements

## Testing Strategy

### Unit Tests
- Runtime sandbox tests
- Script validation tests
- Cron expression parsing

### Integration Tests
- Script execution with database access
- HTTP endpoint creation and routing
- Scheduled job execution

### E2E Tests
- Script creation workflow
- Execution monitoring
- Error handling scenarios

## Performance Considerations

- Script compilation caching
- Connection pooling for database access
- Execution queue with worker pool
- Metrics collection for monitoring

## Migration Strategy

### For Existing IOTA SDK Users

1. **Gradual Adoption**
   - Scripts module can be enabled/disabled via feature flag
   - No impact on existing functionality when disabled
   - Can start with read-only scripts before enabling modifications

2. **Migration Path for Custom Logic**
   - Identify repetitive manual tasks currently performed
   - Create script templates for common operations
   - Gradually move business logic from external systems to scripts

3. **Data Migration**
   - No data migration required for new installations
   - For existing custom integrations, provide import tools

## Risks & Mitigation Strategies

### Technical Risks

1. **Performance Impact**
   - Risk: Script execution affecting main application performance
   - Mitigation: Separate worker processes, resource limits, circuit breakers

2. **Security Vulnerabilities**
   - Risk: Malicious scripts accessing unauthorized data
   - Mitigation: Strict sandboxing, service whitelisting, audit logging

3. **Resource Exhaustion**
   - Risk: Scripts consuming excessive CPU/memory
   - Mitigation: Hard limits, timeouts, monitoring, quotas per tenant

### Operational Risks

1. **Debugging Complexity**
   - Risk: Hard to debug production issues in user scripts
   - Mitigation: Comprehensive logging, execution replay, development tools

2. **Backward Compatibility**
   - Risk: API changes breaking existing scripts
   - Mitigation: API versioning, deprecation notices, migration tools

3. **Support Burden**
   - Risk: Users requiring help with script development
   - Mitigation: Extensive documentation, examples, community forum

## Alternative Approaches Considered

1. **WebAssembly (WASM)**
   - Pros: Language agnostic, better sandboxing
   - Cons: Complex toolchain, limited ecosystem, harder debugging
   - Decision: Goja chosen for simplicity and JavaScript familiarity

2. **Lua Integration**
   - Pros: Lightweight, designed for embedding
   - Cons: Less familiar to developers, smaller ecosystem
   - Decision: JavaScript preferred for wider adoption

3. **Remote Code Execution Service**
   - Pros: Complete isolation, language flexibility
   - Cons: Network latency, complex deployment, harder integration
   - Decision: Embedded runtime for better performance and integration

## Success Metrics

1. **Adoption Metrics**
   - Number of active scripts per tenant
   - Script execution frequency
   - User engagement with script editor

2. **Performance Metrics**
   - Average script execution time
   - VM pool utilization
   - Error rates by script type

3. **Business Impact**
   - Reduction in custom development requests
   - Time saved on repetitive tasks
   - Customer satisfaction scores

## Conclusion

The JavaScript runtime integration represents a significant enhancement to IOTA SDK's extensibility. By following the existing DDD architecture and leveraging Goja's pure-Go implementation, we can provide a powerful, secure, and maintainable scripting solution that empowers users while maintaining system integrity.

The phased implementation approach allows for iterative development and testing, ensuring each component is production-ready before moving to the next phase. With proper monitoring, security measures, and developer tools, this feature will enable IOTA SDK users to customize their experience without compromising the core system's stability.

## Appendix: Code References

### Key IOTA SDK Patterns Used
- Module registration: modules/*/module.go:Register
- Domain entities: modules/finance/domain/aggregates/payment_category/payment_category.go:49
- Repository pattern: modules/finance/domain/aggregates/payment_category/payment_category_repository.go:33
- Service layer: modules/finance/services/payment_category_service.go:12
- Event publishing: pkg/eventbus/event_bus.go:13
- RBAC integration: pkg/rbac/rbac.go:82
- Context usage: pkg/composables/auth.go:67
- Controller pattern: modules/core/presentation/controllers/crud_controller.go:95
- Templ templates: modules/finance/presentation/templates/pages/payment_categories/categories.templ:20