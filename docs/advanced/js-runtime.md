---
layout: default
title: JavaScript Runtime
parent: Advanced
nav_order: 1
description: "JavaScript runtime integration using Goja for IOTA SDK"
---

# JavaScript Runtime

The IOTA SDK includes a powerful JavaScript runtime based on [Goja](https://github.com/dop251/goja) for executing user-defined scripts with access to database, services, and HTTP capabilities.

## Overview

The JavaScript Runtime enables:

- **Scheduled Scripts**: Run periodic tasks with cron expressions
- **One-off Scripts**: Execute ad-hoc scripts on demand
- **HTTP Endpoints**: Create custom API endpoints with JavaScript
- **Event Handlers**: Respond to application events
- **Data Transformations**: Custom validation and transformation rules
- **System Integration**: Access to services, database, and external APIs

## Architecture

### Runtime Features

```go
// Goja runtime provides:
// - ECMAScript 5.1 compliance
// - ES6/ES2015 features support
// - Pure Go implementation (no CGO)
// - Excellent error messages
// - Synchronous execution model
// - Resource limits (CPU time, memory)
```

### Script Types

1. **Cron Jobs**: Scheduled execution with cron expressions
2. **One-Off Scripts**: Manual execution from UI or API
3. **HTTP Endpoints**: Dynamic routes responding to HTTP requests
4. **Embedded Scripts**: Inline scripts for validation/transformation
5. **Event Handlers**: React to domain events

## API Surface

### Context Object
```javascript
const context = {
    tenant: { id: "uuid", name: "Tenant Name" },
    user: { id: "uuid", email: "user@example.com", name: "User Name" },
    script: { id: "uuid", name: "Script Name", type: "cron" },
    execution: { id: "uuid", triggeredBy: "cron|manual|api" }
};
```

### Database Access
```javascript
// Tenant-scoped database queries
const result = await db.query("SELECT * FROM users WHERE id = $1", [userID]);
const user = await db.findOne("users", { email: "test@example.com" });
const users = await db.findMany("users", { status: "active" });
```

### Service Layer
```javascript
// Access to IOTA SDK services (tenant-scoped)
const clients = await services.clients.list({ limit: 100 });
const client = await services.clients.get(clientID);
const newClient = await services.clients.create({ name: "New Client" });
```

### HTTP Client
```javascript
// Make external HTTP requests
const response = await http.get("https://api.example.com/data");
const posted = await http.post("https://api.example.com/data", { key: "value" });
```

### Storage (Key-Value Store)
```javascript
// Tenant-scoped persistent storage
await storage.set("key", { data: "value" }, 3600); // TTL: 1 hour
const value = await storage.get("key");
await storage.delete("key");
```

### Utilities
```javascript
// UUID generation
const id = utils.uuid();

// Date/time helpers
const now = utils.date.now();
const formatted = utils.date.format(now, "YYYY-MM-DD");
const future = utils.date.addDays(now, 30);

// Crypto utilities
const hash = utils.crypto.hash("data", "sha256");
```

## Usage Examples

### Cron Job: Send Notifications

```javascript
// Cron expression: "0 9 * * *" (daily at 9 AM)
async function main() {
    console.info(`Running notification job for tenant: ${context.tenant.name}`);

    // Get inactive clients (no activity in 30 days)
    const thirtyDaysAgo = utils.date.addDays(utils.date.now(), -30);

    const clients = await services.clients.list({
        filters: [
            { field: 'last_activity', operator: 'lt', value: thirtyDaysAgo }
        ],
        limit: 100
    });

    let notified = 0;

    for (const client of clients) {
        try {
            // Send notification
            await services.notifications.sendEmail({
                to: client.email,
                subject: "We miss you!",
                body: `Hello ${client.name}, we haven't seen you in 30 days.`
            });

            // Log the event
            await events.publish('client.notified', {
                client_id: client.id,
                type: 'inactivity'
            });

            notified++;
            console.log(`Notified ${client.name}`);
        } catch (error) {
            console.error(`Failed to notify ${client.name}: ${error.message}`);
        }
    }

    return {
        success: true,
        processed: clients.length,
        notified: notified
    };
}
```

### Order Validation Rule

```javascript
// Validate orders before processing
async function validateOrder(order) {
    const errors = [];

    // Check inventory
    for (const item of order.items) {
        const product = await services.products.get(item.product_id);
        if (!product) {
            errors.push(`Product not found: ${item.product_id}`);
            continue;
        }

        if (product.stock < item.quantity) {
            errors.push(`Insufficient stock for ${product.name}`);
        }
    }

    // Check credit limit
    const customer = await services.clients.get(order.customer_id);
    if (customer.credit_limit > 0) {
        const outstanding = await db.query(
            "SELECT COALESCE(SUM(amount), 0) as total FROM invoices WHERE customer_id = $1 AND status != 'paid'",
            [customer.id]
        );

        if (outstanding[0].total + order.total > customer.credit_limit) {
            errors.push(`Order exceeds credit limit`);
        }
    }

    return {
        valid: errors.length === 0,
        errors: errors
    };
}
```

### HTTP Endpoint: Custom Report

```javascript
// Endpoint: GET /api/custom/sales-report?start=2024-01-01&end=2024-01-31
async function handleRequest(req) {
    // Validate permissions
    if (!context.user.permissions.includes('reports.view')) {
        return {
            status: 403,
            body: { error: 'Access denied' }
        };
    }

    const startDate = req.query.start || utils.date.addDays(utils.date.now(), -30);
    const endDate = req.query.end || utils.date.now();

    // Query sales data
    const sales = await db.query(
        `SELECT DATE(order_date) as date, COUNT(*) as orders, SUM(total) as revenue
         FROM orders
         WHERE order_date BETWEEN $1 AND $2
         GROUP BY DATE(order_date)
         ORDER BY date DESC`,
        [startDate, endDate]
    );

    // Aggregate data
    const summary = {
        total_orders: sales.reduce((sum, day) => sum + day.orders, 0),
        total_revenue: sales.reduce((sum, day) => sum + day.revenue, 0),
        days_included: sales.length
    };

    return {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
        body: {
            summary: summary,
            daily: sales
        }
    };
}
```

### Event Handler: Update Analytics

```javascript
// Handle order.created event
async function handleOrderCreated(event) {
    const order = event.data;

    // Update daily sales cache
    const today = utils.date.format(utils.date.now(), "YYYY-MM-DD");
    const cacheKey = `daily_sales_${today}`;

    const current = await storage.get(cacheKey) || { count: 0, total: 0 };
    current.count += 1;
    current.total += order.total;

    await storage.set(cacheKey, current, 86400); // Cache for 24 hours

    console.info(`Updated sales cache: ${current.count} orders, $${current.total}`);
}
```

## Configuration

### Environment Variables

```bash
# Enable JavaScript runtime
SCRIPTS_ENABLED=true

# Script execution limits
SCRIPT_TIMEOUT_SECONDS=30
SCRIPT_MAX_MEMORY_MB=128

# VM pool configuration
SCRIPT_VM_POOL_SIZE=10
SCRIPT_VM_TIMEOUT_SECONDS=30
```

### Module Registration

```go
// In your application setup
app.RegisterModule(scripts.NewModule())

// Permissions for script management
permissions.ScriptCreate     // Create new scripts
permissions.ScriptRead       // View scripts
permissions.ScriptUpdate     // Modify scripts
permissions.ScriptDelete     // Delete scripts
permissions.ScriptExecute    // Execute scripts
```

## Security Considerations

### Sandboxing

Scripts are executed in a sandbox with:
- Disabled global scope access
- No filesystem access
- Limited to whitelisted services
- Memory and CPU time limits
- Request timeout enforcement

### Tenant Isolation

All database queries and service calls are automatically scoped to the current tenant:

```javascript
// All queries are tenant-scoped
const users = await db.query("SELECT * FROM users"); // Only current tenant's users
const clients = await services.clients.list(); // Only current tenant's clients
```

### Permissions

Scripts inherit user permissions:

```javascript
// User's role determines accessible services
// If user can't access reports, script can't either
if (!context.user.permissions.includes('reports.view')) {
    throw new Error('Access denied');
}
```

## Error Handling

### Compilation Errors

```javascript
// Syntax errors caught during compilation
try {
    // Invalid syntax
    const x = [;
} catch (e) {
    // Returns detailed error with line/column info
    console.error(`Syntax error: ${e.message}`);
}
```

### Runtime Errors

```javascript
// Runtime errors with stack traces
try {
    const result = await services.clients.get(null);
} catch (e) {
    console.error(`Service error: ${e.message}`);
    // Returns error with context
}
```

### Timeout Handling

```javascript
// Scripts automatically timeout if execution exceeds limit
// DEFAULT: 30 seconds
// Configure via SCRIPT_TIMEOUT_SECONDS

// Catch timeout errors
try {
    while (true) { } // Infinite loop
} catch (e) {
    console.error("Script timeout"); // Caught at 30 seconds
}
```

## Performance Optimization

### Caching

```javascript
// Cache expensive computations
const cacheKey = `expensive_calculation_${param}`;
const cached = await storage.get(cacheKey);

if (cached) {
    return cached;
}

// Expensive operation
const result = await expensiveCalculation();

// Cache for 1 hour
await storage.set(cacheKey, result, 3600);
return result;
```

### Batch Operations

```javascript
// Use batch operations for efficiency
const userIDs = await db.query(
    "SELECT id FROM users WHERE status = 'active' LIMIT 1000"
);

// Process in batches to avoid memory issues
for (let i = 0; i < userIDs.length; i += 100) {
    const batch = userIDs.slice(i, i + 100);
    await processBatch(batch);
}
```

## Testing Scripts

### Local Development

```bash
# Use SDK's script development server
make scripts dev-server

# Test scripts locally before uploading
node scripts/test-runner.js my-script.js
```

### Unit Testing

```javascript
// Test validation logic
describe('Order Validation', () => {
    it('should reject orders exceeding credit limit', async () => {
        const order = { customer_id: 123, total: 10000 };
        const result = await validateOrder(order);
        expect(result.valid).toBe(false);
    });
});
```

## Monitoring and Logging

### Structured Logging

```javascript
// Logs include context automatically
console.log('User action', { user_id: context.user.id, action: 'create' });

// Outputs:
// {
//     "message": "User action",
//     "level": "info",
//     "tenant_id": "...",
//     "user_id": "...",
//     "user_action": "create",
//     "source_file": "my-script.js"
// }
```

### Metrics

Scripts can publish metrics:

```javascript
// Track execution metrics
await events.publish('script.metric', {
    script_name: context.script.name,
    duration_ms: 1234,
    items_processed: 500,
    errors: 0
});
```

## Limitations and Workarounds

| Limitation | Workaround |
|-----------|-----------|
| No filesystem access | Use database or object storage |
| No native modules | Use SDK-provided APIs |
| Single-threaded | Use async operations |
| Memory limit | Process in batches |
| Time limit | Optimize code or split tasks |

## Examples Repository

See `/docs/examples/scripts/` for more examples:

- `send-notifications.js` - Scheduled notifications
- `data-sync.js` - Sync data with external service
- `report-generator.js` - Generate reports
- `webhook-processor.js` - Process webhooks
- `validation-rules.js` - Order validation

---

For more information, see the [Advanced Features Overview](./index.md) or the [JavaScript Runtime Integration Spec](../js-runtime/js-runtime-integration-spec.md).
